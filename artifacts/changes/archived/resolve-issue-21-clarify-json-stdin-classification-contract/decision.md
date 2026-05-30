# Decision

## Alternatives Considered

### Option A: Template-only wording
Update only `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl` and `internal/tmpl/templates/_partials/command-new-body.tmpl`.

Tradeoff: lowest diff size, but leaves `references/command-reference.md` incomplete and does not resolve all surfaces named by issue #21.

### Option B: Metadata-backed command-reference notes plus targeted templates
Add command-specific notes to the shared toolgen command metadata, render those notes in workflow command references, and update the workflow skill plus `/slipway-new` body.

Tradeoff: slightly expands command metadata, but keeps command-reference prose generated from one source of truth and limits runtime blast radius.

### Option C: Add CLI classification flags
Introduce `--guardrail-domain`, `--complexity`, and `--needs-discovery` flags to `slipway new`.

Tradeoff: broadens runtime behavior and contradicts issue #21's explicit non-problem ruling.

## Selected Approach
Use Option B. Clarify JSON stdin classification in the workflow skill template, `/slipway-new` prompt partial, and generated workflow command reference through shared command metadata. Add generated-surface tests for Codex and Claude outputs. Do not add CLI flags or change runtime classification behavior.

## Interfaces and Data Flow
- `internal/toolgen/toolgen.go` remains the source of command metadata for generated command reference entries.
- `internal/tmpl/templates/skills/workflow/command-reference.md.tmpl` renders command metadata notes under the relevant command entry.
- `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl` renders the primary exported workflow guidance.
- `internal/tmpl/templates/_partials/command-new-body.tmpl` renders generated command/prompt bodies such as Codex `slipway-new.md` and Claude command entries.
- `internal/toolgen/toolgen_test.go` generates temporary adapter surfaces and asserts the exported contract text.

## Rollout and Rollback
Roll forward by changing templates, shared toolgen metadata, and tests in one commit. Roll back by reverting those files; no persisted runtime state or migration is affected.

## Risk
Low. The change is limited to generated documentation surfaces and tests. Main risks are wording drift between surfaces or accidentally implying unsupported CLI flags; tests will assert the JSON stdin and not-flags wording directly.
