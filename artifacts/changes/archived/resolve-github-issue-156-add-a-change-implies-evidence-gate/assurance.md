# Assurance

## Scope Summary

Issue #156 is delivered by adding a governed readiness gate where sensitive
changed files imply explicit owning task evidence. The gate covers schema
migrations, auth/authz/RBAC/permission files, and API contract files. It fails
closed with `sensitive_evidence_missing:<category>:<path>` until passed task
evidence records the matching marker: `migration-applied`, `auth-review`, or
`contract-test`.

The implementation is wired into the current governed readiness path without
replacing existing freshness, worktree, artifact, scope-contract, review, or ship
checks. Missing sensitive evidence also reopens recovery to S2 execution, where
operators can record task evidence through `slipway evidence task`.

The repair pass also fixed the review-discovered lifecycle bug in the new public
skill-evidence command: `slipway evidence skill --skill wave-orchestration` no
longer requires `execution-summary.yaml` before the wave evidence that produces
that summary can be recorded. Wave evidence derives its run version and digest
from runtime task evidence until the execution summary exists; later review and
closeout skills still require the ready execution summary.

## Verification Verdict

Pass for the active standard-preset change. The current verification set covers
the sensitive-evidence evaluator, governed readiness integration, S2 recovery,
canonical reason/recovery contracts, public `evidence skill` recording, digest
stamping and pruning, predecessor ordering, and the wave-orchestration
pre-summary repair.

Fresh proof captured in this closeout window:

- `go test ./cmd -run 'TestEvidenceSkillRecordsWaveOrchestrationFromRuntimeTaskEvidence|TestEvidenceSkillRejectsRunSummaryBoundWithoutExecutionSummary' -count=1`
- `go test ./internal/engine/progression -run 'TestWaveOrchestrationInputDigestUsesRuntimeTaskEvidence|TestMissingWaveDigestEntryStampsCurrentRuntimeTaskEvidenceWithoutTimestampRefusal|TestLoadExecutionTasksFromEvidence' -count=1`
- `go test ./internal/engine/sensitiveevidence ./internal/engine/progression ./internal/model ./internal/state ./cmd`
- `go test ./...`
- `go test -coverprofile=/tmp/slipway-issue156-cover.out ./...`
- `go run github.com/securego/gosec/v2/cmd/gosec@v2.27.1 -fmt=sarif -out=artifacts/changes/resolve-github-issue-156-add-a-change-implies-evidence-gate/verification/sast/gosec.sarif ./...`
- `go run . validate --json --change resolve-github-issue-156-add-a-change-implies-evidence-gate`

The gosec SARIF artifact has one run and zero results, supporting
`high_risk_check:external_api_contracts.safety_baseline=pass`.

## Evidence Index

- `verification/execution-summary.yaml`: run_summary_version 1, all five
  governed tasks passed, tasks_plan_hash
  `ac2b25f649b29a5fcc3ddb96de0ce554f6518cec58f756b5b7780fe09842ea2d`.
- `verification/wave-orchestration.yaml`: passing wave-orchestration evidence
  recorded after runtime task evidence, with digest inputs from
  `wave-plan.yaml` and runtime task evidence.
- `verification/spec-compliance-review.yaml`: passing spec trace for REQ-001
  through REQ-008, including external API contract guardrail coverage.
- `verification/code-quality-review.yaml`: passing line-level code quality and
  security review, with no findings.
- `verification/goal-verification-notes.md`: Exists/Substantive/Wired
  acceptance proof for AC-1 through AC-8.
- `verification/sast/gosec.sarif`: repo-native gosec SAST SARIF, zero results.
- `verification/sast/gosec-summary.md`: SAST command, scope, and triage summary.
- `verification/final-closeout-notes.md`: final closeout checks and assurance
  section judgments.

## Requirement Coverage

- REQ-001: `internal/engine/sensitiveevidence/evaluate.go` classifies schema
  migrations and requires `migration-applied`; evaluator tests cover missing and
  present marker behavior.
- REQ-002: auth/authz/RBAC/permission files require `auth-review`; evaluator
  tests cover path and filename variants.
- REQ-003: API contract files require `contract-test`; evaluator tests cover
  OpenAPI/proto-style contract paths.
- REQ-004: canonical reason code `sensitive_evidence_missing` is registered and
  emitted with `<category>:<path>` detail; reason-code contract tests preserve
  taxonomy stability.
- REQ-005: `evidenceMarkers` accepts matching markers from any passed task in
  the execution summary; evaluator tests cover a separate verification task
  owning contract evidence.
- REQ-006: `readiness.go` appends the sensitive-evidence gate after existing
  readiness checks; progression tests cover blocking, pass behavior, and
  coexistence with other gates.
- REQ-007: recovery text points to `slipway evidence task` and the required
  markers; recovery tests assert no force, skip, or environment-variable bypass.
- REQ-008: `cmd/evidence.go` exposes public skill evidence recording; CLI tests
  cover verification writes, digest stamping/pruning, predecessor rejection,
  run-summary requirements, and the wave-orchestration pre-summary path.

## Residual Risks and Exceptions

The initial sensitive path classifier is intentionally conservative and built
around common repository layouts and file names. A future configurable rule set
can broaden project-specific coverage without changing the fail-closed evidence
contract. No bypass path is introduced for sensitive evidence or high-risk
S4 safety checks.

## Rollback Readiness

Rollback is a git revert of the sensitive-evidence evaluator package,
readiness/recovery integration, public skill-evidence command additions,
verification storage support, reason/recovery contract updates, and generated
surface alignment. After rollback, rerun `go test ./...` and
`go run . validate --json --change resolve-github-issue-156-add-a-change-implies-evidence-gate`
from the active worktree before any archive or replacement change proceeds.

## Archive Decision

Do not archive until the active bundle passes final `validate --json` and the
lifecycle reaches `done-ready`. This closeout intentionally keeps the active
change unarchived while S4 evidence is recorded through the CLI. Before any
final `done` action, capture active validation with:

`go run . validate --json --change resolve-github-issue-156-add-a-change-implies-evidence-gate`
