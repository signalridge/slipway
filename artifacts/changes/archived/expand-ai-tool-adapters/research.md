# Research

## Alternatives Considered

### Architecture
- Affected modules: `internal/toolgen/toolgen.go`, `internal/toolgen/install_profiles.go`, `internal/toolgen/ownership_manifest.go`, `internal/toolgen/surface_manifest.go`, `internal/toolgen/*_test.go`, `docs/ai-tools.md`, `docs/reference/ai-tools.md`, and `docs/SURFACE-MANIFEST.json`.
- Dependency chains: `slipway init --tools` -> `ResolveTools` -> `GenerateWithInstallProfile` -> `generateForTool` -> host skill/command/settings emission -> ownership manifest/sentinel. Evidence: `internal/toolgen/toolgen.go:711-719`, `internal/toolgen/toolgen.go:763-786`, `internal/toolgen/toolgen.go:834-1137`.
- Blast radius: add adapter registry entries and small generator axes for command filename extension, static/non-hook settings registration, and possibly non-Codex command skills. Keep lifecycle and command semantics in the CLI. Evidence: `docs/reference/ai-tools.md:7-9`, `artifacts/changes/expand-ai-tool-adapters/intent.md`.
- Constraints: generated files must be deterministic, sorted, and manifest-owned; refresh must preserve user-owned files. Evidence: `internal/toolgen/toolgen.go:652-668`, `internal/toolgen/toolgen.go:789-800`, `internal/toolgen/ownership_manifest.go:193-214`, `internal/toolgen/ownership_manifest.go:394-414`.

