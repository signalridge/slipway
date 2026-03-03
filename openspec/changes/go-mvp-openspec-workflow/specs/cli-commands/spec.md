## ADDED Requirements

### Requirement: Command Surface
CLI SHALL expose 11 commands:
- `init`, `new`, `do`, `status`, `context`, `done`, `cancel`, `pivot`, `repair`, `analyze`, `review`

#### Scenario: Command list
- **WHEN** `speclane --help` is executed
- **THEN** all 11 commands SHALL be listed

### Requirement: Failure Envelope
Non-zero command failures SHALL emit parse-stable JSON on stderr.

Exit codes:
- `2` invalid usage
- `3` precondition blocked
- `4` state integrity failure
- `5` governance blocked
- `6` runtime failure

Envelope fields:
- `error_code`, `category`, `message`, `remediation`, `exit_code`
- optional `request_id`, `details`

#### Scenario: Invalid flag
- **WHEN** unsupported flag is supplied
- **THEN** command SHALL return exit code `2` with remediation

### Requirement: `speclane init`
`init` SHALL create minimal runtime layout:
- `.speclane/config.yaml`
- `.speclane/runtime/admissions/`
- `.speclane/runtime/changes/`
- `.speclane/runs/`
- `.speclane/archive/admissions/`
- `.speclane/archive/changes/`
- `.speclane/archive/runs/`
- `.speclane/archive/config/`
- `aircraft/changes/`
- `aircraft/changes/archived/`

`init` SHALL NOT create narrative files under `.speclane/`.

`init` adapter flags:
- `--tools <list|all|none>` SHALL control adapter generation
- `--refresh` SHALL force deterministic regeneration of selected adapter files
- adapter generation SHALL remain optional sidecar and SHALL NOT affect core runtime layout success

#### Scenario: Fresh init
- **WHEN** repo is not initialized
- **THEN** `speclane init` SHALL create required runtime paths

#### Scenario: Init with tools none
- **WHEN** `speclane init --tools none` is used
- **THEN** runtime layout SHALL still be created and core workflow SHALL remain functional

### Requirement: `speclane new`
`new` SHALL:
1. run `S0/S1`
2. classify `executable|non_speclane`
3. resolve level (`auto|L1|L2|L3`)
4. reject fixed-level hard conflicts before persistence
5. persist landing state (`L1->S6`, `L2->S4`, `L3->S2`)
6. create governed artifacts only for L2/L3

#### Scenario: Advisory request classified as non_speclane
- **WHEN** intake is non-executable/advisory
- **THEN** `new` SHALL classify as `non_speclane` without creating request runtime files

### Requirement: `non_speclane` Classification Contract
`speclane new` classification of `non_speclane` SHALL be a successful classification outcome, not a runtime error.

Contract:
- exit code SHALL be `0`
- command SHALL emit parse-stable stdout payload with:
  - `classification=non_speclane`
  - `accepted=false`
  - deterministic remediation text
- no request-scoped runtime files SHALL be created

#### Scenario: Successful non-scope classification
- **WHEN** intake is classified as `non_speclane`
- **THEN** command SHALL return exit `0` with classification payload and no runtime writes

### Requirement: Active Context Resolution
Request-scoped commands SHALL require exactly one active request.

Request-scoped commands (`do/done/cancel/pivot/analyze/review`) require exactly one active request.

`status/context` SHALL support diagnostics mode when active set is `0` or `>1`.

#### Scenario: Ambiguous active request
- **WHEN** multiple active requests exist
- **THEN** request-scoped commands SHALL fail with deterministic remediation

### Requirement: `speclane do`
`speclane do` SHALL execute exactly one next action step per invocation.

`do` executes one next action from current state and appends action history.

At governed `S7_REVIEW`, `do` and `review` SHALL use equivalent review execution semantics.

For L1, `do` SHALL NOT auto-evaluate `S7/S8` in the same invocation as `S6`.
L1 progression SHALL remain explicit step-by-step (`S6` then `S7` then `S8`) across separate `do` calls.

#### Scenario: L1 lightweight pass
- **WHEN** L1 finishes `S6` successfully
- **THEN** next `do` invocation SHALL enter `S7` instead of skipping directly to done-ready

### Requirement: `speclane status`
`status` default output SHALL be JSON and include:
- lane mode
- request context (when unique)
- state/lifecycle
- blockers
- next actions
- recent command-check summary from run record
- recent human-confirmation summary from run record

#### Scenario: Diagnostics status
- **WHEN** active set is `0` or `>1`
- **THEN** status SHALL return diagnostics payload

### Requirement: `speclane context`
`context` SHALL support `--format text|yaml|json` and remain compact.

It SHALL include:
- lane + current action
- blockers + next action
- gate-check and confirmation summary

#### Scenario: Diagnostics context
- **WHEN** unique active request is unavailable
- **THEN** context SHALL return diagnostics mode output

### Requirement: `speclane done`
`speclane done` SHALL finalize only when lane-specific completion preconditions are met.

`done` is strict finalizer.

L1:
- requires done-ready verify state
- archives admission and run record by request

L2/L3:
- requires `S8` completion
- requires required checks passed OR explicit operator override for failed checks
- requires human confirmations (`review_done=y`, `ship_ready=y`)
- archives governed runtime/artifacts + linked sealed admission + run record

#### Scenario: Governed done with override
- **WHEN** a required command check failed but operator explicitly overrides
- **THEN** done MAY proceed and override SHALL be persisted in run record

### Requirement: `speclane cancel`
`cancel` SHALL:
- set terminal cancelled lifecycle
- interrupt in-flight task processes (`SIGINT` then `SIGKILL` after grace)
- archive by request-scoped rule (including run record archive)

#### Scenario: In-flight cancel
- **WHEN** tasks are running during cancel
- **THEN** runtime SHALL interrupt before archive migration

### Requirement: `speclane pivot`
`pivot` SHALL be analyze-first and allowed from `S6/S7/S8` only.

`--kind` supports `reroute|rescope` (default `reroute`).
`rescope` is valid only from governed `S6`.

#### Scenario: Invalid rescope state
- **WHEN** `pivot --kind rescope` is invoked from `S7` or `S8`
- **THEN** command SHALL fail with remediation

### Requirement: Override Commands
Override commands SHALL follow strict state preconditions and SHALL NOT bypass core lifecycle contracts.

- `analyze`: rerun analyze in `S1` context without implicit reroute
- `review`: explicit review execution path

`review` is allowed from `S7`, or from `S6/S8` with transition preconditions.

Review transition preconditions:
- from `S7`: always allowed
- from `S6`: allowed only when latest frozen summary exists (`latest_summary_version >= 1`) and no active wave subprocess is running
- from `S8`: allowed only when request is non-terminal and latest frozen summary exists
- otherwise `review` SHALL fail as precondition-blocked (exit code `3`)

#### Scenario: Analyze without active request
- **WHEN** no active request exists
- **THEN** `analyze` SHALL fail with remediation

#### Scenario: Review from S6 without frozen summary
- **WHEN** current state is `S6` and `latest_summary_version=0`
- **THEN** `review` SHALL fail as precondition-blocked with remediation to run/finish at least one wave summary
