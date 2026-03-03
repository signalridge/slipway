# Reference Bundle (Temporary)

This folder is a local reference copy used while drafting the MVP OpenSpec redesign change.

## Included

- `skills/superpowers/`
  - Full Superpowers skill set (copied from `~/.agents/skills/superpowers`)
  - Used for helper-skill trigger and guidance contracts
- `gsd/gsd/`
  - Full GSD command set (copied from `get-shit-done/commands/gsd`)
- `gsd/workflows/`
  - Key orchestration flows (`discuss-phase`, `plan-phase`, `execute-phase`, `verify-work`, `quick`, `discovery-phase`, `progress`, `transition`)
- `gsd/templates/`
  - Key context/review/subagent templates (`phase-prompt`, `verification-report`, `debug-subagent-prompt`, `context`, `discovery`, `state`, `project`, `summary`, `VALIDATION`, `continue-here`)
- `openspec/.claude/commands/opsx/`
  - OpenSpec command prompts (`explore`, `new`, `continue`, `apply`, `verify`, `archive`, `ff`, etc.)
- `openspec/.claude/skills/`
  - OpenSpec skill contracts (`openspec-new-change`, `openspec-continue-change`, `openspec-apply-change`, `openspec-verify-change`, etc.)

## Purpose

- Ground helper trigger design in three concrete ecosystems (Superpowers + GSD + OpenSpec)
- Reuse real command/skill contracts instead of invented trigger lists
- Compare wave/subagent/review interactions before finalizing workflow contracts

All files here are references only and not runtime dependencies.
