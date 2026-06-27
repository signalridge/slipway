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
| `docs/` | Documentation source pages (the source of truth). |
| `website/` | Astro Starlight site that renders `docs/` for GitHub Pages. |
| `artifacts/changes/` | Governed change bundles created by Slipway. |

## Development Commands

```bash
go run . --help
go test ./... -count=1
go build ./...
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
```

For final proof, use the same timeout as CI:

```bash
go test -timeout=20m ./... -count=1
```

## Test Quality Policy

Tests should prove behavior, not implementation trivia or machine timing. Do
not skip bad tests to quiet CI. Delete them and replace them in the same PR with
deterministic behavior coverage.

- **Vacuous tests**: delete tests that only execute code, check hard-coded
  constants, or assert that mocks were called without constraining user-visible
  behavior.
- **Source-grep tests**: delete tests that read `.go` files and assert
  `strings.Contains` over source text. Replace them with tests that exercise
  the exported behavior, parser result, state transition, or rendered output the
  source was supposed to protect.
- **Elapsed-time tests**: delete tests that assert wall-clock elapsed time with
  `time.Since`, duration thresholds, sleeps, or scheduler timing. Replace them
  with deterministic synchronization, fake clocks, controlled contexts, or
  explicit events.
- **Real-race tests**: delete tests that try to prove races by hoping goroutines
  interleave a certain way. Replace them with synchronization barriers, race
  detector coverage, or direct state-machine assertions.

Text assertions are still valid when text is the product behavior: generated
surfaces, golden-output fixtures, and CLI/API contract tests may assert exact
text or required substrings. Keep those fixtures close to the behavior under
test and name the test so reviewers can see that the text is the contract, not
an implementation grep.

The `internal/testlint` analyzer covers the highest-signal local policy checks:
source-grep tests that read `.go` files and assert `strings.Contains`, and
elapsed-time assertions based on `time.Since` or measured duration comparisons.
Run it directly with:

```bash
go run ./internal/testlint/cmd/testlint ./...
```

## Documentation

Documentation lives in `docs/` (the source of truth) and is rendered by the
Astro Starlight site in `website/`. `website/scripts/sync-docs.mjs` transforms
`docs/**` into the Starlight content collection at build time, so never edit
`website/src/content/docs/` by hand. Update the sidebar in
`website/astro.config.mjs` when you add or move a page.

Build or preview locally:

```bash
cd website
npm install
npm run build   # runs sync-docs, then `astro build`
npm run dev     # local preview with content synced from docs/
```

The docs workflow runs the same `npm run build` and deploys to GitHub Pages.

## Adapter Contracts

When command metadata, generated paths, hooks, or prompt surfaces change, update code and tests together:

```bash
go test ./internal/toolgen -count=1
```

Generated surfaces are contract-tested, including supported tool IDs, command paths, Codex command skills and legacy prompt cleanup, OpenCode flat commands, and byte stability.

## Governance Contracts

When lifecycle, artifact, or gate semantics change:

- Add a focused regression test in the owning package.
- Keep shared semantics in a helper rather than duplicating Markdown or state parsing.
- Update generated skills or docs when the host contract changes.
- Verify with `go run . validate --json` inside the active governed worktree.

## Governed Coverage Gates

The governance kernel — `internal/engine/gate`, `internal/engine/governance`, and `internal/engine/progression` (the readiness resolver lives in `progression/readiness.go`) — is protected by a no-regression coverage gate. High-risk public lifecycle surfaces are protected by a second tiered gate covering `cmd` and `internal/state`, including `status`, `next`, `validate`, `done`, `evidence`, verification, worktree, and runtime-state paths. If any gated package's statement coverage drops below its committed floor, CI fails. The gates fail closed: they never auto-lower the floor and have no skip, force, or soft-pass path.

- **Baselines**: `coverage-baseline.json` records the kernel floors, and `coverage-public-surface-baseline.json` records the public-surface floors plus the package/file/surface metadata used in diagnostics. They are generated by the `covergate` tool, never hand-edited.
- **CI job**: the `Kernel Coverage Gate` job runs the full suite once with `-coverpkg` scoped to the union of kernel and public-surface packages, then runs `covergate -target kernel -check` and `covergate -target public-surface -check`. Coverage is measured on a single OS (ubuntu) so the baselines are deterministic.
- **Union semantics**: `-coverpkg` over a multi-package run emits the same block once per test binary; `covergate` unions them (a block counts once and is covered if any occurrence ran), matching `go tool cover`.
- **Mode selection**: `covergate` requires an explicit `-check` or `-write`; invoking it without a mode is rejected so a gate invocation cannot accidentally soft-pass. `-target` selects `kernel` or `public-surface`. `-check` always uses the committed baseline as-is; write-time-only flags such as `-exclude` are rejected in check mode.
- **Public-surface diagnostics**: public-surface failures name the package and the related surface/file metadata, so the next fix is a targeted test rather than a global percentage hunt.

Run the gate locally:

```bash
just coverage-gate
```

When CI reports a regression, the usual fix is to add tests that restore coverage. If a drop is intentional and reviewed (for example, dead code was removed), ratchet the affected baseline and commit the diff so the change is visible in review:

```bash
just coverage-baseline   # regenerates governed coverage baselines from current coverage
```

A downward baseline edit is never automatic — it appears in the pull-request diff and must be reviewed. Run the same command to raise the floor after improving coverage.

**Exclusion list**: pass `-exclude <prefix[,prefix...]>` to `covergate` only at `-write` time, when the gated set is chosen. It is reserved for generated or test-only prefixes if a target's include set broadens; it cannot remove a required kernel or public-surface package floor.

After the CI job is green on the default branch, maintainers should keep `Kernel Coverage Gate` in branch-protection required status checks so either kernel or public-surface regression is red and merge-blocking.
