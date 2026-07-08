# Concerns

## Risks
- Changing recovery priority can alter the primary command hosts follow in blocked states. Mitigation: focused `internal/model/recovery_test.go` cases lock ordering for task evidence before wave evidence and S3 convergence before stale ship evidence.
- Exposing wave projections in read-only surfaces can make payloads noisy. Mitigation: `next` only attaches S3 wave plans when convergence drift is present; `validate` is diagnostic by design.
- Exempting scratch paths from scope-contract drift could hide real source changes if the exemption is too broad. Mitigation: limit the documented exemption to `.slipway-tmp/**` and keep it git-ignored.

## Sensitive Domains
- This change does not alter auth, credentials, schema migrations, irreversible operations, or external API contracts.
- It does alter governance/evidence surfaces, so tests must demonstrate that evidence writes remain fail-closed and that the destructive reexecution path is guarded rather than silently broadened.

## Deferred Concerns
- Automatic promotion of prior-version task evidence after intentional reexecution is a larger evidence-ledger policy decision and remains out of scope for this fix.
- Guardrail-domain confirmation for evidence writes remains a candidate follow-up because it requires a broader attestation contract.
