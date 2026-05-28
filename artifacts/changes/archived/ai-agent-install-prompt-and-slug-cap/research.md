# Research

## Research Findings

### Architecture
- Affected modules: `README.md` (root), `docs/installation.md`, optionally `mkdocs.yml` if a new anchor or nav entry is added.
- Dependency chains: none in code paths. The change touches only Markdown surfaces that mkdocs renders.
- Blast radius: low. Docs-only. No runtime, CLI, schema, or governance behavior changes.
- Constraints:
  - `README.md` line 113 and `docs/installation.md` line 9 already establish a "documented release sources" safety framing — the rewritten prompt must remain consistent with that posture.
  - `README.md` line 166 currently links to `docs/installation.md#ai-tool-installation-prompt`. Whatever the rewritten section heading becomes, the existing anchor (or a redirect link from the old anchor) must keep this reference working.
  - mkdocs strict mode treats unresolved internal links and undefined anchors as failures.

### Patterns
- **Existing prompt structure (baseline at `docs/installation.md:271-303`)**: opener → discovery (root, OS/arch, on-PATH check) → numbered preference list (release artifact → Homebrew Cask → Linux package channels → Scoop/Windows → `go install` → local build → stop-and-report) → adapter initialization → safety re-statement → verification calls → report-out instruction.
- **Conventions from comparable projects**:
  - **Claude Code docs (`code.claude.com/docs/en/quickstart`)** — installation is presented as a `<Tab>` matrix (Native Install (Recommended) → Homebrew → WinGet), each tab carries its own "auto-update vs manual-update" framing, and the quickstart embeds recovery branches like "If you see `'irm' is not recognized`, you're in CMD, not PowerShell." Step-numbered post-install flow.
  - **mise getting-started** — opens with the shell-script installer, then per-platform package managers in a fixed sequence (brew → Windows → apt → dnf → snap). Trust/safety framing appears mid-guide, not next to the install command itself.
  - **gh CLI README** — sequences platforms (macOS → Linux → Windows → Build from source → Codespaces → Actions); explicitly separates "official channels" from "community-supported docs"; calls out verified binaries via Sigstore/cosign.
  - **Aider install docs** — orders methods by preference (`aider-install` → one-liners → `uv` → `pipx` → `pip`) and explicitly warns "one of the above methods is usually safer" before falling back to system package managers.
- **Reusable abstractions**: the existing "documented release sources" framing at `README.md:113` / `docs/installation.md:9` is the right anchor for the safety language; the rewrite should reuse the same vocabulary ("documented release sources", "stop and verify before installing same-name packages from unrelated registries") rather than introduce a competing one.
- **Convention deviations**: comparable projects rarely publish a single agent-targeted paste-into-your-LLM prompt — they target humans via `<Tabs>` or per-platform sections. Slipway's pattern of one agent-agnostic paste block is the unusual surface here; it should be kept as a single block but borrow the recovery-branch and report-out conventions from Claude Code's quickstart and gh CLI's verification framing.

### Risks
- Technical risks:
  - **Medium**: changing the section heading would break the `README.md:166` link (`#ai-tool-installation-prompt`) and any external link to that anchor. Mitigation: keep the heading slug stable, or add an anchor alias.
  - **Low**: mkdocs strict build catches accidental Markdown breakage. Mitigation: run `mkdocs build --strict` as part of acceptance.
  - **Low**: a rewrite that introduces flag/command names not present in the current CLI would mislead the agent (e.g., suggesting `slipway init` flags that no longer exist). Mitigation: cross-check every command against `cmd/` source during execution wave.
- Guardrail domains: none — docs-only delivery, no auth/credentials/PII/financial/schema/irreversible-op/external-API surfaces.
- Reversibility: fully reversible via a single revert commit; no migrations, no released artifacts depend on the new section.

### Test Strategy
- Existing coverage:
  - mkdocs build is enforced by CI (per README.md:196).
  - `go test ./...` covers Go code only; docs are not exercised by Go tests.
  - There is no automated test that pastes the prompt into an agent — that gate is human (operator) per intent.md acceptance.
- Infrastructure needs: none new.
- Verification approach:
  - Mechanical: `mkdocs build --strict`, `go build ./...`, `go test -count=1 ./...` on the worktree.
  - Visual: operator reads rendered README and `docs/installation.md` sections and confirms clarity.
  - Live agent test: operator pastes the new prompt into Claude Code on a clean shell, confirms `slipway --version` works end-to-end. This is the documented goal-verification human gate.

## Alternatives Considered

- **Approach A — Minimal-touch refinement.** Keep the existing prompt's overall flow and numbered list. Add: (1) an explicit shell/OS detection step at the top, (2) per-step recovery branches modeled on Claude Code's quickstart ("if X, then Y"), (3) more explicit registry-ownership-verification language, (4) a clearer "report-out" closing block. Add a copyable preview of the same prompt to `README.md` near `## Install` with a one-line link back to the canonical version in `docs/installation.md`.
  - Tradeoffs: lowest risk, smallest diff, easiest spec-compliance. Doesn't materially change the prompt's organization; readers familiar with the current section see a polished version of what they already know. May feel under-ambitious given the rewrite framing.

- **Approach B — Structured-block rewrite.** Reorganize the prompt into named blocks: **Discovery** (root, OS/arch, on-PATH) → **Install** (numbered preference list with explicit per-step recovery branches) → **Initialize** (adapter init) → **Verify** (post-install smoke checks) → **Report** (what to surface back to the operator). Each block has the safety framing co-located rather than scattered. Add the same blocked structure as a copyable preview in `README.md` near `## Install`. Borrow Claude Code's recovery-branch idiom and gh CLI's "official channels vs community" distinction explicitly.
  - Tradeoffs: medium diff, higher legibility for the agent (clear sectioning makes for less ambiguous parsing), aligns more closely with the rewrite framing in intent.md. Risk: more surface to keep in sync between README preview and canonical docs version; spec-compliance review must check both.

