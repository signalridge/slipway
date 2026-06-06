# Assurance
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine; cmd packages stay thin;
  model remains a leaf; one governed verification YAML per skill.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
This change delivers Recovery UX P2 (#98) for the normal governed lifecycle. The
implemented behavior replaces the old S3/S4 planning-only recovery shape with a
general stale-evidence recovery primitive: `slipway run` detects the earliest
stale governed authority that has been reached, reopens that stage, clears that
stage and downstream verification/generated evidence, preserves compatible
runtime task evidence, and routes the existing owning skill. The change also
removes the normal-path `evidence restamp` command and restamp/Tier recovery
guidance, keeps stale sensitive-domain evidence on rerun/review paths, ports the
#89 structural/scope task-plan hash split, preserves runtime task evidence
through pivot cleanup, and aligns read-only/mutating recovery JSON surfaces.

The change is classified as `external_api_contracts` because it intentionally
changes public Slipway CLI/JSON recovery vocabulary and generated host-tool
guidance. The externally relevant JSON object shape remains stable:
`recovery.primary_command`, `primary_action`, `recovery_class`, and `steps[]`
continue to render; the intentional vocabulary moves from
`stale_planning_recovery_available`/restamp-oriented remediation to
`stale_evidence_recovery_available` plus `slipway run`.

## Verification Verdict
Current verdict: pass for implementation, tests, review, and governed readiness
up to the S4 goal/final-closeout handoff. The active S4 validation immediately
before this assurance refresh reports fresh evidence and only the expected
missing `goal-verification`, `final-closeout`, high-risk baseline, and closeout
assurance tokens. Those tokens are supplied by the S4 verification records, not
by editing engine-owned digests.

## Evidence Index
- Runtime execution summary:
  `verification/execution-summary.yaml` records run_summary_version 1 with all
  fourteen planned tasks completed.
- Fresh final proof:
  `verification/final-proof.yaml` records `go build ./...`, `go vet ./...`,
  `go test ./...`, `go test ./internal/toolgen/...`, the focused freshness
  guard test, `go run . init --refresh --tools all`, `git diff --check`,
  retired-vocabulary scan, active `go run . validate --json`, active
  `go run . health --governance --json`, and targeted cross-package coverage.
- S3 reviews:
  `verification/spec-compliance-review.yaml` includes `layer:R3=pass` and
  confirms requirement-to-code alignment. `verification/code-quality-review.yaml`
  includes `layer:IR1=pass`, `layer:IR3=pass`, and `toolchain_compat:pass`.
- Self-dogfood:
  `verification/early-self-bootstrap.yaml` and `verification/final-proof.yaml`
  record black-box use of the current worktree `go run .` lifecycle surfaces.
  During the change, public `run --json --diagnostics` recovered S2->S1/audit
  and S3->S2 without manual digest/timestamp edits or command guessing.
- Coverage:
  `/tmp/slipway-recovery-ux-p2-coverpkg.out` was produced by a cross-package
  `go test` coverage run over the changed CLI/progression/wave/model/state
  packages. `go tool cover -func` reports total targeted-package coverage of
  80.6%, with the key recovery/hash/digest functions covered directly.

## Requirement Coverage
- REQ-001: implemented by
  `internal/engine/progression/stale_evidence_recovery.go` and
  `internal/engine/progression/advance_governed.go`; covered by S0/S1/S2/S3/S4
  stale recovery tests and dogfooded through the active change.
- REQ-002: implemented by
  `internal/model/evidence_digests.go` and
  `internal/engine/progression/evidence_digests.go`; verified by content-hash,
  deleted-file, directory-target, and no-mtime freshness tests.
- REQ-003: implemented through registry/lifecycle/substep authority ordering in
  `stale_evidence_recovery.go`; S1 validate ordering has a dedicated regression.
- REQ-004: implemented by `internal/engine/wave/parse.go`,
  `internal/state/wave_execution.go`, and
  `internal/engine/progression/wave_sync.go`; target_files-only edits rebuild
  compatible wave-plan evidence while structural task contract edits stale task
  evidence.
- REQ-005: implemented in `cmd/pivot_execution.go`; pivot cleanup preserves
  runtime task evidence and removes only derived/runtime residue.
- REQ-006: implemented in `internal/state/execution_repair.go` and wave sync;
  generated wave-plan/execution-summary files are rebuilt or route to re-walk
  instead of being accepted because they are readable.
- REQ-007/REQ-011/REQ-015: verified by the removed restamp command
  registration, negative command test, repair reroute tests, generated refresh,
  and retired-vocabulary scan.
- REQ-008/REQ-012: verified by next/validate/status/CLIError recovery contract
  tests and S3 external-contract review tokens.
- REQ-009: verified by stale-digest recovery routing through `slipway run` and
  absence of restamp/force-close bypasses for guardrail-domain evidence.
- REQ-010: verified by certified-input coverage tests for governed artifacts and
  downstream authorities.
- REQ-013/REQ-014: verified by black-box agent guidance in CLAUDE.md, generated
  surface refresh, and active lifecycle dogfood with no manual digest edits.

## Residual Risks and Exceptions
- Deliberate freshness fail-open (not a defect): a passing skill with no stored
  digest is treated as fresh (`evidence_digests.go`,
  `skillDigestFreshnessBlockers`), and `input_digest_unavailable` /
  `input_digest_missing` are excluded from automatic re-walk triggers
  (`stale_evidence_recovery.go`, `staleEvidenceRecoveryDetail`). This is the
  chosen "fail-open over fail-closed deadlock" boundary that eliminates the
  #90/#81 stale-evidence dead-ends: stamp-on-accept writes a digest on the next
  advance, so the open window is bounded and self-healing. It does NOT weaken
  guardrail protection — high-risk/domain-review/rollback gates live in
  `internal/engine/gate/gate.go` and are evaluated independently of digest
  freshness, so a sensitive-domain change still fails closed to rerun/review. A
  future agent should treat a no-stored-digest "fresh" reading as intended, not
  as a bug to "harden" (hardening would reintroduce the deadlock #98 removes).
- This is a broad governance-kernel change. Residual risk is concentrated in
  host tooling that consumed the old `stale_planning_recovery_available` token
  or restamp guidance. The intentional token/remediation change is documented
  and guarded by contract tests; rollback is a branch revert.
- The codebase map remains partial and advisory. Source files, governed
  artifacts, runtime evidence, and live `go run .` outputs are the closeout
  authorities for this change.
- Cross-package coverage is diagnostic rather than proof. It shows the key
  changed recovery/hash/freshness functions exercised, while behavioral
  correctness remains grounded in focused regression tests and lifecycle
  dogfood.

## Rollback Readiness
Rollback is branch revert. No durable user data migration is part of this
change. Engine-owned verification/runtime evidence is regenerable by rerunning
the owning governed stage. If reverted, public docs/generated skills must revert
with code so host tools no longer see the generalized recovery vocabulary while
the old implementation is active.

## Archive Decision
Proceed to done-ready after S4 `goal-verification` and `final-closeout` pass,
including `check:external_api_contracts.safety_baseline=pass`,
`closeout:guardrail_baseline=pass`, and
`closeout:assurance_complete=pass`. The active `go run . validate --json` proof
captured before this assurance refresh reported fresh evidence and only those
expected S4 closeout blockers. No `slipway done` archive command should run
until the active worktree reports done-ready after final closeout.
