# Research

Discovery for the engine-consumed context-origin / fresh-context attestation
change. Question: how can the engine bind an independence-critical verdict's
context-origin (distinct-from-authoring) when it can only consume tokens the
host emits via `slipway evidence`, and a single context can emit any token?

The headline finding reframes the objective: within Slipway's host-honored
evidence model the originally-scoped token mechanism is **largely audit-only,
not enforcement**, because the only per-record engine-owned anchors either do
not discriminate one context from four (RunVersion) or are a free byproduct of
in-order serial execution (Timestamp). Genuine discrimination requires an
engine-*issued* anchor (a per-stage nonce or a lifecycle-event boundary) that
neither design originally sized. This is a decision the approved scope did not
anticipate and is surfaced for selection at the research HARD-GATE.

## Alternatives Considered

### Architecture
- **Single inbound channel.** The only seam by which the engine consumes a
  verdict's content is `model.VerificationRecord.References []string`
  (`internal/model/verification.go:21`), read off each per-skill
  `verification/<skill>.yaml`. Every existing attestation rides it:
  `closeout:assurance_complete=pass` (`authority.go:237-243`),
  `closeout:goal_verification_reuse[_run_version=<N>]` (`authority.go:263-266`,
  parsed `:417-434`), `layer:<L>=pass` review tokens (`authority.go:548-549` →
  `parseReviewLayerOutcomes :630-651`). A new token MUST ride this channel —
  there is no other inbound path.
