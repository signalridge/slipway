# Domain: intake-and-framing

Catalog skills that shape intent, scope, context, and the plan bundle before
execution starts.

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `scope-clarification` | T1 | host `intake-clarification`; technique-hint |
| `context-assembly` | T1 | hosts `research-orchestration`, `plan-audit`; technique-hint |
| `plan-authoring` | T1 | host `plan-audit`; host-embedded; export-only |

Role:

1. Drive the S0 / early-S1 kernel states.
2. Constrain scope, gather context, and shape plan bundles before execution
   begins.
3. Export authoring guidance (plan-authoring) to external adapters.

Notes:

- `scope-clarification` is the one intake-posture skill injected above the
  existing `intake-clarification` host. It does not replace the host.
- `context-assembly` carries the `hydrate_references[]` contract from B2 onward;
  resolver emission is pending activation.
- `plan-authoring` absorbs `superpowers/writing-plans`, `wshobson/workflow-patterns`,
  and the spec-kitty mission-system posture.
