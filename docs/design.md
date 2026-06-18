# Design Philosophy

Slipway is a small governance control plane for local AI-assisted development. It does not replace an AI coding tool, a project tracker, or Git. It makes agent work legible by binding every change to a lifecycle, a current authority file, and evidence that can be inspected after the session ends.

## Principles

| Principle | Meaning |
| --- | --- |
| Local-first | The repository contains the active state and audit trail. A hosted service can be useful later, but it is not required to understand a change. |
| One authority | `change.yaml` owns current lifecycle state. Lifecycle logs explain how state changed; they do not replace current state. |
| Bounded autonomy | Agents can move work forward, but Slipway exposes gates, blockers, review requirements, and done-ready proof. |
| Adapter thinness | Claude, Codex, Cursor, Gemini, and OpenCode surfaces route into the CLI. They should not become separate governance engines. |
| Artifact traceability | Intent, research, requirements, decisions, tasks, execution evidence, review evidence, and assurance remain connected. |
| Fresh verification | A completion claim is valid only when current evidence proves the current worktree state. |

## Architecture Model

<div align="center" markdown>

![Slipway architecture model: human and AI tool feed the slipway CLI, which writes the repository system of record (change.yaml, lifecycle.jsonl, Markdown artifacts, verification YAML); read-only surfaces read state, state-mutating surfaces write it](assets/diagrams/architecture.svg)

</div>

The separation matters. `next`, `status`, and `validate` can recompute readiness without mutating lifecycle authority. `run` and `done` are explicit mutation surfaces. Generated host files help AI tools discover the right action, but the CLI remains the execution authority.

## Design Boundaries

Slipway's durable design is expressed through its own authority boundaries, not through ongoing comparison with upstream tools. Adjacent workflow and agent systems can still be useful research inputs, but they do not define Slipway's runtime contract.

| Boundary | Slipway stance |
| --- | --- |
| Runtime authority | Keep `change.yaml` as current-state authority and lifecycle events as trace. |
| State mutation | Keep `next`, `status`, and `validate` read-only; reserve state changes for explicit mutation commands such as `run` and `done`. |
| Adapter surfaces | Generate host files as handoff aids. The stable contract is the generated path plus CLI command, not host-specific governance state. |
| Installation guidance | Document Slipway-owned release and initialization paths without making adapter installation a governance source of truth. |
| Execution evidence | Treat task evidence, review evidence, and final verification as first-class Slipway artifacts bound to the current run. |
| Scope discipline | Reuse small primitives when they fit, but avoid importing lane schedulers, dashboards, or project-management runtimes into the governance kernel. |

## Independence Attestation Tier

Slipway consumes a small set of independence attestations recorded on verification
references. They sit on a deliberate tier boundary, and the design states the
boundary honestly rather than overselling it.

| Attestation | What the engine enforces | Tier |
| --- | --- | --- |
| `context_origin:stage=<stage>=<handle>` emitted by the chain-wide independence skills on the shared worktree, with all selected S3 reviewers using `stage=review` and S3 review-finding fixes using `stage=fix` when recorded | the participant handles owned by each seam are present and pairwise distinct; selected reviewers are keyed by skill name even though they share the `review` wire stage; recorded fix handles must be distinct from implementation and reviewer handles | Audit/structural — raises forging cost and auditability, not cryptographic proof |
| `closeout:reviewer_independence=pass` on final-closeout | Pattern-A presence, now engine-consumed | Structural presence |
| Final ordering `final-closeout >= every selected S3 peer` | final closeout is stamped after the unordered selected review set, including goal-verification | Genuinely enforced ordering |
| `degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>` | a `degraded_sequential` dispatch is paired with a tool-unavailable justification | Structural pairing |

Each gate fails closed at error severity on `standard`/`strict` and is advisory on
`light` — advisory is realized as Pattern-A omission (the gate returns no blocker
on `light`), not a separate advisory channel. No gate adds a bypass, force-close,
or self-stamp path; the engine stays the sole verdict stamper.

### Cross-stage context-origin lattice

`context_origin:stage=<stage>=<handle>` is one chain-wide grammar — emitted by the
independence skills on the shared worktree — that spans the whole governed chain.
S3 uses a selected review set: spec, independent review, and goal-verification
reviewers are selected for every profile; code-quality review joins when the
workflow profile requires code-quality review; security review joins when the
engine-derived security control is selected. Every selected review host records the same
`context_origin:stage=review=<handle>` wire token, but the R2 lattice keys those
participants by the recording review skill name rather than by the shared
`review` stage. The other review-authority participants are the S2 wave
`executor` and optional S3 review-finding `fix` handles recorded on reviewer
evidence; S1 `audit_origin` is owned by the plan gate, not the live S3 review
seam. The collision lattice is owned per seam so no stage re-checks an edge
another seam already owns:

Selected reviewer freshness is keyed through
`verification/suite-result.yaml`, not a silent execution-summary fallback. The
suite-result record carries the current run-summary version plus the shared
full-suite and guardrail SAST digests. Missing or mismatched suite-result data
fails selected S3 review freshness closed; changed shared suite inputs stale the
selected peer set.

| Seam | Owns | Edges |
| --- | --- | --- |
| Plan gate (S1) | only the local `audit_origin != plan_origin` edge (plan-audit author vs auditor self-audit) | 1 |
| Review authority | every edge among `{executor, fix}` plus the selected review-skill keys; S1 `audit_origin` is not a live S3 participant | variable by workflow profile, selected security control, and optional fix handle |
| Ship authority | no additional context-origin edges; ship owns final ordering and closeout presence attestations | 0 |

When a seam fails closed, recovery is to re-run the owning stage or selected
reviewer in a fresh native subagent so it re-emits a distinct `context_origin`
handle; the engine never self-stamps, restamps, force-closes, or treats
unselected security evidence as a hidden lattice participant.

**Honest residual.** The `context_origin` lattice cannot prove that the chain's
stages ran in genuinely independent contexts, because the handles are
host-emitted strings — it is the *same structural tier as the executor-dispatch
handles*, not cryptographic proof of independence. True non-forgeable
distinct-context discrimination would require an engine-issued per-stage nonce or
a lifecycle-event boundary ("Option B"), which is infeasible within this change's
constraints: the independence skills share a run-version, timestamp monotonicity
only catches wrong-order, and the only zero-schema nonce is host-readable
plaintext. So the lattice is presented as audit/structural tier — it makes the
cheapest authoring-context collapse visible and costly across every owned seam —
and never as cryptographic distinct-context proof.

## Non-Goals

- Slipway does not infer a full project plan without governed artifacts.
- Slipway does not make AI-tool generated files authoritative over CLI state.
- Slipway does not treat a green test run as sufficient closeout when review or assurance evidence is missing.
- Slipway does not hide local state mutations behind read-only commands.

## What Counts As Complete

A governed change is complete only when the worktree, artifact bundle, verification records, and lifecycle state all agree.

1. The objective is represented in `intent.md` and the requirements contract.
2. Implementation files and docs satisfy the requirements.
3. Task evidence is fresh for the current execution run.
4. Spec and quality review records pass.
5. Final verification proves the stated acceptance criteria.
6. `slipway done` archives the terminal state after done-ready closeout.
