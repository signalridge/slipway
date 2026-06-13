# Requirements

### Requirement: Engine-computed minimal wave assignment
REQ-001: `PlanWaves` MUST compute each task's wave from the dependency graph
instead of reading a declared wave: a task with no dependencies MUST be
assigned wave 1, and a task with dependencies MUST be assigned
`max(wave of each dependency) + 1` before conflict adjustment. The computed
plan MUST preserve the existing invariants: every wave 1..N is non-empty,
every dependency resolves to a strictly earlier wave, and `PlanWaves` MUST
reject dependency cycles and unknown dependency references with a hard error.

#### Scenario: Independent tasks share the first wave
GIVEN a tasks.md with tasks `t-01`, `t-02`, `t-03` that declare no
  `depends_on` and pairwise-disjoint `target_files`, and `t-04` declaring
  `depends_on: [t-01, t-02, t-03]`
WHEN the wave plan is materialized
THEN wave 1 contains `t-01`, `t-02`, `t-03` with `parallel: true` and wave 2
  contains only `t-04` with `parallel: false`

#### Scenario: Dependency chain still serializes
GIVEN tasks `t-01` ← `t-02` ← `t-03` forming a pure `depends_on` chain
WHEN the wave plan is materialized
THEN the plan has exactly three single-task waves in chain order, each
  `parallel: false`

#### Scenario: Dependency cycle is rejected
GIVEN tasks whose `depends_on` references form a cycle
WHEN wave assignment runs
THEN `PlanWaves` returns a hard error naming the cycle and no wave plan is
  produced

### Requirement: Deterministic conflict bumping
REQ-002: When two tasks would share a wave and their `target_files` conflict
under the existing semantics (exact path, parent/child scope, case-insensitive
alias, or glob overlap), the engine MUST bump the task-ID-later task to the
next wave until its wave is conflict-free. Assignment MUST process tasks in
task-ID-ordered topological order so that repeated materialization of the same
tasks.md content always produces an identical wave plan.

#### Scenario: Shared file forces a later wave deterministically
GIVEN independent tasks `t-01` and `t-02` that both list
  `internal/engine/wave/wave.go` in `target_files`
WHEN the wave plan is materialized repeatedly
THEN every materialization assigns `t-01` to wave 1 and `t-02` to wave 2,
  and no wave contains conflicting targets

#### Scenario: Glob and parent scopes count as conflicts
GIVEN independent tasks where one targets `internal/engine/wave/` (or a glob
  such as `internal/engine/wave/*.go`) and another targets
  `internal/engine/wave/parse.go`
WHEN the wave plan is materialized
THEN the two tasks are never assigned to the same wave

### Requirement: Declared wave metadata is retired fail-closed
REQ-003: The tasks.md parser MUST reject a task carrying a `wave:` metadata
line with a dedicated retirement error that names the task ID, states that the
engine now assigns waves from `depends_on` and `target_files`, and instructs
the author to delete the `wave:` line (declaring real `depends_on` for
intentional ordering). The parser MUST NOT silently ignore the key, MUST NOT
accept it as an override, and MUST NOT offer a compatibility window.

#### Scenario: Legacy wave line fails parsing with remediation
GIVEN a tasks.md whose task `t-02` carries `- wave: 2`
WHEN the tasks checklist is parsed by any consuming surface
THEN parsing fails with the retirement error naming `t-02`, the retirement
  of `wave:`, and the delete-the-line remediation, and no task plan is
  produced

### Requirement: Declared-wave validation vocabulary is retired
REQ-004: The plan validation layer MUST NOT require a declared wave on any
task: the `plan_dimension_execution_missing_wave` blocker MUST be removed from
the validation logic, the reason-code registry, and the remediation
vocabulary, and a tasks.md without any wave declarations MUST pass structural
validation when its other contract obligations are met.

#### Scenario: Waveless checklist passes structural validation
GIVEN a tasks.md whose tasks declare objectives, `depends_on`,
  `target_files`, `task_kind`, and `covers` but no `wave:` lines
WHEN `slipway validate` evaluates the tasks contract at plan-audit
  enforcement
THEN no missing-wave blocker is emitted and the checklist is structurally
  valid

### Requirement: Task-plan freshness hashes drop the wave input
REQ-005: The task-plan hash functions (structural, scope, semantic) MUST NOT
include a wave value in their input entries once waves are computed; hash
inputs MUST be exactly the authored contract fields so that identical authored
content always yields identical hashes. This is an intentional one-time hash
semantics change: previously stored hashes MAY mismatch recomputation once,
and recovery MUST flow through the existing public staleness paths
(re-materialization, `slipway repair`, or rescope) with no special-case shim.

#### Scenario: Hash stability for unchanged authored content
GIVEN a waveless tasks.md whose content does not change
WHEN structural, scope, and semantic hashes are computed repeatedly
THEN all three hashes are identical across runs and contain no wave-derived
  input

### Requirement: Planning surfaces teach the computed-wave contract
REQ-006: The `slipway instructions tasks` guidance and the plan-audit skill
MUST drop every instruction to author a `wave` value and MUST instead teach
the new drift vectors: `depends_on` declares real execution-order dependencies
only (a scheduling input, not narrative order); `target_files` SHALL name
precise files rather than directories or globs wherever feasible because broad
scopes flatten waves through conflict bumping; and small steps that must touch
the same file SHOULD be absorbed into one task rather than split into
serialized tasks. The plan-audit dependency dimension MUST audit dependency
necessity (no cycles, no fabricated or narrative-only dependencies), not
declared-wave ordering.

#### Scenario: Regenerated guidance carries the new contract
GIVEN the rewritten instructions guidance and plan-audit skill template
WHEN `slipway instructions tasks` runs and the plan-audit skill is
  regenerated
THEN neither surface asks the author for a `wave` value, both state that the
  engine assigns waves from `depends_on` and `target_files`, and the guidance
  includes the honest-dependency, precise-target, and same-file-absorption
  rules

### Requirement: Documentation and surface manifest stay aligned
REQ-007: User-facing workflow documentation MUST describe wave assignment as
engine-computed from `depends_on` and `target_files`, and any generated
surface manifests or template-pinning tests that encode the old authored-wave
contract MUST be regenerated or updated in the same change so the product
surface stays consistent.

#### Scenario: Workflow docs match engine behavior
GIVEN the updated documentation set
WHEN docs/workflow.md is read against the new engine behavior
THEN the wave-execution description states that Slipway computes waves from
  declared dependencies and target files and contains no instruction to author
  wave numbers
