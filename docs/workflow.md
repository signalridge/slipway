# Governed Workflow

Slipway routes work through a governed lifecycle:

1. `S0_INTAKE`: capture intent, scope, open questions, and initial evidence.
2. `S1_PLAN`: produce research, requirements, decision, task, and plan-audit artifacts.
3. `S2_EXECUTE`: execute computed waves. Slipway computes the wave schedule from each task's declared dependencies and target files in `tasks.md`; authors never declare wave numbers. Dependency-free, file-disjoint tasks share a wave and are dispatched concurrently by default — `slipway next --json` marks such a wave `parallel: true`. Set `execution.parallelization: off` in `.slipway.yaml` to run waves sequentially instead.
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

## Independence Attestation Tokens

The review, verification, closeout, and wave-orchestration stages record a few
engine-consumed tokens on the verification record's `references` (via
`slipway evidence skill --reference ...`). Each is an error-severity blocker on
`standard`/`strict` and advisory-only on `light` (realized as Pattern-A omission —
the gate simply returns no blocker on `light`, there is no separate advisory
channel in this seam). None of them is a self-stamp of a freshness or final
verdict; the engine remains the sole timestamp and run-version stamper.

| Token | Attests | Enforced | Recovery when the gate fails closed |
| --- | --- | --- | --- |
| `review_origin:skill=<skill>=<handle>` on spec-compliance-review and code-quality-review | each review ran under a distinct per-review context; the two handles must be present and distinct | standard/strict error, light advisory | re-run both reviews under fresh, separately-labelled contexts and re-record via the spec-compliance-review and code-quality-review skills |
| `closeout:reviewer_independence=pass` on final-closeout | the closeout independence attestation the engine previously ignored is now present (Pattern-A) | standard/strict error, light advisory | re-run **final-closeout** and record the token |
| chain ordering `closeout >= goal-verification >= max(spec-compliance-review, code-quality-review)` (always-on, no token) | the four independence-critical verdicts were stamped in order, independent of the opt-in reuse token | standard/strict error, light advisory | re-stamp in order via the reviews, then **goal-verification**, then **final-closeout** (distinct `closeout_chain_order_invalid` code, not the reuse code) |
| `degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>` on wave-orchestration | a `degraded_sequential` dispatch was paired with a genuine tool-unavailable justification | standard/strict error, light advisory | re-record wave-orchestration evidence with the justification reference, or re-run the wave with real concurrent dispatch |

A bare `degraded_sequential` with no paired justification is rejected on every
path that synchronizes governed wave execution, including the
`slipway evidence skill` path — not only advance/next.

The `review_origin` handle gate is **audit/structural tier**: the handles are
host-emitted strings, so it raises the cost and auditability of collapsing four
verdicts into one authoring context but is not cryptographic proof. Genuine
non-forgeable distinct-context discrimination (an engine-issued per-stage nonce or
lifecycle-event boundary, "Option B") is infeasible within this change's
constraints, so no gate here is oversold as cryptographic distinct-context proof.

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
