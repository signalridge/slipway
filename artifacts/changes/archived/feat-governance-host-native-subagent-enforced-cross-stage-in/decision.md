# Decision

## Alternatives Considered

This decision supersedes the stale fixed-pair plan. The on-disk foundation has
already generalized the old `review_origin` grammar into the chain-wide
`context_origin` lattice and has already made spec-compliance-review and
code-quality-review unordered S3 peers. The remaining design is the review-SET
rescope: S3 must become a computed set of three mandatory reviewers plus one
optional security reviewer, with all selected reviewers emitting the same
`context_origin:stage=review=<handle>` wire token while remaining distinct
lattice participants.

- **Q1 security auto-selection. Option B: engine-derived control (SELECTED).**
  Add `ControlSecurityReview` to the existing control derivation pipeline. The
  decision is computed from model inputs plus engine-derived governance signals:
  a security-relevant guardrail domain (`auth_authz`,
  `security_credentials`, `privacy_pii`, `external_api_contracts`), high blast
  radius, or strict preset plus medium-or-higher blast radius. It reuses the
  existing `DeriveControls` blast-radius fallback that degrades missing data to
  medium rather than treating unknown as low. The control mode follows preset
  policy: advisory on light, blocking on standard/strict when selected.
  - **Option A: pure model predicate only (REJECTED).** A pure helper on
    `model.Change` can see guardrail domain and preset but cannot see the
    engine-derived governance blast radius without importing `engine/control`,
    which would create an import cycle and put policy logic in the wrong layer.
  - **Option C: path/name heuristics inside routing (REJECTED).** Routing-level
    auth/crypto/session path matching would duplicate selection outside the
    control pipeline, drift from requiredness, and make the optional reviewer
    appear or disappear for reasons not visible in governance health.

- **Q2 variable review-set semantics. Option A plus R2: registry-conditional
  security and skill-keyed participants (SELECTED).** Add
  independent-review as an unconditional S3 registry entry and security-review
  as a conditional S3 registry entry. Compute one selected-review-skill slice:
  mandatory spec/code/independent plus security when `ControlSecurityReview` is
  selected. Use that one slice for routing, required-skill filtering,
  review-authority participants, owned-stage keys, ship-authority base
  participants, and closeout chain order. Every reviewer records the literal
  `context_origin:stage=review=<handle>` wire token, but the lattice participant
  key is the recording skill name. This keeps `CrossStageContextCollisions`,
  `participantsCollide`, and `ContextParticipant` unchanged.
  - **Option B: persist the selection decision on a derived field (REJECTED).**
    Persisting selection could avoid recomputation, but it risks stale state when
    task targets or blast radius change. The existing codebase already computes
    required skills from live change state and governance policy.
  - **Option C: always require security and suppress it later (REJECTED).**
    Always requiring security would fire `required_skill_missing:security-review`
    on ordinary changes and then require a second suppressing gate, splitting
    truth across surfaces and making the optional reviewer feel like a bypass.

- **Shared `stage=review` token implementation. R2 skill-keyed feeder
  (SELECTED).** Preserve the user-selected wire contract that every reviewer
  emits `context_origin:stage=review=<handle>`, but never key the lattice map by
  that shared stage. The authority feeder creates one single-handle participant
  per selected review record keyed by skill name, reading
  `handles[StageContextReview]` from `passingSkills`. The review owned-set is
  built from the same selected skill-name slice, so intra-review edges are owned
  and cannot be skipped. The pure model helper stays byte-for-byte stable except
  for adding the `StageContextReview` constant.
  - **R1: make `review` a handle set (REJECTED).** A set-valued review node
    would silently dedupe duplicate reviewer handles unless paired with a
    contributor count, creating exactly the kind of fail-open invariant this
    change is meant to remove.
  - **R3: add a new review-set collision primitive (REJECTED).** A new model type
    and reason code would be larger than needed. Existing single-handle pairwise
    collision already proves distinctness once participants are keyed by skill.

- **Q3 independent-review role. Option (i): de-embed and make it standalone
  (SELECTED).** Remove independent-review's host-embedded bindings from
  spec-compliance-review and code-quality-review, keep its command-auto report
  schema binding, and author it as a first-class S3 host. The three mandatory
  dimensions become spec trace, engineering quality, and cold independent
  correctness/safety. Security-review likewise becomes a first-class conditional
  S3 host instead of a checklist embedded in both reviews.
  - **Option (ii): keep base-reader embeddings and add a standalone host
    (REJECTED).** That gives three nominal hosts but keeps the same base read in
    multiple places, undermining the distinct-reviewer intent.
  - **Option (iii): de-embed reads but share a generated verdict partial
    (DEFERRED).** This is a fallback if verdict-shape drift appears; the current
    engine-stamped verification record already gives the shared evidence shape.

