## Context

SpecLane is a governance-aware AI workflow CLI. The agreed MVP target is pragmatic:
- keep deterministic routing and execution states
- avoid enterprise-grade evidence bureaucracy
- gate on executable facts and explicit human decisions

## Goals / Non-Goals

### Goals

- Keep canonical states `S0..S8` + `DONE`
- Keep `L1/L2/L3` risk-based routing
- Keep admission vs governed runtime split
- Keep governed artifact bundle under `aircraft/changes/<slug>/`
- Simplify gate logic to command checks + human confirmations
- Use one flat run record: `.speclane/runs/<request_id>.yaml`

### Non-Goals

- Governance-skill evidence schema and indexing
- Reviewer `session_id` independence enforcement
- Multi-layer policy/registry snapshot system
- Runtime DB

## Decisions

### DEC-01: Keep Route + Execution Skeleton

- Admission phase: `S0_INTAKE -> S1_ANALYZE`
- Execution phase:
  - `L1`: `S6 -> S7 -> S8 -> DONE`
  - `L2`: `S4 -> S5 -> S6 -> S7 -> S8 -> DONE`
  - `L3`: `S2 -> S3 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`

`S8 -> DONE` remains command-gated via `speclane done`.

### DEC-02: Keep Runtime State Ownership Split

- admission/direct lane: `.speclane/runtime/admissions/<request_id>.yaml`
- governed lane: `.speclane/runtime/changes/<request_id>.yaml`
- governed artifacts: `aircraft/changes/<slug>/`

After governed handoff, admission becomes sealed snapshot.

### DEC-03: Replace Governance-Skill Evidence with Gate Checks

Do not use `evidence/skills` as gate input.

Gate inputs are:
1. command checks (machine-verifiable)
2. explicit human confirmations

No gate relies on AI self-attested `verdict` file existence.

### DEC-04: Flat Run Record

Use a single run record per request:
- `.speclane/runs/<request_id>.yaml`

This file stores:
- command check results
- human confirmations
- wave/task execution summary
- run-ledger event history (check/wave/checkpoint related)

Run record is a ledger, not a third lifecycle state source:
- authoritative `current_state`/level/lifecycle status remain in admission/change state files
- terminal archive moves run record to `.speclane/archive/runs/<request_id>.yaml`

No separate `evidence/skills`, `evidence/tasks`, `evidence/runs` directories are required by MVP.

### DEC-05: Minimal Gate Contracts

- `G_scope` (L3 only):
  - `explore.md` required sections present
  - worktree metadata valid (`worktree_path`, `worktree_branch`)
  - human `scope_confirmed=y`
- `G_plan` (L2/L3):
  - governed planning artifacts present and non-stale
  - `openspec validate <change>` passes
  - human `execute_ready=y`
- `G_pivot`:
  - rule gate in MVP (entry-state + analyze-first + pivot-kind validity)
  - no catalog check IDs in MVP
- `G_ship` (L2/L3):
  - required command checks pass
  - human `review_done=y`
  - human `ship_ready=y`

Gate input terminology is `check` only:
- `command_check`
- `human_confirmation`

No gate input object uses `skill_name`.

### DEC-06: Command Check Baseline

Default ship checks:
- `go test ./...` (when code delta exists)
- `golangci-lint run` (when code delta exists)
- `grep -n "^- \[ \]" tasks.md` (must be empty for governed ship)

Plan check:
- `openspec validate <change>`

### DEC-07: Human Decision Points

- `execute_ready`: `Is execution ready? [y/n]`
- `review_done`: `Is review complete? [y/n]`
- `ship_ready`: `Ready to ship? [y/n]`
- L3 adds `scope_confirmed`: `Is scope confirmed? [y/n]`

Prompt localization rule:
- canonical prompt templates are authored in English
- runtime/AI layer MAY localize prompt rendering based on user language
- persisted confirmation identity SHALL remain `check_id`-based

Human decision records are persisted in run YAML.

### DEC-08: User Override for Failed Command Checks

Default rule: failed required command checks block progression.

Operator may continue with explicit override:
- failing check result is shown first
- operator confirms override (`y`)
- optional reason is recorded in run YAML

MVP override intentionally excludes policy layering (no role model, no dual approval, no snapshot policy objects).

### DEC-09: Routing and Locking Contracts Stay

Retain:
- fixed-level safety blocking before persistence
- single-active request resolution in MVP
- bounded lock wait and explicit `repair`
- request-scoped archive behavior for `done` and `cancel`
- `non_speclane` as successful classification outcome (`speclane new` exit `0`, no runtime writes)

