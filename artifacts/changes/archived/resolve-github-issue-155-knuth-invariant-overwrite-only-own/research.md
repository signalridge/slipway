# Research

## Alternatives Considered

### Architecture
- Affected modules: `internal/engine/progression/evidence_digests.go` owns
  skill input digest construction. The relevant branch starts at
  `certifiedSkillInputDigest` and routes `plan-audit`,
  `intake-clarification`, and `research-orchestration` to governed artifact
  inputs.
- Dependency chains:
  `evidence_digests.go` -> `model.ComputeInputHash` for canonical digesting;
  `evidence_digests.go` -> `wave.TaskPlanStructuralHash` for `tasks.md`;
  `evidence_digests.go` -> `state.LoadOptionalEvidenceDigestsForChange` for
  stored/current comparison; stale blockers feed `stale_evidence_recovery.go`.
- Blast radius: planning skill evidence freshness for `intent.md`,
  `requirements.md`, `research.md`, and `decision.md`; research evidence
  freshness for `intent.md` and `research.md`; intake evidence freshness for
  `intent.md`.
- Constraints: evidence freshness remains fail-closed. The classifier may
  suppress engine-owned scaffold/comment churn only when material content is
  absent or recognized as a narrow known default.

### Patterns
- Existing conventions: `tasks.md` already uses structural hashing through
  `wave.TaskPlanStructuralHash`, while prose artifacts call
  `computeProseFileInputHash`.
- Reusable abstractions: artifact templates are accessible through
  `artifact.TemplateContent`, and artifact substance gates already strip HTML
  comments and reject scaffold-only content.
- Convention deviations: the current prose digest is raw-content based; issue
  #155 requires a material-view digest for prose while keeping tasks on their
  existing structural hash path.
- GSD reference: local gsd-core uses `KNOWN_TEMPLATE_DEFAULTS` and
  `stateReplaceFieldIfTemplate` to replace fields only when the current value is
  absent, blank, or a recognized handler-written default.

### Risks
- Technical risks: high false-negative risk if authored prose is omitted from
  the digest; medium false-positive risk if comments/scaffold-only sections keep
  triggering reopens; low performance risk because artifacts are small markdown
  files.
- Guardrail domains: none of the sensitive guardrail domains are directly
  touched, but lifecycle evidence freshness is load-bearing.
- Reversibility: safe to roll back by reverting the prose digest material-view
  helper and tests. Existing evidence records will be recomputed on future
  checks.

### Test Strategy
- Existing coverage: `evidence_digests_test.go` already covers plan-audit
  digest inputs, CRLF normalization, tasks structural hashing, stale blockers,
  and research artifact digest drift.
- Infrastructure needs: no new fixtures beyond writing artifact markdown bodies
  in existing temp-bundle helpers.
- Verification approach: add tests that comment/scaffold-only prose edits keep
  the digest stable, human-authored prose edits stale the relevant named input,
  and unknown non-empty prose is included in the digest.

### Options
- Option 1: Strip only HTML comments before hashing prose artifacts.
  Tradeoff: small and safe, but it does not express the known-default invariant
  from issue #155 and still churns on scaffold-only heading/body defaults.
- Option 2: Add a prose artifact material-view digest that strips comments,
  treats empty/comment-only scaffold sections as non-material, recognizes only
  narrow engine-owned defaults, and includes all unknown non-empty prose.
  Tradeoff: slightly more code, but it matches the GSD invariant without
  weakening fail-closed material edits.
- Option 3: Build a full per-artifact AST and field-overwrite framework.
  Tradeoff: broad and likely over-engineered for the issue; it risks changing
  authoring and validation semantics unrelated to digest freshness.
- Selected: Option 2. It is the smallest repo-native analogue of GSD's
  overwrite-only-own-defaults behavior and gives tests a precise materiality
  boundary.

## Unknowns
- Resolved: Which current Slipway digest/reopen functions own prose artifact
  materiality? -> `computeProseFileInputHash` in
  `internal/engine/progression/evidence_digests.go` is the central seam; stale
  recovery consumes its blockers through `stale_evidence_recovery.go`.
- Resolved: What exact scaffold/default values are engine-owned today? ->
  HTML authoring comments and empty scaffold sections are engine-owned; the
  `intent.md` fallback summary "Describe the change objective." is a narrow
  known default. Other non-empty prose should default to material unless a
  future test adds a narrow known default.
- Resolved: Which tests cover auto-reopen on artifact edits? ->
  `evidence_digests_test.go` covers named digest drift and should be extended
  there before touching broader lifecycle tests.
- Resolved: Does GSD imply a recovery edge case Slipway lacks? -> GSD's key
  invariant is not recovery-specific; it is the classification boundary: absent,
  blank, or known defaults are handler-owned, everything else is preserved and
  therefore material.
- Remaining: None.

## Assumptions
- The materiality classifier should operate at digest-input time, not by
  mutating artifact files. Evidence: current stale recovery is driven by named
  digest input changes.
- A comment-only scaffold refresh is non-material. Evidence: artifact substance
  gates strip HTML comments before deciding whether sections have authored
  content.
- Unknown non-empty prose is material. Evidence: issue #155 explicitly requires
  defaulting to reopen on doubt.

## Canonical References
- `internal/engine/progression/evidence_digests.go:36`
- `internal/engine/progression/evidence_digests.go:388`
- `internal/engine/progression/evidence_digests.go:426`
- `internal/engine/progression/stale_evidence_recovery.go:59`
- `internal/engine/artifact/manager.go:152`
- `internal/engine/artifact/manager.go:329`
- `internal/engine/artifact/manager.go:559`
- `internal/engine/artifact/requirements.go:35`
- `internal/engine/progression/evidence_digests_test.go:18`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/state-document.cts:154`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/state-document.cts:209`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/state-document.cts:257`
