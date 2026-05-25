---
skill_id: worktree-preflight
name: slipway-worktree-preflight
description: "Use when governed execution requires a dedicated worktree and baseline verification. Triggers on missing, invalid, or operator-supplied worktree bindings after automatic early binding is unavailable."
---

# Worktree Preflight

```
IRON LAW: NO DISCOVERY-REQUIRED GOVERNED EXECUTION WITHOUT A DEDICATED WORKTREE AND A VERIFIED BASELINE
```

## Purpose
Verify or repair the dedicated worktree binding required before governed
execution begins. `slipway new` creates the default `.worktrees/<slug>` binding
early when Git has a usable HEAD; this standalone governance preflight handles
the remaining cases where binding is missing, invalid, or operator-supplied.
Mitigates: worktree isolation and baseline drift before governed execution.

## Workflow Outline
1. Verify the dedicated worktree and intended branch, creating it only if the
   runtime did not already bind the default worktree.
2. Run a fresh baseline verification command in that worktree.
3. Write verification references, return to the source workspace, and advance.

## When This Runs
For discovery-required governed changes, the normal path is early automatic
binding during `slipway new`. If `slipway next --json` later returns
`next_skill: worktree-preflight`, treat that as a repair/preflight path for a
missing or invalid binding before wave execution.

## Process

### 1. Establish a Dedicated Worktree
Verify the dedicated git worktree for this change. If the runtime did not create
one, create it using the repo-local default unless project policy or the
operator explicitly chooses another path.

Requirements:
- the worktree path MUST differ from the current repository root
- the worktree MUST be registered in `git worktree list`
- the checked-out branch MUST match the branch you intend to use for this change
- the branch SHOULD include the change slug for traceability

Default path policy: when no operator-supplied path or project policy says
otherwise, use the repo-local ignored path `.worktrees/<slug>` under the source
checkout and branch `feat/<slug>`. Sibling or external worktree paths are valid
only when chosen explicitly by the operator or local project policy.

### 2. Verify a Clean Baseline
Run the project's baseline verification command inside that dedicated worktree before implementation begins.

At minimum:
- confirm the command exits successfully
- capture the exact verification command you ran
- if baseline fails, set verdict to `fail` and record the failure as a blocker

### 3. Write Verification
Write a governance verification record with references that the runtime can parse:

```yaml
# Write to: artifacts/changes/{slug}/verification/worktree-preflight.yaml
verdict: pass
blockers: []
timestamp: "<ISO-8601-UTC>"
run_version: 0
references:
  - "worktree_path:/absolute/path/to/worktree"
  - "worktree_branch:feat/change-slug"
  - "baseline_verify_cmd:go test ./..."
notes: |
  <verification notes>
```

The runtime will reject `pass` verification if any of these references is missing.

### 4. Advance
After verification is written, return to the source workspace and run:

```bash
slipway next
```

<HARD-GATE>Do not run `slipway next` until the dedicated worktree exists, the baseline verification command has passed, and the verification references are complete.</HARD-GATE>

The runtime will validate the worktree binding and persist `worktree_path` and `worktree_branch` before advancing.

## DO NOT SKIP
1. Use a dedicated worktree, not the primary workspace root.
2. Run a fresh baseline verification command before writing `pass` verification.
3. Record absolute worktree path and exact branch name in the verification references.

## Failure Handling
- If no dedicated worktree exists yet, create `.worktrees/<slug>` on
  `feat/<slug>` unless an explicit operator/project override says otherwise,
  then rerun the preflight.
- If the baseline command fails, set verdict to `fail` with the failed command or failing subsystem as a blocker.
- If branch/path metadata changes, emit fresh verification before calling `slipway next`.
