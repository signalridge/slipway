# Decision

## Alternatives Considered

1. Direct binary commands in generated settings.
   This removes script files entirely from the automatic hook path, but it lets
   host AI tools surface command-not-found errors directly when `slipway` is not
   installed or not on `PATH`. Automatic hooks should not interrupt the host
   session for a missing advisory helper.

2. Compiled behavior plus platform-native thin launchers. Selected.
   The Slipway binary owns hook and helper behavior. Generated launchers are
   small platform adapters that find and invoke the binary and fail-silent for
   automatic hooks. Windows can use PowerShell/CMD launchers, POSIX can use
   POSIX shell launchers, and neither side duplicates business logic.

3. Keep scripts and add portability checks.
   Health warnings would make failures easier to diagnose but would not solve
   the generated-script dependency problem. Bash, Python, and jq would still be
   part of the supported helper runtime contract.

4. Force every GitHub helper through the Go standard-library API.
   This removes `gh`, but it also discards a useful native GitHub authentication
   surface that many operators already configure. It also makes GitHub
   Enterprise and host-specific authentication behavior less natural. Rejected
   after rescope.

5. Use a hybrid GitHub backend. Selected for GitHub helpers.
   Keep generated helper scripts retired, but let `slipway tool` choose the
   best authenticated GitHub backend: prefer `gh`, use token-backed API when
   `gh` is unavailable or reports an auth-required error, and fail closed when
   neither backend exists.

## Selected Approach

Implement Option 2 for hooks and generated helper packaging, plus Option 5 for
manual GitHub helper execution.

Automatic hook behavior moves into compiled commands:

- `slipway hook session-start --tool <tool>`
- `slipway hook context-pressure`

Generated settings invoke platform-native launchers rather than `bash
"<hook>.sh"`. The launcher templates contain only dispatch/no-op behavior. The
session-start lifecycle logic currently inside the shell template moves into Go.

Supported generated skill helpers move to a visible `slipway tool` command
family:

- `merge-sarif`
- `pin-actions`
- `find-polluter-go`
- `find-variant`
- `fetch-pr-checks`
- `fetch-pr-feedback`
- `fetch-review-requests`
- `reply-to-thread`

Generated skill instructions reference these commands. Generated executable
script payloads for Slipway-owned helper behavior are removed.

GitHub helper commands expose `--backend auto|gh|api`:

- `auto` prefers authenticated `gh`.
- `auto` falls back to token-backed API only when `gh` is unavailable or reports
  an auth-required error, and `GH_TOKEN` or `GITHUB_TOKEN` is present.
- `gh` requires the GitHub CLI and does not silently fall back.
- `api` requires `GH_TOKEN` or `GITHUB_TOKEN` and never attempts
  unauthenticated fetches.
- no helper installs `gh` or any other dependency implicitly.

## Interfaces and Data Flow

Hook data flow:

1. `toolgen` renders native launcher files for hook-capable adapters.
2. `toolgen` merges settings using the rendered launcher command for the current
   platform and removes legacy Slipway-owned `bash "<hook>.sh"` entries.
3. The launcher invokes `slipway hook session-start --tool <tool>` or
   `slipway hook context-pressure`.
4. The Go command performs lifecycle/context work and emits host hook output.

Skill helper data flow:

1. Generated skill docs name `slipway tool <helper>` commands.
2. The `tool` command validates input and runs the helper through the selected
   backend.
3. GitHub helpers prefer `gh api` / `gh api graphql`, invoked directly without
   shell interpolation; token-backed standard-library HTTP is the fallback
   backend when `gh` is unavailable or reports an auth-required error.
4. Local helpers use filesystem and JSON/YAML-like processing in Go. The
   `find-polluter-go` helper may invoke `go test` because the workflow itself is
   explicitly Go-test based; the helper runtime still does not depend on shell.

Generated support-file flow:

1. `toolgen` continues copying `references/` support files.
2. Slipway-owned `scripts/` payloads are no longer embedded or emitted.
3. Refresh mode removes stale generated `scripts/` directories.

## Rollout and Rollback

Rollout is source-template driven:

1. Add the compiled commands and tests.
2. Change toolgen hook settings/launcher generation.
3. Update skill templates and remove script payloads.
4. Run focused package tests, full `go test -count=1 ./...`, and Slipway
   validation.

Rollback is a normal git revert of the source changes in `cmd/`,
`internal/toolgen/`, `internal/tmpl/`, and docs. Verification for rollback is
`go test -count=1 ./...` plus `go run . validate --json` from the reverted
worktree.

## Risk

- Launcher quoting differs across platforms. Keep launcher arguments fixed and
  test template contents for dispatch-only behavior.
- Hook settings merge can leave stale legacy commands. Remove Slipway-owned
  legacy `bash "<hook>.sh"` entries while preserving unrelated user hooks.
- Helper rewrites can change output shape. Port current fixture expectations
  into command tests before removing scripts.
- GitHub helpers can accidentally hide authentication failures if backend
  fallback is too broad. Prefer `gh` only when present and authenticated, fall
  back to API only when `gh` is unavailable or reports an auth-required error,
  and fail closed rather than making unauthenticated calls.
- Stale generated skill scripts can remain after refresh. Refresh must prune
  `scripts/` directories and tests must reject emitted helper scripts.
