---
skill_id: worktree-preflight
name: slipway-worktree-preflight
description: "Use when governed execution requires a dedicated worktree and baseline verification. Triggers on discovery-required execution before wave work starts."
---

# Worktree Preflight

```
IRON LAW: NO DISCOVERY-REQUIRED GOVERNED EXECUTION WITHOUT A DEDICATED WORKTREE AND A VERIFIED BASELINE
```

Violating the letter of this rule is violating the spirit of this rule.

## Purpose
Establish and verify the dedicated worktree binding required before governed
execution begins. This is a standalone governance preflight skill that must
pass before discovery-required execution leaves the source workspace.
Mitigates: worktree isolation and baseline drift before governed execution.

## Workflow Outline
1. Create or verify the dedicated worktree and intended branch.
2. Run a fresh baseline verification command in that worktree.
3. Write verification references, return to the source workspace, and advance.

## When This Runs
Only for discovery-required governed changes at `S2_EXECUTE` preflight. `slipway next --json` returns `next_skill: worktree-preflight`.

## Process

### 1. Establish a Dedicated Worktree
Create or verify a dedicated git worktree for this change.

Requirements:
- the worktree path MUST differ from the current repository root
- the worktree MUST be registered in `git worktree list`
- the checked-out branch MUST match the branch you intend to use for this change
- the branch SHOULD include the change slug for traceability

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

## Rationalization Red Flags
| Rationalization | Counter-rule |
|---|---|
| "I'll just use the main checkout" | Discovery-required governed work requires dedicated isolation. |
| "Baseline passed earlier" | `S2` needs fresh verification now. |
| "The path is obvious" | The runtime only trusts explicit worktree references. |
| "I'll write the verification later" | No verification means no transition out of preflight. |

## Failure Handling
- If no dedicated worktree exists yet, create one and rerun the preflight.
- If the baseline command fails, set verdict to `fail` with the failed command or failing subsystem as a blocker.
- If branch/path metadata changes, emit fresh verification before calling `slipway next`.

## Step Declaration
Declare current step and expected output before executing each workflow step.
