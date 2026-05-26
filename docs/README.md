# Docs Map

This directory separates stable product contracts from historical planning notes.

## Stable Contracts

- `command-contract-matrix.md`: authoritative command inventory, command tiers, and source-of-truth surfaces
- `execution-surface-boundary.md`: boundary between `next`, `run`, `abort`, and `cancel`
- `agent-contracts.md`: internal governance agent mapping and override rules

## Decisions

- `adr-retire-sync-as-product-verb.md`: historical decision record for earlier sync-era command cleanup
- `worktree-orchestrator-deferment.md`: explicit deferment of first-class worktree / orchestrator promotion

## Testing / Operation

- `workflow-test-menu.md`: executable end-to-end testing paths for the Slipway workflow

## Historical Plans

- `plans/`: time-scoped implementation plans retained as historical context rather than current runtime contract
- `plans/2026-05-24-governance-kernel-runtime-framework.md`: boundary hardening plan for current command contracts and targeted change-authority write helpers
- `plans/skill-primary-migration.md`: migrate Claude Code from agent-primary to skill-primary dispatch architecture
