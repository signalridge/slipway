# Tasks

## Task List

- [x] `t-01` Add structured env wiring metadata to the catalog, runtime contracts, and model tests.
  - depends_on: []
  - target_files: [`internal/model/env_catalog.go`, `internal/model/env_catalog_test.go`, `internal/engine/capability/resolver.go`, `internal/engine/capability/resolver_test.go`]
  - task_kind: code
  - covers: [REQ-001, REQ-004]

- [x] `t-02` Project env wiring metadata through `config list --env` text/JSON and command tests.
  - depends_on: [`t-01`]
  - target_files: [`cmd/config.go`, `cmd/config_test.go`, `cmd/tool_github.go`, `cmd/tool_github_test.go`]
  - task_kind: code
  - covers: [REQ-002, REQ-004]

- [x] `t-03` Add host environment documentation and link it from public reference surfaces.
  - depends_on: [`t-01`, `t-02`]
  - target_files: [`docs/reference/host-environment.md`, `docs/reference/commands.md`, `docs/commands.md`, `docs/index.md`, `README.md`]
  - task_kind: doc
  - covers: [REQ-003]

- [x] `t-04` Run focused and full verification for the env contract repair.
  - depends_on: [`t-01`, `t-02`, `t-03`]
  - target_files: [`artifacts/changes/fix-runtime-host-env-wiring/verification/wave-verification-notes.md`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
