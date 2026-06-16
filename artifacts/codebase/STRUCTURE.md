# Structure

Re-authored for change
`add-an-engine-consumed-context-origin-fresh-context-attestat`.

- `internal/engine/progression/`
  - `authority.go`: review + ship authority gates. `buildShipAuthorityFromReadiness`
    assembles the closeout blockers. Pattern B `closeoutGoalVerificationReuseBlockers`
    holds the opt-in reuse chain; `closeoutGoalVerificationReuseReviewBlocker` is the
    existing review>=goal ordering proof to promote in P1 to an ALWAYS-ON
    closeout>=goal>=max(spec-compliance-review,code-quality-review) gate (preset-sensitive
    severity, NEW reason code). Pattern A presence attestation lives in
    `closeoutAssuranceAttestationBlockers`.
  - `wave_sync.go`: wave-dispatch gates P2 reuses for the review pair. `DispatchEvidenceBlockers`
    (parallel dispatch_mode) and `ExecutorAgentBlockers` (distinct executor handles) are the
    analogues; `session_isolation_warning` token is emitted around line 485; `WaveSyncRun`
    carries `TaskKind`/`TargetFiles`/`SessionID` fields (P4 bases #5 on `task_kind`+`target_files`,
    demotes `session_id`). `degraded_sequential` acceptance (P4 #6 tighten) sits in the
    dispatch/executor comments ~:813-872.
  - `authority_test.go`, `wave_sync_test.go`, `issue227_wave_boundary_test.go`: gate behavior
    coverage to extend for the new ordering + force-parallel-review gates.
- `internal/model/` (pure layer; no cmd/tmpl/toolgen imports)
  - `verification.go`: `VerificationRecord`; `References []string` is the SOLE host-controlled
    inbound channel (Timestamp + RunVersion engine-owned).
  - `reason_code.go`: `canonicalReasonDefinitions` registry; `NewReasonCode` silently downgrades
    unregistered codes (register P1/P2 new codes here first).
  - `recovery.go`: `blockerRemediations` map (third leg of the reason-code contract).
  - `reason_code_contract_test.go`: `TestCanonicalReasonCodeTaxonomySnapshot` +
    `canonicalReasonCodeSnapshot`/`canonicalReasonSeveritySnapshot` frozen lists.
  - `recovery_test.go`: remediation coverage.
- `internal/engine/skill/`
  - `skill.go`: skill registry; `RunSummaryBound:true` on spec-compliance-review,
    code-quality-review, goal-verification, final-closeout (identical RunVersion -> P3 nonce
    discrimination infeasible).
- `cmd/`
  - `evidence.go`: the producer. `makeEvidenceSkillCmd` ("evidence skill") stamps
    engine-owned `Timestamp`/`RunVersion` and passes host `References`; `evidenceSkillRunContext`
    resolves the run version.
- `internal/tmpl/templates/skills/`
  - `final-closeout/SKILL.md.tmpl`: emits the dead `closeout:reviewer_independence=pass` token
    (:136) that P1 gives a real consumer.
  - `wave-orchestration/SKILL.md.tmpl` (+ `references/executor-dispatch-reference.md`): the sole
    surface that documents the review/dispatch tokens P2 reuses
    (`dispatch_mode:wave=<wave_index>:degraded_sequential`,
    `executor_agent:wave=<wave_index>:task=<task_id>:<handle>`).
  - `spec-compliance-review/`, `code-quality-review/`, `goal-verification/` `SKILL.md.tmpl`: the
    review/verify skill surfaces P2 must extend to require force-parallel review dispatch (two
    distinct executor handles for the spec-compliance / code-quality PAIR); they do NOT yet carry
    any dispatch token.
- `internal/tmpl/`
  - `templates_test.go`: token + frontmatter contract tests
    (`TestFinalCloseoutTemplateRequiresAssuranceAttestationOnStandardStrict`).
- `internal/toolgen/`
  - `toolgen_test.go`: generated-skill token contracts (dispatch_mode :1227, executor_agent
    :1251) and Arguments contract (`TestCodexCommandSkillsUseCommandRegistryArguments`).
- `internal/architecture/`
  - `dependency_direction_test.go`: `TestAuthorityPackagesDoNotImportSurfaceRenderers` forbids
    internal/model + internal/state importing cmd/tmpl/toolgen (new gates -> progression, vocab
    -> model).
- `artifacts/changes/add-an-engine-consumed-context-origin-fresh-context-attestat/`
  - `intent.md`, `research.md`: authored (locked Option A hybrid; P3 infeasibility residual).
  - `requirements.md`, `decision.md`, `tasks.md`, `assurance.md`: plan bundle to author after
    research evidence is recorded.
