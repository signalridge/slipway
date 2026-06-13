# Testing

Re-authored for change
`resolve-github-issue-155-knuth-invariant-overwrite-only-own`
(GitHub issue #155).

## Existing Coverage

- `internal/engine/progression/evidence_digests_test.go:18` through
  `internal/engine/progression/evidence_digests_test.go:57` already pins the
  plan-audit digest policy: assurance is excluded, CRLF-only prose rewrites do
  not stale, checkbox-only task edits do not stale, `target_files`-only task
  edits do not stale, and objective changes do stale.
- `internal/engine/progression/evidence_digests_test.go:136` through
  `internal/engine/progression/evidence_digests_test.go:167` proves stale
  evidence is content-digest based rather than mtime based.
- `internal/engine/progression/evidence_digests_test.go:779` through
  `internal/engine/progression/evidence_digests_test.go:807` proves
  `research-orchestration` stales when `research.md` material content changes.
- `internal/engine/artifact/requirements.go:35` through
  `internal/engine/artifact/requirements.go:103` contains narrow requirements
  placeholder detection, useful as a fail-closed warning against overbroad
  scaffold classification.
- `internal/engine/artifact/manager.go:559` through
  `internal/engine/artifact/manager.go:640` derives assurance scaffold
  detection from the embedded template, establishing a local pattern for
  template-derived scaffold checks.

## Gaps For Issue #155

- No test proves comment-only or scaffold-only prose artifact edits are
  non-material for input digests.
- No test proves human-authored prose edits still stale `plan-audit` and
  `research-orchestration`.
- No test covers the fail-closed unknown case: unknown non-empty prose must be
  included in the material view rather than silently ignored.

## Verification Plan

- Add focused unit tests in `internal/engine/progression/evidence_digests_test.go`.
- Cover `intent.md`, `requirements.md`, `research.md`, and `decision.md` through
  the plan-audit digest path.
- Include a `research-orchestration` case so the research artifact digest still
  reacts to authored research content.
- Run `go test -count=1 ./internal/engine/progression` after implementation,
  then broaden to `go test -count=1 ./...` before done-ready.
