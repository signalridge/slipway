# Requirements

### Requirement: Defer assurance.md creation past S1_PLAN
REQ-001: The engine MUST NOT scaffold `assurance.md` during S1_PLAN bundle
creation. Like the #119 deferred artifacts (`requirements.md`, `decision.md`,
`tasks.md`, `research.md`), `assurance.md` MUST be authored by the owning host
skill rather than seeded with a passing placeholder body the skill must overwrite.

#### Scenario: bundle scaffolding skips assurance.md on standard/strict
GIVEN a governed change on the standard or strict effective preset
WHEN the engine scaffolds the governed bundle via ScaffoldGovernedBundleForChange
THEN no assurance.md file is written to the bundle directory, and the planning bundle (requirements/decision/tasks per schema) is unaffected

#### Scenario: light preset stays a no-op
GIVEN a governed change on the light effective preset where assurance.md is not required
WHEN the engine scaffolds the governed bundle
THEN assurance.md is absent, unchanged from prior behavior

### Requirement: A missing assurance.md must not block progression before S3_REVIEW
REQ-002: Because `assurance.md` is a review/verify-phase deliverable, its existence
MUST be enforced solely by the dedicated `AssuranceContractBlockers` gate at
`S3_REVIEW` and later. The generic pre-S3 bundle/readiness existence gates
(`GovernedBundleBlockers` and the artifact-readiness evaluator) MUST NOT emit
`missing_required_artifact:assurance.md` and MUST NOT strand a change at S1_PLAN or
S2_EXECUTE solely because `assurance.md` has not yet been authored. This closes the
gap the source issue left open: deferring creation alone would leave a required
`assurance.md` missing at S1 and block plan-audit. The `slipway repair` / doctor
bundle-consistency diagnostic (`DiagnoseBundleConsistency`) is the same kind of
pre-S3 surface and MUST stay silent on a deferred (absent) `assurance.md` before
`S3_REVIEW` rather than reporting the by-design deferral as a partial-write
inconsistency.

#### Scenario: plan-audit advances without an authored assurance.md
GIVEN a standard-preset change whose requirements.md and tasks.md are authored and valid, and assurance.md has not been authored because it is deferred to S3
WHEN the change is audited and advanced through S1_PLAN and S2_EXECUTE
THEN no missing_required_artifact:assurance.md blocker is raised before S3_REVIEW and progression is not blocked on the absence of assurance.md

#### Scenario: behavior is uniform across discovery and non-discovery
GIVEN two standard-preset changes, one with needs_discovery true and one false
WHEN each is driven through planning
THEN neither is blocked before S3 by a missing assurance.md, where previously only the non-discovery path was stranded because the bundle scaffold that created assurance.md fired only on the research-to-bundle transition

#### Scenario: repair bundle-consistency stays silent on deferred assurance before S3
GIVEN a standard/strict change before S3_REVIEW with no assurance.md on disk
WHEN the slipway repair bundle-consistency diagnostic runs
THEN it emits neither an error nor a warning about a missing assurance.md

### Requirement: assurance.md stays fail-closed at S3_REVIEW and later
REQ-003: Deferral MUST NOT weaken closeout safety. A missing, empty, or
scaffold-only `assurance.md` MUST still be rejected at `S3_REVIEW` and later, and
the change MUST NOT reach done with an unauthored assurance contract. The repair /
doctor bundle-consistency diagnostic MUST continue to report a missing required
`assurance.md` as a consistency error at `S3_REVIEW`, `S4_VERIFY`, and `done`.

#### Scenario: missing assurance.md blocks past S3
GIVEN a change at S3_REVIEW on standard or strict with no assurance.md on disk
WHEN advancement past S3_REVIEW is attempted
THEN AssuranceContractBlockers returns assurance_contract_missing and advancement is blocked

#### Scenario: scaffold-only assurance.md blocks past S3
GIVEN a change at S3_REVIEW whose assurance.md still contains only template or scaffold section bodies
WHEN advancement is attempted
THEN the per-section placeholder floor rejects it, keeping the issue #47 backstop in force

