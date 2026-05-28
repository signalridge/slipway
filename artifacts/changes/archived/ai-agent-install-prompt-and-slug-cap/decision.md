# Decision
## Project Context
- Tech Stack: Go
- Conventions: mkdocs-based site under `docs/`; existing safety framing about "documented release sources" lives at `README.md:113` and `docs/installation.md:9`.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Alternatives Considered

See `research.md` for the full alternatives writeup. Summary:

- **Approach A — Minimal-touch refinement.** Keep the existing numbered-list prompt structure and only add shell/OS detection, per-step recovery branches, and a clearer report-out closing. Lowest risk and smallest diff, but feels under-ambitious against the operator-confirmed "rewrite" framing.
- **Approach B — Structured-block rewrite (selected).** Reorganize the prompt into named blocks **Discovery / Install (with per-step recovery branches) / Initialize / Verify / Report**; co-locate safety framing inside the blocks it applies to. Add a parallel preview block to `README.md`. Medium diff, higher legibility for an agent parsing it, aligned with the rewrite framing.
- **Approach C — Two-tier prompt (rejected).** Split into agent-quickstart (~5 lines) and agent-full-setup. Doubles maintained surface; weakens the "single agent-agnostic prompt" framing in `intent.md`'s In Scope.

### Constraints (from source document)
- `mkdocs build --strict` must succeed.
- `go build ./...` and `go test -count=1 ./...` must remain green.
- Prompt remains agent-agnostic; no user-specific paths, tokens, or secrets.
- Prompt must keep the safety framing already established in the current baseline: prefer documented release sources, stop and verify before installing same-name packages from unrelated registries (matching `README.md` line 113 and `docs/installation.md` line 9).
- All work happens on the dedicated worktree branch; no force-pushes to `main`.

## Selected Approach

**Approach B — Structured-block rewrite, refined post-pivot to "short pointer prompt + canonical prose"**, confirmed by operator 2026-05-28; refined 2026-05-28 after Wave 2 mechanical verification, when the operator flagged the 55-line first cut as too verbose. The same pivot also expanded scope to include a small code change: reducing `MaxSlugLength` from `96` to `60`.

### Docs portion (REQ-001, REQ-002, REQ-003, REQ-004)

1. `docs/installation.md` retains the heading `## AI Tool Installation Prompt` (slug `ai-tool-installation-prompt` preserved so `README.md:166` keeps resolving). A terse intro sentence introduces the prompt and asks the user to review before pasting.
2. Inside the fenced ```` ```text ```` block, the prompt is a short (~10-line) *pointer prompt*: it tells the agent to fetch `https://signalridge.github.io/slipway/installation/`, follow the "AI Tool Installation Prompt" section, detect OS and CPU architecture, prefer documented release sources owned by the Slipway project, stop and report if no documented path applies (do not install same-name packages from unrelated registries), verify with `slipway --version` / `slipway status --json` / `git status --short --branch`, and report what it did.
3. Directly below the prompt code block, readable prose under named sub-sections **Discovery**, **Install** (numbered preference list with per-step recovery branches), **Initialize**, **Verify**, **Report** gives the actual guidance the agent will read once it fetches the canonical page. This prose replaces the inline-in-the-code-block detail of the first cut.
4. `README.md` gains a copyable preview block near `## Install` / `## Quick Install`. The preview is the same short pointer prompt as the canonical docs version. The README does **not** duplicate the Discovery/Install/Initialize/Verify/Report prose. The existing link in the body at `README.md:166` continues to point to the canonical `docs/installation.md#ai-tool-installation-prompt`.

### Code portion (REQ-005)

1. Change `MaxSlugLength` in `internal/model/identity.go` from `96` to `60`. Cap value chosen as the descriptive-but-compact middle ground (alternatives considered: 48 too aggressive, 80 too mild).
2. Verify `TestSlugifyTitleLimitsLongSlugs` in `internal/model/identity_test.go` still passes; if the test references the old number directly, update the reference.
3. Do not migrate or rewrite existing slugs already on disk. `SlugifyTitle` is the only producer; load paths use stored slug strings and are unaffected.

This direction must continue honoring the documented constraints:
- `mkdocs build --strict` must succeed.
- `go build ./...` and `go test -count=1 ./...` must remain green.
- Prompt remains agent-agnostic; no user-specific paths, tokens, or secrets.
- Prompt must keep the safety framing already established in the current baseline.
- All work happens on the dedicated worktree branch; no force-pushes to `main`.

