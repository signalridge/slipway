# Requirements

{{- if or .ProjectTechStack .ProjectConventions .ProjectTestCmd .ProjectBuildCmd .ProjectLanguages }}
## Project Context
- Tech Stack: {{ .ProjectTechStack }}
- Conventions: {{ .ProjectConventions }}
- Test Command: {{ .ProjectTestCmd }}
- Build Command: {{ .ProjectBuildCmd }}
- Languages: {{ .ProjectLanguages }}

{{- end }}

## Requirements

{{ .SeededRequirements }}
