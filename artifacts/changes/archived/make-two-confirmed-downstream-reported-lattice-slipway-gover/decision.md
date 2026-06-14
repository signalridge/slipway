# Decision

## Alternatives Considered
1. **Structured field populated at the readiness layer (selected).** Add
   `ExemptContextFiles` to `scopecontract.Report` and populate it where the
   exemption already lives (`internal/engine/progression/readiness.go`,
   `workspaceChangedFiles`). Surface it through the shared `buildScopeContractView`
   (one builder backs validate/status/review). For #211, omit the run-version JSON
   field when it is `0` and add discoverability to the `evidence task` surface.
   Tradeoff: touches the readiness wiring and a shared view, but is single-source
   and covers all three report consumers at once.
2. **Doc-only / static rule statement.** Just document the exemption, or emit a
   fixed "codebase-map is exempt" note. Rejected: the user explicitly wants the
   actual omitted files visible in JSON; a static note still forces reviewers to
   cross-check `git diff` to learn which files were dropped.
3. **Move the exemption into the scopecontract package.** Relocate
   `scopeContractContextArtifactChangedFile` into `scopecontract` so the Report is
   self-populating. Rejected for this change: larger refactor than the objective
   needs; the dirty-file discovery (git) already lives at the readiness layer.
4. **#211: make `evidence task` accept `0`, or repurpose status to show the NEXT
   version.** Rejected: weakening the `>=1` rule loses a real guard, and
   overloading `progress.run_summary_version` (current persisted) to mean "next to
   record" is misleading. Omitting the rejected value + discoverable guidance is
   honest.

## Selected Approach
Alternative 1. #207: thread the exempted dirty context files from the existing
filter point into the report and disclose them via a new
`exempt_context_files` field on the shared scope-contract view; document it.
#211: stop serializing `run_summary_version` when it is `0` (no execution
summary) on `status` and `next`, and make the first run version (`1`) discoverable
through the `evidence task` help/guidance while keeping the `>=1` rejection.

## Interfaces and Data Flow
- `scopecontract.Report` gains `ExemptContextFiles []string`
  (`json:"exempt_context_files,omitempty"`); `Report.Clone` copies it. Existing
  fields are unchanged (additive/back-compatible JSON contract).
- `readiness.go`: `scopeContractWorkspaceChangedFiles` / `workspaceChangedFiles`
  also yield the set of dirty files matched by
  `scopeContractContextArtifactChangedFile`; after
  `EvaluateBundleWithChangedFiles` returns, the cloned report's
  `ExemptContextFiles` is set before assignment to `readiness.ScopeContract`.
- `cmd/validate.go`: `scopeContractView` gains `ExemptContextFiles`;
  `buildScopeContractView` populates it. This builder is shared by `validate`,
  `status` (`cmd/status_view_build.go`), and `review` (`cmd/review.go`), so all
  three surfaces gain the field with one change.
- `cmd/status.go` / `cmd/next.go`: the `run_summary_version` JSON field becomes
  omit-on-zero (e.g. `omitempty`); `cmd/status_view_build.go` and
  `cmd/next_context_build.go` map accordingly. Human-readable status must not print
  a `0` run version.
- `cmd/evidence.go`: `evidence task` help/guidance states the first run version is
  `1`; the existing `runSummary < 1` rejection
  (`evidence_task_run_summary_version_invalid`) is unchanged.

## Rollout and Rollback
Rollout: ship behind no flag — additive JSON field plus omit-on-zero. Verify with
`go build ./... && go vet ./... && go test ./...` and a manual
`slipway validate --json` / `status --json` against a worktree with a dirty
`artifacts/codebase/*.md`. Rollback: revert the change commit; the JSON additions
are back-compatible so no consumer migration is needed. Rollback verification:
`go test ./...` on the reverted tree.

## Risk
- Shared `buildScopeContractView` also feeds `review` — confirm the added field is
  harmless there (it is read-only display). Covered by building/testing the cmd
  package.
- `omitempty` on an `int` run-version drops legitimate `0` only; since `0` is never
  a valid recorded version, this is safe. Guard the human-readable path too.
- JSON consumers downstream (Lattice) rely on existing field names; the change is
  strictly additive plus an omit of a value that was never valid as input, so it
  does not break existing parsers.
