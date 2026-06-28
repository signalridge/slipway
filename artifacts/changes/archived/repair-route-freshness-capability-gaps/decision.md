# Decision

## Alternatives Considered
### Option A: Keep route data only on successful views
This preserves current behavior but keeps agents parsing remediation prose on precondition failures.

### Option B: Add top-level `invocation_route` to CLIError and diagnostics views
This keeps existing error codes and details intact while adding the same structured route contract to non-success paths.

### Option C: Move all route resolution into a larger workspace index refactor
This could reduce future duplication but would mix correctness repair with a broader performance architecture change.

## Selected Approach
Use Option B for this change. Add route builders for diagnostic/error route kinds, add split freshness to `next`/`done`, split status text freshness prose, and move host capability requirements into registry/template metadata.

## Interfaces and Data Flow
- `CLIError` gains optional `invocation_route`.
- `status`, `validate`, `next`, `run`, and `done` preserve existing fields and add freshness/route fields where missing.
- Capability resolution reads `Skill.HostCapabilities`; template frontmatter mirrors registry data through tests.

## Rollout and Rollback
Roll forward with additive JSON fields and focused tests. Rollback is a standard revert of the additive route/freshness/capability metadata changes.

## Risk
Low to medium risk: public JSON grows additively, but command surface behavior and tests are broad. The biggest risk is stale assumptions in governed artifacts, mitigated by full tests, coverage gate, and performance baseline check.
