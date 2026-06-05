# Research

## Research Findings

### Architecture
- **Affected modules (all read directly, file:line):**
  - `internal/model/reason_code.go` — `ReasonCode{Code,Severity,Message,Detail}` (`:26`) carries `yaml:` tags, so it is **persisted** in verification/gate records and MUST NOT gain presentation fields. `canonicalReasonDefinitions` is the exact-code vocabulary (`:38-207`); `ReasonCodeFromSpec` already does a single `Cut(":")` into {Code, Detail} (`:230-242`); `humanizeReasonCode` is the bare-title fallthrough (`:395-406`).
  - `cmd/next.go` — `nextView` (`:16-54`, the rich internal view, also serialized by `--diagnostics`) with `Blockers []model.ReasonCode` (`:50`) used internally across `assembleSkillView`; `confirmationRequirement` (`:56-66`).
  - `cmd/next_handoff.go` — `nextHandoffView` (`:12-25`, the compact next/run output) built by `buildNextHandoffView` (`:190-234`); the comment at `:109-112` records that the compact view deliberately omits freshness diagnostics.
  - `cmd/next_skill_view.go` — `isRequiredSkillBlocker` already includes `required_skill_stale` post-P0 (`:449-456`); `requiredSkillStaleSet` (`:461-472`); `blockerSkillName` is an ad-hoc `Cut(":")` (`:474-483`) — the scattered decomposition P1 centralizes; `buildRequiredSkillEvidence` already emits a `stale` status on both paths (`:536-539`, `:579-581`, from P0).
  - `cmd/validate.go` — `validateView` (`:16-46`) has `Blockers []model.ReasonCode` but **no warnings/recovery channel** (only `Diagnostics []string`); built by `buildValidateViewForSlug` (`:180-277`).
  - `cmd/errors.go` — `CLIError` already has `Remediation string` (`:43`) and `Reasons []model.ReasonCode` (`:47`); `newCLIErrorWithReasons` (`:68-87`) is the single constructor.
  - `cmd/run.go` — emits `nextHandoffView` compact (`:76`) or the full `nextView` under `--diagnostics` (`:74,79`).
- **Dependency chains:** blocker specs are produced in `internal/engine/progression/*` (`evidence_digests.go`, `wave_sync.go`, `authority.go`, `advance_governed.go`) and `internal/engine/gate`, normalized to `[]model.ReasonCode`, then surfaced by the three `cmd` views + `CLIError`. New vocabulary/parser belongs in `internal/model` (where ReasonCode lives) and is projected at the `cmd` view boundary.
- **Blast radius:** additive only — three view structs and `CLIError` each gain one `recovery` field; `internal/model` gains one new file. No producer/gate logic changes; all `Blockers []ReasonCode` fields keep their type (zero internal ripple).
- **Constraints/invariants preserved:** persisted ReasonCode shape unchanged (no new yaml fields); gates/state transitions untouched; new JSON fields are `omitempty`/backward-compatible.

### Patterns
- **Vocabulary-table pattern:** `canonicalReasonDefinitions map[string]ReasonDefinition` (`reason_code.go:38`) is the established exact-code → {severity, message} table. The remediation table mirrors it: `map[string]blockerRemediation{Remediation, CommandTemplate, RecoveryClass}`, same key space.
- **Prefix family == Code:** every prefix token (`required_skill_stale:<skill>:<artifact>`, `tasks_plan_changed_since_task_evidence:<taskID>`, `review_layer_failed:<layer>`, `missing_required_artifact:<name>`, …) has its family as the first `:`-segment, which `ReasonCodeFromSpec` already stores as `Code`. Keying remediation by `Code` covers all families; the parser only further splits `Detail` into `Subject` (2nd segment) + `Detail` (rest).
- **Single normalization seam:** every blocker passes through `model.NormalizeReasonCodes` before serialization, and CLIError reasons too — the natural single place to project a recovery view.
- **Reuse of P0 surface:** `CLIError.Remediation`/`Reasons` already exist; the P0 `evidence restamp --skill X --dry-run` command (`cmd/evidence.go:50-120`) and `repair` recovery-routing (`cmd/repair.go:594-631`) already exist — remediation command templates point at them rather than inventing new commands (those are P2).

