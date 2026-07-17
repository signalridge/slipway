#!/usr/bin/env python3
"""Fail optional publishers unless rebuilt archives match the core release assets."""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import subprocess
import sys
import tempfile


TAG_PATTERN = re.compile(
    r"^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
    r"(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?$"
)
CHECKSUM_PATTERN = re.compile(r"^([0-9a-f]{64})\s+(.+)$")
ARCHIVE_TARGETS = (
    ("darwin", "amd64", "tar.gz"),
    ("darwin", "arm64", "tar.gz"),
    ("linux", "amd64", "tar.gz"),
    ("linux", "arm64", "tar.gz"),
    ("windows", "amd64", "zip"),
    ("windows", "arm64", "zip"),
)


class VerificationError(RuntimeError):
    pass


def load_checksums(path: Path) -> dict[str, str]:
    checksums: dict[str, str] = {}
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except (OSError, UnicodeError) as error:
        raise VerificationError(f"cannot read checksum file {path}: {error}") from error
    for line_number, line in enumerate(lines, 1):
        match = CHECKSUM_PATTERN.fullmatch(line)
        if match is None:
            raise VerificationError(f"malformed checksum line {line_number} in {path}")
        digest, name = match.groups()
        if not name or Path(name).name != name:
            raise VerificationError(f"unsafe artifact name on line {line_number} in {path}")
        if name in checksums:
            raise VerificationError(f"duplicate artifact {name!r} in {path}")
        checksums[name] = digest
    if not checksums:
        raise VerificationError(f"checksum file is empty: {path}")
    return checksums


def download_core_checksums(tag: str, destination: Path) -> None:
    result = subprocess.run(
        [
            "gh",
            "release",
            "download",
            tag,
            "--repo",
            "signalridge/slipway",
            "--pattern",
            "checksums.txt",
            "--output",
            str(destination),
        ],
        check=False,
        capture_output=True,
        text=True,
        timeout=120,
    )
    if result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip() or f"exit {result.returncode}"
        raise VerificationError(f"cannot download core checksums for {tag}: {detail}")


def expected_archive_names(tag: str) -> set[str]:
    version = tag.removeprefix("v")
    return {
        f"slipway_{version}_{goos}_{goarch}.{extension}"
        for goos, goarch, extension in ARCHIVE_TARGETS
    }


def verify(generated_path: Path, core_path: Path, tag: str) -> int:
    generated = load_checksums(generated_path)
    core = load_checksums(core_path)
    expected = expected_archive_names(tag)
    actual = set(generated)
    if actual != expected:
        missing = ", ".join(sorted(expected - actual)) or "none"
        unexpected = ", ".join(sorted(actual - expected)) or "none"
        raise VerificationError(
            f"rebuilt archive matrix differs: missing [{missing}]; unexpected [{unexpected}]"
        )
    for name in sorted(expected):
        digest = generated[name]
        core_digest = core.get(name)
        if core_digest is None:
            raise VerificationError(f"core release does not contain rebuilt artifact {name!r}")
        if core_digest != digest:
            raise VerificationError(
                f"rebuilt artifact {name!r} differs from the core release; refusing optional publication"
            )
    return len(generated)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--generated", type=Path, required=True)
    parser.add_argument("--core-checksums", type=Path, help=argparse.SUPPRESS)
    args = parser.parse_args()

    if TAG_PATTERN.fullmatch(args.tag) is None:
        print(
            f"optional publisher checksum verification failed: invalid release tag {args.tag!r}",
            file=sys.stderr,
        )
        return 1

    try:
        if args.core_checksums is not None:
            count = verify(args.generated, args.core_checksums, args.tag)
        else:
            with tempfile.TemporaryDirectory(prefix="slipway-release-checksums-") as directory:
                core_path = Path(directory) / "checksums.txt"
                download_core_checksums(args.tag, core_path)
                count = verify(args.generated, core_path, args.tag)
    except (VerificationError, OSError, subprocess.SubprocessError) as error:
        print(f"optional publisher checksum verification failed: {error}", file=sys.stderr)
        return 1

    print(f"verified {count} rebuilt artifact checksums against core release {args.tag}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
