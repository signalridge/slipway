# Research

Mapped issue #80 against current main (release 0.10.0). Exact code locations and
a minimal, advisory/non-blocking fix confirmed.

## Alternatives Considered

### Where the gap is (engine, content-presence only)
- `internal/engine/artifact/codebase_map.go` `AssessCodebaseMapDocs` (~628-680)
  classifies each doc as scaffold_only / baseline / populated / missing purely by
  CONTENT comparison against the scaffold and the deterministic CLI baseline.
  `populated` means only "differs from scaffold/baseline" — NOT "relevant to or
  fresh for the current change." A map authored for a prior change scores
  `populated` forever. There is no relevance/recency/scoped-to input and no
  per-doc metadata file.
- `EnsureCodebaseMapDocs` (~120-150) re-scaffolds only scaffold-only/legacy docs
  and `continue`s past populated ones (never clobbers authored content), so
  `slipway codebase-map` cannot itself refresh a populated-but-stale doc.
- `cmd/next_skill_view.go` `codebaseMapConsumeAdvisory` (~361-376) emits an
  advisory ONLY for scaffold_only/baseline maps under research-orchestration /
  plan-audit; for `populated`/`partial` it returns "" (no advisory). The doc
  comment (~355-360) and tests pin this. So a populated prior-change map consumes
  silently.
- The only populated-map staleness guidance,
  `internal/tmpl/templates/skills/context-assembly/references/codebase-map.md`
  (~62-77), defines staleness via git-mtime distance, renamed entry points, and
  lockfile mismatch — the engine-fingerprint heuristics #80 rejects — and is a
  reference doc, not surfaced at consume time.

### Selected direction (advisory trigger + host-AI judgment)
The architecture already does the right thing for non-durable maps; extend the
SAME pattern to `populated`, split cleanly by the engine/skill boundary:
- ENGINE owns the TRIGGER: add `codebaseMapRelevanceAdvisory` in
  `cmd/next_skill_view.go` that fires for `populated` (and `partial`) maps under
  the map-consuming skills, routed through the non-blocking `view.Warnings`. The
  engine can only state "status reflects content presence, not scope relevance"
  — it has no model to judge relevance itself.
- SKILLS own the JUDGMENT + REFRESH: research-orchestration / plan-audit /
  codebase-mapping SKILL templates instruct the host to judge a populated map
  against the current change scope and re-author stale sections inline (the
  engine re-reads on every assessment, so re-authoring the .md in place IS the
  inline refresh — no new state, no clobbering).
- DOC: rewrite the reference's staleness section to a host-AI semantic relevance
  judgment + inline re-author, replacing the mtime/lockfile fingerprint.

### Rejected alternatives
- A new engine staleness/fingerprint algorithm (mtime/git/lockfile): explicitly
  rejected by the issue; reintroduces engine-owned freshness heuristics.
- A `scoped_to`/timestamp/needs-refresh metadata file: reintroduces engine-owned
  freshness state; heavier than needed.
- A `slipway codebase-map --refresh`/`--force` flag: a new public command surface;
  inline refresh is already achievable by the host re-authoring the .md in place,
  so the flag is deferred, not required.
- Making the advisory blocking: the issue wants advisory/non-blocking host
  judgment.

## Unknowns
- None blocking. The trigger function, its call site, the consuming skills, the
  reference doc, and the existing tests were all located with exact line refs.

## Assumptions
- `view.Warnings` is the correct non-blocking surface (it is a plain []string in
  the next/run JSON and human output, never a blocker — confirmed).
- `AssessCodebaseMapDocs` re-reads doc content on every assessment, so a
  host-re-authored populated doc is immediately re-classified with no state to
  invalidate (confirmed — no metadata cache).
- The advisory chain at the call site is mutually exclusive (consume/discovery
  advisories return "" for populated), so a new populated-case branch will not
  double-fire.

## Canonical References
- Engine trigger: `cmd/next_skill_view.go` (`codebaseMapConsumeAdvisory` ~361-376,
  call site ~298-302), `cmd/next_context_build.go` (~25-37, 77),
  `internal/engine/artifact/codebase_map.go` (`AssessCodebaseMapDocs` ~628-680).
- Public JSON: `cmd/next.go` (codebase_map_status/doc_states ~157-158),
  `cmd/next_handoff.go` (~52-53).
- Skills: `internal/tmpl/templates/skills/research-orchestration/SKILL.md` (~61-81),
  `plan-audit/SKILL.md` (~21-44), `codebase-mapping/SKILL.md` (~42-45,74-78),
  `context-assembly/references/codebase-map.md` (~62-77).
- Tests: `cmd/next_skill_capability_hints_test.go` (~27-51, 290-294),
  `internal/tmpl/templates_test.go` (~190-214).
