# Architecture

- Module responsibilities: adapter definitions and generation live in
  `internal/toolgen/toolgen.go`. `ToolConfig` describes each host's skills,
  commands, settings, hooks, trigger style, and auto-detect roots; `Registry()`
  returns the sorted public adapter set. Evidence:
  `internal/toolgen/toolgen.go:23-43`,
  `internal/toolgen/toolgen.go:45-124`,
  `internal/toolgen/toolgen.go:652-668`.
- Dependency flow: `slipway init --tools` resolves tool IDs through
  `ResolveTools`, then `GenerateWithInstallProfile` iterates selected
  `ToolConfig` entries and writes host files. Generated surfaces route back to
  the Slipway CLI; they are not lifecycle engines. Evidence:
  `internal/toolgen/toolgen.go:711-719`,
  `internal/toolgen/toolgen.go:763-786`,
  `docs/reference/ai-tools.md:7-9`.
- Generation boundary: host skills use `SkillPath(cfg, id)` and command prompts
  are emitted from `CommandsDir` using either flat or nested style. Codex is the
  existing exception that emits per-command skills via `CommandSkillSurface`.
  Evidence: `internal/toolgen/toolgen.go:819-827`,
  `internal/toolgen/toolgen.go:1030-1069`,
  `internal/toolgen/toolgen_test.go:1829-1878`.
- Ownership and refresh boundary: generated adapters are trusted only through a
  per-tool sentinel plus project-local ownership manifest under
  `<ToolRootPath>/slipway/`. Refresh writes generated files through
  `toolRefreshPlan`, refuses unknown or user-modified files, and detects
  existing adapters by sentinel, not by bare host directories. Evidence:
  `internal/toolgen/toolgen.go:789-800`,
  `internal/toolgen/toolgen.go:740-755`,
  `internal/toolgen/ownership_manifest.go:74-80`,
  `internal/toolgen/ownership_manifest.go:193-214`,
  `internal/toolgen/ownership_manifest.go:394-414`.
- Current change blast radius: adding Pi and any additional selected common
  tools should stay inside the toolgen registry, command filename/extension
  helpers, optional adapter settings support, docs, surface manifest, and
  tests. It should not add new lifecycle states or host-specific governance.
  Evidence: `artifacts/changes/expand-ai-tool-adapters/intent.md`.
