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
`test_summary` or blocker evidence. This preserves the codebase-map
relevance/staleness self-check while keeping bulky map content out of the
coordinator context.

## Runtime Boundary
- A `parallel: true` wave (from `slipway next --json`) is dispatched concurrently by default: one fresh executor per task, spawned together, then wait for the whole wave.
- A capable runtime must attempt real executor subagent fan-out for `parallel: true`. If spawning, waiting, result collection, parsing, or cleanup fails, the host must not silently execute the wave inline in the coordinator context; stop and ask for operator direction or record a blocking executor-dispatch failure.
- Run a wave sequentially only when it is `parallel: false`, or when the host has no concurrent-executor support. In the latter case note the degradation in the wave report and record both `dispatch_mode:wave=<wave_index>:degraded_sequential` and `degraded_dispatch_justification:wave=<wave_index>:tool_unavailable=<detail>` in the wave-orchestration verification references. Notes/prose alone are human-readable context and are not parsed as dispatch evidence. If inline execution would pollute coordinator context and the user has not authorized it, stop rather than pretending parallel dispatch happened.
- Executors share a single worktree unless the runtime explicitly provides stronger isolation. Do not run shared-worktree-wide integration commands such as `go build ./...` concurrently inside each task executor; leave merged-state build/test/lint checks to the post-wave integration gate unless a task owns a genuinely isolated command.
- Before spawning a `parallel: true` wave, run a target-overlap preflight over the current wave's `target_files`. If two tasks overlap by exact path, path alias, parent/child scope, case-insensitive match, or glob scope, record `dispatch_blocker:wave=<wave_index>:target_overlap` and stop for explicit operator direction before continuing.
- After executor results return, run a post-result changed-file conflict check across returned `changed_files`. If two parallel executors touched the same file scope or changed files outside declared task targets, record a post-result changed-file conflict and stop before the post-wave integration gate.
- After all executors in a wave finish, run a post-wave integration gate on the merged current worktree before starting the next wave. Use the narrowest meaningful build/test command for code/test waves, the relevant formatter/lint/validation for docs/config/ops-only waves, or record why no executable gate exists. When it passes, continue directly to the next ready wave in the same host run; do not require another `slipway run` just to cross a wave boundary.
- HARD RULE markers describe high-impact behavioral constraints; the engine does not enforce them automatically.

## Dispatch Evidence
- Successful `parallel: true` fan-out records `dispatch_mode:wave=<wave_index>:parallel_subagents`.
- Record one stable executor handle reference per spawned task: `executor_agent:wave=<wave_index>:task=<task_id>:<handle>`.
- Degraded sequential fallback records both `dispatch_mode:wave=<wave_index>:degraded_sequential` and `degraded_dispatch_justification:wave=<wave_index>:tool_unavailable=<detail>`.
- A target-overlap preflight failure records `dispatch_blocker:wave=<wave_index>:target_overlap`.
- Notes explain the decision, but structured references are the reviewable dispatch evidence.

## Target/Changed-File Safety Model And Engine Blockers
- A wave fans out into a single shared worktree; there is no per-executor git
  worktree isolation. Each task's declared `target_files` plus each executor's
  exhaustive `changed_files` are the safety model — that is the only boundary
  preventing same-wave executors from clobbering each other's writes.
- The engine signals, records, and gates on this evidence; it does not spawn or
  execute agents (this host owns the dispatch). When recorded evidence is
  missing or violates the model, the engine records an open blocker and the
  change fails closed to rerun, review, and corrected evidence.
- `task_changed_file_scope_escape:<task_id>:<file>` — a task's `changed_files`
  escaped its declared `target_files`. The engine audits this after results, so
  the target-overlap preflight and the post-result changed-file conflict check
  keep you clear of it; recover by fixing `target_files` or the change and
  re-running the owning lifecycle step. At S3 with frozen task evidence, restore
  honest target coverage or explicitly discard prior task evidence before
  reexecution.
- `parallel_wave_changed_file_overlap:wave=<wave_index>:file=<file>:tasks=<tasks>` — two tasks
  in the same `parallel: true` wave wrote the same file. Sequential waves
  sharing a file are allowed; a parallel overlap is an execution-safety stop
  point requiring operator direction.