## Selected Approach

Implement the review-SET rescope on top of the existing context-origin
foundation with five linked changes.

1. **Keep model vocabulary pure and minimal.** Add only
   `StageContextReview = "review"` to `internal/model/context_attestation.go`
   and pin it in tests. Do not add `StageContextIndependent` or
   `StageContextSecurity`; those are skill-name participant keys owned by the
   authority feeder. Keep `CrossStageContextCollisions`,
   `participantsCollide`, `ContextParticipant`, `plan_origin`, `audit_origin`,
   and executor handle-set behavior unchanged. Preserve `review_origin`
   retirement assertions.

2. **Introduce one engine-derived security selector.** Add
   `ControlSecurityReview` to `internal/model/control.go`, control config,
   preset overrides, actions/health surfaces, and tests. Extend
   `internal/engine/control.DeriveControls` so the selector fires for the
   chosen SAST-domain subset, high blast radius, or strict+medium blast radius.
   Keep #240 itself deliberately unselected by the domain arm because its
   `guardrail_domain` is empty and its security sensitivity is governance
   fail-closed sensitivity, not a SAST-classified code surface. Its actual
   security-review selection still follows the shared blast-radius / strict
   preset threshold.

3. **Route and require one selected review set.** Add `independent-review` and
   conditional `security-review` to `defaultGovernanceRegistry` as
   `StateS3Review` definitions. Extend required-skills filtering with a
   `securitySelected` input, mirroring `closeoutRequired` and the existing
   workflow-profile code-quality filter. Change S3 routing to return
   `{spec-compliance-review, code-quality-review, independent-review}` plus
   `security-review` when the same selector is true. Any caller that presents
   "next skill" must handle the multi-skill set and avoid recreating
   spec-before-code ordering.

4. **Re-key the review lattice feeder to skill names.** In `authority.go`, build
   review participants and review-owned endpoints from the same selected review
   skill slice. Review participant records must come from `passingSkills`, not
   raw disk, so unselected-but-present security evidence cannot become a hidden
   lattice participant. For each selected review record, parse
   `ContextOriginHandlesFromVerification(record)[StageContextReview]` and add a
   participant keyed by the recording skill name. Missing selected records stay
   owned by `required_skill_missing`; selected passing records with no
   `stage=review` handle fail with `context_origin_handle_invalid`; collisions
   fail with `cross_stage_context_not_distinct`. Ship authority and
   `closeoutChainOrderBlockers` consume the same selected set.

5. **Promote hosts and update public surfaces.** Convert independent-review and
   security-review into workflow-owned generated S3 host templates. De-embed the
   independent base-reader and security checklist bindings from spec/code review
   hosts, preserving command-auto report-schema/checklist behavior where still
   appropriate. Update toolgen descriptors, generated skill tests, template
   token tests, docs, command fixtures, and dogfood tests to prove the mandatory
   trio and selected-security quartet both work.

## Interfaces and Data Flow

- **Inputs.** `model.Change` still carries guardrail domain, workflow preset,
  workflow profile, and planned task target files. Governance control derivation
  computes blast radius from `tasks.md` target files before execution and from
  execution summaries after execution; no new `VerificationRecord` field or CLI
  flag is added.

- **Security selector.** `control.DeriveControls` emits
  `ControlSecurityReview` with `ScopeReview` when selected. The selector is then
  threaded into review routing and required-skill filtering from the same
  governance/readiness path that already derives preset policy and runtime
  controls. The selector is not recomputed from raw verification files.

- **Registry and routing.** `RequiredSkillsForStateWithRegistry` receives the
  security-selected boolean and skips conditional security-review when false.
  `ResolveNextSkill` returns the selected S3 review slice rather than a fixed
  pair. S0, S1, S2, and S4 remain effectively single-skill states except for
  existing machine-only steps.

- **Evidence.** All selected review hosts record evidence through
  `slipway evidence skill --reference "context_origin:stage=review=<handle>"`.
  Plan-audit continues to record `plan_origin` and `audit_origin`. Executor
  evidence continues to use the existing `executor_agent` grammar. Timestamps,
  run versions, digests, and verdicts stay engine-owned.

- **Authority.** Review authority receives `passingSkills` from
  `EvaluateRequiredSkillsForChange`. That map is the only source of review
  participants, which makes optional security silent when unselected and blocking
  when selected-but-missing. The plan-audit `audit_origin` remains loaded from
  the plan-audit record because it is not an S3 reviewer. Ship authority merges
  selected review participants with goal/final evidence and evaluates only
  ship-owned goal/closeout edges plus selected-review chain order.

