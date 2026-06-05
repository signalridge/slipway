# Tasks
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Engine digest changes in evidence_digests.go: unify the
  `digestInputsChangedAfterVerdict` safety check across both the read path
  (`skillDigestFreshnessBlockersWithSummary`) and the stamp path
  (`stampPassingSkillDigests`) so a recorded orphan with inputs unchanged after
  the verdict is backfilled and genuine drift reports specific stale inputs;
  remove `assurance.md` from `addPlanningArtifactInputs`; add exported
  `RestampEvidenceDigestTier0`. Convert the deadlock/assurance-coupling tests to
  the new contract (test-first) and add the late-assurance-edit characterization
  test.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/evidence_digests.go", "internal/engine/progression/evidence_digests_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-006]

- [x] `t-02` Prune digests on stale-planning recovery in advance_governed.go: add
  a helper that removes a verification record and its `evidence-digests.yaml`
  entry together, and use it in `beginStalePlanningRecovery` for the five cleared
  skills while preserving wave-orchestration. Extend the recovery integration
  test with digest-prune assertions.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/advance_governed.go", "cmd/lifecycle_commands_test.go"]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-03` Surface stale in cmd/next_skill_view.go: add `required_skill_stale`
  to `isRequiredSkillBlocker`, and report `stale` for digest drift on both the
  precomputed and non-precomputed evidence paths (derived from the
  `required_skill_stale` entries in `view.Blockers`). Add view tests.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/next_skill_view.go", "cmd/progression_next_test.go"]
  - task_kind: code
  - covers: [REQ-004, REQ-005]

- [x] `t-04` Route repair planning/digest drift in cmd/repair.go: make
  `repairDriftNextAction` (and the digest-drift finding) point at
  `slipway evidence restamp --dry-run`, stating repair does not mutate
  engine-owned digests. Extend repair tests.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go", "cmd/repair_test.go"]
  - task_kind: code
  - covers: [REQ-007]

- [x] `t-05` Add the `slipway evidence restamp --skill X [--dry-run]` subcommand
  in cmd/evidence.go over `RestampEvidenceDigestTier0`, registered on
  `makeEvidenceCmd`; refuse with the skill to re-run when not Tier-0 eligible,
  including dry-run cases where digest inputs are unavailable. Add command tests.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/evidence.go", "cmd/evidence_test.go"]
  - task_kind: code
  - covers: [REQ-006]
