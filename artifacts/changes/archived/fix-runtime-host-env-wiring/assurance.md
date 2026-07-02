# Assurance

## Scope Summary

This change repairs GitHub issue #395 by making runtime-host environment wiring
discoverable from public surfaces instead of source-code inspection. It adds
structured env contract metadata to `EnvCatalog()`, projects that metadata
through `slipway config list --env` text and JSON output, and documents host
integration in public docs.

The core implementation is additive:

- `internal/model.EnvCatalogEntry` now includes `value_syntax`,
  `accepted_values`, `examples`, and `unset_behavior`.
- `SLIPWAY_HOST_CAPABILITIES` documents `subagent`, `delegation`, `none`,
  `unavailable`, and the closed-world handling of unrecognized non-empty tokens.
- `SLIPWAY_HOST_CAPABILITY_FALLBACKS` documents same-context and manual fallback
  modes and their case-insensitive token matching.
- `config list --env` preserves the original table and appends contract details;
  `config list --env --json` gains only additive `omitempty` fields.
- Public docs now include `docs/reference/host-environment.md`, README and docs
  index links, reference-command pointers, and the expanded `docs/commands.md`
  row that issue #395 identified as ownership-only.

S3 review also found two small runtime/parser mismatches that were better fixed
than documented around:

- `SLIPWAY_HOST_CAPABILITY_FALLBACKS` now matches fallback tokens
  case-insensitively while returning canonical mode names.
- `SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS` now accepts carriage-return separators,
  matching the catalog/docs and the host capability splitter.

No schema migration, external API shape change, config-file secret storage, or
credential egress relaxation is part of this work.

## Verification Verdict

Current verdict: ready for terminal ship-verification once the refreshed S3
review evidence and this assurance artifact are recorded.

Completed peer review evidence to refresh:

- `spec-compliance-review`: pass, including `layer:R0=pass`,
  `layer:R3=pass`, `scope_contract:pass`, and `negative_path:pass`.
- `code-quality-review`: pass, including `layer:IR1=pass` and
  `layer:IR3=pass`.
- `independent-review`: pass.
- `security-review`: pass.

Because this host has not declared subagent capability, the S3 peer reviews use
the explicit `same_context_degraded` fallback with distinct review handles. The
terminal ship-verification gate must also record the selected fallback and the
reviewer-independence, assurance, and high-risk safety-baseline attestations.

## Evidence Index

- Public command output:
  `go run . config list --env` and `go run . config list --env --json` both
  expose the structured contract fields and updated host capability/fallback
  text.
- Full suite:
  `artifacts/changes/fix-runtime-host-env-wiring/verification/logs/ship-go-test.txt`
  from `go test ./... -count=1`, exit 0.
- Lint:
  `artifacts/changes/fix-runtime-host-env-wiring/verification/logs/ship-golangci-lint.txt`
  from `golangci-lint run ./... --timeout=5m`, exit 0 when available in this
  worktree.
- Coverage:
  `artifacts/changes/fix-runtime-host-env-wiring/verification/logs/ship-coverage.txt`,
  `ship-coverage.out`, and `ship-coverage-func.txt` from
  `go test ./internal/model ./cmd -coverprofile=... -count=1`.
- SAST baseline:
  `artifacts/changes/fix-runtime-host-env-wiring/verification/logs/ship-semgrep.txt`
  and `ship-semgrep.json` from Semgrep `p/gosec` over the changed Go files:
  `cmd/config.go`, `cmd/config_test.go`, `cmd/tool_github.go`,
  `cmd/tool_github_test.go`, `internal/engine/capability/resolver.go`,
  `internal/engine/capability/resolver_test.go`,
  `internal/model/env_catalog.go`, and `internal/model/env_catalog_test.go`.
- Stub scan:
  `artifacts/changes/fix-runtime-host-env-wiring/verification/logs/ship-stub-scan.txt`.
- Review evidence:
  `verification/spec-compliance-review.yaml`,
  `verification/code-quality-review.yaml`,
  `verification/independent-review.yaml`, and
  `verification/security-review.yaml`.

## Requirement Coverage

- REQ-001 Structured Env Wiring Catalog:
  `internal/model/env_catalog.go` exposes structured metadata for all catalog
  entries, including host capability and fallback tokens. The model tests require
  value syntax and unset behavior for every catalog entry, scan public env
  literals, and assert closed-world/fallback wording.
- REQ-002 Host-Facing Config Output:
  `cmd/config.go` returns structured JSON directly from `EnvCatalog()` and
  appends `CONTRACT DETAILS` to text output. Command tests assert raw JSON field
  names, accepted host tokens, unset behavior, and the concrete subagent
  declaration.
- REQ-003 Public Host Integration Documentation:
  `docs/reference/host-environment.md` maps skill preconditions to
  `SLIPWAY_HOST_CAPABILITIES=subagent`, explains `delegation`, `none`,
  `unavailable`, fallback modes, context metrics, handoff owner, GitHub API
  allowlists, and secret boundaries. README, `docs/index.md`,
  `docs/reference/commands.md`, and `docs/commands.md` link or describe the new
  authority.
- REQ-004 Verification Coverage:
  Focused tests, full suite, Semgrep SAST, coverage output, stub scan, and four
  S3 reviews are recorded in the evidence index above.

## Residual Risks and Exceptions

- Text output is more verbose. This is accepted because the original table is
  preserved, and `--json` remains the stable machine-readable contract.
- Documentation is English-only in this change. Existing localized docs are not
  updated; the authoritative command output and primary docs now carry the
  complete contract.
- Case-insensitive fallback matching accepts uppercase spellings of existing
  fallback mode names. This is intentional host interoperability, not a bypass:
  fallback evidence is still required and unrecognized fallback tokens are still
  ignored.
- Carriage-return splitting slightly broadens accepted separator syntax for
  operator-supplied GitHub API allowlists. URL normalization, exact allowlist
  matching, and override-token destination checks still gate credential egress.

## Rollback Readiness

Rollback is straightforward and local:

- Revert `internal/model/env_catalog.go`, `internal/model/env_catalog_test.go`,
  `internal/engine/capability/resolver.go`,
  `internal/engine/capability/resolver_test.go`, `cmd/config.go`,
  `cmd/config_test.go`, `cmd/tool_github.go`, `cmd/tool_github_test.go`,
  `docs/reference/host-environment.md`, `docs/reference/commands.md`,
  `docs/commands.md`, `docs/index.md`, and `README.md`.
- Remove this change's governed bundle if the rollback abandons the change.
- Verify rollback with `go test ./internal/model ./cmd ./internal/engine/capability -count=1`
  and `go run . validate` in the active worktree.

No schema migration, irreversible state change, external API response shape
change, or credential-format change is part of this work.

## Archive Decision

Archive decision: not ready to archive until ship-verification records the
terminal pass evidence, including
`high_risk_check:external_api_contracts.safety_baseline=pass`,
`closeout:reviewer_independence=pass`, and
`closeout:assurance_complete=pass`.

After ship-verification is recorded, `go run . validate` and
`go run . next --json --diagnostics` must be rerun before declaring done-ready.
Archived bundles must be treated as frozen records, not revalidated through the
active validate gate.
