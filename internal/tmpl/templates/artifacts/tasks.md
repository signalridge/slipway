# Tasks

{{- if or .ProjectTechStack .ProjectConventions .ProjectTestCmd .ProjectBuildCmd .ProjectLanguages }}
## Project Context
- Tech Stack: {{ .ProjectTechStack }}
- Conventions: {{ .ProjectConventions }}
- Test Command: {{ .ProjectTestCmd }}
- Build Command: {{ .ProjectBuildCmd }}
- Languages: {{ .ProjectLanguages }}

{{- end }}

## Task List

<!--
Authoring guidance — the engine owns structure, the authoring skill owns substance:
- Each task is "- [ ] `t-NN` <real objective>" with wave, depends_on, target_files,
  task_kind, and covers metadata.
- Replace the seeded placeholder objective below; an unedited placeholder tasks
  list is rejected by the tasks substance gate.
- Run `slipway instructions tasks` for the full template and quality bar.
-->

{{ .SeededTasks }}
