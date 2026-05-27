---
skill_id: coding-discipline
name: slipway-coding-discipline
description: "Use when planning, implementing, reviewing, or refactoring code and you need a compact four-principle discipline that keeps changes scoped, simple, and goal-driven. Triggers on code-writing or code-review work."
---

# Coding Discipline

```
IRON LAW: THINK FIRST, CHANGE LESS, STAY GOAL-DRIVEN
```

Violating the letter of this rule is violating the spirit of this rule.

## Purpose
Apply a compact four-principle discipline for code work without introducing a
new routed/public surface. This is a reusable technique skill: it supplies the
posture for planning, implementation, and review hosts while those hosts keep
their own detailed procedures and evidence contracts.

## Design Stance
This skill keeps Slipway implementation work scoped and evidence-driven. It
compresses coding discipline into four guardrails while leaving planning, TDD,
verification, and review mechanics to the owning governed skills. It is not an
additional routed workflow or independent methodology.

## Think Before Coding
- state assumptions and open questions explicitly
- do not silently choose one interpretation when multiple are plausible
- push back when a simpler approach exists
- stop and resolve confusion before writing code that locks in the wrong shape

## Simplicity First
- write the minimum code that solves the actual problem
- do not add speculative abstraction, configuration, or future-proofing
- do not smuggle in extra features because you are already touching the area
- when the current design is obviously bloated, say so directly and simplify it

## Surgical Changes
- touch only the files and logic required for the task
- avoid drive-by refactors, formatting churn, or opportunistic rewrites
- remove only the orphans created by the current change
- call out unrelated cleanup separately instead of folding it into the diff

## Goal-Driven Execution
Keep the goal visible, then delegate detailed mechanics to the owning Slipway
skills:
- planning and task decomposition -> `slipway-plan-audit`
- guarded implementation and test-first execution -> `slipway-tdd-governance`
- completion and freshness proof -> `slipway-goal-verification`
- explicit review verdict and handoff -> `slipway-independent-review`

If a host already owns a detailed procedure, follow that host for mechanics and
use `slipway-coding-discipline` for posture.

## Tradeoff
These guidelines bias toward caution, smaller diffs, and explicit reasoning.
When the correct solution really is architectural or cross-cutting, state that
openly instead of hiding it inside a sequence of local patches.

## Checklist
- [ ] Assumptions and constraints are explicit before coding starts.
- [ ] The chosen solution is the simplest thing that satisfies the goal.
- [ ] The diff stays scoped to the task instead of broadening opportunistically.
- [ ] Detailed execution/review mechanics are delegated to the owning Slipway skills.
