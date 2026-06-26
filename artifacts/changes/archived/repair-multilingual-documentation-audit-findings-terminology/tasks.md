# Tasks

## Task List

- [x] `t-01` Repair command surface references and generated manifest metadata.
  - depends_on: []
  - target_files: [cmd/root.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/surface_manifest.go, docs/SURFACE-MANIFEST.json, docs/commands.md, docs/reference/commands.md, docs/zh/commands.md, docs/zh/reference/commands.md, docs/ja/commands.md, docs/ja/reference/commands.md]
  - task_kind: code
  - covers: [REQ-002, REQ-004]
  - acceptance: `go test ./internal/toolgen/...` and `go run ./internal/toolgen/cmd/gen-surface-manifest --check` pass; command references include `slipway hook` while no `$slipway-hook` prompt surface is exported.

- [x] `t-02` Repair README parity and English factual documentation drift.
  - depends_on: []
  - target_files: [README.zh.md, README.ja.md, docs/installation.md]
  - task_kind: doc
  - covers: [REQ-001, REQ-004]
  - acceptance: targeted searches find no stale `MkDocs Material` install-stack references and README locale files include the handoff command row without duplicate language switchers.

- [x] `t-03` Repair Simplified Chinese localized terminology and link issues.
  - depends_on: [t-01, t-02]
  - target_files: [README.zh.md, docs/zh/ai-tools.md, docs/zh/commands.md, docs/zh/design.md, docs/zh/installation.md, docs/zh/reference/commands.md, docs/zh/workflow.md]
  - task_kind: doc
  - covers: [REQ-001, REQ-003, REQ-004]
  - acceptance: changed Chinese docs resolve localized SVG paths, use valid command examples, and targeted terminology scans show no remaining audit-listed misleading uses such as evidence `新鲜`, `可追溯`, or `同意图`.

- [x] `t-04` Repair Japanese localized terminology and link issues.
  - depends_on: [t-01, t-02]
  - target_files: [README.ja.md, docs/ja/ai-tools.md, docs/ja/commands.md, docs/ja/design.md, docs/ja/installation.md, docs/ja/reference/commands.md, docs/ja/workflow.md]
  - task_kind: doc
  - covers: [REQ-001, REQ-003, REQ-004]
  - acceptance: changed Japanese docs resolve localized SVG paths, use valid command examples, and targeted terminology scans show no remaining audit-listed misleading uses such as `権限` for authority, `容赦しません`, or `正直な強制レベル`.

- [x] `t-05` Sweep remaining Simplified Chinese documentation pages for audit terminology drift.
  - depends_on: [t-03]
  - target_files: [docs/zh/start-here.md, docs/zh/explanation/design.md, docs/zh/explanation/workflow.md, docs/zh/real-world-scenarios.md, docs/zh/how-to/install-and-refresh-adapters.md, docs/zh/how-to/recover-and-troubleshoot.md, docs/zh/contributing.md, docs/zh/operator-guide.md, docs/zh/tutorials/first-governed-change.md, docs/zh/tutorials/onboarding-existing-codebase.md, docs/zh/reference/ai-tools.md]
  - task_kind: doc
  - covers: [REQ-003, REQ-004]
  - acceptance: full Chinese documentation scans retain only intentional command, URL, or release-path literals for audit-listed tokens; prose uses consistent terms for artifacts, governed changes, fail-closed behavior, freshness/currentness, host/adapters, and diffs.

- [x] `t-06` Sweep remaining Japanese documentation pages for audit terminology drift.
  - depends_on: [t-04]
  - target_files: [docs/ja/start-here.md, docs/ja/explanation/design.md, docs/ja/explanation/workflow.md, docs/ja/real-world-scenarios.md, docs/ja/how-to/install-and-refresh-adapters.md, docs/ja/how-to/recover-and-troubleshoot.md, docs/ja/contributing.md, docs/ja/operator-guide.md, docs/ja/tutorials/first-governed-change.md, docs/ja/tutorials/onboarding-existing-codebase.md, docs/ja/reference/ai-tools.md]
  - task_kind: doc
  - covers: [REQ-003, REQ-004]
  - acceptance: full Japanese documentation scans retain only intentional authority or permission terms, with freshness rendered naturally as currentness/validity rather than `鮮度`.

- [x] `t-07` Polish English audit-listed wording in design and workflow docs.
  - depends_on: [t-03, t-04]
  - target_files: [docs/design.md, docs/workflow.md]
  - task_kind: doc
  - covers: [REQ-003, REQ-004]
  - acceptance: targeted English scans find no audit-listed awkward phrases such as `honest tier`, `honest enforcement tier`, `self-stamp`, or `oversold`.

- [x] `t-08` Repair root README stale documentation badge branding.
  - depends_on: [t-07]
  - target_files: [README.md]
  - task_kind: doc
  - covers: [REQ-001, REQ-004]
  - acceptance: root and localized README badge scans find no `materialformkdocs` or stale MkDocs branding, while docs badges use the current Astro branding.

- [x] `t-09` Repair S3 governance required-action handling for absorbed task-plan drift.
  - depends_on: [t-08]
  - target_files: [internal/engine/governance/runtime_actions.go, internal/engine/governance/runtime_actions_test.go]
  - task_kind: code
  - covers: [REQ-004]
  - acceptance: runtime required actions remain satisfied by fresh S3 reviewer evidence when the only execution-summary issues are S3 review-absorbed task-plan drift tokens; unrelated execution-summary blockers still fail closed.

- [x] `t-10` Repair S3 review-authority ship-gate handling for absorbed task-plan drift.
  - depends_on: [t-09]
  - target_files: [internal/engine/progression/authority.go, internal/engine/progression/authority_test.go]
  - task_kind: code
  - covers: [REQ-004]
  - acceptance: review authority and terminal ship authority treat `tasks_plan_changed_since_task_evidence` as S3 reviewer recertification input, while `stale_execution_evidence` and unrelated blockers remain fail-closed.
