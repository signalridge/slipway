# Workflow Test Menu

This document gives you an executable menu for testing Slipway end to end from
inside the Slipway repository itself.

The goal is not just to list commands. The goal is to let you pick a path,
perform a real exercise, and observe the expected workflow behavior at each
step.

## Recommended Development Sample

Use this concrete exercise when you want one realistic, low-risk change that
still touches the actual product surface:

**Sample change:** tighten `validate-requirements` command description
consistency across CLI, root help, generated command metadata, and docs.

Why this sample works well:

1. It is based on the current project structure and command surface.
2. It exercises real governed work: command help, generated metadata, tests,
   and docs.
3. It is low runtime risk compared with changing lifecycle state logic.
4. It gives you a clean success signal: all public
   `validate-requirements` descriptions should describe the same read-only
   behavior.

Likely files involved:

- `cmd/sync.go` (current implementation home of `validate-requirements`)
- `internal/toolgen/toolgen.go`
- `cmd/root_help_test.go`
- `README.md`

Primary acceptance signals:

1. `go run . --help` and `go run . validate-requirements --help` both describe
   `validate-requirements` as read-only validation of `requirements.md`.
2. Generated command metadata stays aligned with the CLI help surface.
3. Targeted tests and the normal verification bundle pass.

## Preconditions

Run these menus from the repository root.

Recommended setup:

1. Use a dedicated branch or worktree.
2. Keep a note of pre-existing uncommitted changes before starting.
3. Have `go` and `staticcheck` available in your shell.

Unless a menu says otherwise, commands below assume:

```bash
cd /path/to/slipway
```

## Menu 0: Workspace Bootstrap And Diagnostics

Use this when you want to confirm the repo can host a Slipway workspace before
starting governed work.

Commands:

```bash
go run . init --tools none
go run . status --json
go run . health --json
go run . stats --json
go run . codebase-map
```

What to observe:

1. `init` should create `.slipway.yaml` and print a workspace initialization
   message.
2. `status --json` should return diagnostics mode when there is no active
   change.
3. `health --json` should show repo-local findings before `codebase-map`, then
   become cleaner after the map is generated.
4. `stats --json` should report codebase-map freshness.

Good companion test:

```bash
go test ./cmd -run TestCLIEndToEndDiagnosticsAndCodebaseMapFlow -count=1
```

## Menu 1: Lifecycle Smoke Test Without Shipping A Change

Use this when you want to exercise `new`, `status`, `validate`, `next`,
`validate-requirements`, and `cancel` with minimal risk.

Suggested description:

```text
workflow smoke: tighten validate-requirements description consistency
```

Commands:

```bash
go run . new --json --preset standard "workflow smoke: tighten validate-requirements description consistency"
go run . status --format yaml
go run . validate --json
go run . next --preview
go run . validate-requirements --json
go run . cancel --json
go run . status --json
```

What to observe:

1. `new --json` should return a governed change starting at `S0_INTAKE`.
2. `status` should show the active governed change instead of diagnostics mode.
3. `validate --json` should be read-only and keep the change in the same state.
4. `next --preview` should show the next skill context without advancing state.
5. `validate-requirements --json` should validate the current change's
   `requirements.md` and stay read-only.
6. `cancel --json` should archive the change as terminal.
7. Final `status --json` should return to diagnostics mode.

Good companion tests:

```bash
go test ./cmd -run 'TestCLIEndToEndGovernedLifecycleBlockersAndCancel|TestCLIEndToEndNewRepairAndCancelFlow' -count=1
```

## Menu 2: Full Manual Workflow With A Real Project-Specific Change

Use this when you want to drive a realistic change all the way through
governed execution.

Suggested change:

```text
align validate-requirements command description with current read-only behavior
```

### Step 1: Create The Governed Change

```bash
go run . new --json --preset standard "align validate-requirements command description with current read-only behavior"
go run . status --format yaml
go run . next --preview
```

At this point, use the surfaced skill/context from `next --preview` as the
source of truth for what artifact work is expected first.

### Step 2: Fill Intake And Planning Artifacts

Work the change as Slipway asks you to. For this sample, the intended content is:

