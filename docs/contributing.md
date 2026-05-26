# Contributing

Slipway is a Go CLI with generated AI-tool surfaces and governed artifact workflows. Keep changes scoped to the module or contract you are changing.

## Repository Layout

| Path | Purpose |
| --- | --- |
| `cmd/` | Cobra command surfaces and CLI JSON/text views. |
| `internal/model/` | Durable domain types, workflow states, and config schemas. |
| `internal/state/` | Filesystem layout, workspace config, change persistence, and archive helpers. |
| `internal/engine/` | Progression, gates, governance, artifact, context, and wave logic. |
| `internal/toolgen/` | AI-tool adapter generation and frozen surface contracts. |
| `internal/tmpl/templates/` | Embedded command, skill, hook, and artifact templates. |
| `docs/` | MkDocs source pages. |
| `artifacts/changes/` | Governed change bundles created by Slipway. |

## Development Commands

```bash
go run . --help
go test ./... -count=1
go build ./...
go vet ./...
```

For final proof, use the same timeout as CI:

```bash
go test -timeout=20m ./... -count=1
```

## Documentation

Update `docs/` and `mkdocs.yml` together. Every nav target must exist in the current checkout.

Build locally when MkDocs is installed:

```bash
mkdocs build --strict
```

The docs workflow installs MkDocs Material and runs the same strict build.

## Adapter Contracts

When command metadata, generated paths, hooks, or prompt surfaces change, update code and tests together:

```bash
go test ./internal/toolgen -count=1
```

Generated surfaces are contract-tested, including supported tool IDs, command paths, Codex global prompts, OpenCode flat commands, and byte stability.

## Governance Contracts

When lifecycle, artifact, or gate semantics change:

- Add a focused regression test in the owning package.
- Keep shared semantics in a helper rather than duplicating Markdown or state parsing.
- Update generated skills or docs when the host contract changes.
- Verify with `go run . validate --json` inside the active governed worktree.
