# Skills Strengthening Plan — Wave 3 (Draft)

**Status.** Draft. Depends on Wave-2 outcomes (see `2026-04-15-skills-wave2-plan.md`
§6.5); do not open Wave-3 implementation PRs until Wave-2 PR-1 has landed
and its metrics report has been reviewed. The per-skill inventories and
hydrate binding rows below are provisional and must be re-validated
against Wave-2 metrics before Wave-3 PR-1 lands.

## 1. Motivation

Wave-1 covered the 10 high-leverage skills with deep source material.
Wave-2 covered the reference-parallel analysis family. Wave-3 covers the
remaining 9 catalog skills, which share three properties that make them
distinct from Wave-1 / Wave-2:

- **Upstream is typically thin.** Most source skills in this wave are a
  single `SKILL.md` with no `references/` shelf. The leverage comes
  from *body rebalance* + *selective script lift* + *hydrate wiring on
  workflow surfaces*, not from wholesale references migration.
- **Domain is workflow / authoring / collaboration**, not security. The
  hydrate use-case is advising planning, review, or recovery flows, not
  security investigation.
- **Script opportunity is real but narrow.** `iterate-pr` and
  `gh-review-requests` ship Python helpers (`fetch_pr_checks.py`,
  `fetch_pr_feedback.py`, `reply_to_thread.py`, `fetch_review_requests.py`)
  that can be lifted under the Wave-1 Python script contract. Other
  skills ship no helpers and stay prose-only.

This wave's value is therefore concentrated in three places (see §3–§5),
and is smaller than Wave-1 / Wave-2 by construction.

## 2. Non-goals

- No new hydrate contract shape or selection path; reuse Wave-1 PR-4a /
  PR-4b verbatim.
- No further tier-budget lift.
- No new catalog skills.
- No rewrite of any `skills_ref/` upstream.
- No attempt to force thin-source skills into the reference-heavy Wave-1
  pattern. If a skill genuinely has no material beyond `SKILL.md`, its
  Wave-3 work is confined to body rebalance and hydrate wiring; it does
  not need a `references/` directory just for uniformity.

## 3. PR-1 — Body rebalance + references where upstream warrants

**Goal.** For each of the 9 skills, either (a) tighten / expand the body
against the T1/T2 target already lifted in Wave-1, or (b) add a small
number of references where upstream provides them. No curator-authored
synthesis is permitted in this wave unless explicitly justified per row.

### Per-skill plan

| Skill | Action | Planned artifacts |
|-------|--------|-------------------|
| `scope-clarification` | Body rebalance only | None. Upstream is `trailofbits/ask-questions-if-underspecified/SKILL.md` (single file). If body is already at target, record "no change" and continue. |
| `plan-authoring` | Body rebalance + 1 reference | `references/plan-document-review-prompt.md` lifted from `superpowers/writing-plans/plan-document-reviewer-prompt.md`. This is a review-prompt, not a planning prompt; name reflects role. |
| `tdd-proof` | Body rebalance + 1 reference | `references/testing-anti-patterns.md` lifted from `superpowers/test-driven-development/testing-anti-patterns.md` (verbatim or lightly trimmed). |
| `fresh-verification-evidence` | Body rebalance only | None. Upstream is single `SKILL.md`. |
| `parallel-executor-contract` | Body rebalance only | None. Upstream is single `SKILL.md`. |
| `multi-reviewer-calibration` | Body rebalance + 1 reference | `references/review-dimensions.md` lifted from `wshobson/multi-reviewer-patterns/references/review-dimensions.md`. `alirezarezvani/pr-review-expert/SKILL.md` folds into the owning `SKILL.md` body rather than becoming a reference. |
| `git-recovery` | Body rebalance only | None. Upstream `wshobson/git-advanced-workflows` + `wshobson/block-no-verify-hook` are both single SKILL.md files. Their material folds into the body. |
| `review-comment-triage` | Body rebalance only in PR-1; references folded in prose | None in references track. The scripts track (PR-2) is where this skill gets most of its lift. |
| `ci-triage` | Body rebalance only; flag as "no strong upstream" | None. If body still lands in warning-band after rebalance, log it explicitly in PR notes — do not invent references. |

### Distillation rubric

Same as Wave-1 PR-1: keep condition-triggered operational content, drop
narrative motivation, prefer source-aligned filenames, record
collapsed/deferred source sections in provenance. The rubric's
"curator additions must be justified" rule is strict in this wave — no new
reference file may be introduced without a source ref backing it, except
where a row above explicitly calls out a synthesis with reason.

### Code changes

