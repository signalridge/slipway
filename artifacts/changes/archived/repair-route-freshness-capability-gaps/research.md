# Research

## Alternatives Considered

### Architecture
- Affected modules: public command surfaces in `cmd/errors.go`, `cmd/common.go`,
  `cmd/status.go`, `cmd/validate.go`, `cmd/next.go`, `cmd/next_handoff.go`,
  `cmd/done.go`, `cmd/status_render.go`, and `cmd/status_view_build.go`; host
  capability metadata in `internal/engine/capability/*`; independent-review
  generated skill templates under
  `internal/tmpl/templates/skills/independent-review/*`.
- Entry points: `CLIError` now carries optional `invocation_route`
  (`cmd/errors.go:46`); diagnostic route builders cover `no_active`,
  `multi_active`, `explicit_missing`, and `bound_elsewhere`
  (`cmd/common.go:56`, `cmd/common.go:80`, `cmd/common.go:91`,
  `cmd/common.go:102`, `cmd/common.go:113`); successful route construction
  still flows through `buildInvocationRouteView` (`cmd/common.go:1184`) and
  distinguishes local archived worktrees as `archived_local`
  (`cmd/common.go:1211`).
- Dependency chains: command resolution produces CLIError or command views;
  status/validate/next/done project those into JSON/human surfaces; governance
  readiness and execution freshness project through shared helpers in
  `cmd/status_view_build.go:206`, `cmd/status_view_build.go:238`, and
  `cmd/status_view_build.go:266`; host capability selection resolves from
  registry metadata through
  `internal/engine/capability/resolver.go:96`.
- Blast radius: additive public output fields and registry/template metadata.
  No lifecycle mutation, state storage, or release workflow behavior changes.
- Constraints: keep existing `evidence_freshness` and error codes for
  compatibility; non-success route kinds must keep lifecycle execution disabled;
  host capability fallback must remain explicit.

### Patterns
- Existing conventions: command JSON structs use optional `omitempty` fields
  and reuse shared projection helpers instead of ad hoc per-command formatting
  (`cmd/next.go:55`, `cmd/done.go:32`, `cmd/validate.go:34`).
- Reusable abstractions: `invocationRouteView` and route helper functions are
  already the route vocabulary, so the repair extends that vocabulary instead
  of introducing a parallel diagnostic schema (`cmd/common.go:56`,
  `cmd/common.go:1184`).
- Capability conventions: capability registry entries are the durable skill
  metadata authority; the default independent-review skill now declares its
  `subagent` requirement, fallback modes, evidence requirement, and remediation
  in registry data (`internal/engine/capability/registry.go:88`,
  `internal/engine/capability/registry_default.go:47`).
- Template convention: generated skill source frontmatter mirrors registry
  metadata and is guarded by a frontmatter drift test
  (`internal/engine/capability/gates_test.go:76`).
- Convention deviations: none required; the change is additive and uses local
  command/capability patterns.

### Risks
- Public API risk, medium: additive JSON fields are low-risk, but consumers may
  begin depending on them. Existing field names are preserved
  (`cmd/next.go:55`, `cmd/done.go:32`, `cmd/errors.go:46`).
- Fail-closed risk, medium: diagnostic route metadata must not become implicit
  permission to run lifecycle actions. Builders set local/effective lifecycle
  execution false for no-active, multi-active, explicit-missing, and
  bound-elsewhere routes (`cmd/common.go:56`, `cmd/common.go:113`).
- Freshness truthfulness risk, medium: late host capability blockers can change
  readiness after initial freshness projection, so `next` recomputes overall
  readiness freshness after those blockers are appended (`cmd/next.go:500`,
  `cmd/status_view_build.go:244`).
- Host capability risk, medium: independent-review still fails closed when the
  host lacks `subagent`; `delegation` only aliases `subagent`, not future
  unrelated capability names (`internal/engine/capability/resolver.go:140`).
- Security/guardrail domain: external API contracts, because CLI JSON/text
  surfaces and generated skill contracts are external agent-facing APIs.
- Reversibility: safe standard revert of additive command fields, tests, and
  registry/template metadata.

