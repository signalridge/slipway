# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - CLI help and flag declarations in `cmd/status.go`, `cmd/review.go`, and
    `cmd/health.go`.
  - Command metadata and generated command surfaces in
    `internal/toolgen/toolgen.go`,
    `internal/tmpl/templates/_partials/`, and
    `internal/tmpl/templates/skills/workflow/`.
  - Host skill templates and catalog source files under
    `internal/tmpl/templates/skills/`.
  - User-facing docs and diagram SVG accessible text under `docs/`.
- Dependency chains:
  - Cobra command flags -> live `go run . <command> --help` output.
  - `commandRegistry` -> generated command references, generated command
    skills, surface manifest rows, and docs tokens.
  - `ResolveNextSkill` / `SelectedReviewSkills` -> generated governance skill
    descriptions and S3 review docs.
  - `internal/state` runtime path helpers -> operator docs and installation
    docs describing evidence locations.
- Blast radius: docs/help/surface-template behavior only. No lifecycle state
  transitions, gate calculations, or runtime evidence storage paths should
  change.
- Constraints:
  - Generated command-surface prose must not claim `tool` has host wrappers
    because `tool` is explicitly `HasPromptSurface: false`.
  - Flowchart descriptions must describe examples or classes accurately rather
    than presenting narrow command examples as exhaustive.

### Patterns
- Existing conventions:
  - Command descriptions live in `internal/toolgen/toolgen.go` and are consumed
    by both Cobra `Short` descriptions and generated adapter surfaces.
  - Host skills are source templates under `internal/tmpl/templates/skills/`;
    generated local `.codex`/`.claude` directories are not the editing surface.
  - Docs use `docs/SURFACE-MANIFEST.json` tokens and toolgen tests to keep
    generated surfaces discoverable.
  - Runtime evidence paths are reported by `next/status/validate` path-authority
    diagnostics and should be documented from those code paths.
- Reusable abstractions:
  - Existing command/template flag-contract tests should cover help and
    generated argument changes.
  - Existing toolgen manifest tests should catch missing docs tokens and
    generated surface drift.
- Convention deviations:
  - None required. The change should update authoritative source text and tests,
    not generated project-local adapter outputs.

### Risks
- Technical risks:
  - Medium: changing docs/templates without updating manifest or tests can leave
    generated adapter surfaces stale.
  - Medium: overcorrecting security-review trigger prose could hide that
    security review is still selected when the security-review control is
    active.
  - Low: CLI help placeholder changes are user-visible but should not affect
    flag parsing.
  - Low: SVG diagram text edits can accidentally alter visual rendering if text
    length changes without layout checks.
- Guardrail domains: none. This is a docs/help alignment change and does not
  alter auth, credentials, schema migration, irreversible operations, or
  external API contracts.
- Reversibility: high. Changes are text/help/template edits plus tests; they can
  be reverted without data migration.

### Test Strategy
- Existing coverage:
  - `cmd/template_flag_contract_test.go` checks generated command arguments
    against Cobra flags.
  - `cmd/root_help_test.go` and related command tests check root help command
    descriptions.
  - `internal/toolgen/*_test.go` checks generated surface inventory and command
    skill content.
- Infrastructure needs:
  - Add or update focused tests around the `--hydrate-ref` help metavar.
  - Update tests that assert helper docs coverage if `pin-actions` is now part
    of the documented helper namespace.
  - Run surface manifest check/write if doc tokens or generated surface rows
    change.
- Verification approach:
  - Run live help checks for `status`, `review`, and `health` to confirm the
    placeholder is no longer `--hydrate`.
  - Run targeted `go test ./cmd` and `go test ./internal/toolgen`.
  - Run `go run ./internal/toolgen/cmd/gen-surface-manifest --check`.
  - Run `go test ./...` if time/environment allows, because help and generated
    template changes are shared surfaces.

### Options
- Minimal docs-only update:
  - Tradeoff: fixes some drift, but leaves the confirmed `--hydrate-ref` help
    bug and test coverage gap.
- Source-surface alignment:
  - Tradeoff: touches docs, templates, and small CLI help strings; better
    matches the user's "including help" scope without altering lifecycle logic.
- Broader routing redesign:
  - Tradeoff: would change behavior around security path triggers or command
    taxonomy. This is not justified by the confirmed gaps.
- Selected: source-surface alignment. It fixes confirmed docs/templates/help
  drift, updates tests where current guards missed defects, and leaves lifecycle
  behavior unchanged.

## Unknowns
- Resolved: Are there Markdown Mermaid flowcharts in `docs/`? -> No. The docs
  diagram surfaces are SVG files under `docs/assets/diagrams/`.
- Resolved: Does every public CLI command generate a host command surface? ->
  No. `tool` is public CLI-only and generated skills call helper subcommands
  directly.
- Resolved: Does `security-review` selection currently consume auth/crypto/path
  globs? -> No. Current selection is driven by the active security-review
  control, derived from guardrail/blast-radius logic.
- Resolved: Does `worktree-preflight` route invalid non-empty bindings? -> No.
  It routes missing metadata for discovery changes; invalid existing bindings
  surface through validation/recovery blockers.
- Remaining: None.

## Assumptions
- The user's original request is the scope confirmation for including help
  defects discovered during audit. Evidence: user requested "包括help等" and
  asked for parallel subagent investigation before repair.
- Generated local adapter directories are not required in this worktree to fix
  source templates. Evidence: `internal/toolgen/toolgen.go` renders templates
  from `internal/tmpl/templates/`, and no checked-in `.codex` or `.claude`
  directories exist in the bound worktree.
- Root checkout dirty `artifacts/codebase/*` files are unrelated to this change.
  Evidence: the bound worktree was created clean except the new governed
  artifact bundle.

## Canonical References
- `cmd/status.go`
- `cmd/review.go`
- `cmd/health.go`
- `cmd/root.go`
- `cmd/tool.go`
- `cmd/tool_actions.go`
- `internal/toolgen/toolgen.go`
- `internal/engine/progression/skill_resolution.go`
- `internal/engine/skill/skill.go`
- `internal/engine/control/derive.go`
- `internal/state/paths.go`
- `internal/state/store.go`
- `internal/state/wave_execution.go`
- `internal/state/verification.go`
- `internal/state/lifecycle_event.go`
- `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`
- `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`
- `internal/tmpl/templates/skills/worktree-preflight/SKILL.md`
- `internal/tmpl/templates/skills/security-review/SKILL.md`
- `internal/tmpl/templates/skills/security-review/SKILL.md.tmpl`
- `internal/tmpl/templates/skills/git-recovery/SKILL.md`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`
- `docs/ai-tools.md`
- `docs/workflow.md`
- `docs/index.md`
- `docs/operator-guide.md`
- `docs/installation.md`
- `docs/assets/diagrams/architecture.svg`
