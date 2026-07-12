# Host adapters

Slipway generates exactly six explicit capabilities for every supported host:

```text
slipway-run
slipway-clarify
slipway-propose
slipway-decompose
slipway-implement
slipway-review
```

`slipway-run` is the only autopilot entry. `clarify` is stateless. `propose` drafts or publishes explicitly confirmed managed Issues, `decompose` creates confirmed Change relationships, `implement` owns technical activities, and `review` is read-only and reports Intent and Quality findings.

Clarify follows the attributed Matt Pocock `grill-me`/`grilling` behavior: investigate facts, walk dependent decisions, ask one question with a recommendation, confirm changed shared understanding, remain stateless, and stop immediately on wrap-up. There is no implicit clarification-document capability.

| ID | Capability directory | Detection directory |
| --- | --- | --- |
| `claude` | `.claude/skills` | `.claude` |
| `codex` | `.codex/skills` | `.codex` |
| `copilot` | `.github/skills` | `.github/copilot` |
| `cursor` | `.cursor/skills` | `.cursor` |
| `kilo` | `.kilocode/skills` | `.kilocode` |
| `kiro` | `.kiro/skills` | `.kiro` |
| `opencode` | `.opencode/skills` | `.opencode` |
| `pi` | `.pi/skills` | `.pi` |
| `qwen` | `.qwen/skills` | `.qwen` |
| `windsurf` | `.windsurf/skills` | `.windsurf` |

Each capability is a `slipway-<name>/SKILL.md` directory. Every generated skill carries the same untrusted-Issue, trusted-attester, confirmed-publication, and exact destructive-authorization boundaries. Clarify alone receives one `references/decision-interview.md`, adapted from Matt Pocock's MIT-licensed `grill-me` skill with attribution preserved.
Codex capabilities also contain managed `agents/openai.yaml` policy files with `allow_implicit_invocation: false`; Codex does not honor the generic skill frontmatter for this setting. This keeps every Slipway capability invisible to implicit model selection until the user explicitly invokes it.

Adapters do not install ambient session hooks, prompt-submit hooks, launchers, a global router, or a standalone technical-validation capability. Pi settings are preserved rather than removing generic user skill registrations.

## Publication and privacy boundary

Propose/decompose detect `gh` and use first-class relationships at 2.94.0 or newer; older/missing versions use the official REST API or report `environment_unavailable`. They enforce exact Level/Kind labels, same-`github.com` transfer identity refetch, 100/50 limits, approved operation/item UUID markers, body files, expected revisions, readback, and zero/one/multiple reconciliation with `created|matched|failed|ambiguous`—never blind retry.

Every capability warns that accepted Requirements, goals, answers, and command summaries may be sensitive. A public Issue has no private switch. Recognized credential values are redacted while command identity is retained; tokens, raw comments, environment dumps, transcripts, and hidden reasoning are not collected. See [Issue workflow](issue-workflow.md) and [privacy](../explanation/runs-and-privacy.md).

## Ownership safety

The manifest is stored under `<host-root>/slipway/ownership-manifest.json`. Version 2 records repository-relative paths and SHA-256 hashes. Refresh and uninstall mutate only hash-matching files. Modified, unknown, marker-only, malformed, duplicate, out-of-host, or symlinked surfaces fail safely or are preserved and reported.
A first install claims only newly created files. Once a manifest exists, updates require `slipway install --refresh`; this keeps an ordinary repeated install from silently switching managed surfaces. A version 1 manifest is read only as proof for deleting or replacing hash-matching legacy files.
