# Assurance

{{- if or .ProjectTechStack .ProjectConventions .ProjectTestCmd .ProjectBuildCmd .ProjectLanguages }}
## Project Context
- Tech Stack: {{ .ProjectTechStack }}
- Conventions: {{ .ProjectConventions }}
- Test Command: {{ .ProjectTestCmd }}
- Build Command: {{ .ProjectBuildCmd }}
- Languages: {{ .ProjectLanguages }}

{{- end }}

## Scope Summary
Summarize delivered scope.

## Verification Verdict
Summarize verification outcomes.

## Evidence Index
List supporting evidence references.

## Requirement Coverage
Map requirements to verification evidence.

## Residual Risks and Exceptions
List remaining risks or accepted exceptions.

## Rollback Readiness
Summarize rollback constraints, prerequisites, and verification status when rollback planning is required.

## Archive Decision
Record archive readiness decision. Include whether active `validate --json`
freshness/readiness proof was captured before `done`; do not describe archived
bundles as revalidated through the active validate gate.
