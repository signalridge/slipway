# Codebase Map Reference

Method-first reference for when to rely on the `codebase-map` artifact set
during context assembly. The durable output contract is the fixed
`artifacts/codebase/` directory produced by the `slipway codebase-map`
command.

## Prerequisites

- The workspace has been initialized (`slipway init`).
- `slipway codebase-map` is the authoritative entrypoint; do not invent a
  second repo-mapping command from this reference.
- Run `slipway codebase-map` when any of the following is true:
  - First touch of a brownfield area during intake or plan-audit.
  - A context-dependent signal fires (e.g. the user asks "how does this
    work", or a host skill requests grounded context).
  - The previous map output is stale (see "When stale or invalid" below).

## Durable output contract

The command writes a fixed document set under `artifacts/codebase/`:

| File | Purpose |
|------|---------|
| `STACK.md` | Languages, frameworks, build/test tooling, key dependencies |
| `INTEGRATIONS.md` | External APIs, infra bindings, datastores, queues |
| `ARCHITECTURE.md` | Module responsibilities, dependency flow, coupling |
| `STRUCTURE.md` | Directory layout, entry points, ownership hints |
| `CONVENTIONS.md` | House style, naming, error handling, logging |
| `TESTING.md` | Test layout, coverage gaps, fixture patterns |
| `CONCERNS.md` | Known risks, load-bearing invariants, tech debt |

These names are load-bearing. Downstream skills read `input_context`
fields:

- `input_context.codebase_map_dir` — absolute path to `artifacts/codebase/`.
- `input_context.codebase_map_docs` — map from short key (`stack`,
  `integrations`, `architecture`, `structure`, `conventions`, `testing`,
  `concerns`) to file path.

Do not rename files, do not consolidate into one document, and do not add
extra top-level files expecting downstream consumers to discover them.

## Assembly procedure (method-first)

1. Restate the intake question in one sentence before reading anything.
2. Invoke `slipway codebase-map` only to **scaffold** a missing, scaffold-only,
   or baseline document set. The command is idempotent; reruns populate missing
   or scaffold-only files with deterministic baseline repository facts and do
   not overwrite hand-authored substantive content — so it cannot refresh a
   `populated` document. For a `populated`/`partial` map that no longer matches
   the current change (semantically stale), do not rerun the command; re-author
   the affected documents inline instead (see "When stale or invalid").
   - `status: baseline` means the documents contain CLI-detected facts only.
     Treat them as a starting point awaiting authored verification, not as
     completed brownfield analysis.
3. Refine each baseline document with file:line citations or command
   transcripts when the task needs stronger evidence. The artifact is a
   reviewable handoff, not free-form notes.
4. Mark every assumption as assumption, not finding, until it is
   re-derived from code.
5. End with a one-screen summary covering: intent, affected seams,
   load-bearing invariants, and open questions.

## When stale or invalid

Staleness is a host-AI semantic relevance judgment, not an engine fingerprint.
The `codebase_map_status` only reports content presence (`populated`,
`baseline`, `scaffold_only`, `missing`) — it cannot tell whether a populated map
was authored for *this* change. So when you consume a `populated` (or `partial`)
map, judge its relevance yourself:

- Re-read the populated documents and ask whether their affected seams, blast
  radius, module boundaries, and concerns still describe the area **this** change
  touches. A map authored for a prior, unrelated change reads `populated` but is
  not relevant.
- If a section no longer matches the current scope, **re-author it in place** in
  `artifacts/codebase/<DOC>.md`. The assessment re-reads the files on every run,
  so an inline edit is the refresh — there is no engine staleness flag to clear
  and no command that overwrites your authored content.
- A file that is empty or holds only the scaffold template is non-durable
  (`scaffold_only`), not stale — fill it from source.

Do not rely on file mtimes, entry-point renames, or lockfile drift as staleness
signals: those are mechanical proxies that miss the real question — does the map
describe the change in front of you?

Partial output — one or two docs filled, the rest empty — is not stale, it
is unfinished. Complete the set before handing off to a planning or review
skill.

## When not to regenerate

- A context-assembly rerun for an unrelated scope does not require a full
  rebuild. Update only the documents relevant to the new scope.
- If the question is about a stable, load-bearing area already documented
  with citations, re-read rather than regenerate.

## Failure modes

| Symptom | Remediation |
|---------|-------------|
| Doc exists but contains only the scaffold | Run `slipway codebase-map`; if it remains scaffold-only, treat it as missing and fill with citations before handoff. |
| `codebase-map --json` reports `status: "baseline"` | Keep the detected facts, then add source-backed findings and citations before treating the map as reviewed context. |
| Citations reference files that no longer exist | Rerun on the affected scope; annotate historical files as removed. |
| Two docs disagree (e.g., `STRUCTURE.md` names an entry point that `ARCHITECTURE.md` ignores) | The inconsistency itself is a finding; surface it in `CONCERNS.md` rather than silently picking one. |
| Command fails with workspace-uninitialized error | Run `slipway init` first; the command assumes workspace state. |

## Anti-patterns

- Asking a second skill ("please map the repo") instead of running the
  shipped command. The command is the single entrypoint; duplicating it
  fragments the output contract.
- Treating `artifacts/codebase/` as an append-only narrative. It is a
  snapshot of the current repo state; stale entries mislead planning.
- Skipping `CONCERNS.md` because "nothing is broken". Load-bearing
  invariants and known risks belong there whether or not they are
  currently failing.
