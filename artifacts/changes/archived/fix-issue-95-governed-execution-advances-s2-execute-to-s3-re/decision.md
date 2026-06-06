# Decision
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

This change has three independent design forks, one per concern.

**Fork 1 — #88 `<domain>.safety_baseline` producer/satisfy-path.**
- **Option A — new conditionally-required `guardrail-baseline` skill.** A new
  governance skill, required when `guardrail_domain != ""`, records the token.
  Tradeoffs: explicit ownership, but a whole new required skill + registry wiring
  + lifecycle plumbing for what the existing S4 verifier already covers.
- **Option B — goal-verification documents and records the token (selected).**
  goal-verification already runs at S4_VERIFY and its verification-record
  References already feed `ExtractHighRiskChecks` → the ship gate. The defect is
  purely missing guidance (the host never knew the exact token format) + a vague
  remediation. Tradeoffs: smallest change, no new required skill, fail-closed
  (token reflects a real goal-verification verdict); relies on the host reading
  the SKILL.md (mitigated by an actionable remediation + a `slipway next` token
  hint).

**Fork 2 — #88 `repair --focus sast`.**
- **Option A — make repair run semgrep/CodeQL.** Tradeoffs: violates repair's
  bounded local-state-integrity contract; repair must never execute external
  scanners.
- **Option B — emit a routing hint from repair but keep the focus.** Tradeoffs:
  keeps a focus that does nothing on repair; still a half-promise.
- **Option C — remove the false focus from repair, redirect to review/validate
  (selected).** Tradeoffs: honest and smallest; SAST guidance stays reachable on
  review/validate where it legitimately hydrates sast-orchestration; the rejection
  message redirects.

**Fork 3 — #95 incomplete-execution gate + worktree provisioning.**
- #95 blocker shape: **one `incomplete_execution_task:<taskID>` per missing task
  (selected)** vs a single aggregate blocker (less actionable). Recovery:
  **fail-closed complete-or-rescope (selected)** vs a new defer/skip bypass
  (rejected — weakens the gate).
- worktree provisioning: **(A) default-true unconditional with config opt-out
  (selected)** vs (B) preset/complexity-gated vs (C) opt-in default-false. A
  frees the main checkout for every governed change (the user's goal), is the
  most consistent, and `git worktree add` is cheap; `auto_provision_worktree`
  preserves opt-out. Confirmed with the user as "A".

## Selected Approach
- **#95:** add an execution-completeness assertion in
  `evaluateGovernedWaveExecution` that compares `WavePlan.TaskIDs()` to the
  evidenced runs and emits one `incomplete_execution_task:<taskID>` per missing
  task (both preview and mutate paths, suppressed under plan-drift); register the
  canonical reason + a refresh-execution recovery remediation; document the
  completeness contract in the wave-orchestration skill.
- **#88:** goal-verification SKILL.md documents recording
  `high_risk_check:<domain>.safety_baseline=pass|fail` from a real SAST run;
  upgrade the `high_risk_check_missing`/`high_risk_check_failed` remediations to
  name the token + producer + action and surface the required token in the
  `slipway next` goal-verification handoff; remove the `sast` focus from the
  `repair` surface and redirect to review/validate; clarify final-closeout's
  guardrail recheck. No bypass / self-attestation / stamp-it path.
- **worktree:** add `governance.auto_provision_worktree` (default true) and drop
  the `!NeedsDiscovery` skip in `EnsureDefaultWorktreeForChange` so every governed
  change binds `.worktrees/<slug>` on `feat/<slug>` at `slipway new` with the
  bundle inside it; keep the single-active-change guard isolation-correct; archive
  still strips the worktree path.

## Constraints
- Sensitive-domain gates stay fail-closed; no bypass/force-close/self-attestation.
- Slipway never runs external scanners; the host runs them and records evidence.
- The guardrail catalog (which domains require safety_baseline) is unchanged.
- Code, generated skills, docs, and agent instructions stay aligned (toolgen
  self-loop zero drift); `go build/vet/test` green.
- Preserve unrelated local work (`evidence-task-dx-issue.md`).
