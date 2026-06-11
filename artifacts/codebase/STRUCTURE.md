# Structure

Re-authored for change `resolve-github-issue-156-add-a-change-implies-evidence-gate`
(GitHub issue #156).

- `internal/engine/sensitiveevidence/`
  - `evaluate.go`: focused evaluator for sensitive changed-file categories,
    marker extraction, and `sensitive_evidence_missing` blockers.
  - `evaluate_test.go`: category, marker, separate-verification, and no-bypass
    regressions.
- `internal/engine/progression/`
  - `readiness.go`: read-only governance readiness aggregation, now exposing
    `SensitiveEvidence` and appending sensitive-evidence blockers after
    execution-summary readiness.
  - `advance_governed.go`: mutating lifecycle transition path, now blocking in
    S2_EXECUTE or reopening later stages to S2 when sensitive evidence is
    missing.
  - `stale_evidence_recovery.go`: recovery target construction for stale and
    S2-owned evidence failures.
  - `readiness_test.go` and `scope_contract_gate_test.go`: integration
    regressions for readiness, S2 blocking, and S3-to-S2 reopen behavior.
- `internal/model/`
  - `reason_code.go`: canonical reason-code taxonomy entry.
  - `recovery.go`: operator recovery mapping to `slipway run`, with
    evidence-task marker guidance once the workflow is in S2.
  - Contract tests pin canonical reason-code and recovery behavior.
- `internal/state/`
  - `verification.go`: load and save verified skill records from the
    authoritative governed bundle.
  - `verification_test.go`: persistence regressions for validated
    `SaveVerification` records.
- `cmd/`
  - `evidence.go`: public evidence command surface, now including
    `evidence task` for runtime task evidence and `evidence skill` for
    governance skill verification.
  - `evidence_skill_test.go`: Cobra-level regressions for plan-audit recording,
    wrong-state rejection, run-summary-bound skill rejection, and notes-source
    conflict handling.
  - `template_flag_contract_test.go`: generated command reference to Cobra flag
    drift check covering both `evidence` subcommands.
- `internal/toolgen/` and `internal/tmpl/templates/_partials/`
  - `toolgen.go` and `command-evidence-body.tmpl`: adapter-facing command
    metadata and prompt body for `slipway evidence task` plus
    `slipway evidence skill`.
- `artifacts/changes/resolve-github-issue-156-add-a-change-implies-evidence-gate/`
  - Governed artifact bundle and verification evidence for this change.
