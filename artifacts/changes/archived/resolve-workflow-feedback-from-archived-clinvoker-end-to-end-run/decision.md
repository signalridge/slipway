# Decision

## Project Context
- Tech Stack: Go CLI
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered
- Runtime-only minimum: update archived feedback dispositions without changing
  runtime behavior. Rejected because the same worktree, codebase-map, archive,
  stale-evidence, and catalog drift failures would remain reproducible.
- Prompt-only mitigation: document manual steps for agents to create worktrees,
  populate codebase maps, classify stale evidence, and cross-link archives.
  Rejected because the defects live in runtime/generator seams and should not be
  left to prompt discipline.
- Selected: make narrow runtime and generator fixes at each ownership seam:
  default worktree binding in `state/new`, codebase-map population in
  `artifact`, archive relationship metadata in `model/done`, stale-evidence
  reason classification in `state`, and catalog thinning in `toolgen`.

## Selected Approach
Implement the selected ownership-seam fixes with focused regression coverage and
then complete the governed workflow. This preserves existing workflow phases and
does not introduce alternate JSON modes or broad lifecycle redesign.

## Interfaces and Data Flow
- `slipway new --json` adds `worktree_path`, `worktree_branch`,
  `worktree_created`, and `worktree_skipped_reason` fields so callers can see
  early binding or explicit skip reasons.
- `change.yaml` gains `remediation_sources` as durable metadata for archived
  feedback remediation.
- `slipway done --json` reports `archive_path`, `archive_kind`, and
  `remediation_sources`.
- `ReasonCode` output can now report `stale_planning_evidence` separately from
  `stale_execution_evidence`.
- Generated catalog artifacts replace copied `## Full Instructions` bodies with
  `## Instruction Authority` pointers.

## Rollout and Rollback
- Rollout: land code, templates, generated surfaces, feedback dispositions, and
  governed evidence together after focused tests, full tests, and build pass.
- Rollback: revert this change set; no external data migration is required. Any
  generated catalog surfaces can be regenerated from the reverted toolgen.
- Compatibility: no required operator configuration is added; no-head and
  non-Git workspaces skip default worktree creation instead of failing.

## Risk
- External API risk: JSON output gains additive fields for `new` and `done`, and
  reason-code text adds a new stale planning code. Focused tests cover these
  contract changes.
- Workflow risk: default worktree creation can fail if branch/path conflicts;
  the state helper validates existing metadata and returns explicit skip/error
  behavior.
- Governance risk: stale-evidence classification must not hide unsafe drift;
  planning artifacts still block as `stale_planning_evidence`, and unreadable
  freshness artifacts still fail closed as execution evidence blockers.