- `internal/tmpl/templates/skills/<id>/SKILL.md` — body edits per row.
- `internal/tmpl/templates/skills/<id>/references/*.md` — new files for
  the 3 skills that actually get references (plan-authoring, tdd-proof,
  multi-reviewer-calibration).
- `internal/tmpl/templates/skills/<id>/provenance.yaml` — extend
  `inputs:` for any new reference; update body-source links when body
  rebalance absorbs additional upstream material.
- `internal/engine/capability/registry_default.go`,
  `registry_b2.go`, `registry_b4.go`, `registry_b5.go` — populate
  `Skill.HydrateReferences` only for the 3 skills that now ship
  references. Thin-source skills keep an empty hydrate list (Wave-1
  PR-4a tests already accept empty).

### Tests to add / extend

- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences` —
  extend input to include `plan-authoring`, `tdd-proof`,
  `multi-reviewer-calibration`. Do **not** extend to the other 6 Wave-3
  skills — they have no references by design.
- `internal/engine/capability/gates_test.go::TestSizeBudgetsForRegisteredSkills`
  (Wave-1) auto-covers the body rebalance.
- No new hydrate-contract tests are added in Wave-3 PR-1; Wave-1's
  `TestHydrateReferencesMirrorRegistry` +
  `TestHydrateReferencesResolveToFiles` auto-extend.

### Acceptance

- Every Wave-3 skill's body is within T1/T2 target (not just hard-max);
  warning-band outcomes must be explicit in PR notes.
- The 3 new reference files meet the 24 KB / file and 64 KB / skill caps.
- PR notes state, per thin-source skill, whether body rebalance
  absorbed additional upstream material (with source links in
  `provenance.yaml`) or was a no-op.
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  passes.

## 4. PR-2 — Script lift from `iterate-pr` / `gh-review-requests`

**Goal.** Lift the four Python helpers from two upstream getsentry
skills into the Slipway script track, narrowed to Slipway's runtime
contract. `review-comment-triage` is the single owning skill for all
four; they are the PR feedback / review-request flow that this skill is
designed for.

### Planned scripts

| Script | Owning skill | Purpose | Lift source |
|--------|--------------|---------|-------------|
| `scripts/fetch-pr-checks.py` | `review-comment-triage` | Fetch CI check run status for a PR; deterministic JSON output; no side effects. | `getsentry/iterate-pr/scripts/fetch_pr_checks.py` |
| `scripts/fetch-pr-feedback.py` | `review-comment-triage` | Fetch review comments / review threads for a PR; deterministic JSON output. | `getsentry/iterate-pr/scripts/fetch_pr_feedback.py` |
| `scripts/reply-to-thread.py` | `review-comment-triage` | Post a reply to a specific PR review thread. **Write side effect — must require an explicit `--confirm` flag and print the full request body to stderr before posting.** | `getsentry/iterate-pr/scripts/reply_to_thread.py` |
| `scripts/fetch-review-requests.py` | `review-comment-triage` | List open review requests for a user; deterministic JSON output. | `getsentry/iterate-pr/scripts/../../gh-review-requests/scripts/fetch_review_requests.py` (correcting path: `getsentry/gh-review-requests/scripts/fetch_review_requests.py`) |

### Constraints

- Reuse Wave-1 PR-2 Python runtime contract verbatim.
- **`reply-to-thread.py` is write-capable.** Per the user-level
  blast-radius rule in the Slipway user's CLAUDE.md (authorization stays
  within scope), this script must default to dry-run: print the intended
  HTTP request and exit non-zero unless `--confirm` is passed. No
  exceptions. This is not just a Slipway convention — it is the
  safe-by-default posture that the owning skill (`review-comment-triage`)
  should model for its callers.
- All four scripts depend on `gh` or a GitHub token. The owning
  `SKILL.md` must describe the token requirement; scripts must fail
  fast with actionable messages when credentials are missing or `gh` is
  unavailable.
- No dependency on `gh` beyond what the upstream scripts already
  require. If an upstream script uses `requests` directly, keep that;
  do not introduce `gh` shelling just for uniformity.
- Provenance must record the lift source for each script and any
  narrowing (e.g., "removed `--verbose` debug mode present in upstream
  but not used here").

### Tests to add

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` — extends.
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` — extends
  (all four pass `python3 -m py_compile`).
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`:
  - `fetch-pr-checks.py` / `fetch-pr-feedback.py` /
    `fetch-review-requests.py`: with `GH_TOKEN=invalid` (or equivalent),
    assert stable credential-error output; no network fallback.
  - `reply-to-thread.py`: without `--confirm`, assert dry-run output
    contains the intended request line and script exits non-zero; with
    `--confirm` but `GH_TOKEN=invalid`, assert credential-error path.