### Patterns
- Existing Slipway pattern: one declarative `ToolConfig` per host defines skills, commands, settings, hooks, trigger style, and auto-detect roots. Generated host files invoke `slipway ...` rather than reimplementing governance. Evidence: `internal/toolgen/toolgen.go:23-43`, `internal/toolgen/toolgen.go:45-124`.
- Existing Slipway pattern: `Registry()` is sorted, but `ResolveTools("all")` is currently hardcoded to the five existing IDs; adding tools should either derive from registry or update the pinned list and tests. Evidence: `internal/toolgen/toolgen.go:652-668`, `internal/toolgen/toolgen.go:711-719`, `internal/toolgen/toolgen_test.go:61-78`.
- Trellis pattern: common adapter universe includes `kiro`, `copilot`, `pi`, `kilo`, `windsurf`, plus other current hosts. It models Kiro as skill-centric, Copilot as prompts plus skills under `.github`, Pi as `.pi/prompts`, `.pi/skills`, agents, extension, and settings, Kilo as `.kilocode/workflows` plus `.kilocode/skills`, and Windsurf as `.windsurf/workflows` plus `.windsurf/skills`. Evidence: `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/types/ai-tools.ts:10-25`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/types/ai-tools.ts:203-235`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/types/ai-tools.ts:270-285`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/types/ai-tools.ts:319-340`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/types/ai-tools.ts:357-372`.
- Trellis Pi detail: Pi prompts are `.pi/prompts/trellis-<name>.md`, skills are under `.pi/skills`, settings register skills/prompts/extensions, and Pi does not use Python hooks. Evidence: `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/configurators/pi.ts:21-55`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/templates/pi/settings.json:1-12`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/multi-platform.mdx:210-226`.
- Trellis command detail: Pi command prompts use `/trellis-<name>`, Copilot prompt files use `.prompt.md`, and Kiro can be skill-only with `@trellis:<name>`. Evidence: `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx:11-28`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx:38-43`.
- GSD pattern: runtime adapters are descriptor-axis driven (`installSurface`, `writesSharedSettings`, `hooksSurface`, `sandboxTier`, artifact layout) and derive supported runtimes from capability descriptors rather than a separate hand list. Evidence: `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/runtime-config-adapter-registry.cts:13-28`, `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/runtime-config-adapter-registry.cts:65-92`, `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/runtime-config-adapter-registry.cts:124-144`.
- GSD Qwen detail: Qwen has `.qwen` config home, `settings-json`, skills under `skills`, slash-hyphen command style, settings-json hook surface, and Claude hook event dialect. Evidence: `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/qwen/capability.json:1-47`.
- GSD Copilot detail: Copilot is modeled as markdown config, skills under `skills`, slash-hyphen command style, `copilot-instructions` install surface, and no shared settings write. Evidence: `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/copilot/capability.json:1-46`.
- GSD Kilo/Windsurf detail: Kilo is modeled as flat commands plus recursive skills under Kilo config homes; Windsurf is modeled as skills-only with no hook surface. Evidence: `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/kilo/capability.json:1-67`, `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/windsurf/capability.json:1-47`.
- Official freshness check: Pi currently advertises extensibility through skills and prompt templates under `.pi`, Kiro documents open Agent Skills with progressive disclosure, Copilot/VS Code documents `.github/skills` and `.github/prompts/*.prompt.md`, Qwen documents project `.qwen/settings.json` plus `.qwen/skills`, Windsurf documents workflows, skills, hooks, and AGENTS.md, and Kilo documents Agent Skills. Sources: https://pi.dev/, https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md, https://kiro.dev/docs/skills/, https://code.visualstudio.com/docs/agent-customization/agent-skills, https://code.visualstudio.com/docs/copilot/customization/prompt-files, https://qwenlm.github.io/qwen-code-docs/en/users/configuration/settings/, https://docs.windsurf.com/plugins/cascade/workflows, https://docs.windsurf.com/windsurf/cascade/skills, https://kilo.ai/docs/customize/skills.

### Risks
- Medium: Pi settings are not hook settings. Current `SettingsPath` merges a `hooks` object and should not be reused for Pi's `skills`/`prompts` arrays without a typed setting kind. Evidence: `internal/toolgen/toolgen.go:1078-1098`, `internal/toolgen/toolgen.go:1994-2044`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/templates/pi/settings.json:1-12`.
- Medium: Copilot shares `.github` with existing repository configuration. Generated ownership must be scoped to Slipway-prefixed prompts/skills and a dedicated sentinel/manifest path, not a whole-tree cleanup. Evidence: `internal/toolgen/ownership_manifest.go:193-214`, `internal/toolgen/ownership_manifest.go:255-268`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/multi-platform.mdx:182-196`.
- Medium: Copilot prompt files require `.prompt.md`; current command extension handling only supports `.md` and `.toml`. Evidence: `internal/toolgen/toolgen.go:1030-1048`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx:15-27`.
- Low/medium: Kiro and Qwen can likely use the Agent Skills shape, but their manual invocation text differs from Codex. Current command-skill summary hardcodes Codex wording. Evidence: `internal/toolgen/toolgen.go:2304-2315`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx:38-43`.
- Low/medium: Kilo and Windsurf have workflow/skill surfaces rather than the exact existing Claude/Cursor command layout. Their command roots need explicit command style/path coverage, not inference from current nested/flat command assumptions. Evidence: `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx:30-36`, `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/kilo/capability.json:20-58`, `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/windsurf/capability.json:15-39`.
- Low: `--tools all` and tests currently pin five IDs. This is intentional contract coverage, but it means every selected tool must update tests/docs/manifest together. Evidence: `internal/toolgen/toolgen_test.go:61-78`, `internal/toolgen/adapter_contract_test.go:77-83`, `docs/SURFACE-MANIFEST.json:4-38`.
- Guardrail domains: no auth/authz, credentials, PII, financial, schema migration, irreversible operation, or external API contract changes are introduced by the planned thin adapter generation. The change writes project files, so generated ownership remains the main safety concern.
- Reversibility: additive registry/docs/test changes are straightforward to revert; generated refresh behavior must be proven because it affects user workspaces.

### Test Strategy
- Existing coverage: registry count/order, `ResolveTools`, generated files, command surface sync, settings/hook behavior, refresh ownership, adapter contract freeze, and surface manifest freshness. Evidence: `internal/toolgen/toolgen_test.go:61-78`, `internal/toolgen/toolgen_test.go:403-575`, `internal/toolgen/toolgen_test.go:943-983`, `internal/toolgen/adapter_contract_test.go:38-176`, `internal/toolgen/surface_manifest_test.go:15-160`.
- New coverage required for Pi: `LookupTool("pi")`, `ResolveTools("pi")`, `ResolveTools("all")`, `.pi/prompts/slipway-*.md`, `.pi/skills/slipway-*/SKILL.md`, `.pi/settings.json` with `enableSkillCommands`, `skills`, and `prompts`, sentinel/manifest under `.pi/slipway`, refresh refusal for modified generated prompts/skills, and no Python hook launcher emission.
- New coverage required for selected P1 tools: exact generated skill/command/workflow paths, trigger strings, `--tools all` inclusion, auto-detection by sentinel only, refresh preservation beside shared roots, and docs/manifest tokens. Copilot must additionally prove `.github` preservation and `.prompt.md` prompt filenames.
- Infrastructure needs: small helper functions for command extension/name derivation and adapter setting registration so tests do not duplicate path logic.
- Verification approach: run `go test ./internal/toolgen/...`, regenerate/check `docs/SURFACE-MANIFEST.json`, then run `go test ./...`.

### Options
- Option A: implement only `pi` now. Tradeoffs: lowest risk and satisfies the must-have, but leaves the user's "common tools" investigation mostly as documentation and delays Qwen/Kiro/Copilot adoption.
- Option B: implement `pi`, `qwen`, and `kiro`; defer `copilot`. Tradeoffs: covers the required Pi plus two low-friction common adapters. Pi gets prompt files plus static settings; Qwen and Kiro use Agent Skills/command-skill surfaces with host-specific trigger text. Copilot is deferred because `.github` sharing and `.prompt.md` need a filename axis and stricter ownership tests.
- Option C: implement `pi`, `qwen`, `kiro`, and `copilot`. Tradeoffs: broadest common coverage and matches Trellis' current platform set better, but higher blast radius because Copilot needs `.github/prompts/*.prompt.md`, `.github/skills`, and shared-root safety in the same change.
- Option D: implement P1 scope: `pi`, `qwen`, `kiro`, `copilot`, `windsurf`, and `kilo`. Tradeoffs: covers the must-have plus the highest-priority common tools identified by the user after research, but increases generator work: Copilot needs `.prompt.md` filename support and shared `.github` safety, while Kilo/Windsurf need workflow-style command roots in addition to skills.
- Selected: Option D. User selected P1 scope after reviewing the recommendation. Planning must implement `pi`, `qwen`, `kiro`, `copilot`, `windsurf`, and `kilo` in one governed change, with extra acceptance on Copilot shared-root ownership and command filename extension support.

## Unknowns
- Resolved: Is `pi` the only must-have target? -> Yes. User confirmed `pi` is the only required adapter; `kiro`, `copilot`, `qwen`, and similar tools are investigation candidates. Evidence: `artifacts/changes/expand-ai-tool-adapters/intent.md`.
- Resolved: Does Slipway need to copy Trellis' Pi extension/agents? -> No for this change. Intent preserves thin adapters and excludes agent orchestration unless required for adapter discovery; Pi can start with prompts, skills, and settings registration. Evidence: `artifacts/changes/expand-ai-tool-adapters/intent.md`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/multi-platform.mdx:216-226`.
- Resolved: Which common candidates are best evidenced? -> Pi, Kiro, Copilot, and Qwen. Trellis lists Pi/Kiro/Copilot among supported tools; GSD has Qwen and Copilot runtime descriptors; official docs confirm Agent Skills/prompt/settings surfaces for these hosts.
- Resolved: Which option should advance? -> Option D / P1 scope, confirmed by user after alternatives were presented.
- Remaining: None.

## Assumptions
- New adapters must remain project-local and deterministic. Evidence: `artifacts/changes/expand-ai-tool-adapters/intent.md`, `internal/toolgen/ownership_manifest.go:394-414`.
- Generated host files are adapter surfaces only and must route to the current-worktree Slipway CLI. Evidence: `docs/reference/ai-tools.md:7-9`.
- For Pi, static registration of skills/prompts is sufficient for a first thin adapter; Pi extensions and agents are not required to expose Slipway commands and governed skills. Evidence: `artifacts/changes/expand-ai-tool-adapters/intent.md`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/configurators/pi.ts:26-52`.
- For Qwen/Kiro, Agent Skills-compatible `SKILL.md` surfaces are a valid first adapter shape. Evidence: `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/qwen/capability.json:15-39`, `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/tests/qwen-skills-migration.test.cjs:5-11`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx:38-43`.
- For Kilo/Windsurf, a first Slipway adapter can use project-local workflow/command files plus skills where supported, without hooks. Evidence: `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx:30-36`, `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/multi-platform.mdx:228-236`.

## Canonical References
- `artifacts/changes/expand-ai-tool-adapters/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `internal/toolgen/toolgen.go`
- `internal/toolgen/ownership_manifest.go`
- `internal/toolgen/install_profiles.go`
- `internal/toolgen/surface_manifest.go`
- `internal/toolgen/toolgen_test.go`
- `internal/toolgen/adapter_contract_test.go`
- `internal/toolgen/surface_manifest_test.go`
- `docs/ai-tools.md`
- `docs/reference/ai-tools.md`
- `docs/SURFACE-MANIFEST.json`
- `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/types/ai-tools.ts`
- `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/configurators/pi.ts`
- `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/packages/cli/src/templates/pi/settings.json`
- `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/custom-commands.mdx`
- `/Users/yixianlu/ghq/github.com/mindfold-ai/Trellis/docs-site/zh/advanced/multi-platform.mdx`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/qwen/capability.json`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/copilot/capability.json`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/kilo/capability.json`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/capabilities/windsurf/capability.json`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/runtime-config-adapter-registry.cts`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/tests/qwen-skills-migration.test.cjs`
- `https://pi.dev/`
- `https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md`
- `https://kiro.dev/docs/skills/`
- `https://code.visualstudio.com/docs/agent-customization/agent-skills`
- `https://code.visualstudio.com/docs/copilot/customization/prompt-files`
- `https://qwenlm.github.io/qwen-code-docs/en/users/configuration/settings/`
- `https://docs.windsurf.com/plugins/cascade/workflows`
- `https://docs.windsurf.com/windsurf/cascade/skills`
- `https://kilo.ai/docs/customize/skills`
