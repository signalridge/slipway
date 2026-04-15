#!/usr/bin/env bash
# merge-sarif.sh — deterministic SARIF aggregator for SAST orchestration.
#
# Usage:
#   merge-sarif.sh RAW_DIR OUTPUT_FILE
#
# Reads every `*.sarif` file directly under RAW_DIR, merges runs, and writes
# a consolidated SARIF 2.1.0 document at OUTPUT_FILE. Results are deduplicated
# by `(ruleId, artifact uri, startLine)` and rule metadata is merged by rule
# id, preserving first-seen driver tool metadata.
#
# Narrow lift from `trailofbits/semgrep/scripts/merge_sarif.py`, implemented
# as shell + jq so the shipped helper stays offline and deterministic without
# introducing Python runtime caches into the rendered tree.

set -euo pipefail

usage() {
	cat >&2 <<'EOF'
Usage: merge-sarif.sh RAW_DIR OUTPUT_FILE

  RAW_DIR      directory containing one or more *.sarif files
  OUTPUT_FILE  merged SARIF output path

Requires: jq on PATH.
EOF
	exit 2
}

if [ $# -ne 2 ]; then
	usage
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "merge-sarif.sh: jq not found on PATH" >&2
	exit 127
fi

RAW_DIR="$1"
OUTPUT_FILE="$2"

if [ ! -d "$RAW_DIR" ]; then
	echo "Error: $RAW_DIR is not a directory" >&2
	exit 1
fi

mapfile -t SARIF_FILES < <(find "$RAW_DIR" -maxdepth 1 -type f -name '*.sarif' | sort)
if [ "${#SARIF_FILES[@]}" -eq 0 ]; then
	echo "Error: no SARIF files found in $RAW_DIR" >&2
	exit 1
fi

VALID_FILES=()
SKIPPED_FILES=()
for f in "${SARIF_FILES[@]}"; do
	if jq -e . "$f" >/dev/null 2>&1; then
		VALID_FILES+=("$f")
	else
		SKIPPED_FILES+=("$f")
	fi
done

mkdir -p "$(dirname "$OUTPUT_FILE")"

if [ "${#SKIPPED_FILES[@]}" -gt 0 ]; then
	echo "WARNING: ${#SKIPPED_FILES[@]} of ${#SARIF_FILES[@]} SARIF files could not be parsed; results may be incomplete." >&2
	for sf in "${SKIPPED_FILES[@]}"; do
		echo "  Skipped: $sf" >&2
	done
fi

if [ "${#VALID_FILES[@]}" -eq 0 ]; then
	printf '{\n  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",\n  "runs": [],\n  "version": "2.1.0"\n}\n' >"$OUTPUT_FILE"
	echo "Merged 0 SARIF file(s) -> $OUTPUT_FILE (0 result(s))"
	exit 0
fi

jq -S -s '
  . as $docs
  | def run_stream($docs): $docs[] | .runs[]?;
    def first_tool($docs):
      first(run_stream($docs) | select(.tool != null) | .tool)
      // {"driver": {"name": "merge-sarif", "rules": []}};
    def merged_rules($docs):
      reduce (run_stream($docs) | .tool.driver.rules[]? | select(.id? != null and .id != "")) as $r
        ({seen: {}, out: []};
         if .seen[$r.id] then .
         else .seen[$r.id] = true | .out += [$r]
         end)
      | .out;
    def result_key($r):
      [($r.ruleId // ""),
       ($r.locations[0]?.physicalLocation?.artifactLocation?.uri // ""),
       ($r.locations[0]?.physicalLocation?.region?.startLine // 0)]
      | @json;
    def merged_results($docs):
      reduce (run_stream($docs) | .results[]?) as $r
        ({seen: {}, out: []};
         (result_key($r)) as $key
         | if .seen[$key] then .
           else .seen[$key] = true | .out += [$r]
           end)
      | .out;
    merged_results($docs) as $results
    | {
        "version": "2.1.0",
        "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
        "runs":
          (if ($results | length) == 0 then
             []
           else
             [
               {
                 "tool": first_tool($docs),
                 "results": $results
               }
               | .tool.driver = ((.tool.driver // {"name": "merge-sarif"}) | .rules = merged_rules($docs))
             ]
           end)
      }
' "${VALID_FILES[@]}" >"$OUTPUT_FILE"

RESULT_COUNT="$(jq '[.runs[]?.results[]?] | length' "$OUTPUT_FILE")"
echo "Merged ${#VALID_FILES[@]} SARIF file(s) -> $OUTPUT_FILE (${RESULT_COUNT} result(s))"
