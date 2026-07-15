#!/usr/bin/env python3
"""Check repository Markdown links and built Starlight routes without network access."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import html
from html.parser import HTMLParser
from pathlib import Path
import re
import sys
import tempfile
from urllib.parse import unquote, urljoin, urlsplit

SITE_BASE = "/slipway"
EXTERNAL_SCHEMES = {"http", "https", "mailto", "tel", "data"}
SOURCE_NAMES = {
    "AGENTS.md",
    "CLAUDE.md",
    "CONTRIBUTING.md",
    "README.md",
    "README.ja.md",
    "README.zh.md",
    "SECURITY.md",
}
MARKDOWN_SUFFIXES = {".md", ".mdx"}
FENCE_RE = re.compile(r"^\s*(`{3,}|~{3,})")
REFERENCE_DEFINITION_RE = re.compile(
    r"^\s{0,3}\[([^]\n]+)\]:\s*(<[^>\n]+>|\S+)(?:\s+.*)?$", re.MULTILINE
)
REFERENCE_USE_RE = re.compile(r"(?<!!)\[([^]\n]+)\]\[([^]\n]*)\]")
HTML_ATTRIBUTE_RE = re.compile(
    r"\b(?P<name>href|src)\s*=\s*(?:\"(?P<double>[^\"\n]*)\"|'(?P<single>[^'\n]*)'|(?P<bare>[^\s>]+))",
    re.IGNORECASE,
)
HTML_ATTRIBUTE_START_RE = re.compile(r"\b(?:href|src)\s*=", re.IGNORECASE)
FRONTMATTER_LINK_RE = re.compile(r"^\s*link:\s*(.+?)\s*$", re.MULTILINE)
HEADING_RE = re.compile(r"^\s{0,3}(#{1,6})\s+(.+?)\s*#*\s*$", re.MULTILINE)
EXPLICIT_ID_RE = re.compile(r"\b(?:id|name)=[\"']([^\"']+)[\"']", re.IGNORECASE)


@dataclass(frozen=True, order=True)
class Problem:
    path: str
    line: int
    message: str


@dataclass(frozen=True)
class Link:
    target: str
    line: int


def repository_root() -> Path:
    return Path(__file__).resolve().parents[1]


def line_number(text: str, offset: int) -> int:
    return text.count("\n", 0, offset) + 1


def is_escaped(text: str, offset: int) -> bool:
    backslashes = 0
    offset -= 1
    while offset >= 0 and text[offset] == "\\":
        backslashes += 1
        offset -= 1
    return backslashes % 2 == 1


def mask_code(text: str) -> str:
    """Replace fenced and inline code with spaces while preserving offsets/newlines."""
    output: list[str] = []
    in_fence: str | None = None
    for line in text.splitlines(keepends=True):
        match = FENCE_RE.match(line)
        if match:
            marker = match.group(1)
            if in_fence is None:
                in_fence = marker[0]
            elif marker[0] == in_fence:
                in_fence = None
            output.append("".join("\n" if char == "\n" else " " for char in line))
            continue
        if in_fence is not None:
            output.append("".join("\n" if char == "\n" else " " for char in line))
            continue

        chars = list(line)
        index = 0
        while index < len(chars):
            if chars[index] != "`" or is_escaped(line, index):
                index += 1
                continue
            width = 1
            while index + width < len(chars) and chars[index + width] == "`":
                width += 1
            closing = line.find("`" * width, index + width)
            if closing < 0:
                index += width
                continue
            for masked in range(index, closing + width):
                if chars[masked] != "\n":
                    chars[masked] = " "
            index = closing + width
        output.append("".join(chars))
    return "".join(output)


def parse_destination(raw: str) -> str:
    value = raw.strip()
    if not value:
        raise ValueError("empty link destination")
    if value.startswith("<"):
        closing = value.find(">")
        if closing < 0:
            raise ValueError("unterminated angle-bracket destination")
        target = value[1:closing]
        trailer = value[closing + 1 :].strip()
    else:
        escaped = False
        split_at = len(value)
        for index, char in enumerate(value):
            if escaped:
                escaped = False
                continue
            if char == "\\":
                escaped = True
                continue
            if char.isspace():
                split_at = index
                break
        target = value[:split_at]
        trailer = value[split_at:].strip()
    if not target:
        raise ValueError("empty link destination")
    if trailer and not (
        (trailer.startswith('"') and trailer.endswith('"'))
        or (trailer.startswith("'") and trailer.endswith("'"))
        or (trailer.startswith("(") and trailer.endswith(")"))
    ):
        raise ValueError("malformed link title")
    return re.sub(r"\\([\\ ()])", r"\1", target)


def extract_links(path: Path, text: str) -> tuple[list[Link], list[Problem]]:
    masked = mask_code(text)
    links: list[Link] = []
    problems: list[Problem] = []
    consumed: list[tuple[int, int]] = []

    index = 0
    while index + 1 < len(masked):
        if masked[index : index + 2] != "](" or is_escaped(masked, index):
            index += 1
            continue
        depth = 1
        cursor = index + 2
        while cursor < len(masked):
            char = masked[cursor]
            if char == "\n" and depth > 0:
                break
            if is_escaped(masked, cursor):
                cursor += 1
                continue
            if char == "(":
                depth += 1
            elif char == ")":
                depth -= 1
                if depth == 0:
                    break
            cursor += 1
        line = line_number(text, index)
        if depth != 0:
            problems.append(Problem(str(path), line, "unterminated Markdown link destination"))
            index += 2
            continue
        try:
            target = parse_destination(text[index + 2 : cursor])
        except ValueError as error:
            problems.append(Problem(str(path), line, str(error)))
        else:
            links.append(Link(target, line))
        consumed.append((index, cursor + 1))
        index = cursor + 1

    definitions: dict[str, Link] = {}
    for match in REFERENCE_DEFINITION_RE.finditer(masked):
        label = " ".join(match.group(1).lower().split())
        try:
            target = parse_destination(text[match.start(2) : match.end(2)])
        except ValueError as error:
            problems.append(Problem(str(path), line_number(text, match.start()), str(error)))
            continue
        definitions[label] = Link(target, line_number(text, match.start()))
        links.append(definitions[label])

    for match in REFERENCE_USE_RE.finditer(masked):
        if any(start <= match.start() < end for start, end in consumed):
            continue
        label = match.group(2) or match.group(1)
        normalized = " ".join(label.lower().split())
        if normalized not in definitions:
            problems.append(
                Problem(
                    str(path),
                    line_number(text, match.start()),
                    f"undefined Markdown link reference [{label}]",
                )
            )

    html_matches = list(HTML_ATTRIBUTE_RE.finditer(masked))
    html_starts = list(HTML_ATTRIBUTE_START_RE.finditer(masked))
    matched_starts = {match.start() for match in html_matches}
    for match in html_starts:
        if match.start() not in matched_starts:
            problems.append(
                Problem(str(path), line_number(text, match.start()), "malformed HTML href/src attribute")
            )
    for match in html_matches:
        target = match.group("double") or match.group("single") or match.group("bare") or ""
        if not target:
            problems.append(Problem(str(path), line_number(text, match.start()), "empty HTML link"))
        else:
            links.append(Link(html.unescape(target), line_number(text, match.start())))

    for match in FRONTMATTER_LINK_RE.finditer(masked):
        raw = match.group(1).strip()
        if (raw.startswith('"') and raw.endswith('"')) or (
            raw.startswith("'") and raw.endswith("'")
        ):
            raw = raw[1:-1]
        if raw:
            links.append(Link(raw, line_number(text, match.start())))
        else:
            problems.append(Problem(str(path), line_number(text, match.start()), "empty frontmatter link"))

    unique = list(dict.fromkeys(links))
    return unique, problems


def github_slug(value: str) -> str:
    value = re.sub(r"<[^>]+>", "", value)
    value = re.sub(r"!?(?:\[([^]]+)\])\([^)]*\)", r"\1", value)
    value = re.sub(r"[`*_~]", "", html.unescape(value)).strip().lower()
    value = "".join(char for char in value if char.isalnum() or char in " _-")
    return re.sub(r"[\s]+", "-", value)


def markdown_anchors(path: Path) -> set[str]:
    text = path.read_text(encoding="utf-8")
    masked = mask_code(text)
    anchors = {html.unescape(value) for value in EXPLICIT_ID_RE.findall(masked)}
    seen: dict[str, int] = {}
    for match in HEADING_RE.finditer(masked):
        slug = github_slug(match.group(2))
        if not slug:
            continue
        count = seen.get(slug, 0)
        seen[slug] = count + 1
        anchors.add(slug if count == 0 else f"{slug}-{count}")
    return anchors


def exact_case_exists(path: Path, root: Path) -> bool:
    try:
        relative = path.resolve().relative_to(root.resolve())
    except (OSError, ValueError):
        return False
    current = root.resolve()
    for part in relative.parts:
        try:
            names = {entry.name for entry in current.iterdir()}
        except OSError:
            return False
        if part not in names:
            return False
        current /= part
    return current.exists()


def source_files(root: Path) -> list[Path]:
    files = [root / name for name in sorted(SOURCE_NAMES) if (root / name).is_file()]
    for base in (root / "docs", root / "acceptance"):
        if base.is_dir():
            files.extend(path for path in base.rglob("*.md") if path.is_file())
    content = root / "website" / "src" / "content" / "docs"
    for relative in ("index.mdx", "zh/index.mdx", "ja/index.mdx", "404.md"):
        path = content / relative
        if path.is_file():
            files.append(path)
    return sorted(set(files))


def website_routes(root: Path) -> dict[str, Path]:
    routes: dict[str, Path] = {}
    docs = root / "docs"
    if docs.is_dir():
        for path in docs.rglob("*.md"):
            relative = path.relative_to(docs).with_suffix("")
            parts = list(relative.parts)
            if parts[-1].lower() == "index":
                parts = parts[:-1]
            else:
                parts[-1] = parts[-1].lower()
            suffix = "/".join(parts)
            route = f"{SITE_BASE}/{suffix}/" if suffix else f"{SITE_BASE}/"
            routes[route] = path
    content = root / "website" / "src" / "content" / "docs"
    for relative, route in (
        ("index.mdx", f"{SITE_BASE}/"),
        ("zh/index.mdx", f"{SITE_BASE}/zh/"),
        ("ja/index.mdx", f"{SITE_BASE}/ja/"),
        ("404.md", f"{SITE_BASE}/404/"),
    ):
        path = content / relative
        if path.is_file():
            routes[route] = path
    return routes


def split_target(target: str) -> tuple[str, str]:
    parsed = urlsplit(target)
    path = unquote(parsed.path)
    fragment = unquote(parsed.fragment)
    return path, fragment


def normalize_route(path: str) -> str:
    if path == SITE_BASE:
        return f"{SITE_BASE}/"
    if not path.endswith("/") and not Path(path).suffix:
        path += "/"
    return path


def validate_fragment(target: Path, fragment: str, root: Path) -> str | None:
    if not fragment or target.suffix.lower() not in MARKDOWN_SUFFIXES:
        return None
    anchors = markdown_anchors(target)
    if fragment not in anchors:
        return f"fragment #{fragment} was not found in {target.relative_to(root)}"
    return None


def validate_source_link(
    source: Path, link: Link, root: Path, routes: dict[str, Path]
) -> Problem | None:
    target = link.target.strip()
    if not target:
        return Problem(str(source), link.line, "empty link target")
    if target.startswith("//"):
        return None
    parsed = urlsplit(target)
    if parsed.scheme:
        if parsed.scheme.lower() in EXTERNAL_SCHEMES:
            return None
        return Problem(str(source), link.line, f"unsupported or malformed link scheme: {parsed.scheme}")
    if parsed.netloc:
        return None

    path_part, fragment = split_target(target)
    if not path_part:
        message = validate_fragment(source, fragment, root)
        return Problem(str(source), link.line, message) if message else None

    if path_part == SITE_BASE or path_part.startswith(f"{SITE_BASE}/"):
        if path_part.startswith(f"{SITE_BASE}/assets/"):
            candidate = root / "docs" / path_part.removeprefix(f"{SITE_BASE}/")
            if not exact_case_exists(candidate, root):
                return Problem(str(source), link.line, f"website asset does not exist: {path_part}")
            return None
        public_candidate = root / "website" / "public" / path_part.removeprefix(f"{SITE_BASE}/")
        if Path(path_part).suffix and exact_case_exists(public_candidate, root):
            return None
        route = normalize_route(path_part)
        route_source = routes.get(route)
        if route_source is None:
            return Problem(str(source), link.line, f"website route does not exist: {route}")
        message = validate_fragment(route_source, fragment, root)
        return Problem(str(source), link.line, message) if message else None

    if path_part.startswith("/"):
        candidate = root / path_part.lstrip("/")
    else:
        candidate = source.parent / path_part
    try:
        candidate = candidate.resolve()
        candidate.relative_to(root.resolve())
    except (OSError, ValueError):
        return Problem(str(source), link.line, f"local link escapes repository: {target}")
    if not exact_case_exists(candidate, root):
        return Problem(str(source), link.line, f"local target does not exist: {target}")
    message = validate_fragment(candidate, fragment, root)
    return Problem(str(source), link.line, message) if message else None


class BuiltHTMLParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.links: list[str] = []
        self.anchors: set[str] = set()

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        values = dict(attrs)
        for name in ("id", "name"):
            if values.get(name):
                self.anchors.add(values[name] or "")
        for name in ("href", "src"):
            if values.get(name):
                self.links.append(values[name] or "")
        if values.get("srcset"):
            for candidate in (values["srcset"] or "").split(","):
                value = candidate.strip().split()[0] if candidate.strip() else ""
                if value:
                    self.links.append(value)


def built_target(site_dir: Path, current: Path, target: str) -> tuple[Path | None, str]:
    parsed = urlsplit(target)
    if parsed.scheme.lower() in EXTERNAL_SCHEMES or target.startswith("//"):
        return None, ""
    if parsed.scheme or parsed.netloc:
        raise ValueError(f"unsupported or malformed link: {target}")
    if parsed.path.startswith("/"):
        if parsed.path == SITE_BASE:
            relative = ""
        elif parsed.path.startswith(f"{SITE_BASE}/"):
            relative = parsed.path.removeprefix(f"{SITE_BASE}/")
        else:
            raise ValueError(f"built link escapes website base: {target}")
    else:
        current_route = "/" + current.relative_to(site_dir).as_posix()
        absolute = urlsplit(urljoin(f"https://local{current_route}", target)).path
        relative = absolute.lstrip("/")
    relative = unquote(relative)
    candidate = site_dir / relative
    if not Path(relative).suffix:
        candidate /= "index.html"
    elif candidate.is_dir():
        candidate /= "index.html"
    return candidate, unquote(parsed.fragment)


def check_built_site(root: Path, site_dir: Path) -> tuple[int, list[Problem]]:
    if not site_dir.is_dir():
        return 0, [Problem(str(site_dir), 1, "built website directory is missing")]
    parsed_pages: dict[Path, BuiltHTMLParser] = {}
    problems: list[Problem] = []
    checked = 0
    for page in sorted(site_dir.rglob("*.html")):
        parser = BuiltHTMLParser()
        try:
            parser.feed(page.read_text(encoding="utf-8"))
            parser.close()
        except (OSError, UnicodeError, ValueError) as error:
            problems.append(Problem(str(page), 1, f"cannot parse built HTML: {error}"))
            continue
        parsed_pages[page.resolve()] = parser
        for target in parser.links:
            checked += 1
            try:
                candidate, fragment = built_target(site_dir, page, target)
            except ValueError as error:
                problems.append(Problem(str(page.relative_to(root)), 1, str(error)))
                continue
            if candidate is None:
                continue
            candidate = candidate.resolve()
            try:
                candidate.relative_to(site_dir.resolve())
            except ValueError:
                problems.append(
                    Problem(str(page.relative_to(root)), 1, f"built link escapes output: {target}")
                )
                continue
            if not exact_case_exists(candidate, site_dir):
                problems.append(
                    Problem(str(page.relative_to(root)), 1, f"built target does not exist: {target}")
                )
                continue
            if fragment and candidate.suffix.lower() == ".html":
                target_parser = parsed_pages.get(candidate)
                if target_parser is None:
                    target_parser = BuiltHTMLParser()
                    target_parser.feed(candidate.read_text(encoding="utf-8"))
                    target_parser.close()
                    parsed_pages[candidate] = target_parser
                if fragment not in target_parser.anchors:
                    problems.append(
                        Problem(
                            str(page.relative_to(root)),
                            1,
                            f"built fragment #{fragment} was not found in {candidate.relative_to(site_dir)}",
                        )
                    )
    return checked, problems


def check_repository(root: Path, require_site: bool) -> tuple[int, int, int, list[Problem]]:
    root = root.resolve()
    routes = website_routes(root)
    files = source_files(root)
    problems: list[Problem] = []
    link_count = 0
    for path in files:
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeError) as error:
            problems.append(Problem(str(path.relative_to(root)), 1, f"cannot read Markdown: {error}"))
            continue
        links, extraction_problems = extract_links(path.relative_to(root), text)
        problems.extend(extraction_problems)
        link_count += len(links)
        for link in links:
            problem = validate_source_link(path, link, root, routes)
            if problem:
                problems.append(
                    Problem(str(path.relative_to(root)), problem.line, problem.message)
                )

    site_link_count = 0
    site_pages = 0
    if require_site:
        site_dir = root / "website" / "dist"
        site_pages = len(list(site_dir.rglob("*.html"))) if site_dir.is_dir() else 0
        site_link_count, site_problems = check_built_site(root, site_dir)
        problems.extend(site_problems)
    return len(files), link_count + site_link_count, site_pages, sorted(set(problems))


def self_test() -> None:
    with tempfile.TemporaryDirectory(prefix="slipway-link-check-") as directory:
        root = Path(directory)
        (root / "docs").mkdir()
        (root / "acceptance").mkdir(parents=True)
        (root / "website" / "src" / "content" / "docs").mkdir(parents=True)
        (root / "docs" / "index.md").write_text("# Docs\n\n[Guide](guide.md#hello-world)\n")
        (root / "docs" / "guide.md").write_text("# Hello, world!\n")
        (root / "README.md").write_text(
            "[root](/docs/guide.md#hello-world) [site](/slipway/guide/#hello-world) "
            "[external](https://example.invalid/x)\n"
        )
        (root / "website" / "src" / "content" / "docs" / "index.mdx").write_text(
            "---\ntitle: Home\n---\n\n[Guide](/slipway/guide/)\n"
        )
        files, links, _, problems = check_repository(root, require_site=False)
        assert files == 4 and links == 5 and not problems, problems

        (root / "README.md").write_text("[bad](docs/guide.md#missing)\n")
        _, _, _, problems = check_repository(root, require_site=False)
        assert any("fragment #missing" in problem.message for problem in problems), problems

        (root / "README.md").write_text("[broken](docs/guide.md\n")
        _, _, _, problems = check_repository(root, require_site=False)
        assert any("unterminated Markdown" in problem.message for problem in problems), problems
    print("link-check self-test: ok")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=repository_root())
    parser.add_argument("--require-site", action="store_true")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
    root = args.repo_root.resolve()
    files, links, pages, problems = check_repository(root, args.require_site)
    if problems:
        for problem in problems:
            print(f"{problem.path}:{problem.line}: {problem.message}", file=sys.stderr)
        print(f"link-check: {len(problems)} problem(s)", file=sys.stderr)
        return 1
    site = f", {pages} built pages" if args.require_site else ""
    print(f"link-check: {files} Markdown/MDX files, {links} local link observations{site}: ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
