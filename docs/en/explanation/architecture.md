# Architecture

Slipway keeps the control loop in a local CLI and the model-specific work in generated host adapters. This boundary lets the CLI validate state without owning model or GitHub credentials.

![Slipway process architecture: a user explicitly invokes a generated capability in an AI coding host; the host owns model, repository, and authorized GitHub work, while versioned JSON connects it to the local CLI and durable Run store.](../../assets/diagrams/architecture.svg)

## Process boundaries

```text
User
  └─ explicitly invokes a generated capability
       └─ AI coding host
            ├─ reads and changes the repository
            ├─ calls the model and development tools
            ├─ fetches or publishes GitHub data when authorized
            └─ exchanges versioned JSON with Slipway
                 └─ local CLI and Run store
```

The Run state engine never calls a model or GitHub API. It validates a host-provided source envelope, schedules one Action at a time, observes Git independently, and stores recovery state. The public `doctor` command is a command-layer diagnostic exception: it may invoke the user's local `gh` to inspect authentication and repository permissions. Generated host instructions define how a host should investigate, publish, implement, and report; they are not another state engine.

## Package direction

Production dependencies are constrained by an architecture test:

```text
cmd ───────────────→ adapter
 │                   ├─→ tmpl
 │                   ├─→ fsutil
 │                   └─→ jsonstrict
 ├─────────────────→ autopilot
 │                   ├─→ runstore
 │                   │    ├─→ fsutil
 │                   │    └─→ jsonstrict
 │                   └─→ jsonstrict
 └─────────────────→ recoverycmd
```

| Package | Responsibility |
| --- | --- |
| `cmd` | Cobra commands, human/JSON output, root discovery, and exit behavior. |
| `internal/autopilot` | Action/Outcome validation, routing, source candidates, budgets, and structured recovery. |
| `internal/runstore` | Journal replay, projections, locking, material storage, and Git observations. |
| `internal/adapter` | Host registry, generated files, ownership manifests, and transactional install/remove. |
| `internal/tmpl` | Embedded shared capability instructions. |
| `internal/fsutil` | Anchored paths, no-follow operations, transactions, synchronization, and platform safety. |
| `internal/jsonstrict` | Strict JSON decoding shared by protocol, source, store, and adapter boundaries. |
| `internal/recoverycmd` | Human rendering of already structured argv. |

Lower layers do not import command or host-policy layers. GitHub publication remains in generated host instructions rather than becoming a network provider inside the core.

## Run start and repository observation

A new Run discovers three canonical paths: the worktree root, the per-worktree Git directory, and the Git common directory. Their framed identifier binds the Run to that worktree. Slipway does not create, switch, or delete worktrees, but it refuses to mutate a Run from another worktree identity.

The initial Git observation stores fingerprints over exact index and porcelain-v2 command output plus bounded metadata and fingerprints for dirty paths. It does not store those raw Git streams or file content. Later observations support diff-first routing and neutral “changed since start” reporting without claiming which process caused the change.

## Run storage

```text
<git-common-dir>/slipway/runs/<run-id>/
├── journal.jsonl   append-only transition record
├── run.json        replaceable projection
├── run.lock        validated coordination artifact
└── materials/      accepted source sections by content digest
```

On Unix, an OS lock on the opened Run directory serializes writers; on Windows, a named mutex does. The visible `run.lock` file supports validation and diagnostics but is not the sole writer guard.

A mutation writes referenced material before a journal event may point to it. The journal is synchronized before the projection is replaced. If projection refresh fails after a committed journal write, the error reports the committed mutation and stale projection instead of claiming a rollback.

## Source boundary

For issue-backed work, the trusted host fetches the Issue and manifest-referenced comments, then passes a temporary strict envelope. The CLI can validate internal consistency and stable IDs, but it cannot cryptographically prove that the host fetched GitHub honestly.

Accepted sections are content-addressed and available through a local material reader. Actions carry only revisions and a bounded catalog, keeping large requirements out of Action context and allowing offline recovery.

The design rationale and rejected alternatives are recorded in [ADR-0001](../../../adr/0001-source-bundle-v2.md). The complete contract in issue #434 and the versioned schemas are normative; runtime tests are executable evidence of the current implementation.

## Security boundary

![Slipway trust boundaries: Issue content and the working tree are untrusted data that can never grant shell authority, disclose credentials, bypass confirmation, or widen destructive scope; the AI coding host is trusted to act and holds every credential; the local CLI validates strict JSON, sizes, identities, and digests while holding no credentials, but cannot prove the host fetched GitHub honestly.](../../assets/diagrams/trust-boundary.svg)

Slipway assumes that processes with the same account, root, malware, or a compromised host can exceed its protections. Within that boundary it:

- anchors filesystem operations and rejects unsafe symlink traversal;
- validates strict JSON, sizes, identities, and digests;
- keeps credentials out of Slipway storage and GitHub fetch/publication out of the Run core;
- separates one-shot destructive grants from natural-language answers;
- preserves user-modified generated files;
- reports platform durability limitations.

Issue content is data, not host instruction. A generated capability must not treat a command, link, or credential request inside an Issue as permission.

## Deliberate non-responsibilities

Slipway does not:

- run a hosted service or project tracker;
- manage model-provider or GitHub credentials;
- create or manage worktrees;
- certify merge, deployment, or release readiness;
- turn tests, findings, labels, or Issue state into universal repository policy;
- repair Review findings automatically;
- overwrite user-modified adapter files.

External branch protection, CI, organizational policy, and human review remain independent.

See [Core concepts](concepts.md), [Machine protocol](../reference/machine-protocol.md), and [Runs, recovery, and privacy](../guides/runs-and-recovery.md).
