# Decision

## Alternatives Considered

1. Sequential single-issue changes: smallest local blast radius, but repeats
   toolgen/docs churn and leaves the live tracker stale while child items remain
   in flight.
2. Integrated batch with issue-specific seams: one governed worktree resolves the
   confirmed open set, while each issue remains behind focused tests and explicit
   safety constraints.
3. Partial docs/toolgen slice: lower immediate risk, but leaves the public S3
   evidence recovery dead end and bad-test policy gap unresolved.

Selected alternative 2 because the user asked to confirm the live open issues
and then resolve them together. The selected design keeps the batch coherent by
using one shared rule: public developer surfaces may be made easier or safer, but
no evidence gate, file cleanup, or generated-skill profile may bypass governed
fail-closed behavior.

## Selected Approach

Implement the batch as six bounded slices:

- #263: add a narrowly scoped evidence replacement path for passing selected
  reviewer evidence that is invalid under context-origin authority checks.
- #167: refactor toolgen refresh into planned file operations applied through
  `fsutil.ApplyFileTransaction`, and add a private ownership manifest with
  sha256 checksums for cleanup classification.
- #168: add generated-skill dependency/profile metadata and router surfaces that
  reduce eager listing while preserving mandatory lifecycle and sensitive-domain
  skills.
- #170: reorganize docs into Diataxis using GSD Core as the structure reference,
  and add guided tutorials plus Start Here/scenario-oriented onboarding inspired
  by Trellis's Install & First Task, How It Works, and Real-World Scenarios
  pages. Update nav and surface/docs checks.
- #161: add delete-bad-tests policy text and Go analyzer tests for source-grep
  and timing assertions with explicit contract-test exceptions.
- #169: refresh GitHub state after implementation and update or close the tracker
  so it no longer lists closed work as open.

## Interfaces and Data Flow

- CLI evidence flow remains `slipway evidence skill`. The only behavior change is
  a guarded allowance inside the current S3 selected-review recordability check
  when the existing passing record is invalid under selected-review
  context-origin validation.
- Toolgen flow changes from immediate writes/removes to collecting write/remove
  operations, classifying stale files through a generated ownership manifest, and
  applying the operation set transactionally.
- Generated-skill profile flow adds profile metadata to toolgen output selection.
  Router skills are generated surfaces that direct agents to command/host
  surfaces; they do not become lifecycle authorities.
- Documentation flow changes MkDocs navigation and docs paths. GSD Core informs
  the tutorials/how-to/reference/explanation split; Trellis informs first-task,
  task/spec/memory, platform-aware setup, and scenario pages. Public command
  contracts remain generated from existing command registries and surface
  manifest checks.
- Test-lint flow adds a Go analyzer package and test fixtures. It is verification
  tooling, not runtime command behavior.
- GitHub tracker flow is external state: refresh open issues, then update/close
  #169 with the verified state.

## Rollout and Rollback

Rollout is a normal code/docs change in the bound worktree, verified by focused
tests, `go test ./...`, surface manifest checks, and governed review gates.

Rollback is git-based: revert the source/docs changes in this branch and rerun
the focused test set plus `go test ./...`. For generated adapter safety, the
ownership manifest and transaction tests must prove failed refreshes restore the
previous tree; if a generated cleanup classification is wrong, rollback removes
the new ownership-cleanup behavior and preserves unknown files by default.

## Risk

- Evidence overwrite risk: mitigated by allowing replacement only when the
  existing passing selected-review record is invalid under context-origin checks.
- File deletion risk: mitigated by transaction rollback and ownership manifest
  classification that preserves unknown files and backs up/refuses modified
  generated files.
- Profile pruning risk: mitigated by always-included critical skill tests.
- Analyzer false-positive risk: mitigated by documented exception patterns and
  fixture coverage for generated/golden/contract tests.
- Scope risk: mitigated by binding the batch to the six live open issues and
  excluding closed #258 and unrelated worktrees.
