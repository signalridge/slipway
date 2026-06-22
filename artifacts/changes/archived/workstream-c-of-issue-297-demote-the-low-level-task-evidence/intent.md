# Intent

## Summary
Workstream C of issue #297: demote the low-level task-evidence ledger flag protocol from agent-facing surfaces (tdd-governance + wave-orchestration skills, the evidence command-reference body, and toolgen Arguments) and align generated skills/docs/surface-manifest around the evidence-task --result-file result-import model and the slipway evidence suite-result keystone introduced by Workstreams A/B. Keep the still-supported manual flag CLI mode functional and discoverable via --help; posture is demote-not-erase so a supported fallback never becomes hidden knowledge. Regenerate .codex/.claude generated skills, docs/commands.md, docs/reference/*, and docs/SURFACE-MANIFEST.json; update affected contract tests.
## Complexity Assessment
complex
<!-- Rationale -->
Spans ~13 agent-facing surfaces across four owners (skill templates, the evidence
command-reference partial, toolgen Arguments, ignored local adapter outputs for
Codex/Claude, and committed docs/manifest outputs), plus contract-test
alignment. It changes the exported surface contract, so it must be reviewed as
an external contract even though no engine logic changes.

## Guardrail Domains
none — no auth/authz, credentials/PII, financial, schema-migration, or
irreversible operations. The exported surface manifest is a contract surface
(reviewed as such at S3), but the change is wording-demotion + regeneration only;
the manual flag CLI mode stays functional, so no behavior fails closed.

## In Scope
C1 — demote the low-level task-evidence ledger-flag protocol from **agent-facing**
surfaces so they teach only the high-level `--result-file` result-import model:
- `internal/tmpl/templates/skills/tdd-governance/SKILL.md.tmpl`: replace the
  `--task-kind`/`--verdict`/`--evidence-ref` low-level phrasing (~L90-93) with
  high-level result-import framing.
- `internal/tmpl/templates/_partials/command-evidence-body.tmpl`: demote the
  manual-flag Contract bullets (~L29-35) and the per-field Flags enumeration
  (~L45-62) to a single, clearly-labeled `--help` breadcrumb; keep `--result-file`
  and the `evidence skill` flags.
- `internal/toolgen/toolgen.go`: drop the manual-flag variant from the `evidence`
  command Arguments string (~L334).
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`: tighten the
  residual "manual flag mode" cross-references (~L197-201) while keeping the
  guardrail intent (blockers/freshness live in the result JSON, engine-computed).

C5 — regenerate the derived/checked-in surfaces and align tests:
- `.codex/skills/slipway-evidence|slipway-tdd-governance|slipway-wave-orchestration/SKILL.md`
  plus the `.claude/skills/` counterparts and
  `.claude/commands/slipway/evidence.md` (via the repo `init --tools
  codex,claude --refresh` flow using the worktree binary). These adapter
  outputs are ignored local generated surfaces, so verify their content
  explicitly rather than relying on git diff.
- `docs/SURFACE-MANIFEST.json`, `docs/commands.md`, `docs/reference/commands.md`,
  `docs/reference/ai-tools.md` (via `gen-surface-manifest --write` followed by
  `gen-surface-manifest --check`; keep docs/reference tokens aligned with the
  manifest).
- Update affected contract tests (`internal/tmpl/templates_test.go`,
  `internal/toolgen/toolgen_test.go`, `cmd/template_flag_contract_test.go`,
  `cmd/command_description_contract_test.go`) to match the demoted surface.

Posture: **demote-not-erase** — the manual flag protocol is removed from curated
agent guidance but a one-line fallback breadcrumb routes to `slipway evidence
task --help`, preserving discoverability of the still-supported surface.

## Out of Scope
- Removing/hiding the manual flag mode from the cobra CLI or engine — Workstream B
  intentionally kept it; it stays functional and `--help`-discoverable (labeled
  "Manual flag mode only").
- Workstreams A (#300) and B (#305) — already merged.
- C2/C3/C4 semantic rework — verified already satisfied on `main`: `evidence
  skill` is already high-level; `goal-verification` already produces the keystone
  via `slipway evidence suite-result`; `evidence-digests.yaml` is already
  described as engine-owned. Only touch these if S3 review names a concrete gap.
- Any change to engine logic, lifecycle gates, the freshness/digest cascade,
  scope audit, parallel-overlap detection, or run-version derivation.
- Renaming `fix`/`repair` (issue marks this an optional future PR).

## Constraints
- Keep source templates, generated skills, docs, and manifest aligned as one
  contract (project CLAUDE.md change-discipline).
- Regeneration MUST use the worktree binary built from the edited templates, never
  a stale installed binary.
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check` must pass and full
  `go test ./...` must be green.
