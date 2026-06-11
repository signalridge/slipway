# Concerns

Re-authored for change `resolve-github-issue-164-implement-transactional-multi-file`
(GitHub issue #164).

- Load-bearing invariant: a governed lifecycle transition must not report
  success unless its artifact/evidence side effects and authoritative
  `change.yaml` state agree.
- Partial scaffold risk: `advance_governed.go` can scaffold bundle artifacts
  before saving `change.yaml`. If the save fails after a file is created, the
  bundle can look partially materialized while the lifecycle authority remains
  behind.
- Partial stale-reopen risk: `stale_evidence_recovery.go` removes verification,
  wave-plan, and execution-summary files before saving reopened state. A save
  failure can otherwise remove evidence while the lifecycle still points past
  the reopened stage.
- Partial S2-entry risk: `wave_execution.go` writes `wave-plan.yaml` before the
  transition to S2 is saved. A later save failure can otherwise leave a generated
  plan that belongs to a transition that did not complete.
- Rollback risk: rollback can fail too. Error reporting must include the
  original failure and the file path requiring inspection, and irreversible
  operations must fail closed without a force-pass path.
- Scope risk: directory archive/relocation logic is tempting to fold into the
  same abstraction, but the current issue and GSD reference are file-set
  write/delete rollback. Keep directory movement out unless a failing regression
  proves it belongs.
- Durability boundary: the helper should compose with `fsutil.WriteFileAtomic`
  for writes. It should not replace atomic single-file persistence with direct
  best-effort writes.
- Test reliability risk: chmod-based failures are platform-sensitive. Prefer a
  deterministic failure seam in the transaction helper for cross-platform
  regression tests.
