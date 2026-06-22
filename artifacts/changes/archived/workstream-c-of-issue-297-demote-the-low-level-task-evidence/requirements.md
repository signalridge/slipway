# Requirements

Workstream C of issue #297. Finish the agent-facing surface simplification A
(#300) and B (#305) started: curated agent surfaces teach only the high-level
`slipway evidence task --result-file` result-import model, never the old 10-field
task-evidence ledger protocol. Posture: demote-not-erase (Option A) — the manual
flag mode stays a functional CLI surface, discoverable via `--help`.

## Requirements

### Requirement: TDD-Governance Skill Teaches Result Import
REQ-001: The `tdd-governance` skill template MUST NOT instruct agents to use the
manual `--task-kind`, `--verdict`, or `--evidence-ref` flags, and MUST frame
investigation/doc task recording as a high-level `slipway evidence task
--result-file` import with a valid result `verdict`.

#### Scenario: Investigation/doc TDD guidance uses result import
GIVEN the generated `tdd-governance` skill
WHEN an agent reads how to record a non-applicable-TDD investigation or doc task
THEN the guidance directs it to write an executor result JSON and import it with
`slipway evidence task --result-file`, and contains no manual `--task-kind` /
`--verdict` / `--evidence-ref` flag instruction.

### Requirement: Evidence Command Body Is Result-File-Only Plus Breadcrumb
REQ-002: The `command-evidence-body` partial MUST present `evidence task` as
`--result-file` (with `--json` and `--change`) only, replacing the manual
per-field Flags enumeration and manual-only Contract bullets with exactly one
clearly-labeled breadcrumb routing to `slipway evidence task --help`, while
preserving the `evidence skill` documentation.

#### Scenario: Evidence task surface shows result-file and a single fallback breadcrumb
GIVEN the generated evidence command surface (`.codex/skills/slipway-evidence`
and `docs/commands.md`)
WHEN an agent reads how to record task evidence
THEN it sees the `--result-file` contract plus exactly one breadcrumb line
pointing to `slipway evidence task --help` for the manual fallback, and sees no
enumerated manual `--task-id` / `--run-summary-version` / `--task-kind` /
`--verdict` / `--evidence-ref` / `--target-file` flags.

### Requirement: Toolgen Evidence Arguments Drop the Manual Variant
REQ-003: The `evidence` command Arguments string in toolgen MUST expose only
`task --result-file ...`, `skill ...`, and `suite-result ...`; the
`task --task-id ... --run-summary-version ... --task-kind ...` manual variant
MUST be removed from the exported Arguments contract.

#### Scenario: Surface manifest carries no manual task variant
GIVEN the regenerated `docs/SURFACE-MANIFEST.json`
WHEN the `evidence` command Arguments token is read
THEN it contains the result-file task form but no manual `evidence task` flag
variant, and `gen-surface-manifest --check` passes.

### Requirement: Wave-Orchestration Drops Manual-Mode Framing
REQ-004: The `wave-orchestration` skill template MUST NOT frame manual flag mode
as an agent path; residual "manual flag mode" qualifiers around blockers,
`captured_at`, and `freshness_inputs` MUST be reworded to state the guardrail
directly (blockers and freshness live in the result JSON; the engine computes
freshness and timestamps).

#### Scenario: Execution driver teaches a single recording model
GIVEN the generated `wave-orchestration` skill
WHEN an agent reads the task-evidence recording guardrails
THEN they describe the result-JSON model only, with no "manual flag mode"
phrasing that presents manual flags as an agent action.

### Requirement: Derived Surfaces Are Regenerated Consistently
REQ-005: After REQ-001..004, all derived surfaces MUST be regenerated from the
worktree binary so source templates, generated `.codex`/`.claude` adapter
surfaces, docs, and the surface manifest stay aligned as one contract.

#### Scenario: Regeneration is consistent and manual-protocol-free
GIVEN the worktree binary rebuilt from the edited templates and toolgen
WHEN generated local adapter outputs, docs, and `docs/SURFACE-MANIFEST.json` are
regenerated
THEN `.codex/skills/slipway-evidence/SKILL.md`,
`.claude/commands/slipway/evidence.md`, the relevant `.codex`/`.claude`
governance skills, and committed docs all reflect the demoted surface;
`go run ./internal/toolgen/cmd/gen-surface-manifest --write` followed by
`--check` passes; and a search for the manual `evidence task --task-id ...
--run-summary-version ...` protocol over `internal/tmpl`, `.codex/skills`,
`.claude/skills`, `.claude/commands/slipway/evidence.md`, and `docs/` returns no
agent-facing teaching.

### Requirement: Manual CLI Surface Stays Functional and Discoverable
REQ-006: The change MUST NOT remove or hide the manual `evidence task` flags from
the cobra CLI or engine; `slipway evidence task --help` MUST still list them,
labeled "Manual flag mode only".

#### Scenario: Manual mode remains available as a labeled fallback
GIVEN the built CLI after this change
WHEN `slipway evidence task --help` is run
THEN it still shows `--result-file` and every manual flag labeled "Manual flag
mode only", and manual-mode engine behavior is unchanged.

### Requirement: Contract Tests Align and the Full Suite Passes
REQ-007: Contract and template tests that assert on the changed agent surface
MUST be updated to expect the demoted (result-file-only plus breadcrumb) surface,
and the full test suite MUST pass.

#### Scenario: Demoted-surface tests and full suite are green
GIVEN the updated contract/template tests and the demoted surfaces
WHEN `go test ./internal/tmpl ./internal/toolgen ./cmd` and then `go test ./...`
are run
THEN both pass, asserting the agent surfaces teach `--result-file` and no longer
teach the manual ledger protocol.

## Test Strategy
Test-first: update the template/toolgen/description contract tests (t-01) to
expect the demoted surface before editing sources (t-02), so the contract tests
go RED then GREEN. Because REQ-003 removes manual evidence-task flags from the
human/agent Arguments string while REQ-006 keeps those flags Cobra-visible,
t-01 must update the reverse flag-coverage contract with a narrowly justified
evidence-task manual-mode exemption. Regeneration (t-03) writes then checks the
surface manifest with `gen-surface-manifest --write` and `--check`. Final
verification (t-04) runs the focused package tests, the manifest check, the full
`go test ./...`, and black-box `--help` checks proving REQ-006.

## Risks and Mitigations
- Risk: regeneration touches generated files beyond the listed target_files
  (scope escape). Mitigation: list expected generated outputs in t-03 target_files
  and expand them (scope-only-safe at S2) from the actual regen diff before
  recording evidence.
- Risk: over-erasing manual guidance harms discoverability (the user-flagged
  user-visibility concern). Mitigation: demote-not-erase keeps one `--help`
  breadcrumb (REQ-002) and the unchanged cobra `--help` (REQ-006).
- Risk: contract tests assert exact old strings and fail closed. Mitigation:
  REQ-007 updates them in the same change, test-first.
- Risk: removing the manual variant from `toolgen.CommandArguments("evidence")`
  conflicts with the existing reverse Cobra flag coverage contract. Mitigation:
  t-01 encodes the intentional evidence manual-mode exemption with rationale
  while keeping REQ-006 black-box `--help` coverage.
