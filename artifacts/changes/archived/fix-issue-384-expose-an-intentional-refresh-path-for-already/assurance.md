# Assurance

## Scope Summary

This change fixes issue #384 by adding an explicit `--refresh-current` option
to `slipway evidence skill` for the narrow case where a selected S3 review
skill already has current passing evidence and an operator intentionally reruns
that review.

Delivered scope:

- `cmd/evidence.go`: parses `--refresh-current`, threads it into evidence skill
  actionability, and permits replacement only inside the selected S3
  review/current-passing branch.
- `cmd/evidence_skill_test.go`: covers ordinary duplicate rejection and
  explicit refresh success for `spec-compliance-review`, `code-quality-review`,
  and `independent-review`.
- `internal/tmpl/templates/_partials/command-evidence-body.tmpl`: documents the
  flag in generated command-skill invocation, contract, and flags text.
- `internal/toolgen/toolgen.go`: exposes `[--refresh-current]` in the
  command-registry arguments used by generated command skills.
- `internal/toolgen/surface_manifest.go` and
  `internal/toolgen/surface_manifest_test.go`: add and pin the
  `evidence-skill-refresh-current-json` public inventory row.
- `docs/SURFACE-MANIFEST.json`: commits the regenerated inventory row.
- `docs/commands.md`, `docs/reference/commands.md`, `docs/ja/commands.md`,
  `docs/ja/reference/commands.md`, `docs/zh/commands.md`, and
  `docs/zh/reference/commands.md`: expose the full refresh command token and
  selected-review-only scope.

Out-of-scope items remain out of scope: subagent configuration redesign,
execution.auto behavior, plan-audit semantic rigor, release publishing, and
broad evidence schema redesign.

## Verification Verdict

Verdict: pass for the repaired findings, subject to the normal governed
`slipway run` gate before done-ready is re-established.

The implementation satisfies the approved requirements while preserving default
fail-closed duplicate evidence behavior. Selected S3 review evidence is
re-recorded with explicit `same_context_degraded` fallback handles because this
host did not have user-authorized native subagent dispatch for the review batch.

## Evidence Index

- `verification/spec-compliance-review.yaml`: pass; records `layer:R0=pass`,
  `scope_contract:pass`, `negative_path:pass`, and a distinct review context
  handle.
- `verification/code-quality-review.yaml`: pass; records `layer:IR1=pass`,
  `toolchain_compat:pass`, and a distinct review context handle.
- `verification/independent-review.yaml`: pass; records independent review
  pass with a distinct review context handle.
- `verification/logs/ship-suite.txt`: fresh authoritative
  `go test ./... -count=1` transcript.
- `verification/logs/golangci-lint.txt`: `golangci-lint run ./...` transcript.
- `verification/logs/diff-check.txt`: `git diff --check` transcript.
- `verification/logs/surface-manifest-check.txt`:
  `go run ./internal/toolgen/cmd/gen-surface-manifest --check` transcript.
- `verification/logs/surface-manifest-tests.txt`: focused manifest/docs/args
  contract test transcript.

## Requirement Coverage

- REQ-001 explicit current review evidence refresh: covered by
  `cmd/evidence.go` and the refresh-current regression tests.
- REQ-002 duplicate review evidence remains fail-closed by default: covered by
  duplicate-rejection regression tests and unchanged fail-closed validation.
- REQ-003 refresh path is discoverable: covered by CLI help, generated command
  body template, command-registry arguments, committed surface manifest, focused
  manifest tests, English docs, and localized Japanese/Chinese docs.

## Added Args Visibility

The newly added arg is visible in the source-owned and committed public
surfaces:

- CLI help lists `--refresh-current`.
- `internal/toolgen/toolgen.go` command arguments include `[--refresh-current]`.
- The command-skill body template includes the refresh invocation and flag.
- `TestCodexCommandSkillsUseCommandRegistryArguments` proves fresh generated
  Codex command skills render the updated arguments.
- `docs/SURFACE-MANIFEST.json` includes
  `evidence-skill-refresh-current-json`.
- English, Japanese, and Chinese command docs include the full command token.

Caveat: the root checkout's existing
`.codex/skills/slipway-evidence/SKILL.md` is a generated local cache from an
older export and still lacks `--refresh-current` until regenerated. The target
worktree has no `.codex/skills/slipway-evidence` export. This change fixes the
generation sources and tests the generated command-skill argument contract; it
does not hand-edit that stale root-local cache.

## Residual Risks and Exceptions

- Audit-history tradeoff: the selected approach replaces the current
  verification record instead of storing supplemental review history. This was
  accepted in `decision.md` as the smallest fix for #384.
- Same-context degraded review: native subagent dispatch was not user-authorized
  in this host, so review and ship verification use the engine-exposed degraded
  fallback path with explicit references.
- Local generated cache drift: root `.codex/skills` must be refreshed after
  this branch is applied if an agent reads that cache directly.
- No guardrail domain is active for this change; no SAST high-risk token is
  required.

No unresolved blockers remain in the implemented scope.

## Rollback Readiness

Rollback is local and low-risk:

- Remove `--refresh-current` flag registration and the boolean path through
  `validateEvidenceSkillActionable`.
- Restore the previous actionable validation signature and remediation text.
- Remove the refresh-success regression rows and documentation updates.
- Remove `evidence-skill-refresh-current-json` from the manifest generator and
  regenerate `docs/SURFACE-MANIFEST.json`.
- Re-run `go test ./cmd -run 'TestEvidenceSkill' -count=1`,
  `go test ./internal/tmpl ./internal/toolgen -count=1`,
  `go run ./internal/toolgen/cmd/gen-surface-manifest --check`,
  `golangci-lint run ./...`, and `go test ./... -count=1`.

No schema migration, data migration, irreversible operation, or external API
rollback is needed.

## Archive Decision

Archive decision for this turn: do not archive yet. The user requested repair
and confirmation, not `slipway done`.

This assurance supports returning the active change to done-ready. Active
validation/readiness proof must be captured before any future `slipway done`;
archival must use the active `slipway done` gate rather than treating this
active bundle as already archived.
