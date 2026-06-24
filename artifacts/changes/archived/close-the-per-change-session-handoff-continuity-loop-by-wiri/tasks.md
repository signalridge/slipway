# Tasks

## Task List

- [x] `t-01` Escalate the context-pressure hook CRITICAL message into an imperative "author the handoff now" write directive; keep WARNING soft and retain the required substrings while avoiding authority-elevating wording
  - depends_on: []
  - target_files: [cmd/context_pressure_hook.go, cmd/context_pressure_hook_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-005]

- [x] `t-02` Remove the SessionStart hook's change-state auto-injection (active-worktree next view, bound-elsewhere session_handoff_info pointer, handoff summary) and its now-dead helpers; keep only the slipway_entry_skill routing pointer and the fail-silent contract; update the hook test
  - depends_on: []
  - target_files: [cmd/session_start_hook.go, cmd/session_start_hook_test.go, cmd/auto_mode_test.go]
  - task_kind: code
  - covers: [REQ-004]

- [x] `t-03` Add a "Continuing A Change In A Fresh Session" resume-protocol section to the generated slipway workflow skill template after the Runtime Session Handoff block, and add a toolgen guard asserting the generated skill ships it
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/workflow/SKILL.md.tmpl, internal/toolgen/toolgen_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