## Data Models (MVP Contract)

### Runtime Config (`.speclane/config.yaml`)

```go
type SpecLaneConfig struct {
    Tools     []string        `yaml:"tools"`
    Defaults  ConfigDefaults  `yaml:"defaults"`
    Execution ConfigExecution `yaml:"execution"`
    Unknown   map[string]any  `yaml:",inline"`
}

type ConfigDefaults struct {
    LevelMode string `yaml:"level_mode"` // auto|L1|L2|L3
}

type ConfigExecution struct {
    Parallelization        bool `yaml:"parallelization"`
    MaxRetriesPerTask      int  `yaml:"max_retries_per_task"`
    LockWaitTimeoutSeconds int  `yaml:"lock_wait_timeout_seconds"`
    CancelGracePeriodSeconds int `yaml:"cancel_grace_period_seconds"`
}
```

### Admission State

```go
type AdmissionState struct {
    RequestID         string              `yaml:"request_id"`
    Title             string              `yaml:"title"`
    AdmissionStatus   AdmissionStatus     `yaml:"admission_status"`
    IntakeAssessment  IntakeAssessment    `yaml:"intake_assessment"`
    Level             Level               `yaml:"level"`
    LevelSource       LevelSource         `yaml:"level_source"`
    LevelHistory      []LevelHistoryEvent `yaml:"level_history"`
    LastLevelUpdateAt time.Time           `yaml:"last_level_update_at"`
    RouteSnapshot     RouteSnapshot       `yaml:"route_snapshot"`
    CurrentState      string              `yaml:"current_state"`
    TaskRuns          []TaskRun           `yaml:"task_runs,omitempty"`
    ActionHistory     []ActionEvent       `yaml:"action_history,omitempty"`
    SealedAt          *time.Time          `yaml:"sealed_at,omitempty"`
    CreatedAt         time.Time           `yaml:"created_at"`
    UpdatedAt         time.Time           `yaml:"updated_at"`
}
```

### Governed Runtime Change State

```go
type ChangeState struct {
    RequestID         string                    `yaml:"request_id"`
    Slug              string                    `yaml:"slug"`
    Title             string                    `yaml:"title"`
    ChangeStatus      ChangeStatus              `yaml:"change_status"`
    Level             Level                     `yaml:"level"`
    LevelSource       LevelSource               `yaml:"level_source"`
    LevelHistory      []LevelHistoryEvent       `yaml:"level_history"`
    LastLevelUpdateAt time.Time                 `yaml:"last_level_update_at"`
    RouteSnapshot     RouteSnapshot             `yaml:"route_snapshot"`
    CurrentState      string                    `yaml:"current_state"`
    WorktreePath      string                    `yaml:"worktree_path,omitempty"`
    WorktreeBranch    string                    `yaml:"worktree_branch,omitempty"`
    Artifacts         map[string]*ArtifactState `yaml:"artifacts"`
    Gates             map[string]GateStatus     `yaml:"gates"`
    TaskRuns          []TaskRun                 `yaml:"task_runs,omitempty"`
    ActionHistory     []ActionEvent             `yaml:"action_history"`
    CreatedAt         time.Time                 `yaml:"created_at"`
    UpdatedAt         time.Time                 `yaml:"updated_at"`
}

type IntakeAssessment struct {
    IntentType       string   `yaml:"intent_type"` // executable_change|advisory|question|mixed|unclear
    IsExecutable     bool     `yaml:"is_executable"`
    Confidence       float64  `yaml:"confidence"`
    ChangeTargets    []string `yaml:"change_targets,omitempty"`
    IntendedDelta    string   `yaml:"intended_delta,omitempty"`
    AcceptanceAnchor string   `yaml:"acceptance_anchor,omitempty"`
    BlockingUnknowns []string `yaml:"blocking_unknowns,omitempty"`
    AuxiliarySignals []string `yaml:"auxiliary_signals,omitempty"`
}

type RouteSnapshot struct {
    Classification   string         `yaml:"classification"` // executable|non_speclane
    Scores           ScoreBreakdown `yaml:"scores"`
    GuardrailDomain  string         `yaml:"guardrail_domain,omitempty"`
    RoutingRationale []string       `yaml:"routing_rationale,omitempty"`
    BlockingConflicts []string      `yaml:"blocking_conflicts,omitempty"`
}

type ScoreBreakdown struct {
    Novelty           int `yaml:"novelty"`
    Ambiguity         int `yaml:"ambiguity"`
    Impact            int `yaml:"impact"`
    Risk              int `yaml:"risk"`
    ReversibilityCost int `yaml:"reversibility_cost"`
}

type LevelHistoryEvent struct {
    From   Level     `yaml:"from"`
    To     Level     `yaml:"to"`
    Reason string    `yaml:"reason,omitempty"`
    At     time.Time `yaml:"at"`
}

type TaskRun struct {
    TaskID         string    `yaml:"task_id"`
    Verdict        string    `yaml:"verdict"` // pass|fail|blocked|timeout|incomplete|cancelled
    ChangedFiles   []string  `yaml:"changed_files,omitempty"`
    TestSummary    string    `yaml:"test_summary,omitempty"`
    VerifyCmd      string    `yaml:"verify_cmd,omitempty"`
    VerifyExitCode int       `yaml:"verify_exit_code,omitempty"`
    CommitRef      string    `yaml:"commit_ref,omitempty"`
    SummaryVersion int       `yaml:"summary_version,omitempty"`
    RanAt          time.Time `yaml:"ran_at"`
}

type ActionEvent struct {
    Action string    `yaml:"action"`
    State  string    `yaml:"state"`
    Detail string    `yaml:"detail,omitempty"`
    At     time.Time `yaml:"at"`
}

type ArtifactState struct {
    State     string    `yaml:"state"` // draft|in_review|approved|frozen
    Stale     bool      `yaml:"stale,omitempty"`
    UpdatedAt time.Time `yaml:"updated_at"`
}

type GateStatus struct {
    Status    string    `yaml:"status"` // pending|approved|blocked
    Reason    string    `yaml:"reason,omitempty"`
    UpdatedAt time.Time `yaml:"updated_at"`
}
```

