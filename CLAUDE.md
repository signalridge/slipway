# Slipway Agent Principles

Slipway is a user-invoked, interruptible soft autopilot for AI coding. Detailed syntax and protocol fields live in `docs/reference/` and generated capabilities, not in this principle file.

## User owns the process

- Only start a run when the user explicitly asks for one.
- Honor skip, stop, resume, reorder, and manual takeover immediately. Do not ask for a reason.
- Ask for confirmation before destructive operations. Do not request repeated approval for ordinary work that was already requested.

## Investigate before clarifying

- Read the current Git state, relevant code, and repository build/test/typecheck/lint conventions yourself.
- Ask zero questions when the request is complete; proceed without repeated authorization.
- Ask only decisions that repository investigation cannot settle, one at a time, with a recommendation, rationale, and concrete alternatives.
- If clarification changes the implementation understanding, summarize the shared understanding and obtain one confirmation before implementation.
- Stop the interview immediately when the user asks to wrap up.

## Report, do not certify

- Record exact commands, exit codes, changed files, findings, known issues, uncertainties, and activities not performed.
- Never present an unrun activity as run.
- `ended` only means the automatic Action queue is empty.
- Review is advisory and read-only. Findings do not automatically trigger repair work.

## Maintain the whole surface

- Keep the CLI, versioned protocol, host capabilities, docs, and tests consistent.
- Preserve unrelated work and user-modified managed files.
- Make every pause or error name a directly executable next command.
- Do not reintroduce retired commands, ambient hooks, old-state readers, or dual runtime behavior.