1. The problem is public-surface description drift around
   `validate-requirements`.
2. The scope is description alignment plus regression guards.
3. The acceptance target is consistent wording across CLI help, generated
   command metadata, tests, and docs.

Useful checkpoints while doing artifact work:

```bash
go run . status --format yaml
go run . validate --json
go run . next --preview
```

### Step 3: Implement The Sample Change

Once the workflow reaches execution, make the actual code and doc changes.

Suggested implementation checklist:

1. Align the `validate-requirements` description in
   `internal/toolgen/toolgen.go` with the real read-only behavior implemented
   by `cmd/sync.go`, which now backs the retired `sync` verb's replacement
   surface.
2. Keep root help and generated command surfaces consistent with that wording.
3. Update docs if the public command wording changes.
4. Add or adjust tests so this drift is caught again.

Useful commands while implementing:

```bash
rg -n '"validate-requirements"|requirements.md|read-only' cmd internal README.md
go test ./cmd -run 'TestRootHelpUsesCurrentEntrySurfaceDescriptions|TestCLIEndToEndSuccessfulValidateRequirementsChecksRequirements' -count=1
go run . status --format yaml
go run . next --preview
```

### Step 4: Review, Validate, And Close Out

When the change is ready, exercise the governed closeout surfaces:

```bash
go run . review --json
go run . validate --json
go test ./... -count=1
go vet ./...
staticcheck ./...
go test ./... -race -count=1
go run . done --json
```

What success looks like:

1. `review --json` reports a passing or otherwise clean review outcome for the
   active state.
2. `validate --json` reports no blocking readiness problems.
3. The verification bundle is green.
4. `done --json` archives the change successfully.

If you do not want to fully satisfy the closeout gates during rehearsal, stop
after review/validate and use:

```bash
go run . cancel --json
```

## Menu 3: Advanced Branch Coverage Through Existing Tests

Some workflow branches are real but awkward to trigger manually every time.
Use the existing end-to-end and focused tests to cover those branches quickly.

### Checkpoint Branch

```bash
go test ./cmd -run 'TestCLIEndToEndSuccessfulCheckpointAtS5|TestRunRequiresResumeResponseForActiveCheckpoint|TestRunResumesCheckpointWithValidResponse' -count=1
```

### Review-Pass Branch

```bash
go test ./cmd -run 'TestCLIEndToEndSuccessfulReviewPassAtS7|TestReviewRequiresStoredWaveRunsForExecutionSummary' -count=1
```

### Done-Archive Branch

```bash
go test ./cmd -run 'TestCLIEndToEndSuccessfulDoneArchive|TestDoneGovernedValidAssuranceSucceeds' -count=1
```

### Validate-Requirements Branch

```bash
go test ./cmd -run 'TestCLIEndToEndSuccessfulValidateRequirementsChecksRequirements|TestCLIEndToEndValidateRequirementsAfterRequestNext|TestValidateRequirementsCommandValidatesRequirements' -count=1
```

### Governance Consistency Branch

```bash
go test ./cmd -run 'TestReviewLayerBlockersStayConsistentAcrossStatusValidateNextAndReview|TestDoneShipGateReasonsStayConsistentWithSharedReadiness' -count=1
```

### Recovery And Doctor Branch

```bash
go test ./cmd ./internal/state -run 'TestHealthCommandDoctorOutputsPrioritizedRepairActions|TestRepairMaterializesWavePlanRecoversWaveRunsAndClearsStaleCheckpoint' -count=1
```

## Recommended Order If You Only Run One Pass

If you want one practical sequence instead of picking freely, use this order:

1. Run Menu 0 once.
2. Run Menu 1 once to confirm the basic lifecycle loop.
3. Run Menu 2 with the `validate-requirements` description consistency sample.
4. Run the checkpoint and done-archive subsets from Menu 3.

## Notes For Interpreting Results

1. `status`, `validate`, and `next --preview` are the safest inspection tools
   because they are read-only.
2. `review` and `done` are meaningful only when the workflow has reached the
   appropriate governed state.
3. If you only want to test command behavior and state transitions, use Menu 1
   and Menu 3.
4. If you want to test the intended day-to-day authoring loop, use Menu 2.
