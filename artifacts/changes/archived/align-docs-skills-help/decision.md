# Decision

## Alternatives Considered
- Docs-only correction: low implementation risk, but it leaves the live
  `--hydrate-ref` help defect and generated-surface assertions inconsistent.
- Source-surface alignment: update CLI help strings, generated skill templates,
  docs, diagram descriptions, and focused tests from current code authorities.
- Behavior redesign: change routing logic for security-review triggers,
  worktree preflight, or command grouping. This is broader than the confirmed
  documentation/help drift and would require a separate governed objective.

## Selected Approach
Use source-surface alignment. Fix confirmed live help defects and update
authoritative docs/template sources while preserving lifecycle behavior. This
matches the user's request to include help, keeps the change reversible, and
does not introduce new governance semantics.

## Interfaces and Data Flow
- CLI flag parsing remains unchanged. Only Cobra usage strings for
  `--hydrate-ref` are corrected so help output uses the intended metavar.
- Generated command/skill surface data flow remains
  `internal/toolgen/toolgen.go` and `internal/tmpl/templates/**` ->
  generated adapter files and `docs/SURFACE-MANIFEST.json`.
- Runtime evidence storage remains unchanged: task/wave evidence lives under
  git-local Slipway runtime state; lifecycle events and verification YAML remain
  bundle-local.
- S3 review flow remains unchanged: Go resolves/exposes selected review peers;
  host adapters perform native subagent dispatch.

## Rollout and Rollback
- Rollout: commit docs/templates/help/test changes together after targeted Go
  tests, manifest check, and lifecycle validation pass.
- Rollback: revert the commit. No data migrations or persisted runtime format
  changes are introduced.
- Verification command set:
  - `go run . status --help`
  - `go run . review --help`
  - `go run . health --help`
  - `go test ./cmd -run 'TestTemplateFlagsMatchCobraCommands|TestCobraFlagsCoveredByRegistryArguments|TestRootHelpUsesCurrentEntrySurfaceDescriptions' -count=1`
  - `go test ./internal/toolgen -run 'TestCommandRegistry|TestSkillHelperDocsUseSlipwayTool|TestGeneratedAdapterSurfacesStayInSyncWithRegistry|TestGeneratedCommandEntriesIncludeClassMetadata|TestGeneratedNewCommandSurfacesDocumentJSONStdinClassification' -count=1`
  - `go run ./internal/toolgen/cmd/gen-surface-manifest --check`

## Risk
- Help output changes are visible to users but do not change accepted flags.
- Generated-surface wording changes can stale manifest/doc-token checks if not
  regenerated or verified.
- SVG text edits can alter rendering if labels become too long, so diagram text
  changes should stay concise and be inspected via diff.
- Overbroad trigger wording can mislead future agents; descriptions should say
  "when selected" or "host-binding support" where code selection is conditional.