### Flat Run Record (`.speclane/runs/<request_id>.yaml`)

```go
type RunRecord struct {
    RequestID          string              `yaml:"request_id"`
    Checks             []CommandCheck      `yaml:"checks,omitempty"`
    HumanConfirmations []HumanConfirmation `yaml:"human_confirmations,omitempty"`
    WaveSummaries      []WaveSummary       `yaml:"wave_summaries,omitempty"`
    LatestSummaryVersion int               `yaml:"latest_summary_version"`
    History            []RunEvent          `yaml:"history,omitempty"`
    UpdatedAt          time.Time           `yaml:"updated_at"`
}

type CommandCheck struct {
    CheckID      string    `yaml:"check_id"`
    Command      string    `yaml:"command"`
    ExitCode     int       `yaml:"exit_code"`
    Pass         bool      `yaml:"pass"`
    Override     bool      `yaml:"override,omitempty"`
    OverrideNote string `yaml:"override_note,omitempty"`
    OverrideAt *time.Time `yaml:"override_at,omitempty"`
    Detail       string    `yaml:"detail,omitempty"`
    RanAt        time.Time `yaml:"ran_at"`
}

type HumanConfirmation struct {
    CheckID     string    `yaml:"check_id"`
    Prompt      string    `yaml:"prompt"`
    Answer      string    `yaml:"answer"` // y|n
    Note        string    `yaml:"note,omitempty"`
    ConfirmedAt time.Time `yaml:"confirmed_at"`
}

type WaveSummary struct {
    SummaryVersion int       `yaml:"summary_version"`
    CompletedTasks []string  `yaml:"completed_tasks,omitempty"`
    NonPassTasks   []string  `yaml:"non_pass_tasks,omitempty"`
    CarriedDebt    []string  `yaml:"carried_debt,omitempty"`
    OpenBlockers   []string  `yaml:"open_blockers,omitempty"`
    FrozenAt       time.Time `yaml:"frozen_at"`
}

type RunEvent struct {
    Event  string    `yaml:"event"`
    Detail string    `yaml:"detail,omitempty"`
    At     time.Time `yaml:"at"`
}
```

## Command Surface Alignment

Daily:
- `speclane init`
- `speclane new`
- `speclane do`
- `speclane status`
- `speclane context`
- `speclane done`
- `speclane cancel`

Situational:
- `speclane pivot`
- `speclane repair`

Expert override:
- `speclane analyze`
- `speclane review`

## Risks / Trade-offs

### Risk: Less formal audit trail than enterprise governance systems

Mitigation: keep command outputs + human confirmations in run YAML and keep request-scoped archive.

### Risk: Human confirmation quality varies

Mitigation: make prompts explicit and keep deterministic command checks as mandatory machine baseline.

### Risk: Simpler model may miss niche compliance workflows

Mitigation: defer advanced policy layers to post-MVP only if real usage proves need.
