---
name: slipway-clarify
description: Resolve genuine human decisions one question at a time after investigating facts.
disable-model-invocation: true
---

# Slipway Clarify

Use this capability only when the user explicitly invokes it, or when an explicitly started Run returns a Clarify Action. Ordinary conversation must not start it ambiently.

{{ template "interview" . }}

Inside an already-started Run, if the interview adds or changes the execution understanding, summarize the current shared understanding and ask for its explicit confirmation as the single question of the current Clarify Action; the confirmation enters decision context only through the CLI's structured `answer-decision` variant. This is only a consent boundary for the changed understanding—not readiness, quality, Issue status, or delivery certification. If no interview was needed, the original explicit Run request is sufficient and must not trigger duplicate confirmation.

Standalone Clarify never grants implementation authority: end with a summary and wait for a separate explicit Run or Implement invocation. Do not implement, write files, create or edit Issues, persist the transcript, or preserve proposals superseded by later answers as if they remained current. Clarification is stateless; documentation is a separate explicitly invoked tool such as `grill-with-docs`, not a Slipway capability.

Use [the decision interview reference](references/decision-interview.md) for this discipline's provenance and licence.
