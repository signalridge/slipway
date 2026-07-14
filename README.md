<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=astro&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>&nbsp;
  <a href="https://pkg.go.dev/github.com/signalridge/slipway"><img alt="Go Reference" src="https://img.shields.io/badge/Go-Reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
</p>

<p>
  <a href="docs/en/installation.md"><img alt="Homebrew Cask" src="https://img.shields.io/badge/Homebrew_Cask-FBB040?style=flat-square&logo=homebrew&logoColor=black"></a>
  <a href="docs/en/installation.md"><img alt="Scoop" src="https://img.shields.io/badge/Scoop-00BFFF?style=flat-square&logo=windows&logoColor=white"></a>
  <a href="docs/en/installation.md"><img alt="AUR" src="https://img.shields.io/badge/AUR-1793D1?style=flat-square&logo=archlinux&logoColor=white"></a>
  <a href="docs/en/installation.md"><img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?style=flat-square&logo=nixos&logoColor=white"></a>
  <a href="docs/en/installation.md"><img alt="Docker" src="https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/en/installation.md"><img alt="deb" src="https://img.shields.io/badge/deb-A81D33?style=flat-square&logo=debian&logoColor=white"></a>
  <a href="docs/en/installation.md"><img alt="rpm" src="https://img.shields.io/badge/rpm-EE0000?style=flat-square&logo=redhat&logoColor=white"></a>
  <a href="docs/en/installation.md"><img alt="apk" src="https://img.shields.io/badge/apk-0D597F?style=flat-square&logo=alpinelinux&logoColor=white"></a>
  <a href="docs/en/installation.md"><img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
</p>

