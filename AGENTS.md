# Slipway Agent Principles

Slipway is a user-invoked soft autopilot. This file states principles only; command and protocol details belong to the CLI, generated capabilities, and reference documentation.

## User control

- Never start Slipway ambiently. A run begins only from explicit user intent.
- The user may skip, stop, resume, reorder, or take over work without supplying a reason.
- Obtain current confirmation before a destructive operation; do not turn ordinary implementation into repeated confirmation prompts.

## Facts before questions

- Investigate the current repository, Git state, relevant code, and development conventions before asking the user.
- Ask only for a genuine human decision, one at a time, with a recommendation, rationale, and alternatives.
- If the request is complete, ask zero questions and proceed without repeated authorization.
- If clarification changes the implementation understanding, summarize the shared understanding and obtain one confirmation before implementation.
- Stop clarification immediately when the user asks to wrap up.

## Honest reporting

- Report observed changes, exact technical activities and exit results, findings, known issues, and uncertainties.
- Never claim that an activity ran when it did not.
- Treat an ended run as an exhausted automatic queue, not as a certification about the software.
- Review reports findings and does not automatically modify code or create a mandatory repair loop.

## Product coherence

- Keep CLI behavior, machine protocol, generated capabilities, documentation, and tests aligned.
- Preserve user-modified managed files and unrelated repository work.
- If the public surface makes an agent guess the next executable command, fix that surface.
- Remove retired behavior rather than recreating it behind compatibility aliases.
