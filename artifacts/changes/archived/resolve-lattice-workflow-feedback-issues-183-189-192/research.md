# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `cmd/evidence.go:138` reads `--notes-file` before recording skill evidence;
    `cmd/evidence.go:785` through `cmd/evidence.go:819` currently resolves the
    notes file against the project root.
  - `cmd/evidence.go:323` through `cmd/evidence.go:333` owns task-evidence
    wrong-state remediation.
  - `cmd/evidence.go:743` through `cmd/evidence.go:765` owns skill-evidence
    lifecycle-state validation and remediation.
  - `cmd/next.go:664` through `cmd/next.go:760` owns
    `confirmation_requirement` hard-stop action text and action kind.
  - `cmd/next_context_build.go:333` through `cmd/next_context_build.go:340`
    owns the `no_active_checkpoint` error when `--resume-response` is supplied
    without a real checkpoint.
- Dependency chains:
  - `makeEvidenceSkillCmd` -> `resolveActiveChangeRef` -> `loadActiveChange` ->
    `resolveEvidenceSkillNotes` -> `state.SaveVerification`.
  - `makeEvidenceTaskCmd` -> `loadActiveChange` -> S2 state check -> CLI error.
  - `buildNextView` -> `deriveConfirmationRequirement` -> JSON
    `confirmation_requirement`.
- Blast radius: command-layer behavior and tests in `cmd`; no model schema or
  lifecycle state transition change is required.
- Constraints:
  - `--notes-file` must remain workspace-relative and retain
    `validateEvidencePath` protections.
  - Post-review repairs must remain fail-closed through S3/S4 verification
    evidence, not S2 evidence mutation.
  - `resume_response_supported` must stay true only for active checkpoints.

### Patterns
- Existing conventions:
  - Worktree-bound change authority is resolved through
    `state.WorkspaceRootForChange` and `state.ResolveChangePaths`, not by
    guessing from root checkout paths.
  - User-facing command errors use stable `error_code` values with improved
    remediation text and `details` metadata.
  - Confirmation surface tests already assert `confirmation_requirement` fields
    in `cmd/progression_next_test.go`.
- Reusable abstractions:
  - `state.WorkspaceRootForChange` should be reused for the notes-file base.
  - `progression.S4VerificationRecoveryRemediation()` already centralizes S4
    "rerun goal-verification, then rerun final-closeout" wording.
- Convention deviations: none required. The fix can be additive and local.

### Risks
- Technical risks:
  - Medium: changing notes-file base can affect root-checkout invocations that
    use `--change` for a worktree-bound change. Mitigation: use the active
    change's authoritative workspace, which is the same path authority already
    exposed by `next/status`.
  - Low: remediation text updates can affect brittle tests that assert exact
    prose. Mitigation: keep error codes stable and update focused assertions.
  - Low: making skill-handoff guidance more explicit could lengthen JSON
    strings. Mitigation: keep field names and action kind stable.
- Guardrail domains: external API contracts, because CLI JSON and errors are
  automation surfaces.
- Reversibility: the change is reversible by reverting command-layer helper and
  test edits; no persisted data migration is involved.

### Test Strategy
- Existing coverage:
  - `cmd/evidence_skill_test.go:18` through `cmd/evidence_skill_test.go:84`
    covers `--notes-file` on a root-bound change.
  - `cmd/evidence_test.go:25` through `cmd/evidence_test.go:50` covers S4 task
    evidence wrong-state remediation.
  - `cmd/evidence_skill_test.go:291` through `cmd/evidence_skill_test.go:313`
    covers generic wrong-state skill evidence.
  - `cmd/progression_next_test.go:1702` through
    `cmd/progression_next_test.go:1722` covers `--resume-response` without a
    checkpoint.
  - `cmd/progression_next_test.go:2069` through
    `cmd/progression_next_test.go:2167` covers structured
    `confirmation_requirement`.
- Infrastructure needs: command tests using existing helpers
  `initGitRepoForWorktreeTests`, `runGit`, `state.PersistScopeWorktreeMetadata`,
  and `state.RelocateGovernedBundle`.
- Verification approach:
  - Add #183 regression proving relative notes read from the bound worktree.
  - Add #192 regression for S3 task wrong-state and S3 `wave-orchestration`
    wrong-state remediation.
  - Add #189 assertions for skill-handoff action text and
    `no_active_checkpoint` remediation.
  - Run focused `go test ./cmd`, then broader `go test ./...`.

### Options
- Option A: narrow CLI-surface repair.
  - Use authoritative workspace resolution for notes-file.
  - Improve remediation/action text without adding new lifecycle machinery.
  - Tradeoff: does not add a first-class post-review evidence refresh command,
    but solves the current confusing and misleading surfaces.
- Option B: add a new post-review evidence refresh command.
  - Tradeoff: could model review-driven repairs more directly, but it expands
    lifecycle semantics and is beyond the reported immediate gaps.
- Option C: accept delegated autonomy as a replacement for fresh intake
  confirmation.
  - Tradeoff: could reduce pauses, but weakens a hard-stop confirmation control
    and contradicts the fail-closed scope boundary.
- Selected: Option A. It fixes all three current Lattice-reported symptoms while
  preserving existing lifecycle gates and avoiding bypass semantics.

## Unknowns
- Resolved: Is the current codebase map relevant? -> No. The populated map is
  explicitly re-authored for issue #184 in `artifacts/codebase/ARCHITECTURE.md:3`
  through `artifacts/codebase/ARCHITECTURE.md:5` and
  `artifacts/codebase/TESTING.md:3` through `artifacts/codebase/TESTING.md:5`,
  so this research re-derived the current seams from source.
- Resolved: Does #183 reproduce in this worktree? -> Yes. The intake evidence
  command using `--notes-file artifacts/...` failed by opening the root checkout
  path, matching the issue report; the `.worktrees/<slug>/artifacts/...`
  workaround succeeded.
- Remaining: None.

## Assumptions
- The three Lattice issues are safe to resolve together because they touch the
  same command-layer governance surfaces and no implementation requires a schema
  migration or new lifecycle state. Evidence: affected files are concentrated in
  `cmd/evidence.go`, `cmd/next.go`, and `cmd/next_context_build.go`.
- Existing error codes should remain stable for automation. Evidence: current
  tests assert codes like `evidence_skill_wrong_state`,
  `evidence_task_wrong_state`, and `no_active_checkpoint`.
- Post-review repairs should be represented by S3/S4 skill evidence, not by
  refreshing S2 task evidence after S2. Evidence:
  `internal/engine/progression/authority.go:214` already routes S4 recovery to
  goal-verification and final-closeout.

## Canonical References
- `cmd/evidence.go:138`
- `cmd/evidence.go:323`
- `cmd/evidence.go:743`
- `cmd/evidence.go:785`
- `cmd/next.go:664`
- `cmd/next.go:718`
- `cmd/next_context_build.go:333`
- `internal/engine/progression/authority.go:214`
- `cmd/evidence_skill_test.go:18`
- `cmd/evidence_skill_test.go:291`
- `cmd/evidence_test.go:25`
- `cmd/progression_next_test.go:1702`
- `cmd/progression_next_test.go:2069`
- `artifacts/codebase/ARCHITECTURE.md:3`
- `artifacts/codebase/TESTING.md:3`
