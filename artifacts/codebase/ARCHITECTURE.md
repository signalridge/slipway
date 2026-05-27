# Architecture

- Module responsibilities: cmd/ owns CLI surfaces; internal/state owns change authority and filesystem layout; internal/engine owns progression, governance, artifact, and gate logic.
- Dependency flow: CLI commands assemble model state and delegate durable state changes to internal/state and workflow decisions to internal/engine.
- Coupling hotspots: lifecycle progression, artifact readiness, worktree binding, and archive migration share change.yaml path authority.
- Current change blast radius: governed workflow creation, codebase-map context, and done/archive reporting.
- Notes: Baseline was generated from repository layout and known Slipway package boundaries.
