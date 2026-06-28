# Assurance

## Scope Summary
Delivered the confirmed repair for public lifecycle route diagnostics, split
freshness reporting, and independent-review host capability metadata.

The implemented scope covers:
- `invocation_route` on non-success CLI error and diagnostic surfaces for
  no-active, multi-active, explicit missing change, bound-elsewhere, and local
  archived/non-active read contexts.
- A distinct `archived_local` route kind with lifecycle execution disabled.
- Split freshness fields for `next`, default `next --json` handoff output, and
  `done`, plus split human status prose.
- Registry and generated-skill template metadata for independent-review
  `subagent` host capability requirements, fallback modes, evidence requirement,
  and remediation.
- Regression tests, governed task evidence, and S2 wave-orchestration evidence
  for all planned tasks.

## Verification Verdict
Implementation verification passed for the S2 execution scope. The current
change is in S3 review, so final archive readiness remains pending until
spec-compliance, code-quality, independent, security, and ship-verification
evidence are recorded.

## Evidence Index
- `go run . validate --json`
- `go test ./cmd -run 'Test(ResolveActiveChangeRefReportsBoundElsewhereFromRoot|StatusFromRootReportsBoundElsewhere|NoActiveStatusAndValidateExposeInvocationRoute|MultiActiveStatusSummaryExposesInvocationRoute|InvocationRouteWithoutNextCommandUsesInspectOnlyRemediation|ValidateChangeFlagRejectsMissingSlugWithoutWritingState|StatusChangeFlagMissingSlugExposesExplicitMissingRoute|BoundWorktreeCommandsExposeConsistentLocalInvocationRoute|NextChangeFlagFromRootTargetsBoundWorktree)' -count=1`
- `go test ./cmd -run 'Test(ReviewBatchHostCapabilityUnavailableFailsClosedUnlessFallbackSelected|DoneJSONReportsWorktreeArchivePathWhenRunFromWorktree|RenderStatusTextSplitsEvidenceFreshness|DiagnosticCommandsExposePathAuthorityWhenFreshnessUnknown|NextJSONDefaultOmitsFreshnessDiagnosticsWhenDiagnosticsViewHasThem)' -count=1`
- `go test ./internal/engine/capability -run 'Test(ResolveHostCapabilityRequirement|ResolveHostCapabilityRequirementUsesRegistryContract|FrontmatterMirrorsRegistryHostCapabilities|FrontmatterMirrorsRegistryBindings|FrontmatterMirrorsRegistryHydrateReferences)' -count=1`
- `gofmt -l cmd/active_change_resolution_test.go cmd/common.go cmd/done.go cmd/errors.go cmd/lifecycle_commands_test.go cmd/next.go cmd/next_handoff.go cmd/progression_next_test.go cmd/status.go cmd/status_render.go cmd/status_render_test.go cmd/status_view_build.go cmd/validate.go internal/engine/capability/gates_test.go internal/engine/capability/registry.go internal/engine/capability/registry_default.go internal/engine/capability/resolver.go internal/engine/capability/resolver_test.go`
- `go test ./cmd -count=1`
- `go test ./internal/engine/capability ./internal/tmpl ./internal/toolgen -count=1`
- `git diff --check`
- `go test ./... -count=1`
- `just coverage-gate`
- `go run ./internal/perfbaseline/cmd/state-read-baseline -mode check -baseline state-read-performance-baseline.json -out /tmp/slipway-state-read-current-route-freshness-rerun.json -samples 3 -warmups 1 -check-attempts 3`
- Runtime task evidence: `t-01`, `t-02`, `t-03`, `t-04`, and `t-05` all
  recorded as pass for run summary version 1.
- Governance evidence:
  `artifacts/changes/repair-route-freshness-capability-gaps/verification/wave-orchestration.yaml`
  records pass, including `dispatch_mode:wave=2:parallel_subagents` and one
  executor handle for each wave-2 task.

## Requirement Coverage
- `REQ-001`: covered by route diagnostics tests and `t-02` task evidence.
- `REQ-002`: covered by `next`/`done` split freshness tests and `t-03` task
  evidence.
- `REQ-003`: covered by human status freshness rendering tests and `t-03` task
  evidence.
- `REQ-004`: covered by registry resolver and frontmatter parity tests plus
  `t-04` task evidence.

## Residual Risks and Exceptions
- Large WorkspaceIndex/performance architecture work remains deferred.
- Internal/state semantic relocation remains deferred to a dedicated
  architecture-boundary change.
- Generated adapter skill output should be refreshed by the normal
  `slipway init --refresh` flow when publishing generated surfaces.
- S3 review-stage domain, security, independent-review, and ship-verification
  gates remain pending at the time of this S3 assurance repair.

## Rollback Readiness
Rollback is a normal source rollback of the changed Go files, generated skill
templates, tests, governed artifacts, and codebase-map sections. No schema
migration, irreversible operation, external service mutation, credential
rotation, or data migration was introduced.

## Archive Decision
Not archive-ready yet. Active `validate --json` freshness/readiness proof was
captured after S2 wave evidence and before this S3 review repair, but the
change still requires review and ship-verification evidence before `done`.