### Test Strategy
- Existing coverage was extended at the command surface rather than only unit
  helpers. Route tests cover bound-elsewhere, no-active, multi-active,
  explicit-missing, and archived-local behavior
  (`cmd/active_change_resolution_test.go:40`,
  `cmd/active_change_resolution_test.go:123`,
  `cmd/active_change_resolution_test.go:173`,
  `cmd/active_change_resolution_test.go:297`).
- Freshness tests cover next/run/validate blocked freshness, done pre-archive
  freshness, and human status split prose (`cmd/progression_next_test.go:1239`,
  `cmd/lifecycle_commands_test.go:128`, `cmd/status_render_test.go:60`).
- Capability tests cover registry-owned requirements, bounded aliases, and
  template drift gates (`internal/engine/capability/resolver_test.go:175`,
  `internal/engine/capability/gates_test.go:76`).
- Verification approach: run focused tests for changed surfaces, then full
  `go test ./... -count=1`, `just coverage-gate`, `git diff --check`, and the
  state-read performance baseline check.

### Options
- Option A: Keep route/freshness/capability behavior as-is and document
  operator recovery. Tradeoff: no code risk, but leaves agents parsing prose and
  resolver-private assumptions.
- Option B: Add route/freshness/capability metadata at the public surface while
  preserving existing fields. Tradeoff: small additive API growth, but fixes the
  confirmed correctness/discoverability gaps without broad architecture churn.
- Option C: Fold this into a larger WorkspaceIndex/state-read architecture
  change. Tradeoff: could reduce later duplication, but mixes performance
  architecture with a public-surface correctness repair.
- Selected: Option B. It matches the confirmed current gaps, preserves
  compatibility, keeps lifecycle mutation untouched, and has focused testable
  acceptance criteria.

## Unknowns
- Resolved: Do `next`/`done` lack split freshness concepts? -> Yes; repaired by
  adding execution/governance/overall fields and tests.
- Resolved: Does human status collapse freshness into one misleading line? ->
  Yes; repaired by split prose.
- Resolved: Are non-success route kinds incomplete? -> Yes; repaired for
  no-active, multi-active, explicit-missing, bound-elsewhere, and
  archived-local.
- Resolved: Is host capability metadata resolver-private? -> Yes; repaired via
  registry/template metadata and drift tests.
- Resolved: Does `run` lack host capability checks? -> No; existing
  `runGovernedLoop`/next-view behavior already surfaced host capability blockers
  and tests cover `runView`.
- Resolved: Are GitHub branch/tag/environment protections absent? -> No; live
  `gh api` checks on 2026-06-27 UTC showed main branch protection, active branch
  ruleset, active release tag ruleset, and release-publish reviewer protection.
- Remaining: None.

## Assumptions
- Additive public fields are preferable to replacing existing fields. Evidence:
  existing command structs use optional output fields and tests assert preserved
  blockers/error codes.
- The user's original instruction to fact-check and then fully repair confirmed
  findings is scope authorization for Option B. Evidence: `intent.md` approved
  summary and acceptance signals.
- WorkspaceIndex/state-read route caching remains out of scope. Evidence:
  current defects are command-surface contract gaps, while the stale codebase map
  performance context was explicitly re-authored and deferred.

## Canonical References
- `cmd/errors.go:46`
- `cmd/common.go:56`
- `cmd/common.go:1184`
- `cmd/status.go:322`
- `cmd/validate.go:180`
- `cmd/next.go:55`
- `cmd/next_handoff.go:22`
- `cmd/done.go:32`
- `cmd/status_render.go:145`
- `cmd/status_view_build.go:206`
- `internal/engine/capability/registry.go:88`
- `internal/engine/capability/registry_default.go:47`
- `internal/engine/capability/resolver.go:96`
- `internal/tmpl/templates/skills/independent-review/SKILL.md`
- `cmd/active_change_resolution_test.go:123`
- `cmd/progression_next_test.go:1239`
- `cmd/lifecycle_commands_test.go:128`
- `cmd/status_render_test.go:60`
- `internal/engine/capability/resolver_test.go:175`
- `internal/engine/capability/gates_test.go:76`
- Live GitHub checks: `gh api repos/signalridge/slipway/branches/main`,
  `gh api repos/signalridge/slipway/rulesets`, and
  `gh api repos/signalridge/slipway/environments` on 2026-06-27 UTC.
