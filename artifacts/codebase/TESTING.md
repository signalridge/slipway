# Testing

Re-authored for change
`resolve-github-issue-151-thin-host-disk-handoff-return-contr`
(GitHub issue #151).

- Existing focused coverage:
  - `internal/tmpl/thin_host_content_test.go:11` verifies
    goal-verification thin-host delegation and host-owned verdict language.
  - `internal/tmpl/thin_host_content_test.go:37` verifies worktree-preflight
    keeps only bounded baseline summaries.
  - `internal/tmpl/thin_host_content_test.go:56` verifies wave-orchestration
    delegates source/map reads by path to task executors.
  - `internal/tmpl/templates_test.go:17` verifies governance skills render and
    keep required host sections.
  - `internal/tmpl/templates_test.go:781` verifies run-version source language
    for review and verification hosts.
- Gap for issue #151:
  - No focused regression currently asserts disk-handoff guidance for the
    remaining heavy hosts named by the issue:
    `research-orchestration`, `plan-audit`, `intake-clarification`,
    `spec-compliance-review`, and `code-quality-review`.
  - No focused regression currently asserts that a short subagent confirmation
    is a claim, not evidence, across those remaining hosts.
- Planned verification:
  - Add a failing test that renders/loads the five remaining heavy host
    surfaces and asserts path-based disk handoff, short confirmation, and
    CLI-owned stamping/freshness language.
  - Run the targeted test first, then `go test -count=1 ./internal/tmpl`, and
    finish with `go test -count=1 ./...` before closeout.
