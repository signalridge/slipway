# Decision
## Project Context
- Tech Stack: Go CLI
- Conventions: structured JSON command surfaces, repo-native tests, governed Slipway artifacts
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Alternative A: Minimal diagnostics only
Add prose explanations and a few extra JSON fields around the existing `evidence_input_hash` and next-skill behavior.

Tradeoffs:
- Lowest implementation cost.
- Keeps old execution-summary shape.
- Does not remove the opaque hash contract that caused the incident.
- Leaves multiple command paths at risk of diverging again.

### Alternative B: Centralized diagnostic model with explicit freshness contract (selected)
Replace task freshness hash authority with an explicit structural input object, compare the stored object field-by-field, and route command diagnostics through shared helpers for actionable next skill, review tokens, paths, repair findings, and blocking semantics.

Tradeoffs:
- Larger change and a JSON contract break for execution-summary task freshness.
- Gives operators direct expected/current values without reading source.
- Makes stale hash-only evidence fail closed with a clear regeneration path.
- Reduces duplicate next-skill and diagnostics logic across commands.

### Alternative C: Repair-first compatibility path
Keep reading legacy `evidence_input_hash`, add a repair command to backfill structural fields, and accept either representation during a migration window.

Tradeoffs:
- Softer migration for existing local runtime evidence.
- Adds compatibility branches and ambiguity about which contract is authoritative.
- Conflicts with the user-selected direction to make a thorough change with no compatibility layer.

## Selected Approach
Use Alternative B.

The selected implementation will:
- Remove `evidence_input_hash` as the task freshness authority.
- Store explicit structural freshness inputs on each execution task summary.
- Treat hash-only task evidence as stale or unsupported for the new contract, with remediation to regenerate evidence.
- Centralize actionable next-skill resolution so `next`, `validate`, and `run --diagnostics` report the same blocking skill.
- Expose exact review-layer tokens from the same gate logic that validates them.
- Add structured diagnostics for lifecycle resume boundaries, path authority, repair boundaries, artifact DAG blocking state, health active-change impact, and confirmation boundaries.

## Interfaces and Data Flow

### Execution freshness
- Producer: execution-summary generation in `internal/state/execution_summary.go`.
- Model: `internal/model/execution_summary.go` stores a structural task freshness object rather than a hash-only field.
- Consumer: freshness diagnostics compare stored fields to current expected fields and emit field-level expected/current values.
- No compatibility layer: old summaries containing only `evidence_input_hash` are not silently accepted.

### Next-skill and review diagnostics
- Producer: progression/readiness logic determines the current blocking skill and required review layers.
- Consumers: `cmd/next.go`, `cmd/validate.go`, and `cmd/run.go` use a shared view builder for actionable next skill and rationale.
- Templates: generated spec-compliance and code-quality skill prompts show gate tokens such as `layer:R0=pass`, `layer:R3=pass`, `layer:IR1=pass`, and `layer:IR3=pass` when required.

### Path authority
- Producer: state/path helpers resolve invocation workspace, governed bundle path, verification path, and git-common runtime evidence path.
- Consumers: stale diagnostics, repair output, status, and run diagnostics expose these paths where relevant.

### Repair/status/health diagnostics
- `repair --json` reports `applied_repairs` and `unrepaired_drift` separately.
- `status --json` artifact DAG entries include current blocking semantics.
- `health --json` findings include active-change blocking impact and a concrete next action.

## Rollout and Rollback
Rollout is a code and artifact change in this worktree only.

Verification path:
- targeted package and command tests for each changed JSON contract;
- `go test ./...`;
- `go build ./...`;
- `go vet ./...` if the repo remains compatible;
- `git diff --check`;
- governed validation/review evidence before closeout.

Rollback path:
- revert this branch's code and template/doc changes;
- regenerate any local execution summaries created under the new structural contract if returning to an older binary;
- use `slipway repair --json` or remove local runtime evidence only when the operator explicitly chooses to rebuild that evidence from source artifacts.

## Risk
- JSON contract churn: replacing `evidence_input_hash` can break scripts that consumed the old field. This is accepted because the user selected a no-compatibility thorough fix.
- Runtime evidence churn: old local summaries may be reported stale until regenerated.
- Shared view regressions: centralizing next-skill logic must preserve lifecycle safety semantics and cannot auto-skip required hard gates.
- Linked-worktree path reporting: path diagnostics must show authoritative paths without changing where evidence is stored.
- Guardrail review: because the public CLI JSON surface changes, external API contract review must verify field names and semantics.