- **Verdict producers consumed at the gate.** The four records are keyed by
  skill name in one `map[string]VerificationRecord`: spec-compliance-review
  `authority.go:94`, code-quality-review `:100`, goal-verification `:279`,
  final-closeout `:233/:259`. Review tokens (#1/#3) belong in
  `evaluateReviewAuthorityWithPolicy` (`authority.go:77-115`); verify/closeout
  tokens (#2/#4) + the four-way chain check belong in
  `buildShipAuthorityFromReadiness` (`authority.go:133-205`), where both
  `reviewAuthority.PassingSkills` and `verifyPassingSkills` are in scope
  (`:156`) — a unified `#serial` gate is wiring-feasible.
- **Engine-owned anchors the host cannot set.** `VerificationRecord.Timestamp`
  = `time.Now().UTC()` and `RunVersion` are stamped by the engine at evidence
  write (`cmd/evidence.go:166-167`); for run-summary-bound skills `RunVersion`
  is forced to the execution-summary `LatestRunVersion`
  (`cmd/evidence.go:634-635`). Task evidence rejects future `captured_at`
  (`cmd/evidence.go:453-464`) and forces `freshness_inputs` to
  `state.ExpectedExecutionTaskFreshnessInputs` (`:517`). The host controls only
  the `References` strings.
- **Blast radius.** New gates: `internal/engine/progression/authority.go`
  (#1-#4 + #serial), `wave_sync.go` (#5/#6). New reason codes:
  `internal/model/reason_code.go`. Recovery remediation:
  `internal/model/recovery.go` (sibling to `:496-504`). Producer + generated
  surfaces: `cmd/evidence.go`, `internal/tmpl/templates/skills/*`,
  `internal/toolgen`. Layering invariant: `internal/architecture/
  dependency_direction_test.go:14-69` forbids `internal/model` and
  `internal/state` from importing `cmd`/`tmpl`/`toolgen` — any token
  grammar/parser placed in `model` must stay pure (matches `wave_execution.go`).

### Patterns
- **Pattern A — presence attestation** (`closeoutAssuranceAttestationBlockers`,
  `authority.go:229-250`): early-return nil when preset not required → look up
  owning record → linear-scan `References` for an exact const string → missing
  record == missing attestation. Adequate for a "token present" floor; **not
  tamper-evident** (a single context emits the string).
- **Pattern B — cross-stage binding** (`closeoutGoalVerificationReuseBlockers`,
  `authority.go:252-359`): the stronger analogue. It does not trust the token's
  truth value; it cross-checks the host claim against engine-owned anchors —
  `RunVersion` equality across goal/closeout/execution-summary (`:285,:290,
  :300`), `Timestamp` ordering review ≤ goal ≤ closeout and goal ≥ latest
  execution evidence (`:305-318`, `:338-359`), freshness == Fresh and per-skill
  digest freshness (`:319-334`). This is the realistic shape for a binding gate.
- **Existing context-identity primitive.** `TaskEvidencePayload.SessionID`
  (`wave_sync.go:35`, set from `--session-id` at `cmd/evidence.go:518,:582`) is
  consumed today only as an advisory `session_isolation_warning` when >1 task
  shares a session (`wave_sync.go:476-487`, `reason_code.go:509-512`,
  Warning-severity). **Caveat: `--session-id` exists only on `evidence task`,
  not `evidence skill`** — the four review/verify/closeout verdicts carry no
  origin field at all.
- **Preset gating.** `assuranceRequired := inputs.Policy.EffectivePreset !=
  model.WorkflowPresetLight` (`authority.go:154`); the comment at `:146-153`
  documents why `CloseoutRefreshRequired` is the wrong signal. Ship-stage
  blockers are dual-appended to `verifySkillBlockers` (`:158`) AND `unresolved`
  (`:181`) so `EvaluateGShip` emits the specific code, not generic
  `verification_evidence_missing`.
- **Reason-code + recovery registration is a three-file, contract-tested
  contract**: `reason_code.go` `canonicalReasonDefinitions` map +
  `reason_code_contract_test.go` `canonicalReasonCodeSnapshot()` list and
  severity map + `recovery.go` `blockerRemediations`. `model.NewReasonCode`
  silently downgrades an unregistered code to `unknown_reason_code`
  (`reason_code.go:713-715`), so registration is load-bearing. Tests must assert
  on `.Code`/`.Detail`, never `.Message` (AST lint
  `reason_code_contract_test.go:357-377`).

### Risks
- **Forgeability ceiling (HIGH, inherent).** The engine cannot prove the
  emitting context differs from the authoring context. A new
  `closeout:context_origin=<id>` token is structurally identical to the already-
  ignored `closeout:reviewer_independence=pass`
  (`final-closeout/SKILL.md.tmpl:136`, zero Go consumers, verified). Any claim
  must be scoped to "distinct-context-**asserted** + cross-stage-consistent",
  never "distinct-context-**proven**, or it ships a false guarantee = a bypass
  dressed as a gate.
- **RunVersion does not discriminate (HIGH, verified, design-breaking).** All
  four skills are `RunSummaryBound:true` (`internal/engine/skill/skill.go:53-86`)
  so `RunVersion` is *identical* across them when execution is not re-run
  (`cmd/evidence.go:634-635`). RunVersion-equality cross-checks catch a
  stale/mismatched run, **not** a same-run serial burst.
- **Timestamp monotonicity is a free byproduct (HIGH, verified).** Sequential
  `slipway evidence skill` calls naturally advance `time.Now()`, so
  review ≤ goal ≤ closeout is satisfied by an in-order single-context burst.
  Timestamp ordering distinguishes wrong-order from right-order, **not** one
  context from four. The serial-collapse attacker is right-order by definition.
- **session_id is host-supplied and empty-default-excluded (HIGH, verified).**
  `wave_sync.go:478` requires `sessionID != ""` before grouping, and
  `--session-id` is optional. So a #5 check keyed on session_id is bypassed two
  ways: empty default (silently excluded) or two distinct host-chosen ids (no
  collision). It punishes the honest empty-default case and rewards forgery.
- **False-positive deadlock / perverse incentive (HIGH).** A hard id-distinctness
  gate blocks a *legitimate* single fresh review host that honestly stamps one
  id across spec-compliance + code-quality, while a dishonest author emitting
  four distinct fake ids passes — teaching hosts to fabricate. Distinctness must
  not block honest-shared ids.
- **Mid-flight migration (HIGH).** Active standard/strict changes at G_ship have
  no origin token and could not have set one; the gate fails them closed the
  instant it ships. CLAUDE.md forbids bypass/force-close, so recovery is
  re-running the owning stage — the remediation MUST name the exact skill or it
  is a recovery dead-end. Loop-safety against the digest/freshness reopen cascade
  is unverified (`stale_evidence_recovery.go` not traced).
- **JSON/Lattice (MEDIUM).** Token-in-`References` is fully additive (safest).
  A new `VerificationRecord`/`ExecutionTaskSummary` struct field changes the
  external contract + toolgen Arguments contract test (`toolgen.go:200`); avoid.
- **Reversibility (LOW).** Token-in-references + a dedicated blocker is cheap to
  revert (delete the call + recovery entry); no data migration. A persisted
  struct field is a one-way schema commitment baked into archived bundles.

### Test Strategy
- **Clonable templates exist.** `TestCloseoutAssuranceAttestationBlockers`
  (`authority_test.go:22-64`, unit) + `TestBuildShipAuthority…PresetGating`
  (`:75-199`, 3-subtest preset table asserting fail-closed-standard /
  surfaced-in-G_ship / advisory-on-light) are the exact shape for the new
  gates. `TestCloseoutGoalVerificationReuseBlockers` (`:201-345`) is the
  blueprint for cross-stage Timestamp/RunVersion assertions.
- **Contract-test burden.** Every new code touches `reason_code_contract_test.go`
  (snapshot list + severity map), `recovery_test.go` (three completeness tests
  `:74-129` + `sampleRecoveryDetail` `:249-318`), and the `.Message`-prose AST
  lint. Template token tests are literal-string
  (`templates_test.go:148-165`), one per emitted token across the 5 templated
  skills.
- **Net-new infra for #5/#6 advisory-on-light.** `SyncGovernedWaveExecution`
  is preset-agnostic (`wave_sync.go:77-79`); safety-net blockers are appended
  unconditionally (`:189-206`). Making #5/#6 advisory on light requires threading
  `governance.ResolvePresetPolicy().EffectivePreset` into the assembly point —
  there is **no** preset-parametrized wave_sync test to clone.
- **Verification approach per acceptance signal** (intent.md:42-49): a pure-
  function unit test + an integration test through the owning entry point
  (`buildShipAuthorityFromReadiness` for #2/#4/#serial,
  `evaluateReviewAuthorityWithPolicy` for #1/#3, `SyncGovernedWaveExecution`
  for #5/#6) asserting durability in `summary.OpenBlockers`. Full-suite
  `go test ./...` (27 pkgs) + `gofmt -s -l` + golangci-lint are explicit signals.

### Options
Three viable directions emerged. They differ in **what the gate genuinely
enforces** vs **what it merely records for audit**, and in scope.

- **Option A — Honest hybrid (recommended).** Ship the genuinely-enforced part
  and label the audit-only part honestly.
  - *Part 1 (REAL enforcement):* give the dead `reviewer_independence` token a
    real engine consumer by **promoting the cross-stage ordering already proven
    in `closeoutGoalVerificationReuseReviewBlocker` (`authority.go:338-359`)
    from the opt-in reuse branch to an always-on chain-ordering gate** — closeout
    must not predate goal; goal must be at/after latest review evidence — at
    error severity on standard/strict (`EffectivePreset != light`), dual-surfaced,
    with a *distinct* new reason code (never folded into
    `closeout_goal_verification_reuse_invalid`). This catches the cheapest real
    cheat (wrong-order: closeout authored before review) with proven machinery.
  - *Part 2 (REVIEW FORCE-PARALLEL, audit/structural tier):* instead of inventing
    a new `context_origin` token, REUSE the proven wave-dispatch machinery for the
    spec-compliance-review / code-quality-review pair — require a parallel
    `dispatch_mode` plus two DISTINCT recorded `executor_agent` handles (analogues
    of `DispatchEvidenceBlockers`/`ExecutorAgentBlockers`, `wave_sync.go:824-914`),
    fail-closed on standard/strict, advisory on light. The per-review handle IS the
    review's context id, so it SUBSUMES a separate context_origin token. The two
    reviews are UNORDERED peers (both ≤ goal); verify and closeout stay ORDERED
    after the pair (P1). HONEST: handles are host-emitted, so this is
    audit/structural-tier distinct-context evidence (raises forging cost +
    auditability), NOT cryptographic proof; distinctness must scope the collision to
    the review pair's handles and must not deadlock a legitimate shared review host.
  - *Part 3 (documented residual):* record that true non-forgeable discrimination
    requires an engine-issued per-stage nonce / lifecycle-event boundary (Option
    B); accept the residual explicitly so the gate is not oversold.
  - *Demotion:* drop `session_id` as the #5 discriminator; base #5 on engine-owned
    `task_kind`+`target_files` structure, or defer #5/#6 to a separate change.
  - Tradeoffs: smallest honest change; ships only what is enforced; no false
    guarantee; no honest-flow deadlock. Does **not** fully close the serial-
    collapse (correctly-ordered single-context burst remains forgeable, by design).

- **Option B — Engine-issued nonce / lifecycle-event boundary (true
  discrimination).** Add the one mechanism that actually discriminates one
  context from four: the engine *issues* a per-stage token only after the prior
  stage's verification record is sealed (or binds the verdict Timestamp to
  post-date the prior *stage-transition event*, not just the prior evidence
  write), and the producing context must echo it on the next `slipway evidence`.
  Because the value is engine-generated, not host-chosen text, a back-to-back
  single-context burst cannot satisfy it without genuinely re-entering each
  stage across the engine boundary.
  - Tradeoffs: the only path to genuine tamper-evidence; larger; likely needs new
    persistence (nonce store / event-boundary read) and more research (feasibility
    was dismissed, not sized — see Unknowns); higher risk of false-positive and
    schema commitment. Closes the residual Option A documents.

- **Option C — Re-scope / split.** Ship Option-A Part 1 now (the proven, real
  win) as *this* change; spin context-origin attestation (#1-#4 + #serial,
  Option B's nonce) and #5/#6 into a separately-researched follow-up.
  - Tradeoffs: keeps this change small, fully-enforced, and shippable through its
    own strict flow; honors "smallest clean design"; defers the hard part to a
    change that can research the nonce properly. Cost: the original four-verdict
    objective is only partially delivered now.

- **Selected: Option A — honest hybrid. LOCKED by user 2026-06-16.** Ship the
  genuinely-enforced ordering gate, realize the distinct-context signal by reusing
  wave dispatch (not a new token), document the unreachable residual, and demote
  `session_id`. Four parts:
  - **P1 (REAL enforcement) — always-on chain-ordering gate.** Promote the
    cross-stage ordering to an always-on gate: `closeout ≥ goal ≥ max(spec-
    compliance, code-quality)`. Fresh-verify detail: this ordering today is gated
    behind the opt-in `closeout:goal_verification_reuse=pass` token — the parent
    `closeoutGoalVerificationReuseBlockers` hard early-returns at
    `authority.go:263-265` unless that token is present. The ordering lives in TWO
    places that must BOTH move out from behind that guard: the review ≤ goal half in
    the helper `closeoutGoalVerificationReuseReviewBlocker` (`:338-359`, called at
    `:311`) and the closeout ≥ goal half at `:314-318`. Both currently funnel into
    `closeout_goal_verification_reuse_invalid` (`:445-447`), so P1's DISTINCT new
    reason code means rewriting the blocker constructor, not just relocating the
    calls. Error severity on standard/strict (`EffectivePreset != light`), advisory
    on light, dual-surfaced. This finally gives the dead
    `closeout:reviewer_independence=pass` token a real engine consumer and catches
    the cheapest real cheat (wrong-order: closeout or verify authored before review).
  - **P2 (distinct-context signal) — review force-parallel via a NEW review-context
    handle grammar (ADAPT the wave-dispatch PATTERN, do not reuse its code).**
    Fresh-verify finding: the existing `executor_agent:wave=<int>:task=<id>` grammar
    (`internal/model/wave_execution.go:326,363-389`) is structurally wave/task-bound
    and its consumer `ExecutorAgentBlockers` iterates `plan.Waves[].Tasks[].TaskID`
    against the S2 wave-orchestration record only — a review skill record has no wave
    index or task ids, so it CANNOT carry the existing handle. What is reusable is the
    PATTERN, not the functions: introduce a NEW pure-`internal/model` grammar (e.g.
    `review_origin:skill=<skill>=<handle>` + parser, named to avoid colliding with the
    existing unrelated `review_context` JSON object on the next/handoff surface
    (`cmd/next.go:119`) and kept pure per
    `dependency_direction_test.go:14-69`) and a NEW blocker consumed in
    `evaluateReviewAuthorityWithPolicy` (`authority.go:77-115`), NOT in wave_sync.
    Require two DISTINCT review-context handles across the spec-compliance /
    code-quality PAIR, fail-closed standard/strict, advisory light. The handle IS the
    per-review context id → SUBSUMES a separate context_origin token (do not invent a
    second one). The two reviews are UNORDERED peers (both ≤ goal). HONEST: handles
    are host-emitted → audit/structural tier, NOT cryptographic proof; distinctness
    must not deadlock a legitimate shared review host.
  - **P3 (documented residual) — Option B deferred.** True non-forgeable
    discrimination (an engine-issued per-stage nonce / lifecycle-event boundary) is
    INFEASIBLE within this change's constraints (proven below) and is recorded as an
    explicit residual for a future change, so the gate is never oversold.
  - **P4 (#5/#6 in scope, demote `session_id`).** Keep #5/#6 in this change. #5:
    base the test≠impl distinctness on engine-owned `task_kind`+`target_files`, NOT
    host-supplied `session_id` (host-set + empty-default-excluded at
    `wave_sync.go:478` → invalid discriminator). #6: tighten dispatch so a bare
    `degraded_sequential` is accepted only when paired with a genuine tool-unavailable
    signal. Fresh-verify detail: this is grammar-level work, NOT a one-line edit at
    `wave_sync.go:844` — `:844` accepts ANY valid token via `mode.IsValid()` and
    degraded_sequential passes only because `WaveDispatchMode.IsValid()`
    (`wave_execution.go:271-273`) returns true for it. The honest, additive path is a
    NEW justification REFERENCE token read in a degraded-specific branch of
    `DispatchEvidenceBlockers`, not a new struct field (research flags struct fields as
    MEDIUM Lattice/JSON risk). Also: #235 added a `SyncGovernedWaveExecution` call in
    `makeEvidenceSkillCmd` (`cmd/evidence.go:242`), so a tightened
    `DispatchEvidenceBlockers` now ALSO fires on the `evidence skill` path, not only on
    advance/next — the plan must account for this extra trigger site.
- **Verify is NOT parallel with review.** The force-parallel scope is review∥review
  only (same-stage S3 peers). goal-verification (S4) and final-closeout (S4) stay
  ORDERED after the review pair via P1's chain (`closeout ≥ goal ≥ max(review)`).
  goal-verification has no data dependency on the review verdicts, so it *could* run
  concurrently — but the ordering IS the enforcement: parallelizing verify with
  review would erase the `goal ≥ review` half of the chain and defeat the
  serial-collapse closer.
- **Why not Option B now (proven INFEASIBLE within constraints).** Option B is the
  only path to genuine tamper-evidence, but every host-honored anchor fails to
  discriminate one context from four, and the approved constraints (no leaf-subagent
  spawn, engine stays sole stamper, no new `VerificationRecord` struct field,
  `References` is the only inbound channel) leave no escape: (a) all four
  independence skills are `RunSummaryBound` (`skill.go:53-86`) → identical
  `RunVersion` (`cmd/evidence.go:634-635`); (b) `Timestamp` monotonicity is a free
  byproduct of in-order serial evidence — catches wrong-order, not
  one-context-vs-four; (c) the only zero-schema nonce (the prior stage's
  `skill.evidence_recorded` `EventID` in `events/lifecycle.jsonl`) is host-readable
  plaintext in the worktree — a single shell can `cat` it and echo `seal:<skill>=
  <uuid>` in the SAME burst, proving only post-seal plaintext read, not a separate
  fresh context; (d) the four verdicts collapse onto two inter-stage transitions
  (spec+code ∈ S3, goal+closeout ∈ S4) and recording a verdict never crosses a
  transition, so a lifecycle-event boundary only catches wrong-order. Genuine
  distinct-from-authoring discrimination needs an out-of-band secret /
  leaf-subagent isolation / cryptographic host-identity the host cannot pre-read —
  all out of scope.
- **Option C** (split: ship P1 only, defer the rest) remains the fallback if the
  user later restricts this change to the proven ordering gate; NOT selected — P2
  and P4 are kept in scope.

### Fresh re-verification (2026-06-16)
A 5-agent adversarial workflow re-verified this research's load-bearing code claims
against worktree HEAD `c7a828d` (post-#235). Verdict: all substantive premises HOLD —
the dead `reviewer_independence` token (zero Go consumers), the opt-in-only cross-stage
ordering (P1 premise), the RunSummaryBound-shared RunVersion (Option-B-infeasibility
premise), and the unconditional `degraded_sequential` acceptance are all confirmed.
#235 touched only `wave_sync.go` (`ResumeWaveIndexFromTaskEvidence`/
`singleTaskEvidenceRunVersion`) and added a `SyncGovernedWaveExecution` call in
`cmd/evidence.go:242`; it changed NOTHING in the P1–P4 enforcement surfaces. Two
mechanism refinements were folded into Selected P2/P4 and the Canonical References:
(1) P2 ADAPTs the wave-dispatch *pattern* (a NEW review-context grammar) — it does NOT
literally reuse the wave/task-bound `executor_agent` machinery (the Options-section
Part 2 wording "reuse" is superseded by Selected P2); (2) P4 #6 is grammar-level work
(a NEW justification reference token), not a `:844` edit, and now also fires on the
`evidence skill` path via #235's added call site. All other corrections were
line-number drifts, fixed inline in the Canonical References above.

## Unknowns
- Resolved: *Can the engine bind context-origin via host-emitted tokens?* →
  Only as **audit-trail visibility**, not enforcement. The host-honored model's
  forgeability ceiling is real (verified: RunVersion shared across the four
  RunSummaryBound skills; Timestamp monotonicity a free serial byproduct;
  session_id host-supplied + empty-excluded). Genuine discrimination needs an
  engine-issued anchor.
- Resolved: *What is the strongest genuinely-enforced win available now?* →
  Promoting the proven cross-stage **ordering** check (`authority.go:338-359`)
  to always-on, which catches wrong-order forgery and gives `reviewer_independence`
  a real consumer.
- Resolved (now closed against Option B): **engine-issued nonce / lifecycle-event-
  boundary feasibility** — the `internal/state` lifecycle event log WAS evaluated
  as an anchor and rejected within constraints. The only zero-schema nonce, the
  prior stage's `skill.evidence_recorded` `EventID` in `events/lifecycle.jsonl`, is
  host-readable plaintext in the worktree (`status_view_build.go:287,301` also
  surface it), so a single context can read it and echo it in the same burst; and
  the four verdicts collapse onto two inter-stage transitions (spec+code ∈ S3,
  goal+closeout ∈ S4), so a stage-transition timestamp only catches wrong-order.
  Conclusion: a non-forgeable engine-issued nonce is unreachable WITHOUT a
  leaf-subagent spawn / out-of-band secret / new persisted field — all out of
  scope. Option B is the documented residual (Selected P3); not pursued here.
- Remaining: **honest-shared-context topology** — is one fresh review host
  producing both spec-compliance and code-quality in one session an allowed
  topology? If yes, distinctness must be scoped to authoring-vs-review, not
  review-vs-review.
- Remaining: **producer surface fork** — token-in-`--reference` (no new flag,
  engine validates nothing) vs a new `--context-origin` flag on `evidence skill`
  (`cmd/evidence.go:298-303` has none today; adding one edits the toolgen
  Arguments contract test).
- Remaining: **mid-flight migration loop-safety** — prove re-recording the four
  verdicts clears the new blocker without a freshness/digest reopen cascade
  (`stale_evidence_recovery.go` untraced).
- Remaining: **#5/#6 scope + preset wiring** — defer, or accept net-new preset
  threading into `SyncGovernedWaveExecution`; if #5 ships, replace session_id
  with engine-owned `task_kind`+`target_files` distinctness.

## Assumptions
- The engine remains the sole inline verdict-stamping authority; the attestation
  is a token the gate consumes — Evidence: `cmd/evidence.go:166-167` is the only
  Timestamp/RunVersion writer; intent.md Constraints.
- Fail-closed on standard/strict, advisory on light, keyed on
  `EffectivePreset != WorkflowPresetLight` — Evidence: `authority.go:154`,
  `:146-153`.
- A token in `References` is additive and needs no schema change; a struct field
  would be a one-way Lattice/JSON commitment — Evidence:
  `verification.go:16-23` (`references,omitempty`), `toolgen.go:200`.
- New reason codes require the three-file registration or contract tests fail —
  Evidence: `reason_code.go:708-727`, `reason_code_contract_test.go:19-39`,
  `recovery_test.go:74-129`.
- The codebase map was authored for a prior change (`eliminate-non-native-hook-
  and-skill-script-runtime-dependenc`) and is semantically stale for this
  change's scope; its ARCHITECTURE/CONCERNS/TESTING/STRUCTURE sections are
  re-authored inline for the progression-engine scope before being relied upon —
  Evidence: `artifacts/codebase/ARCHITECTURE.md` header.

## Canonical References
(Line numbers fresh-verified against worktree HEAD `c7a828d`, post-#235, 2026-06-16.)
- `internal/model/verification.go:16-23` — VerificationRecord; `References` (`:21`,
  `references,omitempty`) is the sole *structured* host-controlled channel (`Notes`
  `:22` is also host-supplied free text); Timestamp/RunVersion engine-owned
- `internal/engine/progression/authority.go:210,229-243` — Pattern A
  (`closeoutAssuranceAttestationBlockers`; token loop `:237-242`; `:245-250` is the
  separate `…MissingBlocker` helper)
- `internal/engine/progression/authority.go:252-336,417-434` — Pattern B
  (`closeoutGoalVerificationReuseBlockers`; RunVersion `:285/:290/:300`; Timestamp
  ordering `:305-318`). P1 promotion targets are the SEPARATE helper
  `closeoutGoalVerificationReuseReviewBlocker` `:338-359` (review ≤ goal, called at
  `:311`) + the closeout ≥ goal half `:314-318`, both behind the opt-in guard
  `:263-265`, both funnelling into `closeout_goal_verification_reuse_invalid` `:445-447`
- `internal/engine/progression/authority.go:77-115,133-205` — review vs ship
  authority placement (P2's new review-context blocker belongs in
  `evaluateReviewAuthorityWithPolicy` `:77-115`); `:154` preset predicate;
  attestation dual-surfaced into verifySkillBlockers `:158` + unresolved `:180`
- `internal/engine/progression/wave_sync.go:476-487` — session_isolation_warning
  (advisory, task-only, empty-id-excluded at `:478`)
- `internal/engine/progression/wave_sync.go:824-853` — DispatchEvidenceBlockers
  (`:844` accepts ANY valid dispatch token via `mode.IsValid()`; degraded_sequential
  passes only because `WaveDispatchMode.IsValid()` returns true — #6 is grammar-level,
  not a `:844` tweak); `ExecutorAgentBlockers` `:864-914`
- `internal/model/wave_execution.go:271-273,326,363-389` — `WaveDispatchMode.IsValid`;
  `executor_agent:wave=<int>:task=<id>` grammar is wave/task-bound (consumer iterates
  `plan.Waves[].Tasks[]`) → P2 needs a NEW review-context grammar, cannot reuse this
- `internal/engine/skill/skill.go:57-90` — the four independence skills are
  `RunSummaryBound:true` (`:61` spec, `:69` code, `:77` goal, `:86` closeout →
  RunVersion shared → cannot discriminate); `:53` wave-orchestration is a fifth
- `cmd/evidence.go:166-167,517,634-635` — engine-owned stamping; future captured_at
  rejected in `parseEvidenceTaskCapturedAt` `:963-965` (call site `:453-462`);
  `:298-303` skill flags (no --context-origin/--session-id; --session-id is
  `evidence task`-only `:582`); `:242` #235's added `SyncGovernedWaveExecution` call
  (safety-net gates now fire on `evidence skill` too)
- `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl:136` — dead
  `reviewer_independence` token (zero Go consumers, grep-confirmed)
- `internal/model/reason_code.go` (`canonicalReasonDefinitions` map `:40-701`;
  `NewReasonCode` downgrade-to-unknown `:712-714`) + `internal/model/
  reason_code_contract_test.go` (snapshot helper `:41`, severity `:211`,
  `.Message`-prose AST lint `:357-377`) + `internal/model/recovery.go:496-504`
  (`blockerRemediations` sibling) + `internal/model/recovery_test.go:74-129`
  (completeness), `:249-326` (`sampleRecoveryDetail`)
- `internal/architecture/dependency_direction_test.go:14-69` — model/state must
  not import cmd/tmpl/toolgen
- `internal/engine/progression/authority_test.go:22-64` (unit), `:75-199`
  (3-subtest preset-gating), `:201-345` (cross-stage) — clonable tests
- `internal/tmpl/templates_test.go:148-165` — final-closeout attestation
  literal-token test (4 tokens `:161-164`); add a sibling per new emitted token
- `cmd/status_view_build.go:301` — surfaces prior stage's
  `skill.evidence_recorded` EventID (`:287` is the `ReadLifecycleEvents` call) →
  why an Option-B nonce is host-readable
- `internal/toolgen/toolgen.go:200` — evidence command `Arguments` registry value
  (NOT a test; struct field on `CommandDef` `:138`); a new evidence flag edits `:200`
