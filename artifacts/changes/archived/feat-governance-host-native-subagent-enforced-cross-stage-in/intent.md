# Intent

## Summary
feat(governance): host-native-subagent enforced cross-stage independence across
the full author->review->verify->close chain, with S3 review generalized from a
fixed review pair to a VARIABLE PARALLEL REVIEW-SET (issue #240).

CORE (locked #240 design; t-01..t-08 are implemented on disk and carry over as the
foundation): generalize #239's `review_origin` into ONE chain-wide per-subagent
context-handle grammar `context_origin:stage=<stage>=<handle>` in `internal/model`
(pure; no cmd/tmpl/toolgen import) that RETIRES `review_origin` (clean break, no
compat shim), plus `plan_origin`/`audit_origin` for the plan-audit author/auditor
pair; enforce a cross-stage distinctness lattice at the owning authority seams
(plan gate `EvaluatePlanGate`, review authority `evaluateReviewAuthorityWithPolicy`,
ship authority `buildShipAuthorityFromReadiness`), pairwise-distinct, fail-closed on
standard/strict and advisory on light; drive each independence-critical judgment
through a dedicated host-native subagent on the SHARED change worktree (the Go
engine does NOT fork headless processes); register reason codes across the
four-file contract with actionable remediations naming the owning stage; update the
generated skill templates + docs; no bypass/force-close/self-stamp.

RESCOPE (2026-06-17 — SUPERSEDES the earlier "parallel-pair, no new host"
amendment): S3 review is a VARIABLE PARALLEL SET of independent fresh-context
reviewers, not a fixed pair:
- Mandatory, always, all concurrent: `spec-compliance-review`,
  `code-quality-review`, AND `independent-review` (promoted from the embedded
  base-reader contract to a first-class standalone S3 host).
- Optional: `security-review` (promoted from a technique/checklist skill to a
  standalone S3 host), AUTO-SELECTED by the engine from change signals; when
  selected it joins the SAME parallel fan-out.
- Min 3, max 4 reviewers. Every dispatched reviewer is its own fresh-context
  native subagent emitting `context_origin:stage=review=<handle>`; the cross-stage
  lattice proves mutual distinctness across the DISPATCHED set; the review-authority
  gate requires EVERY dispatched reviewer to pass; an absent optional reviewer
  (security not selected) is silent / non-blocking. Fail-closed on standard/strict,
  advisory on light.

## Complexity Assessment
complex (escalated)
<!-- Rationale: all of the prior #240 complex rationale (pure model grammar, three
     authority seams, four-file reason-code/recovery contract, generated templates,
     docs, fail-closed dogfood) PLUS promoting two skills (independent-review,
     security-review) to first-class standalone S3 hosts, generalizing the
     review-authority lattice from a fixed 4-participant set to a COMPUTED 5-6
     participant set, and adding signal-driven auto-selection of an optional
     reviewer. Still modifies Slipway's own fail-closed independence enforcement, so
     it MUST fail closed with no bypass and be dogfooded through its own strict flow. -->