### Risks
- **Low — additive JSON contract drift:** a new `recovery` field could surprise strict consumers. Mitigation: `omitempty` + additive + documented (in-scope docs task). 
- **Low — golden-test churn:** existing view-JSON assertions may need the new field; mitigated because it is `omitempty` (absent unless an actionable recovery exists), so only blocked/stale fixtures change.
- **Medium — primary-command rule drifting toward the P2 planner:** the simple stage-priority selection must stay a static constant, not a derived dependency graph. Mitigation: implement as a fixed ordered list of recovery classes; unit-test it; document the P1/P2 boundary; never build a per-change graph or `--from-artifact` index (those are #85).
- **Guardrail domains:** none touched (read-only surface). **Reversibility:** fully reversible (additive fields + one new file).

### Test Strategy
- **Existing coverage:** there is no `cmd/next_test.go`/`validate_test.go`; view behavior is exercised via `cmd/lifecycle_commands_test.go`, `cmd/error_contract_test.go`, `cmd/status_view_build_test.go`, with helpers in `cmd/common_test.go` (`withCommandWorkspace`, `commandForRoot`, change-state seeding patterns like `change.CurrentState = model.StateS2Execute`). `internal/model` has no `reason_code_test.go` yet.
- **Infrastructure needs:** none new — reuse the cmd harness to seed a governed change in a blocked/stale state and assert the `recovery` object on next/run/validate JSON; plain table-driven unit tests for the model layer.
- **Verification approach per acceptance signal:**
  - *Primary recovery command present* → seed a `required_skill_stale`/blocked change; run next/run/validate; assert `recovery.primary_command` non-empty.
  - *Every known token renders a remediation* → table test over every remediation-table key + representative prefix tokens; assert non-empty Remediation, no humanize fallthrough; add the missing canonical message for `tasks_plan_changed_since_task_evidence`.
  - *Single parser* → unit-test `ParseBlocker` for all prefix families; refactor `blockerSkillName` to use it; assert surfacing routes through `model.BuildRecovery`.
  - build + tests green via `go build ./...` / `go test ./...`.

## Alternatives Considered
- **A — Enrich `ReasonCode` with Remediation/Command fields populated in `Normalize`.** Max reuse (every blocker everywhere carries remediation). Tradeoff: `ReasonCode` is persisted via `yaml:` tags → leaks presentation into verification/gate evidence (would need `yaml:"-"` hacks); broad golden-test churn across all commands. **Rejected:** pollutes the persisted evidence type.
- **B — Change every view's `Blockers []ReasonCode` to a rendered `[]RenderedBlocker`.** Per-blocker remediation directly on the blockers array. Tradeoff: `nextView.Blockers` is used internally throughout `assembleSkillView` (`appendReasonCodes`, `NormalizeReasonCodes`, `requiredSkillStaleSet`, capability signals) → invasive ripple, higher regression risk. **Rejected:** violates the minimal/read-only constraint.
- **C — New `internal/model` vocabulary + a top-level `recovery` projection at the view boundary.** Add `ParsedBlocker{Code,Subject,Detail,Raw}`+parser, a `blockerRemediation` table keyed by Code, and `RecoverySummary{PrimaryCommand, PrimaryAction, RecoveryClass, Steps[]}` where each Step is an actionable blocker rendered with parsed Subject/Detail + remediation + command. `nextView`/`nextHandoffView`/`validateView` and `CLIError` each gain one `recovery *RecoverySummary` field, all built by the same `model.BuildRecovery`. The `blockers` arrays stay `[]ReasonCode` (zero ripple; persisted ReasonCode unpolluted). Primary command is chosen by a static stage-priority constant (NOT the P2 planner). Also add the missing canonical message for `tasks_plan_changed_since_task_evidence` and peers so they no longer humanize-fallthrough.

- **Selected: C.** It satisfies all three acceptance signals (grouped remediation via recovery steps, one top-level primary recovery command, a single parser consumed by views + CLIError) while honoring read-only/additive, keeping the persisted ReasonCode shape intact, and cleanly isolating the P1/P2 boundary. See `decision.md` `## Selected Approach`.

## Unknowns
- Resolved: `ParsedBlocker` lives in `internal/model` (new file beside `reason_code.go`). Evidence: `reason_code.go` owns the blocker vocabulary.
- Resolved: views construct blockers as three serialized structs with `Blockers []ReasonCode`. Evidence: `cmd/next.go:50`, `cmd/next_handoff.go:22`, `cmd/validate.go:33`.
- Resolved: primary-command rule = static recovery-class stage-priority list, first match wins; not a dependency graph. Evidence: #84 non-goals exclude the planner.
- Resolved: CLIError wiring = add `recovery` built from `Reasons` in `newCLIErrorWithReasons`. Evidence: `cmd/errors.go:43,47,68-87`.
- Remaining for plan-audit: the exact remediation-table key/message set (the recovery-relevant subset) and the precise stage-priority ordering — to be enumerated in `requirements.md`/`tasks.md`.

## Assumptions
- The recovery surface is presentation-only; producers keep emitting the same `[]ReasonCode`. Evidence: `internal/engine/progression/*` producers + `model.NormalizeReasonCodes`.
- An `omitempty` recovery pointer keeps existing non-blocked outputs byte-identical. Evidence: Go `encoding/json` omitempty on a nil pointer.
- The P0 `evidence restamp --skill X --dry-run` command and `repair` routing remain the command targets for digest-drift remediation. Evidence: `cmd/evidence.go:50-120`, `cmd/repair.go:594-631`.

## Canonical References
- `internal/model/reason_code.go` — blocker vocabulary, normalization, current decomposition.
- `cmd/next.go`, `cmd/next_handoff.go`, `cmd/next_skill_view.go` — next/run views + blocker surfacing.
- `cmd/validate.go` — validate view (needs the recovery channel).
- `cmd/errors.go` — CLIError remediation/reasons.
- `cmd/evidence.go`, `cmd/repair.go` — P0 recovery command targets the remediation templates reference.
- `internal/engine/progression/{evidence_digests,wave_sync,advance_governed,authority}.go` — prefix-token blocker producers.
