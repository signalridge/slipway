# Tasks
## Project Context
- Tech Stack: Go
- Conventions: governance kernel; deterministic kernel + AI host. Single-validator reuse (`AssuranceStructureBlockers`) covers done + validate. Skill template source of truth is `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl`; `.claude`/`.codex` copies are generated.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Layer 2 deterministic floor: in `internal/engine/artifact/manager.go` add a template-derived assurance scaffold detector (lazily derive each required section's canonical scaffold body from the embedded `assurance.md` template via `TemplateContent` + `markdownSectionLines`; normalize whitespace; cache once). Extend `AssuranceStructureBlockers` so that, after `validateSectionStructure` passes, each required section whose normalized body equals or still contains its template scaffold or derived seed sentence yields an `assurance_section_placeholder:<heading>` blocker. Empty-body case stays owned by the structure check.
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/artifact/manager.go, internal/engine/progression/validation.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-02` Layer 1 attestation enforcement: in `internal/engine/progression/authority.go` (`buildShipAuthorityFromReadiness`) and the S4 readiness/advance wiring, when the effective preset is standard/strict, require a passing `final-closeout` record to carry the `closeout:assurance_complete=pass` reference; when the record is missing or the passing record omits the reference, append a `closeout_assurance_attestation_missing` reason code to the verify-skill blockers (mirroring the `parseReviewLayerOutcomes`/`reviewLayerBlockerSpecs` reference pattern). Route `cmd/next_skill_view.go` through the same final-closeout-required predicate so diagnostics `skill_evidence` matches the S4 route/blockers. Do not re-read assurance prose. Light preset is unaffected.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/next_skill_view.go, internal/engine/progression/advance_governed.go, internal/engine/progression/authority.go, internal/engine/progression/readiness.go]
  - task_kind: code
  - covers: [REQ-005, REQ-006]

- [x] `t-03` Strengthen `final-closeout` skill: edit `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl` to require per-section authored-vs-scaffold judgment and promote `closeout:assurance_complete=pass` from advisory ("also add") to a required reference on standard/strict. Regenerate the `.claude/skills/slipway-final-closeout/SKILL.md` and `.codex/skills/slipway-final-closeout/SKILL.md` copies so generated surfaces match the template.
  - wave: 1
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl, .claude/skills/slipway-final-closeout/SKILL.md, .codex/skills/slipway-final-closeout/SKILL.md]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-04` Tests: in `internal/engine/artifact/manager_test.go` add placeholder-only, partial-placeholder, old Archive Decision seed-sentence, fully-authored, and template-drift-safety cases for `AssuranceStructureBlockers`. Add Layer 1 authority/readiness tests asserting `closeout_assurance_attestation_missing` fires when a standard/strict change lacks `final-closeout` or has a passing record that omits the attestation, and clears when present. Update `internal/tmpl/templates_test.go` (currently `TestFinalCloseoutTemplateKeepsAssuranceReferenceConditional`, lines 81-93) to the new required-reference contract, and update CLI done/next fixtures so standard done-ready paths carry the required final-closeout attestation, diagnostics `skill_evidence` lists missing plain-standard final-closeout, and light optional-closeout behavior stays covered.
  - wave: 2
  - depends_on: [t-01, t-02, t-03]
  - target_files: [cmd/lifecycle_commands_test.go, cmd/progression_next_test.go, internal/engine/artifact/manager_test.go, internal/engine/progression/authority_test.go, internal/tmpl/templates_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-005, REQ-006]

- [x] `t-05` Mechanical verification: run `go build ./...` and `go test ./...`; both MUST pass on the worktree. Confirm no unrelated test regressions from the new blockers or diagnostic evidence predicate sharing.
  - wave: 3
  - depends_on: [t-04]
  - target_files: [cmd/next_skill_view.go, internal/engine/artifact/manager.go, internal/engine/progression/authority.go, internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
