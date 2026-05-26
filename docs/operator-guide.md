# Operator Guide

This guide is for people and agents maintaining a Slipway workspace.

## State Authority

| Path | Role |
| --- | --- |
| `.slipway.yaml` | Repository-local Slipway configuration. |
| `artifacts/changes/<slug>/change.yaml` | Current lifecycle and routing authority for an active change. |
| `artifacts/changes/<slug>/events/lifecycle.jsonl` | Append-only trace for mutating lifecycle events. |
| `artifacts/changes/<slug>/verification/*.yaml` | Skill and verification evidence. |
| `artifacts/changes/<slug>/*.md` | Intent, research, requirements, decisions, tasks, and assurance. |

Do not treat `events/lifecycle.jsonl` as a replacement for `change.yaml`. It is audit evidence only.

## Worktrees

Governed work may be bound to a dedicated worktree under `.worktrees/<slug>`. Use the worktree that owns the active governed diff:

```bash
git status --short --branch
go run . status --json
```

Avoid deciding readiness from `main...HEAD` alone. Pair branch comparisons with direct worktree status and diff checks.

## Health And Repair

Inspect before mutating:

```bash
slipway health --doctor --json
slipway validate --json
slipway status --json
```

Run repair only when the doctor output matches the observed issue:

```bash
slipway repair --json
```

Repair is intended for bounded local integrity issues such as stale locks, interrupted archives, corrupt config, or repairable layout drift.

## Verification Stack

Use targeted checks while implementing:

```bash
go test ./internal/stringutil ./internal/engine/progression ./internal/engine/governance -run 'TestHasBlockingOpenQuestions|TestAdvanceIntake_OpenQuestionsUseResolvedItemSemantics|TestTraceability.*OpenQuestions' -count=1
```

Use the full proof before closeout:

```bash
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
mkdocs build --strict
```

Run docs build only when MkDocs dependencies are available locally. CI installs MkDocs dependencies for docs verification.

## Adapter Refresh

Refresh generated AI-tool surfaces after changing templates or command contracts:

```bash
slipway init --tools all --refresh
```

Check generated path changes before committing. Codex prompt files may be written outside the project under `$CODEX_HOME/prompts`.

## Closeout

Before `done`:

1. Confirm `go run . validate --json` reports the relevant gates approved.
2. Confirm task evidence is fresh for the current run version.
3. Confirm `git diff --check`.
4. Stage intended files only.
5. Confirm `git diff --cached --check`.
6. Run `slipway done --json` when the change is done-ready.
