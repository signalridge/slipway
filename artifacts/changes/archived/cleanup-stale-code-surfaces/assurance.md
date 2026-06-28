# Assurance

## Scope Summary

This assurance covers governed change `cleanup-stale-code-surfaces` in the
bound worktree `.worktrees/cleanup-stale-code-surfaces`.

Cleanup authority is `redundancy-candidates.md`. Older worktrees were not used
as deletion evidence. S3 repair follow-up artifacts document the removal of the
remaining closeout compatibility parameter chain; fresh S3 review and
ship-verification evidence must certify those repaired inputs.

## Verification Verdict

Pending final S3 evidence refresh. The implementation waves, final verification
task, and wave-orchestration skill evidence were previously complete; the S3
repair follow-up touched required-skill evaluation and task scope artifacts, so
fresh review and ship-verification evidence must be recorded before this change
is done-ready. Current `validate` reports `scope_contract.status=pass`;
remaining blockers are stale or missing S3 review and ship-verification
evidence for the repaired inputs.

## Evidence Index

- Task evidence: `task-result-t-01.json` through `task-result-t-13.json`
- Wave skill evidence: `verification/wave-orchestration.yaml`
- Candidate inventory: `redundancy-candidates.md`
- Final tests and checks: `go test ./...`, `golangci-lint run ./...`, focused
  cleanup lint, `go run ./internal/toolgen/cmd/gen-surface-manifest --check`,
  and `git diff --check`
- Lifecycle proof: final `status`, `validate`, and `next --diagnostics`
  current-worktree outputs

## Requirement Coverage

| Requirement | Evidence |
| --- | --- |
| REQ-001 current-main cleanup boundary | `redundancy-candidates.md` is the candidate inventory; task results `t-01` through `t-12` each record current-worktree evidence and blockers are empty. |
| REQ-002 remove confirmed dead/test-held surfaces | `t-01` removed test-held wrappers/no-consumer helpers; `t-02` removed dead model/state/capability fields and methods. |
| REQ-003 remove retired compatibility wiring | `t-03` removed inert closeout conditional wiring, legacy handoff hygiene, and retired `S2_EXECUTE`/`S4_VERIFY` canonicalization while preserving fail-closed retired input rejection. |
| REQ-004 remove no-longer-emitted reason codes | `t-04` removed dead reason-code/remediation surfaces and synchronized recovery/reason tests. |
| REQ-005 resolve lint-confirmed cleanup | `t-05` resolved confirmed `unparam`, `staticcheck`, `ineffassign`, and `wastedassign` findings; it also records the intentional auto-skip `skill_evidence` suppression behavior repair with command-level coverage; focused cleanup lint is clean after the S3 repair follow-up. |
| REQ-006 verify lifecycle/generated surfaces | Final `go test ./...`, `golangci-lint run ./...`, focused cleanup lint, `git diff --check`, and surface manifest check pass after the S3 repair follow-up. Current lifecycle scope contract passes; final readiness waits on fresh S3 review and ship-verification evidence. |
| REQ-007 remove dead config/public no-op surfaces | `t-06` removed no-op validation config flags, retired public no-op `done --json` and `validate --json` flags, and preserved live `core`, `expanded`, and `custom` artifact schema behavior. |
| REQ-008 consolidate redundant implementations | `t-07` through `t-12` consolidated or proved live command route/freshness wiring, GitHub helpers, engine/artifact/cache/recovery helpers, S3 review template text, verification test helpers, and tiny binary root discovery. |

## Candidate Dispositions

