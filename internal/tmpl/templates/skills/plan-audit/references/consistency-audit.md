# Consistency Audit

Use this sidecar when plan-audit needs the semantic consistency judgement that
the engine cannot compute deterministically.

## What To Check
- Requirements, decision, research, and tasks use the same terms for the same
  concepts.
- Requirement statements, scenarios, task objectives, and task acceptance
  signals support one another instead of drifting into adjacent goals.
- The selected approach matches the alternatives and tradeoffs that the plan
  claims were considered.
- Acceptance signals are checkable and unambiguous; do not treat advisory prose
  as an executable contract.
- `can_advance=true`, a green structural check, or a complete checklist is not
  semantic proof that the plan is coherent.

## How To Record
For a pass, record a parseable reference on the plan-audit verification record:

```bash
--reference "dim:consistency=pass:<repo-path-or-artifact-line>"
```

The evidence reference may point to a governed artifact line when the judgement
is about internal artifact consistency. It may also point to source, docs,
config, or templates when those are the concrete consistency anchor.

For a fail, record:

```bash
--reference "dim:consistency=fail:<repo-path-or-artifact-line>" \
--blocker "plan_dimension_consistency_failed:<reason>"
```

Do not use notes alone as self-evidence. Notes explain the judgement; the
`dim:` reference makes the judgement parseable and recoverable.
