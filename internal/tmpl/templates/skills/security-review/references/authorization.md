# Authorization — secure defaults

## Default posture
- **Deny by default.** Access is granted only when a policy check
  succeeds. Absence of a deny is not a grant.
- Authorization is enforced **at the resource boundary** (route
  handler, RPC entrypoint, DB gateway), not at the UI layer.
- Every protected operation resolves a single explicit policy
  evaluation. "Implied from session presence" is not authorization.

## Model selection
| model | use when | watch-out |
|-------|----------|-----------|
| RBAC | roles map cleanly to responsibility | role explosion when roles encode data attributes |
| ABAC | access depends on request attributes | policy language becomes the load-bearing surface |
| ReBAC | access derives from object graphs (docs, orgs, teams) | graph traversal must be bounded |

Most systems are hybrid: RBAC at the surface for coarse gating,
ABAC/ReBAC at the resource for fine-grained checks.

## IDOR and resource ownership
- Object identifiers exposed to clients must be authorized on every
  access, not only on creation or listing.
- Queries must include the caller's scope in the WHERE clause:
  `SELECT ... FROM docs WHERE id = ? AND owner_id = ?`.
- Do not rely on "unguessable UUIDs"; treat them as identifiers, not
  secrets.

## Privileged operations
- Require step-up authentication (fresh MFA or re-auth) for:
  - Changing primary email / password / MFA
  - Creating or rotating API keys
  - Changing billing or disabling security features
  - Admin impersonation of another user
- Record a tamper-evident audit row: actor, target, operation,
  justification, timestamp, request id.

## Cross-tenant isolation
- A policy bug in a multi-tenant system that leaks data across tenants
  is a blocker, not a major finding.
- Tenant id must be authenticated (bound to the session), not accepted
  from the request body. Derive from principal, not from payload.
- Background jobs inherit the originating principal's tenant; fan-out
  workers must not drop the scope.

## Authorization tests
- Positive tests: each role can do what it's supposed to.
- Negative tests: each role is denied everything else.
- Horizontal tests: user A cannot access user B's resources.
- Vertical tests: non-admin cannot reach admin operations even via
  direct URL / API path.
- Regression tests on every route-level authorization change;
  route-level authz is brittle against refactors.

## Delegation and impersonation
- Admin-as-user (impersonation) is always opt-in, time-bounded, and
  logged separately from the admin's own actions.
- Service-to-service credentials scope to the minimum operations
  needed; no shared "god" service accounts across domains.

## Review cues
- Any `if user.is_authenticated` gate with no role / resource check
  is a blocker for privileged operations.
- `fetch(id).owner == current_user` pattern: acceptable if enforced
  in a single chokepoint; suspicious if repeated inline across
  handlers.
- Middleware-only authorization with per-route exceptions is fragile;
  flag any exception list as a maintenance hazard.

## Anti-patterns
- "We'll authorize in the front end."
- "The UI doesn't show that button, so only admins can hit it."
- Authorization decisions inferred from HTTP method (GET vs POST).
- Global `is_superuser` short-circuits that bypass resource scoping.
