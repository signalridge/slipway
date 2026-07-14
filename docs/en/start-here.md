# Start here

This page is the shortest path from "I have a repository" to "Slipway is running one bounded change under my control."

Slipway is explicitly invoked, issue-first but never issue-gated, and never certifies "done." Work flows through a small, durable set of surfaces:

| Slipway surface | What it does |
| --- | --- |
| Objective Issue | Optional planning parent for multiple independent deliveries. Never executable. |
| Change Issue | The only issue-backed Run source. Self-contained; carries all effective Requirements. |
| Run | One revision-pinned, interruptible execution attempt under `.git/slipway/runs/<run-id>/`. |
| Host capabilities | Exactly six: `run`, `clarify`, `propose`, `decompose`, `implement`, `review`. |
| Pinned source | Manifest-addressed chapter catalog pinned by digest; raw bodies never persisted. |

The CLI is the authority. The host executes Actions; Slipway schedules them, observes Git independently, and stores recovery history. A host can author Issue drafts and run technical activities, but it should not invent lifecycle state, edit evidence by hand, or treat Issue prose as instructions.

> English is a non-normative guide. The complete [Chinese product contract](../zh/reference/product-contract.md) and [machine schema](../reference/machine-protocol.schema.json) are implementation authorities.

## Choose your path

| Situation | Start with |
| --- | --- |
| You are new to Slipway and want a small end-to-end run. | [Issue workflow](reference/issue-workflow.md) then `slipway run` below. |
| You want the platform and adapter commands. | [Installation](installation.md) and [Host adapters](reference/adapters.md). |
| A Run is paused, stopped, or confusing. | [Commands](reference/commands.md) (`status`, `stop`, resume) and [Runs and privacy](explanation/runs-and-privacy.md). |
| You are evaluating the design. | [Product authority](reference/product-overview.md) and [Architecture](explanation/architecture.md). |
| You need the machine contract. | [Machine protocol](reference/machine-protocol.md). |

## Install and confirm

Pick an official release-backed path for your platform:

| Platform | Recommended path |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | Use the `.deb`, `.rpm`, `.apk`, `tar.gz`, AUR, or container image paths in [Installation](installation.md#linux-packages). |
| Go fallback | `go install github.com/signalridge/slipway@latest` |

Then confirm the binary is visible:

```bash
slipway --version
slipway doctor
```

The full platform matrix, release archive paths, checksum verification, and source-build instructions remain in [Installation](installation.md).

## Generate host capabilities

Run from any directory inside a Git worktree to install the six capabilities for the host you use:

```bash
slipway install --tool claude
slipway install --tool codex,cursor,pi
slipway install --tool all
slipway install --tool kiro --surface ide   # or: --surface cli
```

Without `--tool`, Slipway installs adapters whose host directories it detects. `--refresh` updates only files whose ownership hash still matches. Kiro's first install requires `--surface ide|cli`; later refresh and uninstall infer the recorded surface.

## Start one Run

Work is issue-first, not issue-gated. Use an Objective only for multiple independent deliveries; a Change is the only issue-backed source and must carry all effective Requirements.

### Ad-hoc

For tiny, sensitive, urgent, offline work — or when you simply do not want an Issue:

```bash
slipway run --budget 8 --json --root "$PWD" -- "add a CSV export to reports"
```

### Issue-bound

A trusted host fetches one strict GitHub Change envelope once and passes the transient raw envelope to the CLI:

```bash
slipway run --budget 8 --json --root "$PWD" \
  --source-file /safe/temp/change-envelope.json -- "implement the bounded Change"
```

The marker-valid body is Level authority; title/label drift warns but does not gate. The CLI validates the Issue-body manifest and its exactly referenced comments, pins each chapter by digest, and stores only a bounded catalog. The temporary file and GitHub are never needed for local material reads or resume.

Read the [Issue workflow](reference/issue-workflow.md) before publication. A public Issue has no private switch; sensitive work may require a private repository, an appropriate security channel, or an ad-hoc Run.

## User control

Slipway starts only when you explicitly invoke it. Once a Run is authorized, it advances one versioned Action at a time and pauses only for a real decision, a source amendment, an environment failure, or a destructive confirmation. Control needs no reason:

| Intent | What happens |
| --- | --- |
| **Skip this** | Invokes the exact current skip control for the outstanding Action. |
| **Stop** | Runs `slipway stop`; the journal is preserved and the Run can resume. |
| **Take over** | Stops first, preserves and reports the Run ID, and does not execute the outstanding Action. |
| **Reorder / do X first** | Stops the automatic loop and hands control back; no queue is silently changed and the request is never translated into a skip. |

Work continues only after an explicit resume. The host investigates repository facts before asking. Clarify follows the [Matt Pocock `grill-me`](https://github.com/mattpocock/skills) discipline: one dependent human decision at a time with a recommendation and trade-offs, zero questions for a complete request, confirmation only when grilling changed execution understanding, and immediate stateless stop on wrap-up.

Review is read-only, reports Intent/Quality findings, and never starts a repair loop. `ended` means only that the automatic queue is empty — not correctness, delivery, or release readiness.

## If it pauses

A pause is a feature. Slipway reached a point that needs you or its environment. Every pause and error returns a structured `next` object with typed, resolvable variants — never a shell string to reconstruct.

```bash
slipway status --json          # current state and the fresh derived next
slipway status <run-id> --json
```

Then follow the named recovery variant. Issue-bound resume requires exactly one source mode: import a fresh envelope, explicitly continue with the pinned snapshot, or resolve the current candidate by its exact ID. See [Machine protocol](reference/machine-protocol.md).

## Keep going

- [Product authority](reference/product-overview.md) — the four-axis model.
- [Issue workflow](reference/issue-workflow.md) — markers, labels, publication.
- [Commands](reference/commands.md) — the seven public commands.
- [Host adapters](reference/adapters.md) — the ten hosts.
- [Runs and privacy](explanation/runs-and-privacy.md) — what the journal holds.
