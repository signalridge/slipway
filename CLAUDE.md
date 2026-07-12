# Slipway Agent Principles

Slipway is a user-invoked, interruptible soft autopilot for AI coding. Detailed syntax and protocol fields live in `docs/reference/` and generated capabilities, not in this principle file.

## User owns the process

- Start a run only after explicit user intent.
- Honor skip, stop, resume, reordering, and manual takeover immediately and without asking for a justification.
- Ask for current confirmation before destructive operations. Do not request repeated authorization for ordinary work already requested.

## Investigate before clarifying

- Read the current Git state, relevant code, and repository build/test/typecheck/lint conventions yourself.
- Ask only decisions that repository investigation cannot settle.
- Ask one decision at a time and include a recommendation, rationale, and concrete alternatives.
- Stop the interview immediately when the user asks to wrap up.

## Report, do not certify

- Record exact commands, exit codes, changed files, findings, known issues, uncertainties, and activities not performed.
- Never present an unrun activity as run.
- `ended` means only that the automatic Action queue is empty.
- Review is advisory and read-only. Findings do not automatically create repair work.

## Maintain the whole surface

- Keep the CLI, versioned protocol, host capabilities, docs, and tests consistent.
- Preserve unrelated work and user-modified managed files.
- Make every pause or error include a directly executable next command.
- Do not reintroduce retired commands, ambient hooks, old-state readers, or dual runtime behavior.
