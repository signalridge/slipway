# Intent

## Summary
Implement issue #297 Workstream B: replace the agent-filled `evidence task`
ledger flag protocol with a compact executor result import and make
`run_summary_version` engine-owned.

## Complexity Assessment
complex

The change touches the governed execution ledger, wave-plan materialization,
freshness inputs, task evidence writing, CLI UX, and generated agent guidance.
It must preserve fail-closed freshness, per-task changed-file scope audit, and
S3 re-entry behavior.

## Guardrail Domains
None detected as a sensitive user-data domain, but this is governance-kernel
work and must preserve existing evidence and lifecycle fail-closed behavior.

## In Scope
- Add `slipway evidence task --result-file <json> --json` as the supported
  compact executor-result import path, with repeated `--result-file` values for
  atomic multi-task batch import.
- Define and persist an engine-owned active execution `run_summary_version`
  boundary, anchored to wave-plan materialization and advanced for fresh
  re-execution.
- Derive task evidence ledger fields from Slipway-owned state:
  `task_kind`, `run_summary_version`, `target_files`, `captured_at`, and
  `freshness_inputs`.
- Reject executor result files that try to supply Slipway-owned ledger fields,
  reference unknown tasks, contain invalid verdicts, contain invalid paths, or
  hit ambiguous existing run-version state.
- Preserve executor-owned `changed_files` and existing scope/parallel-overlap
  safety.
- Update the B-relevant wave-orchestration / evidence-task agent guidance so
  agents are taught to write result JSON and import it, not hand-assemble
  ledger flags.
- Add focused tests for result import, run-version derivation, fail-closed
  ambiguity, and S3/re-execution version behavior where the current engine path
  requires it.

## Out of Scope
- Workstream A deletion of `checkpoint`, `learn`, and `stats` command surfaces.
- Workstream C-wide cleanup/demotion of all low-level evidence docs, except
  B-required guidance that would otherwise still teach the old task ledger flag
  protocol.
- Re-implementing `evidence suite-result`; that existing command is not part of
  B except where tests or guidance would be broken by B edits.
- Adding a free-form `notes` field to task evidence unless a real ledger field
  and tests become necessary; the preferred B schema omits it.

## Constraints
- Use the current worktree implementation as lifecycle authority.
- Do not hand-edit engine-owned freshness state, evidence digests, task evidence
  files, or suite-result YAML.
- `next` and `validate` must remain read-only.
- The long legacy flag protocol may remain only if needed as an internal or
  compatibility surface during B, but generated/default agent guidance must not
  continue teaching agents to provide `run_summary_version`, `task_kind`, or
  `target_files`.
- Native built-in subagents may be used, with parallelism capped at 2.

## Acceptance Signals
- `go run . evidence task --help` documents `--result-file` as the supported
  compact result import path.
- A valid result file writes full task evidence with engine-derived
  `task_kind`, `run_summary_version`, `target_files`, `captured_at`, and
  `freshness_inputs`.
- Result import rejects attempts to set Slipway-owned ledger fields and fails
  closed on invalid task IDs, verdicts, paths, or ambiguous run-version state.
- Repeating `--result-file` imports multiple task results atomically and leaves
  no partial task evidence when any member is invalid.
- A fresh S2 re-execution after review/fix re-entry advances the active
  run-summary version instead of reusing stale execution evidence.
- Existing freshness, scope-contract, stale task evidence, and wave-summary sync
  tests continue to pass.
- Generated wave-orchestration guidance no longer teaches the old long
  `evidence task --task-id ... --run-summary-version ... --task-kind ...`
  command as the agent path.
- Required checks from issue #297 B pass, including focused package tests,
  `go run ./internal/toolgen/cmd/gen-surface-manifest --check`, and eventually
  `go test ./...` before S3 completion.

## Open Questions
- [x] Which existing wave-plan materialization path is the safest authority for
  persisting and advancing the engine-owned run version?
- [x] Does current S3 -> S2 re-entry re-materialize a wave plan, or must B add
  that re-entry path so run versions advance on review-driven re-execution?
- [x] Which generated templates and tests are the minimal B-owned guidance
  surface for replacing the old task ledger flag protocol without taking all of
  Workstream C?

## Deferred Ideas
- Fully deleting `checkpoint`, `learn`, and `stats` remains Workstream A.
- Full suite-result and evidence-digest wording cleanup remains Workstream C.

## Approved Summary
Confirmed from the user-provided objective for issue #297 Workstream B on
2026-06-22T06:34:46Z.

Implement the B slice only: add compact executor-result import through
`slipway evidence task --result-file`, allow repeated `--result-file` values for
atomic multi-task batch import, make `run_summary_version` engine-owned, derive
internal task evidence fields from Slipway-owned state, preserve changed-file
scope safety, and update the B-relevant agent guidance and tests. The change
explicitly excludes Workstream A command deletion and the full Workstream C
surface cleanup, except for B-required guidance that would otherwise keep
teaching the old manual task-ledger flag protocol. The primary acceptance signal
is valid result JSON importing full task evidence with engine-derived fields
while invalid or ambiguous ledger state fails closed.