- `dispatch_mode_absent_on_started_parallel_wave:<wave_index>` — a started
  parallel wave recorded no dispatch-mode evidence. Silent parallel inference is
  gone: record `dispatch_mode:wave=<wave_index>:parallel_subagents`, or record
  `dispatch_mode:wave=<wave_index>:degraded_sequential` together with
  `degraded_dispatch_justification:wave=<wave_index>:tool_unavailable=<detail>`
  for the wave.
- `executor_agent_missing:wave=<wave_index>:task=<task_id>` — a `parallel_subagents` wave
  is missing a per-task executor handle. Record exactly one
  `executor_agent:wave=<wave_index>:task=<task_id>:<handle>` per planned task;
  `degraded_sequential` and non-parallel waves require no handles.

## Wait And Recovery
- Wait for every spawned executor handle before accepting the wave.
- If an executor handle is lost, record `executor_handle_lost` with the known handles and last observed state, then ask for operator direction.
- If an executor returns no parseable result contract, record `executor_result_missing` and do not infer success from narration.
- If waiting stalls beyond the host's reasonable timeout or progress signal, record `executor_dispatch_stalled`, report active handles and last observed state, and ask whether to keep waiting, retry, or switch to an explicitly authorized recovery path.
- Do not inline the task, mark task evidence pass, or start the next wave while any spawned executor result is missing.

## Tool-Specific Dispatch Examples

### Claude Code
- Spawn one `Task` subagent per task.
- Pass file paths only; never inline file content.
- Wait for all task executions in a wave before starting the next wave.
- Use background fan-out only when the runtime supports it safely. If worktree-creating agents are involved and the runtime can race on `.git/config.lock`, spawn calls one at a time while allowing agents to run concurrently after creation.
- Do not wrap a spawner workflow inside another subagent when that prevents nested executor spawning.

### Codex
- Use a `spawn_agent` runtime adapter for executor fan-out. If `spawn_agent` is not currently visible, use deferred tool discovery such as `tool_search` before deciding the runtime lacks the primitive.
- Spawn one agent per planned task with a bounded executor message and fresh-context behavior such as `fork_context: false` where the available Codex tool supports it.
- Pass task IDs, acceptance criteria, target file paths, locked decisions, codebase-map paths, and evidence instructions; do not inline source file contents.
- Spawn each executor, then collect agent IDs, wait for all results, parse each result into the shared executor result contract, and close each agent after result collection when the runtime exposes close semantics.
- If Codex requires explicit user authorization for `spawn_agent` and the current invocation lacks that authorization, stop and ask for it. Do not force the spawn, and do not claim degraded sequential or same-context completion unless the user explicitly authorizes that recovery path.
- Codex `spawn_agent` has no direct mapping for Claude-style `isolation="worktree"`. If the surface claims worktree isolation or the task requires unavailable isolation, fail closed or ask for operator direction instead of running workspace-write executors inline.
- While agents are active, do not read files, edit code, or run tests for their tasks in the coordinator context.

### Cursor
- Use a fresh Composer session per task.
- Use a real fresh-session or subagent primitive when present.
- Execute sequentially only when the tool does not support concurrent task sessions, and record the degraded dispatch mode as described above.

### Shared Executor Checklist
- Inputs: task ID, acceptance criteria, changed-file scope, locked decisions, technique references.
- Inputs for map-aware tasks: `input_context.codebase_map_dir`, relevant `input_context.codebase_map_docs`, and `codebase_map_doc_states`.
- Outputs: `task_id`, `verdict`, `changed_files`, optional `no_op_justification` (only for a pass code task that changed zero files), `test_summary`, `evidence_ref`, `blockers`, and concise `notes`.
- Executor result acceptance requires a matching `executor_agent:wave=<wave_index>:task=<task_id>:<handle>` dispatch reference.
- Enforce TDD for `task_kind=code`, then run post-execution self-check before accepting the task.
- Executors may report evidence refs, but `slipway evidence task` remains the only supported task evidence ledger writer. Executor agents must not self-stamp `captured_at`, freshness inputs, or governed verification YAML.