## Guardrail Domains
None as a SAST/classification domain (not Auth/Credentials/PII/Financial/Schema/
Irreversible/External-API). Nonetheless sensitive in the fail-closed sense
(identical to #239): it modifies Slipway's own review/independence enforcement, so
per CLAUDE.md "Review And Safety" it must fail closed and introduce no bypass,
force-close, or private attestation path. Dogfooded through its own strict flow.

## In Scope
Carry the t-01..t-08 on-disk foundation (the `context_origin` grammar +
`plan_origin`/`audit_origin` parsers + `CrossStageContextCollisions` lattice helper
in `internal/model`; the plan/review/ship seam wiring in `advance_governed.go`,
`authority.go`, `readiness.go`; the four-file reason-code contract; the
fixed-pair parallel routing; the docs/templates retiring `review_origin`) as the
BASE, then layer the review-SET generalization on top:

- `internal/engine/progression/skill_resolution.go`: `ResolveNextSkill` at
  `S3_REVIEW` returns a COMPUTED reviewer set — mandatory
  `{spec-compliance-review, code-quality-review, independent-review}` plus
  `security-review` when the auto-selection signal fires — replacing the fixed
  `{spec, code}` pair. The set is a concurrent parallel-dispatch contract
  (min 3 / max 4); S0/S1/S2/S4 stay single-skill.
- Promote `independent-review` to a MANDATORY standalone S3-dispatchable host: a
  generated host template that runs the base independent-reader contract as its OWN
  fresh-context review and emits `context_origin:stage=review=<handle>`; its own
  dispatch and its own lattice participant.
- Promote `security-review` to an OPTIONAL standalone S3-dispatchable host: the same
  host wiring, conditionally dispatched per the auto-selection rule; emits its own
  `context_origin:stage=review=<handle>`.
- `internal/engine/progression/authority.go`: the review-authority lattice
  participants grow from `{executor, audit_origin, spec, code}` to add `independent`
  (always) and `security` (when dispatched). The gate requires every DISPATCHED
  reviewer to pass and to carry a distinct well-formed handle; an absent optional
  security reviewer is silent (owned by selection, not a missing-skill blocker);
  fail-closed on `EffectivePreset != light`, advisory on light. The cross-stage edge
  set generalizes from the fixed pair to the computed participant set.
- Security auto-selection: the engine derives whether `security-review` is dispatched
  from change signals (guardrail_domain / preset / blast_radius / changed-file path
  heuristics). The EXACT signal set, combination, and fail-closed default is a
  research unknown (see Open Questions).
- `internal/tmpl/templates/skills/...` + `internal/tmpl/templates_test.go`: add
  `independent-review` + `security-review` S3 host templates (native-subagent
  dispatch on the shared worktree, emitting `context_origin:stage=review=`);
  generalize the concurrent-fan-out language and `docs/design.md` + `docs/workflow.md`
  from a 2-review pair to the N-reviewer set.
- New canonical reason codes if the variable-set gate / selection needs them, across
  the four-file contract (`internal/model/{reason_code.go, reason_code_contract_test.go,
  recovery.go, recovery_test.go}`), each error severity + actionable remediation
  naming the owning stage to re-run.
- Tests across all of the above: routing returns the mandatory trio + conditional
  security; lattice pass-with-distinct / fail-on-collision / fail-on-missing-handle
  across a variable N; absent-optional-silent; advisory-on-light; auto-selection
  fires / does-not-fire per signal.

## Out of Scope
- #239's P1 (chain ordering) and P4 (#5 test!=impl, #6 degraded_dispatch) gate LOGIC
  — kept as-is; #240 only consumes the executor handle P4 already records.
- Per-stage / per-executor worktree isolation (rejected in #239; independence comes
  from the native-subagent boundary on the SHARED worktree, not the filesystem).
- The Go engine forking headless `claude -p` processes or spawning leaf subagents
  (#239 rejected; the engine stays the sole inline verdict-stamper consuming
  host-emitted tokens).
- Cryptographic / non-forgeable distinct-context discrimination (the disclosed
  residual; lattice stays audit/structural tier).
- intake-clarification and research-orchestration independence (backstopped by the
  user-confirmation handshake; not lattice participants).
- A new CLI flag or `VerificationRecord` struct field — all handles ride
  `References` via the existing `--reference` flag.
- Explicit-opt-in selection for security (considered and NOT chosen this round —
  auto-derive selected instead); making security mandatory or adding a 5th+ reviewer
  (security stays optional; max 4).

## Constraints
- The engine remains the SOLE inline verdict-stamping authority; handles are tokens
  the gate CONSUMES on `VerificationRecord.References`. Hosts/subagents must not
  self-stamp freshness or final verdicts.
- `internal/model` stays pure (stdlib-only; no cmd/tmpl/toolgen import), enforced by
  `dependency_direction_test.go`.
- New gates fail closed at error severity on `standard`/`strict`; advisory (nil) on
  `light`.
- No backward-compat scaffolding for the retired `review_origin` token; no bypass,
  force-close, or private attestation path.
- Use the current worktree's Slipway CLI as source of truth; dogfood through this
  change's own strict governed flow — it must satisfy its own new variable-set
  lattice gate with at least three distinct concurrent reviewer handles.

## Acceptance Signals
- The generalized `context_origin` grammar + `plan_origin`/`audit_origin` parsers
  exist in pure `internal/model` and fail closed on ambiguous/conflicting handles;
  `review_origin` is fully removed (no references in code, templates, or docs).
- On standard/strict, each lattice seam fails closed with a named reason code +
  actionable remediation when a participant handle is missing or collides; advisory
  on light. Proven at the plan gate, review authority, and ship authority.
- plan-audit fails closed when `audit_origin` is missing or equals `plan_origin`;
  passes when both present and distinct.
- At `S3_REVIEW`, `ResolveNextSkill` returns the mandatory trio
  `{spec-compliance-review, code-quality-review, independent-review}` and ADDS
  `security-review` when the auto-selection signal fires; tests prove both the
  3-reviewer and 4-reviewer dispatch.
- All dispatched reviewers run as CONCURRENT fresh-context native subagents from a
  single fan-out point, each recording a distinct `context_origin:stage=review=`
  handle; the review-authority gate passes only when every dispatched reviewer passes
  with mutually-distinct handles, fails closed on a missing/colliding handle, and
  treats absent optional security as silent. Tests + the change's own dogfood prove it.
- `independent-review` and `security-review` exist as generated standalone S3 host
  templates emitting `context_origin` tokens; toolgen/template contract tests green.
- New reason codes registered across the four-file contract; nothing downgrades to
  `unknown_reason_code`.
- docs describe the variable review-set, its concurrent dispatch, and the lattice
  trust tier + honest residual (structural tier, not cryptographic).
- `go test ./...`, `gofmt -s -l`, golangci-lint all clean.
- The change ships through its own strict governed flow with fresh dogfood evidence.

## Open Questions
<!-- genuine design unknowns that route to S0_INTAKE/research -->
- [x] Cross-stage distinctness lattice semantics with the executor → RESOLVED
  (research.md Axis 1, Model A set-disjointness): executor joins as the SET of all
  recorded non-empty task-handles; each stage handle must be absent from that set;
  executor-internal distinctness stays owned by #239 P4; empty set silent; advisory
  on light. CARRIES OVER unchanged.
- [x] Seam ownership of the lattice edges → RESOLVED (research.md Axis 2,
  earliest-resolvable-seam keyed on `max(stage(a),stage(b))`): plan gate owns the 1
  local edge; review authority owns the executor/audit_origin/spec/code edges; ship
  authority owns the goal/closeout edges. CARRIES OVER, but the review-seam
  participant set now EXPANDS (see the variable-participant unknown below).
- [x] (SUPERSEDED) S3 parallel-review as a FIXED pair, "Option B / no new host" →
  SUPERSEDED by this rescope. `independent-review` is now a mandatory standalone host
  and `security-review` an optional standalone host, so S3 is a variable SET, not a
  fixed pair; the "no new host" resolution no longer holds and is replaced by the
  unknowns below.
- [x] security-review auto-selection signal contract → RESOLVED by research.md Q1
  Option B and formalized in decision.md/requirements.md: which signals (guardrail_domain,
  preset, blast_radius, changed-file path heuristics such as auth/crypto/session),
  how they combine into a fail-closed dispatch decision, the default when signals are
  ambiguous, and whether a sensitive-but-guardrail-None change (like #240 itself) is
  correctly caught or deliberately not. The APPROACH is settled (auto-derive from
  signals); the precise signal contract routes to research.
- [x] Variable-participant review-authority lattice semantics → RESOLVED by
  research.md Q2 Option A + R2 and formalized in decision.md/requirements.md:
  how the gate enumerates
  the DISPATCHED reviewer set (3 mandatory + optional security), proves mutual
  distinctness across a variable N, requires every dispatched reviewer to pass, treats
  absent-optional security as silent WITHOUT double-firing `required_skill_missing`,
  and stays fail-closed — generalizing the fixed `{executor, audit_origin, spec, code}`
  edge set to a computed participant set.
- [x] independent-review's dual role → RESOLVED by research.md Q3 Option (i)
  de-embed and formalized in decision.md/requirements.md: today it is the base reader EMBEDDED in the
  spec/quality hosts. As a standalone mandatory reviewer, do spec/quality STOP
  embedding it (so the three reviews are genuinely distinct dimensions — spec-trace vs
  engineering-quality vs unanchored-correctness/safety) or keep the base contract while
  independent adds a distinct unanchored read? Resolve the template content + the
  distinctness so the three mandatory reviewers are not three copies of the same read.

## Deferred Ideas
- A structural build/CI guard that fails if a delegating surface is nested inside an
  isolated/forked execution (gsd bug-936 analogue) — noted in #239, still future.
- Engine-issued per-stage nonce / lifecycle-event boundary for true non-forgeable
  discrimination (#239 "Option B") — infeasible within constraints, the disclosed
  residual.

## Approved Summary
Generalize #240's cross-stage independence work so that S3 review is a VARIABLE
PARALLEL SET of independent fresh-context reviewers instead of a fixed pair. On top
of the locked #240 foundation (the chain-wide `context_origin:stage=<stage>=<handle>`
grammar retiring `review_origin`, the `plan_origin`/`audit_origin` plan-audit pair,
and the pairwise-distinctness lattice owned at the plan/review/ship seams —
fail-closed on standard/strict, advisory on light, every judgment driven through a
host-native subagent on the shared worktree), this change:

- Promotes `independent-review` from an embedded base-reader to a MANDATORY
  standalone S3 host, and `security-review` from a technique skill to an OPTIONAL
  standalone S3 host.
- Makes `ResolveNextSkill` at S3 return a COMPUTED reviewer set — mandatory
  `{spec-compliance-review, code-quality-review, independent-review}` plus
  `security-review` when AUTO-SELECTED from change signals — dispatched as concurrent
  native subagents from one fan-out point (min 3, max 4), each emitting a distinct
  `context_origin:stage=review=<handle>`.
- Generalizes the review-authority lattice from `{executor, audit_origin, spec, code}`
  to a computed set adding `independent` (always) + `security` (when dispatched); the
  gate requires EVERY dispatched reviewer to pass with mutually-distinct handles,
  treats absent optional security as SILENT, and stays fail-closed (advisory on light).

Key boundaries: security is auto-derived from signals (NOT explicit opt-in); security
stays optional (max 4 reviewers); all handles ride `References` (no new CLI flag or
struct field); no bypass/force-close/self-stamp; `internal/model` stays pure.

Out of scope (named): per-stage worktree isolation, the engine forking headless
processes, cryptographic non-forgeable discrimination, intake/research independence,
and explicit-opt-in security selection.

Primary acceptance signal: at S3, `ResolveNextSkill` returns the mandatory trio and
adds security when the signal fires; all dispatched reviewers run concurrently with
distinct `context_origin` handles; the review-authority gate passes only when every
dispatched reviewer passes with mutually-distinct handles and fails closed on a
missing/colliding handle — proven by tests AND the change's own strict dogfood.

The Open Questions above are resolved by `research.md`, `decision.md`,
`requirements.md`, and `tasks.md`; future changes to the security selector,
variable-participant lattice, or standalone review hosts must update those plan
artifacts together.
