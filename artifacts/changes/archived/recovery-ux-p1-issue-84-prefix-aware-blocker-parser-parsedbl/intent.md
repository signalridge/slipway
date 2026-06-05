# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: standard Go; governance engine under `internal/engine`, model types under `internal/model`, CLI views under `cmd/`

## Summary
Recovery UX P1 (issue #84): prefix-aware blocker parser (ParsedBlocker with Code/Subject/Detail/Raw), a code/prefix-family remediation vocabulary table parallel to canonicalReasonDefinitions, a top-level read-only recovery object carrying grouped `recovery.steps[]` remediation and command on next/run/validate JSON while `blockers[]` remains `[]ReasonCode`, compact handoff preserving the primary recovery command, and canonical messages for internal prefix tokens such as required_skill_stale:* and tasks_plan_changed_since_task_evidence:*

## Complexity Assessment
complex
<!-- Rationale -->
Multi-surface, coordination-heavy: touches the model layer (new ParsedBlocker type + remediation vocabulary parallel to canonicalReasonDefinitions), three CLI read-only views (next, run, validate), the compact handoff path, and CLIError — all of which must consume one shared parse result. Read-only/additive, but the single-parse invariant and cross-surface consistency make it more than a bounded one-file change.

## Guardrail Domains
<!-- none detected -->
None. Read-only, additive JSON surface + internal vocabulary. No auth, credentials, PII, financial, schema/migration, irreversible, or external-contract behavior is changed. New JSON fields are backward-compatible (optional, additive); existing gate/evaluation/state-transition logic is untouched. Consistent with the P0 (#83) precedent which was also `guardrail_domain=""`.

## In Scope
<!-- What is explicitly included -->
- **Blocker parser.** New `ParsedBlocker{Code, Subject, Detail, Raw}` type and a parser that decomposes prefix tokens (`required_skill_stale:<skill>:<artifact>`, `tasks_plan_changed_since_task_evidence:<taskID>`, and the other prefix families) exactly once. This is the single decomposition point.
- **Remediation vocabulary table.** A `code / prefix-family -> {Remediation, CommandTemplate, RecoveryClass}` table parallel to `canonicalReasonDefinitions` (`internal/model/reason_code.go`), supporting both exact codes and prefix families.
- **Read-only `recovery` object on views.** Added to `next`, `run`, and `validate` JSON. `recovery.steps[]` carries `remediation` / `command` per parsed `(code, subject)` blocker group while the existing `blockers[]` array remains `[]ReasonCode`. The object carries one `primary_recovery_command` / next-action chosen by a simple stage-priority rule. `cmd/validate.go` currently has no warnings/recovery channel — add one.
- **Compact handoff preserves the primary recovery command.** `cmd/next_handoff.go` must keep the single primary recovery command / next-action in the compact `next` / `run` JSON (the surface a host hits first), instead of stripping it the way `freshness_diagnostics` is stripped.
- **Canonical messages for internal tokens.** Definitions/messages for `required_skill_stale:*`, `tasks_plan_changed_since_task_evidence:*`, and the other prefix tokens that currently fall through `humanizeReasonCode` to bare titles.
- **Docs.** Document the new read-only `recovery` object and its grouped remediation steps in README/CLAUDE.md (they change the host-facing JSON contract).
- **Tests** covering parser, vocabulary coverage, recovery object on all three views, compact-handoff preservation, and canonical messages.

## Out of Scope
<!-- What is explicitly excluded -->
- The cross-gate dependency-ordering **recovery planner** (`internal/engine/recovery`) and the digest input dependency index → **P2 (#85)**.
- The **`slipway recover`** command (`--dry-run`, `--from-artifact`, `--restamp`) → **P2 (#85)**.
- **Tier-2 operator-attested restamp** + its audit event + guardrail fail-closed → **P2 (#85)**.
- **P3 narrow-gap** lifecycle dead-ends (worktree rebind, dual-active naming, abort→repair loop, S2 scope guidance) and the broad README/docs sweep → **P3 (#86)**.
- Any change to gate/evaluation logic, governance semantics, state transitions, or what blocks. P1 changes only **how** blockers are described/surfaced, never **whether** they block.

## Constraints
<!-- Technical / business / time constraints -->
- **Read-only / additive only.** Must not change governance decisions, gate outcomes, or state transitions. New JSON fields must be backward-compatible (optional/additive); a host unaware of `recovery` keeps working.
- **Single-parse invariant.** The parser is the only place prefix tokens are decomposed; views and `CLIError` consume the same parse result (no re-splitting tokens at call sites).
- Reuse existing infrastructure: `CLIError.Remediation` already exists (`cmd/errors.go`); the vocabulary sits parallel to `canonicalReasonDefinitions`.
- Go; `go build ./...` and `go test ./...` must stay green.

## Acceptance Signals
<!-- What verifiable signals indicate completion -->
- `validate --json` and the default compact `next` / `run` JSON carry a non-empty primary recovery command / next-action for stale/blocked states (test: construct a blocked/stale change, assert the field is present and non-empty).
- Every recovery-relevant blocker rendered through `recovery.steps[]` carries a remediation string — no bare humanized fallthrough for the known internal tokens (test asserts a non-empty remediation for each known exact code and prefix family).
- The parser is the single decomposition point; views and `CLIError` consume the same parse result (parser unit tests + assertion that the surfacing paths route through it).
- `go build ./...` and `go test ./...` green.

## Open Questions
<!-- All entry-time unknowns were RESOLVED during S1 research; see research.md `## Unknowns`. -->
- RESOLVED (S1 research): `ParsedBlocker` lives in `internal/model` (new `recovery.go`) beside `reason_code.go`; the three serialized views (`nextView`, `nextHandoffView`, `validateView`) construct `Blockers []ReasonCode` directly.
- RESOLVED (S1 research): the single `primary_recovery_command` is selected by a static `recoveryClassPriority` constant list (not a per-change dependency graph), which keeps it inside `internal/model` and away from the P2 planner.
- RESOLVED (S1 research): `CLIError` already carries `Remediation`/`Reasons`; it consumes the shared `model.BuildRecovery` over its `Reasons` in `newCLIErrorWithReasons`.
- RESOLVED (S1 research): the recovery-relevant prefix-token inventory is enumerated in the `blockerRemediations` table; only `tasks_plan_changed_since_task_evidence` needed a new canonical message.

## Deferred Ideas
<!-- Identified but postponed ideas -->
- Full dependency-ordered recovery planner + `slipway recover` + Tier-2 attested restamp → P2 (#85).
- P3 narrow-gap lifecycle dead-ends and broad docs sweep → P3 (#86).

## Approved Summary
<!-- Re-confirmed after rescope pivot (2026-06-05): scope amended to include the cmd/repair.go caller of the refactored blockerSkillName. -->
P1 (#84) is done **standalone** per user direction ("P1 单独做", 2026-06-05), under the standing `/goal` authorization to drive this change to done-ready.

The change adds, as a **read-only / additive** layer only: (1) a prefix-aware blocker parser (`ParsedBlocker`), (2) a remediation vocabulary table parallel to `canonicalReasonDefinitions`, (3) a read-only `recovery` object — grouped `recovery.steps[]` with `remediation`/`command` plus one `primary_recovery_command` selected by a static stage-priority rule — on `next`/`run`/`validate` and `CLIError`, (4) compact-handoff preservation of that primary command, (5) canonical messages for internal prefix tokens, and (6) docs for the new JSON fields. The existing `blockers[]` arrays remain `[]ReasonCode`; the `blockerSkillName` refactor to the shared parser updates both call sites (`cmd/next_skill_view.go` and `cmd/repair.go`).

It explicitly **excludes** the P2 recovery planner, the `slipway recover` command, Tier-2 attested restamp (all → #85), and the P3 narrow-gap lifecycle fixes (→ #86). It changes only how blockers are described/surfaced, never whether they block, and makes no gate/semantics/state changes.

Primary acceptance: blocked/stale states surface a non-empty primary recovery command on `validate`/`next`/`run`, and every known internal blocker token renders a remediation string; build and tests stay green.
