#!/usr/bin/env bash
# find-polluter-go.sh — bisect a Go test suite to locate the test that
# mutates shared on-disk state (the "polluter").
#
# Slipway-specific helper for Go's `go test` invocation. It stays offline and
# deterministic while reporting the first package that creates the pollution
# path.
#
# Usage:
#   find-polluter-go.sh <pollution-path> <package-glob> [-run <regex>]
#
# Example:
#   find-polluter-go.sh ./.cache.tmp ./internal/... -run '^TestIntegration'
#
# The script enumerates test binaries under <package-glob>, sorts them
# deterministically, and runs each with `go test -count=1 -run <regex>`.
# If <pollution-path> appears during a run, the offending package is
# printed and the script exits non-zero. No network calls are made.

set -euo pipefail

usage() {
	cat >&2 <<'EOF'
Usage: find-polluter-go.sh <pollution-path> <package-glob> [-run <regex>]

  <pollution-path>  file or directory whose appearance signals pollution
  <package-glob>    Go package selector (e.g. ./... or ./internal/state/...)
  -run <regex>      optional -run filter forwarded to `go test`

Requires: go (1.18+) on PATH.
EOF
	exit 2
}

if [ $# -lt 2 ]; then
	usage
fi

POLLUTION_PATH="$1"
PKG_GLOB="$2"
shift 2

RUN_FILTER=""
while [ $# -gt 0 ]; do
	case "$1" in
	-run)
		shift
		if [ $# -lt 1 ]; then usage; fi
		RUN_FILTER="$1"
		shift
		;;
	*)
		echo "unknown option: $1" >&2
		usage
		;;
	esac
done

if ! command -v go >/dev/null 2>&1; then
	echo "find-polluter-go.sh: go toolchain not found on PATH" >&2
	exit 127
fi

if [ -e "$POLLUTION_PATH" ]; then
	echo "find-polluter-go.sh: pollution already present at $POLLUTION_PATH; clean before bisecting" >&2
	exit 1
fi

# Enumerate candidate packages. `go list` keeps output deterministic and
# includes packages with either in-package tests or external `_test` packages.
LIST_OUTPUT=""
LIST_STDERR_FILE="$(mktemp "${TMPDIR:-/tmp}/find-polluter-go-list-stderr.XXXXXX")"
trap 'rm -f "$LIST_STDERR_FILE"' EXIT

if LIST_OUTPUT="$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' "$PKG_GLOB" 2>"$LIST_STDERR_FILE")"; then
	LIST_STATUS=0
else
	LIST_STATUS=$?
fi
LIST_STDERR="$(cat "$LIST_STDERR_FILE")"

if [ "$LIST_STATUS" -ne 0 ]; then
	echo "find-polluter-go.sh: go list failed for $PKG_GLOB" >&2
	if [ -n "$LIST_STDERR" ]; then
		printf '%s\n' "$LIST_STDERR" >&2
	fi
	if [ -n "$LIST_OUTPUT" ]; then
		printf '%s\n' "$LIST_OUTPUT" >&2
	fi
	exit 1
fi

if [ -n "$LIST_STDERR" ]; then
	printf '%s\n' "$LIST_STDERR" >&2
fi

PKGS=()
while IFS= read -r pkg; do
	PKGS+=("$pkg")
done < <(printf '%s\n' "$LIST_OUTPUT" | sed '/^[[:space:]]*$/d' | sort)

if [ "${#PKGS[@]}" -eq 0 ]; then
	echo "find-polluter-go.sh: no test packages found under $PKG_GLOB" >&2
	exit 1
fi

echo "find-polluter-go.sh: checking ${#PKGS[@]} package(s) for polluter $POLLUTION_PATH"

RUN_ARGS=("-count=1")
if [ -n "$RUN_FILTER" ]; then
	RUN_ARGS+=("-run" "$RUN_FILTER")
fi

for PKG in "${PKGS[@]}"; do
	echo "-- go test $PKG ${RUN_ARGS[*]}"
	# We intentionally discard test output: the signal is whether the
	# pollution path exists afterwards, not the pass/fail of the test.
	go test "${RUN_ARGS[@]}" "$PKG" >/dev/null 2>&1 || true

	if [ -e "$POLLUTION_PATH" ]; then
		echo ""
		echo "POLLUTER: $PKG created $POLLUTION_PATH"
		echo "Reproduce with: go test -count=1 ${RUN_FILTER:+-run $RUN_FILTER }-v $PKG"
		exit 1
	fi
done

echo "find-polluter-go.sh: no polluter found across ${#PKGS[@]} package(s)"
exit 0
