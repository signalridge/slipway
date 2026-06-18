# Research

## Current Findings

- Cobra commands are registered in `cmd/root.go`; command descriptions and
  adapter metadata are centralized in `internal/toolgen/toolgen.go`.
- `cmd/run.go` already drives the same `nextView` contract used by `next`; stage
  commands can reuse that machinery with state guards instead of duplicating the
  progression engine.
- Generated command bodies live in
  `internal/tmpl/templates/_partials/command-*-body.tmpl`.
- Surface manifest rows are generated from toolgen authorities via
  `go run ./internal/toolgen/cmd/gen-surface-manifest --write`.
- Scope drift guidance is produced in `internal/engine/progression/readiness.go`
  and recovery summaries are produced from `internal/model/recovery.go`.
- S3 review repair needs a public dispatch surface because recovery guidance that
  says "use a repair subagent" but points back to `slipway review` is a dead-end.
- S3 reviewer context independence is consumed from verification references via
  `context_origin:stage=review=<handle>`; a review repair can carry
  `context_origin:stage=fix=<handle>` on affected reviewer rereview evidence.
- `suite-result.yaml` provides shared reviewer input digests, including
  `suite-result:full_suite`, so current code repairs can invalidate all selected
  reviewers rather than a file-scoped subset.
- Wave-to-wave execution behavior is governed by the wave-orchestration skill
  template and executor dispatch reference, not by a separate CLI transition
  between waves.

## Design Implications

- The least risky implementation path is to add explicit stage command wrappers
  around the existing progression path, passing the calling command into
  lifecycle trace events.
- `run` should annotate JSON with its delegated primary command so callers know
  which stage it drove.
- Current-change amendment language belongs in recovery guidance and generated
  command surfaces.
- Review finding repair belongs in `slipway fix`, not `slipway repair`, so the
  local-integrity repair command remains narrow and agents have a clean S3 repair
  entrypoint.
- Goal-verification should remain part of the selected S3 peer set. The public
  templates must not ask agents to record retired goal/closeout-stage
  context-origin references.
- Retired repair command surfaces should be deleted from code and generated
  inventories; retaining aliases would keep teaching agents the old model.

## Alternatives Considered

- Introduce a new top-level command for scope changes. This was rejected because
  same-intent amendment is part of the current lifecycle authority, not a new
  lifecycle branch.
- Keep the stale-evidence recovery state machine. This was rejected because the
  new model treats stale evidence as repair guidance and review convergence,
  not as a state rewind.
- Keep the S2 public name as execute. This was rejected because implement names
  the user-facing work more clearly and matches the explicit `slipway implement`
  command.
- Promise file-scoped reviewer reruns. This was rejected for this change because
  reviewer inputs currently include shared suite and workspace-diff digests; the
  reliable behavior is to stale the full selected set when ownership cannot be
  proven narrower.

## Unknowns

- Whether the active governed bundle will need a separate follow-up to refresh
  all task evidence after this self-hosted redesign. This implementation keeps
  the code and artifacts aligned, while stale runtime evidence remains a normal
  S2+ refresh concern.
- Whether historical archived bundles should be migrated from retired wording.
  This change keeps archived records historical and normalizes only current
  public rendering where necessary.
- Whether a future change should implement file-scoped reviewer ownership to
  reduce S3 repair cost after code edits.

## Assumptions

- Same-intent scope changes should stay inside the active change and be verified
  by review, not routed through a separate command.
- Intent conflicts should start a new governed change rather than mutate the
  current authority.
- `recovery` remains useful as a read-only remediation object, but stale evidence
  repair must not mutate lifecycle state backward.
- `fix` is a repair-dispatch command, not a proof source; reviewers still own the
  pass/fail verdict.
- S3 task-plan amendments must not be projected as S2 execution staleness. Raw
  tasks.md -> wave-plan/execution-summary drift remains inspectable, but public
  S3 status/validate/next/closeout surfaces treat task-plan-only drift as
  review/fix input so task alignment can converge.

## Canonical References

- `cmd/root.go`, `cmd/stage.go`, and `cmd/fix.go` define the explicit command
  surfaces.
- `internal/engine/progression/evidence_repair.go` owns stale-evidence repair
  target detection.
- `internal/engine/progression/evidence_digests.go` and
  `internal/model/evidence_digests.go` own the suite-result reviewer digest
  keystone.
- `internal/model/context_attestation.go` and
  `internal/engine/progression/authority.go` own review/fix context-origin
  distinctness.
- `internal/model/recovery.go` owns recovery/remediation summaries.
- `internal/toolgen/toolgen.go` and `docs/SURFACE-MANIFEST.json` define generated
  adapter surfaces.
