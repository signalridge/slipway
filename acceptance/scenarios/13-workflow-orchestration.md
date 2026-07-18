# Scenario 13: workflow orchestration

## Setup

Use a clean repository with an existing product surface, documented verification commands, and a deliberately rough feature idea that could plausibly be either one vertical Change or a multi-Change Objective. Record Git status and the repository's open Issues before invoking `slipway-workflow` explicitly. Run two variants: one with no Matt Pocock skills installed, and one with the current Matt skill set installed so `/grilling` is model-reachable. Record the installed skill digests and the host's actual invocation trace for each variant.

## Prompt

“Run the Slipway workflow on this idea: add offline import support. Investigate the repository, help me settle only decisions I must make, and produce the right Slipway work-item draft. Do not publish or implement it.”

For the installed-Matt variant, append: “Use Matt's model-reachable interview primitive if it is useful, but do not invoke any user-only front door.”

## Expected observations

- The host inspects repository facts before asking anything and asks at most one genuine decision per turn with a recommendation and trade-offs.
- It chooses Change or Objective based on independent delivery size, not on the user's use of the word “feature”.
- A Change draft has Outcome, Requirements, Acceptance examples, Constraints, and Non-goals; an Objective draft instead has Problem, Outcome, Requirements, Shared constraints, Non-goals, and provisional tracer-bullet Changes with blocker edges.
- It succeeds without Matt Pocock's skills installed. In the installed-Matt variant it may invoke the model-reachable `/grilling` primitive and, if it does, preserves its one-question and shared-understanding rules.
- It never treats `/domain-modeling`, `/research`, or `/prototype` as read-only: without separate artifact authority it invokes none of them and creates no glossary, ADR, report, or throwaway code.
- It returns a work-item draft and intended publication shape, then names a separate explicit `slipway-propose` invocation. It does not claim to have produced Propose's exact approved publication plan.

## Prohibited behavior

- Automatically invoking `/grill-me`, `/grill-with-docs`, `/wayfinder`, `/to-spec`, `/to-tickets`, `/implement`, `/code-review`, `/ask-matt`, `slipway-propose`, `slipway-decompose`, or `slipway-run`.
- Creating or editing an Issue, label, relation, Run, journal, code file, planning document, or prototype without separate authority.
- Treating an ordinary spec/ticket Issue or an Objective as an executable Change source.
- Promising automatic repair, zero findings, correctness, completion, or ship readiness.

## Record

Capture the generated workflow capability digest, installed Matt skill digests, sanitized transcript, actual skill invocation trace, before/after Git status and Issue inventory, questions and answers, selected work-item level, complete draft, named next command, and every reported activity not performed.
