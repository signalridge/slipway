# Slipway Agent Principles

Slipway is the lifecycle authority for governed work. This file is not a
command manual, classification guide, JSON reference, or recovery cookbook. It
sets the principles an AI agent must follow when working in this repository.

## Lifecycle Authority

- Treat the current worktree's Slipway CLI as the source of truth.
- Use the Slipway behavior produced by the current worktree, not stale installed
  binaries, remembered flows, or copied recipes.
- Let Slipway decide lifecycle state, readiness, recovery, and next governed
  handoff. Do not infer those from source code, artifact guesses, or old chat
  context.
- If a public surface is ambiguous, insufficient, or requires command guessing,
  treat that as a product defect to fix.

## Self-Optimization Loop

- When the workflow exposes friction, improve the system so the next agent does
  not hit the same friction.
- Prefer fixing the kernel, generated skills, command surfaces, docs, or
  evidence contracts over teaching a one-off workaround.
- A recovery that only works because the agent knows private sequencing is not a
  valid recovery; make the public surface carry the next action.
- A validation failure that is technically correct but operationally unclear is
  still a product problem.
- Do not normalize manual digest edits, timestamp edits, source-inspection
  recovery, or hidden host knowledge. Remove the need for them.

## Evidence Discipline

- No completion claim is valid without fresh evidence from the current worktree.
- Passing tests are useful evidence, but they do not replace governed readiness.
- Governed evidence must be produced by the owning stage and accepted by the
  lifecycle. Do not forge, restamp, or hand-edit engine-owned freshness state.
- If evidence becomes stale, re-enter the owning lifecycle stage through the
  public Slipway flow.
- If a stale state cannot be resolved through that public flow, fix Slipway
  before claiming progress.

## Change Discipline

- Keep changes scoped to the current governed objective.
- Remove obsolete surfaces instead of preserving compatibility for behavior that
  the objective intentionally retires.
- Keep code, generated skills, docs, and agent instructions aligned as one
  product surface.
- Preserve unrelated local work.
- Prefer the smallest clean design that makes the requested end state true.

## Review And Safety

- Sensitive-domain work must fail closed to rerun, review, and explicit
  evidence. It must not gain bypass, force-close, or private attestation paths.
- Review public CLI, JSON, generated skills, and docs as external contracts when
  behavior changes.
- A clean implementation is not enough if the governed bundle, recovery
  guidance, or final closeout remains unclear.

## Agent Instruction Boundary

- Keep this file principle-only.
- Put detailed command syntax, field lists, examples, and generated references
  in the surfaces that own them.
- If this file starts to explain a specific procedure, move that procedure into
  the appropriate CLI/help/generated-skill surface instead.
