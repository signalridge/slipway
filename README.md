<div align="center">

<img alt="Slipway - Governance CLI for AI-assisted software delivery" src="docs/assets/brand/slipway-mark.svg" width="112">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=materialformkdocs&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>&nbsp;
  <a href="https://pkg.go.dev/github.com/signalridge/slipway"><img alt="Go Reference" src="https://img.shields.io/badge/Go-Reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
</p>

<p>
  <a href="docs/installation.md"><img alt="Homebrew Cask" src="https://img.shields.io/badge/Homebrew_Cask-FBB040?style=flat-square&logo=homebrew&logoColor=black"></a>
  <a href="docs/installation.md"><img alt="Scoop" src="https://img.shields.io/badge/Scoop-00BFFF?style=flat-square&logo=windows&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="AUR" src="https://img.shields.io/badge/AUR-1793D1?style=flat-square&logo=archlinux&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?style=flat-square&logo=nixos&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Docker" src="https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="deb" src="https://img.shields.io/badge/deb-A81D33?style=flat-square&logo=debian&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="rpm" src="https://img.shields.io/badge/rpm-EE0000?style=flat-square&logo=redhat&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="apk" src="https://img.shields.io/badge/apk-0D597F?style=flat-square&logo=alpinelinux&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
</p>

[Documentation](https://signalridge.github.io/slipway/) |
[Start Here](docs/start-here.md) |
[Quick Start](#quick-start) |
[Installation](docs/installation.md) |
[Release Notes](CHANGELOG.md)

</div>

# Slipway

**A local, Git-native governance CLI for AI-assisted software delivery. Your
agent writes the code; Slipway decides when the change is actually done.**

AI coding agents are fast, but they can skip verification, drift from the plan,
or report "done" before the current worktree proves it. Slipway turns one unit
of work into a governed change with lifecycle state, planning artifacts, task
evidence, review evidence, and a final archive that stays in the repo.

Slipway is not a hosted service, project tracker, or replacement for an AI
coding tool. It is the control plane that makes agent work legible and
fail-closed.

The core advantage is not another checklist. Slipway separates the current
worktree, generated host instructions, lifecycle state, and review contexts,
then makes the CLI recompute whether those pieces still agree.

## Why Slipway?

| Capability | What it changes |
| --- | --- |
| **Compiled done gate** | `slipway done` rechecks current review, verification, scope, and guardrail proof before archive. Missing or stale evidence blocks finalization. |
| **Thin AI adapters** | Generated Claude, Codex, Cursor, Gemini, and OpenCode files route agents back to the CLI instead of becoming separate workflow engines. |
| **Plain-language entry** | After `slipway init --tools <id>`, users can describe a change normally; the generated entry skill routes the agent into the governed lifecycle. |
| **Current-worktree authority** | `status`, `validate`, and `next` recompute state from the owning worktree instead of trusting stale summaries or archived records. |
| **Context isolation checks** | Plan audit, implementation, selected S3 review peers, repair, goal verification, and final closeout carry distinct context-origin evidence and ordering checks. |
| **Worktree-bound execution** | Discovery-heavy changes can run in a dedicated `.worktrees/<branch>` checkout; worktree path and branch binding are validated before execution continues. |
| **Actual-edit wave audit** | Dependency-ordered waves can run in parallel, then Slipway audits real changed files, executor handles, dispatch mode, and scope containment after implementation. |
| **Repo-owned audit trail** | `artifacts/changes/`, `.git/slipway/runtime/`, lifecycle events, and archived bundles keep the record inspectable after the session ends. |

## Quick Start

Install Slipway, initialize your repository, and generate the adapter for the AI
tool you actually use:

```bash
brew install --cask signalridge/tap/slipway
# or
go install github.com/signalridge/slipway@latest

slipway init --tools codex
```

Other adapter IDs are `claude`, `cursor`, `gemini`, `opencode`, `all`, and
`none`.

For non-trivial work, create a governed change:

```bash
slipway new "add a dry-run mode to export" --preset standard
slipway status --json
slipway next --json --diagnostics
slipway run --json --diagnostics
```

In an AI-tool session, you usually describe the change in plain language. The
generated entry skill should call the same CLI surfaces and stop when Slipway
returns a skill handoff, blocker, checkpoint, or done-ready state.

<details>
<summary><strong>Command-first lifecycle</strong></summary>

```bash
slipway new "refresh governance docs" --preset standard
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway validate --json
slipway done --json
```

`slipway run --json --diagnostics` is the shortcut driver. It delegates to the
current primary stage command and stops at operator-facing boundaries.

### Execution auto mode

`execution.auto` in `.slipway.yaml` is **off by default**. When opted in (or
overridden per run with `slipway run --auto`), `slipway run` auto-advances
pure-pacing pauses on prior authorization — review batches, non-sensitive skill
handoffs, and **fresh** human-verify checkpoints — without stopping for a fresh
confirmation. `slipway run --no-auto` forces a single run back to manual pacing
(`--no-auto=false` is not an affirmative override and falls through to config).

Config-level `execution.auto` also applies to the stage commands
(`slipway intake` / `slipway plan` / `slipway implement`), which auto-advance
consistently with `run` but expose no per-stage flag; the per-run `--auto` /
`--no-auto` overrides live only on `slipway run`.

Auto mode never relaxes governance. Sensitive/guardrail confirmations, the
intake Approved Summary, decision and human_action checkpoints, stale checkpoints,
and every evidence gate are **never** auto-advanced; they always hard-stop for
explicit operator input and fresh evidence. The upgrade-only preset auto-confirm
only ever raises governance strictness (never lowers it), so it is not one of
these red lines.

</details>

## How It Works

<div align="center">
  <img alt="Slipway governed lifecycle: new, S0 Intake, S1 Plan, S2 Implement, S3 Review, done-ready, done" src="docs/assets/diagrams/lifecycle.svg" width="920">
</div>

| Stage | What Slipway expects |
| --- | --- |
| `S0_INTAKE` | Intent, scope, open questions, risk class, and initial authorization. |
| `S1_PLAN` | Research, requirements, decision, task plan, and plan-audit evidence. |
| `S2_IMPLEMENT` | Dependency-ordered waves, changed files, and task evidence. |
| `S3_REVIEW` | Selected peer reviews, repair evidence, goal verification, assurance, and final closeout. |
| `done` | Terminal archive under `artifacts/changes/archived/<slug>/`. |

`change.yaml` owns current lifecycle authority. Markdown artifacts explain the
work, YAML verification records prove specific stages, and lifecycle events give
an append-only trace of mutations. Read-only surfaces (`status`, `validate`,
`next`) are the first place to look when a session resumes or a change is
confusing. The primary mutation surfaces are `slipway new`, `slipway intake`,
`slipway plan`, `slipway implement`, `slipway review`, `slipway fix`,
`slipway done`, and the `slipway run` shortcut driver.

## Design Philosophy

Slipway follows three project rules:

- **One current authority.** `change.yaml` owns lifecycle state; logs and
  Markdown support it but never replace it.
- **Separated contexts, checked later.** Authoring, audit, review, repair, and
  closeout evidence are recorded as separate participants; the gate checks that
  the independence chain did not collapse.
- **Human-readable, machine-checkable.** People can review the artifacts, while
  the CLI re-derives freshness from structured inputs.
- **Smallest useful control plane.** Host adapters stay thin; governance lives
  in the CLI and repository artifacts.

Read [Design](docs/explanation/design.md) and
[Workflow](docs/explanation/workflow.md) for the shorter explanation, or the
legacy deep dives in [Design Philosophy](docs/design.md) and
[Governed Workflow](docs/workflow.md).

<details>
<summary><strong>Deep enforcement axes</strong></summary>

Behind the gate, every stage owns evidence the engine re-derives instead of
trusting. These are the implementation axes that make a faked "done" fail:

| Axis | Engine behavior |
| --- | --- |
| Attested fresh context | Review, plan audit, repair, and closeout records carry distinct context-origin evidence and ordering checks. |
| Tamper-evident evidence | Freshness is derived from changed files, artifacts, run-summary version, suite-result records, and runtime task evidence, not from a file saying `pass`. |
| Two-sided parallel safety | Planned file-disjoint waves are followed by audits of the actual changed files, executor handles, dispatch mode, and scope contract. |
| Scope containment | `target_files` and disclosed exemptions are checked against the real diff; out-of-lane edits fail closed. |
| Drift-aware recovery | Plan or evidence drift reopens the change in place and `slipway next` names the forward repair command. |
| Local-first audit | Active and archived records stay in the repository, with runtime proof under `.git/slipway/runtime/`. |
| Risk-tiered guardrails | Sensitive domains require domain-aware review, high-risk checks, and explicit evidence before ship approval. |

</details>

## How Slipway Compares

Most AI workflow systems are good at structuring work. Slipway's narrower bet
is enforcement: the final lifecycle authority lives in a deterministic CLI that
recomputes state from repo evidence.

<details>
<summary><strong>Adjacent tools and trade-offs</strong></summary>

| Tool | How you drive it | Done enforcement |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) | `/speckit.*` slash commands | Advisory checklists and phase prompts. |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | `/opsx:*` slash commands | Flexible spec workflow; verification is optional. |
| [spec-kitty](https://github.com/Priivacy-ai/spec-kitty) | `/spec-kitty.*` commands plus autopilot | Some status gates, but review remains advisory. |
| [GSD Core](https://github.com/open-gsd/gsd-core) | Runtime surfaces plus `/gsd-*` phase commands | Strong phase loop and fresh-context orchestration; final proof is still workflow-artifact mediated. |
| [superpowers](https://github.com/obra/superpowers) | Auto-firing skills | Strong agent discipline, but rules live in model context. |
| **Slipway** | Plain language through thin adapters, or direct CLI | Compiled, fail-closed gates backed by repo evidence. |

Slipway trades breadth for authority. It supports fewer first-class adapters
than broad prompt frameworks, but each generated surface routes back to the same
CLI. It is heavier than a lightweight prompt pack on throwaway edits, but much
stricter when stale evidence, scope drift, or risky domains would otherwise be
easy to miss.

</details>

## AI Tool Adapters

Generate host-tool surfaces with `slipway init --tools <id>` and refresh managed
files with `slipway init --refresh`. Generated files are ownership-tracked so
refresh can replace Slipway-owned files without deleting adjacent user-owned
customizations.

<details>
<summary><strong>Generated surfaces per tool</strong></summary>

| Tool | Generated surfaces |
| --- | --- |
| Claude | `.claude/skills/slipway-*/SKILL.md`, `.claude/commands/slipway/*.md`, `.claude/settings.json` hook entries |
| Codex | `.codex/skills/slipway-*/SKILL.md` entry, command, and governance skills |
| Cursor | `.cursor/skills/slipway-*/SKILL.md`, `.cursor/commands/*.md`, session-start hook launchers |
| Gemini | `.gemini/skills/slipway-*/SKILL.md`, `.gemini/commands/slipway/*.toml`, `.gemini/settings.json` hook entries |
| OpenCode | `.opencode/skills/slipway-*/SKILL.md`, `.opencode/commands/slipway-*.md`, session-start hook launchers |

Exported generated skill rows are pinned by public skill directory:
`slipway/SKILL.md`, `slipway-ci-triage/SKILL.md`,
`slipway-code-quality-review/SKILL.md`, `slipway-codebase-mapping/SKILL.md`,
`slipway-coding-discipline/SKILL.md`, `slipway-context-assembly/SKILL.md`,
`slipway-coverage-analysis/SKILL.md`, `slipway-final-closeout/SKILL.md`,
`slipway-git-recovery/SKILL.md`, `slipway-goal-verification/SKILL.md`,
`slipway-incident-response/SKILL.md`, `slipway-independent-review/SKILL.md`,
`slipway-intake-clarification/SKILL.md`, `slipway-plan-audit/SKILL.md`,
`slipway-research-orchestration/SKILL.md`,
`slipway-root-cause-tracing/SKILL.md`, `slipway-security-review/SKILL.md`,
`slipway-spec-compliance-review/SKILL.md`, `slipway-spec-trace/SKILL.md`,
`slipway-tdd-governance/SKILL.md`, `slipway-test-design/SKILL.md`,
`slipway-wave-orchestration/SKILL.md`, and
`slipway-worktree-preflight/SKILL.md`.

Codex does not have a session-hook surface; its entry skill pulls governed state
with `slipway status --json` when needed. The other adapters can inject compact
session-start state through hook-capable host surfaces.

See [AI Tool Adapters](docs/reference/ai-tools.md) and the generated
[Surface Manifest](docs/SURFACE-MANIFEST.json) for the exact command and skill
inventory.

</details>

## Runtime Files

<details>
<summary><strong>Repository state written by Slipway</strong></summary>

| Path | Role |
| --- | --- |
| `artifacts/changes/<slug>/change.yaml` | Current lifecycle and routing authority. |
| `artifacts/changes/<slug>/*.md` | Intent, research, requirements, decisions, tasks, and assurance records. |
| `artifacts/changes/<slug>/verification/` | Skill verification records consumed by the ship gate. |
| `artifacts/changes/<slug>/events/lifecycle.jsonl` | Append-only lifecycle mutation trace. |
| `.git/slipway/runtime/changes/<slug>/evidence/` | Git-local task evidence and runtime proof. |
| `.git/slipway/runtime/changes/<slug>/handoff.md` | Optional per-change advisory continuation notes for fresh AI sessions; never lifecycle authority, evidence, freshness, or a gate. |
| `.git/slipway/locks/change-create.lock`, `.git/slipway/locks/repair.lock` | Workspace/scope-level coordination locks for change creation and repair. They are intentionally not per-change because they protect operations that begin before or outside a stable change slug. |
| `artifacts/changes/archived/<slug>/` | Terminal record after `slipway done`. |
| `artifacts/codebase/` | Repo-scoped context map used for brownfield planning and review. |
| `.worktrees/<branch>/` | Dedicated governed worktrees when a change is isolated. |

AI-tool sessions read generated host surfaces from the project root. A governed
worktree holds the code changes, but root host adapter files are not copied into
each worktree.
Legacy repo-level handoff files such as `.git/slipway/runtime/handoff.md` are
reported as local runtime hygiene findings and are not used as current change
authority.

</details>

## Documentation

Documentation is organized by task:

- [Start Here](docs/start-here.md): shortest path from install to one governed
  change.
- [Real-World Scenarios](docs/real-world-scenarios.md): adoption patterns.
- [First Governed Change](docs/tutorials/first-governed-change.md): guided
  tutorial.
- [Onboarding Existing Codebase](docs/tutorials/onboarding-existing-codebase.md):
  brownfield setup.
- [Install and Refresh Adapters](docs/how-to/install-and-refresh-adapters.md):
  operational adapter commands.
- [Recover and Troubleshoot](docs/how-to/recover-and-troubleshoot.md):
  fail-closed recovery.
- [Commands](docs/reference/commands.md): command and JSON surface reference.
- [AI Tool Adapters](docs/reference/ai-tools.md): generated host files and
  invocation style.
- [Design](docs/explanation/design.md) and
  [Workflow](docs/explanation/workflow.md): concepts and rationale.

## Verification

Useful local checks while developing:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go run ./internal/testlint/cmd/testlint ./...
golangci-lint run --timeout=5m
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
mkdocs build --strict
```

CI runs Markdown/YAML/action linting, Go linting, Slipway testlint, Go tests
across platforms, race tests, kernel coverage, build checks, security scans, Nix
checks, and the docs workflow.

## Repository Status

![Repobeats analytics image](https://repobeats.axiom.co/api/embed/20e468225cab8a858d9bc969314a0e9c3d12bddb.svg "Repobeats analytics image")
