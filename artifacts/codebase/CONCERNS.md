# Concerns

Re-authored for change
`make-two-confirmed-downstream-reported-lattice-slipway-gover`
(GitHub issues #207 and #211).

- Public contract drift (Lattice): `validate`/`status`/`review --json` are an
  external contract. The change must stay additive/back-compatible — add
  `scope_contract.exempt_context_files`, never rename or remove existing fields,
  and never change a gate pass/fail outcome.
- Behavior-preservation risk: #207 must disclose the exemption, not weaken it.
  Exempted `artifacts/codebase/**` files must remain out of `changed_files` /
  `out_of_scope_files`, and `scope_contract.status` must stay `pass` when only
  context files are dirty. A regression here would silently change scoping.
- `omitempty` correctness: making `progress.run_summary_version` omit-on-zero
  drops only the never-valid `0`. Any legitimate version is `>= 1` and still
  serializes. The human-readable status/next line must also stop printing `0`.
- Rule-preservation risk: #211 must keep the `evidence task --run-summary-version
  >= 1` rejection (`evidence_task_run_summary_version_invalid`) and the
  run-version counter semantics intact; only the *surfacing* of the rejected `0`
  changes.
- Shared-view blast radius: `buildScopeContractView` feeds `validate`, `status`,
  and `review`. The added field is read-only display on all three, so the single
  edit is intended to reach every surface — confirm review output stays correct.
- Discoverability completeness: omitting the `0` removes a misleading value, so
  the correct first run version (`1`) must be discoverable from a public surface
  (`evidence task` help) or a user is left guessing.
- Test-fixture fidelity: the exemption and omit-on-zero behaviors are easy to
  assert at the unit/cmd layer with fixtures; the manual acceptance check needs a
  real dirty `artifacts/codebase/*.md` present in the worktree.
