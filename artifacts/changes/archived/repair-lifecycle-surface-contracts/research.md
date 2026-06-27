# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `cmd/common.go:445` and `cmd/common.go:535` own explicit slug resolution and `change_not_found` projection.
  - `cmd/common.go:1070` owns the existing `buildInvocationRouteView` model used by route-aware command views.
  - `cmd/freshness_diagnostics.go:29` applies invocation route views to `next`, `validate`, and `status`.
  - `cmd/status_view_build.go:202` projects split execution, governance, and overall readiness freshness for `status` and `validate`.
  - `cmd/status_view_build.go:248` projects the action contract shared by `status` and `validate`.
  - `cmd/next.go:741` derives `next` confirmation/action contract; `cmd/next.go:863` applies host capability blockers.
  - `cmd/done.go:23` defines `doneView` without route data; `cmd/done.go:266` resolves the active change through the older non-context helper.
  - `cmd/evidence.go:40` defines `evidenceSkillView` without route data; `cmd/evidence.go:124` and `cmd/evidence.go:390` resolve evidence targets through the older non-context helper.
- Dependency chains:
  - CLI commands -> `stateReadContext` / `resolveActiveChangeRefWithReadContext` -> `internal/state` for change authority and worktree binding.
  - CLI commands -> `progression.EvaluateGovernanceReadiness` -> `internal/model` reason codes and recovery projection.
  - CLI commands -> `internal/engine/capability.ResolveHostCapabilityRequirement` for host capability availability and fallback contracts.
- Blast radius:
  - Keep production edits in `cmd` route/view assembly and, only if necessary, `internal/engine/capability`.
  - Do not move lifecycle authority, archive behavior, evidence stamping, or readiness evaluation into a new layer.
- Constraints:
  - `internal/state` must remain a read/write authority layer and not gain engine lifecycle semantics.
  - The user explicitly requested no compatibility layers; incorrect public contracts should be replaced directly.

### Patterns
- Existing conventions:
  - New command JSON fields are small view structs in the command package (`nextView`, `validateView`, `statusView`).
  - `buildInvocationRouteView` already centralizes route fields and remediation for active/archived/bound cases.
  - `stateReadContext` already exists for per-command state reuse and should be used instead of adding another cache.
  - Tests prefer black-box command execution with `commandForRoot`, `withWorkspace`, `withNestedWorkingDirectory`, and JSON decode assertions.
- Reusable abstractions:
  - Reuse `invocationRouteView` rather than introduce a parallel route struct for `done` and `evidence`.
  - Reuse `resolveActiveChangeRefWithReadContext` in `done` and `evidence` so explicit missing slug behavior stays aligned.
  - Reuse `hostCapabilityViewsForSkills`, `applyHostCapabilityContractToNext`, and `applyHostCapabilityContractToValidate` rather than duplicate capability resolution.
- Convention deviations:
  - `done` and `evidence` are mutating commands, so route information should appear in successful JSON views and error/remediation tests, but should not alter stamping or archiving semantics.

### Risks
- Technical risks:
  - Medium: adding route fields to mutating command outputs could accidentally reload state after mutation, especially `done` after archiving. Mitigation: compute the route from the pre-archive active change and include it as execution context only.
  - Medium: capability unavailable blockers could change `run` behavior if appended too early or for the wrong selected skills. Mitigation: keep capability checks only on selected S3 review/host surfaces and add focused tests.
  - Low: split freshness fields already exist, but legacy `evidence_freshness` remains. Mitigation: assert `overall_readiness_freshness` is the completion signal and do not rely on the legacy field in new code.
  - Low: explicit missing slug is already tested for `validate`; adding evidence/done route tests should not weaken this.
- Guardrail domains:
  - Lifecycle governance and public CLI contract. No credential, PII, financial, schema migration, or release-secret exposure.
- Reversibility:
  - Source changes are normal CLI/view/test edits and can be reverted by commit. Runtime evidence files are generated and ignored except for governed top-level artifacts.

### Test Strategy
- Existing coverage:
  - `cmd/active_change_resolution_test.go:17` tests root `resolveActiveChangeRef` bound-elsewhere behavior.
  - `cmd/active_change_resolution_test.go:127` verifies local bound route consistency for `status`, `next`, and `validate`.
  - `cmd/active_change_resolution_test.go:224` verifies `validate --change missing` fails closed as `change_not_found`.
  - `cmd/progression_next_test.go:1144` through `cmd/progression_next_test.go:1180` verifies review batch action and split freshness alignment for `next`, `validate`, and `status`.
  - `cmd/progression_next_test.go:1234` through `cmd/progression_next_test.go:1310` verifies host capability unavailable and manual fallback behavior across `next`, `validate`, and `run`.
  - `internal/engine/capability/resolver_test.go:92` verifies capability availability and fallback modes.
