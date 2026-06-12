# Requirements

## Requirements

### Requirement: Parallel waves require executor fan-out

REQ-001: The generated `wave-orchestration` surface MUST state that a
`parallel: true` wave requires one executor subagent per planned task when the
runtime has a real subagent or fresh-session primitive.

#### Scenario: Capable runtime dispatches a parallel wave

GIVEN `slipway next --json` marks a multi-task wave as `parallel: true`
WHEN a host follows the generated `wave-orchestration` skill
THEN the host attempts one executor subagent per task, collects executor handles,
waits for the wave, parses structured results, records task evidence, and runs a
post-wave integration gate before the next wave.

### Requirement: Codex uses spawn_agent semantics

REQ-002: The generated Codex dispatch guidance MUST map executor fan-out to a
`spawn_agent`-style runtime adapter, including deferred tool discovery,
fresh-context behavior such as `fork_context: false`, collecting agent IDs,
waiting for all results, parsing results, and closing agents when close
semantics exist.

#### Scenario: Codex generated guidance is inspected

GIVEN a generated or rendered Codex `wave-orchestration` surface
WHEN the Codex dispatch section is read
THEN `spawn_agent`, `tool_search`, `fork_context: false`, wait, collect, parse,
and close semantics are present, and `codex -q --task` is not presented as the
primary executor fan-out path.

### Requirement: Capable runtimes cannot silently inline parallel waves

REQ-003: The generated dispatch contract SHALL fail closed or require operator
direction when a runtime has a subagent primitive but spawning fails, is
unavailable after discovery, or would not provide the isolation level the
surface claims.

#### Scenario: Capable runtime cannot complete subagent dispatch

GIVEN a `parallel: true` wave and a runtime that normally supports executor
subagents
WHEN executor spawning fails or cannot provide the claimed isolation
THEN the generated guidance blocks or asks for operator direction instead of
silently executing the whole wave inline in the coordinator context.

### Requirement: Incapable runtimes report structured degradation

REQ-004: The generated dispatch contract MUST distinguish genuinely incapable
runtimes from capable runtime failures, and incapable runtimes MUST report
unsupported or degraded sequential dispatch explicitly.

#### Scenario: Runtime lacks a fresh executor primitive

GIVEN a `parallel: true` wave and a runtime with no subagent or fresh-session
primitive
WHEN the host cannot dispatch isolated executors
THEN it records structured dispatch metadata such as
`dispatch_mode:wave=<n>:degraded_sequential`, states the limitation in notes,
and stops for operator direction when inline execution would pollute coordinator
context.

### Requirement: Executor result and evidence ownership are stable

REQ-005: The generated surface MUST name a stable executor result contract with
`task_id`, `verdict`, `changed_files`, `test_summary`, `evidence_ref`,
`blockers`, and concise notes, and MUST preserve `slipway evidence task` as the
task evidence ledger writer.

#### Scenario: Executor completes a planned task

GIVEN an executor returns from a dispatched task
WHEN the coordinator accepts the result
THEN the result includes the required fields, the coordinator records evidence
through `slipway evidence task`, and no subagent is instructed to self-stamp
governed freshness or write verification YAML directly.

### Requirement: Generated-surface tests prevent regression

REQ-006: The repository MUST include contract tests covering the generated and
rendered wave-orchestration surfaces so the Codex adapter cannot regress to
shell/prose-only dispatch.

#### Scenario: Contract tests run

GIVEN the template/reference changes are implemented
WHEN `go test -count=1 ./internal/tmpl ./internal/toolgen` runs
THEN the tests prove the required dispatch, fallback, result-contract, and
evidence-ownership wording is present and the old primary `codex -q --task`
path is absent.

### Requirement: Single worktree dispatch preflights target conflicts

REQ-007: The generated dispatch contract MUST state that parallel executor
agents share the current worktree and therefore require a cheap pre-dispatch
target-overlap backstop for the current `parallel: true` wave before spawning.

#### Scenario: Target overlap is detected before dispatch

GIVEN a `parallel: true` wave whose tasks no longer appear target-disjoint
WHEN the coordinator preflights the current wave's task `target_files`
THEN it must not spawn those tasks in parallel, must record a blocking
single-worktree dispatch conflict or rescope/replan, and must not treat notes
alone as conflict evidence.

### Requirement: Dispatch evidence names executor handles

REQ-008: The generated dispatch contract MUST require per-task executor
agent/session references for every spawned task in a `parallel: true` wave so
review can prove which executor handled which planned task.

#### Scenario: Parallel single worktree wave is dispatched

GIVEN a `parallel: true` wave is dispatched through runtime subagents
WHEN the coordinator records wave-orchestration references
THEN the references include a structured dispatch mode and one stable
`executor_agent:wave=<n>:task=<id>:<handle>` or equivalent per-task executor
handle before task evidence is accepted.

### Requirement: Wait and stalled-executor recovery is explicit

REQ-009: The generated dispatch contract MUST define bounded wait semantics for
spawned executors, including missing result, lost handle, and stalled executor
recovery paths.

#### Scenario: Spawned executor does not return a parseable result

GIVEN one or more executor agents are active for a `parallel: true` wave
WHEN an executor handle is lost, no parseable result is returned, or the wait
stalls past the host's reasonable timeout
THEN the coordinator records an executor-dispatch blocker with known handles and
last observed state, asks for operator direction, and does not inline or mark
the task complete without explicit authorization.

### Requirement: Codex authorization boundary is fail-closed

REQ-010: The generated Codex guidance MUST state that when the available Codex
runtime requires explicit user authorization for subagent spawning and the
current invocation lacks it, the coordinator stops and asks rather than forcing
`spawn_agent` or silently executing inline.

#### Scenario: Codex spawn requires authorization

GIVEN a `parallel: true` wave in Codex and `spawn_agent` exists but runtime
policy requires explicit user authorization
WHEN the coordinator has not received that authorization
THEN the coordinator asks for operator direction and records the pending
authorization boundary instead of claiming degraded sequential execution or
same-context completion.
