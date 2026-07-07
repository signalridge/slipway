# Governed Workflow

Slipway routes work through a governed lifecycle:

1. `S0_INTAKE`: capture intent, scope, open questions, and initial evidence.
2. `S1_PLAN`: produce research, requirements, decision, task, and plan-audit artifacts. Plan-audit is the review that permits S2 to start; it reviews the plan bundle itself, not a frozen wave cache.
3. `S2_IMPLEMENT`: execute computed waves. Slipway computes the wave schedule live from each task's declared dependencies and target files in the current `tasks.md`; authors never declare wave numbers. Dependency-free, file-disjoint tasks share a wave and are dispatched concurrently by default — `slipway next --json` marks such a wave `parallel: true`. Set `execution.parallelization: off` in `.slipway.yaml` to run waves sequentially instead.
4. `S3_REVIEW`: verify implementation against artifacts, run selected review checks, repair feedback through separate subagents, then run the single terminal `ship-verification` gate (one authoritative full suite, acceptance proof, freshness recheck, the `assurance.md` attestation, and reviewer-independence attestation) to produce a done-ready outcome.

The active lifecycle state is stored in `artifacts/changes/<slug>/change.yaml`.
Bundle-local lifecycle events stay under
`artifacts/changes/<slug>/events/`, and skill verification records stay under
`artifacts/changes/<slug>/verification/`. Runtime task evidence recorded during
wave execution lives under
`.git/slipway/runtime/changes/<slug>/evidence/...`.

<div align="center" markdown>

![Slipway governed lifecycle: new, S0 Intake, S1 Plan, S2 Implement, S3 Review, done-ready, done, with explicit lifecycle commands and run as a shortcut](assets/diagrams/lifecycle.svg)

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

`run` stops on a surfaced skill, blocker, or done-ready outcome.

## Independence Attestation Tokens

The review, ship-verification, and wave-orchestration stages record a few
engine-consumed tokens on the verification record's `references` (via
`slipway evidence skill --reference ...`). Each is an error-severity blocker on
`standard`/`strict` and advisory-only on `light` (realized as Pattern-A omission —
the gate simply returns no blocker on `light`, there is no separate advisory
channel in this seam). No token lets a stage self-certify freshness or a final
verdict; the engine remains the sole timestamp and run-version
stamper.

| Token | Attests | Enforced | Recovery when the gate fails closed |
| --- | --- | --- | --- |
| `context_origin:stage=<stage>=<handle>` across the chain participants, with selected S3 reviewers all using `stage=review` and review-finding fixes using `stage=fix` when present | each owned participant ran under distinct contexts on the shared worktree; selected reviewers are keyed by skill name and must be pairwise distinct; recorded fix handles must not collapse with implementation or review handles | standard/strict error, light advisory | re-run the owning reviewer or fix through its configured fresh delegated session so it re-emits a distinct `context_origin` handle |
| `closeout:reviewer_independence=pass` on ship-verification | the reviewer-independence attestation is present on the terminal ship record (Pattern-A); missing fails closed with `ship_verification_reviewer_independence_missing` | standard/strict error, light advisory | re-run **ship-verification** and record the token |
| `closeout:assurance_complete=pass` on ship-verification | the host attests `assurance.md` is complete on the terminal ship record; missing fails closed with `ship_verification_assurance_attestation_missing` | standard/strict error, light advisory | re-run **ship-verification** and record the token |
| terminal ordering `ship-verification >= every selected S3 peer` (always-on, no token) | the terminal ship record was stamped after every selected S3 review peer rather than before any of them, so the gate observed the final review evidence | every preset (always-on; no light carveout) | re-stamp the stale selected reviewer, then re-run **ship-verification** so its verdict timestamp is at or after every peer |
| `degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>` on wave-orchestration | a `degraded_sequential` dispatch was paired with a genuine tool-unavailable justification | standard/strict error, light advisory | re-record wave-orchestration evidence with the justification reference, or re-run the wave with real concurrent dispatch |

A bare `degraded_sequential` with no paired justification is rejected on every
path that synchronizes governed wave execution, including the
`slipway evidence skill` path — not only advance/next.

