# Research

Question: How should Slipway repair public command-surface drift so root help,
toolgen registry, surface manifest, command docs, adapter docs, and Cobra flags
stay aligned?

## Alternatives Considered

### Architecture
- Affected modules: root command registration lives in `cmd/root.go:82` and
  `cmd/root.go:213`; `config` implementation lives in `cmd/config.go:13`;
  adapter/command metadata lives in `internal/toolgen/toolgen.go:229`; manifest
  rows are built from `commandRegistry` in
  `internal/toolgen/surface_manifest.go:31`.
- Dependency chains: Cobra commands in `cmd/*` expose runtime behavior; toolgen
  `commandRegistry` feeds generated command skills, registry arguments,
  manifest command rows, JSON contract rows, and command-reference expectations;
  docs pages consume both the registry-derived surface and hand-authored token
  tables.
- Blast radius: low runtime behavior, medium public-contract surface. Changes
  touch command metadata, docs, manifest generation/checks, and directly affected
  command tests.
- Constraints: CLI-only commands can be registered with
  `HasPromptSurface: false` (`internal/toolgen/toolgen.go:237`), while
  `commandIDs()` only includes prompt-surface commands
  (`internal/toolgen/toolgen.go:443`).

### Patterns
- Existing conventions: public command metadata belongs in `commandRegistry`,
  which is described as the single source of truth in
  `internal/toolgen/toolgen.go:240`; root help should use registry descriptions
  through `desc()` where possible (`cmd/root.go:82`).
- Existing CLI-only precedent: `tool` is in `commandRegistry` with
  `HasPromptSurface: false`, appears in manifest/command docs, and explicitly
  avoids host prompt wrappers (`internal/toolgen/toolgen.go:337`).
- Manifest pattern: command rows are derived from every registry entry, while
  JSON contract rows are derived only from registry entries whose `Arguments`
  contain `--json` (`internal/toolgen/surface_manifest.go:36` and
  `internal/toolgen/surface_manifest.go:147`).
- Flag contract pattern: every visible Cobra flag must appear in registry
  arguments unless narrowly exempted (`cmd/template_flag_contract_test.go:280`);
  `review --artifact` is currently the only review exemption
  (`cmd/template_flag_contract_test.go:313`).

### Risks
- Technical risks: adding `config` as a prompt surface would create generated
  host skills for a repo-local configuration editor, expanding adapter behavior
  more than required. Severity: medium.
- Technical risks: leaving `config` outside `commandRegistry` preserves the
  current blind spot where manifest checks pass while root help exposes an
  undocumented command. Severity: high for public-surface correctness.
- Technical risks: keeping `review --artifact` as an unsupported flag preserves
  a dead public affordance and forces a permanent flag-contract exemption.
  Severity: medium.
- Guardrail domain: external API/public contract surface. The repair must fail
  closed to manifest/docs/tests and S3 review evidence.
- Reversibility: registry/docs/test changes are reversible with ordinary git
  revert; removing `review --artifact` removes an unsupported MVP flag, not a
  working behavior.

### Test Strategy
- Existing coverage: `cmd/config_test.go` verifies `config` behavior and root
  registration; `cmd/root_help_test.go` verifies root help visibility;
  `internal/toolgen/surface_manifest_test.go` checks committed manifest
  freshness and docs tokens; `cmd/template_flag_contract_test.go` checks Cobra
  flags against registry arguments.
- Coverage gaps: no test currently asserts root-visible commands are in
  `commandRegistry`; localized command token tables are not tied to manifest
  tokens; adapter design prose and SVG copy can drift from the 10-tool registry.
- Verification approach: add a root-command/registry alignment test; update
  registry count expectations; remove `review --artifact` test/exemption; run
  manifest generation/check; add docs-token coverage where practical for
  English/Japanese/Chinese command token tables and adapter count prose.

### Options
- Approach A: register `config` as CLI-only command metadata, remove the dead
  `review --artifact` flag, update docs/manifest/localized token tables, and add
  targeted drift tests. Tradeoffs: smallest behavior change, preserves adapter
  thinness, prevents the current blind spot, but `config` remains manual CLI
  usage instead of a generated host skill.
- Approach B: register `config` with `HasPromptSurface: true` and generate host
  command skills for configuration. Tradeoffs: maximal discoverability, but it
  expands adapter behavior for repo config mutation and increases generated
  surface blast radius without a proven need.
- Approach C: leave `config` out of registry and add separate manifest/doc rows
  by hand. Tradeoffs: minimal code churn, but preserves two authorities and
  makes future drift likely.
- Recommended: Approach A. It matches the existing `tool` CLI-only precedent,
  makes root help, manifest, docs, and tests agree, and avoids exporting repo
  configuration mutation through generated host prompt wrappers.
- Selected: Approach A. The user selected A in chat after reviewing the
  alternatives. This keeps `config` CLI-only while making it visible to the
  registry, manifest, and docs, and removes the unsupported `review --artifact`
  flag instead of preserving a dead public affordance.

## Unknowns
- Resolved: whether `config list --json` currently returns 22 keys or 25 keys
  -> current command output returns 25 catalog entries.
- Resolved: whether `docs/reference/commands.md` already contains the canonical
  `run --json` and `handoff show --json` tokens -> yes, but the detailed English,
  Japanese, and Chinese command pages contain stale `run [--auto|--no-auto]
  --json` tokens and omit handoff JSON.
- Remaining: None.

## Assumptions
- `config` should be visible in public command inventory because root help
  already exposes it and `cmd/config.go` has behavior tests. Evidence:
  `cmd/root.go:82`, `cmd/root.go:213`, `cmd/config_test.go:432`.
- `config` should not generate host prompt wrappers unless the user explicitly
  chooses that product expansion. Evidence: `tool` demonstrates the CLI-only
  registry pattern in `internal/toolgen/toolgen.go:337`.
- Removing `review --artifact` is acceptable because the flag is currently
  unsupported and returns `unsupported_flag` before review execution. Evidence:
  `cmd/review.go:77`, `cmd/review.go:146`.
- Docs pages under `docs/` are the source that should be edited, not website
  generated content. Evidence: `docs/contributing.md:77`.

## Canonical References
- `cmd/root.go:82`
- `cmd/root.go:213`
- `cmd/config.go:13`
- `cmd/review.go:77`
- `cmd/review.go:146`
- `cmd/template_flag_contract_test.go:280`
- `cmd/template_flag_contract_test.go:313`
- `cmd/command_description_contract_test.go:13`
- `cmd/config_test.go:432`
- `internal/toolgen/toolgen.go:229`
- `internal/toolgen/toolgen.go:237`
- `internal/toolgen/toolgen.go:240`
- `internal/toolgen/toolgen.go:337`
- `internal/toolgen/toolgen.go:443`
- `internal/toolgen/surface_manifest.go:31`
- `internal/toolgen/surface_manifest.go:147`
- `internal/toolgen/surface_manifest_test.go:149`
- `docs/reference/commands.md:11`
- `docs/reference/commands.md:44`
- `docs/commands.md:229`
- `docs/commands.md:251`
- `docs/ja/commands.md:129`
- `docs/ja/commands.md:151`
- `docs/zh/commands.md:196`
- `docs/zh/commands.md:213`
- `docs/reference/ai-tools.md:11`
- `README.md:107`
- `docs/explanation/design.md:41`
- `docs/design.md:12`
- `docs/assets/diagrams/tool-adapters.svg:1`
