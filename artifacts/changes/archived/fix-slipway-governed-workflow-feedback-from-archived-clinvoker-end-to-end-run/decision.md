# Decision

## Project Context
- Tech Stack: Go CLI governance engine
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, Markdown, YAML

## Alternatives Considered

### Documentation-only
- Pros: minimal runtime risk.
- Cons: leaves reproduced action text, codebase-map, task parser, and template/schema failures in place.
- Decision: rejected.

### Broad path architecture rewrite
- Pros: could fully address early worktree binding, worktree-local archives, shared read locks, and catalog generation in one pass.
- Cons: high blast radius across state discovery, repair, archive lookup, and generated tool surfaces.
- Decision: deferred for path changes that cannot be proven safely in focused tests.

### Layered compatibility fix
- Pros: fixes reproduced runtime/template/parser defects, keeps JSON additions backward compatible, documents intentional locking constraints, and rewrites archive artifact metadata while preserving project-root archive discovery.
- Cons: does not fully relocate worktree-bound archives or implement shared read locks.
- Decision: selected.

## Selected Approach

Implement a layered compatibility fix:
- Change misleading required-action wording and host handoff surfaces with regression tests.
- Add scaffold-only codebase-map detection to command output, stats/health semantics, and next technique hints.
- Align research and verification skill guidance with runtime schemas.
- Extend task metadata parsing for `evidence` and `acceptance`, including semantic hash coverage.
- Document exclusive state-lock behavior and explicit full-suite timeout.
- Keep project-root archives for compatibility, but rewrite frozen artifact paths to the archive-local bundle path.

## Interfaces and Data Flow

- `codebase-map --json` may add advisory status fields while preserving existing `codebase_map_dir`, `codebase_map_docs`, and `created`.
- `stats --json` may add scaffold-only/populated counts while preserving existing freshness fields.
- `next/run --json --diagnostics` should become clearer without removing existing fields.
- `tasks.md` parser accepts `evidence` and `acceptance`; these fields affect task semantic hashes.
- `ArchiveChange` keeps archive placement but rewrites artifact metadata paths after freezing.

## Rollout and Rollback

- Rollout: land focused code/template/docs changes behind existing command contracts and test with temp-workspace fixtures.
- Rollback: revert the touched files; archived bundles remain readable because archive placement is not moved.
- Verification: targeted Go tests, full `go test -timeout=20m ./... -count=1`, `go build ./...`, and governed `validate/next/run` checks.

## Risk

- Risk: adding `next_skill` at S1 bundle could alter `run` behavior if implemented in progression instead of view shaping. Mitigation: prefer an explicit next-view handoff/action contract or tests that lock transition behavior.
- Risk: codebase-map scaffold detection could over-classify terse but valid notes as placeholders. Mitigation: only treat generated templates with empty bullet values as scaffold-only.
- Risk: archive artifact path rewriting can break tests expecting old active paths. Mitigation: add archive-local path tests and keep project-root archive discovery unchanged.
