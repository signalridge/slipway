# Tasks

## Task List

- [x] `t-01` Add a `repairDriftNextAction` case (`cmd/repair.go`) for tasks.md parse-failure drift (unknown/unsupported metadata key and wave-plan derivation/load failure) that routes to "edit tasks.md to fix/remove the unsupported metadata key, then re-run `slipway repair` / `slipway validate`" instead of the generic default, and align the sibling `wave_plan_load_failed` remediation wording in `cmd/common.go` for this drift class as one product surface (only where it diverges; keep run's early interception behavior).
  - depends_on: []
  - target_files: ["cmd/repair.go", "cmd/common.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Add a regression test (`cmd/issue275_repair_guidance_test.go`) that seeds an S2 change whose tasks.md carries an unknown metadata key, then asserts (a) `slipway repair --json` `unrepaired_drift[].next_action` for that parse failure routes to fixing tasks.md and contains no "run `slipway run`", and (b) `repair` / `validate` / `run` / `next` stay mutually consistent (all point to fixing tasks.md) for this drift.
  - depends_on: ["t-01"]
  - target_files: ["cmd/issue275_repair_guidance_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-003]
