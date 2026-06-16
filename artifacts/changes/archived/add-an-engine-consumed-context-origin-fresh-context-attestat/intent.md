# Intent

## Summary
Add an engine-consumed context-origin / fresh-context attestation contract so independence-critical lifecycle steps cannot be satisfied from the authoring context. Today the engine only truly enforces fresh context at S2 wave dispatch (DispatchEvidenceBlockers, defeatable via self-asserted degraded_sequential) and the goal-verification SAST safety_baseline; review/verify/closeout rely on host-honored markdown and self-asserted tokens the engine ignores. Concretely: final-closeout writes closeout:reviewer_independence=pass (SKILL.md L160) but NO engine code consumes it (authority.go parses only closeout:assurance_complete=pass at :210/:238), so the same context that authored, reviewed, and verified can also perform closeout and stamp independence=pass. The objective is a tamper-evident, engine-consumed attestation binding a verdict's context-origin (distinct-from-authoring), starting with making final-closeout's reviewer_independence actually gated, then extending to chain-binding across spec-compliance-review, code-quality-review, and goal-verification to close the serial author->review->verify->close collapse. The engine stays the sole inline verdict-stamping authority; the fix is an attestation the gate consumes, NOT spawning review/verify hosts as leaf subagents.

## Complexity Assessment
complex
<!-- Rationale: cross-cuts the progression authority (authority.go), the wave-sync
     gates (wave_sync.go), reason-code + recovery vocab, generated host skills /
     thin-host content, and docs; touches fail-closed governance enforcement, so it
     must fail closed with no bypass and be dogfooded through its own strict flow.
     The tamper-evidence mechanism is a genuine design unknown routed to research. -->

## Guardrail Domains
None as a SAST/classification domain (not Auth/Credentials/PII/Financial/Schema/Irreversible/External-API). It is nonetheless sensitive in the fail-closed sense: it modifies Slipway's own review/independence enforcement, so per CLAUDE.md "Review And Safety" it must fail closed and must not introduce bypass, force-close, or private attestation paths.

