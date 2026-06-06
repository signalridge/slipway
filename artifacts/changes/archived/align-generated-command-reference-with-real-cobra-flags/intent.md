# Intent

## Summary
Align every surface that documents Slipway command/flag behavior with the real
Cobra command logic, in both directions: generated docs must list the real
flags, and `--help` text itself must describe actual behavior (no phantom
flags, no wrong defaults, no stale prose). Add a reverse flag-contract guard so
missing-flag drift fails closed in CI. Additionally, redesign the Slipway entry
skill so it is actually discoverable and triggers on real task intent — not only
on insider lifecycle vocabulary.

## Complexity Assessment
complex
<!-- Rationale -->
The objective spans many independent description surfaces (Cobra --help
Short/Long + flag usage, toolgen commandRegistry Arguments, command body
templates, generated skill SKILL.md surfaces, references, docs/, README), adds
an audit dimension (help text vs actual logic), adds a new regression guard, and
adds a design change to the entry skill's triggering/discovery model. Touches
public CLI/JSON/skill/doc contracts that must be reviewed as external contracts.

## In Scope
Bring all of the following into agreement with the real per-command Cobra flag
set and actual command behavior, and keep them in agreement via a guard:

- **Cobra `--help` text** (`cmd/*.go`): each command's `Short`/`Long` and every
  flag's usage string must describe real, current behavior — no phantom flags,
  correct defaults, no stale prose.
- **toolgen `commandRegistry[].Arguments`** (`internal/toolgen/toolgen.go`):
  backfill every non-hidden flag (drives `command-reference.md` + codex prompts).
  Known gaps: next(`--no-auto-pass`), status(`--format,--hydrate,--hydrate-ref,
  --root,--stats`), review(`--format,--hydrate,--hydrate-ref`),
  validate(`--format`), repair(`--format`), health(`--format,--hydrate,
  --hydrate-ref`).
- **Command body templates** (`internal/tmpl/templates/_partials/command-*-body.tmpl`)
  `## Flags` sections: backfill missing flags (e.g. pivot `--reroute/--rescope`,
  done `--all-ready`, next `--no-auto-pass`, review/validate focus/format set).
- **Generated skill SKILL.md surfaces** (slipway entry + `slipway-*` hosts):
  any place that cites a flag, command argument, or usage example.
- **References** (`references/command-reference.md`, `references/skill-index.md`)
  and any other generated markdown under generated tool dirs.
- **docs/ and README**: command/flag usage and examples (enumerate during plan).
- **Reverse flag-contract guard test**: assert every non-hidden Cobra flag
  appears in the registry Arguments (and core action flags in body templates),
  with a documented exemption list (e.g. `help`, `review:--artifact`
  unsupported-in-MVP). Extends `cmd/template_flag_contract_test.go`, whose
  current check is one-directional (phantom-flag only).
- **Entry-skill design optimization** (source: the toolgen-rendered Slipway
  entry skill template + its injected description):
  - Rewrite the `description` toward task-side trigger language so an agent that
    does NOT yet know Slipway — arriving with "change code / add a feature / fix
    a bug" intent — can match it. Fix the discovery paradox where the skill
    describes itself only in insider vocabulary (governed change / routing /
    lifecycle / uncertainty).
  - Make an upstream discovery path point AT the entry skill: the SessionStart
    hook output should surface "load the slipway skill" (turn the hook from a
    silent replacement into a trigger).
  - Tighten/clarify the three-layer responsibility boundary (entry skill vs
    `slipway:*` commands vs `slipway-*` hosts) to remove responsibility dilution.
  - Keep the entry skill's route table / entry rules / handoff consistent with
    current CLI behavior (same alignment goal as above).

## Out of Scope
- Changing actual command behavior/logic. This change aligns docs TO logic, not
  logic to docs. A genuine behavior bug surfaced by the audit is recorded, not
  fixed in this change.
- The `needs_discovery` ↔ worktree-provisioning coupling friction (the fact that
  a parallel worktree can only be provisioned by setting `needs_discovery=true`)
  — noted as a separate product observation, not fixed here.
- The unrelated active change `fix-issue-95-...` (preserved untouched).

## Constraints
- `.claude/` and other tool dirs are gitignored generated output; the fix must
  live in the generators/sources (toolgen registry, templates, cmd help, entry
  skill template), then be regenerated — not hand-edited in the generated tree.
- Public CLI/JSON/skill/doc surfaces are external contracts (reviewed as such).
- Guard must fail closed in CI; no bypass.

## Acceptance Signals
- New reverse-guard test passes: every non-hidden Cobra flag is covered by the
  registry Arguments (exemptions explicitly listed and justified).
- After `slipway init --refresh`, a drift scan (each `slipway <cmd> --help` flag
  set vs each generated surface) reports zero missing flags.
- No generated markdown, skill, doc, or README references a non-existent flag,
  and none omits a real flag for a documented command.
- Every command `--help` flag-usage line corresponds to real behavior (audit
  pass; divergences either fixed as doc or recorded as out-of-scope behavior
  notes).
- Entry skill: `description` carries task-side trigger language (not only insider
  lifecycle terms); the SessionStart hook output explicitly references loading
  the slipway skill; entry-skill flag/command citations pass the same drift guard.
- `go build ./...` and `go test ./cmd/... ./internal/toolgen/... ./internal/tmpl/...`
  green.

## Open Questions
<!-- No research-grade unknowns; surface enumeration (docs/README/skill md) is a
mechanical grep done during planning, not a research route. -->

## Approved Summary
Aligns every Slipway surface that documents command/flag behavior with the real
Cobra logic — bidirectionally (generated docs list real flags; `--help` text
describes real behavior) — across: Cobra help (cmd/*.go), toolgen
commandRegistry Arguments, command body templates, generated skill SKILL.md
surfaces, references, docs/, and README. Adds a reverse flag-contract guard so
missing-flag drift fails closed in CI. Also redesigns the Slipway entry skill so
it is discoverable on real task intent (task-side description, SessionStart hook
pointing at it, clearer three-layer boundary).

Scope boundary (user-confirmed): this change aligns docs/skills/help TO actual
logic and improves the entry skill's design; it does NOT change command runtime
behavior. A genuine behavior bug surfaced by the help-vs-logic audit is recorded
as a note, not fixed here. Out of scope: the needs_discovery↔worktree coupling
friction, and the unrelated fix-issue-95 change.

Primary acceptance: reverse-guard test green + zero-drift scan after
`slipway init --refresh` + help audit pass + entry-skill description/hook updated
+ build/test green.

Confirmed by user on 2026-06-06 (continue + add entry-skill design optimization).
