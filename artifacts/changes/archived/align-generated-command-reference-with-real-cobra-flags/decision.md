# Decision
## Project Context
- Tech Stack: Go
- Conventions: Slipway Agent Principles (CLAUDE.md). Fix generator sources +
  regenerate; never hand-edit the gitignored generated tree.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

Three approaches were evaluated (full detail in `research.md`):

- **Approach A — Handwritten surfaces + reverse contract guard.** Backfill the
  handwritten surfaces (registry Arguments, body `## Flags`, `--help` text, entry
  skill, docs) and lock them with a reverse cobra→surface guard test. Prose body
  templates keep their design; only core action-flag omissions are added.
  Tradeoffs: preserves authored flag descriptions and prose design; focused,
  low-risk; guard stops future missing-flag drift. Body surface stays curated
  (not exhaustively complete by construction); the `--help`-vs-logic audit is
  manual.
- **Approach B — Generator single-source-of-truth.** Extract flag metadata into a
  shared low-level package; toolgen and `--help` both derive Arguments from the
  Cobra FlagSet. Tradeoffs: most durable, but a large refactor (break the
  toolgen↔cmd import cycle, new shared package), loses handwritten usage prose
  and the curated prose templates, higher risk, scope far beyond this objective.
- **Approach C — Doc backfill only, no guard.** Fix current gaps, add no guard.
  Tradeoffs: smallest, but drifts again on the next added flag; fails the
  "comprehensive + don't-recur" intent.

### Constraints (from intent)
- `.claude/` and other tool dirs are gitignored generated output; fixes live in
  the generator sources (toolgen registry, templates, cmd help, entry skill
  template), then regenerate — not hand-edited in the generated tree.
- Public CLI/JSON/skill/doc surfaces are external contracts (reviewed as such).
- Guard must fail closed in CI; no bypass.

## Selected Approach
**Approach A (user-confirmed 2026-06-06).** Backfill every handwritten
flag-description surface to match the real Cobra flag set and behavior, and add a
reverse flag-contract guard so missing-flag drift fails closed. Body prose
templates (`new`/`status`/`init`/`repair`) keep their curated structure; only
real core action-flag omissions are added. The entry skill is redesigned for
discoverability (task-side description, three-layer boundary, hook trigger).
Approach B (auto-derive) is explicitly deferred as a larger follow-up; this
change does not break the toolgen↔cmd cycle.

Honors the documented constraints above (source-then-regenerate; external
contracts; fail-closed guard).

## Interfaces and Data Flow
- New exported function `toolgen.CommandArguments(id string) string` (wraps the
  existing unexported `commandArguments`), mirroring `CommandDescription` /
  `CommandClassification`. This is the contract the reverse guard asserts against.
- No runtime/CLI behavior change, no JSON schema change, no new flags. Cobra
  command registration is unchanged; only help *text* and generated *descriptions*
  are corrected.
- Data flow unchanged: `commandRegistry.Arguments` → `command-reference.md` +
  codex prompts; `command-*-body.tmpl` → command surfaces; `cmd/*.go` flag usage
  → `--help`. The change makes these consistent with the FlagSet, not rewired.

## Rollout and Rollback
- Rollout: edit generator sources → `slipway init --refresh` → run guard test +
  drift scan. Single PR from worktree `feat/align-generated-command-reference-with-real-cobra-flags`.
- Rollback: pure `git revert` of the PR; no migration, no persisted state, no
  data. Generated `.claude/` tree is reproducible via `slipway init --refresh`
  from any commit, so rollback cannot leave a half-migrated surface.
- Verification command: `go test ./cmd/... ./internal/toolgen/... ./internal/tmpl/...`
  plus the `--help`-vs-generated drift scan.

## Risk
- LOW overall: docs/metadata/help/test alignment, no runtime logic change;
  fully reversible (git).
- Package cycle (toolgen↔cmd): mitigated — the reverse guard lives in the `cmd`
  test package, which already imports both.
- Entry-skill `description` rewrite is subjective (MEDIUM): mitigated by human
  review of the trigger language at S3.
- `--help`-vs-logic audit may surface a real behavior bug: bounded — recorded as
  an out-of-scope note, not fixed here (no scope creep into engine behavior).
- Guardrail domains: NONE.
