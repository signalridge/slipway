# ADR-0002: Add a seventh guidance-only capability, `slipway-workflow`

## Status

Accepted — 2026-07-18. This record explains why the previously closed six-capability set was deliberately expanded to seven. For repository behavior after this decision, this ADR narrowly supersedes every exact-six statement in issue #434: §6, §13 (including the Copilot count), §16, §18 scenario 28, and the non-normative English summary. It clarifies §6.1 only by keeping external workflows out of Clarify while adding this distinct Slipway-owned capability. It reaffirms §15's exclusion of launchers and global routers. Adding Workflow itself did not require changing the versioned machine protocol, Run model, or the other #434 boundaries.

## Context

The base contract in issue #434 §6 generated exactly six host capabilities — `run`, `clarify`, `propose`, `decompose`, `implement`, `review` — and §6.1 excluded introducing an external workflow. Users nonetheless wanted a single explicit entry point that carries a rough idea through the stateless upstream half (investigate → grill → synthesize a self-contained Change or Objective draft) and then hands off to the existing publication and Run capabilities, instead of manually chaining several separate user-invoked skills.

That upstream half cannot be folded into any of the six: `clarify` is stateless and does not materialize, `propose` only materializes, `decompose` only splits an Objective, and `run` drives an already-started Action loop. It also cannot be a router that fires other skills, because every relevant skill — Matt Pocock's front doors (`grill-me`, `wayfinder`, `to-spec`, `to-tickets`, `implement`) and Slipway's own `slipway-propose`/`slipway-run` — is user-invoked (`disable-model-invocation: true`) and, by that same invocation contract, unreachable by another skill.

## Decision drivers

- Provide one explicit entry point for the idea→work-item half without a launcher, router, or ambient hook.
- Preserve the two existing human-authorization boundaries (publish, Run start); add no third governance gate.
- Stay self-contained: depend on no external skill set; treat Matt Pocock's skills as optional prose pointers, never runtime dependencies.
- Add no new CLI verb, Action kind, machine-protocol field, Run state, or journal.

## Decision

Add `slipway-workflow` as a seventh host capability. This decision replaces #434's six-item generated-surface list with the same six plus exactly one host-side, stateless, guidance-only composite capability; it does not admit arbitrary external workflows or a global router:

1. It is user-invoked (`disable-model-invocation: true`, and Codex `allow_implicit_invocation: false`). One explicit call authorizes only the stateless first-half orchestration, analogous to how one explicit Run authorizes its bounded Action loop.
2. It is self-contained. It internalizes the interview, work-item selection, and synthesis disciplines and may optionally run Matt's already-installed, model-invocable `/grilling` skill. Missing primitives never block the workflow or trigger installation. Artifact-producing primitives (`/domain-modeling`, `/research`, `/prototype`) remain separately write-authorized and are never treated as a read-only shortcut. It never fires a user-invoked skill.
3. It never publishes and never starts a Run. It names `slipway-propose` (or an Objective plus `slipway-decompose`) and then `slipway-run` as the next explicit commands the user types.
4. It is generated and ownership-managed exactly like the other six, claiming only the files generated for its own host surface. It adds no CLI command, Action kind, machine-protocol field, Run state, or journal.

## Consequences

The generated surface, install/list/doctor counts, tests, acceptance harness, and the trilingual docs move from six to seven; skill-native hosts such as `claude` now report eight managed files (seven capabilities plus one shared reference), and a full `--tool all` install writes 135 files rather than 120. In the initial 2026-07-18 implementation, the six pre-existing capability files remained byte-compatible so that specific refresh was additive and ownership-safe. At the time this ADR was accepted, live issue #434 still recorded six capabilities in §6, §13, §16, §18 scenario 28, and its English summary. That external contract was reconciled on 2026-07-18 together with ADR-0003's lifecycle-scope refinement.

## Implementation evolution

The protocol-unchanged and byte-compatible statements above describe the initial
Workflow addition, not a permanent freeze on Slipway's other contracts. Later
direction-preserving fixes changed machine-protocol recovery and privacy inputs
and corrected wording in pre-existing capability templates. Those changes do
not alter this ADR's accepted product decision: Workflow remains the seventh
explicit, host-side navigator; it is not a router or a second runtime.

Adapter ownership remains current-generator-only and fail-closed. Bytes claimed
by an earlier generator are preserved with a warning rather than treated as
automatically replaceable historical bytes. Slipway does not maintain a
historical-digest allowlist or infer ownership from a reconstructed
six-capability fixture.

## Rejected alternatives

- A repo-owned skill outside Slipway's managed surface avoids a contract revision but is not installed, doctored, or ownership-tracked, and is therefore not "part of the framework" as users requested.
- Auto-invoking Matt Pocock's front doors (`grill-me`/`wayfinder`/`to-spec`/`to-tickets`) is impossible: they are user-invoked and unreachable by another skill.
- Letting the workflow publish would either duplicate Propose's exact-plan authority (and risk a second drifting publication implementation) or require invoking the user-only Propose capability. Letting it publish or start a Run would also collapse the two-authorization boundary the contract deliberately keeps; inventing a direct Issue format would additionally fork `change/v2`.
