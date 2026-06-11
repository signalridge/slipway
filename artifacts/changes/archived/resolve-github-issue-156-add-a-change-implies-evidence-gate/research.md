# Research

## Alternatives Considered

- Extend scope-contract with sensitive categories.
  Rejected because scope-contract answers whether a changed file was planned,
  while issue #156 asks whether a sensitive changed file has category-specific
  owning proof.
- Require spec/code review evidence for sensitive files.
  Rejected because those review skills run in S3. Using them for the first proof
  would deadlock S2 execution before review can occur.
- Use execution-summary task evidence markers.
  Selected because `ExecutionTaskSummary` already carries `ChangedFiles`,
  `TargetFiles`, `TaskKind`, and `EvidenceRef`, and the public
  `slipway evidence task` surface can record stable category markers.
- Add a bypass flag or environment variable.
  Rejected because the change is in the `external_api_contracts` guardrail
  domain and must fail closed.

## Unknowns

None. The first rule set is intentionally narrow and can be expanded later with
configuration after the built-in contract is stable.

## Assumptions

- Execution summary changed files are the correct authority for post-execution
  readiness because they are already consumed by scope-contract evaluation.
- A marker in any passed task in the same execution summary can own the category
  proof. This permits a dedicated verification task to validate a sensitive
  implementation task.
- The initial categories should cover schema migrations, auth/authz, and API
  contracts. Broader matching would increase false positives without improving
  the issue #156 core contract.
- Recovery should point operators to `slipway evidence task` and explicit
  markers, not to private review notes or force paths.

## Canonical References

- `internal/model/execution_summary.go`: execution task summary fields consumed
  by readiness.
- `cmd/evidence.go`: public task evidence recording surface.
- `internal/engine/progression/readiness.go`: shared readiness evaluator used by
  validate, next, run, status, and review surfaces.
- `internal/engine/scopecontract/evaluate.go`: adjacent scope-contract changed
  file evaluation that this change complements.
- `internal/model/reason_code.go`: canonical reason taxonomy.
- `internal/model/recovery.go`: operator-facing remediation contract.
