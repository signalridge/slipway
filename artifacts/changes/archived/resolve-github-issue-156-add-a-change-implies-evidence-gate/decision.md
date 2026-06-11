# Decision

## Alternatives Considered

- Extend scope-contract with sensitive categories.
  This would reuse the changed-file path, but it would mix two contracts:
  planned scope coverage and operational owning evidence.
- Require S3 review skills to attest every sensitive file.
  This keeps review central, but it would strand S2 because review evidence is
  not available until after execution.
- Add a dedicated readiness evaluator fed by execution-summary task evidence.
  This preserves the current scope/freshness contracts while adding the exact
  issue #156 proof at the point where changed-file evidence exists.
- Restore a public skill-evidence recording command.
  This is not the sensitive-evidence classifier itself, but the re-audit exposed
  that generated host instructions require `slipway evidence skill`; without it,
  plan-audit evidence can only be hand-edited, which violates the workflow
  authority boundary.

## Selected Approach

Implement `internal/engine/sensitiveevidence` and call it from
`EvaluateGovernanceReadiness` after execution-summary readiness and
scope-contract evaluation, plus from `AdvanceGoverned` before a ready execution
summary can move past the S2-owned evidence boundary. The evaluator classifies
sensitive changed files, extracts category markers from passed task evidence
references, and emits `sensitive_evidence_missing` blockers for missing proof.

This is selected because it uses current lifecycle authority, avoids review
deadlock, and keeps sensitive-domain behavior fail-closed.

## Interfaces and Data Flow

- Input: `model.ExecutionSummary`, task `ChangedFiles`, task `EvidenceRef`, and
  workspace changed files already used by scope-contract readiness.
- Output: `sensitiveevidence.Report` plus canonical reason codes appended to
  `GovernanceReadiness.Blockers` and used by mutating advancement to block in
  S2_EXECUTE or reopen later stages to S2_EXECUTE.
- Public evidence interface: `slipway evidence task --evidence-ref` records
  markers such as `migration-applied:<command>`, `auth-review:<review-ref>`, and
  `contract-test:<test-command>`.
- Public skill-verification interface: `slipway evidence skill` validates the
  requested governance skill against the current lifecycle state and current
  actionable predecessor, records a `model.VerificationRecord` through
  `state.SaveVerification`, stamps the engine-owned skill freshness digest for
  passing evidence, prunes any previous digest when a non-passing record
  overwrites a passing record, updates the change evidence reference, and leaves
  lifecycle advancement to `slipway run`.
- Recovery interface: `model.BuildRecovery` maps
  `sensitive_evidence_missing` to `slipway run`, so the workflow first stays in
  or reopens S2_EXECUTE before the operator records replacement task evidence.

## Rollout and Rollback

Rollout is covered by targeted unit tests, progression readiness tests, reason
code contract tests, recovery tests, `evidence skill` command tests, state
verification persistence tests, and `go test ./...`.

Rollback is a normal git revert of the evaluator, readiness wiring, and
reason/recovery entries. Verification after rollback is `go test ./...` plus
`go run . validate --json` on the active change.

## Risk

- False positives are possible if path patterns overmatch. The initial matcher
  is intentionally narrow and table-tested.
- False negatives are possible for project-specific sensitive conventions. A
  future configurable rule set can add repository-specific patterns after the
  built-in gate is stable.
- Sensitive-domain work must not expose a bypass. Tests assert the recovery text
  does not mention bypass or skip mechanisms, and the evaluator does not read
  environment variables.
- Generated host instructions can drift from registered CLI commands. The
  restored `evidence skill` surface is tested through Cobra command execution so
  future command registration regressions fail before operators are stranded.
- Public skill-evidence recording must not become a freshness bypass. The CLI
  now rejects downstream review/closeout skill records before their predecessor
  passes, writes a skill digest alongside passing verification evidence, and
  removes a skill digest when a later non-passing record invalidates that pass.
