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

## Codebase Map Inputs
The coordinator does not inline durable codebase-map documents into its own
context. When task execution needs repository-map guidance, pass references and
metadata instead:
- `input_context.codebase_map_dir`
- relevant paths from `input_context.codebase_map_docs`
- `codebase_map_doc_states`

Each executor performs the relevance/staleness self-check before relying on a
map document. Treat `scaffold_only`, `baseline`, or `missing` doc states as
non-durable for that document, and report stale or irrelevant map guidance in
`test_summary` or blocker evidence. This preserves PR #112 while keeping bulky
map content out of the coordinator context.

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
- Inputs for map-aware tasks: `input_context.codebase_map_dir`, relevant `input_context.codebase_map_docs`, and `codebase_map_doc_states`.
- Outputs: `changed_files`, `test_summary`, `evidence_ref`.
- Enforce TDD for `task_kind=code`, then run post-execution self-check before accepting the task.
