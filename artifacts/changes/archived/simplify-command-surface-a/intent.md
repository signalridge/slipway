# Intent

## Summary
simplify command surface A

Implement Workstream A from GitHub issue #297: remove `checkpoint`, `learn`,
and `stats` as product/agent-facing command surfaces while preserving the
governed lifecycle, fail-closed evidence behavior, and ledger-backed
`run --resume` recovery.

## Complexity Assessment
complex

Rationale: this deletes exported command surfaces and checkpoint-specific
lifecycle state across command registration, generated skills/docs, runtime
state validation/repair, and tests. The change is not security-sensitive, but it
touches workflow authority and must be verified with black-box command output
and focused regression tests.

## Guardrail Domains
None detected.

## In Scope
- A1: delete the `slipway checkpoint` command surface and the checkpoint
  lifecycle concept, including `ActiveCheckpoint`, `--resume-response`,
  `resume_checkpoint` handoff fields, checkpoint metadata/reason codes,
  checkpoint-specific status/next/run/health/repair behavior, generated skill
  guidance, and live product/docs references.
- Preserve `run --resume`, interrupted-wave recovery, task verdicts
  `blocked`/`incomplete`, blocker details, and `handoff.md` as context-only
  continuation notes.
- A2: delete the `learn` command surface and generated/docs references,
  including the `learn_apply_unsupported` dead-end command behavior.
- A3: delete the `stats` command surface and generated/docs references while
  preserving reusable repository statistics internals where still used by
  `status --stats`, `health`, or other diagnostics.
- Refresh generated surface docs/manifests/adapters needed by A.
- Add or update focused tests proving deleted surfaces are absent and preserved
  resume/diagnostic paths still work.

## Out of Scope
- Workstream B: `evidence task --result-file`, executor-result import schema,
  and engine-owned run-version boundary.
- Workstream C: broad low-level evidence wording cleanup, `suite-result`
  documentation alignment, and evidence-digest cache demotion, except for
  references that directly advertise deleted A surfaces.
- Renaming or redesigning retained commands such as `fix`, `repair`,
  `validate`, `health`, `preset`, `abort`, `cancel`, `delete`, `evidence skill`,
  or `evidence suite-result`.
- Adding a new structured human-decision record to replace checkpoint
  `allowed-responses`; issue #297 explicitly accepts that loss for A1.

## Constraints
- Use the governed worktree
  `/Users/yixianlu/ghq/github.com/signalridge/slipway/.worktrees/simplify-command-surface-a`
  as authority for lifecycle and validation.
- Do not hand-edit engine-owned freshness state, runtime task evidence,
  evidence digests, or generated verification verdict YAML.
- `next` and `validate` must remain read-only.
- Deleted command surfaces must disappear from actual exported surfaces, not
  only from Cobra `Hidden` fields.
- Use native built-in delegation/subagent mechanisms only if delegation is
  needed; do not rely on external agent runtimes for authoritative lifecycle
  progression.

## Acceptance Signals
- `go run . --help` does not show `checkpoint`, `learn`, or `stats`.
- `go run . run --help` and stage driver help do not show
  `--resume-response`, while `run --help` still shows `--resume`.
- Search checks for live product/template/docs references to
  `slipway checkpoint`, `ActiveCheckpoint`, `resume-response`,
  `checkpoint_type`, `resume_checkpoint`, checkpoint reason codes,
  `$slipway-learn`, and `$slipway-stats` have no non-historical leftovers.
- Focused tests covering command registration/help/toolgen/docs and preserved
  resume behavior pass.
- Required local checks from the A scope pass, including targeted Go packages,
  surface-manifest check, and the final governed review gates through S3.

## Open Questions
- [x] Confirm whether `run --resume` already re-dispatches tasks recorded as
  `blocked`/`incomplete`, or whether A must add that path so checkpoint removal
  has no human-input dead end. Resolution: research confirmed the existing
  non-checkpoint resume seam is S2 execution-summary readiness plus incomplete
  wave index; REQ-004 requires implementation to preserve it and add a focused
  blocked/incomplete rerun path if current behavior is insufficient.
- [x] Identify every checked-in generated surface refreshed by the repo-native
  toolchain for command deletion. Resolution: research identified toolgen
  command metadata, install profiles, templates, docs, manifest, and generated
  skill inventory expectations; task `t-05` owns those updates.

## Deferred Ideas
- Workstream B result import and run-version ownership.
- Workstream C evidence wording and suite-result documentation alignment.
- Future command naming changes for retained review/repair surfaces.

## Approved Summary
Confirmed by the user-provided objective on 2026-06-22: implement issue #297
Workstream A under Slipway governance through the end of S3, making best
available decisions on blockers. The change removes `checkpoint`, `learn`, and
`stats` as product/agent-facing command surfaces, deletes checkpoint-specific
resume state/protocol, preserves ledger-backed governance and `run --resume`,
and excludes Workstreams B and C except where needed to remove direct A-surface
references.
