# Decision

## Project Context
- Tech Stack: Go CLI
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### DEC-001: Add structured diagnostics, freshness guards, and contract documentation while keeping existing lifecycle semantics
This approach exposes already-computed traceability gaps on the failing health check, adds explicit next-action/resume-support metadata to confirmation requirements, rejects stale `wave-orchestration` evidence when newer runtime task evidence exists for the same run version, documents the `validate`/`run`/`health` authority boundary, and replaces the GNU-only placeholder scan with a portable command. Covers REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, and REQ-007.

> **Note (descope):** an earlier revision of this approach also added an mtime-based
> `plan-audit` source-freshness guard. It was removed before ship because Slipway
> rewrites `tasks.md` during execution (checkbox writeback), so an mtime/timestamp
> comparison false-positives on a normal `[ ]`→`[x]` tick. Content-addressed
> plan-audit freshness is tracked separately in #66. REQ-006 in this change is
> therefore satisfied by the `wave-orchestration` half only.

Tradeoffs:
- Low blast radius and backward-compatible additive JSON fields.
- Keeps current snapshot recomputation behavior instead of redesigning the cache.
- Keeps blocker severity semantics intact: blockers still describe the current stop condition, while docs explain how to read them after a successful `run` transition.

### DEC-002: Replace governance snapshot caching with always-live health evaluation and change skill handoff boundaries
This approach would make health always recompute from scratch and rename/reclassify skill handoff boundaries. Covers REQ-001 and REQ-002.

Tradeoffs:
- Stronger conceptual cleanup, but higher risk to performance, existing JSON consumers, and unrelated governance behavior.
- Larger change than required because current tests already prove material-change recomputation.

### DEC-003: Create active checkpoints for every skill handoff
This approach would convert skill handoffs into resumable checkpoint state accepted by `--resume-response`. Covers REQ-002.

Tradeoffs:
- Resolves the reporter's checkpoint confusion literally.
- Conflates workflow skill handoffs with execution checkpoints and expands state-machine semantics well beyond the issue scope.

## Selected Approach
Select DEC-001. It is the narrowest approach that satisfies all requirements: expose existing traceability gap data in health output, add explicit action metadata to confirmation output without changing the checkpoint contract, fail closed on stale wave-orchestration evidence (runtime task evidence freshness), document the command authority boundary, and make the goal-verification scan portable. This direction preserves existing JSON fields while adding tested optional metadata for external API consumers. (Plan-audit source-freshness is deliberately excluded; see the descope note above and #66.)

## Interfaces and Data Flow
- `internal/engine/governance.GovernanceHealthCheck` gains optional structured details for traceability failures. `cmd/health.go` JSON encodes those details automatically, and text output may print them for human health output.
- `cmd.confirmationRequirement` gains optional action metadata, such as whether `--resume-response` is supported and the next command/action a caller should take.
- `deriveConfirmationRequirement` sets non-checkpoint action metadata for skill handoffs and checkpoint action metadata only when `input_context.resume_checkpoint` is actually pending.
- `progression.SyncGovernedWaveExecution` rejects a passing `wave-orchestration` record whose timestamp predates current runtime task evidence (`captured_at`) for the same run summary version.
- Plan-audit source-freshness is intentionally not implemented here. `EvaluateRequiredSkillsForChange` and `EvaluatePlanGate` retain their prior semantics (missing/not-passing plan-audit evidence only); content-addressed plan-audit freshness is deferred to #66.
- `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl` uses a portable empty-block scan command.
- `docs/commands.md` and generated command prompts document the distinction between `validate` as active-readiness authority, `run` as mutating transition surface, and `health --governance` as diagnostic feedback.

## Rollout and Rollback
- Rollout is a normal code change with regression tests in `cmd/health_test.go`, `cmd/governance_gate_consistency_test.go`, `cmd/progression_next_test.go`, `internal/engine/progression/evidence_test.go`, and `internal/tmpl/templates_test.go`.
- Rollback is a standard git revert. No runtime data migration is introduced.
- Verification: focused tests for the changed command/template surfaces, then `go test ./...`, `go build ./...`, and Slipway validation.

## Risk
- Medium external API risk: JSON surfaces gain new optional fields. Mitigation: keep existing fields stable and add tests for the added metadata.
- Medium governance risk if stale runtime evidence is accepted. Mitigation: compare wave-orchestration timestamps with current runtime task evidence (`captured_at`) during wave sync. (An mtime-based plan-audit guard was considered and rejected — it false-positives on Slipway's own `tasks.md` checkbox writeback; the content-digest replacement is tracked in #66.)
- Medium operator-risk if details are too terse. Mitigation: reuse `TraceabilityGap` fields so gap ID, type, issue, and blocking status are visible.
- Low portability risk: Perl one-liner replaces GNU `grep -P`; the target macOS environment supports Perl and the issue reporter used Perl successfully.
