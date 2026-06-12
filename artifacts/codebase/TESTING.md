# Testing

Re-authored for change
`resolve-github-issue-184-add-gsd-style-automatic-subagent-di`
(GitHub issue #184).

## Existing Coverage

- `internal/toolgen/toolgen_test.go:1069` through
  `internal/toolgen/toolgen_test.go:1094` asserts generated
  `slipway-wave-orchestration` text includes parallel-by-default dispatch,
  `parallel: true`, degradation visibility, `parallelization: off`,
  post-wave integration gate, and structured dispatch references.
- `internal/tmpl/thin_host_content_test.go:56` through
  `internal/tmpl/thin_host_content_test.go:92` renders
  `wave-orchestration/SKILL.md.tmpl` and reads the executor dispatch reference.
  It already checks that codebase-map content is passed by path rather than
  inlined into the coordinator context.
- `internal/tmpl/wave_isolation_content_test.go:11` through
  `internal/tmpl/wave_isolation_content_test.go:27` checks rendered
  wave-orchestration dispatch contract text around test-authoring isolation.

## Gaps For Issue #184

- No current test requires Codex guidance to use `spawn_agent`.
- No current test rejects `codex -q --task` as the primary Codex fan-out path.
- No current test requires spawn/wait/collect/close semantics, `fork_context:
  false`, coordinator stop-work wording, or the full executor result field set.
- Existing degradation assertions require visibility, but do not distinguish a
  genuinely incapable runtime from a capable runtime that failed to dispatch.

## Verification Plan

- Add focused content tests under `internal/tmpl` for the reference and rendered
  host contract.
- Expand `internal/toolgen/toolgen_test.go` so generated adapter output carries
  the new contract, not just source templates.
- Run `go test -count=1 ./internal/tmpl ./internal/toolgen` after template and
  test edits.
- Run `go test -count=1 ./...`, `git diff --check`, and
  `go run . validate --json` before claiming done-ready.
