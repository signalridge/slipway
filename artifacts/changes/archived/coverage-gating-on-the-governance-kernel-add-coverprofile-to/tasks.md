# Tasks

## Task List

- [x] `t-01` Implement the coverage checker core: parse a Go coverage profile with union/dedup semantics, aggregate covered/total statements per kernel package, load/compare a JSON baseline, and apply an exclusion list
  - depends_on: []
  - target_files: [internal/coverage/coverage.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-005]

- [x] `t-02` Implement the `covergate` CLI tool with `-check` (compare profile to committed baseline; non-zero exit on regression, with a `--write` remediation message) and `-write` (regenerate baseline), mirroring `internal/toolgen/cmd/gen-surface-manifest`
  - depends_on: [t-01]
  - target_files: [internal/coverage/cmd/covergate/main.go, internal/coverage/cmd/covergate/main_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-004]

- [x] `t-03` Unit-test the checker core: union/dedup of duplicate coverpkg blocks, fail-closed on a simulated below-baseline drop, pass on equal/above, exclusion-list applied, and missing/unreadable baseline rejected
  - depends_on: [t-01]
  - target_files: [internal/coverage/coverage_test.go, internal/coverage/cmd/covergate/main_test.go]
  - task_kind: test
  - covers: [REQ-002, REQ-003, REQ-005]

- [x] `t-04` Generate the committed kernel coverage baseline by running the full suite with `-coverpkg` scoped to the kernel and `covergate -write`
  - depends_on: [t-02]
  - target_files: [coverage-baseline.json]
  - task_kind: ops
  - covers: [REQ-002]

- [x] `t-05` Add an ubuntu-only `coverage` CI job that runs the full suite with `-coverpkg` scoped to the kernel packages and `-coverprofile`, then runs `covergate -check`; no skip/force/soft-pass path
  - depends_on: [t-02, t-04]
  - target_files: [.github/workflows/ci.yml]
  - task_kind: ops
  - covers: [REQ-001, REQ-002, REQ-004]

- [x] `t-06` Add a justfile recipe that runs the kernel-scoped coverage gate locally (profile + `covergate -check`)
  - depends_on: [t-02]
  - target_files: [justfile]
  - task_kind: ops
  - covers: [REQ-001]

- [x] `t-07` Document the coverage gate: the governance-kernel package set, how a regression is reported, the exclusion list, and the `covergate -write` ratchet-update workflow
  - depends_on: [t-05]
  - target_files: [docs/contributing.md]
  - task_kind: doc
  - covers: [REQ-005, REQ-006]
