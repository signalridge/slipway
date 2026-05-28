# Intent
{{- if or .ProjectTechStack .ProjectConventions .ProjectTestCmd .ProjectBuildCmd .ProjectLanguages }}

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: {{ .ProjectTechStack }}
- Languages: {{ .ProjectLanguages }}
- Test Command: {{ .ProjectTestCmd }}
- Build Command: {{ .ProjectBuildCmd }}
- Conventions: {{ .ProjectConventions }}
{{- end }}

## Summary
{{ if .InitialRequest }}{{ .InitialRequest }}{{ else }}Describe the change objective.{{ end }}
## Complexity Assessment
{{ .ComplexityLevel }}
<!-- Rationale: provide justification for the assessed complexity level -->
{{- if ge .ComplexityRank 2 }}

## Guardrail Domains
{{ if .GuardrailDomain }}{{ .GuardrailDomain }}{{ else }}<!-- none detected -->{{ end }}
{{- end }}

## In Scope
<!-- What is explicitly included -->
{{- if ge .ComplexityRank 1 }}

## Out of Scope
<!-- What is explicitly excluded -->

## Constraints
<!-- Technical / business / time constraints -->
{{- end }}

## Acceptance Signals
<!-- What verifiable signals indicate completion -->
{{- if ge .ComplexityRank 2 }}

## Open Questions
<!-- Unresolved questions → consumed by S1_PLAN/research -->

## Deferred Ideas
<!-- Identified but postponed ideas -->
{{- end }}

## Approved Summary
<!-- User-confirmed final summary + confirmation timestamp -->