### Acceptance

- All four scripts pass static checks and fixture contracts.
- `reply-to-thread.py` cannot post without `--confirm` (enforced by test).
- `init --tools codex --refresh` writes the scripts into the generated
  skill tree.

## 5. PR-3 — Hydrate wiring on workflow surfaces

**Goal.** Populate hydrate references for the 3 Wave-3 skills that have
references, plus the script-carrying `review-comment-triage`, on the
selection paths each skill actually participates in. Thin-source skills
(no references, no scripts) do not get hydrate rows in this wave.

### First-wave binding table (tentative)

Verify each row against the b0 / b2 / b4 / b5 registry constructors
before authoring tests; registry is authority. If a row's binding surface
does not match the intended hydrate surface, flag in PR notes rather than
adding a new binding.

| Skill | Existing bindings (to be re-verified) | Selection path | Initial hydrated refs | First surfaced in |
|-------|----------------------------------------|----------------|-----------------------|-------------------|
| `plan-authoring` | TBD | Manual explicit via `--mode=plan-authoring` | `plan-document-review-prompt.md` | Wherever manual `--mode` admits; likely `review` or a plan-specific surface |
| `tdd-proof` | TBD | Manual explicit via `--mode=tdd-proof` | `testing-anti-patterns.md` | `validate` or `review` |
| `multi-reviewer-calibration` | TBD | Manual explicit via `--mode=multi-reviewer-calibration` | `review-dimensions.md` | `review` |
| `review-comment-triage` | TBD | Manual explicit via `--mode=review-comment-triage` | None (scripts-only skill; hydrate refs empty) | Scripts track exposes helpers; hydrate row stays empty |

Note: `review-comment-triage` is intentionally listed with **empty
hydrate refs**. Its Wave-3 value is the scripts track, not reference
material. The empty row is included to document that the absence is
intentional, not an oversight.

### Code changes

- `internal/engine/capability/<registry_file>.go` — populate
  `Skill.HydrateReferences` for the 3 reference-carrying skills.
- No changes in `cmd/*.go`; Wave-1 PR-4a rendering handles it.

### Tests to add

- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  — auto-extends.
- `cmd/hydrate_view_test.go` — golden cases for the 3 reference-carrying
  skills' manual-explicit paths.

### Acceptance

- The 3 reference-carrying Wave-3 skills surface hydrate keys on at least
  one command surface.
- `review-comment-triage` does not surface hydrate keys anywhere —
  asserted by a negative golden in `cmd/hydrate_view_test.go`.
- No regression in existing `cmd/...` / `capability` golden tests.

## 6. Execution order and gates

1. **PR-1 first.** Body rebalance + the 3 new references.
2. **PR-2 second.** Scripts lift. Depends on PR-1 only for ownership
   clarity (which skill owns which script); body rebalance is a
   precondition for touching the owning `SKILL.md` without churn.
3. **PR-3 third.** Hydrate wiring. Depends on PR-1 references existing
   on disk; cannot be parallelized with PR-1.
4. Each PR runs the same three hard gates as Wave-1 §8.5 / Wave-2 §6.3.
5. **English + zh-CN stay in lockstep**, same rule.
6. **Wave completion report.** Within 7 days of Wave-3 PR-3 merge,
   produce a short closeout report covering: which thin-source skills
   stayed no-op in PR-1, whether `review-comment-triage` scripts cover
   the intended operator flow, and whether any warning-band body sizes
   persisted. After this report, the 25-skill catalog is considered
   "strengthening-complete" for the current source corpus; further work
   happens only when `skills_ref/` is re-imported or specific gaps are
   raised by operators.

## 7. Out of scope

- Rewriting `skills_ref/` provenance; Wave-3 only adds pointers.
- Adding typed partials to any Wave-3 skill. `independent-review`
  remains the PR-3 typed-partials reference example; no other Wave-3
  skill becomes a typed-partial host in this wave.
- Any change to hydrate contract, selection paths, tier budgets, or
  other Wave-1 infrastructure. If a Wave-3 implementation PR would need
  such a change, stop and escalate.
- Adapting `alirezarezvani/prompt-governance` — still deferred per
  Wave-1 §9, awaiting plugin-shape support.
- Lifting scripts from any skill not explicitly listed in §4. In
  particular, `alirezarezvani/incident-response/scripts` and
  `incident-commander/scripts` remain deferred (already noted in Wave-1
  PR-1 for `incident-response`); their Python responders do not belong
  in Wave-3 because they target a different skill family.
