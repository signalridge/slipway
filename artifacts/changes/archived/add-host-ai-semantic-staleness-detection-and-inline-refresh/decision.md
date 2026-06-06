# Decision
## Project Context
- Tech Stack: Go
- Conventions: engine under internal/engine (read-only over model); cmd thin orchestrators; generated skills/commands from internal/tmpl/templates + toolgen; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Selected Approach
Extend the existing non-durable consume-advisory pattern to `populated` maps,
split cleanly across the engine/skill boundary:

1. **Engine trigger (REQ-001).** Add `codebaseMapRelevanceAdvisory(status,
   nextSkillName)` in `cmd/next_skill_view.go` that fires for
   `CodebaseMapStatusPopulated` / `CodebaseMapStatusPartial` when the next skill
   is research-orchestration or plan-audit, wired as a new branch in the existing
   mutually-exclusive advisory chain at the S1 call site and routed through the
   non-blocking `view.Warnings`. The wording states the status reflects content
   presence, not scope relevance, and prompts a host relevance self-check +
   inline refresh. The engine cannot judge relevance itself, so it only triggers.
2. **Skill judgment (REQ-002).** research-orchestration / plan-audit /
   codebase-mapping SKILL templates instruct the host to treat `populated` as
   "present, not verified-relevant," judge the map against the current change
   scope, and re-author stale sections inline. Re-authoring the .md in place IS
   the inline refresh because `AssessCodebaseMapDocs` re-reads on every
   assessment — no engine overwrite, no clobbering authored content.
3. **Reference doc (REQ-003).** Rewrite the "When stale or invalid" section of
   `context-assembly/references/codebase-map.md` to a host-AI semantic relevance
   judgment + inline re-author, removing the git-mtime / entry-point /
   lockfile-mismatch fingerprint definition.

## Key Decisions
- Engine owns the TRIGGER, skills own the JUDGMENT — the engine has no semantic
  model of change-vs-map, so it can only state "populated ≠ verified-relevant"
  and prompt; the host judges + refreshes. [research]
- Advisory is non-blocking (`view.Warnings`), additive for a status that
  previously produced none; public JSON field shape unchanged. [research]
- Inline refresh = host re-authors the .md in place (engine re-reads on assess);
  no new state, no clobbering of authored content. [research]

## Rejected Alternatives
- New engine staleness/fingerprint algorithm (mtime/git/lockfile): rejected by
  the issue; reintroduces engine-owned heuristics.
- `scoped_to`/timestamp/needs-refresh metadata file: reintroduces engine-owned
  freshness state.
- `slipway codebase-map --refresh`/`--force` flag: a new public command surface;
  inline refresh already works via host re-author; deferred.
- Blocking advisory: the issue wants advisory/non-blocking host judgment.

## Interfaces and Data Flow
- `cmd/next_skill_view.go`: new `codebaseMapRelevanceAdvisory` + a branch at the
  existing advisory call site; output flows into `view.Warnings` →
  `next`/`run` JSON `warnings` (additive). No change to `codebase_map_status` /
  `codebase_map_doc_states` fields.
- Generated skills regenerate from the edited SKILL templates + reference doc via
  toolgen / `slipway init --refresh`.
- No schema change; no new CLI flag; no engine assessment change.

## Rollout and Rollback
- Rollout: additive advisory + skill/doc guidance; no flag. Verified by
  `go build/vet/test ./...`, `golangci-lint run`, `go test ./internal/toolgen/...`,
  `go run . init --refresh --tools all` + diff review (only intended additions),
  and an advisory-matrix test.
- Rollback: revert the branch. No data migration; no engine state added.
  Generated skills/docs revert with code.

## Risk
- Advisory could double-fire with the existing non-durable advisory → the chain
  is mutually exclusive (consume/discovery advisories return "" for populated);
  add a matrix test pinning populated→relevance-advisory and non-durable→existing
  advisory.
- Existing test asserts populated→no advisory → update it to expect the new
  relevance advisory (the durability advisory still returns "" for populated;
  only the new relevance advisory fires).
- Generated-surface drift from template edits → regenerate and review that only
  the intended skill/reference additions appear.
- Engine/skill boundary creep → keep the engine output to a trigger string only;
  the semantic judgment + refresh lives entirely in the skills/doc.
