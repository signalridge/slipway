# Assurance

## Scope Summary
This change fixes stale generated command skills by changing adapter refresh
cleanup from name-list matching to generated command-skill content recognition.
Retired `slipway-<command>` skill directories are recognized by generated
frontmatter, `surface: skill`, matching `command_id`, absence from the live
command registry, and the authoritative generated CLI footer.

The implementation is limited to generator cleanup and regression coverage:
`internal/toolgen/toolgen.go`, `internal/toolgen/toolgen_test.go`, and
`cmd/template_flag_contract_test.go`. It does not reintroduce retired commands
or change command execution semantics.

## Verification Verdict
Implementation-wave verification passed for the planned scope. The focused
adapter, command-surface, and manifest checks all passed against the current
worktree before S3 review.

S3 review and terminal ship verification passed on run version 3. The selected
peer reviews all recorded fresh passing evidence:
spec-compliance-review, code-quality-review, and independent-review. Terminal
ship-verification then recorded `verdict: pass` with the authoritative full
suite, lint, manifest check, diff check, evidence-freshness attestation,
reviewer-independence attestation, and assurance-completeness attestation.

## Evidence Index
- `artifacts/changes/fix-stale-command-skills/verification/wave-orchestration-notes.md`
  records root cause, implementation decisions, and task-level evidence.
- `artifacts/changes/fix-stale-command-skills/task-results/t-01.json` records
  regression-test evidence for retired command-skill cleanup and live command ID
  resolution.
- `artifacts/changes/fix-stale-command-skills/task-results/t-02.json` records
  implementation evidence for content-signature cleanup and fail-closed
  deletion behavior.
- `artifacts/changes/fix-stale-command-skills/task-results/t-03.json` records
  focused verification evidence.
- `artifacts/changes/fix-stale-command-skills/verification/wave-orchestration.yaml`
  records the wave-orchestration skill verdict.
- `artifacts/changes/fix-stale-command-skills/verification/spec-compliance-review.yaml`
  records fresh run-version-3 spec compliance evidence.
- `artifacts/changes/fix-stale-command-skills/verification/code-quality-review.yaml`
  records fresh run-version-3 implementation quality evidence.
- `artifacts/changes/fix-stale-command-skills/verification/independent-review.yaml`
  records fresh run-version-3 independent review evidence.
- `artifacts/changes/fix-stale-command-skills/verification/ship-verification.yaml`
  records terminal ship verification and closeout attestations.
- `artifacts/changes/fix-stale-command-skills/verification/logs/ship-suite.txt`
  records the authoritative `go test ./...` proof.
- Focused verification commands:
  - `go test ./internal/toolgen -run 'TestGenerateRefreshPrunesRetiredCommandSkillDirsByContent|TestGenerateRefreshPreservesUnknownCleanupTargetsAndRefusesManagedModified|TestGenerateRefreshWithoutOwnershipManifestBootstrapsIntoTrackedState|TestGeneratedAdapterSurfacesStayInSyncWithRegistry|TestCodexCommandSkills|TestCodexCommandSkillsUseCommandRegistryArguments|TestCodexCommandSkillsIncludeTierAndSurface' -count=1`
  - `go test ./cmd -run 'TestGeneratedCommandSkillIDsResolveOnLiveRootCommands|TestTemplateFlagsMatchCobraCommands|TestCobraFlagsCoveredByRegistryArguments|TestGeneratedCommandEntriesExposeChangeSelectorForSupportedCommands' -count=1`
  - `go test ./internal/toolgen/cmd/gen-surface-manifest -count=1`
  - `go run ./internal/toolgen/cmd/gen-surface-manifest --check`
- Terminal ship verification commands:
  - `go test ./...`
  - `golangci-lint run ./...`
  - `go run ./internal/toolgen/cmd/gen-surface-manifest --check`
  - `git diff --check`

## Requirement Coverage
- REQ-001 is covered by `cleanupStaleSkillDirs` and
  `TestGenerateRefreshPrunesRetiredCommandSkillDirsByContent`, including
  manifest-absent residue, manifest-tracked residue, and a synthetic retired
  command ID that was never enumerated in the reported stale set.
- REQ-002 is covered by post-refresh disk-tree assertions in
  `TestGenerateRefreshPrunesRetiredCommandSkillDirsByContent` and by
  `TestGeneratedCommandSkillIDsResolveOnLiveRootCommands`, which resolves
  generated command-skill `command_id` values through `newRootCmd().Commands()`
  instead of through the generator registry slice.
- REQ-003 is covered by tests preserving user-owned adjacent `slipway-*` skills,
  preserving manifest-absent user-modified generated-shape content, and refusing
  manifest-tracked user-modified managed content through the existing
  `managed-modified` fail-closed path.
- REQ-004 is covered by table-driven command-skill host coverage for every host
  with `CommandSkillSurface` enabled: codex, kiro, and qwen.

## Residual Risks and Exceptions
Other retired generated-surface families, such as governance, technique, catalog
host skills, or nested command entries, may share the same broad registry-coupled
cleanup pattern. They were documented as out of scope and were not changed here.

`docs/SURFACE-MANIFEST.json` is part of verification scope as a generator/docs
drift signal. The manifest check reported it is current; the file itself did not
need content changes.

The codebase map was treated as advisory because live lifecycle output reported
it may be stale for this change. Implementation and review authority stayed with
the bound worktree, current governed artifacts, and current source files.

## Rollback Readiness
Rollback is a normal source revert of the generator and tests touched by this
change, plus the governed artifact bundle if the governed change is abandoned.
There is no data migration, external API change, persisted state conversion, or
irreversible operation.

If rollback is needed after merge, revert the source changes to
`internal/toolgen/toolgen.go`, `internal/toolgen/toolgen_test.go`, and
`cmd/template_flag_contract_test.go`, then rerun the adapter and command-surface
test subset before shipping the revert.

## Archive Decision
Ready to archive. Active `validate --json` from the bound worktree reports
fresh evidence, no blockers, and approved `G_ship` before `done`. The archive is
appropriate because every requirement has implementation, test coverage, fresh
selected review evidence, terminal ship verification, and rollback guidance.

After `done`, treat the archived bundle as a frozen record. Any later
verification should run against the source branch or a new active governed
change rather than describing the archived bundle as revalidated through the
active validate gate.
