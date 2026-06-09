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

[Documentation](https://signalridge.github.io/slipway/) | [Installation](docs/installation.md) | [Release Notes](CHANGELOG.md)

</div>

# Slipway

**Governance that keeps your AI coding agent honest — "done" means proven, not promised. Driven in plain language, no slash commands to memorize.**

AI coding agents are fast, but they cut corners — they skip the tests, drift from the plan, and announce "done" on work that was never verified. Slipway is a local, Git-native governance layer that makes that hard to fake: it turns the agent's work into a durable, inspectable change record and holds the lifecycle authority itself, in the repository — not in a hosted service. Your AI tool does the work; Slipway decides when it's actually done. Works with **Claude Code, Codex, Cursor, Gemini, and OpenCode**.

- **The model can't fake "done."** Completion is gated on fresh review *and* verification evidence — checks compiled into the CLI, not advisory prompts the agent can rationalize past. If the evidence goes stale or the work drifts from the plan, Slipway reopens the change instead of waving it through.
- **No commands to learn.** After a one-time `slipway init`, your AI tool picks up the governed lifecycle from a session hook and routes ordinary plain-language requests through it. You describe the change; the agent drives the process.
- **Lighter on tokens.** The governance logic runs as compiled Go in the CLI — not as phase prompts your model re-reads and re-reasons every turn. One thin entry skill stays resident; stage skills load only when the CLI asks for them.

## See it in action

You talk to your AI tool the way you already do. The agent does the governance — you never type a Slipway command:

```text
You:  Add a --dry-run flag to the export command.

(Slipway's session hook has already told the agent this repo is governed
 and that there's no active change — no action needed from you.)

Agent (routing on its own):
  → slipway new      captures intent, scope, and guardrail class
  → slipway next     intake → planning; writes requirements / decision / tasks
  → implements the flag, runs your test + build commands
  → slipway run      spec review, quality review, goal verification
  → done-ready ✓     every step backed by evidence committed beside the code

You:  Looks good, finalize it.

Agent:
  → slipway done     archives the terminal state

You never typed a slash command. Slipway handed the agent the lifecycle;
the agent drove it.
```

And if the agent had tried to call it `done` before the tests ran and the review evidence existed? `slipway done` refuses — the gate lives in the CLI, not in a prompt the agent can decide to skip.

## How Slipway compares

Spec, workflow, and skill toolkits for AI coding are all good at *structuring* work. The axis that sets Slipway apart is **where the rules live and whether the model can ignore them**: almost all of them encode the process as prompts or Markdown the agent is *asked* to follow, so the gates stay advisory. Slipway compiles the process into a deterministic CLI backed by repo evidence, so the gates fail closed.

| Tool | How you drive it | Enforcement of "done" |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) (GitHub) | `/speckit.*` slash command per phase | Advisory — an incomplete checklist passes on a "yes" |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | `/opsx:*` slash commands | Advisory by design — "fluid, not rigid"; verify is optional |
| [spec-kitty](https://github.com/Priivacy-ai/spec-kitty) | `/spec-kitty.*` plus a `spec-kitty next` autopilot loop | Partial — `merge` gates on status; review is an advisory "nudge" |
| [gsd](https://github.com/gsd-build/get-shit-done) | 80+ `/gsd-*` slash commands | Advisory — hooks must not block; `--skip-*` / `--auto` bypass |
| [superpowers](https://github.com/obra/superpowers) | Skills auto-fire from a session bootstrap (no commands) | Self-discipline — an "Iron Law" the model is *asked* to obey |
| **Slipway** | **Plain language via a session hook** | **Compiled & fail-closed** — gates live in the CLI + repo evidence |

The pattern is consistent: those tools enforce process by asking the model to comply; Slipway enforces it in code the model runs but can't rewrite. (superpowers is the closest on *experience* — it also auto-triggers from a session hook — but its rules live in the model's context, not in a binary.) On capability the lines blur — Slipway also runs dependency-ordered waves, dedicated worktrees, and TDD governance — so the real divide is enforcement, not feature count. Where the peers genuinely lead is **reach and ecosystem**: far more supported agents, more mileage, and models Slipway lacks, like OpenSpec's delta-specs or Spec Kit's large integration catalog.

### What Slipway deliberately trades off

Several of the differences above are intentional, not gaps. Slipway optimizes for *provable* outcomes on changes that matter, and accepts the costs that come with that:

| Dimension | Most spec / skill toolkits | Slipway's deliberate choice | What the trade-off buys |
| --- | --- | --- | --- |
| Agent reach | 15–30+ agents via generated prompt files | 5 first-class adapters (growing) | A tested contract per tool, not a lowest-common-denominator prompt |
| Install & runtime | `npx` / `uvx`, no binary | A single versioned Go binary | One deterministic engine; no per-session prompt or version drift |
| State integrity | Repo or spec files the model maintains | Engine-owned state with freshness digests | Stale or hand-edited evidence is detected, not trusted |
| Flexibility | Edit any artifact anytime ("fluid") | A staged lifecycle that reopens on drift | Plan and code can't silently diverge — at the cost of less freeform editing |
| Speed vs. assurance | Fast by default; gates optional | Gates are mandatory and freshness-checked | Slower on throwaway edits; safe on the changes that matter |
| Failure mode | Skipping a step degrades silently | Missing or stale evidence fails closed | You learn the work isn't done *before* you ship, not after |

What is **not** a deliberate trade-off — just where Slipway is today — is mileage: it's younger and less battle-tested than Spec Kit or superpowers, and the five-tool list is still growing. The bet is that fail-closed depth is the harder thing to retrofit later; breadth and mileage come with time.

## Depth, not just a gate

A single "did you run the tests?" check is easy to fake. The point of Slipway is the depth behind the gate: a chain of stage-owned checkpoints that each demand their own evidence, so quality is enforced all the way through — not rubber-stamped at the end.

- **Intake** fixes intent, scope, open questions, and a **guardrail class**. Sensitive domains — auth, credentials/PII, financial, schema migration, irreversible ops, external-API contracts — fail closed harder and get no bypass, force-close, or private attestation path.
- **Planning** binds a `requirements.md` / `decision.md` / `tasks.md` bundle that execution is held to, so the agent can't quietly re-scope mid-flight.
- **Review** is *two independent passes* — spec-compliance, then code-quality — read with fresh context, not by the agent that wrote the code.
- **Goal verification** re-checks the acceptance criteria against fresh evidence before anything may claim done.
- **Drift and freshness** are enforced, not trusted: edit the plan or let evidence go stale and Slipway reopens the *earliest* affected stage and re-walks it — a change can't limp to done on half-stale proof.

That chain is what command-driven spec toolkits and in-context skill packs don't carry: not a single end-of-run checkbox, but an auditable trail of stage-owned evidence the next session — human or AI — can re-inspect and trust.

## Quick start

**1. Install** (pick one — full matrix in [Installation](docs/installation.md)):

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

This writes `.slipway.yaml`, a managed `.gitignore` block, and — for each `--tools` target — a session hook plus governed skills that teach the AI tool how to enter the lifecycle. (Omit `--tools` to set up runtime only.)

**3. Just talk to your AI tool.** Start a new session and describe what you want — "add a retry to the upload client", "fix the off-by-one in pagination", "refactor the auth middleware". The session hook surfaces governed state, the entry skill routes the request, and Slipway walks it from intake to done-ready. No command to remember.

<details>
<summary><strong>Prefer explicit control, or scripting?</strong> The same lifecycle is available command-first.</summary>

<br/>

```bash
slipway new "refresh governance docs" --preset standard
slipway next --json        # read-only handoff: what's next, what's blocking
slipway run --json         # advance until a skill, blocker, or done-ready stop
slipway status --json
slipway done --json
```

</details>

## How it works

<div align="center">
  <img alt="Slipway governed lifecycle: new, S0 Intake, S1 Plan, S2 Execute, S3 Review, S4 Verify, done" src="docs/assets/diagrams/lifecycle.svg" width="920">
</div>

A governed change moves through intake → planning → execution → review → verification → closeout. `change.yaml` holds the current authority; mutating lifecycle events append to `events/lifecycle.jsonl`; evidence accumulates beside the code. `slipway next`, `slipway status`, and `slipway validate` are read-only inspection surfaces; `slipway new`, `slipway run`, `slipway done`, and a few others are the explicit mutation surfaces.

The plain-language experience rests on two generated pieces per AI tool:

- **A session-start hook** that, every session, asks the CLI for current governed state and injects a compact handoff — so the agent always knows whether a change is active and what step owns the next action, without you prompting it.
- **A thin entry skill** whose description triggers on natural-language change requests and routes into the right CLI command. It never re-implements governance; it hands off to the CLI and the CLI decides state, readiness, recovery, and the next governed step.

All these surfaces emit `--json`, so agents and scripts get structured handoffs, and `slipway health`, `repair`, `stats`, and `codebase-map` inspect or recover local state. When evidence drifts, `slipway next` shows the recovery path and `slipway run` re-walks it — the deeper JSON contracts live in the [Operator Guide](docs/operator-guide.md#diagnostic-json) and [Command Reference](docs/commands.md).

## Design philosophy

- **One current authority** — a single `change.yaml` owns lifecycle state; logs and Markdown support it but never replace it.
- **Human-readable, machine-checkable** — Markdown stays readable to people while stable sections and YAML give the runtime something deterministic to inspect.
- **Smallest useful control plane** — Slipway stays narrower than adjacent spec, workflow, and agent frameworks by keeping authority in the CLI and repository artifacts.

See [Design Philosophy](docs/design.md) for the longer architecture explanation.

## AI tool adapters

Generate host-tool surfaces with `slipway init --tools <id>` (`claude`, `codex`, `cursor`, `gemini`, `opencode`, `all`, or `none`). Use `--refresh` to regenerate managed files deterministically.

<details>
<summary>Generated surfaces per tool</summary>

<br/>

| Tool | Generated surfaces |
| --- | --- |
| Claude | `.claude/skills/slipway-*/SKILL.md`, `.claude/commands/slipway/*.md`, `.claude/hooks/slipway-session-start.sh`, `.claude/settings.json` |
| Codex | `.codex/skills/slipway-*/SKILL.md`, `$CODEX_HOME/prompts/slipway-*.md` |
| Cursor | `.cursor/skills/slipway-*/SKILL.md`, `.cursor/commands/*.md`, `.cursor/hooks/slipway-session-start.sh` |
| Gemini | `.gemini/skills/slipway-*/SKILL.md`, `.gemini/commands/slipway/*.toml`, `.gemini/hooks/slipway-session-start.sh`, `.gemini/settings.json` |
| OpenCode | `.opencode/skills/slipway-*/SKILL.md`, `.opencode/commands/slipway-*.md`, `.opencode/hooks/slipway-session-start.sh` |

</details>

Want an agent to install and initialize Slipway for you? Paste the [AI Tool Installation Prompt](docs/installation.md#ai-tool-installation-prompt) into Claude Code, Codex, OpenCode, or another tool — read it first and supervise the run. See [AI Tool Adapters](docs/ai-tools.md) for invocation spellings and safety rules.

## Runtime files

- `artifacts/changes/` — governed change bundles: `change.yaml` plus the Markdown artifacts (intent, research, requirements, decision, tasks, assurance) and verification records for each change. Active records are runtime authority; archived records stay Git-safe in the owning workspace.
- `artifacts/changes/**/evidence/`, `events/`, `verification/` — raw local proof directories, ignored by Slipway-managed `.gitignore` rules; keep them for local audit and validation.
- `artifacts/codebase/` — advisory repo-scoped codebase maps from `slipway codebase-map`, git-tracked by default so brownfield context is shared rather than hidden.
- `.worktrees/` — dedicated governed worktrees, local-only by default.

See the [Operator Guide](docs/operator-guide.md) for state authority, freshness state, and recovery details.

## Documentation

- [Installation](docs/installation.md) — platform packages, source builds, repo initialization, and the AI-tool install prompt.
- [Design Philosophy](docs/design.md) — governing principles, authority boundaries, and adjacent-system tradeoffs.
- [Governed Workflow](docs/workflow.md) — lifecycle states, read-only surfaces, mutating commands, and Open Questions semantics.
- [Command Reference](docs/commands.md) — core, situational, and diagnostics commands.
- [AI Tool Adapters](docs/ai-tools.md) — generated paths and host invocation styles.
- [Operator Guide](docs/operator-guide.md) — worktrees, state authority, health, repair, verification, and closeout.
- [Contributing](docs/contributing.md) — repo layout, docs build, adapter contracts, and governance tests.

## Verification

Use focused package tests while developing, then run the full local proof before closeout:

```bash
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
mkdocs build --strict
```

CI also runs Markdown/YAML/action linting, Go tests across platforms, race tests, build checks, security scans, release checks, Nix checks, and the docs workflow in `.github/workflows/docs.yml`.

## Repository status

![Repobeats analytics image](https://repobeats.axiom.co/api/embed/20e468225cab8a858d9bc969314a0e9c3d12bddb.svg "Repobeats analytics image")
