# Structure

Re-authored for change
`resolve-github-issue-155-knuth-invariant-overwrite-only-own`
(GitHub issue #155).

- `internal/engine/progression/`
  - `evidence_digests.go`: skill input digest construction, named stale input
    blockers, and the prose artifact digest seam for this change.
  - `evidence_digests_test.go`: existing digest policy tests and the target for
    new prose materiality regression coverage.
  - `stale_evidence_recovery.go`: consumes stale digest blockers to reopen the
    earliest affected lifecycle authority; not expected to change for issue
    #155.
- `internal/engine/artifact/`
  - `manager.go`: artifact schemas, embedded template access, scaffold/deferred
    artifact behavior, and existing template-derived scaffold detection pattern.
  - `requirements.go`: narrow requirements placeholder detection, useful as a
    warning against broad substring-based materiality suppression.
  - `schemas.yaml`: expanded artifact graph for `intent.md`,
    `requirements.md`, `decision.md`, `tasks.md`, `assurance.md`, and
    discovery-only `research.md`.
- `internal/tmpl/templates/artifacts/`
  - `intent.md`, `requirements.md`, `research.md`, and `decision.md`: embedded
    artifact templates whose comments and scaffold-only sections define
    engine-owned prose defaults.
- Local reference outside this repo:
  `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/state-document.cts`.