## In Scope
- `internal/engine/progression/authority.go`: parse and require a new context-origin attestation token for final-closeout (analogue of `assuranceCompleteReference`/`closeout:assurance_complete=pass` at :210/:238) so `closeout:reviewer_independence=pass` (today written at SKILL.md but engine-ignored) is actually consumed and gated.
- A new engine-consumed context-origin attestation contract applied to the four independence-critical verdicts: spec-compliance-review (#1), goal-verification (#2), code-quality-review (#3), final-closeout (#4) — each verdict must attest its producing context is distinct-from-authoring.
- Chain-binding assertion (the serial-collapse closer): the engine rejects a run where a single context produced spec-compliance + code-quality + goal-verification + closeout in one unbroken context.
- `internal/engine/progression/wave_sync.go` #5: engine gate that for the same `target_files`, the `task_kind=test` author context is distinct from the `task_kind=code` context, and that test tasks are frozen/dispatched before their dependent code tasks (today instructional only at wave-orchestration/SKILL.md L165-170). Framed as the relational test!=impl distinctness gate.
- `internal/engine/progression/wave_sync.go` #6: change `degraded_sequential` acceptance (currently accepted without blocking at ~:818) to attempt-based — accept only when paired with a genuine tool-unavailable signal, per gsd's capability-by-attempt rule.
- Reason codes + recovery remediation (`internal/model/reason_code.go`, `internal/model/recovery.go`) for every new blocker, with actionable public remediation.
- Generated host skills + thin-host content + toolgen that emit and document the new attestation tokens (`.claude/skills/slipway-{spec-compliance-review,code-quality-review,goal-verification,final-closeout,wave-orchestration}`, `internal/tmpl/`, `internal/toolgen/`), plus docs.
- Tests across all of the above (engine gates, reason-code contract, toolgen/thin-host content, fail-closed-on-standard/strict + advisory-on-light).

## Out of Scope
- Implementation-code's OWN context-window isolation as a fail-closed gate. Investigated and judged type-a (producer hygiene): nothing trusts the impl-author's self-verdict; it is a pure producer re-judged downstream. It stays a documented host-hygiene default (wave-orchestration/SKILL.md L152) and remains advisory, not a governed MUST.
- Per-executor / per-task worktree isolation (previously and deliberately rejected by the user; isolation stays change-level with target_files + post-result changed-file audit as the safety net).
- Spawning review/verify/closeout hosts as leaf subagents or otherwise stripping the engine's CLI verdict-stamping authority (explicitly rejected approach — the fix is an attestation the gate consumes).
- intake-clarification and research-orchestration independence (backstopped by the user-confirmation/user-selection handshake; should_consider, not this change).
- security-review as a standalone clean-context gate (inherits #1/#3 context; its real teeth are the goal-verification SAST safety_baseline, already engine-gated).
- plan-audit readiness-verdict isolation and adding a separate plan-checker agent (separate follow-up; structural gap vs gsd, not this change).

## Constraints
- The engine remains the SOLE inline verdict-stamping authority; the attestation is a token the gate CONSUMES. Hosts/subagents must not self-stamp freshness or final verdicts.
- New gates fail closed on `standard`/`strict`; advisory (non-blocking) on `light`, consistent with `closeout:assurance_complete` ("required on standard/strict").
- No backward-compat scaffolding for the old/ignored behavior; no bypass, force-close, or private attestation path (CLAUDE.md Review And Safety).
- This change must be dogfooded: it must itself ship through the new strict gates (self-host evidence).
- Use the current worktree's Slipway CLI as source of truth.

## Acceptance Signals
- `authority.go` parses and requires the final-closeout context-origin token on standard/strict; a missing token fails closed with a named reason code + remediation. Test proves pass-with-token / fail-without on standard/strict and advisory on light.
- An engine blocker rejects any review/verify/closeout verdict attested as produced in the authoring context, and rejects a single context producing the whole spec->quality->verify->close chain. Tests prove fail-closed (standard/strict) and advisory (light).
- `wave_sync.go` #5: engine fails closed when the same context authored test+code for the same `target_files`, or when test tasks were not frozen before dependent code dispatch. Test proves it.
- `wave_sync.go` #6: a bare self-asserted `degraded_sequential` token no longer passes DispatchEvidenceBlockers; it is accepted only when paired with a genuine tool-unavailable signal. Test proves the regression is closed.
- Generated skills + thin-host content + docs emit and explain the new tokens; toolgen/thin-host contract tests are green.
- `go test ./...` green; `gofmt -s -l` clean; golangci-lint clean.
- The change ships through its own strict governed flow with fresh dogfood evidence.

## Open Questions
- [x] Attestation tamper-evidence mechanism: how can the engine bind a verdict's context-origin (distinct-from-authoring) when it can only consume tokens the host emits via `slipway evidence`, and a single context can emit any token? Design the minimal mechanism — candidates: an engine-issued per-stage nonce/handshake the authoring context never sees; binding to run_version / captured_at / freshness-input divergence that only a separate fresh read can produce; or an explicit recorded-and-auditable attestation that raises forging cost. Determine what "tamper-evident" realistically means inside Slipway's host-honored evidence model and the smallest design that closes the serial-collapse path WITHOUT a leaf-subagent spawn and WITHOUT giving hosts self-stamp authority.

## Deferred Ideas
- plan-audit readiness-verdict isolation + a dedicated plan-checker agent (Slipway is structurally one gate behind gsd here).
- Routing sensitive-domain hardening through the goal-verification SAST safety_baseline producer (require `fresh:command_ref=` to bind to a real, fresh SAST artifact) rather than a diffuse security overlay.
- A structural build guard (gsd bug-936 analogue) that fails the build if a delegating surface is ever nested inside an isolated/forked execution.

## Approved Summary
User-confirmed 2026-06-15T19:39:41Z.

Add an engine-consumed context-origin attestation contract (verdict producing-context must be distinct-from-authoring), so independence-critical steps cannot be self-satisfied in the authoring context. Start by making final-closeout's `closeout:reviewer_independence=pass` actually parsed+required in `authority.go` (today engine-ignored), then extend to spec-compliance-review, code-quality-review, and goal-verification, plus a chain-binding assertion that rejects a single context producing the whole spec->quality->verify->close chain. Also add S2 gates: #5 the relational test!=impl distinctness gate (same `target_files`: `task_kind=test` context distinct from `task_kind=code`, tests frozen before dependent code dispatch) and #6 attempt-based `degraded_sequential` (accepted only when paired with a genuine tool-unavailable signal).

Scope boundary: the engine stays the sole inline verdict-stamping authority — the fix is an attestation the gate CONSUMES, NOT spawning review/verify hosts as leaf subagents. New gates fail closed on standard/strict, advisory on light (consistent with `closeout:assurance_complete`).

Out of scope (named): implementation-code's own context-window isolation (judged type-a producer hygiene, stays a documented host default); per-executor worktree isolation (previously rejected); intake/research independence (user-handshake backstopped); standalone security-review gate; plan-audit verdict isolation.

Primary acceptance signal: on standard/strict a missing/authoring-context attestation fails closed with a named reason code + remediation, proven by tests; `go test ./...`, gofmt, golangci-lint green; the change ships through its own strict governed flow.

Unresolved technical unknown kept under Open Questions (routes to research): the attestation tamper-evidence mechanism (engine can only consume host-emitted tokens; how to bind context-origin without a leaf-subagent spawn or host self-stamp authority).
