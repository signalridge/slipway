# Tasks

{{- if or .ProjectTechStack .ProjectConventions .ProjectTestCmd .ProjectBuildCmd .ProjectLanguages }}
## Project Context
- Tech Stack: {{ .ProjectTechStack }}
- Conventions: {{ .ProjectConventions }}
- Test Command: {{ .ProjectTestCmd }}
- Build Command: {{ .ProjectBuildCmd }}
- Languages: {{ .ProjectLanguages }}

{{- end }}

## Format

Use checkbox-native tasks so humans and automation read the same file.

Each task starts with a checkbox line, then indented metadata bullets:

```
- [ ] `t-01` Short objective                            # required
  - wave: 1                                            # required
  - depends_on: []                                      # required (may be empty)
  - target_files: [path/to/file.go]                     # required
  - task_kind: code                                     # optional — improves auditability
  - covers: [REQ-001]                                   # optional — improves traceability
```

**Required fields**: task_id (in checkbox line), objective (in checkbox line), wave, depends_on, target_files.
**Optional/recommended fields**: task_kind, covers.

Allowed `task_kind` values: `code`, `test`, `doc`, `ops`, `verification`, `investigation`, `other`.
`wave` must be a positive integer. Declare the execution grouping explicitly in `tasks.md`; Slipway validates the grouping and materializes `wave-plan.yaml` from it.

## Task List

{{ .SeededTasks }}
