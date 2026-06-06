# Assurance
## Project Context
- Tech Stack: Go
- Conventions: engine under internal/engine (read-only over model); cmd thin orchestrators; generated skills/commands from internal/tmpl/templates + toolgen; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Adds host-AI semantic staleness detection and inline refresh for the codebase map
(issue #80). The engine adds a non-blocking consume-time advisory
(`codebaseMapRelevanceAdvisory`) for `populated`/`partial` maps under
research-orchestration/plan-audit that states the status reflects content
presence, not scope relevance, and prompts a host relevance self-check + inline
refresh. The research-orchestration, plan-audit, and codebase-mapping skills
instruct the host to judge a populated map against the current change scope and
re-author stale sections inline (the assessment re-reads the docs, so the inline
edit is the refresh). The codebase-map reference doc replaces the rejected
git-mtime/entry-point/lockfile staleness fingerprint with that host-AI semantic
relevance judgment. No engine fingerprint, no metadata file, no new CLI flag, no
blocking gate; public JSON field shape is unchanged (the advisory is an additive
`warnings` entry). Classified `external_api_contracts` (additive public JSON +
generated host-tool guidance).

## Verification Verdict
Pass for implementation, tests, and review up to the S4 goal/closeout handoff.
`go build/vet/test ./...` green; `golangci-lint run` 0 issues;
`go run . init --refresh --tools all` regenerates only the intended skill/
reference additions.

## Evidence Index
- Engine trigger (t-01): `cmd/next_skill_view.go` `codebaseMapRelevanceAdvisory`
  + the advisory-chain branch; tests `TestCodebaseMapRelevanceAdvisoryMatrix`,
  `TestNextSurfacesCodebaseMapRelevanceAdvisoryForPopulatedMap`.
- Skill + reference guidance (t-02): research-orchestration / plan-audit /
  codebase-mapping `SKILL.md` populated-map self-check + inline-refresh
  instruction; `context-assembly/references/codebase-map.md` staleness rewrite.
- Proof (t-03): build/vet/test, golangci-lint, toolgen, init-refresh.

## Requirement Coverage
- REQ-001: `cmd/next_skill_view.go` `codebaseMapRelevanceAdvisory` + wiring;
  advisory-matrix + integration tests.
- REQ-002: research-orchestration / plan-audit / codebase-mapping SKILL templates.
- REQ-003: `context-assembly/references/codebase-map.md` staleness rewrite.
- REQ-004: advisory additive (no JSON field-shape change), non-blocking
  (`view.Warnings`), generated surfaces regenerated with only intended additions.

## Residual Risks and Exceptions
- The engine cannot judge semantic relevance; it only surfaces the trigger. The
  host AI owns the relevance judgment and the inline refresh — by design (engine
  owns structure/trigger, skill owns substance). If a host ignores the advisory,
  the map is still consumed; the advisory is intentionally non-blocking.
- Inline refresh relies on the host re-authoring the doc in place; no engine
  overwrite of authored content (the never-clobber invariant is preserved). A
  `--refresh` convenience flag is deferred.
- The codebase map remains advisory; source and live `go run .` output are the
  authorities.

## Rollback Readiness
Rollback is a branch revert. No data migration; no engine state added. Generated
skills/docs revert with the templates.

## Archive Decision
Proceed to done-ready after S4 `goal-verification` and `final-closeout` pass.
Active `go run . validate --json` freshness/readiness proof must be captured
before `slipway done`; archived bundles are not described as revalidated through
the active validate gate. (Global `health --governance` reflects concurrent
multi-active changes in other worktrees — an environment condition, not this
change.)
