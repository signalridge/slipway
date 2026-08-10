#!/usr/bin/env python3
"""Black-box tests for the optional release checksum verifier."""

from __future__ import annotations

from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "verify_optional_release_checksums.py"
TARGETS = (
    ("darwin", "amd64", "tar.gz"),
    ("darwin", "arm64", "tar.gz"),
    ("linux", "amd64", "tar.gz"),
    ("linux", "arm64", "tar.gz"),
    ("windows", "amd64", "zip"),
    ("windows", "arm64", "zip"),
)


def checksum_lines(version: str, digest: str = "0" * 64) -> list[str]:
    return [
        f"{digest}  slipway_{version}_{goos}_{goarch}.{extension}\n"
        for goos, goarch, extension in TARGETS
    ]


class OptionalReleaseChecksumVerifierTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.directory = Path(self.temporary_directory.name)
        self.core = self.directory / "core.txt"
        self.generated = self.directory / "generated.txt"
        lines = checksum_lines("0.42.0")
        self.core.write_text("".join(lines), encoding="utf-8")
        self.generated.write_text("".join(lines), encoding="utf-8")

    def run_verifier(self, tag: str = "v0.42.0") -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                "-I",
                str(SCRIPT),
                "--tag",
                tag,
                "--generated",
                str(self.generated),
                "--core-checksums",
                str(self.core),
            ],
            check=False,
            capture_output=True,
            text=True,
            timeout=30,
        )

    def test_accepts_exact_archive_matrix_with_matching_core_bytes(self) -> None:
        result = self.run_verifier()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("verified 6 rebuilt artifact checksums", result.stdout)

    def test_rejects_incomplete_archive_matrix(self) -> None:
        self.generated.write_text("".join(checksum_lines("0.42.0")[:-1]), encoding="utf-8")
        result = self.run_verifier()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("rebuilt archive matrix differs", result.stderr)

    def test_rejects_bytes_that_differ_from_core_release(self) -> None:
        self.generated.write_text("".join(checksum_lines("0.42.0", "1" * 64)), encoding="utf-8")
        result = self.run_verifier()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("differs from the core release", result.stderr)

    def test_rejects_container_incompatible_build_metadata_tag(self) -> None:
        result = self.run_verifier("v0.42.0+build.1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("invalid release tag", result.stderr)


if __name__ == "__main__":
    unittest.main()
