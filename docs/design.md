# Design Philosophy

Slipway is a small governance control plane for local AI-assisted development. It does not replace an AI coding tool, a project tracker, or Git. It makes agent work legible by binding every change to a lifecycle, a current authority file, and evidence that can be inspected after the session ends.

## Principles

| Principle | Meaning |
| --- | --- |
| Local-first | The repository contains the active state and audit trail. A hosted service can be useful later, but it is not required to understand a change. |
| One authority | `change.yaml` owns current lifecycle state. Lifecycle logs explain how state changed; they do not replace current state. |
| Bounded autonomy | Agents can move work forward, but Slipway exposes gates, blockers, review requirements, and done-ready proof. |
| Adapter thinness | Claude, Codex, Copilot, Cursor, Kilo, Kiro, OpenCode, Pi, Qwen, and Windsurf surfaces route into the CLI. They should not become separate governance engines. |
| Artifact traceability | Intent, research, requirements, decisions, tasks, execution evidence, review evidence, and assurance remain connected. |
| Fresh verification | A completion claim is valid only when current evidence proves the current worktree state. |

## Advantage Axes

Slipway's value is not one gate; it is that every governed stage owns evidence the engine **re-derives instead of trusting**, across several independent axes. Each axis is stated at its actual enforcement level — structural where it is structural, genuinely enforced where it is — and never overstated. Adjacent spec, workflow, and skill toolkits structure work well; the divide is that they enforce process by *asking* the model to comply, while these axes are checked in compiled code the model runs but cannot rewrite.

| Axis | Enforcement tier | In one line |
| --- | --- | --- |
| 1. Attested fresh context | Audit/structural | A per-seam `context_origin` lattice fails closed when stages that must be independent share a handle |
| 2. Tamper-evident evidence | Input digest (S3 certs) + structural (execution) | Freshness is re-derived from authoritative inputs, never the verification record's own claims |
| 3. Two-sided parallel safety | Genuinely enforced | File-disjoint wave planning plus four post-dispatch changed-file safety nets |
| 4. Scope containment | Genuinely enforced | `target_files` is a contract checked with the planner's own `TargetCoversPath` predicate |
| 5. Drift-aware forward recovery | Genuinely enforced | Forward-only reopen; `next` projects the repair as a named command |
| 6. Local-first, git-native audit | Genuinely enforced | `change.yaml` authority plus an append-only, readback-verified `lifecycle.jsonl` |
| 7. Risk-tiered guardrails | Genuinely enforced (fail-closed) | Sensitive domains require high-risk checks and get no bypass, force-close, or self-attestation path |

The actual enforcement level matters: axes 3–7 are mechanisms the engine genuinely enforces, axis 2 mixes an input-digest check (S3 review certificates) with structural freshness (execution summaries), and axis 1 is deliberately the *audit/structural* tier — it raises the cost of faking independence without claiming cryptographic proof. The sections below give each axis its mechanism and competitive boundary.

### 1. Attested fresh context

