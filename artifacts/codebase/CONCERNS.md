# Concerns

- Shared directory risk: Copilot uses `.github/...`, which often already
  contains user-authored CI, prompts, agents, and instructions. A Copilot
  adapter must own only Slipway-prefixed files and its sentinel/manifest, never
  the whole `.github` tree. Evidence:
  `docs/reference/ai-tools.md:31-32`,
  `internal/toolgen/ownership_manifest.go:193-214`,
  `internal/toolgen/ownership_manifest.go:255-268`.
- Settings risk: current `SettingsPath` means "merge hook settings.json" for
  Claude/Gemini. Pi settings are registration arrays (`skills`, `prompts`, and
  optionally `extensions`), and Copilot/Qwen have different config surfaces.
  Do not overload hook-only settings semantics without a typed adapter setting
  kind. Evidence: `internal/toolgen/toolgen.go:1078-1098`,
  `internal/toolgen/toolgen.go:1994-2044`.
- Command filename risk: Copilot prompt files require `.prompt.md`; current
  command extension logic only supports `.md` and Gemini `.toml`. Add an
  explicit filename/extension axis before implementing Copilot prompts.
  Evidence: `internal/toolgen/toolgen.go:1030-1048`,
  `internal/toolgen/toolgen.go:1263-1279`.
- Workflow root risk: Kilo and Windsurf use workflow-style command files in
  Trellis, not the existing nested command root used by Claude/Gemini. Add an
  explicit command path style or keep the mapping narrow and pinned by frozen
  adapter contract tests.
- Trigger prose risk: `InvocationSummary()` special-cases command-skill
  surfaces as Codex-flavored `$slipway-*`. If Kiro or Qwen also use skill
  command surfaces, summaries and generated references must derive from each
  tool's trigger axes. Evidence: `internal/toolgen/toolgen.go:2279-2284`,
  `internal/toolgen/toolgen.go:2304-2315`.
- Compatibility risk: Trellis implements Pi via a TypeScript extension and
  agents, but this Slipway change is scoped to thin adapters. Pi support should
  start with prompts, skills, and minimal settings registration; extension and
  agent orchestration remain out of scope unless a later requirement proves
  they are necessary.
- Reversibility: adapter changes are mostly additive and reversible by removing
  tool registry entries and regenerated docs, but generated-file refresh logic
  must remain fail-closed because it can touch user project files.