[Documentation](https://signalridge.github.io/slipway/) |
[Start Here](docs/en/start-here.md) |
[Quick Start](#quick-start) |
[Installation](docs/en/installation.md) |
[Release Notes](CHANGELOG.md)

<br/>

**English** · [简体中文](README.zh.md) · [日本語](README.ja.md)

</div>

# Slipway

**An explicitly invoked, issue-first but never issue-gated soft autopilot for
AI coding. The host writes the code; Slipway schedules bounded work, pins the
source, and reports facts without certifying "done."**

> **English is a non-normative summary.** The complete
> [Chinese product contract](docs/zh/reference/product-contract.md) and the
> versioned [machine protocol schema](docs/reference/machine-protocol.schema.json)
> are the implementation authorities.

AI coding hosts are fast, but they can drift from a goal, lose the thread across
sessions, or treat an Issue as a rubber stamp. Slipway turns one unit of work
into a governed, recoverable Run: a host investigates the repository, clarifies
genuine human decisions, implements a bounded change, reviews the observed diff,
and summarizes — all under explicit user control.

Slipway is not a hosted service, a project tracker, or a replacement for your AI
coding tool. It is the control plane that makes agent work bounded, recoverable,
and honest. It holds no model provider and no GitHub token; a trusted host fetches
the source, and the CLI validates it.

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
            orient → clarify if needed → implement → review on observed diff → summarize
```

## Why Slipway?

| Capability | What it changes |
| --- | --- |
| **Requirements-only** | Slipway keeps no Spec, Delta, or permanent requirements registry. An open Issue is a temporary delivery contract; once delivered, code and tests become current fact. |
| **Issue-first, not issue-gated** | Non-trivial work starts from a self-contained Change Issue, but GitHub can never block a Run. Tiny, sensitive, urgent, offline, or deliberately untracked work starts ad-hoc. |
| **Pinned source** | A Run never trusts a mutable `#42`. The CLI deterministically parses a strict manifest, pins each chapter by digest, and stores only a bounded catalog plus domain-separated revisions. |
| **Six explicit capabilities** | Ten adapters generate exactly `run`, `clarify`, `propose`, `decompose`, `implement`, and read-only `review`. No ambient hooks, no global router, no implicit invocation. |
| **Seven public commands** | `install`, `uninstall`, `list`, `doctor`, `run`, `status`, `stop` — plus versioned hidden `_machine` operations. The machine protocol is a stable contract. |
| **Honest recovery** | Append-only journals under `.git/slipway/runs/` are the recovery authority. A Run can stop, resume, and be replayed; `ended` means only that the queue is empty. |
| **No completion certification** | Test failures, unrun tests, Review findings, dirty worktrees, and Issue state never gate progression. Slipway reports, it does not certify "done", deployable, or shippable. |
| **Untrusted Issue content** | Issue bodies, comments, and labels are data, never instructions. Prompt-injection and credential requests inside an Issue carry no host authority. |
| **Exact destructive authority** | Destructive work needs a one-shot, scope-bound structured grant. Natural-language "yes" never grants it, and a trusted host is an attester, not a cryptographic proof of a human. |

## Design Philosophy

Slipway follows three binding rules:

- **The user owns the process.** Slipway starts only from explicit invocation.
  The user can skip, stop, resume, or take over any Action without giving a
  reason. Ordinary implementation never re-asks for authorization; real human
  decisions, source amendments, environment failures, and destructive work pause.
- **Facts before questions.** The host investigates the repository, Git state,
  and conventions before asking anything. Decisions a host can resolve from code
  are never offloaded to the user; genuine human decisions are asked one at a
  time, each with a recommendation, rationale, and alternatives.
- **Honest reporting.** Slipway reports observed changes, exact activities, exit
  codes, findings, known issues, and uncertainties. It never claims an activity
  ran when it did not, and never turns an empty queue into a correctness,
  delivery, or release-readiness certification.

Requirements are temporary delivery contracts, not a permanent model of the
system. An Objective exists only when one outcome necessarily needs multiple
independent Changes; every Change is self-contained and does not inherit runtime
requirements from its parent or ordinary discussion comments. The first exact
body marker is Level authority; labels, titles, `ready-for-agent`, Project
fields, tests, and findings are warning-only projections that never gate a
marker-valid Run.

Read [Product authority](docs/en/reference/product-overview.md),
[Issue workflow](docs/en/reference/issue-workflow.md), and
[Architecture](docs/en/explanation/architecture.md) for the full model.

## Quick Start

Install Slipway from an official release-backed channel, then generate the host
adapter for the AI tool you actually use.

| Platform | Recommended path |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | Use the `.deb`, `.rpm`, `.apk`, `tar.gz`, AUR, or container image paths in [Installation](docs/en/installation.md#linux-packages). |
| Go fallback | `go install github.com/signalridge/slipway@latest` |

```bash
slipway --version
cd your-repository
slipway install --tool claude
```

Supported tool IDs are `claude`, `codex`, `copilot`, `cursor`, `kilo`, `kiro`,
`opencode`, `pi`, `qwen`, and `windsurf`. Use repeated `--tool`, a comma-separated
value, or `--tool all`. Kiro requires `--surface ide|cli` on first install.

### Ad-hoc escape hatch

Tiny, sensitive, urgent, offline work — or work where you simply do not want an
Issue — starts without any source:

```bash
slipway run --budget 8 --json --root "$PWD" -- "add a CSV export to reports"
```

### Issue-bound Run

A trusted host fetches one strict manifest-addressed Source Bundle once and
passes the transient raw envelope to the CLI:

```bash
slipway run --budget 8 --json --root "$PWD" \
  --source-file /safe/temp/change-envelope.json -- "implement the bounded Change"
```

The CLI validates the Issue-body manifest and its exactly referenced comments,
pins each chapter by digest, and stores only a bounded catalog. The temporary
file and GitHub are never needed for local material reads or resume.

That is the whole interaction model. In your AI-tool session you invoke a
`slipway-<name>` capability explicitly; the host executes one Action at a time,
pausing only for a real decision, a source amendment, an environment failure, or
a destructive confirmation. Control needs no reason: "skip this", "stop", "take
over", and "do X first" are all honored literally.

<details>
<summary><strong>What the CLI does and does not hold</strong></summary>

The CLI does **not**:

- hold a GitHub token or call a model provider;
- implement a GitHub/Project provider or tracker runtime;
- scan ordinary discussion comments or treat comment order as authority;
- create, switch, bind, or delete worktrees;
- claim exactly-once Issue creation or body compare-and-swap;
- promise a secret-free journal.

The CLI **does**:

- strictly validate a host-attested raw envelope and reject unknown fields,
  duplicate JSON keys, bad UTF-8, BOM, and trailing data;
- deterministically parse the manifest and pin chapters by domain-separated
  digest into a private content-addressed material store;
- return one bounded versioned Action at a time with a structured local reader;
- keep an append-only journal as the recovery authority and a replaceable
  projection;
- expose seven public commands plus versioned hidden `_machine` operations.

</details>

## How It Works

| Stage | What Slipway expects |
| --- | --- |
| `orient` | Investigate repository facts, Git state, and conventions before asking anything. Suggest the next Action or pause for a real decision. |
| `clarify` | One dependent human decision at a time, each with a recommendation and trade-offs. Stateless: writes no files, creates no Issue, stops on wrap-up. A complete request asks zero questions. |
| `implement` | Execute the bounded change the current Action authorized. Report exact commands, exit codes, changed files, known issues, and uncertainties. |
| `review` | Read-only check of Intent (does it meet the pinned Requirements?) and Quality. Never edits code, never `needs_input`, never opens a repair loop. |
| `summarize` | Consolidate findings and activities. After acceptance, the Run is `ended`. |

A Run advances one versioned Action at a time. Routing is diff-first: when the
CLI observes a change since the immutable Run-start Git fingerprint and Review is
enabled, it routes to Review regardless of what the host reported. Review always
routes to Summary, and Summary routes to `ended`. Failed activities and Review
findings are **data**, never gates — they flow into the Summary without creating
an automatic repair loop.

## Six Capabilities, Seven Commands

```text
Adapters generate:        run  clarify  propose  decompose  implement  review
Public CLI commands:      install  uninstall  list  doctor  run  status  stop
```

Every capability requires explicit invocation. Clarify follows the attributed
[Matt Pocock `grill-me` / `grilling`](https://github.com/mattpocock/skills)
discipline: investigate facts, walk dependent decisions one at a time with a
recommendation, confirm changed shared understanding, remain stateless, and stop
immediately on wrap-up. Review is read-only and never repairs or opens a
re-review loop.

Hidden versioned `_machine submit/answer/skip/resume/material` operations power
the autopilot loop and are documented in the
[machine protocol](docs/en/reference/machine-protocol.md).

<details>
<summary><strong>AI Tool Adapters</strong></summary>

Generate host-tool surfaces with `slipway install --tool <id>` and refresh
managed files with `slipway install --refresh`. Generated files are
ownership-tracked so refresh replaces Slipway-owned files without deleting
adjacent user-owned customizations.

| Tool | Native surface | Explicit invocation |
| --- | --- | --- |
| `claude` | `.claude/skills` | invoke the `slipway-<name>` skill |
| `codex` | `.codex/skills` (per-skill `agents/openai.yaml`) | `$slipway-<name>` |
| `copilot` | `.github/copilot/agents/*.agent.md` | select the `slipway-<name>` custom agent |
| `cursor` | `.cursor/skills` | invoke the `slipway-<name>` skill |
| `kilo` | `.kilo/commands/*.md` | `/slipway-<name>` |
| `kiro` IDE | `.kiro/steering/*.md` | manually include `#slipway-<name>` |
| `kiro` CLI | `.kiro/agents/*.json` | `kiro-cli chat --agent slipway-<name>` |
| `opencode` | `.opencode/commands/*.md` | `/slipway-<name>` |
| `pi` | `.pi/skills` | `/skill:slipway-<name>` |
| `qwen` | `.qwen/skills` | invoke the `slipway-<name>` skill |
| `windsurf` | `.windsurf/workflows/*.md` | `/slipway-<name>` |

Adapters install no ambient session hook, prompt-submit hook, launcher, global
router, or standalone technical-validation capability. Host settings are outside
adapter ownership and are never modified.

See [Host adapters](docs/en/reference/adapters.md) and
[Installation](docs/en/installation.md) for the exact surfaces, ownership rules, and
Kiro `--surface` handling.

</details>

## How Slipway Compares

Most AI workflow systems structure work with spec files and phase prompts.
Slipway's narrower bet is **bounded, honest authority**: the lifecycle state
lives in a deterministic CLI that recomputes facts from the repository and pins
the source by digest, instead of trusting an agent's summary.

<details>
<summary><strong>Adjacent tools and trade-offs</strong></summary>

| Tool | Model | Done enforcement |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) | Spec files + slash commands | Advisory checklists and phase prompts. |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | Spec-driven workflow | Flexible spec workflow; verification is optional. |
| [GSD Core](https://github.com/open-gsd/gsd-core) | Runtime surfaces + phase commands | Strong phase loop; final proof is artifact-mediated. |
| [superpowers](https://github.com/obra/superpowers) | Auto-firing skills | Strong agent discipline; rules live in model context. |
| **Slipway** | Explicit capabilities + pinned source | Bounded, recoverable Runs; honest reporting; no completion gate. |

Slipway trades breadth for honesty and control. It is lighter than a full spec
framework on a one-line edit (just use ad-hoc), but far stricter when you need a
recoverable, source-pinned, diff-observed trail that never silently rewrites
history or rubber-stamps an Issue.

</details>

## Recovery, Privacy, and Evidence

Recovery authority lives at `.git/slipway/runs/<run-id>/`:

```text
.git/slipway/runs/<run-id>/
├── journal.jsonl   append-only state-transition authority
├── run.json        replaceable projection
├── run.lock        serializes journal mutation
└── materials/      content-addressed chapter blobs (0600)
```

A journal may contain accepted Requirements, goals, user answers, and truthful
command summaries. Slipway minimizes data and redacts recognized credentials, but
**does not promise a secret-free journal** — treat the run directory as local
private data. Unix modes and Windows current-user ACL intent have root, backup,
malware, inherited-ACL, and same-account limitations.

Deleting a run directory removes recovery capability only. It is not secure
erase, backup purge, or key destruction. Read
[Runs and privacy](docs/en/explanation/runs-and-privacy.md),
[Windows behavior](docs/en/reference/windows-rendering-and-durability.md), and the
honest [acceptance evidence matrix](docs/en/reference/acceptance-evidence.md).

## Documentation

Documentation is organized by task:

- [Start here](docs/en/start-here.md) — shortest path from install to one Run.
- [Product authority](docs/en/reference/product-overview.md) — the four-axis model,
  six capabilities, and seven commands.
- [Issue workflow](docs/en/reference/issue-workflow.md) — Objective/Change markers,
  labels, self-containment, GitHub limits, and publication reconciliation.
- [Installation](docs/en/installation.md) — platform paths and adapter commands.
- [Commands](docs/en/reference/commands.md) — public command and JSON surface.
- [Machine protocol](docs/en/reference/machine-protocol.md) — versioned Action /
  Outcome contract and hidden operations.
- [Host adapters](docs/en/reference/adapters.md) — ten hosts, six capabilities,
  ownership safety.
- [Architecture](docs/en/explanation/architecture.md) — package layout and
  dependency direction.
- [Runs and privacy](docs/en/explanation/runs-and-privacy.md) — journal contents,
  retention, and the privacy promise.
- [Windows rendering and durability](docs/en/reference/windows-rendering-and-durability.md)
  — argv rendering and crash durability.
- [Acceptance evidence](docs/en/reference/acceptance-evidence.md) — evidence types
  and the 35-scenario matrix.

## Verification

Useful local checks while developing:

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go test -timeout=20m ./... -race -count=1
go build ./...
git diff --check
```

CI runs Markdown/YAML/action linting, Go linting, Slipway testlint, Go tests
across platforms, race tests, build checks, native Windows cmd/PowerShell suites,
adapter shell acceptance, and the docs build.

## Contributing

Contributions go through a fork-and-pull-request workflow. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the contribution flow and
[development reference](docs/en/contributing.md) for development details.

## License

Slipway is distributed under the [BSD 3-Clause License](LICENSE).
