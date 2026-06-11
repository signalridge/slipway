# Concerns

Re-authored for change `resolve-github-issue-156-add-a-change-implies-evidence-gate`
(GitHub issue #156).

- Stale map risk: populated codebase-map documents only help when their seams,
  blast radius, and risks match the active change; stale map content is not
  planning authority for this change.
- Load-bearing invariant: freshness proves "when" evidence was produced, while
  issue #156 requires proving that the right owning evidence exists for touched
  sensitive files. The new gate must not replace existing freshness or
  scope-contract checks.
- Guardrail risk: sensitive-domain checks must fail closed. A missing marker for
  migration-applied, auth-review, or contract-test evidence should block with a
  canonical error reason and precise remediation.
- Deadlock risk: requiring review-stage skill evidence during S2 execution could
  strand the lifecycle before review can run. The first implementation should
  use execution-summary task evidence markers, not post-review skill evidence,
  for the change-implies-evidence proof.
- Lifecycle bypass risk: read-only readiness alone is insufficient because
  mutating advancement can otherwise move S2 evidence into review. The gate must
  run inside `AdvanceGoverned` before normal state transition, reopening S3/S4
  to S2 when repair requires `slipway evidence task`.
- Compatibility risk: broad filename patterns can overmatch ordinary files. Keep
  initial built-in patterns explicit and test category examples rather than
  attempting a complete ecosystem catalog.
- Recovery risk: new readiness blockers need entries in the canonical reason
  code and recovery-remediation maps. The public recovery command must be
  `slipway run`, because `slipway evidence task` is only legal from S2_EXECUTE.
- Public-surface dead-end risk: generated host instructions require
  `slipway evidence skill` to record verification evidence. If the command is
  missing or accepts evidence in the wrong lifecycle state, agents either
  hand-edit engine-owned YAML or get stranded before plan-audit can complete.
