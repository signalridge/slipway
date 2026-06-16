# Concerns

Re-authored for change
`add-an-engine-consumed-context-origin-fresh-context-attestat`.

- Forgeability ceiling: the engine can never PROVE the emitting context differs
  from the authoring context. Every host-supplied token enters through one
  channel — `model.VerificationRecord.References []string`
  (internal/model/verification.go) — and is structurally identical to the
  already-dead `closeout:reviewer_independence=pass` literal, which today has NO
  consumer in the engine. Its only structurally-identical LIVE precedent is
  `closeout:assurance_complete=pass` (`assuranceCompleteReference`,
  authority.go), which `closeoutAssuranceAttestationBlockers` (authority.go)
  merely checks for presence. Any new attestation must be labelled
  "distinct-context-asserted + cross-stage-consistent", never "proven"; a
  "proven" claim ships a false security guarantee.
- RunVersion does not discriminate context: the four independence skills
  (spec-compliance-review, code-quality-review, goal-verification,
  final-closeout) are all `RunSummaryBound: true` in skill.go
  `defaultGovernanceRegistry`, so a single run stamps an IDENTICAL `RunVersion`
  on all of them (engine-owned, set in cmd/evidence.go). RunVersion equality is
  a same-run binding, not a freshness-of-context signal.
- Timestamp monotonicity is a weak byproduct, not a discriminator: the ordering
  checks in `closeoutGoalVerificationReuseReviewBlocker` and the goal/closeout
  comparisons in `closeoutGoalVerificationReuseBlockers` (authority.go) catch
  wrong-ORDER serial evidence; they cannot distinguish one context emitting four
  records from four contexts. P1's always-on chain-ordering gate inherits the
  same ceiling.
- session_id is an invalid discriminator: `TaskEvidencePayload.SessionID`
  (wave_sync.go) is host-supplied and `omitempty`, and the empty default is
  excluded — it only powers a soft `session_isolation_warning` (wave_sync.go
  ~:485), never a gate. P4 must base #5 on engine-owned `task_kind`+`target_files`
  and NOT promote session_id to a hard signal.
- False-positive deadlock / perverse incentive: a hard executor-handle
  distinctness gate (extending `ExecutorAgentBlockers`, wave_sync.go) would block
  a legitimate single fresh review host while rewarding a forger who emits two
  fake handles. Distinctness must require two non-empty DISTINCT handles for the
  review pair under parallel dispatch, but must NOT punish an honest shared
  review host — fail-closed only where the parallel claim was actually made.
- Mid-flight migration dead-end: active standard/strict changes carry no new
  token, so every new gate (P1 chain-order, P2 parallel-review) fails them
  closed. CLAUDE.md forbids bypass/force-close/private attestation, so each new
  blocker's remediation (`internal/model/recovery.go` blockerRemediations) MUST
  name the exact owning skill to re-run — closeout names final-closeout, ordering
  names the lagging reviewer — or the change strands with no public recovery.
- Loop-safety vs the digest/freshness reopen cascade: re-running the owning skill
  re-stamps timestamps, which feeds back into the freshness/digest checks
  (`skillDigestFreshnessBlockersWithSummary`, ExecutionSummaryFreshness in
  authority.go). The new ordering gate must converge — a single re-run of the
  named skill must clear it without re-triggering an upstream reopen — and that
  convergence must be test-proven.
- JSON/Lattice external-contract creep: a token carried in `References` is purely
  additive and safe. Adding a NEW field to `VerificationRecord` is a one-way
  external JSON + toolgen contract commitment (the struct is `json`-tagged and
  surfaced) and should be avoided; keep the attestation as a References literal.
- Reason-code three-file contract: new reason codes must be registered in all
  three of `reason_code.go` canonicalReasonDefinitions, the
  reason_code_contract_test.go snapshot list, and the severity map, plus an
  `internal/model/recovery.go` remediation. `NewReasonCode` (reason_code.go)
  SILENTLY downgrades an unregistered code to `unknownReasonCode` — a missing
  registration produces a generic blocker with no remediation, not a build
  failure.
- Honest tier labelling: P2 executor-agent handles and parallel dispatch_mode are
  HOST-emitted, so they are audit/structural-tier evidence, not cryptographic
  proof. Option B (engine-issued nonce / lifecycle-event boundary) is the
  documented residual (P3) — do not let docs or reason-code messages imply the
  handles deliver true context discrimination.
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
