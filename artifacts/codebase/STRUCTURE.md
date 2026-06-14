# Structure

Re-authored for change
`make-two-confirmed-downstream-reported-lattice-slipway-gover`
(GitHub issues #207 and #211).

- `internal/engine/scopecontract/`
  - `evaluate.go`: `Report` struct + `Clone`; the scope-contract audit data
    contract. Gains `ExemptContextFiles`.
- `internal/engine/progression/`
  - `readiness.go`: workspace changed-file collection and the
    `artifacts/codebase/**` exemption (`scopeContractContextArtifactChangedFile`);
    threads exempted files into the report.
  - `readiness_optimization_test.go`: engine-level regressions for the exemption
    path; gains coverage that exempted files land in `ExemptContextFiles` while
    staying out of `changed_files`.
- `internal/engine/status/`
  - `view.go`: engine Progress model; `RunSummaryVersion` (0 = no execution
    summary yet).
- `cmd/`
  - `validate.go`: shared `scopeContractView` struct and `buildScopeContractView`
    (single builder backing validate/status/review). Gains/maps
    `exempt_context_files`.
  - `validate_test.go`: cmd-level assertion that the field appears with
    `changed_files` still omitting the file and status `pass`.
  - `status.go` / `status_view_build.go`: status `progress.run_summary_version`
    JSON + view mapping (omit-on-zero).
  - `next.go` / `next_context_build.go`: next `progress.run_summary_version`
    JSON + view mapping (omit-on-zero).
  - `status_test.go`: asserts omission with no summary and the real value once a
    run exists.
  - `evidence.go`: `evidence task --run-summary-version` validation (`>= 1`) and
    help/guidance for the first run version.
  - `evidence_test.go`: asserts the guidance text and that `0` still fails.
- `docs/`
  - `commands.md` / `operator-guide.md`: document the `artifacts/codebase/**`
    scope-contract exemption and the new `exempt_context_files` field.
- Governed artifact bundle:
  - `artifacts/changes/make-two-confirmed-downstream-reported-lattice-slipway-gover/`:
    `requirements.md` (REQ-001..003), `decision.md` (selected approach),
    `tasks.md` (t-01..t-05 across two waves).
