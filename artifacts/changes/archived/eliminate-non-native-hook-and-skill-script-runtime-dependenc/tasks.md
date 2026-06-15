# Tasks

## Task List

- [x] `t-01` Implement compiled hook behavior and native launcher templates.
  - depends_on: []
  - target_files: [cmd/context_pressure_hook.go, cmd/session_start_hook.go, cmd/session_start_hook_test.go, internal/tmpl/templates/hooks/session-start.sh.tmpl, internal/tmpl/templates/hooks/context-pressure-post-tool-use.sh.tmpl, internal/tmpl/templates/hooks/session-start.ps1.tmpl, internal/tmpl/templates/hooks/context-pressure-post-tool-use.ps1.tmpl, internal/tmpl/templates/hooks/session-start.cmd.tmpl, internal/tmpl/templates/hooks/context-pressure-post-tool-use.cmd.tmpl, internal/tmpl/hooks_behavior_test.go, internal/tmpl/templates_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - evidence: compiled hook command tests and launcher template contract tests
  - acceptance: `go test ./cmd ./internal/tmpl` passes for hook-related tests without executing generated bash business logic.

- [x] `t-02` Update toolgen hook registry, settings merge, stale cleanup, and generated adapter contracts.
  - depends_on: [t-01]
  - target_files: [internal/toolgen/toolgen.go, internal/toolgen/adapter_contract_test.go, internal/toolgen/toolgen_test.go, internal/toolgen/worktree_provision_test.go, internal/toolgen/surface_manifest_test.go, internal/state/worktree_provision_test.go, docs/SURFACE-MANIFEST.json]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-004]
  - evidence: toolgen tests reject legacy `bash "<hook>.sh"` registrations and preserve unrelated user hook settings
  - acceptance: `go test ./internal/toolgen` passes and generated settings contain native launcher commands, not canonical `bash` hook commands.

- [x] `t-03` Implement binary-backed skill helper commands and command tests.
  - depends_on: []
  - target_files: [cmd/root.go, cmd/command_description_contract_test.go, cmd/tool.go, cmd/tool_sarif.go, cmd/tool_actions.go, cmd/tool_polluter_go.go, cmd/tool_variant.go, cmd/tool_github.go, cmd/tool_test.go]
  - task_kind: code
  - covers: [REQ-003]
  - evidence: command tests for every supported `slipway tool` helper using local files, in-process HTTP servers, and injected GitHub backend selection
  - acceptance: `go test ./cmd` passes for tool helper tests without generated bash, Python, jq, or shell helper payloads; GitHub helpers prefer `gh`, support token API fallback, and fail closed when no authenticated backend is available.

- [x] `t-04` Migrate generated skill templates away from executable helper scripts.
  - depends_on: [t-03]
  - target_files: [internal/tmpl/templates.go, internal/tmpl/templates/skills/ci-triage/SKILL.md, internal/tmpl/templates/skills/review-comment-triage/SKILL.md, internal/tmpl/templates/skills/root-cause-tracing/references/root-cause-tracing.md, internal/tmpl/templates/skills/sast-orchestration/references/sarif-merge.md, internal/tmpl/templates/skills/variant-analysis/SKILL.md, internal/tmpl/templates/skills/ci-triage/references/fetch-pr-checks-shell-evaluation.md, internal/tmpl/templates/skills/sast-orchestration/scripts/merge-sarif.sh, internal/tmpl/templates/skills/review-comment-triage/scripts/reply-to-thread.py, internal/tmpl/templates/skills/review-comment-triage/scripts/fetch-review-requests.sh, internal/tmpl/templates/skills/review-comment-triage/scripts/fetch-pr-feedback.py, internal/tmpl/templates/skills/ci-triage/scripts/fetch-pr-checks.py, internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh, internal/tmpl/templates/skills/gha-security-review/scripts/pin-actions.sh, internal/tmpl/templates/skills/variant-analysis/scripts/find-variant.sh, internal/tmpl/templates/skills/_shared/scripts/gh-common.sh, internal/toolgen/support_files_test.go, internal/toolgen/testdata/skill_tree_inventory.codex.golden]
  - task_kind: code
  - covers: [REQ-003, REQ-004]
  - evidence: template inventory and support-file tests show no generated Slipway-owned `scripts/` helper payload remains
  - acceptance: `go test ./internal/tmpl ./internal/toolgen` passes and `find internal/tmpl/templates/skills -path '*/scripts/*' -type f` returns no Slipway helper files.

- [x] `t-05` Update operator documentation and run governed verification.
  - depends_on: [t-02, t-04]
  - target_files: [docs/ai-tools.md, docs/commands.md, docs/installation.md, artifacts/changes/eliminate-non-native-hook-and-skill-script-runtime-dependenc/verification/implementation-verification.md, artifacts/changes/eliminate-non-native-hook-and-skill-script-runtime-dependenc/assurance.md]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - evidence: implementation verification note with focused tests, GitHub backend selection checks, full repo tests, diff check, and Slipway validation
  - acceptance: `gofmt -l`, `go test -count=1 ./...`, `git diff --check`, and `go run . validate --json` complete successfully before review closeout.
