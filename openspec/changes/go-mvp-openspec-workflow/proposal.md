## Why

Current draft over-emphasizes governance evidence mechanics for an AI-agent CLI workflow:
- complex `evidence/skills` schema introduces naming confusion with real tool skills
- gate progression depends on AI-generated evidence records instead of reproducible command results
- operational complexity is too high for solo/small-team usage patterns

The agreed direction is to keep SpecLane pragmatic:
- keep routing and execution skeleton (`L1/L2/L3`, `S0..S8`)
- keep minimal gates (`G_scope/G_plan/G_pivot/G_ship`)
- gate on command-check facts plus explicit human confirmation
- store one flat run record per request (`.speclane/runs/<request_id>.yaml`)

## What Changes

- Keep two-phase model:
  - admission (`S0 -> S1`)
  - execution by level path (`L1`: `S6..S8`, `L2`: `S4..S8`, `L3`: `S2..S8`)
- Keep `non_speclane` boundary for pure Q&A/advisory
- Keep admission/governed runtime state split:
  - `.speclane/runtime/admissions/<request_id>.yaml`
  - `.speclane/runtime/changes/<request_id>.yaml`
- Keep governed artifact bundle under `aircraft/changes/<slug>/`

Simplify governance model:
- remove governance-skill evidence contract and `evidence/skills` path
- replace with gate checks contract: command checks + human confirmations
- gate checks are run-time facts (exit code, grep count, artifact presence), not AI self-attestation
- remove reviewer `session_id` independence comparator from readiness blocking rules
- use `check` terminology only (`command_check`, `human_confirmation`) to avoid collision with tool skills

Flat run record:
- add `.speclane/runs/<request_id>.yaml` as canonical execution/checkpoint/check-result record
- store machine check results and human confirmations in this file
- archive run record on terminal transitions to `.speclane/archive/runs/<request_id>.yaml`
- no `check registry`/`policy snapshot` layers in MVP

Gate behavior (minimal):
- `G_scope`: L3 discovery/scope/worktree readiness checks + human scope confirmation
- `G_plan`: governed planning artifact readiness + `openspec validate` pass + human execute confirmation
- `G_pivot`: explicit pivot rule gate (entry-state + analyze-first + pivot-kind validity)
- `G_ship`: command checks pass + human review confirmation + human ship confirmation

Command checks baseline:
- `go test ./...` (when code delta exists)
- `golangci-lint run` (when code delta exists)
- `grep -n "^- \[ \]" tasks.md` (no unchecked governed tasks before ship)
- `openspec validate <change>` for governed plan readiness

Human confirmations baseline:
- `execute_ready` (`Is execution ready? [y/n]`)
- `review_done` (`Is review complete? [y/n]`)
- `ship_ready` (`Ready to ship? [y/n]`)
- L3 additional `scope_confirmed` (`Is scope confirmed? [y/n]`)

Prompt wording policy:
- canonical prompts in specs are English
- AI runtime renders localized prompts based on user language when needed

Failed required command checks are blocked by default, but MAY continue only via explicit user override with run-record trace.

Other retained constraints:
- fixed-level hard safety conflicts still fail before persistence
- request-scoped archive remains default for `done` and `cancel`
- `speclane status/context` remain diagnostics-first when active context is missing/ambiguous
- `non_speclane` remains successful classification outcome (no runtime file writes)

## MVP Scope (Plan Artifact Level)

- No runtime DB
- No evidence/skills schema
- No reviewer session-id independence gating
- No policy registry / policy snapshot layering
- Keep route + execution skeleton and minimal gates
- Keep command checks and explicit human decision points

## Source Strategy (Explicit)

- **OpenSpec (reference base):** artifact lifecycle, status/validate/archive semantics, gate framing
- **GSD (reference execution):** wave planning/execution, conflict split, retry/skip/abort/pivot loop, fact-based spot checks
- **Superpowers (reference interaction):** clarification discipline and explicit human decision points
- **SpecLane (local authority):** level routing and state-machine/runtime contracts

## Capabilities

### New Capabilities

- `action-workflow`: admission + execution split with canonical state taxonomy
- `state-persistence`: admission/change state + flat run record persistence
- `gate-checks`: per-level gate command checks and human confirmations
- `cli-commands`: route-first `new`, minimal gate interactions, diagnostics/read-only status/context

### Modified Capabilities

- `routing-engine`: keeps internal scoring and single-level output, no manual `--scores`
- `gate-engine`: evaluates command-check outcomes + human confirmations (not governance-skill evidence)
- `wave-execution`: keeps DAG/wave model; writes run outcomes into `.speclane/runs/<request_id>.yaml`
- `review-engine`: removes mandatory layer taxonomy and enforces explicit review entry preconditions
- `context-engine`: consumes compact run-ledger summaries without requiring duplicated state mirrors
- `artifact-lifecycle`: defines assurance ownership/timing across `S7/S8` and closeout freshness
- `tool-adapters`: keeps optional sidecar behavior with canonical `speclane` runtime command routing

## Impact

- Reduced governance ceremony and naming ambiguity
- Improved trust model: gating decisions are based on reproducible command facts + explicit user decisions
- Lower implementation complexity while preserving risk-based routing and state-machine discipline
