# Architecture

Re-authored for change
`resolve-github-issue-151-thin-host-disk-handoff-return-contr`
(GitHub issue #151).

Question: how should remaining heavy governed hosts adopt a disk-handoff
contract without letting subagents self-stamp evidence?

- Affected host surfaces:
  - `internal/tmpl/templates/skills/research-orchestration/SKILL.md:23`
    currently asks the host to read governed artifacts directly and later
    author `research.md`.
  - `internal/tmpl/templates/skills/plan-audit/SKILL.md:18` owns S1 plan bundle
    audit and writes `verification/plan-audit.yaml`.
  - `internal/tmpl/templates/skills/intake-clarification/SKILL.md:20` owns
    intent clarification and writes `verification/intake-clarification.yaml`.
  - `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl` and
    `internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl` are the
    S3 review hosts that produce review certificates.
- Established thin-host precedent:
  - `internal/tmpl/thin_host_content_test.go:11` already pins
    goal-verification delegation for bulky evidence.
  - `internal/tmpl/thin_host_content_test.go:37` pins worktree-preflight bounded
    baseline summaries.
  - `internal/tmpl/thin_host_content_test.go:56` pins wave-orchestration
    path-based delegation and executor-owned codebase-map reads.
- Dependency flow:
  - Authored templates under `internal/tmpl/templates/skills/...` are the source
    of truth.
  - `internal/tmpl` renders static and templated skill content for tests and
    generated exports.
  - Evidence authority remains in Slipway CLI and governed verification files;
    template prose must not introduce a bypass where a subagent return line
    becomes evidence.
- Blast radius:
  - Medium to high workflow semantics: five host surfaces are touched, but the
    implementation should stay in template contracts and tests rather than
    runtime lifecycle state.
