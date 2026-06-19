# Decision

## Alternatives Considered

- Implement only `pi`: lowest implementation risk and satisfies the original
  must-have, but does not meet the user's later request to reach P1.
- Implement `pi`, `qwen`, and `kiro`: adds common skill-first adapters with
  relatively low generator risk, but still defers Copilot, Windsurf, and Kilo.
- Implement P1 in one change: `pi`, `qwen`, `kiro`, `copilot`, `windsurf`, and
  `kilo`. This matches the user's selected scope, but requires extending the
  adapter model for Copilot `.prompt.md`, Pi non-hook settings registration, and
  workflow-style roots for Windsurf/Kilo.

## Selected Approach

Implement P1 in one governed change. Keep each adapter thin and project-local:
generated prompts, workflows, skills, settings, sentinels, and ownership
manifests route agents back to `slipway` CLI commands and do not introduce
host-specific lifecycle engines.

Extend `ToolConfig` narrowly instead of adding per-host branches:

- add a command filename extension override so Copilot can generate
  `.github/prompts/slipway-*.prompt.md`;
- add a typed settings mode so Pi can merge/register `enableSkillCommands`,
  `skills`, and `prompts` without using hook-settings semantics;
- keep Qwen/Kiro command surfaces as generated command skills;
- keep Windsurf/Kilo command surfaces as flat workflow-style Markdown files;
- keep all generated files manifest-owned and sentinel-detected.

This preserves the existing Slipway pattern while absorbing the host layout
differences found in Trellis, gsd-core, and official docs.

## Interfaces and Data Flow

Changed interfaces:

- `ToolConfig` gains minimal adapter axes for command filename extension and
  settings behavior.
- `toolRegistry` gains P1 entries for `pi`, `qwen`, `kiro`, `copilot`,
  `windsurf`, and `kilo`.
- Command path generation, cleanup, refresh invalidation, contract tests, and
  docs derive from the same path helper to avoid mismatched generated paths.
- Pi settings generation writes/merges project-local `.pi/settings.json` with
  skills and prompts registration while preserving unrelated settings.

Data flow:

1. `ResolveTools` accepts explicit P1 IDs or expands `all`.
2. `GenerateWithInstallProfile` calls `generateForTool` for each selected
   adapter.
3. `generateForTool` writes generated skills, command prompts/workflows or
   command skills, optional settings, skill index, ownership manifest, and
   sentinel.
4. Refresh reads the ownership manifest, removes/replaces only trusted generated
   files, and refuses unknown or modified user-owned files.

No runtime lifecycle state, evidence semantics, or CLI command behavior changes.

## Rollout and Rollback

Rollout:

- ship the registry/model changes, tests, docs, and regenerated
  `docs/SURFACE-MANIFEST.json` together;
- verify with `go test ./internal/toolgen/...` and `go test ./...`;
- users opt in with `slipway init --tools <id>` or `slipway init --tools all`.

Rollback:

- revert the toolgen registry/model changes, tests, docs, and manifest update;
- rerun `go run ./internal/toolgen/cmd/gen-surface-manifest --write` if needed
  after rollback;
- verify with `go test ./internal/toolgen/...`.

Generated adapter files in user workspaces remain project-local and can be
removed by users or refreshed only when still proven by Slipway ownership
manifest.

## Risk

- Copilot's `.github` root is shared with CI and other project configuration.
  Mitigation: sentinel/manifest under a dedicated Copilot adapter root and
  tests proving unowned `.github` files survive refresh.
- Pi settings are not hook settings. Mitigation: typed settings mode for Pi
  registration arrays, not reuse of `mergeHookSettingsJSON`.
- Copilot `.prompt.md` differs from existing `.md`/`.toml` command extensions.
  Mitigation: explicit command extension helper covered by contract tests.
- Kiro/Qwen command skills need non-Codex invocation text. Mitigation: derive
  command triggers and invocation summaries from each tool config.
- The expanded P1 set increases docs and manifest drift risk. Mitigation:
  surface manifest check and docs-token tests remain required verification.
