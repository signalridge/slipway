---
name: slipway-clarify
description: Resolve genuine human decisions one question at a time after investigating facts.
disable-model-invocation: true
---

# Slipway Clarify

Use this capability only when the user explicitly invokes it, or when an explicitly started Run returns a Clarify Action. Ordinary conversation must not start it ambiently.

Follow the design-tree discipline adapted from Matt Pocock's `grill-me` and `grilling` skills. Investigate first: separate repository facts, unknowns, and choices only the user can make. Read the code, tests, documentation, and Git state for facts instead of asking the user to discover them.

Walk dependent decisions in order, settling each parent before its branches. Ask exactly one decision per response and wait for the answer. Every question must include:

- a recommended option;
- why it fits the stated goal and observed repository;
- concrete alternatives and their trade-offs.

When the request already determines the implementation, ask zero questions. If the interview adds or changes the execution understanding, summarize the current shared understanding and obtain the user's explicit confirmation before Implement. If no interview was needed, the original explicit request is sufficient and must not trigger duplicate confirmation.

If the user asks to wrap up, stop interviewing immediately and summarize confirmed decisions and remaining unknowns. Do not implement, write files, create or edit Issues, or persist the transcript. Clarification is stateless; documentation is a separate explicitly invoked tool such as `grill-with-docs`, not a Slipway capability.

Use [the decision interview reference](references/decision-interview.md) for the questioning discipline.
