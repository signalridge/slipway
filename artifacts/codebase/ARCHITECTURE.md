# Architecture

Re-authored for change
`resolve-github-issue-155-knuth-invariant-overwrite-only-own`
(GitHub issue #155).

Question: which Slipway freshness/digest seams should classify prose artifact
edits as engine-default/scaffold versus human material while preserving
fail-closed reopen behavior?

## Affected Seams

- `internal/engine/progression/evidence_digests.go:36` through
  `internal/engine/progression/evidence_digests.go:79` is the skill input
  digest switch. `intake-clarification` consumes `intent.md`,
  `research-orchestration` consumes `intent.md` plus `research.md`, and
  `plan-audit` consumes the planning artifacts.
- `internal/engine/progression/evidence_digests.go:388` through
  `internal/engine/progression/evidence_digests.go:423` is the plan-audit
  artifact input collector. It hashes `intent.md`, `requirements.md`,
  `research.md`, and `decision.md`, while `tasks.md` already uses
  `wave.TaskPlanStructuralHash`.
- `internal/engine/progression/evidence_digests.go:426` through
  `internal/engine/progression/evidence_digests.go:448` is the prose file
  digest seam. `computeProseFileInputHash` currently hashes raw content, so
  authoring comments and scaffold-only sections can churn evidence digests.
- `internal/engine/progression/stale_evidence_recovery.go:59` through
  `internal/engine/progression/stale_evidence_recovery.go:93` turns digest
  drift into the earliest stale evidence recovery target. A digest false
  positive here reopens earlier lifecycle stages.
- `internal/engine/artifact/manager.go:152` through
  `internal/engine/artifact/manager.go:167` exposes artifact templates, and
  `internal/engine/artifact/manager.go:329` through
  `internal/engine/artifact/manager.go:349` documents which artifacts are
  skill-authored rather than always engine-scaffolded.

## Dependency Flow

Governed artifacts live in `artifacts/changes/<slug>/`. Skill verification is
accepted through `slipway evidence skill`, which stamps current input digests.
Later readiness checks recompute those digests and compare named inputs; changed
inputs become `required_skill_stale:<skill>:<artifact>` blockers that can reopen
the change to the earliest affected stage.

## Constraints And Invariants

- Evidence freshness remains fail-closed. Unknown or material content must keep
  changing the digest and reopening stale evidence.
- `tasks.md` structural hashing is intentionally separate and already excludes
  checkbox-only and `target_files`-only churn for plan-audit.
- Prose artifact materiality should be derived from repo-owned scaffold/template
  knowledge, not from mutable file timestamps.
- GSD is a behavioral reference only. Slipway must not gain a runtime dependency
  on the local GSD checkout.
