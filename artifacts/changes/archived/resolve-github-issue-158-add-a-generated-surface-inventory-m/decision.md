# Decision

## Alternatives Considered

- Filesystem-scanning script outside the Go package: simple to run, but it would add a second toolchain path and would tend to duplicate Slipway's Go-owned command and skill registries.
- Test-only golden manifest in `internal/toolgen/testdata`: lower public surface area, but it would not satisfy the issue's request for a committed generated-surface inventory that operators and reviewers can inspect as a product artifact.
- Go-native public manifest implementation: adds a small amount of package code and a regeneration entrypoint, but keeps source authority in `internal/toolgen`, makes CI and maintainers use the same manifest builder, and exposes the product inventory in `docs/`.

Selected: Go-native public manifest implementation.

## Selected Approach

Implement a surface manifest builder in `internal/toolgen` that derives rows from existing Slipway authorities:

- adapter tool configs and path rules from `Registry`, `SkillPath`, `GeneratedAdapterMarkerPath`, and command surface path conventions
- command prompt surfaces from `commandRegistry` and `commandIDs`
- generated governance, standalone, catalog, and technique skill surfaces from existing toolgen descriptors and capability registry filtering
- JSON/user-facing contract rows from the documented command JSON surfaces and stable model/contract authorities already used by tests
- documentation rows from stable tokens in `README.md`, `docs/ai-tools.md`, `docs/commands.md`, and `docs/operator-guide.md`

Commit the generated JSON as `docs/SURFACE-MANIFEST.json`. Add a small Go regeneration entrypoint with check and write modes that uses the same builder. Add sync tests that rebuild the manifest, compare it with the committed file, and verify docs tokens. Keep the existing README command-token/description tests active.

## Interfaces and Data Flow

New internal package surface:

- `internal/toolgen` exposes or owns a `BuildSurfaceManifest` style helper returning deterministic manifest data.
- The manifest writer marshals that data with stable ordering and indentation.
- Tests and the regeneration entrypoint both call the same builder.

New command/script surface:

- A Go entrypoint under `internal/toolgen/cmd/gen-surface-manifest` supports check mode and write mode.
- Check mode compares live output with `docs/SURFACE-MANIFEST.json` and prints actionable differences.
- Write mode overwrites `docs/SURFACE-MANIFEST.json` from live authorities.

Data flow:

`internal/toolgen` registries and docs token definitions -> manifest builder -> JSON encoder -> committed `docs/SURFACE-MANIFEST.json` -> sync/docs tests and regeneration entrypoint.

No runtime lifecycle state, evidence model, or command behavior changes are required.

Review repair addendum:

- While re-certifying the fixed manifest, the current lifecycle produced a
  dead-end: S2 execution had completed, old S3/S4 review evidence was stale, and
  `slipway run` reported the stale future evidence before the change could reach
  S3 where `slipway evidence skill` is allowed to refresh it.
- The narrow repair keeps digest freshness fail-closed but filters
  previously-consumed skill evidence by lifecycle position. Directly passing
  evidence for the current stage is still stamped. Previously consumed evidence
  from future stages is checked only after the lifecycle reaches those stages.
- This changes no public command shape and does not bypass review/verify gates;
  it only prevents future-stage evidence from blocking the current S2 gate before
  the owning S3/S4 stage can re-certify it.
- A second S2 command-surface dead-end appeared after task evidence was refreshed:
  `slipway run` requires passing `wave-orchestration` evidence before it can
  build `execution-summary.yaml`, but the generic `slipway evidence skill` path
  required `execution-summary.yaml` before recording any run-summary-bound skill.
- The selected repair is a narrow bootstrap in `slipway evidence skill` for
  `wave-orchestration` while the change is in S2. It derives the run version
  from the current flat task evidence ledger, rejects missing, invalid, or
  mixed-version task evidence, and leaves S3/S4 run-summary-bound skills
  fail-closed until `execution-summary.yaml` exists.

## Rollout and Rollback

Rollout:

1. Add the manifest builder, regeneration entrypoint, committed manifest, tests, and docs links.
2. Run focused tests: `go test ./internal/toolgen ./cmd`.
3. Run full verification: `go test ./...`.
4. Complete governed review and validation.

Rollback:

1. Revert the manifest builder, regeneration entrypoint, committed manifest, docs additions, and tests.
2. Run `go test ./internal/toolgen ./cmd` and `go test ./...` to verify existing generated-surface tests still pass.

## Risk

- Duplicate-authority risk: mitigated by deriving rows from existing `internal/toolgen` and capability authorities rather than introducing a second command/skill list.
- Docs brittleness risk: mitigated by checking stable tokens and table/path entries, not broad prose snapshots.
- Scope drift risk: mitigated by keeping implementation out of runtime lifecycle state and limiting changed production code to manifest generation.
- Maintenance risk: mitigated by using one builder for committed JSON, check mode, write mode, and tests.
- Recovery dead-end risk: mitigated by a focused regression that proves S2 wave
  digest stamping ignores stale evidence from future lifecycle stages while
  preserving stale checks once those stages are reached.
- Wave evidence bootstrap risk: mitigated by limiting the task-ledger run-version
  bootstrap to the S2 `wave-orchestration` skill and preserving
  `evidence_skill_run_summary_missing` for review/verify skills without an
  execution summary.