#### Scenario: repair bundle-consistency flags missing assurance at S3+
GIVEN a standard/strict change at S3_REVIEW, S4_VERIFY, or done with no assurance.md on disk
WHEN the slipway repair bundle-consistency diagnostic runs
THEN it reports the missing assurance.md as a consistency error

### Requirement: Public surfaces describe assurance.md as a deferred review artifact
REQ-004: All public-facing surfaces that describe assurance.md timing MUST be
consistent with deferred authoring. The `slipway instructions assurance` guidance
MUST NOT claim the engine creates a scaffold to overwrite; the plan-audit host
skill MUST NOT require `assurance.md` to be present at S1; the final-closeout host
skill and the end-user docs MUST NOT describe an engine-created early scaffold.

#### Scenario: instructions guidance matches deferred authoring
GIVEN the assurance authoring guidance returned by slipway instructions assurance
WHEN the guidance is read
THEN it states the engine defers the body and the host authors it from the template, with no "engine may create the scaffold; replace it" language

#### Scenario: plan-audit skill no longer demands an S1-present assurance.md
GIVEN the plan-audit host skill text
WHEN the required-artifact section is read
THEN it does not list assurance.md as a required-present artifact at S1 plan-audit

### Requirement: Plan-audit evidence digest carries no obsolete assurance exclusion
REQ-005: The plan-audit input digest MUST NOT carry the now-obsolete special-case
rationale for excluding `assurance.md`. After deferral, `assurance.md` simply does
not exist at plan-audit time, so it cannot stale the plan-audit digest; the
explanatory exclusion comment/branch is removed while assurance.md remains an input
to the final-closeout digest.

#### Scenario: digest inputs exclude assurance without special-case prose
GIVEN the plan-audit input digest computed by addPlanningArtifactInputs
WHEN the digest input set is inspected
THEN assurance.md is not among the plan-audit digest inputs, and the code no longer carries the "intentionally excluded ... a late edit must not retroactively stale" rationale tied to early creation

### Requirement: Out-of-scope drift is recoverable and accurately guided from review/verify states
REQ-006: When a governed change at `S3_REVIEW` or `S4_VERIFY` has changed files
outside the planned Scope Contract, the documented recovery MUST be reachable and
accurately described. This gap was found while dogfooding this change: a
legitimate out-of-scope edit at review time produced `scope_contract_drift`, but
the engine had reopened the change to `S3_REVIEW` for stale review evidence while
`slipway pivot --rescope` required `S2_EXECUTE`, leaving the documented recovery
unreachable.
(a) `slipway pivot --rescope` MUST be available from every post-planning
execution state (`S2_EXECUTE`, `S3_REVIEW`, `S4_VERIFY`), not `S2_EXECUTE` alone —
it resets the change to `S0_INTAKE` regardless of the starting state, so the
S2-only restriction was artificial. Before execution (`S0_INTAKE`/`S1_PLAN`)
rescope MUST still be rejected with `rescope_state_invalid`.
(b) The `scope_contract_drift` recovery guidance (the CLI remediation and the
`scope_contract_recovery_guidance` diagnostic) MUST present the non-destructive
path first — amend the owning task's `target_files` in `tasks.md` and re-run — and
MUST NOT describe `slipway pivot --rescope` as a non-destructive `tasks.md`
`target_files` edit; it MUST state that rescope resets the change to `S0_INTAKE`.
Public CLI docs describing rescope's valid states MUST match.

#### Scenario: rescope reachable from S3_REVIEW and S4_VERIFY
GIVEN a governed change at S3_REVIEW or S4_VERIFY
WHEN slipway pivot --rescope is invoked
THEN it is accepted (not rejected with rescope_state_invalid) and resets the change to S0_INTAKE

#### Scenario: rescope rejected before execution
GIVEN a governed change at S0_INTAKE or S1_PLAN
WHEN slipway pivot --rescope is invoked
THEN it is rejected with rescope_state_invalid

#### Scenario: scope-drift guidance leads with the non-destructive path and describes rescope honestly
GIVEN the scope_contract_drift recovery guidance and CLI remediation
WHEN they are read
THEN they direct the reader to amend tasks.md target_files as the non-destructive primary path and describe slipway pivot --rescope as a full re-plan that resets the change to S0_INTAKE
