# Decision

{{- if or .ProjectTechStack .ProjectConventions .ProjectTestCmd .ProjectBuildCmd .ProjectLanguages }}
## Project Context
- Tech Stack: {{ .ProjectTechStack }}
- Conventions: {{ .ProjectConventions }}
- Test Command: {{ .ProjectTestCmd }}
- Build Command: {{ .ProjectBuildCmd }}
- Languages: {{ .ProjectLanguages }}

{{- end }}

## Alternatives Considered

{{ .SeededDecision }}

## Selected Approach
{{ .SeededDecisionApproach }}

## Interfaces and Data Flow
{{ .SeededDecisionInterfaces }}

## Rollout and Rollback
{{ .SeededDecisionRollback }}

## Risk
{{ .SeededDecisionRisk }}
