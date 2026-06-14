# Intent

## Summary
Make two confirmed downstream-reported (Lattice) Slipway governance-surface transparency gaps self-explaining, without changing underlying behavior. (1) Issue #207: scope_contract silently filters dirty artifacts/codebase/** files out of changed_files while reporting status=pass, so reviewers must infer the hidden exemption from a git-diff disagreement. Surface the codebase-map exemption explicitly in validate/status JSON output (and align docs/help) so the omission is visible rather than inferred; keep the exemption itself intact. (2) Issue #211: status --json exposes progress.run_summary_version=0 (a 'no execution summary yet' sentinel) but slipway evidence task rejects 0 and requires >=1, so the visible value misleads task-evidence recording. Stop surfacing a value the evidence surface itself refuses and make the correct task-evidence run version unambiguous; keep the >=1 evidence rule intact. Both fixes are to Slipway's own CLI/JSON/doc surfaces (external contracts consumed downstream); no change to scope-contract exemption behavior or the run-summary-version validation rule.
## Complexity Assessment
complex
<!-- Rationale: two distinct governance surfaces (scope_contract reporting; status
     run_summary_version + evidence-task version discoverability), both change
     external JSON contracts consumed downstream (Lattice), and both need aligned
     code + tests + docs. More than a single-surface simple edit. -->

## Guardrail Domains
<!-- none detected -->
External API contracts (advisory): `slipway validate`/`status` JSON output is a
public contract consumed downstream. Changes here are additive/back-compatible
only — no rename/removal of existing fields, no change to gate pass/fail outcomes.

## In Scope
Surface-transparency fixes to Slipway's own CLI/JSON/doc surfaces. Behavior of the
underlying exemption and the evidence-version rule stays unchanged.

#207 — make the scope_contract codebase-map exemption explicit:
- `internal/engine/scopecontract/evaluate.go`: add a structured field to `Report`
  (e.g. `ExemptContextFiles []string`, json `exempt_context_files`) and populate it
  from the evaluate path.
- `internal/engine/progression/readiness.go`: where `scopeContractContextArtifactChangedFile`
  filters `artifacts/codebase/**` out of changed files (~L829–845), collect the
  dirty-but-exempted files and thread them into the Report instead of silently dropping.
- `cmd/validate.go`: add the field to `scopeContractView` (~L68–76) and populate it in
  `buildScopeContractView` (~L333–346).
- `cmd/status*` view builders (e.g. `cmd/status_view_build.go`): ensure `status --json`
  scope_contract carries the same field.
- Docs: `docs/commands.md` and/or `docs/operator-guide.md` document the exemption and
  the new field (and command help if it owns the description).

#211 — stop surfacing a run_summary_version the evidence surface refuses, and make the
correct value discoverable:
- `internal/engine/status/view.go`: `Progress.RunSummaryVersion` (~L38, L117–122, L173–184)
  must not emit `0` when no execution summary exists yet — omit/null it (pointer or
  omitempty + guard) so the visible value is never one `evidence task` rejects.
- `cmd/status_view_build.go` (~L484): map the omitted/null value through to JSON and the
  human status line.
- Make the correct first task-evidence run version (`1`) discoverable from a public
  surface: `cmd/evidence.go` (`evidence task` help/remediation, ~L342–349) and/or
  `next --json` diagnostics — the exact surface is settled at plan-audit; acceptance is
  that a user in early S2 finds the right run version without guessing.
- Tests covering both: status omits the field with no summary; evidence-task version-1
  path is discoverable.

## Out of Scope
- Changing the exemption itself: `artifacts/codebase/**` stays excluded from
  `changed_files`/`out_of_scope_files` accounting and scope_contract stays `pass` when
  only context files are dirty.
- Changing the `evidence task` rule: `--run-summary-version` must remain `>= 1`.
- Changing the run_summary_version counter semantics / increment logic.
- `slipway done` dirty-advisory output (separate surface; this change is validate/status).
- New lifecycle gates, codebase-map generation, or any gate pass/fail outcome change.

## Constraints
- JSON output is an external contract: additive/back-compatible only; do not rename or
  remove existing scope_contract / progress fields.
- Keep code, generated skills, docs, and help aligned as one product surface.
- README is contract-tested by `internal/toolgen` (keep required tokens) if touched;
  prefer `docs/` for prose.
- Lint gate is golangci-lint gofmt **simplify**: verify with `gofmt -s -l` / local
  `golangci-lint run`, not plain `gofmt -l`.
- `go build ./... && go vet ./... && go test ./...` must stay green.

## Acceptance Signals
- #207: with a dirty `artifacts/codebase/*.md` present, `slipway validate --json` and
  `status --json` show the new `scope_contract.exempt_context_files` listing that file,
  while `changed_files` still omits it and `scope_contract.status` stays `pass`
  (covered by a test).
- #207: `docs/` (and command help if it owns the text) describe the codebase-map
  exemption and the new field.
- #211: `slipway status --json` before any execution summary exists does NOT emit
  `progress.run_summary_version=0` (field omitted/null), and a public surface makes the
  correct first task-evidence run version (`1`) discoverable (covered by a test).
- #211: `slipway evidence task --run-summary-version 0` still fails with
  `evidence_task_run_summary_version_invalid` (rule intact; covered by a test).
- `go build ./... && go vet ./... && go test ./...` green; `gofmt -s -l` clean; existing
  scope_contract / status regression tests stay green.

## Open Questions
None.

## Deferred Ideas
- Extend the same explicit-exemption disclosure to `slipway done` dirty-advisory output.
- Generalize a "context artifacts" concept beyond codebase-map if more advisory dirs appear.

## Approved Summary
Confirmed 2026-06-14T13:28:44Z.

Make two confirmed downstream-reported (Lattice) Slipway governance-surface
transparency gaps self-explaining, without changing underlying behavior.

- #207: scope_contract silently drops dirty `artifacts/codebase/**` from
  `changed_files` while reporting `pass`. Add a structured `exempt_context_files`
  field to the scope_contract Report, populate it from the exemption point in
  readiness.go, surface it in both `validate --json` and `status --json`, and
  document the exemption + field. The exemption itself stays intact.
- #211: `status --json` exposes `progress.run_summary_version=0` before any
  execution summary exists, but `evidence task` rejects 0 (requires `>=1`). Stop
  surfacing the rejected 0 (omit/null when no summary) and make the correct first
  task-evidence run version (`1`) discoverable from a public surface. The `>=1`
  rule stays intact.

Scope boundaries: behavior-preserving CLI/JSON/doc surface change only; JSON
additions are back-compatible (no rename/removal, no gate outcome change). Out of
scope: the exemption rule, the `>=1` evidence rule, run_summary_version counter
semantics, `slipway done` advisory, and any new gate.

Primary acceptance signal: with a dirty `artifacts/codebase/*.md`,
`validate`/`status --json` show `scope_contract.exempt_context_files` listing it
while `changed_files` still omits it and `status=pass`; and `status --json` no
longer emits `run_summary_version=0` while the correct version `1` is discoverable
— all covered by tests, with `go build/vet/test ./...` green.