Every stage records the distinct context handle it ran under (`context_origin:stage=<stage>=<handle>`), and a per-seam collision lattice fails closed when two stages that must be independent share a handle — reviewer versus implementer, plan auditor versus plan author, fix versus either. Recovery is to re-run the owning stage through its configured fresh delegated session, defaulting to native host dispatch, so it re-emits a distinct handle. This is **audit/structural tier**: the handles are host-emitted strings, so the lattice raises the cost and visibility of collapsing the chain into one authoring context, but it is not cryptographic proof of independence. gsd and superpowers *spawn* fresh subagents; Slipway also *checks the independence held*. See [Independence Attestation Tier](#independence-attestation-tier) for the full per-seam edge model and the honest residual.

### 2. Tamper-evident evidence

Freshness is computed from authoritative inputs, not from the verification record's own claims. Selected S3 review certificates are keyed to an engine-owned input digest (code diff, planning artifacts, task-scope hash, and run-summary version); the one authoritative full suite is owned by the terminal `ship-verification` gate rather than a shared keystone the peers consume. Execution-summary task freshness is **structural** (`change_id`, `run_summary_version`, `task_id`, `guardrail_domain`), and old hash-only summaries are treated as stale and regenerated. Either way, a hand-edited verdict or a drifted input is detected and named (`required_skill_stale:<skill>:<input>`) rather than trusted. The engine remains the sole verdict and run-version stamper; no gate can self-certify freshness, restamp a verdict, or force-close the record. Adjacent tools store state as Markdown/YAML the model maintains and could edit freely.

### 3. Two-sided parallel safety

The wave planner buckets tasks into dependency-ordered, file-disjoint waves with a deterministic topological sort; multi-task waves run concurrently by default (`execution.parallelization: off` opts out). The distinguishing half is the *post-dispatch* audit: four fail-closed safety nets check the **actual** recorded `changed_files` — scope escape, parallel-wave file overlap, missing or unjustified dispatch mode, and missing per-task executor handles. Dispatch itself is host-driven (the AI host fans out through the configured `executor` slot, defaulting to native subagents, per the materialized plan); Slipway runs no concurrent scheduler, it validates the resulting evidence. Peers that parallelize check the plan before dispatch; Slipway also audits what the agents actually edited afterward.

### 4. Scope containment

Each task's declared `target_files` in `tasks.md` is a scope contract, evaluated with the same `TargetCoversPath` predicate the wave planner uses for conflict detection, so "covers" and "conflicts" share one implementation. Recorded changes outside the contract fail closed (`scope_contract_drift` and siblings), each mapped to an actionable remediation. The scope contract has two disclosed exemptions, each surfaced on `validate`/`status`/`review --json` rather than silently applied. The durable codebase map under `artifacts/codebase/`: when only those context files are dirty, they stay out of `scope_contract.changed_files` and are surfaced as `scope_contract.exempt_context_files`. And a pass code task that honestly changed zero files, when it carries a `no_op_justification`, is exempted from the changed-files requirement and surfaced as `scope_contract.no_op_justified_tasks`.

### 5. Drift-aware forward recovery

When plan, code, and evidence diverge, the lifecycle reopens the change *in place* and forward-only — there is no backward state cascade that could hide the gap, and same-intent S3 plan or task amendments stay in review/fix while S2 remains completed. Blockers project into a `RecoverySummary` with one primary command and one step per blocker group, surfaced on the read-only `next`/`status`/`validate` JSON so the agent reads the next forward action directly instead of inferring private sequencing. A recovery that only works because the agent memorized a hidden flow is treated as a product defect, not a feature.

### 6. Local-first, git-native audit

`change.yaml` is the single current authority; `events/lifecycle.jsonl` is an append-only trace of every mutating event, written as an atomic rewrite with post-write readback verification and tagged with the acting surface (`actor_kind`, and `skill_id` for skill-driven steps). Evidence lives beside the code in the governed bundle under `artifacts/changes/`. Nothing requires a hosted service to understand a change, so the audit trail is sovereign by default and re-inspectable by any later human or AI session. The lifecycle log is audit evidence only — it never replaces `change.yaml` as current-state authority.

### 7. Risk-tiered guardrails

Sensitive domains — auth/authz, credentials/PII, financial flows, schema/data migration, irreversible operations, and external-API contracts — fail closed harder. They require per-domain high-risk checks before ship authority is granted, gate sensitive evidence at both S2 and S3, and get no bypass, force-close, or private-attestation path. The same lifecycle that is light on a throwaway change is unforgiving on the changes that can actually hurt you; `light` preset relaxes advisory tiers but never the sensitive-domain fail-closed lines.

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
| `closeout:reviewer_independence=pass` on ship-verification | Pattern-A presence, engine-consumed on the terminal ship record (`ship_verification_reviewer_independence_missing` when absent) | Structural presence |
| `closeout:assurance_complete=pass` on ship-verification | Pattern-A presence attesting `assurance.md` is complete (`ship_verification_assurance_attestation_missing` when absent) | Structural presence |
| Terminal ordering `ship-verification >= every selected S3 peer` | the terminal ship record is stamped at or after the unordered selected review set, so the gate observes the final review evidence | Genuinely enforced ordering |
| `degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>` | a `degraded_sequential` dispatch is paired with a tool-unavailable justification | Structural pairing |

Each gate fails closed at error severity on `standard`/`strict` and is advisory on
`light` — advisory is realized as Pattern-A omission (the gate returns no blocker
on `light`), not a separate advisory channel. No gate adds a bypass, force-close,
or self-attestation path; the engine stays the sole verdict stamper.

### Cross-stage context-origin lattice

`context_origin:stage=<stage>=<handle>` is one chain-wide grammar — emitted by the
independence skills on the shared worktree — that spans the whole governed chain.
S3 uses a selected review set: spec and independent review reviewers are selected
for every profile; code-quality review joins when the workflow profile requires
code-quality review; security review joins when the engine-derived security
control is selected. The terminal `ship-verification` gate runs after this set
converges and is not one of its peers. Every selected review host records the same
`context_origin:stage=review=<handle>` wire token, but the R2 lattice keys those
participants by the recording review skill name rather than by the shared
`review` stage. The other review-authority participants are the S2 wave
`executor` and optional S3 review-finding `fix` handles recorded on reviewer
evidence; S1 `audit_origin` is owned by the plan gate, not the live S3 review
seam. The collision lattice is owned per seam so no stage re-checks an edge
another seam already owns:

Selected reviewer freshness is keyed through the current diff, planning
artifacts, and run-summary version; there is no shared suite-result keystone for
the peers to consume. The one authoritative full suite — and any guardrail SAST
baseline — is run once by the terminal `ship-verification` gate after the peers
converge, recorded on its own evidence rather than a peer-shared record.

| Seam | Owns | Edges |
| --- | --- | --- |
| Plan gate (S1) | only the local `audit_origin != plan_origin` edge (plan-audit author vs auditor self-audit) | 1 |
| Review authority | every edge among `{executor, fix}` plus the selected review-skill keys; S1 `audit_origin` is not a live S3 participant | variable by workflow profile, selected security control, and optional fix handle |
| Ship authority | no additional context-origin edges; the terminal `ship-verification` gate owns the terminal ordering invariant plus the reviewer-independence and assurance-complete presence attestations | 0 |

When a seam fails closed, recovery is to re-run the owning stage or selected
reviewer through its configured fresh delegated session so it re-emits a
distinct `context_origin` handle; the engine never accepts self-issued claims,
restamps, force-closes, or treats unselected security evidence as a hidden
lattice participant.

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
- Slipway does not treat a green test run as a passing `ship-verification` gate when review, acceptance, or assurance evidence is missing.
- Slipway does not hide local state mutations behind read-only commands.

## What Counts As Complete

A governed change is complete only when the worktree, artifact bundle, verification records, and lifecycle state all agree.

1. The objective is represented in `intent.md` and the requirements contract.
2. Implementation files and docs satisfy the requirements.
3. Task evidence is fresh for the current execution run.
4. Spec and quality review records pass.
5. The terminal `ship-verification` gate proves the stated acceptance criteria with fresh 3-level evidence and the one authoritative full suite.
6. `slipway done` archives the terminal state after the done-ready outcome.
