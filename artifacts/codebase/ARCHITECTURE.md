# Architecture

Re-authored for change `resolve-github-issue-156-add-a-change-implies-evidence-gate`
(GitHub issue #156).

Question: where should "sensitive file changed implies owning evidence" be
checked so it complements freshness and scope-contract readiness without
creating a bypass path?

- Affected modules:
  - `internal/engine/progression/readiness.go:184` resolves the bound governed
    bundle path before computing readiness.
  - `internal/engine/progression/readiness.go:211` loads passing required skill
    verification records.
  - `internal/engine/progression/readiness.go:275` only invokes the current
    scope-contract gate after an execution summary exists.
  - `internal/engine/progression/advance_governed.go:146` enforces the same
    sensitive-evidence result on mutating advancement so S2 cannot advance past
    missing sensitive proof and later stages reopen to S2 for repair.
  - `internal/engine/progression/stale_evidence_recovery.go:477` builds the
    S2 recovery target for sensitive-evidence failures.
  - `internal/engine/scopecontract/evaluate.go:41` evaluates planned targets
    versus changed files but has no sensitive-evidence category logic.
  - `internal/engine/sensitiveevidence/evaluate.go:49` classifies sensitive
    changed files and checks passed task evidence references for category
    markers.
  - `internal/state/verification.go` owns validated skill-verification record
    persistence under the authoritative governed bundle.
  - `internal/model/execution_summary.go:28` stores task evidence fields,
    including `ChangedFiles`, `TargetFiles`, `EvidenceRef`, and `TaskKind`.
  - `cmd/evidence.go` writes runtime task evidence from
    `slipway evidence task` and governance skill verification from
    `slipway evidence skill`.
  - `internal/toolgen/toolgen.go` and
    `internal/tmpl/templates/_partials/command-evidence-body.tmpl` publish the
    same `evidence task` and `evidence skill` surfaces to generated command
    references and prompts.
- Dependency flow:
  - `slipway evidence task` records runtime task evidence.
  - Runtime task evidence materializes into `verification/execution-summary.yaml`.
  - Host verification evidence is recorded through `slipway evidence skill`,
    which writes `verification/<skill>.yaml` and records the change evidence
    reference without advancing lifecycle state.
  - `EvaluateGovernanceReadiness` consumes that summary, existing skill
    verifications, and artifact state to produce blockers for `status`,
    `validate`, `next`, `run`, and review surfaces.
  - `AdvanceGoverned` reuses the same evaluator before normal state transition,
    preserving the read-only and mutating contract.
- Architectural boundary:
  - Scope-contract should continue to own planned-target and changed-file drift.
  - Sensitive evidence should be a sibling readiness evaluator that reuses the
    same execution-summary changed files but reports its own reason code and
    remediation.
  - Generated host instructions must record skill verification through public
    CLI commands, not hand-edited verification YAML.
- Blast radius:
  - Runtime readiness and reason-code/remediation contracts.
  - Public evidence command surface for skill verification.
  - Generated adapter command metadata for the `evidence` command.
  - Unit tests in a new focused evaluator package plus a narrow progression
    integration test if wiring requires it.