- Infrastructure needs:
  - Add command-level tests for `done` and `evidence` route/error consistency using existing command test helpers.
  - If implementation changes capability projection, add focused tests in `cmd/progression_next_test.go` or `internal/engine/capability/resolver_test.go`.
- Verification approach:
  - First run targeted `go test ./cmd -run 'Test(.*InvocationRoute|.*ChangeFlag|.*HostCapability|.*ReviewBatch|.*Freshness)' -count=1` adjusted to actual test names.
  - Run package tests for touched areas, then `go test ./... -count=1`.

### Options
- Option 1: Minimal route-completion patch.
  - Add `InvocationRoute *invocationRouteView` to `doneView`, `evidenceSkillView`, `evidenceTaskView`, and `evidenceTaskBatchView`; switch done/evidence target resolution to `stateReadContext`; apply route views on successful JSON outputs; add black-box tests for bound-worktree/root/explicit cases.
  - Tradeoffs: Smallest code change and directly closes the concrete gap in `opt.md` 1.1. Does not over-rewrite already working freshness/action/capability code.
- Option 2: New public lifecycle contract package.
  - Move route, action, freshness, and capability projections into a new shared package consumed by every command.
  - Tradeoffs: More architectural purity, but much larger blast radius and likely duplicates existing `cmd` view conventions. Higher risk for no immediate contract gain.
- Option 3: Full rewrite of command diagnostics.
  - Replace command-specific JSON structures with one common envelope.
  - Tradeoffs: Most uniform output but violates minimal-change discipline and would effectively preserve old behavior through transitional adapters or break a large surface unnecessarily.
- Selected: Option 1.
  - Rationale: Current main already contains shared route, split freshness, action contract, and host capability foundations for `status`, `next`, `validate`, and `run`. The remaining concrete gap is extending the route contract and tests to `done` and `evidence` while preserving the existing fail-closed behavior. This matches the user's no-compatibility-layer instruction and keeps the work scoped to the public lifecycle surface.

## Unknowns
- Resolved: Are `opt.md` 4.1-4.4 still pending? -> No. Current main contains merged state-read performance work through PRs #355 and #358, including read-context, explicit `--change` fast path, and timeline tail read evidence.
- Resolved: Does `validate --change <missing>` still fall through to diagnostics? -> No. `cmd/active_change_resolution_test.go:224` tests fail-closed `change_not_found`, and `cmd/validate.go:174` only falls back to diagnostics when no explicit slug is provided.
- Resolved: Are split freshness and review action contracts absent? -> No. `cmd/status_view_build.go:202`, `cmd/status_view_build.go:248`, and `cmd/progression_next_test.go:1144` through `cmd/progression_next_test.go:1180` show this is already present for the main read surfaces.
- Remaining: None.

## Assumptions
- The current worktree CLI behavior is the authority, not old chat or remembered branch state. Evidence: `go run . status --json` in the bound worktree reported active `repair-lifecycle-surface-contracts`.
- `done` route data must be computed before archive mutation. Evidence: `cmd/done.go:328` archives the active bundle, after which active-route loading is no longer valid.
- Evidence route data should describe the target change of the evidence command, not the written verification file. Evidence: `cmd/evidence.go:124` and `cmd/evidence.go:390` both first resolve an active change before writing skill or task evidence.

## Canonical References
- `opt.md` section 1.1-1.5 for required lifecycle surface scope.
- `cmd/common.go:445` explicit change resolution.
- `cmd/common.go:1070` invocation route view construction.
- `cmd/freshness_diagnostics.go:29` existing route application hooks.
- `cmd/status_view_build.go:202` split freshness projection.
- `cmd/status_view_build.go:248` action contract projection.
- `cmd/next.go:741` confirmation/action contract projection.
- `cmd/next.go:863` host capability contract projection.
- `cmd/done.go:23` and `cmd/done.go:266` missing done route projection.
- `cmd/evidence.go:40`, `cmd/evidence.go:124`, and `cmd/evidence.go:390` missing evidence route projection.
- `cmd/active_change_resolution_test.go:127` existing route consistency tests.
- `cmd/progression_next_test.go:1144` through `cmd/progression_next_test.go:1310` existing freshness/action/capability tests.
- `internal/engine/capability/resolver.go:92` host capability resolver.
