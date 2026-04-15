# Skills Strengthening Plan — Wave 3 (Draft)

**Status.** Draft. Depends on the Wave-2 completion gate (see
`2026-04-15-skills-wave2-plan*.md` §6 item 6) and the full landing of
`2026-04-15-route-surface-refactor-plan*.md` PR-1 / PR-2 / PR-3 per that
cross-plan ordering. Do not open Wave-3 implementation PRs until Wave-2
PR-1 / PR-2 / PR-3 have all landed, its closeout / metrics report has been
reviewed, and the route-surface refactor is complete. The per-skill
inventories and hydrate binding rows below are provisional and must be
re-validated against Wave-2 metrics and the post-refactor surface model before
Wave-3 PR-1 lands.

## 1. Motivation

Wave-1 covered the 10 high-leverage skills with deep source material.
Wave-2 covered the reference-parallel analysis family on top of the
route-surface refactor and explicitly froze `coverage-analysis` as a no-op
under that same post-refactor surface. Wave-3 therefore covers the final 9
remaining scoped skills in this strengthening family on the same
post-refactor surface model (`--focus` / `--view` aliases, no
`BindingCommandManual`, host/support-only for hosts-only skills), and shares
three properties that make this wave distinct from Wave-1 / Wave-2:

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
  that are candidates for lift, but `fetch_review_requests.py` currently has a
  higher upstream interpreter floor (`>=3.12`). Wave-3 only ships these helpers
  if that script is genuinely narrowed to the shared Wave-1 Python contract.
  Other skills ship no helpers and stay prose-only.

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
- No late backfill of `coverage-analysis` into Wave-3. That skill was
  explicitly frozen in Wave-2 because it is already within the T1 target,
  lacks a worthwhile upstream `references/` / `scripts` shelf, and already
  fits the post-refactor host path without new strengthening work.

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
| `ci-triage` | Body rebalance in PR-1; add 1 script lift in PR-2 | No references. `getsentry/iterate-pr/scripts/fetch_pr_checks.py` lands in PR-2 as `scripts/fetch-pr-checks.py`; PR-1 still limits itself to body tightening and does not force-fit references. |

### Distillation rubric

Same as Wave-1 PR-1: keep condition-triggered operational content, drop
narrative motivation, prefer source-aligned filenames, and capture
collapsed/deferred source sections in the PR mapping log. The rubric's
"curator additions must be justified" rule is strict in this wave — no new
reference file may be introduced without a source ref backing it, except
where a row above explicitly calls out a synthesis with reason.

### Code changes

- `internal/tmpl/templates/skills/<id>/SKILL.md` — body edits per row.
- `internal/tmpl/templates/skills/<id>/references/*.md` — new files for
  the 3 skills that actually get references (plan-authoring, tdd-proof,
  multi-reviewer-calibration).
- `internal/engine/capability/registry_default.go`,
  `registry_b4.go` — populate
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
  `TestFrontmatterMirrorsRegistryHydrateReferences` +
  `TestHydrateReferencesResolveToFiles` auto-extend.

### Acceptance

- Every Wave-3 skill's body is within T1/T2 target (not just hard-max);
  warning-band outcomes must be explicit in PR notes.
- The 3 new reference files meet the 24 KB / file and 64 KB / skill caps.
- PR notes state, per thin-source skill, whether body rebalance
  absorbed additional upstream material (with source links in the mapping
  notes) or was a no-op.
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  passes.

## 4. PR-2 — Script lift from `iterate-pr` / `gh-review-requests`

**Goal.** Lift the four Python helpers from two upstream getsentry
skills into the Slipway script track, narrowed to Slipway's runtime
contract, and split ownership along the nearest existing skill boundary:
`ci-triage` owns CI-status fetching, while `review-comment-triage` owns
review-feedback / thread-reply / review-request helpers. Do not force all
four helpers under one skill when their operational contracts differ.

### Planned scripts

