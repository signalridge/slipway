# Intent

## Summary
Fix stale generated command skills so retired command surfaces cannot remain
discoverable after CLI commands are removed.

## Complexity Assessment
simple
The scope is a bounded generator/test repair in the adapter surface area. It
does not change lifecycle state semantics, command behavior, or external API
contracts.

## In Scope
- Update the Slipway toolgen refresh/prune contract so any previously generated
  command-skill directory for a retired command is removed by class (content
  signature + command_id absent from the registry), not by a hand-maintained
  retired-name list, and covering both manifest-tracked and legacy
  manifest-absent residue.
- Keep removal fail-closed: route deletion through the ownership-manifest safety
  path so user-modified content under a retired name is preserved, never
  silently destroyed.
- Add regression coverage proving retired command skills (including a synthetic
  never-enumerated id, manifest-present residue, and every command-skill host)
  cannot persist after refresh, while user-owned/modified skills survive.
- Add a non-tautological command-surface contract check that parses each
  generated command skill `command_id` and asserts it resolves on the live root
  command tree.
- Run focused Go tests and manifest checks that cover command/adapter surfaces.

## Out of Scope
- Do not reintroduce `slipway stats`, `slipway learn`, `slipway checkpoint`,
  `slipway pivot`, `run --resume-response`, or related retired behavior.
- Do not submit root-checkout local `.codex/skills` cleanup as the PR fix by
  itself; the durable fix must live in tracked generator code and tests.
- Do not alter unrelated active changes or their governed bundles.
- Other retired generated-surface families (governance/technique/catalog host
  skills, nested command entries) plausibly share this registry-coupled cleanup
  leak; they are the same defect class but are deferred to a follow-up unless a
  generalized content-signature recognizer naturally covers them.

## Constraints
- Preserve user-owned adjacent adapter files during refresh.
- Keep the cleanup fail-closed for modified managed files.
- Keep command behavior and generated command references aligned with
  `commandRegistry`.

## Open Questions
None.

## Acceptance Signals
- Refreshing a generated adapter prunes retired command-skill directories for
  ANY retired command (including a synthetic never-enumerated id and
  manifest-present residue), across every command-skill host (REQ-001/004).
- A directory under a retired name holding user-modified content is preserved,
  not deleted (REQ-003 fail-closed).
- A contract test resolves each generated command skill `command_id` against the
  live root command tree, failing if any maps to a non-registered command
  (REQ-002; non-tautological).
- Focused `go test` coverage for `cmd` and `internal/toolgen` passes.
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check` reports the
  manifest is current. (REQ-002 generator/docs-drift signal only; it is built
  from live registries and is structurally blind to on-disk residue, so it does
  NOT falsify REQ-001 — the behavioral prune tests carry REQ-001.)

## Approved Summary
Confirmed by user on 2026-06-25T04:41:09Z: implement a PR-ready generator and
test fix for stale command skills. The change should make retired generated
command-skill directories prune reliably and add contract coverage so command
skills cannot point at commands absent from the current CLI/registry. Directly
deleting root-checkout local `.codex/skills` residue is out of scope unless it
falls out of the tracked generator behavior.