- **Public surfaces.** `next`, `validate`, `status`, `evidence`, `review`,
  stale-evidence recovery, toolgen export, generated skills, and docs all expose
  the same selected review set. A conventional primary skill may remain only for
  code paths that explicitly need a single representative, such as stale
  ordering, and must not imply S3 reviewer ordering.

## Rollout and Rollback

- **Rollout.** Ship through the governed strict workflow for this change. The
  current on-disk context-origin foundation remains in place; the review-SET
  layer is added before S2 execution resumes so planned blast radius and
  required-review selection are evaluated from the rescope plan. Standard and
  strict changes fail closed when selected review evidence is missing, malformed,
  colliding, stale, or out of order. Light keeps advisory behavior where the
  underlying gates already allow it.

- **Mid-flight behavior.** Existing active changes with only spec/code evidence
  will need to run independent-review, and security-review when selected, before
  S3 can pass. This is intentional fail-closed behavior. Recovery must route
  through public skill execution and engine-stamped evidence only; there is no
  force-close, self-stamp, or private bypass.

- **Rollback.** Rollback removes `ControlSecurityReview`, the conditional S3
  registry entry, the mandatory independent-review registry promotion, the
  skill-keyed review feeder, the two workflow-owned review templates, and the
  docs/tests for the variable review set. The existing context-origin foundation
  can remain if rollback is scoped to the rescope layer; a full #240 rollback
  would also remove the earlier lattice foundation and restore the pre-#240
  review model. Because all new evidence rides `References`, rollback has no
  schema migration.

- **Verification command set.** Focused checks: `go test ./internal/model
  ./internal/engine/control ./internal/engine/skill
  ./internal/engine/progression ./internal/tmpl ./internal/toolgen ./cmd`.
  Full checks: `go test ./...`, `gofmt -s -l .`, and `golangci-lint run` when
  available. Governance checks: rebuild the CLI to `/tmp`, run
  `validate --json`, record fresh plan-audit evidence with `plan_origin` and
  `audit_origin`, then advance only after explicit user confirmation.

## Risk

- **Owned-set re-key fail-open (HIGH).** If review participants are keyed by
  skill name but `ownedStages` still uses old model stage constants, intra-review
  edges have no owned endpoint and duplicate review handles silently pass.
  Mitigation: derive participants and owned endpoints from the same selected
  review skill slice and add a regression where two selected reviewers share the
  same `stage=review` handle.

- **Participant-source fail-open (HIGH).** If review participants are loaded
  from raw disk, an unselected but still-present `security-review.yaml` can be
  resurrected into the lattice. Mitigation: source selected review participants
  only from `passingSkills`, whose keys are already filtered by requiredness, and
  add an unselected-security-on-disk regression.

- **Selection divergence (HIGH).** If routing, required-skills filtering,
  authority, and closeout ordering compute security selection separately, the
  system can route-but-not-require, require-but-not-route, or check an unrequired
  reviewer. Mitigation: one selected-review-skill helper or one passed decision
  feeds all consumers, and tests compare surfaces.

- **Layering violation (HIGH).** Blast radius is engine-derived, not a pure
  `model.Change` field. Moving selection into `internal/model` would either drop
  the strongest signal or import `engine/control` into `model`. Mitigation:
  compute `ControlSecurityReview` in `internal/engine/control` and keep
  `internal/model` limited to vocabulary.

- **Review dimensional collapse (MEDIUM).** Promoting independent-review while
  keeping its base-reader procedure embedded in spec/code would produce three
  hosts that perform overlapping reads. Mitigation: de-embed the base-reader
  bindings and tests assert spec/code templates no longer carry that procedure.

- **Public surface confusion (MEDIUM).** Existing `next` and command tests still
  contain a primary-review convention and some spec-before-code fallbacks.
  Mitigation: update the public surfaces to show the selected review set while
  preserving a clearly named primary helper only where a single representative is
  genuinely needed.

- **Reason-code drift (MEDIUM).** The R2 design should not need new codes, but a
  new diagnostic code introduced during implementation would degrade to
  `unknown_reason_code` without the four-file contract. Mitigation: keep the
  reason-code task explicit and fail tests on any unregistered producer.

- **Codebase-map relevance (LOW).** The durable codebase map was authored for
  the fixed-pair foundation and is stale for the review-SET rescope. Mitigation:
  use live code anchors and `research.md` for review-SET planning, and record the
  map relevance as an advisory in plan-audit notes rather than treating the map
  as authoritative for new scope.
