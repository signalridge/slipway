# Testing

Re-authored for change
`resolve-github-issue-185-prevent-s4-goal-verification-from-s`
(GitHub issue #185).

- Existing coverage:
  - `internal/engine/progression/evidence_digests_test.go:809` through
    `internal/engine/progression/evidence_digests_test.go:831` proves a stored
    `goal-verification` digest stales when a normal target file changes.
  - Nearby tests cover review digest staleness, deleted S4 target files,
    execution-summary metadata exclusion, and final-closeout assurance input.
- Gap:
  - No prior test represented the issue #185 case where the target file is the
    current `artifacts/changes/<slug>/change.yaml` and the only subsequent
    mutation is `EvidenceRefs`.
- New regression:
  - `TestGoalAndCloseoutDigestIgnoresEvidenceRefOnlyChangeYAMLMutation` covers
    both `goal-verification` and `final-closeout`.
  - It stamps a digest for current `change.yaml`, records only the skill
    evidence pointer, and expects no stale blocker.
  - It then mutates `Description` in `change.yaml` and expects
    `required_skill_stale:<skill>:artifacts/changes/<slug>/change.yaml`.
- Verification commands:
  - Failing before fix:
    `go test -count=1 ./internal/engine/progression -run TestGoalAndCloseoutDigestIgnoresEvidenceRefOnlyChangeYAMLMutation`
  - Passing after fix:
    `go test -count=1 ./internal/engine/progression -run TestGoalAndCloseoutDigestIgnoresEvidenceRefOnlyChangeYAMLMutation`
  - Broader package check:
    `go test -count=1 ./internal/engine/progression`
- Planned final checks:
  - `go test -count=1 ./...`
  - `git diff --check`
  - `go run . validate --json`
