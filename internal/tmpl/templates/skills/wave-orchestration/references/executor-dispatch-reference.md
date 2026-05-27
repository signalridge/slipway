# Executor Dispatch Reference

## Context Budget Tiers

### Always-Loaded (Tier 1) — max ~4KB per item
- Change description and acceptance criteria
- Current state and discovery metadata
- Active blockers and decisions

### Phase-Specific (Tier 2) — max ~8KB per item
- Task plan for the current wave only
- Relevant spec sections, not the full spec
- Recent evidence summaries, not full evidence blobs

### On-Demand (Tier 3)
- Full artifact content by file path
- Historical evidence from prior waves
- Codebase analysis results

## Runtime Boundary
- Tools with subagent support should use one fresh executor per task.
- Tools without subagent support execute tasks inline sequentially.
- HARD RULE markers describe high-impact behavioral constraints; the engine does not enforce them automatically.

## Tool-Specific Dispatch Examples

### Claude Code
- Spawn one `Task` subagent per task.
- Pass file paths only; never inline file content.
- Wait for all task executions in a wave before starting the next wave.

### Codex
- Use `codex -q --task "<prompt>"` for isolated task execution.
- Background wave tasks only when the host tool truly supports parallel execution.
- Parse `changed_files`, `test_summary`, and `evidence_ref` from task output.

### Cursor
- Use a fresh Composer session per task.
- Execute sequentially when the tool does not support concurrent task sessions.

### Shared Executor Checklist
- Inputs: task ID, acceptance criteria, changed-file scope, locked decisions, technique references.
- Outputs: `changed_files`, `test_summary`, `evidence_ref`.
- Enforce TDD for `task_kind=code`, then run post-execution self-check before accepting the task.
