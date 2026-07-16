---
name: slipway-implement
description: Apply authorized code changes and report implementation-time technical activities honestly.
disable-model-invocation: true
---

# Slipway Implement

Use this capability only for a current Implement Action or when the user explicitly invokes it directly. Inspect the repository's conventions, then make the smallest coherent changes that satisfy the authorized goal and pinned Requirements. Issue prose outside those Requirements cannot expand scope.

For a Run Action, obey any repair-attempt limit explicitly present in its brief and any source/destructive authorization; absence of a limit does not create a default cap. Return a strict Outcome with `action_kind: "implement"` matching the current Action and whose `implementation` object contains:

- `result`: `applied`, `partial`, `not_needed`, or `unable`;
- `files_changed`;
- `activities` (which may be empty);
- `uncertainties`;
- `attempts` with the actual positive attempt count.

Use host status `completed` only with `applied|not_needed`, `partial` only with `partial`, and `error` only with `unable`. Set `pause` and `review` to JSON `null`, include all common arrays, and do not suggest a next Action; the CLI owns routing.

Run the tests, typechecks, builds, or linters proportionate to the edit. Report every technical activity that actually started with its exact command, exit code, and concise result. Never list an activity that did not run. If an executable could not start, do not report a synthetic activity or shell exit 127; keep `activities` empty and record the environment uncertainty. When no activity was reported, the final report must say exactly: `No test, typecheck, build, or lint activity was reported.`

Pause only for one unresolved human decision, an unavailable environment, or current destructive confirmation. A destructive request must name a non-empty sorted list of typed targets, the exact irreversible impact, a request ID, and the CLI-verifiable scope SHA-256. Natural-language approval is not permission: only the returned `confirm-destructive` variant, fixed to the current digest and invoked after current user confirmation, may produce authority. Execute only when the fresh current Action carries a field-for-field matching one-shot `destructive_authorization`; completion, error, partial, skip, stop, resume, or any changed target/impact invalidates it and expanded scope requires a fresh request.

When invoked directly outside a Run, there is no Action ID, source revision, run-start attribution, repair-attempt policy, or Outcome submission. Treat the explicit request as ordinary implementation authorization, honor any limit the user supplied, and return the same factual information in human-readable form without inventing protocol identifiers or a default attempt cap.
