# Assurance
## Project Context
- Tech Stack: Go CLI, Markdown documentation, MkDocs Material
- Conventions: keep docs aligned with release configuration and generated adapter contracts
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go, Markdown, YAML

## Scope Summary
Delivered a tracked Slipway documentation system under `docs/`, redesigned the README as a product and design entrypoint, added a Design Philosophy page, documented all configured platform/package install surfaces, added Mermaid lifecycle/architecture/adapter diagrams, added AI-tool installation guidance with explicit OpenCode paths, added the docs workflow, ignored MkDocs `site/` output, and fixed the recurring `Open Questions` intake blocker with shared semantics and regression tests.

## Verification Verdict
Execution, review, goal-verification, and final closeout evidence pass for the corrected docs scope.

## Evidence Index
- `mkdocs build --strict`: pass after adding `docs/design.md`, Mermaid fences, and updated nav.
- README contract test: `go test ./internal/toolgen -run TestReadmeAndCommandDescriptionsReflectCurrentEntrySurface -count=1`: pass.
- Targeted Open Questions regression: `go test ./internal/stringutil ./internal/engine/progression ./internal/engine/governance -run 'TestHasBlockingOpenQuestions|TestAdvanceIntake_OpenQuestionsUseResolvedItemSemantics|TestTraceability.*OpenQuestions' -count=1`: pass.
- Full Go suite: `go test -timeout=20m ./... -count=1`: pass after the review-correction docs expansion.
- Build: `go build ./...`: pass.
- Vet: `go vet ./...`: pass.
- Whitespace check: `git diff --check`: pass.
- Governance validation: `go run . validate --json`: pass with `G_ship=approved`, fresh evidence, and no blockers after goal verification.
- Final closeout refresh: corrected OpenCode/artifact evidence, `mkdocs build --strict`, corrected targeted Open Questions regression, `go test -timeout=20m ./... -count=1`, `go build ./...`, `go vet ./...`, and `git diff --check`: pass at 2026-05-26T16:50:07Z.
- Local availability note: `yamllint` and `markdownlint-cli2` are not installed in this shell. CI still owns those lint checks.

## Requirement Coverage
- REQ-001: `mkdocs.yml` now points to tracked `docs/index.md`, `design.md`, `installation.md`, `workflow.md`, `commands.md`, `ai-tools.md`, `operator-guide.md`, and `contributing.md`; `mkdocs build --strict` passes.
- REQ-002: `research.md` records concrete borrowed patterns from spec-kitty, OpenSpec, Spec Kit, Superpowers, GSD, and OpenCode; `docs/design.md` maps those patterns to Slipway-specific behavior.
- REQ-003: `README.md` and `docs/` cover what Slipway is, design philosophy, install/init, governed workflow, command reference, AI-tool adapters, operator guide, and contributor guidance.
- REQ-004: `docs/installation.md` includes a copy-paste AI-tool prompt; `docs/ai-tools.md` names OpenCode `.opencode/skills`, `.opencode/commands/slipway-*.md`, hook path, and `/slipway-*` invocation style.
- REQ-005: `stringutil.HasBlockingOpenQuestions` is used by intake and traceability; tests cover `(none)`, checked items, unchecked items, plain bullets, and canonical last-section behavior.
- REQ-006: `docs/installation.md` has an explicit release-first install order, macOS/Linux/Windows amd64/arm64 matrix, direct release archives, Go install fallback, source build, Nix, container, checksums, Homebrew, Scoop, AUR, `.deb`, `.rpm`, and `.apk` guidance with optional-channel caveats.
- REQ-007: `README.md` now presents product value, design philosophy, core capabilities, Mermaid lifecycle, install summary, quick workflow, AI-tool adapters, runtime files, and docs map while linking detailed platform commands to `docs/installation.md`.

## Residual Risks and Exceptions
- Local markdown/yaml lint executables are unavailable in this shell; CI remains configured to run Markdown and YAML lint.
- The docs workflow installs `mkdocs-material` from PyPI in CI; dependency pinning can be tightened later if release reproducibility requires it.
- Package-manager install docs are intentionally conservative: they describe configured release channels and fallback paths without claiming every optional channel is published for every release.
- OpenCode command presentation can vary by OpenCode build; docs name the generated file path as the stable Slipway contract and mention possible project-prefix display.

## Rollback Readiness
Rollback is file-level: revert README/docs/MkDocs/docs-workflow changes and the narrow Open Questions helper/caller/test changes. No migration or external service state change is required.

## Archive Decision
Ready to finalize with `slipway done`. The corrected docs scope has passing execution, review, goal-verification, final closeout, fresh governance validation, and no remaining blockers.
