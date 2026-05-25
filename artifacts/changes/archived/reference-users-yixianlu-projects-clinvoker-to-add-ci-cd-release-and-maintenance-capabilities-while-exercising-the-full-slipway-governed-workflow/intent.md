# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go, GitHub Actions, shell
- Languages: Go, YAML, Shell
- Test Command: go test ./... -count=1
- Build Command: go build ./...
- Conventions: 

## Summary
reference /Users/yixianlu/Projects/clinvoker to add CI/CD, release, and maintenance capabilities while exercising the full Slipway governed workflow
## Complexity Assessment
complex
Rationale: this change spans repository automation, release packaging, maintenance/security workflows, and the Slipway governed workflow itself. It requires comparing an existing reference project, adding repo-facing automation, and validating both generated artifacts and lifecycle gates.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Compare `/Users/yixianlu/Projects/clinvoker` CI/CD, release, and maintenance automation against the current Slipway repository.
- Add or adapt GitHub Actions CI, release, security, package, container, docs, Nix, and maintenance workflows from clinvoker where they can be made coherent for Slipway.
- Add release metadata/configuration needed for automated changelog/release PRs, tag-based release artifact generation, SBOM/signing/provenance, package manifests, and release verification.
- Add CLI/version support required so release artifacts and package outputs can be self-verified.
- Add supporting distribution files where needed, such as GoReleaser, Docker, Nix, lint, Dependabot, docs, and release helper configuration.
- Keep a feedback record for workflow issues discovered while exercising Slipway governance.
- Verify the final implementation with repository-native Go build/test commands and Slipway lifecycle validation.

## Out of Scope
- Do not create external registry accounts, repository secrets, signing keys, package repository credentials, or GitHub repository settings.
- Do not run real publishing commands locally.
- Do not refactor Slipway lifecycle internals unless a workflow bug blocks the governed change and needs an immediate focused fix.
- Do not copy clinvoker-specific application tests or server/API checks that do not apply to a Go CLI governance tool.

## Constraints
- Prefer Slipway repository conventions and existing Go/Cobra structure over copying clinvoker literally.
- Keep workflow permissions minimal and auditable.
- Keep release automation dry-run/checkable where possible, and guard secret-dependent publishing paths behind explicit GitHub event and secret conditions.
- Record any unreasonable Slipway workflow friction as feedback instead of silently working around it.
- Preserve current CLI behavior except for explicitly scoped release/version support.
- Use `go-version-file: go.mod` for GitHub Actions rather than copying clinvoker's fixed Go version.
- Adapt clinvoker distribution breadth to Slipway names, binary paths, package metadata, and current repository files.

## Resolved Intake Research
- User selected Approach C after alternatives were presented, so the target is full clinvoker-style distribution parity rather than the earlier GitHub-Release-only recommendation.
- Include docs, Nix, container, package manager, SBOM/signing/provenance, and package verification surfaces when they can be represented as repository configuration and guarded workflows.
- Add version support before release artifact verification because Slipway currently has no release metadata surface, while clinvoker verifies `clinvk version`.
- Include maintenance/security workflows that are directly applicable to a Go CLI repository: PR title convention, Dependabot, govulncheck/SARIF, filesystem/container scanning, SBOM, release validation, and flake lock upkeep if a flake is added.
- Treat missing external credentials and repository settings as rollout prerequisites, not as reasons to shrink the selected scope.

## Acceptance Signals
- CI workflow covers `go test ./... -count=1`, `go vet ./...`, static analysis or equivalent linting, `go test ./... -race -count=1`, and `go build ./...`.
- Release automation includes release-please configuration and a tag/manual release workflow that builds cross-platform Slipway binaries, creates GitHub Release artifacts, generates SBOM/provenance/signing outputs, publishes or prepares package/container outputs through guarded steps, and verifies release artifacts.
- Maintenance automation includes directly applicable workflows adapted from clinvoker, including PR title checks, Dependabot, vulnerability/security scanning, and Nix/docs upkeep where supporting files are added.
- Distribution configuration exists for the selected package/container/Nix/docs surfaces and uses Slipway-specific metadata rather than clinvoker names.
- Local verification passes: `go test ./... -count=1` and `go build ./...`.
- Slipway lifecycle commands progress through the governed workflow and final `validate`/closeout evidence supports completion.

## Open Questions
<!-- none -->

## Deferred Ideas
- Repository owners can later decide whether to enable every configured external publishing target in GitHub repository settings and package registries.
- Additional downstream package repositories beyond the clinvoker-style set can be added after this baseline is stable.

## Approved Summary
Confirmed 2026-05-25T06:41:02Z from continuation of the active objective after the proposed scope summary: implement directly applicable CI/CD, release, and maintenance automation for Slipway by referencing `/Users/yixianlu/Projects/clinvoker`, exercise the Slipway governed workflow end to end, fix blocking workflow/debug issues when encountered, and record unreasonable workflow friction as feedback. Exclude secret-dependent external package publishing, clinvoker-specific application/runtime checks, and broad Slipway internals refactors unless they are necessary to unblock this governed workflow.

Updated 2026-05-25T06:55:17Z after the user selected `c`: proceed with Approach C, meaning full clinvoker-style distribution breadth adapted to Slipway. Secret provisioning, external account setup, and real local publishing remain out of scope, but repository configuration and guarded CI/CD paths for those distribution surfaces are in scope.
