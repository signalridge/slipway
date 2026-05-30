# Research

## Research Findings

### Architecture
- Affected modules: `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`, `internal/tmpl/templates/_partials/command-new-body.tmpl`, `internal/toolgen/toolgen.go`, and `internal/toolgen/toolgen_test.go`.
- Dependency chains: `commandRegistry` metadata feeds `buildCommandRenderData`, `buildWorkflowCommandEntries`, `renderStandaloneWorkflowCommandReference`, and generated per-command prompt entries. Workflow skill content comes from the workflow skill template and command-reference template.
- Blast radius: low; the change updates exported generated documentation and tests, not command execution or lifecycle state.
- Constraints: `cmd/new.go` remains the runtime authority for JSON stdin parsing. The generated surfaces must not imply unsupported `--guardrail-domain`, `--complexity`, or `--needs-discovery` flags.

### Patterns
- Existing conventions: command argument and prerequisite text are centralized in `commandRegistry`; workflow command references are rendered from shared toolgen data; generated-surface tests assert important strings in temp generated outputs.
- Reusable abstractions: extend command metadata with command-specific notes instead of hand-maintaining the same text in each generated workflow reference section.
- Convention deviations: none required.

### Risks
- Technical risks: low risk of stale generated prose if tests only check one adapter; mitigate by checking generated workflow content for every registered adapter and Codex/Claude prompt surfaces where relevant.
- Guardrail domains: none.
- Reversibility: safe to roll back because the change is documentation/test-only except for metadata used to render generated references.

### Test Strategy
- Existing coverage: `internal/toolgen/toolgen_test.go` already generates Codex, Claude, Cursor, Gemini, and OpenCode surfaces in temp directories and asserts key workflow/command reference contracts.
- Infrastructure needs: no new fixtures or snapshots are required.
- Verification approach: add assertions that generated workflow skill text explains JSON stdin classification, generated command reference includes `slipway new` stdin notes, and generated Codex/Claude `/slipway-new` surfaces document the explicit JSON stdin path without unsupported flags.

## Alternatives Considered
- Template-only wording: update only workflow and command-new templates. Tradeoff: fixes two visible surfaces but leaves generated command reference incomplete, which issue #21 calls out directly.
- Metadata-backed command reference notes: add command-specific notes to toolgen metadata and render them in the workflow command reference. Tradeoff: slightly expands metadata shape, but keeps command reference generated from the same registry path.
- Runtime flag support: add `--guardrail-domain`, `--complexity`, and `--needs-discovery` flags. Tradeoff: contradicts the issue's non-problem ruling and broadens runtime behavior unnecessarily.
- Selected: metadata-backed command reference notes plus targeted template updates and generated-surface tests, because it resolves the documented contract gap without changing CLI behavior.

## Unknowns
- Resolved: exact generated-surface test helpers -> `TestWorkflowSkillGenerationAndReference`, `TestCommandEntryPrerequisitesAreCommandSpecific`, `TestCodexPromptsUseCommandSpecificPrerequisites`, and generated prompt tests provide direct string assertion coverage.
- Remaining: None.

## Assumptions
- The checked issue state at HEAD `0f92e5d` is the relevant baseline. Evidence: GitHub issue #21 and local `git rev-parse --short HEAD`.
- The correct fix is contract clarity, not new flags. Evidence: issue #21 explicitly classifies this as documentation UX gap and says stale-flag framing is not the bug.
- `cmd/new.go` JSON stdin behavior is already functional. Evidence: `stdinClassificationInput` includes `guardrail_domain`, `needs_discovery`, and `complexity`; `readStdinClassificationInput` is used only in JSON mode when stdin is non-terminal.

## Canonical References
- `https://github.com/signalridge/slipway/issues/21`
- `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`
- `internal/tmpl/templates/_partials/command-new-body.tmpl`
- `internal/tmpl/templates/skills/workflow/command-reference.md.tmpl`
- `internal/toolgen/toolgen.go`
- `internal/toolgen/toolgen_test.go`
- `cmd/new.go`
- `artifacts/codebase/TESTING.md`