- **Approach C — Two-tier prompt (quickstart + full setup).** Split into two prompts in `docs/installation.md`: a short **agent-quickstart** (install + `slipway --version` only, ~5–8 lines) and a longer **agent-full-setup** (adapter init + verify + report). README previews the agent-quickstart and links to the full setup.
  - Tradeoffs: most flexible for different operator needs, but doubles the maintained surface and weakens the "single agent-agnostic prompt" framing in intent.md's In Scope. Higher risk of divergence between the two prompts over time.

- **Selected: Approach B — Structured-block rewrite, refined post-pivot to a "short pointer prompt + canonical prose" form.** Operator-confirmed 2026-05-28; refined 2026-05-28 after the first Wave 1 produced a ~55-line prompt and the operator asked for a tighter form. Final shape: the prompt code block in `docs/installation.md` and the README preview is a short (~10 line) *pointer prompt* that instructs the agent to fetch `https://signalridge.github.io/slipway/installation/` and follow the canonical guidance. The Discovery / Install / Initialize / Verify / Report detail lives as readable prose inside the same `## AI Tool Installation Prompt` section so the agent has the steps available once it fetches the page. Safety framing remains agent-agnostic and lives in the prose. The heading slug `ai-tool-installation-prompt` is preserved.

## Post-Pivot Addendum (2026-05-28)

The operator-confirmed pivot expanded scope to include a single targeted code change: reduce `MaxSlugLength` in `internal/model/identity.go` from `96` to `60`.

- **Architecture:** the constant is defined at `internal/model/identity.go:10` and consumed by `SlugifyTitle()` in the same file. `SlugifyTitle` is the single producer of new change slugs (called by `slipway new` via the new-change pipeline). No other call sites depend on the exact value — confirmed by `grep -rn "MaxSlugLength" internal/ cmd/` (only the constant declaration and the existing test reference it).
- **Patterns:** the slug truncation already trims dangling `-` after slicing, so reducing the cap requires no other adjustment.
- **Risks:** existing slugs on disk (length up to 96) are not migrated and continue to load through their stored values; only newly-generated slugs are affected. No filesystem-name compatibility issue because 60 is well under any filesystem cap.
- **Test strategy:** `internal/model/identity_test.go` has `TestSlugifyTitleLimitsLongSlugs` which asserts `len(slug) <= MaxSlugLength`. The assertion is relative to the constant, so it stays valid; if the test relies on a specific hard-coded length elsewhere, it will be updated.
- **Alternatives considered for the cap value:** 48 (aggressive, ~6-word summary), **60 (selected)** — descriptive but compact, comfortably below all filesystem name limits, and approximately twice the typical git short-hash + label combo length, 80 (mild trim from current). Operator-confirmed 60 implicitly via the pivot Approved Summary.
- **Unknowns:** none.

## Unknowns
- Resolved:
  - "Which comparable projects to model after" → Claude Code quickstart (recovery branches, step-numbering), mise (install-path ordering), gh CLI (official-vs-community framing, verified binaries), Aider (method preference ordering with safety warning).
  - "Which install paths the prompt should drive" → already settled at intake: the channels enumerated in `.goreleaser.yaml` (Homebrew Cask, GitHub Release archives, deb/rpm/apk, AUR, container, `go install`, build-from-source). Rewrite refines wording/order, not the channel set.
- Remaining: operator selection of approach A / B / C before planning bundles tasks. No other unknowns.

## Assumptions
- The README link at `README.md:166` to `docs/installation.md#ai-tool-installation-prompt` is currently working — evidence: read of `README.md:166` and `docs/installation.md:271` confirms the anchor matches the heading slug.
- The existing safety framing at `README.md:113` and `docs/installation.md:9` is the authoritative posture for the rewrite — evidence: both files use matching "documented release sources" language and the operator-confirmed intent.md Constraints carry it forward.
- mkdocs treats Markdown anchors generated from headings as stable across the rewrite as long as the heading text doesn't change — evidence: standard mkdocs/material behavior and the existing CI strict build at `README.md:196`.
- `go test ./...` and `go build ./...` are unaffected by docs-only changes — evidence: `internal/` and `cmd/` packages do not embed or reference Markdown files at compile/test time (confirmed by reading `artifacts/codebase/STRUCTURE.md`).

## Canonical References
- `artifacts/changes/ai-agent-install-prompt-and-slug-cap/intent.md` — operator-confirmed scope, acceptance, constraints.
- `README.md:113,166` — current "documented release sources" framing and the link to the docs anchor that the rewrite must preserve.
- `docs/installation.md:9,271-303` — current safety framing line and the canonical AI Tool Installation Prompt section being rewritten.
- `.goreleaser.yaml` — authoritative list of install channels published by the Slipway project.
- `artifacts/codebase/ARCHITECTURE.md`, `artifacts/codebase/STRUCTURE.md`, `artifacts/codebase/CONVENTIONS.md`, `artifacts/codebase/CONCERNS.md`, `artifacts/codebase/TESTING.md` — durable codebase context generated for this change.
- Comparable-project references (external): Claude Code quickstart (`code.claude.com/docs/en/quickstart`), mise getting-started (`mise.jdx.dev/getting-started.html`), gh CLI README (`github.com/cli/cli`), Aider install (`aider.chat/docs/install.html`).