| Candidates | Owning evidence | Disposition |
| --- | --- | --- |
| C-001, C-002 | `task-result-t-01.json` | Removed test-held wrappers and no-consumer internal API; focused tests and unused lint passed. |
| C-003 | `task-result-t-02.json` | Removed confirmed dead fields/methods in model/state/capability/skill surfaces, including the no-writer bundle-consistency warnings field. |
| C-004, C-005, C-006 | `task-result-t-03.json` | Removed closeout conditional wiring and retired-state compatibility; preserved explicit retired input rejection. Final full lint also removed two leftover unused `cmd/repair.go` helpers already owned by t-03. |
| C-007 | `task-result-t-04.json` | Removed no-longer-emitted reason codes/remediations and updated tests. |
| C-008, C-009 | `task-result-t-05.json` | Resolved lint-confirmed cleanup, covered the intentional auto-skip `skill_evidence` output repair, and consolidated duplicate route path-authority wiring. |
| C-010, C-011, C-012, C-013 | `task-result-t-06.json` | Removed dead config/state/public no-op surfaces. `ReviewIntentDriftFailures` was preserved as live digest-normalization input; live artifact schema behavior was preserved. |
| C-014 | `task-result-t-07.json` | Consolidated status route/freshness projection. `statusRoute` and `EvidenceFreshness` remain live public/private compatibility surfaces with synchronized ownership. |
| C-015 | `task-result-t-08.json` | Consolidated GitHub object/check-run pagination helpers, combined status extraction, and pagination parameter construction. HTTP and `gh` page walkers remain separate because they follow different current output contracts. |
| C-016, C-017, C-018, C-019 | `task-result-t-09.json` | Consolidated stale evidence blocker predicates, artifact source reads, strict YAML decode mechanics, and retained cache-specific sentinel contracts and reason-domain tests. |
| C-020 | `task-result-t-10.json` | Preserved template text where renderer constraints made partial extraction non-durable; added rendered-template coverage as the current-worktree proof. |
| C-021 | `task-result-t-11.json` | Consolidated verification test helper usage where safe and preserved coverage. |
| C-022 | `task-result-t-12.json` | Shared tiny-binary repository root discovery through `internal/fsutil`. |

## Intentional Breaking Retirements

`done --json` and `validate --json` are intentionally retired public no-op
flags. The current contract is that `done` and `validate` already emit JSON for
machine-readable output; passing `--json` is now invalid usage.

Retired workflow-state compatibility is intentionally removed. Change loading no
longer normalizes `S2_EXECUTE` or `S4_VERIFY` into current workflow states, and
source, tests, docs, and README surfaces no longer keep those retired workflow
state tokens as compatibility fixtures. They remain named only in this governed
artifact set as the explicit breaking retirement being documented.

Retired suite-result compatibility is intentionally fail-closed. The public
`slipway evidence suite-result` subcommand now returns invalid usage, and legacy
`suite-result.yaml` verification files are strict-decoded as unsupported
engine-owned verification records rather than silently skipped. The replacement
is `slipway evidence skill --skill ship-verification ...`.

## Verification Details

Current-worktree verification run after all code changes:

- `go test ./...`: pass
- `golangci-lint run ./...`: pass, `0 issues`
- `golangci-lint run --enable-only=unparam --tests=false ./...`: pass, `0 issues`
- `golangci-lint run --enable-only=staticcheck,ineffassign,wastedassign --tests=false ./...`: pass, `0 issues`
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check`: pass, `docs/SURFACE-MANIFEST.json is up to date`
- `git diff --check`: pass
- `go run . validate --change cleanup-stale-code-surfaces` after S3 repair follow-up: `scope_contract.status=pass`; remaining blockers are stale or missing S3 review and ship-verification evidence for the repaired inputs.
- `go run . status --json --change cleanup-stale-code-surfaces` after S3 repair follow-up: current state is `S3_REVIEW`; the next actionable skill is `spec-compliance-review`; selected review skills are spec-compliance, code-quality, independent, and security review.

## Residual Risks and Exceptions

No code, lint, manifest, task evidence, or scope-contract blocker remains after
the S3 repair follow-up. Lifecycle readiness still requires fresh S3 review and
ship-verification evidence for the repaired inputs.

Some apparent duplication is intentionally preserved with current-worktree
evidence: `statusRoute`, `EvidenceFreshness`, HTTP vs `gh` GitHub pagination
walking, cache-specific sentinel errors, and the S3 template text that cannot be
durably extracted with the current renderer behavior.

## Rollback Readiness

Rollback is ordinary source rollback of this change branch plus governed
artifact removal before archive. No irreversible data migration, external API
contract change, credential change, or production deployment was performed.
The intentional breaking CLI/lifecycle compatibility retirements are confined
to source and generated documentation/manifest surfaces in this worktree.

## Archive Decision

Not archived yet. Archive should happen only after fresh S3 review and
ship-verification evidence are recorded and `validate` reports fresh readiness
with no blockers.