| Script | Owning skill | Purpose | Lift source |
|--------|--------------|---------|-------------|
| `scripts/fetch-pr-checks.py` | `ci-triage` | Fetch CI check run status for a PR; deterministic JSON output; no side effects. | `getsentry/iterate-pr/scripts/fetch_pr_checks.py` |
| `scripts/fetch-pr-feedback.py` | `review-comment-triage` | Fetch review comments / review threads for a PR; deterministic JSON output. | `getsentry/iterate-pr/scripts/fetch_pr_feedback.py` |
| `scripts/reply-to-thread.py` | `review-comment-triage` | Post a reply to a specific PR review thread. **Write side effect — must require an explicit `--confirm` flag and print the full request body to stderr before posting.** | `getsentry/iterate-pr/scripts/reply_to_thread.py` |
| `scripts/fetch-review-requests.py` | `review-comment-triage` | List open review requests for a user; deterministic JSON output. | `getsentry/gh-review-requests/scripts/fetch_review_requests.py` |

### Constraints

- Reuse Wave-1 PR-2 Python runtime contract verbatim.
- **`reply-to-thread.py` is write-capable.** Per the user-level
  blast-radius rule in the Slipway user's CLAUDE.md (authorization stays
  within scope), this script must default to dry-run: print the intended
  HTTP request and exit non-zero unless `--confirm` is passed. No
  exceptions. This is not just a Slipway convention — it is the
  safe-by-default posture that the owning skill (`review-comment-triage`)
  should model for its callers.
- All four scripts depend on `gh` or a GitHub token. Both owning
  `SKILL.md` files (`ci-triage`, `review-comment-triage`) must document
  the helper-specific token requirement; scripts must fail fast with
  actionable messages when credentials are missing or `gh` is unavailable.
- No dependency on `gh` beyond what the upstream scripts already
  require. If an upstream script uses `requests` directly, keep that;
  do not introduce `gh` shelling just for uniformity.
- `fetch-review-requests.py` is the only helper in scope whose upstream file
  currently declares a higher interpreter floor (`requires-python >=3.12`).
  Wave-3 does **not** silently inherit that exception. The committed path is to
  narrow it to the shared Wave-1 Python contract before inclusion and record
  that interpreter-floor narrowing in PR notes. If that narrowing cannot be
  done cleanly, stop and amend this plan instead of shipping a one-off Python
  floor.
- Wave-3 does **not** own any provenance bookkeeping. Any metadata or
  source-coverage cleanup belongs only to the closeout destination,
  `2026-04-16-knowledge-only-refactor-plan*.md`. If a proposed helper lift
  cannot land cleanly without touching that cleanup surface, stop and amend
  scope instead of widening this wave.
