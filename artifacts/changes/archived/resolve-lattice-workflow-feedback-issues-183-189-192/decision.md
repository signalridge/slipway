# Decision

## Alternatives Considered

### Option A: Narrow CLI-surface repair
Resolve the active change workspace for `--notes-file`, improve wrong-state
remediation for post-S2 evidence attempts, and clarify checkpoint-vs-skill
handoff text. This keeps all existing lifecycle states and error codes.

Tradeoff: it does not create a new post-review evidence command, but it removes
the misleading guidance that caused the Lattice workflow friction.

### Option B: Add a post-review execution evidence refresh command
Create a new command or evidence category that can explicitly refresh execution
evidence after review-driven repairs.

Tradeoff: this may be useful later, but it expands lifecycle semantics and
requires a broader evidence model. It is larger than the current issue reports.

### Option C: Treat delegated autonomy as fresh intake confirmation
Allow the original user objective to satisfy later skill-handoff confirmation.

Tradeoff: this weakens the current hard-stop confirmation boundary and risks a
bypass path for governance skills.

## Selected Approach

Select Option A. It directly fixes issues #183, #189, and #192 while preserving
fail-closed lifecycle behavior:

- #183 is fixed by resolving notes-file paths against the active change's
  authoritative workspace.
- #192 is fixed by making wrong-state evidence errors point to the accepted
  S3/S4 evidence path for review-driven repairs.
- #189 is fixed by making the JSON/action surface and `--resume-response`
  remediation explicit that missing skill evidence is not checkpoint resume.

## Interfaces and Data Flow

Changed command-layer data flow:

- `makeEvidenceSkillCmd` loads the active change before notes-file resolution.
- `resolveEvidenceSkillNotes` receives the active change and resolves
  `--notes-file` against `state.WorkspaceRootForChange(root, change)`.
- `validateEvidenceSkillStage` keeps existing validation and error codes but
  uses a post-S2 remediation helper when the requested evidence belongs to an
  earlier lifecycle state.
- `confirmation_requirement.next_action` remains prose but becomes more precise
  for `skill_handoff:*` reasons.
- `no_active_checkpoint` keeps its error code and category but gets clearer
  remediation.

No model schema, persisted state shape, or lifecycle transition changes are
planned.

## Rollout and Rollback

Rollout:

- Land command-layer code and focused regression tests together.
- Verify with `go test ./cmd`, `go test ./...`, `git diff --check`, and current
  Slipway validation/next evidence.

Rollback:

- Revert the code and test changes in this branch.
- No data migration rollback is required because only command behavior and tests
  change.

## Risk

- Automation compatibility risk is limited because existing error codes and JSON
  field names remain stable.
- Path authority risk is mitigated by using the existing bound-workspace
  resolver and preserving path validation.
- Governance weakening risk is avoided by keeping `resume_response_supported`
  false for skill handoffs and by not creating bypass/force-close behavior.
