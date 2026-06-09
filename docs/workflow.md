# Governed Workflow

Slipway routes work through a governed lifecycle:

1. `S0_INTAKE`: capture intent, scope, open questions, and initial evidence.
2. `S1_PLAN`: produce research, requirements, decision, task, and plan-audit artifacts.
3. `S2_EXECUTE`: execute dependency-ordered waves from `tasks.md`.
4. Review and closeout stages: verify implementation against artifacts, run quality checks, author the `assurance.md` closeout record, and finalize done-ready work.

The active lifecycle state is stored in `artifacts/changes/<slug>/change.yaml`.

<div align="center" markdown>

![Slipway governed lifecycle: new, S0 Intake, S1 Plan, S2 Execute, S3 Review, S4 Verify, done, with clarify, audit, wave and changes-requested loop-backs and a primary command loop of new, next, run and done](assets/diagrams/lifecycle.svg)

</div>

## Create A Change

```bash
slipway new "refresh governance docs" --preset standard
```

JSON stdin lets AI callers provide classification directly:

```bash
echo '{"guardrail_domain":"","needs_discovery":true,"complexity":"complex","test_cmd":"go test ./...","build_cmd":"go build ./...","languages":["Go","Markdown"]}' \
  | slipway new --json "refresh governance docs"
```

When classification is omitted, Slipway uses conservative defaults:

- `guardrail_domain=""`
- `needs_discovery=true`
- `complexity="complex"`

## Progression Styles

Use `next` for explicit handoff control:

```bash
slipway next --json
# complete the surfaced skill or resolve blockers
slipway run --json
slipway next --json
```

Use `run` when you want Slipway to advance until an operator-facing stop:

```bash
slipway run --json --diagnostics
```

`run` stops on a surfaced skill, blocker, checkpoint, or done-ready outcome.

## Read-Only Surfaces

These commands inspect state without mutating lifecycle authority:

- `slipway next`
- `slipway status`
- `slipway validate`
- `slipway learn --preview`

Use `--json` for machine-readable output. Use `--diagnostics` on `next` or `run` when you need gate details, artifact readiness, transition traces, or context-budget diagnostics.

## Open Questions Semantics

`intent.md` may contain a canonical `## Open Questions` section. The engine gates
on **structure, not prose**: only an unchecked checklist item blocks intake.

These read as resolved (intake advances to `S0_INTAKE/confirm`):

```markdown
## Open Questions
(none)
```

```markdown
## Open Questions
- None requiring research — the page model is already specified.
```

```markdown
## Open Questions
- [x] Installer path resolved by research.
```

Only an unchecked `- [ ]` entry blocks (routes to `S0_INTAKE/research`):

```markdown
## Open Questions
- [ ] Which installer path should be documented?
```

Free-form prose and bare bullets are **documentation, never a blocker**. Deciding
whether something is a genuine open question is a semantic judgment owned by the
`intake-clarification` skill, which records a real unknown as a `- [ ]` item; the
engine never parses intent prose. This keeps a no-unknowns change (`None`, a
sentinel sentence, or an empty section) from silently detouring into research,
while letting an artifact preserve question history with `- [x]`. When an entry
does block, `slipway run` names the specific `- [ ]` line so the routing is not
silent.

## Done

When the governed state is done-ready:

```bash
slipway done --json
```

`done` finalizes the active change and archives terminal state. If local state looks inconsistent after interruption, inspect first with `slipway health --doctor`, then run `slipway repair` if the suggested repairs match the issue.
