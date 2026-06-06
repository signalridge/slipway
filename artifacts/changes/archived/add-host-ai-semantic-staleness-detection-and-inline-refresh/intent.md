# Intent

## Summary
Add host-AI semantic staleness detection and inline refresh for the codebase map
(issue #80). Today `AssessCodebaseMapDocs` derives status from pure content
presence, so a codebase map authored for a PRIOR change scores `populated` and is
consumed as authoritative with zero signal; the consume advisory
(`codebaseMapConsumeAdvisory`) is silent for `populated` maps; and the only
staleness guidance (`context-assembly/references/codebase-map.md`) uses the
git-mtime / renamed-entry-point / lockfile-mismatch fingerprint heuristics this
issue explicitly rejects. Replace that with a host-AI relevance self-check
surfaced at codebase-map consume time, and instruct the host to refresh stale
sections inline (re-author the doc in place — the engine re-reads on every
assessment) when the map no longer matches the current change scope. Keep it
advisory/non-blocking; the engine owns the trigger, the skills own the judgment.

## Complexity Assessment
complex
<!-- Rationale -->
Touches a public consume-time surface (`next`/`run` warnings), three generated
governed skills, and a reference doc that several skills depend on. The risk is
low (additive, non-blocking advisory; no new engine fingerprint; no blocking
gate) but the change must keep the engine/skill boundary clean — the engine can
only state "populated ≠ verified-relevant," and the host AI owns the semantic
relevance judgment and the inline refresh.

## Guardrail Domains
`external_api_contracts` — intent-based. The change adds a new advisory string to
the public `next`/`run` JSON `warnings` for `populated` codebase maps and changes
generated host-tool guidance (research-orchestration / plan-audit /
codebase-mapping skills) plus the codebase-map reference doc. The advisory is
purely additive (a status that previously produced no advisory); no existing
JSON field shape changes. Generated-surface refresh must show only the intended
additions.

## In Scope
- Engine trigger: add `codebaseMapRelevanceAdvisory` in `cmd/next_skill_view.go`
  that fires for a `populated` (and `partial`) codebase map when the next skill
  is a map consumer (research-orchestration / plan-audit), wired into the
  existing mutually-exclusive advisory chain and routed through the non-blocking
  `view.Warnings`. The advisory states that status reflects content presence, not
  scope relevance, and prompts the host to judge relevance and refresh stale
  sections inline.
- Skill instructions (the host-AI judgment + refresh action):
  `research-orchestration/SKILL.md`, `plan-audit/SKILL.md`, and
  `codebase-mapping/SKILL.md` — when the map is `populated`, the host must still
  judge whether it matches the current change scope (affected seams, blast
  radius, concerns) and re-author stale/irrelevant sections inline before relying
  on it; `populated` ≠ relevant.
- Reference doc: rewrite the "When stale or invalid" section of
  `context-assembly/references/codebase-map.md` to replace the
  mtime/entry-point/lockfile fingerprint definition with a host-AI semantic
  relevance judgment ("re-read the populated doc and judge whether it still
  describes the area in scope; if not, re-author the file in place — there is no
  engine staleness flag").
- Tests: advisory matrix for the new relevance advisory (fires for
  populated/partial under consuming skills; empty for non-consumers and for the
  non-durable statuses the existing advisories own); update the existing test
  that asserts `populated` produces no advisory.
- Regenerate generated skills/commands; zero unintended drift.

## Out of Scope
- A new engine staleness/fingerprint algorithm (mtime/git/lockfile) — explicitly
  rejected by the issue.
- A `scoped_to` / timestamp / "needs refresh" metadata file on the codebase map —
  would reintroduce engine-owned freshness state.
- A new `slipway codebase-map --refresh`/`--force` CLI flag — the host re-authors
  the doc in place (the engine re-reads on assessment), so inline refresh needs
  no new command surface; a convenience re-scaffold flag is deferred.
- Making the advisory a blocker — it stays advisory/non-blocking.
- Issues #95/#92/#91/#88/#86/#75 — separate efforts.

## Constraints
- Engine owns the trigger only; the engine cannot judge semantic relevance (no
  model of change-vs-map). The host AI owns relevance judgment + inline refresh
  via the skills.
- Advisory is non-blocking (`view.Warnings`), never a blocker; it must not gate
  progression.
- Public JSON field shape is unchanged (the advisory is an additive warning
  string); generated-surface refresh shows only the intended additions.
- The never-auto-clobber-authored-content invariant of `EnsureCodebaseMapDocs`
  stays intact (no engine overwrite of populated docs).
- Verify against the repo's own loop: `go build/vet/test ./...`,
  `go test ./internal/toolgen/...`, current-worktree `go run . init --refresh
  --tools all` with zero unintended drift, and `golangci-lint run` clean.

## Acceptance Signals
- `go build/vet/test ./...` green; `golangci-lint run` 0 issues; toolgen
  self-loop drift is only the intended generated-skill additions.
- A `populated` codebase map consumed by research-orchestration / plan-audit
  surfaces a non-blocking advisory that says status reflects content presence not
  scope relevance and prompts the host to judge relevance + refresh inline; the
  advisory does not appear for non-consuming skills.
- The three consuming/authoring skills instruct the host to self-check a
  populated map against the current change scope and re-author stale sections
  inline; the codebase-map reference doc no longer defines staleness via
  mtime/entry-point/lockfile heuristics.
- Public `next`/`run` JSON field shape is unchanged; the advisory is an additive
  `warnings` entry.

## Open Questions
<!-- None: the consume-advisory trigger, the engine/skill boundary, and the
target skills/doc were mapped against current main (release 0.10.0) with exact
code locations confirmed; no blocking unknowns. -->

## Deferred Ideas
- A `slipway codebase-map --refresh` flag to re-scaffold a stale populated doc
  back to baseline as a re-authoring starting point.
- Recording the refresh as explicit governed evidence.

## Approved Summary
Deliver issue #80 as an `external_api_contracts` governed change: the engine adds
a non-blocking consume-time advisory for `populated` codebase maps stating that
status reflects content presence, not scope relevance; the consuming/authoring
skills (research-orchestration, plan-audit, codebase-mapping) instruct the host
AI to judge a populated map against the current change scope and re-author stale
sections inline; and the codebase-map reference doc replaces the rejected
git-mtime/entry-point/lockfile staleness heuristics with that host-AI semantic
relevance judgment. No engine fingerprint, no metadata file, no new CLI flag, no
blocking gate; public JSON field shape is unchanged.

Confirmed by user: 2026-06-06 (goal: resolve P3 issues #86/#80 to done-ready).
