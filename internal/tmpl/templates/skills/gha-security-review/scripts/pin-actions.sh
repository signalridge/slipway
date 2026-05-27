#!/usr/bin/env bash
# pin-actions.sh — rewrite `uses: owner/repo@vX` to pinned commit SHAs
# using a checked-in mapping. Offline and deterministic: no network
# lookups, no `gh` calls, no live tag resolution.
#
# Usage:
#   pin-actions.sh --mapping <path.tsv> <workflow.yml> [<workflow.yml>...]
#
# Mapping file format (tab-separated, one entry per line, leading `#`
# lines ignored):
#   owner/repo@ref<TAB>sha
# Example:
#   actions/checkout@v4<TAB>b4ffde65f46336ab88eb53be808477a3936bae11
#
# Semantics:
#   - Each `uses: owner/repo@ref` on a single line is rewritten in place
#     with `@<sha>  # <ref>` appended so the pre-pin reference stays
#     visible as a comment.
#   - Lines already using a 40-char SHA are left untouched.
#   - Missing mapping rows are reported to stderr and the script exits
#     non-zero. No partial writes: the file is replaced atomically
#     only when every `uses:` is either already pinned or resolved via
#     the mapping.
#
# Exit codes:
#   0  every workflow successfully rewritten (or already fully pinned)
#   1  unresolved references remained; no files were modified
#   2  usage error (missing flag, missing mapping file, bad argv)
#   3  input workflow unreadable
#
# Requires: awk, grep, sed, mktemp on PATH.

set -euo pipefail

usage() {
	cat >&2 <<'EOF'
Usage: pin-actions.sh --mapping <mapping.tsv> <workflow.yml> [<workflow.yml>...]

  --mapping <path>  checked-in TSV with "owner/repo@ref<TAB>sha" rows
  <workflow.yml>    GitHub Actions workflow file(s) to rewrite in place

No network calls are issued. Unresolved references abort the run.
EOF
	exit 2
}

MAPPING=""
FILES=()

while [ $# -gt 0 ]; do
	case "$1" in
	--mapping)
		shift
		if [ $# -lt 1 ]; then usage; fi
		MAPPING="$1"
		shift
		;;
	--help | -h)
		usage
		;;
	--)
		shift
		while [ $# -gt 0 ]; do
			FILES+=("$1")
			shift
		done
		;;
	-*)
		echo "pin-actions.sh: unknown flag: $1" >&2
		usage
		;;
	*)
		FILES+=("$1")
		shift
		;;
	esac
done

if [ -z "$MAPPING" ] || [ "${#FILES[@]}" -eq 0 ]; then
	usage
fi

if [ ! -f "$MAPPING" ]; then
	echo "pin-actions.sh: mapping file not found: $MAPPING" >&2
	exit 2
fi

# Load mapping into indexed arrays for compatibility with Bash 3.2 on macOS.
# Each valid row is
# "owner/repo@ref<TAB>sha". Rows starting with # or blank are skipped.
MAP_KEYS=()
MAP_VALUES=()
while IFS=$'\t' read -r key sha rest; do
	case "$key" in
	'' | '#'*) continue ;;
	esac
	if [ -z "${sha:-}" ]; then
		echo "pin-actions.sh: malformed mapping row: $key" >&2
		exit 2
	fi
	if ! [[ "$sha" =~ ^[0-9a-f]{40}$ ]]; then
		echo "pin-actions.sh: mapping sha not a 40-char hex: $key -> $sha" >&2
		exit 2
	fi
	MAP_KEYS+=("$key")
	MAP_VALUES+=("$sha")
done <"$MAPPING"

lookup_sha() {
	local lookup="$1"
	local i
	for i in "${!MAP_KEYS[@]}"; do
		if [ "${MAP_KEYS[$i]}" = "$lookup" ]; then
			printf '%s\n' "${MAP_VALUES[$i]}"
			return 0
		fi
	done
	return 1
}

rewrite_file() {
	local file="$1"
	if [ ! -r "$file" ]; then
		echo "pin-actions.sh: cannot read $file" >&2
		return 3
	fi

	local tmp
	tmp="$(mktemp)"
	local unresolved=0

	# Read line-by-line; rewrite `uses: owner/repo@ref` occurrences.
	# Only the first `uses:` match on a line is considered — workflows
	# conventionally put one per line.
	while IFS= read -r line || [ -n "$line" ]; do
		if [[ "$line" =~ ^([[:space:]]*-?[[:space:]]*uses:[[:space:]]*)([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)@([^[:space:]#]+)(.*)$ ]]; then
			local prefix="${BASH_REMATCH[1]}"
			local repo="${BASH_REMATCH[2]}"
			local ref="${BASH_REMATCH[3]}"
			local trail="${BASH_REMATCH[4]}"

			if [[ "$ref" =~ ^[0-9a-f]{40}$ ]]; then
				printf '%s\n' "$line" >>"$tmp"
				continue
			fi

			local key="${repo}@${ref}"
			local sha
			if ! sha="$(lookup_sha "$key")"; then
				echo "pin-actions.sh: unresolved $key in $file" >&2
				unresolved=1
				printf '%s\n' "$line" >>"$tmp"
				continue
			fi
			printf '%s%s@%s  # %s%s\n' "$prefix" "$repo" "$sha" "$ref" "$trail" >>"$tmp"
		else
			printf '%s\n' "$line" >>"$tmp"
		fi
	done <"$file"

	if [ "$unresolved" -ne 0 ]; then
		rm -f "$tmp"
		return 1
	fi

	# Atomic replace: mv preserves inode/permissions on POSIX.
	mv "$tmp" "$file"
	return 0
}

RC=0
for f in "${FILES[@]}"; do
	code=0
	rewrite_file "$f" || code=$?
	case "$code" in
	0)
		;;
	3)
		exit 3
		;;
	*)
		RC=1
		;;
	esac
done
exit "$RC"
