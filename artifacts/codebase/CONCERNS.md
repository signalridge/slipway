# Concerns

- Stale map risk resolved: this file replaces prior Workstream A context with
  Workstream B-specific seams and risks.
- Ledger authority risk: accepting `run_summary_version`, `task_kind`,
  `target_files`, `captured_at`, or `freshness_inputs` from result JSON would
  preserve the old ledger-clerk model. The result import must reject those
  fields and derive them from Slipway state.
- Run-boundary risk: simply reading the latest existing run version cannot
  create a new execution run. Workstream B must introduce an engine-owned run
  boundary at wave-plan materialization or another explicit execution-open
  operation.
- S3 repair risk: current `fix` and `review` commands do not reopen S2, so
  acceptance around review-driven re-execution requires a deliberate path rather
  than relying on existing lifecycle behavior.
- Scope safety risk: `changed_files` is executor knowledge and feeds
  scope-contract and same-wave overlap safety. The compact schema must retain it
  for task kinds that require changed files.
- Guidance drift risk: even if the CLI supports `--result-file`, generated
  wave-orchestration guidance, command docs, manifest examples, and diagnostic
  blockers can keep teaching the old long flag protocol.
- Compatibility risk: the old flags may remain host-internal for this B slice if
  needed, but generated/default agent guidance must prefer result import and
  must not require agents to choose run versions or task kinds.
- Reversibility: result import and engine-owned run metadata are additive, but
  changing run-version authority affects execution freshness. Focused tests must
  prove stale/mixed run versions still fail closed.
