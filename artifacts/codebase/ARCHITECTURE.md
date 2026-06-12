# Architecture

Re-authored for change
`resolve-github-issue-184-add-gsd-style-automatic-subagent-di`
(GitHub issue #184).

Question: how should Slipway's generated `wave-orchestration` host surface make
`parallel: true` waves executable through real runtime subagents, especially for
Codex, without moving lifecycle or evidence authority out of the CLI?

## Affected Seams

- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:69` through
  `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:116` is the
  generated host contract for wave execution. It already says waves dispatch
  concurrently by default, but it still allows degraded sequential mode as
  visible-but-nonblocking and names only `changed_files`, `test_summary`, and
  `evidence_ref` as executor outputs.
- `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md:34`
  through
  `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md:61`
  is the detailed runtime adapter reference. It currently maps Codex to
  `codex -q --task`, which is the core issue #184 mismatch.
- `internal/toolgen/toolgen_test.go:1069` through
  `internal/toolgen/toolgen_test.go:1094` generates a Claude adapter tree and
  asserts high-level parallel-by-default contract text. This is the established
  generated-surface regression location.
- `internal/tmpl/thin_host_content_test.go:56` through
  `internal/tmpl/thin_host_content_test.go:92` directly renders the
  `wave-orchestration` template and reads the executor dispatch reference,
  making it a good focused place for reference-level contract tests.

## Dependency Flow

Template and reference sources under `internal/tmpl/templates/skills/` feed
`internal/toolgen.Generate`, which renders host-specific adapter trees into
tool roots during tests and during `slipway init --tools ... --refresh`.
Codex project-local skills are still generated under `.codex/skills` in
toolgen tests, while Codex command prompts are global under `$CODEX_HOME/prompts`.

The CLI remains lifecycle authority. Runtime hosts consume generated
instructions, but task evidence is recorded through `slipway evidence task`;
the generated surface must not ask executors to write governed verification
state directly.

## Constraints And Invariants

- `parallel: true` is the host dispatch signal from `slipway next --json`; the
  surface must not make same-context inline execution look equivalent on
  runtimes with a real subagent primitive.
- `parallel: false` and `execution.parallelization: off` must remain clean
  sequential modes without degraded-mode noise.
- Executors share the current Slipway worktree unless a runtime explicitly
  provides stronger isolation. The post-wave integration gate remains the
  coordinator-owned merged-state check.
- Codex `spawn_agent` has no direct equivalent for Claude-style
  `isolation="worktree"`; the generated Codex guidance must not claim that
  isolation unless a separate explicit protocol exists.
