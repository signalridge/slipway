# Architecture

Re-authored for change
`make-two-confirmed-downstream-reported-lattice-slipway-gover`
(GitHub issues #207 and #211).

Question: which Slipway reporting seams turn two silent governance-surface
behaviors (a hidden scope-contract exemption; a run version the evidence surface
refuses) into self-explaining JSON, without changing the underlying behavior?

## Affected Seams

- `internal/engine/scopecontract/evaluate.go` owns the `Report` data contract
  for the scope-contract audit (`changed_files`, `out_of_scope_files`,
  `planned_targets`, blockers) and its `Clone`. It gains a new additive
  `ExemptContextFiles []string` (`json:"exempt_context_files,omitempty"`) field;
  no existing field changes.
- `internal/engine/progression/readiness.go` owns workspace changed-file
  collection and the `artifacts/codebase/**` exemption
  (`scopeContractContextArtifactChangedFile`, ~L841/L844) that drops dirty
  codebase-map files from `changed_files` while keeping `scope_contract.status`
  `pass`. It is the seam (~L829) where the dropped files must be captured and
  threaded into the cloned `Report.ExemptContextFiles` before the report is
  assigned to `readiness.ScopeContract` (~L289).
- `internal/engine/status/view.go` owns the engine Progress model. Its
  `RunSummaryVersion` is `0` until an execution summary exists (set to the real
  version only when `summary.RunSummaryVersion >= 1`). `0` is the "no summary
  yet" sentinel that `evidence task` rejects.
- `cmd/validate.go` owns the shared `scopeContractView` struct and its single
  builder `buildScopeContractView`. That one builder backs all three report
  consumers — `validate` (`cmd/validate.go`), `status`
  (`cmd/status_view_build.go`), and `review` (`cmd/review.go`) — so the new
  `exempt_context_files` field reaches every surface from one edit.
- `cmd/status.go` / `cmd/status_view_build.go` and `cmd/next.go` /
  `cmd/next_context_build.go` own the user-facing `progress.run_summary_version`
  JSON. They are the only two surfaces that emit it; the field becomes
  omit-on-zero and the human-readable line must not print a `0`.
- `cmd/evidence.go` owns `evidence task --run-summary-version` validation
  (`runSummary < 1` → `evidence_task_run_summary_version_invalid`, ~L342) and its
  help. The rejection stays; the help gains discoverability of the correct first
  run version (`1`).

## Dependency Flow

For #207: readiness gathers dirty workspace files, evaluates the scope contract,
then filters `artifacts/codebase/**` out of `changed_files`. Today the filtered
files are discarded silently. The change captures that same dropped set and
records it on the cloned `Report.ExemptContextFiles`; `buildScopeContractView`
then maps it onto the shared view, so `validate`/`status`/`review --json` all
disclose what was exempted while `changed_files` still omits it and the status
stays `pass`.

For #211: the engine Progress carries `RunSummaryVersion` (0 when no summary).
The cmd JSON structs in `status` and `next` serialize it as
`progress.run_summary_version`. Making those fields omit-on-zero stops surfacing
a value `evidence task` refuses; the correct first version (`1`) is instead made
discoverable on the `evidence task` help surface, with the `>= 1` rejection
unchanged.

## Constraints And Invariants

- The exemption behavior is preserved: `artifacts/codebase/**` stays out of
  `changed_files`/`out_of_scope_files`, and `scope_contract.status` stays `pass`
  when only context files are dirty. The change only discloses the exemption.
- The `evidence task` `>= 1` rule and the `run_summary_version` counter semantics
  are preserved; only the *surfacing* of the rejected `0` changes.
- JSON output is an external contract consumed downstream (Lattice): changes are
  additive/back-compatible only — no rename or removal of existing fields, and
  no gate pass/fail outcome change. `omitempty` on the int run-version drops only
  the never-valid `0`.
- One shared `buildScopeContractView` keeps validate/status/review aligned; the
  field is read-only display on all three.