- PR notes must record the lift source for each script and any narrowing
  (e.g., "removed `--verbose` debug mode present in upstream but not used
  here").

### Code changes

- `internal/tmpl/templates/skills/ci-triage/scripts/fetch-pr-checks.py`
  — add the CI-status helper.
- `internal/tmpl/templates/skills/review-comment-triage/scripts/fetch-pr-feedback.py`
  — add the review-feedback helper.
- `internal/tmpl/templates/skills/review-comment-triage/scripts/reply-to-thread.py`
  — add the default-dry-run thread-reply helper.
- `internal/tmpl/templates/skills/review-comment-triage/scripts/fetch-review-requests.py`
  — add the review-request helper.
- `internal/tmpl/templates/skills/ci-triage/SKILL.md` and
  `internal/tmpl/templates/skills/review-comment-triage/SKILL.md` —
  document helper entrypoints, credential requirements, and read/write
  posture; do not leave those runtime prerequisites only inside script
  comments.

### Tests to add

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` — extends.
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` — extends
  (all four pass `python3 -m py_compile` only after the shared-contract
  narrowing above; no Wave-3 script keeps a higher interpreter floor).
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
- No shipped Wave-3 script keeps a one-off Python runtime floor above the
  shared Wave-1 contract.
- No Wave-3 PR expands into provenance bookkeeping just to mirror helper-lift
  source metadata; that cleanup belongs only to
  `2026-04-16-knowledge-only-refactor-plan*.md`.

## 5. PR-3 — Hydrate wiring on workflow surfaces

**Goal.** Verify and, if needed, minimally fix hydrate behavior for the 3
Wave-3 reference-carrying skills on the selection paths each actually owns
under the post-refactor surface model: host-embedded paths for host/support-only
skills, and the explicit-focus alias for `multi-reviewer-calibration`. The
script-carrying `ci-triage` and `review-comment-triage` do not get routed
hydrate surfaces in this wave. Thin-source skills (no references, no scripts)
do not get hydrate rows at all. The actual `Skill.HydrateReferences`
declarations land earlier in PR-1 together with the reference/frontmatter
changes.

### First-wave binding table

Verify each row against the b0 / b2 / b4 / b5 registry constructors and
the surface-policy registry before authoring tests; the registries are
authority.

| Skill | Post-refactor exposure | Selection path | Initial hydrated refs | First surfaced in |
|-------|------------------------|----------------|-----------------------|-------------------|
| `plan-authoring` | host/support-only (refactor plan §5.5) | host-embedded on `plan-audit` host (already the binding shape in `registry_default.go`); no public `--focus` or `--mode` selector | `plan-document-review-prompt.md` | `plan-audit` host path (surfaces whenever the planning host is active) |
| `tdd-proof` | host/support-only (refactor plan §5.5) | host-embedded on `tdd-governance` / `wave-orchestration` + technique-hint on `tdd-governance` (already the binding shape in `registry_default.go`); no public `--focus` or `--mode` selector | `testing-anti-patterns.md` | whichever of those hosts is active |
| `multi-reviewer-calibration` | `--focus calibration` on `review` (refactor plan §5.3); retains `code-quality-review` host-embedded attachment | explicit focus alias resolves through surface-policy to backing skill; host path continues to attach the skill as a support without its own hydrate surface (refactor's `TestCalibrationHostAttachmentSurvivesFocusMigration` protects this) | `review-dimensions.md` | `review --focus calibration` |
| `ci-triage` | suggested-only on `repair` / `status` (refactor plan §5.2); no public explicit selector | resolver `SuggestedCapabilities[]`; scripts track exposes helper | None (scripts-only skill; hydrate refs empty) | suggestion channel on `repair` / `status`; hydrate surface stays empty |
| `review-comment-triage` | suggested-only on `repair` (refactor plan §5.2); no public explicit selector | resolver `SuggestedCapabilities[]`; scripts track exposes helpers | None (scripts-only skill; hydrate refs empty) | suggestion channel on `repair`; hydrate surface stays empty |

Note on the script-only suggested skills: `ci-triage` and
`review-comment-triage` are reached through their trigger clauses via
`SuggestedCapabilities[]`, not by a public selector — the route-surface
refactor's PR-3 removed `BindingCommandManual` from the taxonomy, so no
manual-selector path exists for these skills. Their Wave-3 value is the
scripts track, not reference material. The empty hydrate rows are
documented here so the absence is intentional, not an oversight.

### Code changes

- No new `Skill.HydrateReferences` declarations in PR-3. Those records land in
  PR-1 so the existing frontmatter-vs-registry gates remain satisfied from the
  first reference-bearing PR.
- No new bindings are added just to carry hydrate refs. Wave-1 PR-4a already
  surfaces hydrate keys on host-embedded paths when
  `Skill.HydrateReferences` is non-empty and the host is active, and the
  route-surface refactor's PR-2 already routes `--focus calibration`
  through surface-policy to the backing skill's hydrate keys.
- Default expectation: `cmd/*.go` and production resolver code do not need to
  change here. If the post-refactor host / focus path fails to surface the
  already-declared refs, PR-3 carries only the minimal fix required to restore
  the documented path.
- 32 KB hydrate output cap from Wave-1 PR-4b applies.

### Tests to add

- `internal/engine/capability/gates_test.go::TestFrontmatterMirrorsRegistryHydrateReferences`
  — auto-extends.
- `internal/engine/capability/resolver_test.go` — add cases proving:
  - `plan-authoring` hydrate keys surface on the `plan-audit` host path.
  - `tdd-proof` hydrate keys surface on the `tdd-governance` (and
    `wave-orchestration`) host path.
  - `multi-reviewer-calibration` hydrate keys surface on `--focus
    calibration` via surface-policy; the `code-quality-review` host path
    continues to attach the skill without its own hydrate surface.
  - `ci-triage` and `review-comment-triage` never surface hydrate keys on
    their suggestion paths.
- `cmd/hydrate_view_test.go` — golden cases for `review --focus
  calibration` listing `multi-reviewer-calibration/review-dimensions.md`;
  negative golden: `review --mode=multi-reviewer-calibration` returns the
  refactor's `unknown_route_mode` usage error, and
  `review --focus calibration` does not list `ci-triage/*` or
  `review-comment-triage/*` hydrate keys.

### Acceptance

- `plan-authoring`, `tdd-proof`, and `multi-reviewer-calibration` surface
  hydrate keys on the post-refactor selection paths described in the
  binding table above.
- `ci-triage` and `review-comment-triage` do not surface hydrate keys
  anywhere — asserted by negative cases in `resolver_test.go` and
  `cmd/hydrate_view_test.go`.
- No raw `--mode=<skill-id>` selector is reintroduced; attempting one
  must hit the refactor's `unknown_route_mode` path.
- No regression in existing `cmd/...` / `capability` golden tests.

## 6. Execution order and gates

1. **PR-1 first.** Body rebalance + the 3 new references.
2. **PR-2 second.** Scripts lift. Depends on PR-1 only for ownership
   clarity (which skill owns which script); body rebalance is a
   precondition for touching the owning `SKILL.md` without churn.
3. **PR-3 third.** Hydrate wiring. Depends on PR-1 references existing
   on disk; cannot be parallelized with PR-1.
4. Each PR runs the same three hard gates as Wave-1 §8.5 / Wave-2 §6.3,
   adapted to the post-refactor surface model. Command smoke checks are:
   - `review --focus calibration --json`
   - `review --list-focuses --format=json` (must include `calibration`)
   - a negative smoke asserting `review --mode=multi-reviewer-calibration`
     (or any other raw skill-id selector) returns
     `unknown_route_mode`
   - host-path verification that `plan-authoring` / `tdd-proof` hydrate
     keys appear only when the relevant host is active (covered by the
     resolver tests in §5, not by a user-invoked command smoke)
5. **English + zh-CN stay in lockstep**, same rule.
6. **Wave completion report.** Within 7 days of Wave-3 PR-3 merge,
   produce a short closeout report covering: which thin-source skills
   stayed no-op in PR-1, whether `review-comment-triage` scripts cover
   the intended operator flow, whether any warning-band body sizes
   persisted, and whether the host-path hydrate surfacing for
   `plan-authoring` / `tdd-proof` behaves as described in §5. After this
   report is reviewed, the scoped strengthening set is considered complete
   for the current source corpus, and
   `2026-04-16-knowledge-only-refactor-plan*.md` may proceed. Further
   strengthening work happens only when `skills_ref/` is re-imported or
   specific gaps are raised by operators.

## 7. Out of scope

- Rewriting `skills_ref/`; Wave-3 only adds pointers.
- Adding typed partials to any Wave-3 skill. `independent-review`
  remains the PR-3 typed-partials reference example; no other Wave-3
  skill becomes a typed-partial host in this wave.
- Any change to hydrate contract, selection paths, tier budgets, or
  other Wave-1 infrastructure. If a Wave-3 implementation PR would need
  such a change, stop and escalate.
- This wave is limited to the skill set enumerated in §§3–5. It does not
  rescope unrelated plugin-shape or incident-responder sources into the current
  strengthening family.
