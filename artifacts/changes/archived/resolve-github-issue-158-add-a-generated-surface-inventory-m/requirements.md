# Requirements

## Requirements

### Requirement: Deterministic Surface Manifest
REQ-001: The system MUST generate a deterministic public surface manifest that represents Slipway-owned generated skills, CLI command prompt surfaces, JSON/user-facing contract surfaces, and documentation rows from existing Slipway authorities rather than a duplicated hand-maintained registry.

#### Scenario: Manifest is rebuilt from current authorities
GIVEN the repository contains the current `internal/toolgen` command and skill registries, generated adapter path rules, capability surfaces, and public docs
WHEN the surface manifest builder runs
THEN it emits stable JSON rows for the current generated product surfaces with deterministic ordering and no volatile timestamp-only drift.

#### Scenario: New surface lacks inventory coverage
GIVEN a developer adds a new generated command, skill, JSON contract surface, or docs-facing surface authority
WHEN the manifest sync test rebuilds the live manifest
THEN the test fails until the committed manifest and required documentation representation are updated.

### Requirement: Check And Write Regeneration
REQ-002: The system MUST provide a regeneration entrypoint with check and write modes so maintainers can verify the committed manifest or update it without hand-editing JSON.

#### Scenario: Synced manifest passes check mode
GIVEN `docs/SURFACE-MANIFEST.json` matches the live generated-surface authorities
WHEN the regeneration entrypoint runs in check mode
THEN it exits successfully and reports that the manifest is up to date.

#### Scenario: Stale manifest fails check mode
GIVEN `docs/SURFACE-MANIFEST.json` is stale relative to the live generated-surface authorities
WHEN the regeneration entrypoint runs in check mode
THEN it exits non-zero with actionable additions or removals so the maintainer can regenerate intentionally.

#### Scenario: Write mode updates the committed manifest
GIVEN the live generated-surface authorities differ from the committed manifest
WHEN the regeneration entrypoint runs in write mode
THEN it rewrites `docs/SURFACE-MANIFEST.json` deterministically from live authorities.

### Requirement: Documentation And README Coverage
REQ-003: The system MUST keep manifest rows tied to stable documentation coverage and MUST preserve the existing README command-token/contract tests.

#### Scenario: Documentation row is missing
GIVEN a manifest row requires a stable docs or README token
WHEN the docs coverage test scans the repository documentation
THEN it fails if the token is missing from the expected docs file.

#### Scenario: Existing README checks remain active
GIVEN the generated-surface manifest checks are added
WHEN `go test ./internal/toolgen` runs
THEN the existing README command-token and command-description contract tests still execute and pass.

### Requirement: Governed Verification
REQ-004: The system MUST verify the manifest implementation with focused tests, full Go tests, and governed readiness evidence before done-ready.

#### Scenario: Focused and full verification succeed
GIVEN the implementation and manifest have been updated
WHEN the focused package tests and `go test ./...` run
THEN they pass and provide task evidence for the governed workflow.

#### Scenario: Governed readiness reaches done-ready
GIVEN code, docs, manifest, and tests satisfy the requirements
WHEN Slipway validation and review/verification gates run
THEN the governed change reaches done-ready without bypassing lifecycle gates.

### Requirement: Review Re-Certification Recovery
REQ-005: The lifecycle MUST allow a change that has completed S2 execution to advance back to S3 review after legitimate S2 inputs are refreshed, without being blocked in S2 by stale evidence from future review or verify stages.

#### Scenario: Future-stage stale evidence does not deadlock S2
GIVEN S2 execution evidence is being refreshed after S3/S4 review evidence was previously consumed
AND a later edit makes the old S3/S4 evidence stale
WHEN S2 stamps the current wave-orchestration evidence
THEN stale future-stage review or verify evidence does not block S2 from reaching the S3 review handoff where it can be re-certified.

### Requirement: Wave Evidence Bootstrap
REQ-006: The evidence command surface MUST allow the S2 wave-orchestration host to record its run-summary-bound verification evidence from the current task evidence ledger before `execution-summary.yaml` exists, while later run-summary-bound review and verification skills remain fail-closed until an execution summary exists.

#### Scenario: Wave evidence is bootstrapped from task evidence
GIVEN an active S2 change has passing task evidence for a single `run_summary_version`
AND `execution-summary.yaml` has not been generated yet
WHEN `slipway evidence skill --skill wave-orchestration --verdict pass` records the S2 host verdict
THEN the command binds the wave-orchestration record to that task evidence `run_summary_version` and stamps its digest without requiring a pre-existing execution summary.

#### Scenario: Review skill still requires execution summary
GIVEN an active review-stage change has no `execution-summary.yaml`
WHEN `slipway evidence skill --skill spec-compliance-review --verdict pass` is attempted
THEN the command fails closed with `evidence_skill_run_summary_missing` instead of inventing a run version.
