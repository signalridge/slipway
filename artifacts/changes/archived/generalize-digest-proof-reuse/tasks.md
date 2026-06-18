# Tasks

## Task List

- [x] `t-01` Implement the internal proof-reuse edge validator and preserve the closeout wrapper
  - depends_on: []
  - target_files: ["internal/engine/progression/authority.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - evidence: verdict
  - acceptance: authority.go contains an internal proof-reuse edge/check validator that is not hardcoded to final-closeout, and the existing closeout wrapper still maps invalid reuse to closeout_goal_verification_reuse_invalid

- [x] `t-02` Rename genuinely shared closeout-only reuse helpers to proof-reuse-neutral helpers
  - depends_on: ["t-01"]
  - target_files: ["internal/engine/progression/authority.go", "internal/engine/progression/evidence_digests.go"]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: verdict
  - acceptance: only helpers shared by the generalized validator use proof-reuse-neutral names; public closeout references and blocker names remain unchanged

- [x] `t-03` Extend focused engine tests for edge validation and fail-closed invalidation
  - depends_on: ["t-01", "t-02"]
  - target_files: ["cmd/evidence.go", "cmd/evidence_skill_test.go", "cmd/execution_summary_test_helper_test.go", "internal/engine/progression/authority_test.go", "internal/engine/progression/evidence_digests_test.go", "internal/engine/progression/freshness_guard_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-005]
  - evidence: verdict
  - acceptance: tests cover valid closeout -> goal reuse plus stale run version, missing reuse run version, changed content, source proof older than execution evidence, missing or malformed suite-result proof, command-surface guardrail suite-result fixtures, missing guardrail SAST digest rejection, and selected-review context-origin restamp recovery

- [x] `t-04` Align generated goal-verification and final-closeout guidance with validated reuse
  - depends_on: ["t-01"]
  - target_files: ["internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl", "internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl", "internal/tmpl/templates_test.go"]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: verdict
  - acceptance: templates prefer engine-validated final-closeout reuse when current proof is fresh, keep rerun as the fail-closed fallback, and do not tell goal-verification to unconditionally skip suite-result production

- [x] `t-05` Run focused verification and update task evidence
  - depends_on: ["t-03", "t-04"]
  - target_files: ["artifacts/changes/generalize-digest-proof-reuse/verification/execution-summary.yaml"]
  - task_kind: verification
  - covers: [REQ-005]
  - evidence: artifact
  - acceptance: focused verification records the passing command output for go test ./internal/engine/wave ./internal/engine/progression -count=1 and any template test command required by t-04
