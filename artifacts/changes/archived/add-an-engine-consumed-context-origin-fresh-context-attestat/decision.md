# Decision

## Alternatives Considered

Three whole-change directions were weighed in `research.md`, plus several
mechanism-level sub-decisions. The deciding axis is **what the gate genuinely
enforces vs what it merely records for audit** inside Slipway's host-honored
evidence model, where `model.VerificationRecord.References []string` is the only
inbound channel the engine consumes and the host can place any string on it.

- **Option A — honest hybrid (SELECTED, user-LOCKED 2026-06-16).** Ship the
  genuinely-enforced ordering gate (P1), realize a distinct-context signal by
  adapting the wave-dispatch *pattern* into a new review-context handle grammar
  (P2), keep the S2 hardening (P4 #5/#6), and document the unreachable residual
  (P3). Smallest honest change; ships only what is enforced; no false guarantee;
  no honest-flow deadlock. It does not fully close a correctly-ordered
  single-context burst — accepted by design and disclosed.

- **Option B — engine-issued per-stage nonce / lifecycle-event boundary
  (REJECTED, infeasible within constraints).** The only mechanism that truly
  discriminates one context from four. Rejected after fresh verification because
  every host-honored anchor fails: all four independence skills are
  `RunSummaryBound` so `RunVersion` is identical; `Timestamp` monotonicity is a
  free byproduct of in-order serial evidence (catches wrong-order, not
  one-context-vs-four); the only zero-schema nonce (the prior stage's
  `skill.evidence_recorded` EventID in `events/lifecycle.jsonl`) is host-readable
  plaintext that one shell can `cat` and echo in the same burst; and the four
  verdicts collapse onto two inter-stage transitions, so an event boundary again
  only catches wrong-order. Genuine discrimination would need an out-of-band
  secret, leaf-subagent isolation, or cryptographic host identity the host cannot
  pre-read — all explicitly out of scope (intent.md forbids leaf-subagent spawn;
  the engine stays the sole stamper). Recorded as the documented residual (P3).

- **Option C — re-scope / split (REJECTED, fallback only).** Ship P1 now and
  spin P2/P4 into a separately-researched follow-up. Not selected: the user kept
  P2 and P4 in scope. Retained as the fallback if the change is later restricted
  to the proven ordering gate.

Mechanism-level sub-decisions (all SELECTED to minimize external-contract blast
radius and keep `internal/model` pure):

- **Producer surface: token-in-`References` via the existing `--reference` flag**
  (selected) over a new `--context-origin` / `--review-context` CLI flag
  (rejected). A new flag would edit the toolgen `Arguments` contract and the
  external Lattice command surface; a token on `References` is purely additive.

- **P2 grammar: a NEW pure `internal/model` review-context grammar** (selected)
  over reusing the `executor_agent:wave=<int>:task=<id>` grammar (rejected). The
  executor-agent grammar is wave/task-bound and its consumer iterates
  `plan.Waves[].Tasks[]` against the S2 wave-orchestration record; a review-skill
  record carries no wave index or task id, so it cannot ride the existing handle.
  Adapt the pattern, not the code.

- **P1 reason codes: distinct, new codes** (selected) over folding into
  `closeout_goal_verification_reuse_invalid` (rejected). The ordering must fire
  even when the reuse opt-in token is absent, so it cannot share the reuse
  blocker's identity or its early-return guard.

- **Review-pair distinctness scope: per-review-record handles** (selected) — this
  resolves the open "honest-shared-context topology" question. The handle is a
  per-review-record identifier, so any honest host (two parallel review contexts,
  or one fresh review host labelling each pass distinctly) can always emit two
  distinct handles; the gate only catches the degenerate signature of one uniform
  handle stamped on both reviews (or a missing handle). This keeps the gate from
  deadlocking a legitimate review topology while still flagging the cheapest
  authoring-context collapse.

- **#5 discriminator: engine-owned `task_kind` + `target_files`** (selected) over
  the host-supplied `session_id` (rejected). `session_id` is host-set and
  excluded on the empty default, so it both punishes honest empty-default runs and
  rewards two forged distinct ids — an invalid discriminator.

- **#6 mechanism: a NEW additive justification reference token** (selected) over
  a new struct field (rejected: MEDIUM Lattice/JSON one-way schema risk) or a
  `wave_sync.go:844` edit (rejected: `:844` accepts any token via
  `mode.IsValid()`; `degraded_sequential` is valid, so the fix must be a pairing
  requirement, not a validity tweak).

## Selected Approach

Implement **Option A** as four parts, wiring each new gate into the authority/
wave-sync seam that already owns the relevant records, with every new token
riding `References` and every new grammar staying pure in `internal/model`.

- **P1 — final-closeout independence becomes engine-consumed.** In ship-authority
  evaluation (`buildShipAuthorityFromReadiness`), (a) require
  `closeout:reviewer_independence=pass` to be present on the final-closeout record
  (Pattern A presence), and (b) promote the cross-stage ordering halves —
  `review ≤ goal` (`closeoutGoalVerificationReuseReviewBlocker`) and
  `closeout ≥ goal` — out from behind the opt-in
  `closeout:goal_verification_reuse=pass` early-return guard into an always-on
  invariant `closeout ≥ goal ≥ max(spec-compliance, code-quality)`. Each facet
  fails closed at error severity on standard/strict (`EffectivePreset != light`),
  is advisory on light, and is dual-surfaced into the ship gate so the specific
  code (not a generic `verification_evidence_missing`) is emitted. P1 rewrites the
  blocker constructor so the ordering carries its own distinct reason code rather
  than the reuse code.

- **P2 — review-context handle pair.** Introduce a new pure `internal/model`
  review-context grammar (a `review_origin:` reference token + parser — named to
  avoid colliding with the existing unrelated `review_context` JSON object on the
  next/handoff surface) and a new blocker consumed in
  `evaluateReviewAuthorityWithPolicy`. Require both
  spec-compliance-review and code-quality-review to record a handle, and require
  the two handles to differ; missing or identical handles fail closed on
  standard/strict, advisory on light. The handle is the per-review context id and
  subsumes any separate `context_origin` token. The two reviews remain unordered
  peers; the handle gate imposes no ordering between them (ordering is P1's job).

- **P3 — documented residual.** Record in artifacts and docs that true
  non-forgeable distinct-context discrimination (Option B) is infeasible within
  this change's constraints, so the handle gate is presented as audit/structural
  tier (raises forging cost and auditability), never as cryptographic proof.

- **P4 — S2 hardening.** #5: in `SyncGovernedWaveExecution`, enforce that for
  shared `target_files` a `task_kind=test` task is structurally distinct from and
  dispatched before its dependent `task_kind=code` task, derived only from
  engine-owned `task_kind` + `target_files` (never `session_id`). #6: in
  `DispatchEvidenceBlockers`, accept `degraded_sequential` only when paired with
  the new tool-unavailable justification reference token. Both fail closed on
  standard/strict and are advisory on light, which requires threading the
  resolved preset policy into the otherwise preset-agnostic
  `SyncGovernedWaveExecution`. Because #235 added a `SyncGovernedWaveExecution`
  call on the `evidence skill` path, the tightened dispatch gate now fires there
  as well as on advance/next.

Every new blocker registers a distinct canonical reason code across the
three-file contract (`reason_code.go`, `reason_code_contract_test.go`,
`recovery.go`) with an actionable remediation that names the owning skill to
re-enter. Generated skill templates, thin-host content, toolgen output, and docs
emit and explain the new tokens.

## Interfaces and Data Flow

- **Inbound channel (unchanged shape):** all new tokens ride
  `VerificationRecord.References` via the existing `--reference` flag. No new CLI
  flag, no new `VerificationRecord` / `ExecutionTaskSummary` struct field, so the
  external Lattice JSON and toolgen `Arguments` contract stay additive-only.
- **New pure grammar in `internal/model`:** a `review_origin:` token (+parser)
  and a degraded-sequential justification token (+parser), each kept free of any
  `cmd`/`tmpl`/`toolgen` import per the dependency-direction invariant; siblings
  of the existing `wave_execution.go` token grammar.
- **New engine gates:** P1 presence + ordering in `authority.go`
  (`buildShipAuthorityFromReadiness`); P2 handle distinctness in
  `evaluateReviewAuthorityWithPolicy`; P4 #5/#6 in
  `wave_sync.go` (`SyncGovernedWaveExecution` / `DispatchEvidenceBlockers`).
- **Preset flow:** `EffectivePreset` already reaches authority evaluation; for
  #5/#6 advisory-on-light the resolved preset policy is newly threaded into
  `SyncGovernedWaveExecution`.
- **Outbound surfaces:** generated skill templates emit the new tokens; docs and
  thin-host content document them. No behavioral change for the `light` preset
  beyond advisory findings.

## Rollout and Rollback

- **Rollout:** ships through this change's own strict governed flow (dogfood). The
  new gates are fail-closed on standard/strict from first landing; `light` users
  see advisory findings only.
- **Mid-flight migration:** active standard/strict changes at G_ship that predate
  the tokens will fail closed the instant the gate lands; per CLAUDE.md there is
  no bypass/force-close — recovery is re-running the owning stage, and every new
  remediation names the exact skill to re-enter (final-closeout for P1,
  spec/code review for P2, wave-orchestration for P4). Loop-safety against the
  freshness/digest reopen cascade is verified during implementation.
- **Rollback:** cheap and migration-free. Because the change adds only
  tokens-in-`References`, dedicated blockers, reason codes, and template strings —
  no persisted struct field and no data migration — rollback is deleting the
  blocker call sites, recovery entries, reason codes, and emitted tokens. No
  archived bundle is schema-bound to the new fields.
- **Verification command:** `go test ./...` (all packages) plus `gofmt -s -l` and
  golangci-lint clean; dogfood evidence recorded through the strict flow.

## Risk

- **Forgeability ceiling (inherent, disclosed).** Handles are host-emitted, so P2
  is audit/structural tier, not cryptographic proof; a correctly-ordered
  single-context burst with two fabricated distinct handles still passes. Mitigated
  by honest scoping in artifacts/docs (P3) and by P1 catching the cheaper
  wrong-order cheat.
- **Mid-flight fail-closed on existing changes.** Highest operational risk;
  mitigated by remediations that name the exact owning skill and by verifying the
  re-record path clears the blocker without a reopen cascade.
- **False-positive deadlock.** Avoided by making P2 distinctness per-review-record
  (always emittable by an honest host) and by advisory-on-light; #5 keyed only on
  engine-owned structure so it cannot punish the honest empty `session_id` case.
- **Preset threading into `wave_sync.go` is net-new.** No preset-parametrized
  wave-sync test exists to clone, so #5/#6 advisory-on-light needs fresh test
  infrastructure; the extra `evidence skill` trigger site (from #235) must be
  covered.
- **External-contract drift (Lattice).** Held to additive-only by riding
  `References` and adding no struct field or CLI flag; toolgen `Arguments`
  contract test must stay green.
- **Layering violation.** New grammar must not import `cmd`/`tmpl`/`toolgen`;
  enforced by `internal/architecture/dependency_direction_test.go`.
