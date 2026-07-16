# Sanitized host transcript format

Use this format only after collecting a real host run. Do not create placeholder transcripts that look like model evidence. The [acceptance matrix](../README.md) is the single place that records current H-evidence availability.

## Required record format

Create one directory per real run:

```text
transcripts/<yyyy-mm-dd>-<host>-<scenario>/
  metadata.json
  transcript.sanitized.md
  evaluator-notes.md
  outcome.sanitized.json        # when a Run Action was submitted
```

`metadata.json` contains only:

```json
{
  "format_version": 1,
  "scenario": 1,
  "host": "recorded host ID",
  "host_version": "recorded version",
  "binary_revision": "git revision",
  "capability_sha256": "sha256:...",
  "fixture_revision": "git revision or synthetic fixture ID",
  "started_at": "RFC3339 timestamp",
  "redactions": ["categories only; no original values"]
}
```

`transcript.sanitized.md` records user-visible turns and exact invoked command **shape** in order. Replace credentials, personal data, private repository/account names, absolute user paths, customer text, and unrelated content with typed tokens such as `[REDACTED_TOKEN]` or `[REDACTED_PATH]`.

Preserve truthful command identity, argument position, Action kind/ID relationship, status, and exit code. Do not include raw hidden/system prompts, chain-of-thought, environment dumps, complete Issue comments, or the unsanitized source envelope.

`evaluator-notes.md` separates observed facts from judgment and includes:

- expected and prohibited behavior from the linked scenario;
- whether invocation was explicit;
- repository facts investigated before questions;
- question count, dependency order, recommendation and trade-offs;
- shared-understanding confirmation and stateless wrap-up;
- external publication preview, confirmation, and reconciliation when relevant;
- read-only Review and no-repair behavior;
- destructive structured grant behavior when relevant;
- uncertainties, deviations, and why the transcript is or is not usable H evidence.

`outcome.sanitized.json` keeps the strict public Outcome shape while redacting values. Verify it against the versioned schema and journal before publication; never infer activity from prose.

Native Windows W evidence is recorded under `../evidence/windows/`; it is not transcript evidence. Live GitHub G collection uses [`../live-github/README.md`](../live-github/README.md).
