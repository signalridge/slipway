## Why

MVP specs still had three consistency gaps:
- governance evidence did not require `request_id`, while review readiness keys by `(request_id, run_summary_version)`
- governed `tasks.md` had presence checks but no deterministic machine-parseable structure
- CLI failure taxonomy normatively depended on `design.md DEC-30`, creating an avoidable contract drift point

These gaps create ambiguous evidence ownership, planner parsing variance, and brittle source-of-truth boundaries.

## What Changes

- make `request_id` required in governance evidence schemas and use request-scoped skills evidence path
- add canonical governed `tasks.md` task-node structure contract (heading + YAML + schema + dependency validation)
- bind `S4` and `G_plan` to tasks structure validity (not file presence only)
- make CLI failure taxonomy self-canonical in `cli-commands` spec
- reduce denormalized drift by preferring derived mitigation mapping (`skill_name`) and omitting `mitigation_target` by default

## Impact

- closes evidence ownership ambiguity
- makes wave planning deterministic for governed lanes
- removes cross-document contract coupling for CLI failure semantics
- keeps backward compatibility for optional fields and existing evidence roots
