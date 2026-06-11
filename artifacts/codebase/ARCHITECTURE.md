# Architecture

Re-authored for change `resolve-issue-163-decisions-gate` (GitHub issue #163).

Question: where should Slipway parse `decision.md` status and fail closed when a
stage would build on a superseded or deprecated decision?

- Affected modules:
  - `internal/engine/artifact/decision_contract.go:34` through
    `internal/engine/artifact/decision_contract.go:64` evaluates decision
    artifact substance for `validate --json` and instruction-readiness surfaces.
  - `internal/engine/artifact/decision_contract.go:78` through
    `internal/engine/artifact/decision_contract.go:96` enforces required
    decision sections and template-placeholder rejection.
  - `internal/engine/artifact/manager.go:463` through
    `internal/engine/artifact/manager.go:488` parses selected direction and
    selected approach text from `decision.md`.
  - `cmd/next_skill.go:20` through `cmd/next_skill.go:40` injects parsed
    decision text into next-skill constraints as pending or locked depending on
    `G_plan`.
  - `internal/engine/progression/validation.go:587` through
    `internal/engine/progression/validation.go:610` wires decision contract
    blockers into plan-audit and post-plan readiness.
  - `internal/model/reason_code.go` and `internal/model/recovery.go` own the
    canonical diagnostics and recovery guidance for new fail-closed blockers.
- Dependency flow:
  - Artifact parsing belongs in `internal/engine/artifact` so `cmd` and
    progression can share the same structured decision result.
  - Progression readiness should consume the parsed decision contract to block
    `S1_PLAN/audit` and later when status is dead.
  - Next-skill constraints should stop surfacing pending or locked decisions when
    the decision status is dead, and should use the same parser as readiness.
- Architectural boundary:
  - Keep status parsing independent from lifecycle gate approval. Status answers
    whether the decision is usable; `G_plan` still answers whether an otherwise
    usable decision is pending or locked.
  - Preserve #119 behavior: missing, unreadable, structurally empty, and
    template-only decisions stay on existing contract paths.
  - Do not move decision authoring into research; `research.md` remains upstream
    input and `decision.md` is authored by plan-audit.
- GSD reference:
  - Local GSD Core has a status reject set for `superseded`, `rejected`, and
    `deprecated`, status-heading aliases, and a normalizing
    `shouldRejectAdrStatus` helper in
    `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/adr-parser.cts`.
  - Slipway should borrow the pattern, not the ADR file model.
