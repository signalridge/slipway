# Development reference

This page supplements the root [Contributing guide](../../CONTRIBUTING.md) with repository-specific layout and documentation checks.

## Repository layout

| Path | Purpose |
| --- | --- |
| `cmd/` | Public CLI commands and JSON/human presentation. |
| `internal/autopilot/` | Run routing, protocol validation, source handling, and recovery choices. |
| `internal/runstore/` | Journals, projections, locking, materials, and Git observation. |
| `internal/adapter/` | Host generation and ownership-aware filesystem changes. |
| `internal/tmpl/` | Embedded capability instructions shared across hosts. |
| `internal/fsutil/` | Filesystem safety and transactions. |
| `docs/{en,zh,ja}/` | Equivalent user, guide, reference, and explanation pages. |
| `docs/reference/` | Language-neutral JSON schemas. |
| `adr/` | Maintainer decision history; not user documentation. |
| `tests/acceptance/` | Black-box scripts, prompt scenarios, and manual evidence procedures. |
| `website/` | Starlight site generated from `docs/`. |

The enforced package direction is documented in [Architecture](explanation/architecture.md).

## Local checks

For a normal Go change:

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go build ./...
git diff --check
```

Run the race suite for concurrency, locking, journal, or filesystem changes:

```bash
go test -timeout=20m ./... -race -count=1
```

When available, also run:

```bash
golangci-lint run --timeout 5m
goreleaser check
```

## Documentation checks

Repository Markdown is the source for the website. Generated files below `website/src/content/docs/` must not be edited, except the three locale splash pages.

```bash
python3 -I tests/acceptance/link_check.py --self-test
npm --prefix website ci
npm --prefix website run build
python3 -I tests/acceptance/link_check.py --require-site
git diff --check
```

Documentation rules:

- describe current behavior, not PR instructions or a mutable planning Issue;
- separate user guidance, integration reference, maintainer architecture, ADR rationale, and acceptance evidence;
- keep English, Chinese, and Japanese pages equivalent in scope;
- do not make one language the implementation contract;
- use JSON schemas for exact machine shape and state runtime-only semantic checks explicitly;
- qualify host-side instructions as host behavior rather than Go CLI guarantees;
- keep run-specific evidence and release history out of stable user pages;
- update source links, website splash links, and sidebar slugs together when moving a page.

## Test focus by change type

| Change | Focus |
| --- | --- |
| CLI flags or output | Cobra help, JSON schema tests, human rendering, and command docs. |
| Action/Outcome routing | Autopilot contract/service tests, machine shell acceptance, and protocol docs. |
| Journals or locking | Replay/adversarial tests, race suite, durability diagnostics, and recovery docs. |
| Source handling | Strict parser, hash/size/identity tests, source schema, Issue guide, and privacy docs. |
| Adapters/templates | Generator tests, ownership tests, `tests/acceptance/adapters.sh`, and adapter docs. |
| Release channels | GoReleaser checks, artifact validation, and installation compatibility wording. |
| Documentation | Link checker, markdown lint, website build, and locale parity review. |

## Product constraints

Changes must preserve explicit invocation, user control, facts-before-questions, truthful activity reporting, read-only Review, recoverable journals, ownership-aware generated files, and a network/credential-free core. If a public surface changes, update code, schemas, generated capabilities, tests, and all three documentation locales together.
