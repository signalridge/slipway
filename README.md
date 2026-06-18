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

[Documentation](https://signalridge.github.io/slipway/) | [Quick Start](#quick-start) | [Installation](docs/installation.md) | [Release Notes](CHANGELOG.md)

</div>

# Slipway

**Governance that keeps your AI coding agent honest: "done" means proven, not promised. You drive it in plain language; Slipway turns that work into a local, reviewable lifecycle with hard gates.**

## What is Slipway?

Slipway is a local, Git-native governance CLI for AI-assisted software delivery.
It wraps coding-agent work in a durable change record, checks that the plan,
implementation, review, and verification still match, and refuses to archive a
change until the evidence is fresh. Your AI tool writes the code; Slipway decides
whether the work is actually done.

It is built for teams that like the structured phase loop of
[GSD Core](https://github.com/open-gsd/gsd-core) and the repo-persisted specs,
tasks, and memory model of [Trellis](https://github.com/mindfold-ai/Trellis),
but want the final authority to live in a compiled CLI rather than in prompts or
Markdown the model can quietly skip.

## Why Slipway?

| Capability | What it changes |
| --- | --- |
| **Fail-closed done gate** | `slipway done` rechecks fresh review, verification, scope, and guardrail evidence before archive. Missing or stale proof blocks the change. |
| **Plain-language agent entry** | After `slipway init`, generated skills route normal requests through the lifecycle across Claude Code, Codex, Cursor, Gemini, and OpenCode. |
| **Repo-owned memory and audit** | Plans, decisions, evidence, lifecycle events, and archived bundles stay in the repository, so a later human or AI session can inspect what happened. |
| **Fresh-context review discipline** | Selected S3 peer reviews, goal verification, and final closeout are recorded separately, and the engine checks that the independence chain still holds. |
| **Parallel work with after-the-fact safety** | Implementation waves can run in file-disjoint parallel work, then Slipway audits the actual changed files for overlap, scope drift, and stale evidence. |
| **Compiled governance, lower prompt load** | The rules run in Go and emit JSON handoffs. The model sees compact instructions instead of re-reading a full governance playbook every turn. |

## See it in action

You talk to your AI tool the way you already do. The agent handles the governance, and you never type a Slipway command:

```text
You:  Add a --dry-run flag to the export command.

(The repo is governed. The entry skill picks that up and routes the
 request on its own — no command from you, nothing beyond the one-time init.)

Agent (driving the lifecycle):
  → slipway new        creates the governed change
  → slipway intake     captures intent, scope, and guardrail class
  → slipway plan       writes requirements, decision, tasks, and plan-audit evidence
  → slipway implement  implements the flag and runs your test + build commands
  → slipway review     runs selected S3 peers and review-finding repairs
  → done-ready ✓       every step backed by evidence committed beside the code

You:  Looks good, finalize it.

Agent:
  → slipway done     archives the terminal state

You never typed a slash command. Slipway handed the agent the lifecycle;
the agent drove it.
```

And if the agent had tried to call it `done` before the tests ran and the review evidence existed? `slipway done` refuses: the gate lives in the CLI, not in a prompt the agent can decide to skip.

## Where Slipway goes deep

Behind the gate, every stage owns evidence the engine **re-derives instead of trusting**. These seven axes are what make a faked "done" fall over — and together, no adjacent tool enforces them in code. Each is stated at its honest enforcement tier; the [Design Philosophy](docs/design.md#advantage-axes) carries the full mechanism and the residual caveats.

| Axis | What the engine does | Why it's hard to fake (and what no peer matches) |
| --- | --- | --- |
| **Attested fresh context** | Each stage records the distinct `context_origin` handle it ran under; a per-seam lattice fails closed if the reviewer, plan auditor, or fix collapses into the implementer's context | gsd and superpowers *spawn* fresh subagents; Slipway also *checks the independence held* (audit/structural tier, not cryptographic proof) |
| **Tamper-evident evidence** | Re-derives freshness from the real inputs — code diff, planning artifacts, run-summary version, shared suite-result — never the verification file's own claims | Peers store state as Markdown/YAML the model can quietly edit; Slipway names the stale input (`required_skill_stale:…`), reopens the change, and stays the sole verdict stamper |
| **Two-sided parallel safety** | Deterministic file-disjoint waves run concurrently; four safety nets then audit the *actual* changed files (scope escape, wave overlap, dispatch mode, executor handles) | Peers that parallelize check the *plan* before dispatch; Slipway also audits what the agents *actually edited* afterward |
| **Scope containment** | Declared `target_files` is a contract checked with the planner's own path predicate; out-of-lane edits fail closed (`scope_contract_drift`) | The codebase map under `artifacts/codebase/` is the one *disclosed* exemption (`exempt_context_files`), not a silent gap |
| **Drift-aware forward recovery** | Plan or evidence drift reopens the change *in place*, forward-only; `slipway next` projects the next repair as a concrete command with the blocker named | No backward cascade can hide the gap, and recovery never depends on the agent knowing a private sequence |
| **Local-first, git-native audit** | `change.yaml` is the single authority; an append-only, readback-verified `events/lifecycle.jsonl` traces every mutation beside the code under `artifacts/changes/` | Nothing leaves the repository — the audit trail is sovereign by default and re-inspectable by any later human or AI session |
| **Risk-tiered guardrails** | Sensitive domains (auth, credentials/PII, financial, schema migration, irreversible ops, external-API) require per-domain high-risk checks and gate sensitive evidence at S2 and S3 | No bypass, force-close, or private-attestation path — light on throwaway changes, unforgiving on dangerous ones |

Instead of a single end-of-run checkbox, you get an auditable trail of stage-owned evidence the next session — human or AI — can re-inspect and trust.

## How Slipway compares

Spec, workflow, and skill toolkits for AI coding are all good at *structuring* work. GSD Core is especially strong at fresh-context phase execution and workstream orchestration; Trellis is especially clear about repo-persisted specs, tasks, and project memory. Slipway's narrower bet is **where the rules live and whether the model can ignore them**: most adjacent systems encode the process as prompts, skills, or Markdown the agent is *asked* to follow, while Slipway compiles the process into a deterministic CLI backed by repo evidence, so the gates fail closed.

| Tool | How you drive it | Enforcement of "done" |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) (GitHub) | `/speckit.*` slash command per phase | Advisory: an incomplete checklist passes on a "yes" |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | `/opsx:*` slash commands | Advisory by design: "fluid, not rigid," and verify is optional |
| [spec-kitty](https://github.com/Priivacy-ai/spec-kitty) | `/spec-kitty.*` plus a `spec-kitty next` autopilot loop | Partial: `merge` gates on status, but review is an advisory "nudge" |
| [GSD Core](https://github.com/open-gsd/gsd-core) | Installer-generated runtime surfaces plus `/gsd-*` phase commands | Strong phase loop and fresh-context orchestration; final enforcement still rests on agent-followed workflow artifacts |
| [Trellis](https://github.com/mindfold-ai/Trellis) | `trellis init` plus generated specs, tasks, and platform surfaces | Strong repo-persisted specs/tasks/memory and broad platform reach; checks are orchestrated by agents, not a compiled lifecycle authority |
| [superpowers](https://github.com/obra/superpowers) | Skills auto-fire from a session bootstrap (no commands) | Self-discipline: an "Iron Law" the model is *asked* to obey |
| **Slipway** | **Plain language; an entry skill auto-fires (no commands)** | **Compiled, fail-closed**: gates live in the CLI and repo evidence |

The pattern is consistent: those tools enforce process by asking the model to comply, while Slipway enforces it in code the model runs but can't rewrite. GSD Core and Trellis both informed the shape of Slipway's public README because they explain workflow and onboarding clearly; Slipway differs at the enforcement boundary. The capability set overlaps — Slipway also runs dependency-ordered waves, dedicated worktrees, and TDD governance — so the divide is not feature count but enforcement: across the [axes where Slipway goes deep](#where-slipway-goes-deep), the engine re-derives evidence instead of trusting it. Where the peers genuinely lead is **reach and ecosystem**: far more supported agents, more mileage, and models Slipway lacks, like OpenSpec's delta-specs, GSD Core's broad runtime matrix, Trellis' multi-platform harness, or Spec Kit's large integration catalog.

### What Slipway deliberately trades off

Several of the differences above are intentional, not gaps. Slipway optimizes for *provable* outcomes on changes that matter, and accepts the costs that come with that:

| Dimension | Most spec / skill toolkits | Slipway's deliberate choice | What the trade-off buys |
| --- | --- | --- | --- |
| Agent reach | Broad generated surfaces across many agents, as in GSD Core and Trellis | 5 first-class adapters (growing) | A tested contract per tool, not a lowest-common-denominator prompt |
| Install & runtime | `npx` / `uvx`, no binary | A single versioned Go binary | One deterministic engine; no per-session prompt or version drift |
| State integrity | Repo or spec files the model maintains | Engine-owned state with freshness digests | Stale or hand-edited evidence is detected, not trusted |
| Flexibility | Edit any artifact anytime ("fluid") | A staged lifecycle that reopens on drift | Plan and code can't silently diverge, at the cost of less freeform editing |
| Speed vs. assurance | Fast by default; gates optional | Gates are mandatory and freshness-checked | Slower on throwaway edits; safe on the changes that matter |
| Failure mode | Skipping a step degrades silently | Missing or stale evidence fails closed | You learn the work isn't done *before* you ship, not after |

One thing is **not** a deliberate trade-off, just where Slipway is today: mileage. It is younger and less battle-tested than Spec Kit or superpowers, and the five-tool list is still growing. The bet is that fail-closed depth is the harder thing to retrofit later, and breadth and mileage come with time.

## Quick start

**1. Install** (pick one; full matrix in [Installation](docs/installation.md)):

```bash
brew install --cask signalridge/tap/slipway   # macOS
scoop install slipway                          # Windows (after adding the bucket)
go install github.com/signalridge/slipway@latest   # any platform with Go
```

Linux users also have `.deb` / `.rpm` / `.apk`, the `ghcr.io/signalridge/slipway` container image, AUR `slipway-bin`, and Nix. See [Installation](docs/installation.md) for every path and checksum verification.

**2. Initialize your repo and generate the adapter for your AI tool:**

```bash
slipway init --tools claude        # or: codex, cursor, gemini, opencode
slipway init --tools all           # generate every adapter
```

For each `--tools` target, this writes `.slipway.yaml`, a managed `.gitignore` block, and the governed skills that teach the AI tool how to enter the lifecycle. Tools that support session hooks also get one. (Omit `--tools` to set up runtime only.)

**3. Just talk to your AI tool.** Start a new session and describe what you want: "add a retry to the upload client," "fix the off-by-one in pagination," "refactor the auth middleware." The entry skill routes the request, and on tools with session hooks governed state is surfaced automatically, so Slipway walks it from intake to done-ready. No command to remember.

<details>
<summary><strong>Prefer explicit control, or scripting?</strong> The same lifecycle is available command-first.</summary>

<br/>

```bash
slipway new "refresh governance docs" --preset standard
slipway next --json        # read-only handoff: what's next, what's blocking
slipway plan --json        # explicit lifecycle stage command
slipway implement --json
slipway review --json
slipway run --json         # shortcut: delegates to the current stage command
slipway status --json
slipway done --json
```

</details>

## How it works

<div align="center">
  <img alt="Slipway governed lifecycle: new, S0 Intake, S1 Plan, S2 Implement, S3 Review, done-ready, done" src="docs/assets/diagrams/lifecycle.svg" width="920">
</div>

A governed change moves through S0 Intake, S1 Plan, S2 Implement, S3 Review, done-ready, then done. S3 owns the selected peer evidence; goal-verification is one unordered peer, and final closeout is strictly last before done-ready. `change.yaml` holds the current authority; mutating lifecycle events append to `events/lifecycle.jsonl`; evidence accumulates beside the code. `slipway next`, `slipway status`, and `slipway validate` are read-only inspection surfaces. The primary mutation surfaces are `slipway new`, `slipway intake`, `slipway plan`, `slipway implement`, `slipway review`, `slipway fix`, and `slipway done`; `slipway run` is an auto-driver shortcut that delegates to the current primary stage.

The plain-language experience rests on generated surfaces per AI tool:

- **A thin entry skill** (every supported tool) whose description triggers on natural-language change requests and routes into the right CLI command. It never re-implements governance; it hands off to the CLI, and the CLI decides state, readiness, recovery, and the next governed step.
- **A session-start hook** (on tools that support session hooks: Claude, Cursor, Gemini, OpenCode, but not Codex) that asks the CLI for current governed state every session and injects a compact handoff, so the agent knows whether a change is active before you prompt it. Where a tool has no hook, the entry skill has the agent pull the same state with `slipway status --json` on first contact, so enforcement is identical and the state is just pulled instead of pushed.

All these surfaces emit `--json`, so agents and scripts get structured handoffs, and `slipway health`, `repair`, `stats`, and `codebase-map` inspect or recover local state. When evidence drifts, `slipway next` shows the forward repair path and `slipway run` delegates to the current primary stage command. The deeper JSON contracts live in the [Operator Guide](docs/operator-guide.md#diagnostic-json) and [Command Reference](docs/commands.md).

## Design philosophy

- **One current authority.** A single `change.yaml` owns lifecycle state; logs and Markdown support it but never replace it.
- **Human-readable, machine-checkable.** Markdown stays readable to people, while stable sections and YAML give the runtime something deterministic to inspect.
- **Smallest useful control plane.** Slipway stays narrower than adjacent spec, workflow, and agent frameworks by keeping authority in the CLI and repository artifacts.

See [Design Philosophy](docs/design.md) for the longer architecture explanation.

## AI tool adapters

Generate host-tool surfaces with `slipway init --tools <id>` (`claude`, `codex`, `cursor`, `gemini`, `opencode`, `all`, or `none`). Use `--refresh` to regenerate managed files deterministically.

<details>
<summary>Generated surfaces per tool</summary>

<br/>

| Tool | Generated surfaces |
| --- | --- |
| Claude | `.claude/skills/slipway-*/SKILL.md`, `.claude/commands/slipway/*.md`, `.claude/settings.json` (inline `slipway hook ...` entries, no launcher file) |
| Codex | `.codex/skills/slipway-*/SKILL.md` (entry, per-command, and governance skills) |
| Cursor | `.cursor/skills/slipway-*/SKILL.md`, `.cursor/commands/*.md`, `.cursor/hooks/slipway-session-start` (plus `.ps1` / `.cmd`) |
| Gemini | `.gemini/skills/slipway-*/SKILL.md`, `.gemini/commands/slipway/*.toml`, `.gemini/settings.json` (inline `slipway hook ...` entries, no launcher file) |
| OpenCode | `.opencode/skills/slipway-*/SKILL.md`, `.opencode/commands/slipway-*.md`, `.opencode/hooks/slipway-session-start` (plus `.ps1` / `.cmd`) |

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

Every tool gets the entry skill. Codex enters the lifecycle through its skills (the entry skill, one skill per command, and governance host skills); the other four also get an auto-injecting session-start hook (Codex has no session-hook surface to attach to, so its agent pulls governed state via `slipway status --json` instead).

</details>

Want an agent to install and initialize Slipway for you? Paste the [AI Tool Installation Prompt](docs/installation.md#ai-tool-installation-prompt) into Claude Code, Codex, OpenCode, or another tool, but read it first and supervise the run. See [AI Tool Adapters](docs/ai-tools.md) for invocation spellings and safety rules.

## Runtime files

- `artifacts/changes/`: governed change bundles. Each holds `change.yaml` plus the Markdown artifacts (intent, research, requirements, decision, tasks, assurance) and verification records for the change. Active records are runtime authority; archived records stay Git-safe in the owning workspace.
- `artifacts/changes/**/evidence/`, `events/`, `verification/`: raw local proof directories, ignored by Slipway-managed `.gitignore` rules. Keep them for local audit and validation.
- `artifacts/codebase/`: advisory repo-scoped codebase maps from `slipway codebase-map`, git-tracked by default so brownfield context is shared rather than hidden.
- `.worktrees/`: dedicated governed worktrees, local-only by default.

The governed AI-tool session runs from the **project root** and reads skills and
hooks from the root host surfaces (`.claude`, `.gemini`, etc.). The per-change
git worktree under `.worktrees/<branch>` holds that change's edited files, but
the session's working directory stays at the root — it does not run with its cwd
inside the worktree, so host-adapter surfaces are not provisioned into worktrees.

See the [Operator Guide](docs/operator-guide.md) for state authority, freshness state, and recovery details.

## Documentation

- [Installation](docs/installation.md): platform packages, source builds, repo initialization, and the AI-tool install prompt.
- [Design Philosophy](docs/design.md): governing principles, authority boundaries, and adjacent-system tradeoffs.
- [Governed Workflow](docs/workflow.md): lifecycle states, read-only surfaces, mutating commands, and Open Questions semantics.
- [Command Reference](docs/commands.md): core, situational, and diagnostics commands.
- [AI Tool Adapters](docs/ai-tools.md): generated paths and host invocation styles.
- [Surface Manifest](docs/SURFACE-MANIFEST.json): generated inventory of adapter, command, skill, JSON, and documentation surfaces.
- [Operator Guide](docs/operator-guide.md): worktrees, state authority, health, repair, verification, and closeout.
- [Contributing](docs/contributing.md): repo layout, docs build, adapter contracts, and governance tests.

## Verification

Use focused package tests while developing, then run the full local proof before closeout:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
mkdocs build --strict
```

CI also runs Markdown/YAML/action linting, Go tests across platforms, race tests, build checks, security scans, release checks, Nix checks, and the docs workflow in `.github/workflows/docs.yml`.

## Repository status

![Repobeats analytics image](https://repobeats.axiom.co/api/embed/20e468225cab8a858d9bc969314a0e9c3d12bddb.svg "Repobeats analytics image")
