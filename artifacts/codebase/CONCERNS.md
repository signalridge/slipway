# Concerns

Re-authored for change
`feat-governance-host-native-subagent-enforced-cross-stage-in` (#240).

Baseline: #239 (engine-consumed reviewer-independence + context-origin/`review_origin`
attestation, closeout chain-ordering, and the wave dispatch/executor gates) has
SHIPPED (commit 2d2adac, 0.26.0); this change builds on that baseline and partly
retires it, replacing `review_origin` with the chain-wide `context_origin` lattice.

- Forgeability ceiling: the engine can never PROVE the emitting context differs
  from the authoring context. Every host-supplied token enters through one
  channel — `model.VerificationRecord.References []string`
  (internal/model/verification.go). `context_origin:stage=review=<handle>` and
  the goal/closeout stage tokens are structural assertions compared for
  cross-stage consistency; they are not non-forgeable context proofs. Any new
  attestation must be labelled "distinct-context-asserted +
  cross-stage-consistent", never "proven"; a "proven" claim ships a false
  security guarantee.
- RunVersion does not discriminate context: the selected review hosts,
  goal-verification, and final-closeout are all `RunSummaryBound: true` in
  skill.go `defaultGovernanceRegistry`, so a single run can stamp an identical
  engine-owned `RunVersion` on all of them. RunVersion equality is a same-run
  binding, not a freshness-of-context signal.
- Timestamp monotonicity is a weak byproduct, not a discriminator: the selected-set
  chain-order checks catch wrong-order serial evidence; they cannot distinguish
  one context emitting every review, goal, and closeout record from genuinely
  separate contexts. The always-on chain-order gate inherits the same ceiling.
- session_id is an invalid discriminator: `TaskEvidencePayload.SessionID`
  (wave_sync.go) is host-supplied and `omitempty`, and the empty default is
  excluded — it only powers a soft `session_isolation_warning` (wave_sync.go
  ~:485), never a gate. Review-set independence must stay anchored to the
  selected reviewers' `context_origin` references plus engine-owned evidence
  timestamps, not session identifiers.
- False-positive deadlock / perverse incentive: the selected-review lattice must
  fail closed only for selected reviewers. Unselected security evidence on disk can
  be stale, colliding, or absent without becoming a hidden lattice participant;
  selected-but-missing security is handled by required-skill blockers. The R2
  skill-keyed map is what keeps duplicate reviewer handles visible while avoiding
  a fake set-valued `review` participant that could dedupe collisions.
- Mid-flight migration dead-end: active standard/strict changes with only the old
  review evidence now fail closed until the mandatory independent reviewer and any
  selected security reviewer record current `stage=review` handles. CLAUDE.md
  forbids bypass/force-close/private attestation, so each blocker's remediation
  (`internal/model/recovery.go` blockerRemediations) must name a public skill
  re-run path and rely on engine-stamped evidence.
- Loop-safety vs the digest/freshness reopen cascade: re-running the owning skill
  re-stamps timestamps, which feeds back into the freshness/digest checks
  (`skillDigestFreshnessBlockersWithSummary`, ExecutionSummaryFreshness in
  authority.go). Selected-set ordering must converge — a re-run of the stale
  selected reviewer, followed by goal verification and final closeout when needed,
  must clear the blocker without re-triggering an upstream reopen — and that
  convergence must stay test-proven.
- JSON/Lattice external-contract creep: a token carried in `References` is purely
  additive and safe. Adding a NEW field to `VerificationRecord` is a one-way
  external JSON + toolgen contract commitment (the struct is `json`-tagged and
  surfaced) and should be avoided; keep the attestation as a References literal.
- Reason-code four-file contract: new reason codes must be registered in
  `reason_code.go` canonicalReasonDefinitions, the
  reason_code_contract_test.go snapshot list, the severity map, and
  `internal/model/recovery.go` remediations, with produced blockers covered by
  `recovery_test.go`. `NewReasonCode` (reason_code.go) silently downgrades an
  unregistered code to `unknownReasonCode` — a missing registration produces a
  generic blocker with no remediation, not a build failure.
- Honest tier labelling: selected-review `stage=review` handles, executor-agent
  handles, and dispatch claims are host-emitted, so they are audit/structural-tier
  evidence, not cryptographic proof. Option B (engine-issued nonce /
  lifecycle-event boundary) remains the documented residual — do not let docs or
  reason-code messages imply the handles deliver true context discrimination.
- Preset fail-closed invariant: every new gate must be ERROR severity on
  standard/strict and advisory on light, keyed on
  `policy.EffectivePreset != model.WorkflowPresetLight` (the existing pattern in
  authority.go, e.g. assuranceRequired). A new code defaulting to the wrong
  severity in the severity map would silently neuter the guarantee.
- Layering invariant: new vocab/constants go in internal/model (kept pure) and
  new gate logic in internal/engine/progression;
  internal/architecture/dependency_direction_test.go forbids internal/model and
  internal/state from importing cmd/tmpl/toolgen, so the attestation literals and
  reason codes must not pull engine or command imports into the model package.
