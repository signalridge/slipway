#!/usr/bin/env python3
"""Validate GoReleaser archives, package metadata, and native Linux smoke behavior."""

from __future__ import annotations

import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path, PurePosixPath
import platform
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
from typing import Any
import zipfile

EXPECTED_LICENSE = "BSD-3-Clause"
EXPECTED_PACKAGE_FORMATS = {".apk", ".deb", ".rpm"}
ARCHITECTURES = {"amd64": "64bit", "arm64": "arm64"}


class CheckFailure(RuntimeError):
    pass


def fail(message: str) -> None:
    raise CheckFailure(message)


def strict_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            fail(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_json(path: Path) -> Any:
    try:
        with path.open("r", encoding="utf-8") as stream:
            return json.load(stream, object_pairs_hook=strict_object)
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        fail(f"cannot read strict JSON {path}: {error}")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def resolve_artifact(raw: object, dist: Path, repo: Path) -> Path:
    if not isinstance(raw, str) or not raw:
        fail(f"artifact path is missing or invalid: {raw!r}")
    candidate = Path(raw)
    attempts = [candidate] if candidate.is_absolute() else [repo / candidate, dist / candidate, dist / candidate.name]
    existing = [path.resolve() for path in attempts if path.is_file()]
    if existing:
        return existing[0]
    matches = sorted(path.resolve() for path in dist.rglob(candidate.name) if path.is_file())
    if len(matches) == 1:
        return matches[0]
    if not matches:
        fail(f"artifact does not exist: {raw}")
    fail(f"artifact path is ambiguous: {raw}: {matches}")


def safe_member_names(names: list[str], archive: Path) -> set[str]:
    counts = Counter(names)
    duplicates = sorted(name for name, count in counts.items() if count > 1)
    if duplicates:
        fail(f"{archive.name} has duplicate members: {duplicates}")
    normalized: set[str] = set()
    for name in names:
        pure = PurePosixPath(name)
        if pure.is_absolute() or ".." in pure.parts or not pure.parts:
            fail(f"{archive.name} has unsafe member path: {name}")
        normalized.add(pure.as_posix().removeprefix("./"))
    return normalized


def archive_contents(path: Path) -> dict[str, bytes]:
    result: dict[str, bytes] = {}
    if path.name.endswith(".tar.gz"):
        try:
            with tarfile.open(path, "r:gz") as archive:
                members = [member for member in archive.getmembers() if member.isfile()]
                names = [member.name for member in members]
                safe_member_names(names, path)
                for member in members:
                    stream = archive.extractfile(member)
                    if stream is None:
                        fail(f"cannot read {member.name} from {path.name}")
                    result[PurePosixPath(member.name).as_posix().removeprefix("./")] = stream.read()
        except (OSError, tarfile.TarError) as error:
            fail(f"cannot inspect tar archive {path}: {error}")
    elif path.suffix.lower() == ".zip":
        try:
            with zipfile.ZipFile(path) as archive:
                infos = [info for info in archive.infolist() if not info.is_dir()]
                names = [info.filename for info in infos]
                safe_member_names(names, path)
                for info in infos:
                    result[PurePosixPath(info.filename).as_posix().removeprefix("./")] = archive.read(info)
        except (OSError, zipfile.BadZipFile) as error:
            fail(f"cannot inspect zip archive {path}: {error}")
    else:
        fail(f"unsupported archive format: {path}")
    return result


def artifacts_of_type(artifacts: list[dict[str, Any]], artifact_type: str) -> list[dict[str, Any]]:
    return [artifact for artifact in artifacts if artifact.get("type") == artifact_type]


def one_artifact(
    artifacts: list[dict[str, Any]], artifact_type: str, dist: Path, repo: Path
) -> tuple[dict[str, Any], Path]:
    matches = artifacts_of_type(artifacts, artifact_type)
    if len(matches) != 1:
        fail(f"expected exactly one {artifact_type} artifact, found {len(matches)}")
    return matches[0], resolve_artifact(matches[0].get("path"), dist, repo)


def validate_archives(
    artifacts: list[dict[str, Any]], dist: Path, repo: Path
) -> tuple[dict[tuple[str, str], Path], dict[tuple[str, str], dict[str, bytes]]]:
    license_bytes = (repo / "LICENSE").read_bytes()
    readme_bytes = (repo / "README.md").read_bytes()
    binaries = artifacts_of_type(artifacts, "Binary")
    archives = artifacts_of_type(artifacts, "Archive")
    expected_targets = {
        (str(binary.get("goos")), str(binary.get("goarch")))
        for binary in binaries
        if binary.get("goos") in {"darwin", "linux", "windows"}
        and binary.get("goarch") in ARCHITECTURES
    }
    actual_targets = {(str(item.get("goos")), str(item.get("goarch"))) for item in archives}
    if expected_targets != actual_targets:
        fail(
            "archive target matrix differs from binary matrix: "
            f"expected={sorted(expected_targets)} actual={sorted(actual_targets)}"
        )
    if expected_targets != {
        (goos, goarch)
        for goos in ("darwin", "linux", "windows")
        for goarch in ("amd64", "arm64")
    }:
        fail(f"release target matrix is incomplete: {sorted(expected_targets)}")

    paths: dict[tuple[str, str], Path] = {}
    contents_by_target: dict[tuple[str, str], dict[str, bytes]] = {}
    for artifact in archives:
        goos = str(artifact.get("goos"))
        goarch = str(artifact.get("goarch"))
        target = (goos, goarch)
        path = resolve_artifact(artifact.get("path"), dist, repo)
        if goos == "windows" and path.suffix.lower() != ".zip":
            fail(f"Windows archive is not zip: {path.name}")
        if goos != "windows" and not path.name.endswith(".tar.gz"):
            fail(f"Unix archive is not tar.gz: {path.name}")
        contents = archive_contents(path)
        binary_name = "slipway.exe" if goos == "windows" else "slipway"
        required = {binary_name, "README.md", "LICENSE"}
        if not required.issubset(contents):
            fail(f"{path.name} lacks required members: {sorted(required - set(contents))}")
        if contents["LICENSE"] != license_bytes:
            fail(f"{path.name} LICENSE bytes differ from repository LICENSE")
        if contents["README.md"] != readme_bytes:
            fail(f"{path.name} README.md bytes differ from repository README.md")
        if not contents[binary_name]:
            fail(f"{path.name} contains an empty binary")
        paths[target] = path
        contents_by_target[target] = contents
    return paths, contents_by_target


def validate_scoop(
    artifacts: list[dict[str, Any]], archives: dict[tuple[str, str], Path], dist: Path, repo: Path
) -> Path:
    _, path = one_artifact(artifacts, "Scoop Manifest", dist, repo)
    manifest = load_json(path)
    if not isinstance(manifest, dict):
        fail("Scoop manifest must be a JSON object")
    if manifest.get("license") != EXPECTED_LICENSE:
        fail(f"Scoop license differs: {manifest.get('license')!r}")
    if manifest.get("homepage") != "https://github.com/signalridge/slipway":
        fail("Scoop homepage differs")
    architecture = manifest.get("architecture")
    if not isinstance(architecture, dict) or set(architecture) != set(ARCHITECTURES.values()):
        fail("Scoop architecture must contain exactly 64bit and arm64")
    for goarch, scoop_arch in ARCHITECTURES.items():
        entry = architecture.get(scoop_arch)
        if not isinstance(entry, dict):
            fail(f"Scoop architecture {scoop_arch} is not an object")
        archive = archives[("windows", goarch)]
        url = entry.get("url")
        if not isinstance(url, str) or not url.startswith(
            "https://github.com/signalridge/slipway/releases/download/"
        ) or not url.endswith(f"/{archive.name}"):
            fail(f"Scoop {scoop_arch} URL does not identify {archive.name}: {url!r}")
        if entry.get("hash") != sha256(archive):
            fail(f"Scoop {scoop_arch} hash differs from {archive.name}")
        if entry.get("bin") != ["slipway.exe"]:
            fail(f"Scoop {scoop_arch} bin must be slipway.exe")
    return path


def require_pkgbuild_line(text: str, pattern: str, message: str) -> None:
    if re.search(pattern, text, re.MULTILINE) is None:
        fail(message)


def validate_aur(
    artifacts: list[dict[str, Any]], archives: dict[tuple[str, str], Path], dist: Path, repo: Path
) -> Path:
    _, path = one_artifact(artifacts, "PKGBUILD", dist, repo)
    text = path.read_text(encoding="utf-8")
    require_pkgbuild_line(text, r"^pkgname='slipway-bin'$", "AUR pkgname differs")
    require_pkgbuild_line(text, r"^license=\('BSD-3-Clause'\)$", "AUR license differs")
    require_pkgbuild_line(
        text,
        r'install -Dm755 "\./slipway" "\$\{pkgdir\}/usr/bin/slipway"',
        "AUR binary install path differs",
    )
    require_pkgbuild_line(
        text,
        r'install -Dm644 "\./README\.md" "\$\{pkgdir\}/usr/share/doc/slipway/README\.md"',
        "AUR README install path is missing",
    )
    require_pkgbuild_line(
        text,
        r'install -Dm644 "\./LICENSE" "\$\{pkgdir\}/usr/share/licenses/slipway/LICENSE"',
        "AUR LICENSE install path is missing",
    )
    arch_names = {"amd64": "x86_64", "arm64": "aarch64"}
    for goarch, aur_arch in arch_names.items():
        archive = archives[("linux", goarch)]
        source_match = re.search(rf"^source_{aur_arch}=\(\"([^\"]+)\"\)$", text, re.MULTILINE)
        if source_match is None or not source_match.group(1).endswith(f"/{archive.name}"):
            fail(f"AUR {aur_arch} source does not identify {archive.name}")
        checksum_match = re.search(
            rf"^sha256sums_{aur_arch}=\('([0-9a-f]{{64}})'\)$", text, re.MULTILINE
        )
        if checksum_match is None or checksum_match.group(1) != sha256(archive):
            fail(f"AUR {aur_arch} checksum differs from {archive.name}")
    if "SKIP" in text:
        fail("AUR PKGBUILD must not skip checksums")
    result = subprocess.run(
        ["bash", "-n", str(path)], check=False, capture_output=True, text=True, timeout=30
    )
    if result.returncode != 0:
        fail(f"AUR PKGBUILD failed bash -n: {result.stderr.strip()}")

    srcinfo_matches = artifacts_of_type(artifacts, "SRCINFO")
    if len(srcinfo_matches) != 1:
        fail(f"expected exactly one SRCINFO artifact, found {len(srcinfo_matches)}")
    srcinfo = resolve_artifact(srcinfo_matches[0].get("path"), dist, repo).read_text(encoding="utf-8")
    if "\tlicense = BSD-3-Clause" not in srcinfo:
        fail("AUR .SRCINFO license differs")
    return path


def validate_linux_packages(artifacts: list[dict[str, Any]], dist: Path, repo: Path) -> None:
    packages = artifacts_of_type(artifacts, "Linux Package")
    targets: dict[tuple[str, str], set[str]] = {}
    required_destinations = {
        "/usr/bin/slipway",
        "/usr/share/doc/slipway/README.md",
        "/usr/share/licenses/slipway/LICENSE",
    }
    for artifact in packages:
        path = resolve_artifact(artifact.get("path"), dist, repo)
        if path.suffix not in EXPECTED_PACKAGE_FORMATS:
            fail(f"unexpected Linux package format: {path.name}")
        if path.stat().st_size == 0:
            fail(f"empty Linux package: {path.name}")
        target = (str(artifact.get("goos")), str(artifact.get("goarch")))
        targets.setdefault(target, set()).add(path.suffix)
        extra = artifact.get("extra")
        files = extra.get("Files") if isinstance(extra, dict) else None
        if not isinstance(files, list):
            fail(f"Linux package metadata lacks Files: {path.name}")
        destinations = {
            str(item.get("dst")) for item in files if isinstance(item, dict) and item.get("dst")
        }
        if not required_destinations.issubset(destinations):
            fail(
                f"{path.name} lacks package destinations: "
                f"{sorted(required_destinations - destinations)}"
            )
    expected = {("linux", "amd64"), ("linux", "arm64")}
    if set(targets) != expected:
        fail(f"Linux package target matrix differs: {sorted(targets)}")
    for target, formats in targets.items():
        if formats != EXPECTED_PACKAGE_FORMATS:
            fail(f"Linux package formats differ for {target}: {sorted(formats)}")


def native_linux_smoke(
    archives: dict[tuple[str, str], Path], contents: dict[tuple[str, str], dict[str, bytes]]
) -> bool:
    if platform.system() != "Linux":
        return False
    architecture = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(
        platform.machine().lower()
    )
    if architecture is None:
        return False
    target = ("linux", architecture)
    if target not in archives:
        fail(f"native Linux archive is missing for {architecture}")
    with tempfile.TemporaryDirectory(prefix="slipway-release-smoke-") as directory:
        binary = Path(directory) / "slipway"
        binary.write_bytes(contents[target]["slipway"])
        binary.chmod(binary.stat().st_mode | stat.S_IXUSR)
        for flag in ("--version", "--help"):
            result = subprocess.run(
                [str(binary), flag], check=False, capture_output=True, text=True, timeout=30
            )
            if result.returncode != 0:
                fail(
                    f"native Linux archive binary {flag} failed with {result.returncode}: "
                    f"{result.stderr.strip()}"
                )
            if "slipway" not in (result.stdout + result.stderr).lower():
                fail(f"native Linux archive binary {flag} output does not identify slipway")
    return True


def validate(
    dist: Path, repo: Path, expected_version: str | None, core_only: bool
) -> dict[str, Any]:
    if not dist.is_dir():
        fail(f"GoReleaser output directory is missing: {dist}")
    artifacts_path = dist / "artifacts.json"
    artifacts = load_json(artifacts_path)
    if not isinstance(artifacts, list) or not artifacts:
        fail("artifacts.json must be a non-empty array")
    if any(not isinstance(item, dict) for item in artifacts):
        fail("artifacts.json entries must be objects")

    metadata_path = dist / "metadata.json"
    metadata = load_json(metadata_path)
    if not isinstance(metadata, dict) or not isinstance(metadata.get("version"), str):
        fail("metadata.json version is missing")
    version = metadata["version"]
    if expected_version and version != expected_version.removeprefix("v"):
        fail(f"GoReleaser version differs: expected {expected_version!r}, got {version!r}")

    archives, contents = validate_archives(artifacts, dist, repo)
    scoop: Path | None = None
    aur: Path | None = None
    optional_types = {"Homebrew Cask", "Scoop Manifest", "PKGBUILD", "SRCINFO"}
    if core_only:
        unexpected = sorted(
            str(artifact.get("type"))
            for artifact in artifacts
            if artifact.get("type") in optional_types
        )
        if unexpected:
            fail(f"core release unexpectedly contains optional publisher artifacts: {unexpected}")
    else:
        scoop = validate_scoop(artifacts, archives, dist, repo)
        aur = validate_aur(artifacts, archives, dist, repo)
    validate_linux_packages(artifacts, dist, repo)
    native_smoke = native_linux_smoke(archives, contents)
    return {
        "archive_count": len(archives),
        "aur": str(aur.relative_to(repo) if aur and aur.is_relative_to(repo) else aur) if aur else None,
        "license_sha256": sha256(repo / "LICENSE"),
        "native_linux_smoke": native_smoke,
        "scoop": str(scoop.relative_to(repo) if scoop and scoop.is_relative_to(repo) else scoop)
        if scoop
        else None,
        "version": version,
    }


def main() -> int:
    default_repo = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=default_repo)
    parser.add_argument("--dist", type=Path, default=Path("dist"))
    parser.add_argument("--expected-version")
    parser.add_argument(
        "--core-only",
        action="store_true",
        help="validate core archives and Linux packages when optional publisher artifacts were skipped",
    )
    args = parser.parse_args()
    repo = args.repo_root.resolve()
    dist = args.dist if args.dist.is_absolute() else repo / args.dist
    try:
        summary = validate(dist.resolve(), repo, args.expected_version, args.core_only)
    except (CheckFailure, OSError, UnicodeError, subprocess.SubprocessError) as error:
        print(f"release artifact smoke failed: {error}", file=sys.stderr)
        return 1
    json.dump(summary, sys.stdout, ensure_ascii=False, sort_keys=True)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
