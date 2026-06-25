# Tasks

## Task List

- [x] `t-01` Add regression coverage proving retired command-skill cleanup
  generalizes by class — a synthetic never-enumerated retired id, both
  manifest-absent (legacy) and manifest-present residue, and every
  command-skill host (codex, kiro, qwen, table-driven) — while staying
  fail-closed so a retired-named dir holding user-modified generated-shape
  content is preserved; plus a non-tautological contract test that resolves each
  generated command skill `command_id` on the live root command tree
  (`newRootCmd().Commands()`, not the registry slice the generator iterates).
  - depends_on: []
  - target_files: [internal/toolgen/toolgen_test.go, cmd/template_flag_contract_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-02` Make refresh prune retired command-skill directories by content
  signature for discovery (generated shape plus a `command_id` absent from the
  current registry) while routing every deletion through the ownership-manifest
  fail-closed path for safety (sha256 managed-modified refusal for
  manifest-tracked residue; exact canonical-body match for manifest-absent
  legacy residue). No hand-maintained retired-name list, and do not grow the
  registry-derived static set in `allGeneratedSkillDirNameSet`. Fix locus is
  `cleanupStaleSkillDirs` in toolgen.go, cfg-parameterized so it covers every
  command-skill host.
  - depends_on: [t-01]
  - target_files: [internal/toolgen/toolgen.go, internal/toolgen/install_profiles.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004]

- [x] `t-03` Run focused adapter and command-surface verification.
  - depends_on: [t-02]
  - target_files: [internal/toolgen/toolgen_test.go, cmd/template_flag_contract_test.go, docs/SURFACE-MANIFEST.json]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
