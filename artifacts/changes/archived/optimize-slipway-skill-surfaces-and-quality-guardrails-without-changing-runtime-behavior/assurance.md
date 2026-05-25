# Assurance

## Project Context
- Tech Stack: Go CLI, generated Agent Skills templates
- Conventions: Go-owned capability metadata, deterministic toolgen output, generated surfaces tested through `internal/toolgen`.
- Test Command: `go test ./internal/toolgen ./cmd ./internal/engine/capability`
- Build Command: `go build ./...`
- Languages: Go, Markdown, Shell, Python

## Scope Summary
Delivered scope is limited to generated lookup surfaces, quality guardrail
tests, and concise reference navigation:
- public `--focus` aliases now render from `capability.surfacePolicy` into the generated workflow command reference;
- the workflow-owned skill index now includes a compact public focus alias table that does not expose non-exported host paths;
- the workflow skill clarifies that focus aliases are command selectors, not host-skill paths;
- very long references now include `## Quick Navigation`;
- tests guard public-focus rendering and long-reference navigation.

Runtime lifecycle behavior, public command semantics, JSON shapes, gates,
state files, and exported host skill allowlists were not changed.

## Verification Verdict
Pass.

Commands run:
- `go test ./internal/engine/capability ./internal/toolgen ./cmd -count=1`
- `go test ./internal/engine/capability ./internal/toolgen -coverprofile=/tmp/slipway-skill-surfaces-coverage.out -count=1`
- `go test ./... -count=1`
- `go build ./...`

## Evidence Index
- `verification/intake-clarification.yaml`
- `verification/research-orchestration.yaml`
- `verification/plan-audit.yaml`
- `verification/wave-orchestration.yaml`

## Requirement Coverage
- REQ-001: satisfied by `internal/engine/capability/export.go`, `internal/toolgen/toolgen.go`, and `internal/tmpl/templates/skills/workflow/command-reference.md.tmpl`; verified by capability and toolgen tests.
- REQ-002: satisfied by `internal/engine/capability/export_test.go` and `internal/toolgen/toolgen_test.go`; verified by focused and full Go tests. The command-reference guard now iterates the full explicit-focus surface list, and the long-reference guard requires quick navigation near the top of the file.
- REQ-003: satisfied by workflow prose and quick navigation additions in the three long reference files; verified by `TestLongReferenceFilesHaveQuickNavigation`.
- REQ-004: satisfied by preserving existing CLI/runtime code paths and existing export/no-catalog tests; verified by focused tests, `go test ./...`, and `go build ./...`.

## Residual Risks and Exceptions
- No accepted runtime behavior exceptions.
- Residual risk is limited to prose usefulness: the new generated lookup text improves discoverability but does not attempt third-party skill curation.
- Broad marketplace/catalog ideas remain deferred by design.

## Rollback Readiness
Rollback is source-only: revert renderer, template, reference, test, and artifact
edits. No migration, archive repair, external dependency, or generated persisted
state cleanup is required.

## Archive Decision
Ready for review and final closeout after Slipway governance review evidence is recorded.
