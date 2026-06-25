# Tasks

## Task List

- [x] `t-01` Repair registry, CLI command surface, and command-surface tests.
  - depends_on: []
  - target_files: ["cmd/root.go", "cmd/config.go", "cmd/review.go", "cmd/review_test.go", "cmd/template_flag_contract_test.go", "cmd/command_description_contract_test.go", "cmd/root_help_test.go", "cmd/config_test.go", "cmd/root_path.go", "cmd/root_internal_test.go", "cmd/learn_test.go", "cmd/retired_commands_test.go", "internal/tmpl/templates/skills/workflow/command-reference.md.tmpl", "internal/toolgen/toolgen.go", "internal/toolgen/toolgen_test.go", "internal/toolgen/surface_manifest_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-005, REQ-007]

- [x] `t-02` Repair command, adapter, handoff, and manifest documentation.
  - depends_on: ["t-01"]
  - target_files: ["README.md", "docs/reference/commands.md", "docs/ja/reference/commands.md", "docs/zh/reference/commands.md", "docs/commands.md", "docs/ja/commands.md", "docs/zh/commands.md", "docs/reference/ai-tools.md", "docs/ai-tools.md", "docs/explanation/design.md", "docs/ja/explanation/design.md", "docs/zh/explanation/design.md", "docs/design.md", "docs/ja/design.md", "docs/zh/design.md", "docs/assets/diagrams/tool-adapters.svg", "docs/SURFACE-MANIFEST.json"]
  - task_kind: doc
  - covers: [REQ-001, REQ-003, REQ-004, REQ-006]

- [x] `t-03` Run focused verification and record generated-surface consistency.
  - depends_on: ["t-01", "t-02"]
  - target_files: ["cmd/root.go", "cmd/config.go", "cmd/review.go", "cmd/review_test.go", "cmd/template_flag_contract_test.go", "cmd/command_description_contract_test.go", "cmd/root_help_test.go", "cmd/config_test.go", "cmd/root_path.go", "cmd/root_internal_test.go", "cmd/learn_test.go", "cmd/retired_commands_test.go", "internal/tmpl/templates/skills/workflow/command-reference.md.tmpl", "internal/toolgen/toolgen.go", "internal/toolgen/toolgen_test.go", "internal/toolgen/surface_manifest_test.go", "README.md", "docs/reference/commands.md", "docs/ja/reference/commands.md", "docs/zh/reference/commands.md", "docs/commands.md", "docs/ja/commands.md", "docs/zh/commands.md", "docs/reference/ai-tools.md", "docs/ai-tools.md", "docs/explanation/design.md", "docs/ja/explanation/design.md", "docs/zh/explanation/design.md", "docs/design.md", "docs/ja/design.md", "docs/zh/design.md", "docs/assets/diagrams/tool-adapters.svg", "docs/SURFACE-MANIFEST.json"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
