# Testing

Re-authored for change `resolve-github-issue-164-implement-transactional-multi-file`
(GitHub issue #164).

- Existing coverage:
  - `internal/fsutil/atomic.go:14` through `internal/fsutil/atomic.go:73`
    implements the single-file atomic write primitive but does not test
    multi-file all-or-nothing semantics.
  - `cmd/preset_test.go:315` through `cmd/preset_test.go:340` covers a
    preset-command scaffold failure rollback, but that is command-specific and
    not a reusable governed stage-transition transaction.
  - `internal/state/lifecycle_test.go:396` through
    `internal/state/lifecycle_test.go:430` covers archive rollback when
    persisting archived authority fails, but that path is directory promotion,
    not ordered file writes/removes inside an active stage transition.
- Gaps for issue #164:
  - No package-level test proves an ordered file transaction restores original
    bytes and removes newly-created files when a later operation fails.
  - No test proves rollback-failure errors include the path requiring
    inspection.
  - No progression regression simulates failure after governed bundle scaffold
    writes but before `change.yaml` persistence.
  - No progression regression simulates failure after stale evidence deletion
    but before reopened `change.yaml` persistence.
  - No test proves S1-to-S2 `wave-plan.yaml` materialization is part of the same
    file-set boundary as transition state persistence.
- Planned verification:
  - Add `internal/fsutil` table-driven tests for write/write, remove/write, and
    rollback-failure paths using deterministic injected failures.
  - Add progression or artifact tests that trigger S1 bundle scaffold under an
    injected later write failure and assert no scaffold-owned partial file
    remains.
  - Add stale-evidence recovery tests that delete verification evidence under an
    injected later save failure and assert removed files and digest state are
    restored.
  - Add wave-plan transition coverage or state-level coverage proving a failed
    state save does not leave a visible `wave-plan.yaml` from the failed S2
    transition.
  - Run targeted package tests first, then `go run . validate --json` and the
    governed evidence commands selected by the current lifecycle.