## Interfaces and Data Flow

Affected interfaces (post-pivot):

- **README anchor link** to `docs/installation.md#ai-tool-installation-prompt` — preserved by keeping the section heading text stable.
- **Slug-producing API**: `model.SlugifyTitle(string) string` retains its signature and contract (lowercase, hyphenated, non-empty, capped). Only the cap value changes (`96 → 60`). Callers of `SlugifyTitle` (the new-change pipeline) consume the returned string directly and do not depend on the exact cap.

Data-flow changes: no migration. Existing `change.yaml` records and bundle directory names retain their stored slug strings; the new cap applies only to slugs produced after the change lands.

Interface and data-flow changes must respect these documented constraints:
- `mkdocs build --strict` must succeed.
- `go build ./...` and `go test -count=1 ./...` must remain green.
- Prompt remains agent-agnostic; no user-specific paths, tokens, or secrets.
- Prompt must keep the safety framing already established in the current baseline.
- All work happens on the dedicated worktree branch; no force-pushes to `main`.

## Rollout and Rollback

Rollout: merge the worktree branch to `main` via PR after operator-gated Claude Code acceptance passes. No staged rollout, no feature flag.

Rollback: a single `git revert <merge-commit>` on `main` returns `README.md`, `docs/installation.md`, `internal/model/identity.go`, and `internal/model/identity_test.go` to their pre-merge contents. Existing change directories on disk are unaffected by either rollout or rollback (the cap only governs new slug *production*). No downstream consumers (releases, packages, schema) depend on the slug cap value. Verification commands after rollback: `git diff main^ -- README.md docs/installation.md internal/model/identity.go internal/model/identity_test.go` (expect empty), `mkdocs build --strict`, and `go test -count=1 ./internal/model/...`.

Rollback planning must preserve these documented constraints:
- `mkdocs build --strict` must succeed.
- `go build ./...` and `go test -count=1 ./...` must remain green.
- Prompt remains agent-agnostic; no user-specific paths, tokens, or secrets.
- Prompt must keep the safety framing already established in the current baseline.
- All work happens on the dedicated worktree branch; no force-pushes to `main`.

## Risk

- **Heading-slug drift (medium):** if the rewritten section heading were renamed, `README.md:166` would 404 to the anchor. *Mitigation*: keep the heading text `AI Tool Installation Prompt` verbatim; add a heading-stability check to `t-06`.
- **mkdocs strict-build failure (low):** Markdown syntax slip in the new content. *Mitigation*: `t-03` runs `mkdocs build --strict` before review.
- **Suggesting a non-existent CLI flag (low):** rewrite that names `slipway init --tools …` with stale flags could mislead the agent. *Mitigation*: cross-check every command in the new prompt against `cmd/` source during `t-01` execution; the existing prompt already uses real flags and we are not introducing new ones.
- **README preview drift from canonical version (low):** the two surfaces could diverge in future edits. *Mitigation*: `t-02` adds the preview as a single fenced block that mirrors the canonical structure; spec-compliance review checks both surfaces together.
- **Agent-paste acceptance not reproducible (low, accepted):** the operator-gated Claude Code live install is non-mechanical. *Mitigation*: acceptance is gated at goal-verification with the operator; documented in `intent.md` Acceptance Signals and the post-pivot `tasks.md`.
- **Slug cap silently truncates valuable description suffix (low):** reducing the cap from 96 to 60 means long descriptions lose more trailing context. *Mitigation*: callers should rely on `description` (preserved verbatim in `change.yaml`) when they need the full text; the slug is for filesystem and routing only. Deferred follow-up (intent.md Deferred Ideas): add a CLI-level warning when input length far exceeds the cap.
- **Existing 96-char slug for this change (low, accepted):** the directory and branch for the active change keep their long slug because the cap only affects future `SlugifyTitle` calls. *Mitigation*: intentional non-migration; documented in REQ-005 and the Rollback section.

Constraint-driven risks to keep explicit during implementation:
- `mkdocs build --strict` must succeed.
- `go build ./...` and `go test -count=1 ./...` must remain green.
- Prompt remains agent-agnostic; no user-specific paths, tokens, or secrets.
- Prompt must keep the safety framing already established in the current baseline.
- All work happens on the dedicated worktree branch; no force-pushes to `main`.
