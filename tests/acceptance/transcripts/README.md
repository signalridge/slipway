# Sanitized host transcript evidence

No Claude, Codex, or Pi transcript is collected in this repository yet. Do not create a placeholder transcript that looks like a model run. The H statuses in `../README.md` remain `not collected / external` until an evaluator records a real run.

## Required record format

Create one directory per real run only after collection:

```text
transcripts/<yyyy-mm-dd>-<host>-<scenario>/
  metadata.json
  transcript.sanitized.md
  evaluator-notes.md
  outcome.sanitized.json        # when a Run Action was submitted
```

`metadata.json` must contain only:

```json
{
  "format_version": 1,
  "scenario": 1,
  "host": "claude|codex|pi",
  "host_version": "recorded version",
  "binary_revision": "git revision",
  "capability_sha256": "sha256:...",
  "fixture_revision": "git revision or synthetic fixture ID",
  "started_at": "RFC3339 timestamp",
  "redactions": ["categories only; no original values"]
}
```

`transcript.sanitized.md` records user-visible turns and exact invoked command **shape** in order. Replace credentials, personal data, private repository/account names, absolute user paths, customer text, and unrelated content with typed tokens such as `[REDACTED_TOKEN]` or `[REDACTED_PATH]`. Preserve truthful command identity, argument position, Action kind/ID relationship, status, and exit code. Do not include raw hidden/system prompts, chain-of-thought, environment dumps, full Issue comments, or the unsanitized source envelope.

`evaluator-notes.md` must separate observed facts from judgment and include:

- expected and prohibited behavior from the linked scenario;
- whether invocation was explicit;
- repository facts investigated before questions;
- question count, dependency order, recommendation/trade-offs, shared-understanding confirmation, stateless wrap-up;
- external publication preview/confirmation/reconciliation when relevant;
- Review read-only/no-repair behavior;
- destructive structured grant behavior when relevant;
- uncertainties, deviations, and why the transcript is or is not usable H evidence.

`outcome.sanitized.json` keeps the strict public Outcome shape while redacting values. Verify it against the versioned schema and journal before publication; do not infer activity from prose.

## Evidence still not collected

- H for Claude across the 12 prompt scenarios and publication fault workflow;
- H for Codex across the same set;
- H for Pi across the same set;
- sampled H for Copilot, Cursor, Kilo, Kiro, OpenCode, Qwen, and Windsurf;
- live GitHub G evidence described in `../live-github/README.md`;
- native Windows W evidence from `../windows/`.

These are honest external evidence gaps. Missing H/G/W is not a CLI progression control, Review result, or delivery verdict.
