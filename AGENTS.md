# Slipway Agent Principles

Slipway is a user-invoked, interruptible soft autopilot for AI coding. This file states principles only. Command syntax and protocol fields belong to the CLI, the generated capabilities, and `docs/reference/`, not here.

This is the single principle file for every agent working in this repository. `CLAUDE.md` points here on purpose: two copies of these rules drifted into two different wordings once already.

## User owns the process

- Never start Slipway on its own. A run begins only from explicit user intent.
- Honor skip, stop, resume, reorder, and manual takeover immediately. The user owes no reason.
- Ask for confirmation before a destructive operation. Do not require repeated approval for ordinary work that was already requested.

## Investigate before clarifying

- Read the current Git state, relevant code, and repository build, test, typecheck, and lint conventions yourself.
- Ask zero questions when the request is complete, and proceed without repeated authorization.
- Ask only genuine human decisions that repository investigation cannot settle, one at a time, with a recommendation, rationale, and concrete alternatives.
- If clarification changes the implementation understanding, summarize the shared understanding and obtain one confirmation before implementation.
- Stop the interview immediately when the user asks to wrap up.

## Report, do not certify

- Record exact commands, exit codes, changed files, findings, known issues, uncertainties, and activities not performed.
- Never present an unrun activity as run.
- `ended` means the automatic Action queue is empty. It is not a judgement about the software.
- Review is advisory and read-only. Findings never automatically trigger repair work.
- Report state the product can honor. Do not publish authority, an Action, or an observation that the current state contradicts.

## Maintain the whole surface

- Keep the CLI, versioned protocol, host capabilities, docs, and tests consistent.
- Preserve unrelated work and user-modified managed files.
- Never make an agent guess its next step. A pause or error names the operation and its typed inputs; if the public surface leaves the next command to guesswork, fix that surface.
- Do not reintroduce retired commands, ambient hooks, old-state readers, dual runtime behavior, or compatibility aliases. Remove retired behavior instead.
