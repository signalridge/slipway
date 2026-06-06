# Slipway Agent Principles

Slipway is the lifecycle authority for governed work. This file is a
principle-only companion to `CLAUDE.md`; it exists so agent entry surfaces do not
depend on a missing or injected instruction file.

## Black-Box Lifecycle

- Use the current worktree's Slipway behavior as the authority.
- Do not substitute remembered workflows, source-derived guesses, or installed
  binaries for current worktree lifecycle output.
- If a public surface does not make the next action clear, fix the product
  surface immediately.

## Self-Optimization

- Treat repeated friction as a system defect, not as operator training material.
- Improve the kernel, generated skills, docs, evidence contracts, or recovery
  output so the next agent does not need private knowledge.
- Do not preserve retired behavior for compatibility when the governed objective
  intentionally removes it.

## Evidence And Review

- Completion requires fresh current-worktree evidence and governed readiness.
- Do not forge, restamp, or hand-edit engine-owned freshness state.
- Sensitive-domain work must fail closed to rerun, review, and explicit
  evidence, with no bypass or force-close path.

## Instruction Boundary

- Keep agent instruction files principle-only.
- Put command syntax, field lists, examples, and generated references in the
  surfaces that own them.
- If an agent has to guess, the workflow has already failed its usability
  contract.
