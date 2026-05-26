# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go, GitHub Actions, Nix
- Languages: Go, YAML, Shell, Nix
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
repair fixable remote CI failures by tightening lint scopes, cross-platform behavior, Nix hash, and security workflow configuration while removing GitHub Pages automation
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
external_api_contracts

## In Scope
- Tighten CI lint scope so YAML and Markdown linting target maintained source/config surfaces instead of archived governance evidence or generated host templates.
- Fix cross-platform test failures that are repairable in code or checked-in scripts, including Windows directory sync behavior and macOS Bash compatibility.
- Update Nix package metadata required by the current Go dependency graph.
- Adjust security workflow configuration that can be fixed in checked-in YAML, including SARIF upload permissions and govulncheck SARIF validity handling.
- Remove GitHub Pages workflow automation for now.

## Out of Scope
- Do not repair repository or organization settings such as enabling GitHub Pages.
- Do not add, rotate, or depend on missing tokens/secrets such as `GH_PAT`.
- Do not retry remote CI without a classified root cause.
- Do not make unrelated release or publishing behavior changes beyond removing Pages and fixing checked-in workflow defects.

## Constraints
- Keep changes scoped to the CI failures already classified from remote logs.
- Preserve existing Slipway runtime behavior unless a failing test proves a portability defect.
- Use the Slipway governed worktree as the authority for implementation.

## Acceptance Signals
- Local targeted tests cover changed Go portability and generated script behavior.
- `go test ./... -count=1` and `go build ./...` pass locally.
- Lint commands that previously failed on broad scope are either locally reproduced with the intended scope or covered by equivalent config inspection.
- Nix vendor hash is updated to match the current `go.mod` / `go.sum`.
- GitHub Pages workflow files are removed from the checked-in workflow set.

## Open Questions
<!-- None. The user confirmed Pages and token/secret gaps are intentionally out of scope. -->

## Deferred Ideas
- Re-enable GitHub Pages only after repository Pages settings are deliberately configured.
- Configure `GH_PAT` or repository Actions settings for Release Please PR creation in a separate operational change.

## Approved Summary
The user confirmed on 2026-05-26 that this change should follow the Slipway workflow and repair all fixable remote CI failures in code/config. In scope: lint scope, cross-platform test defects, Nix hash, and security workflow issues that do not require new secrets. GitHub Pages automation should be removed for now. Missing token/secret or repository-setting failures are explicitly out of scope. Completion requires local targeted verification plus broad Go build/test proof where feasible.
