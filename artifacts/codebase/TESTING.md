# Testing

Re-authored for change `resolve-github-issue-156-add-a-change-implies-evidence-gate`
(GitHub issue #156).

- Existing coverage:
  - `internal/engine/scopecontract/evaluate_test.go:14` proves changed files
    inside planned targets pass.
  - `internal/engine/scopecontract/evaluate_test.go:40` proves out-of-scope
    changed files block deterministically.
  - `internal/engine/scopecontract/evaluate_test.go:91` proves missing
    changed-file evidence is already a blocker.
  - `internal/model/reason_code_contract_test.go:22` keeps canonical reason
    code coverage stable.
  - `internal/model/recovery_test.go:91` catches recovery-relevant reason codes
    without remediation.
- Gaps for issue #156:
  - No test asserts that a sensitive changed file requires category-specific
    owning evidence.
  - No test asserts that matching owning evidence clears the sensitive blocker.
  - No test asserts that sensitive blockers have precise remediation and no
    bypass path.
  - No test asserts that generated host verification can be recorded through the
    public `slipway evidence skill` command.
- Planned verification:
  - Add table-driven unit tests for a sensitive-evidence evaluator covering
    schema migration, auth/authz, and API contract categories.
  - Add progression tests for mutating lifecycle enforcement: block in
    S2_EXECUTE when marker evidence is missing, reopen S3_REVIEW to S2_EXECUTE,
    and pass when the category marker exists.
  - Add reason-code and recovery-contract assertions for the new blocker.
  - Add Cobra command tests for `slipway evidence skill`, including plan-audit
    recording, wrong-state rejection, run-summary-bound skill rejection, and
    notes-source conflict handling.
  - Add state persistence coverage for `state.SaveVerification`.
  - Extend command-template flag coverage so the generated `evidence` command
    reference includes the new `evidence skill` flags.
  - Run the targeted package tests first, then broader `go test ./internal/...`
    and governed `validate --json` after implementation.
