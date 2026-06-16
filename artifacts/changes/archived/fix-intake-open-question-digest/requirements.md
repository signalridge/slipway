# Requirements

## Requirements

### Requirement: Intake Digest Excludes Research-Owned Open Questions
REQ-001: The system MUST compute the `intake-clarification` input digest for
`intent.md` without treating `## Open Questions` checklist state or resolution
notes as intake-owned material.

#### Scenario: Open Questions resolution after intake
GIVEN a governed change has passing `intake-clarification` evidence stamped from
an `intent.md` with an unresolved Open Question
WHEN research resolves that question by changing `- [ ]` to `- [x]` and adding a
resolution note under `## Open Questions`
THEN stale evidence recovery MUST NOT target `S0_INTAKE` for
`intake-clarification`.

### Requirement: Substantive Intake Changes Still Reopen Intake
REQ-002: The system MUST continue to stale `intake-clarification` evidence when
substantive intake sections such as Summary, scope, constraints, acceptance
signals, or Approved Summary change after intake evidence is recorded.

#### Scenario: Summary changes after intake
GIVEN a governed change has passing `intake-clarification` evidence
WHEN the `## Summary` section in `intent.md` changes after that evidence was
stamped
THEN stale evidence recovery MUST target `S0_INTAKE` with
`required_skill_stale:intake-clarification:intent.md`.

### Requirement: Downstream Planning Inputs Remain Complete
REQ-003: The system MUST keep downstream discovery and planning skills sensitive
to the full `intent.md` material they already consume unless a skill-specific
ownership boundary is explicitly defined.

#### Scenario: Research and plan digests still see Open Questions
GIVEN research-orchestration and plan-audit input digests are computed from a
governed bundle
WHEN `## Open Questions` content in `intent.md` changes
THEN those downstream digests SHALL continue to change because they consume the
full planning artifact.