- Introduce no bypass/force-close/private-attestation path and no hidden-host-
  knowledge requirement (the breadcrumb + `--help` keep the fallback public).

## Acceptance Signals
- `slipway evidence task --help` still shows `--result-file` plus the manual flags
  labeled "Manual flag mode only" (CLI unchanged).
- Generated `.codex`/`.claude` adapter surfaces for evidence, tdd-governance,
  and wave-orchestration no longer teach the 10-field ledger protocol as a
  co-equal path; they teach `--result-file` only, plus the single `--help`
  breadcrumb.
- `rg "evidence task --task-id .*--run-summary-version|--task-kind|--target-file"`
  over `internal/tmpl`, generated skills, and `docs/` returns no agent-facing
  teaching of the manual protocol (engine code/tests may still reference it).
- `gen-surface-manifest --check` passes (manifest regenerated and consistent with
  the demoted Arguments string).
- `go test ./...` is green.
- A fresh agent reading the wave-orchestration/tdd-governance/evidence skills only
  needs to learn: write executor result JSON → `slipway evidence task
  --result-file <path> --json`.

## Open Questions
None — scope is fully grounded by direct source verification against `main`
(checkpoint/learn/stats already removed by A; `--result-file` and engine-owned
run-version already present from B; engine-emitted `record_command` already
result-file based). No technical unknown requires a research route.

## Deferred Ideas
- Optionally name `slipway evidence suite-result` explicitly inside the
  consumer-side reviewer templates (security-review / spec-compliance-review),
  which today route to "fresh suite-result proof" via the goal-verification
  producer path. Not required for C3 acceptance; deferred unless S3 flags it.

## Approved Summary
Deliver Workstream C of issue #297: finish the surface-simplification that
Workstreams A (#300) and B (#305) started, so agent-facing surfaces teach only
the high-level `slipway evidence task --result-file` result-import model instead
of the old 10-field task-evidence ledger protocol.

Scope (verified remaining work): C1 edits four agent-facing source surfaces —
the tdd-governance and wave-orchestration skill templates, the evidence
command-reference partial, and the toolgen evidence Arguments string — then C5
regenerates the derived local adapter surfaces (`.codex`/`.claude` skills plus
Claude's evidence command surface), committed docs (`docs/commands.md`,
`docs/reference/*`, `docs/SURFACE-MANIFEST.json`), and aligns the affected
contract tests. C2/C3/C4 are already satisfied on `main` and are out of scope
unless S3 review names a concrete gap.

Posture decision (the UX / user-visibility tradeoff): **Option A —
demote-not-erase.** The manual per-field flag mode stays a functional CLI surface
(Workstream B kept it; cobra `slipway evidence task --help` still lists it labeled
"Manual flag mode only"). Curated agent skills/docs stop teaching the 10-field
protocol and instead teach `--result-file` only, plus a single clearly-labeled
breadcrumb line routing to `slipway evidence task --help` for host-internal /
recovery use. This delivers the issue's clean mental model while preserving
discoverability, so a still-supported surface never becomes hidden host knowledge
(consistent with this repo's CLAUDE.md). Options B (full erase) and C (keep
subordinate) were presented with concrete rendered examples and not chosen.

Out of scope: removing/hiding the manual mode from cobra `--help` or the engine;
A/B rework; any engine logic, gate, freshness/digest, scope-audit, or run-version
change.

Primary acceptance signal: a fresh agent reading the regenerated
wave-orchestration / tdd-governance / evidence skills only needs to learn "write
executor result JSON → `slipway evidence task --result-file <path> --json`"; no
agent-facing surface still teaches the manual protocol; `gen-surface-manifest
--check` and `go test ./...` are green.

Confirmation: scope and the Option-A posture confirmed under the user's standing
"遇到 block 做最优决策 / 怎么做最好后开始" directive after the agent presented
concrete A/B/C examples for the user-visibility tradeoff (2026-06-23 UTC).
