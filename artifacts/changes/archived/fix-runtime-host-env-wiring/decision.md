# Decision

## Alternatives Considered

1. Docs-only repair.
   - This would add a host integration page explaining
     `SLIPWAY_HOST_CAPABILITIES=subagent`.
   - It is too weak because `slipway config list --env --json` would remain
     unable to tell hosts accepted values, unset behavior, or examples.

2. Skill-remediation repair.
   - This would patch each affected generated skill to name
     `SLIPWAY_HOST_CAPABILITIES`.
   - It crosses the instruction boundary: skills should describe workflow
     prerequisites and evidence obligations, not become host integration
     manuals. It also duplicates the same wiring across many stage skills.

3. Structured env catalog repair with doc projection.
   - This adds optional structured wiring metadata to `EnvCatalogEntry`, fills
     it for env vars with real contracts, projects it through
     `config list --env` text/JSON, and adds concise public docs that point back
     to the catalog.
   - It is additive, machine-readable, testable, and keeps runtime behavior
     unchanged.

## Selected Approach

Select alternative 3: extend the env catalog as the single public authority for
environment-variable contracts, then document host integration around that
authority.

The concrete design:

- Add optional fields to `internal/model.EnvCatalogEntry`:
  - `ValueSyntax string` for free-form value shape.
  - `AcceptedValues []EnvAcceptedValue` for constrained tokens and their
    behavior.
  - `Examples []string` for copyable declarations.
  - `UnsetBehavior string` for unset, empty, malformed, or fallback behavior.
- Populate these fields for all catalog entries where research found non-obvious
  wiring: host capabilities, fallback modes, context window/metrics/session
  owner, and GitHub token/API environment variables.
- Keep existing fields and runtime behavior unchanged.
- Update `cmd/config.go` text output so humans see the contract in
  `config list --env`; JSON output receives the new fields automatically.
- Add a host environment reference page and link it from existing config and
  command docs.
- Add tests that connect the public catalog contract to existing runtime
  semantics, especially the `subagent` / `delegation` / `none` / `unavailable`
  behavior.

## Interfaces and Data Flow

Changed public interface:

- `slipway config list --env --json` gains additive optional fields on each env
  entry. Existing JSON fields remain unchanged.
- `slipway config list --env` gains human-readable contract columns or detail
  lines that expose value syntax, accepted values, examples, and unset behavior.

Internal data flow:

1. Runtime code continues to read environment variables from the same call sites.
2. `EnvCatalog()` records the public contract for those reads.
3. `runConfigList(..., envFlag=true)` reads `EnvCatalog()` and projects it to
   JSON/text.
4. Docs point host integrators to `slipway config list --env` for the current
   authority.

No persisted change-state format, lifecycle transition, capability resolver, or
credential routing behavior changes.

## Rollout and Rollback

Rollout:

- Ship additive model fields, output formatting, docs, and tests together.
- Existing JSON consumers that ignore unknown fields continue to work.
- Humans get richer text output from the same command.

Rollback:

- Revert changes to `internal/model/env_catalog.go`, `cmd/config.go`, affected
  tests, and docs.
- Verify rollback with `go test ./internal/model ./cmd -count=1`.

## Risk

- JSON compatibility risk: mitigated by additive `omitempty` fields only.
- Text-output churn risk: mitigated by retaining the existing core columns and
  appending structured contract details rather than changing command semantics.
- Secret handling risk: mitigated by preserving `Secret=true` and empty
  `FileConfigKey` for secret entries.
- Documentation drift risk: mitigated by making the catalog/output the primary
  authority and testing key host capability wording there.
- External API contract risk: mitigated by review gates and by avoiding runtime
  behavior changes.
