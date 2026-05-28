# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go, Markdown
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: mkdocs-based site under `docs/`; `README.md` is the primary entry doc; an `## AI Tool Installation Prompt` section already exists in `docs/installation.md`; `README.md:166` references that anchor. `MaxSlugLength` is defined at `internal/model/identity.go:10`. The work is *docs rewrite + targeted code change* (post-pivot scope), not a from-scratch addition.

## Summary
Two coupled deliverables in this governed change:

1. **Docs rewrite (refined)** — Replace the body of `## AI Tool Installation Prompt` in `docs/installation.md` and add a parallel preview block in `README.md` near `## Install` / `## Quick Install`. The pasted prompt becomes a short *pointer prompt* (≈10 lines) that directs the AI agent to fetch the canonical Installation page at `https://signalridge.github.io/slipway/installation/` and follow it. The actual Discovery / Install / Initialize / Verify / Report guidance lives as readable prose inside the `## AI Tool Installation Prompt` section so the agent sees it when it fetches the URL, and so a human reading the page also understands the structure.
2. **Slug length cap** — Reduce `MaxSlugLength` in `internal/model/identity.go` from `96` to a tighter value (proposed `60`). Update the related test in `internal/model/identity_test.go` so the cap is exercised. This is the scope addition admitted by the operator-confirmed pivot on 2026-05-28.

## Complexity Assessment
complex
<!-- Rationale: post-pivot, the change couples a docs rewrite with a small code change in internal/model. The acceptance gate still includes a live operator-run Claude Code paste-and-install pass. Multi-step, multi-surface. -->

## Guardrail Domains
<!-- none detected; the slug-length constant is not a guardrail-domain concern. -->

## In Scope
- Replace the prompt code block in `## AI Tool Installation Prompt` of `docs/installation.md` with a short (≈10-line) pointer prompt that instructs the agent to fetch the canonical Installation page and follow it, after detecting OS/arch and `slipway --version`.
- Add readable prose under `## AI Tool Installation Prompt` in `docs/installation.md` describing the Discovery / Install / Initialize / Verify / Report steps the agent should perform once it has fetched the page (so the page is self-contained for an agent that follows the pointer).
- Mirror the same short pointer prompt as a copyable preview block in `README.md` near `## Install` / `## Quick Install` (no detailed prose in the README; canonical detail lives in `docs/installation.md`).
- Preserve the heading slug `ai-tool-installation-prompt` so `README.md:166` keeps resolving.
- Preserve the existing manual installation checklist in `docs/installation.md` unchanged outside the rewritten section.
- Reduce `MaxSlugLength` in `internal/model/identity.go` from `96` to `60`. Update the test in `internal/model/identity_test.go` that exercises the cap (if it relies on the old number).
- Confirm the slug-length change does not break the rest of `go test ./...` (no other tests should reference the old value).

## Out of Scope
- Edits to `docs/index.md`, `docs/operator-guide.md`, or other docs beyond what is strictly needed.
- Redesign of the manual installation checklist.
- Adding new install channels or changing the documented install surfaces (the canonical page already covers them).
- Per-agent variants (no separate Claude Code / Cursor / Codex prompt blocks). One agent-agnostic prompt only.
- Re-introducing PR #8's `codebase_map`, `learn`, or `local_ignore` CLI affordances (still deferred to separate changes).
- Backfilling old slugs (existing change directories with 96-char slugs stay as-is; the cap only affects new changes created after this lands).
- Adding a CLI-level warning when input description is much longer than the cap; that is a separate future improvement.

## Constraints
- `mkdocs build --strict` must succeed.
- `go build ./...` and `go test -count=1 ./...` must remain green.
- Prompt remains agent-agnostic; no user-specific paths, tokens, or secrets.
- Prompt must keep the safety framing established in the current baseline: prefer documented release sources, stop and verify before installing same-name packages from unrelated registries.
- All work happens on the dedicated worktree branch; no force-pushes to `main`.
- Backwards compatibility: existing slugs on disk are not affected. Only `SlugifyTitle` output for new descriptions is shortened.

## Acceptance Signals
- `mkdocs build --strict` passes locally on the worktree.
- `go test -count=1 ./...` and `go build ./...` pass on the worktree, including the updated `internal/model/identity_test.go` cap assertion.
- The rewritten `docs/installation.md` section and the new README preview block render cleanly and read clearly to the operator.
- The new pointer prompt, when pasted into Claude Code on a clean shell, lets the agent fetch the canonical Installation page and drive a successful end-to-end install (`slipway --version` resolves on PATH). This live run is performed by the operator as the final acceptance gate.
- README's existing link at line 166 still resolves to `docs/installation.md#ai-tool-installation-prompt`.
- A fresh `slipway new` invocation with a long description produces a slug whose length is `≤ 60` characters (sanity check, operator-run if desired).

## Open Questions
<!-- Resolved in this intake: -->
<!-- Q (prompt verbosity): operator chose short pointer prompt + canonical page, 2026-05-28 (post-pivot). -->
<!-- Q (slug cap value): proposed 60; flagged for operator confirmation in Approved Summary. -->
- [x] S1_PLAN/research will reconfirm that the pointer-prompt approach is consistent with comparable-project conventions surfaced earlier (Aider's short installer, Claude Code's tabbed quickstart, gh CLI's documentation-driven install). No new research dimension is opened; the existing research.md already covers the four dimensions.

## Deferred Ideas
- Per-agent tailored sections (Claude Code / Cursor / Codex specific) — deferred.
- Re-introducing the PR #8 CLI affordances (`codebase_map`, `learn`, etc.) — deferred.
- CLI-level warning when `slipway new` receives a description much longer than `MaxSlugLength` — deferred; separate change.
- Migrating existing long slugs on disk — deferred; not needed for the cap to take effect on new changes.

## Approved Summary
Two coupled deliverables (post-pivot scope, operator-confirmed 2026-05-28):

1. **Docs — short pointer prompt + canonical prose.** Replace the body of `## AI Tool Installation Prompt` in `docs/installation.md` with a short (~10-line) pointer prompt that tells the agent to fetch `https://signalridge.github.io/slipway/installation/` and follow it. Move the Discovery / Install / Initialize / Verify / Report detail into readable prose under the same section so the page is self-contained when the agent fetches it. Mirror the same short pointer prompt as a copyable preview block in `README.md` near `## Install` / `## Quick Install`. Preserve the section heading slug `ai-tool-installation-prompt` and the existing manual installation checklist.
2. **Code — slug length cap.** Reduce `MaxSlugLength` in `internal/model/identity.go` from `96` to `60`. Update the test in `internal/model/identity_test.go` that exercises the cap. Existing slugs on disk are not migrated.

Acceptance: `mkdocs build --strict` and `go test -count=1 ./...` pass on the worktree; the operator reads the rendered `README.md` and `docs/installation.md` and confirms the pointer prompt + prose are clear; the operator pastes the short prompt into Claude Code on a clean shell and confirms a successful end-to-end install (`slipway --version` resolves on PATH); `README.md:166` still resolves to the docs anchor.

Confirmed by user (initial framing): 2026-05-28. **Pivot-rescope** confirmed by operator on 2026-05-28 after Wave 2 mechanical verification revealed the verbose Approach-B prompt and the long auto-generated slug as separate concerns to address in one delivery.
