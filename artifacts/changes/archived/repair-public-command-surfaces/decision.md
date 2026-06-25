# Decision

## Alternatives Considered

### A. Registry-backed CLI-only config surface
Register `config` in `commandRegistry` with `HasPromptSurface: false`, mirror it
in root help through `desc("config")`, include its JSON contract in the
manifest, and document it as CLI-only setup/config surface. Remove
`review --artifact` rather than preserving an unsupported public flag.

Tradeoffs: fixes the manifest/docs blind spot with the smallest behavior change,
but keeps `config` out of generated host command wrappers.

### B. Generated prompt surface for config
Register `config` with `HasPromptSurface: true` so adapter command skills are
generated.

Tradeoffs: more discoverable in host tools, but expands repo configuration
mutation into every generated adapter surface and increases the contract blast
radius.

### C. Hand-maintained manifest/docs rows
Keep `config` outside `commandRegistry` and add separate manifest/doc rows.

Tradeoffs: least code churn, but preserves the exact two-authority problem that
let root help and manifest drift.

## Selected Approach
Use Approach A. The user selected it after reviewing the research alternatives.
It follows the existing `tool` CLI-only registry precedent, makes public command
inventory machine-checkable, removes a dead review flag, and avoids expanding
adapter prompt surfaces for repo config mutation.

## Interfaces and Data Flow
- `cmd/root.go` uses registry descriptions for `config` instead of a local
  config-only description constant.
- `internal/toolgen/toolgen.go` adds `config` to the command registry with
  `HasPromptSurface: false`; manifest command and JSON rows derive from the
  same registry path as other public commands.
- `commandIDs()` remains prompt-surface-only, so generated command skills do not
  include `config`.
- `cmd/review.go` removes the `artifact` field, flag registration, and
  unsupported-flag branch.
- Documentation is updated from the registry/manifest semantics: command index,
  setup tables, stable JSON token tables, adapter inventory prose, README
  handoff examples, and SVG alt text.

## Rollout and Rollback
Rollout is a normal source/docs/test change in the governed worktree. Regenerate
`docs/SURFACE-MANIFEST.json` with:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
```

Verification includes targeted command/toolgen tests, manifest check, full Go
tests as feasible, and `git diff --check`.

Rollback is a git revert of the source/docs/test diff, followed by regenerating
the manifest and rerunning the same checks.

## Risk
- Public-contract risk: changing `review --help` removes an unsupported flag.
  Risk is bounded because the flag never performed useful work and returned
  `unsupported_flag`.
- Adapter risk: adding `config` to the registry could accidentally generate host
  command wrappers if `HasPromptSurface` is wrong. Tests must assert it remains
  absent from `commandIDs()`.
- Documentation drift risk: localized command pages can drift because the
  manifest only anchors `docs/reference/commands.md`. Add docs-token checks for
  detailed EN/JA/ZH token tables where practical.
- Scope risk: `docs/SURFACE-MANIFEST.json` must be regenerated, not hand-edited.