`context_origin:stage=<stage>=<handle>` is one chain-wide grammar that spans the
whole governed chain. The S3 selected review set includes spec and independent
reviewers for every workflow profile; code-quality review joins when the profile
requires code-quality review, and security review joins when the engine-derived
security control selects it. The terminal `ship-verification` gate is not a
selected peer — it runs last, after the peers converge. All selected review hosts emit
`context_origin:stage=review=<handle>`; the R2 lattice keys each review
participant by skill name, not by the shared `review` stage. The other
participants are the S2 wave `executor`, the S1 plan-audit `audit_origin` (paired
against the plan's `plan_origin` author), and optional S3 review-finding `fix`
handles recorded on reviewer evidence. The collision lattice is owned per seam,
so each edge is checked exactly once:

| Seam | Owns | Edges |
| --- | --- | --- |
| Plan gate (S1) | only the local `audit_origin != plan_origin` edge (plan-audit author vs auditor self-audit) | 1 |
| Review authority | every edge among `{executor, fix}` plus the selected review-skill keys; S1 `audit_origin` is not a live S3 participant | variable by workflow profile, selected security control, and optional fix handle |
| Ship authority | no additional context-origin edges; the terminal `ship-verification` gate owns the terminal ordering invariant plus the reviewer-independence and assurance-complete presence attestations | 0 |

When a seam fails closed, re-run its owning stage or selected reviewer through
the configured fresh delegated session, defaulting to native host dispatch, so
the stage re-emits a distinct `context_origin` handle; the engine remains the
sole verdict stamper and never restamps the collapsed handle.

The `context_origin` lattice is **audit/structural tier**: the handles are
host-emitted strings — the same structural tier as the executor-dispatch
handles — so it raises the cost and auditability of collapsing chain stages into
one authoring context but is never cryptographic proof of independence. Genuine
non-forgeable distinct-context discrimination (an engine-issued per-stage nonce or
lifecycle-event boundary, "Option B") is infeasible within this change's
constraints, so no gate here is overstated as cryptographic distinct-context proof.

## S3 Review Dispatch

At `S3_REVIEW` the engine resolves one selected review set and exposes that set
through the command surfaces. Spec and independent review are selected for every
workflow profile; code-quality review joins only when the profile requires
code-quality review, and the security reviewer joins only when the engine-derived
security control is selected. `slipway next` exposes the selected set, and host
adapters fan those reviewers out using the configured `review` slot, defaulting
to concurrent native subagents when no slot is configured. Any conventional
single primary skill is only a compatibility projection for surfaces that truly
need one; it does not imply review ordering. The terminal `ship-verification`
gate is dispatched after this peer set converges, never as a member of it.

Selected reviewers are **unordered peers**: none blocks another, and requiredness,
review authority, ship authority, and stale-evidence recovery all consume the
same selected set. Every selected reviewer records
`context_origin:stage=review=<handle>` with its own distinct handle. The R2
lattice compares those handles under skill-name participant keys, so duplicate
reviewer handles fail closed even though the wire token's stage label is shared.
When a review finding is repaired through `slipway fix`, the affected reviewer
also records `context_origin:stage=fix=<repair-handle>` on rereview; any recorded
fix handle participates in the same distinct-context lattice.
Missing selected reviewer evidence is owned by required-skill blockers; a passing
selected review record with no well-formed `stage=review` handle fails closed
with `context_origin_handle_invalid`; collisions fail with
`cross_stage_context_not_distinct`. Unselected security evidence on disk is
silent and never becomes a hidden participant.

Selected reviewer freshness is anchored by the current diff, planning artifacts,
and `run_summary_version`; there is no shared suite-result keystone for the peer
set to consume. The one authoritative full suite — and any guardrail SAST
baseline — is run by the terminal `ship-verification` gate, once, after the
peers converge, and recorded on its own evidence record rather than a record
shared with the reviewers.

## Read-Only Surfaces

These commands inspect state without mutating lifecycle authority:

- `slipway next`
- `slipway status`
- `slipway validate`

Use `validate` directly for its machine-readable JSON report. Use `--json` on
read-only commands that still expose text by default, such as `next` and
`status`. Use `--diagnostics` on `next` or `run` when you need gate details,
artifact readiness, or transition traces.

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
slipway done
```

`done` finalizes the active change and archives terminal state. If local state looks inconsistent after interruption, inspect first with `slipway health --doctor`, then run `slipway repair` if the suggested repairs match the issue.
