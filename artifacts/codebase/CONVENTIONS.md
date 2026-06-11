# Conventions

Re-authored for change
`resolve-github-issue-185-prevent-s4-goal-verification-from-s`
(GitHub issue #185).

- Keep public CLI command behavior stable unless the issue requires a surface
  change. #185 can be fixed in digest input semantics without changing flags or
  command ordering.
- Required skill freshness must fail closed for real input changes and report
  canonical `required_skill_stale:<skill>:<input>` details.
- Verification YAML, `evidence-digests.yaml`, timestamps, run versions, and
  evidence refs remain engine-owned. Tests may use helpers, but normal workflow
  should not hand-edit these files.
- Use focused package tests for digest behavior before broadening to full
  repository tests.
- Special-case logic should be path-scoped and skill-scoped; avoid broad
  exclusions that make unrelated content invisible to freshness checks.
