# Requirements
## Project Context
- Tech Stack: Go
- Conventions: # Slipway Agent Principles

Slipway is the lifecycle authority for governed work. This file is not a
command manual, classification guide, JSON reference, or recovery cookbook. It
sets the principles an AI agent must follow when working in this repository.

## Lifecycle Authority

- Treat the current worktree's Slipway CLI as the source of truth.
- Use the Slipway behavior produced by the current worktree, not stale installed
  binaries, remembered flows, or copied recipes.
- Let Slipway decide ...
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: goal-verification delegates bulky evidence while keeping a fail-closed safety anchor
REQ-001: The `goal-verification` skill template (`internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`) MUST instruct the host to keep bulky evidence (stub/placeholder scans, SAST output, fresh test output) out of the main context — reading it in a fresh verifier context when the runtime supports one, and otherwise reducing it to a bounded summary plus a referenceable artifact. The main host MUST retain final-verdict ownership and the HARD-GATE. When a guardrail domain is set, the `high_risk_check:<domain>.safety_baseline=pass` token MUST be recorded only with a `fresh:command_ref` pointing to a real SAST output artifact; the host MUST NOT record the token from a delegated prose verdict alone, and missing, stale, or inconclusive delegated evidence MUST fail closed.

#### Scenario: delegated verifier returns pass without a real SAST artifact reference
GIVEN a change with an active guardrail domain whose goal-verification work is delegated to a verifier context
WHEN the verifier returns a `pass` verdict but provides no `fresh:command_ref` to an actual SAST output artifact
THEN the host does not record `high_risk_check:<domain>.safety_baseline=pass` and the ship gate stays blocked with `high_risk_check_missing`.

### Requirement: worktree-preflight retains only a bounded baseline summary in the main context
REQ-002: The `worktree-preflight` skill template (`internal/tmpl/templates/skills/worktree-preflight/SKILL.md`) MUST instruct the host to retain only the baseline command, its exit code, and a bounded failure summary in the main context, writing the full baseline output to a referenceable artifact, while still recording the required worktree path, branch, and exact baseline command references.

#### Scenario: baseline command succeeds during preflight
GIVEN worktree-preflight runs the baseline command in a bound worktree
WHEN the baseline passes
THEN the main host context holds the command, exit code, bounded summary, and a reference to the full output — not the full baseline transcript — and the required worktree/branch/command references are still recorded.

### Requirement: wave-orchestration coordinator stops holding the codebase map yet preserves the PR #112 staleness self-check
REQ-003: The `wave-orchestration` skill template (`internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`) MUST NOT instruct the coordinator to read the four broad codebase-map documents (`STRUCTURE.md`, `CONVENTIONS.md`, `TESTING.md`, `CONCERNS.md`) into its own context; it MUST instead pass `input_context.codebase_map_dir` and the relevant document paths to executors, and limit the coordinator to the engine-authoritative `wave_plan` metadata it already consumes. The change MUST preserve the codebase-map relevance/staleness self-check shipped in PR #112 (issue #80) by relocating it — either to per-executor refresh of the documents an executor reads, or to a coordinator decision driven by `input_context.codebase_map_doc_states` metadata rather than the document bodies — and MUST NOT delete it.

#### Scenario: wave-orchestration runs against a populated-but-possibly-stale codebase map
GIVEN `input_context.codebase_map_status` is `populated` and the relevance advisory has fired
WHEN the wave-orchestration host begins execution
THEN the coordinator does not read the four codebase-map document bodies into its context, executors receive the map directory and relevant document paths, and the codebase-map relevance/staleness self-check still occurs before stale map content is relied on for execution targeting.

### Requirement: delegation phrasing is portable across runtimes
REQ-004: All three refactored skill templates MUST express delegation using the existing portable idiom already used by `wave-orchestration` ("an isolated context ... when supported; otherwise <bounded inline fallback>") so that runtimes without exported agent directories (for example Codex) are never instructed to invoke an unavailable subagent API. The summary-first contract MUST be the portable baseline; fresh-context/subagent isolation MUST be expressed as an optional enhancement, not a precondition.

#### Scenario: a runtime without exported agent directories reads a refactored skill
GIVEN a generated skill surface for a runtime that does not support isolated subagent contexts
WHEN the host follows the delegation instruction
THEN it follows the summary-first bounded-output fallback and is not directed to call a runtime-specific subagent/`Task` API.

### Requirement: governed contracts are preserved with no new bypass
REQ-005: The change MUST preserve every existing governed contract on the three templates — the IRON LAW line, the HARD-GATE confirmation, freshness/`run_version` handling, the evidence-artifact write contract, the guardrail `safety_baseline` requirement, and the `slipway evidence task` requirement — and MUST NOT introduce any bypass, force-pass, or private-attestation path. Context reduction MUST change only where output is read, not what evidence the gates require.

#### Scenario: contract assertions run against the refactored templates
GIVEN the refactored `goal-verification`, `worktree-preflight`, and `wave-orchestration` templates
WHEN the embedded-template contract tests run
THEN the IRON LAW lines, HARD-GATE, guardrail `safety_baseline` requirement, freshness/`run_version` language, and the `slipway evidence task` requirement are all still present and no bypass/force-pass token is introduced.

### Requirement: regression tests lock in thin-host behavior
REQ-006: The repository MUST gain focused embedded-template tests that assert the thin-host / summary-first constraints for `goal-verification`, `worktree-preflight`, and `wave-orchestration`, and that fail if a future edit reintroduces inline full-output-heavy host behavior or removes the PR #112 self-check relocation.

#### Scenario: a future edit reintroduces inline full-output reading
GIVEN the regression tests are in place
WHEN a later edit makes a refactored template instruct the host to read full SAST/baseline/codebase-map output inline in the main context again
THEN at least one test fails.

### Requirement: Scope Contract ignores discovery-only codebase-map artifact dirt during S2 execution checks
REQ-007: The S2 Scope Contract workspace-diff sampling MUST exclude durable `artifacts/codebase/` discovery-context artifacts from execution changed-file drift checks, because those files are governed research/planning context rather than implementation task outputs. The exemption MUST NOT hide real implementation/test diffs, MUST NOT exempt the active task execution bundle, and MUST NOT remove `artifacts/codebase/` files from the `done` dirty-worktree advisory.

#### Scenario: codebase map changes exist alongside implementation diffs
GIVEN a governed change has modified `artifacts/codebase/STRUCTURE.md` during discovery and also has a real implementation diff
WHEN Scope Contract evaluates S2 execution changed-file coverage
THEN the codebase-map artifact is ignored for S2 drift, while the real implementation diff still requires task `target_files`/`changed_files` coverage and `done` still reports the codebase-map artifact as dirty.
