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

<!--
Authoring guidance — the engine owns structure, the authoring skill owns substance:
- Each requirement is "### Requirement: <title>" + a stable "REQ-" identifier line
  whose body states what the system MUST, SHALL, or is REQUIRED to do (an RFC-2119
  strong-obligation keyword).
- Each requirement needs at least one concrete "#### Scenario:" with real GIVEN/WHEN/THEN
  (no placeholder or tautology lines).
- Replace the seeded placeholder below; an unedited scaffold is rejected by the
  requirements substance gate and cannot reach done.
- Run `slipway instructions requirements` for the full template and quality bar.
-->

{{ .SeededRequirements }}
