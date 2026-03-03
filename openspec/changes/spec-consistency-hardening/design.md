## Context

This is a spec-hardening change with no workflow-state topology rewrite.
Scope is limited to schema and validation contract consistency.

## Design Decisions

### DEC-01: Governance Evidence Must Be Request-Scoped

- required field: `request_id`
- path: `.spln/evidence/skills/<request_id>/<session_id>--<skill_name>.json`
- readiness checks reject evidence missing `request_id`

### DEC-02: Governed Planning Uses Canonical Task Nodes

- `tasks.md` must expose `## Task Nodes` + fenced YAML root `tasks`
- each task node must carry required wave fields
- dependency references must resolve and DAG must be acyclic
- malformed structure blocks planning before DAG build

### DEC-03: CLI Failure Taxonomy Is Local Canonical Contract

- CLI error taxonomy and envelope are normative inside `cli-commands`
- design/proposal documents can restate rationale but cannot override values

### DEC-04: Minimize Denormalized Mitigation State

- `mitigation_target` remains optional for readability
- writers should omit by default; consumers derive from `skill_name`
- if present, value must match registry mapping

## Risks

- Existing tooling that assumes global `.spln/evidence/skills/*` flat listing must add request partition handling.
- Governed authors now need canonical task-node block in `tasks.md`; template/parser updates must stay aligned.

## Validation

- `openspec status --change spec-consistency-hardening`
- `openspec validate --changes`
