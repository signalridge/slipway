# Tasks
## Project Context
- Tech Stack: Go
- Conventions: mkdocs-based site under `docs/`; preserve `README.md:166` anchor link to `docs/installation.md#ai-tool-installation-prompt`; `MaxSlugLength` constant lives at `internal/model/identity.go:10`.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Task List

- [x] `t-01` Rewrite the `## AI Tool Installation Prompt` section in `docs/installation.md`: replace the fenced code block with a short (~10-line) pointer prompt that instructs the agent to fetch `https://signalridge.github.io/slipway/installation/` and follow it, including OS/arch detection, documented-release-source preference, the "do not install same-name packages from unrelated registries" safety line, and post-install verify + report. Add readable prose below the code block under named sub-sections **Discovery**, **Install** (numbered preference list with per-step recovery branches matching `.goreleaser.yaml` channels), **Initialize**, **Verify**, **Report**. Preserve the heading text so the anchor `ai-tool-installation-prompt` stays stable. Do not modify anything outside this section.
  - wave: 1
  - depends_on: []
  - target_files: [docs/installation.md]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004]

- [x] `t-02` Add an additive copyable preview of the short pointer prompt to `README.md` near `## Install` / `## Quick Install`. Use the same prompt text as the canonical `docs/installation.md` version. Include a short framing line ("review before pasting; supervise the agent") and a link back to the canonical anchor `docs/installation.md#ai-tool-installation-prompt`. Do NOT duplicate the Discovery/Install/Initialize/Verify/Report prose in the README. Keep the existing `README.md:166` link unchanged.
  - wave: 1
  - depends_on: []
  - target_files: [README.md]
  - task_kind: code
  - covers: [REQ-002, REQ-004]

- [x] `t-03` Reduce the slug-length cap by changing `MaxSlugLength` in `internal/model/identity.go` from `96` to `60`. Update `internal/model/identity_test.go` if the test references the old constant value directly; otherwise leave the test assertion (which compares against `MaxSlugLength`) unchanged. Confirm by running `go test -count=1 ./internal/model/...` and observing `TestSlugifyTitleLimitsLongSlugs` passing under the new value.
  - wave: 1
  - depends_on: []
  - target_files: [internal/model/identity.go, internal/model/identity_test.go]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-04` Mechanical verification: run `mkdocs build --strict` (confirms clean build, no broken anchors, `README.md:166` anchor resolves), `go build ./...`, and `go test -count=1 ./...`. All three must pass on the worktree.
  - wave: 2
  - depends_on: [t-01, t-02, t-03]
  - target_files: [docs/installation.md, README.md, internal/model/identity.go, internal/model/identity_test.go]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]

- [ ] `t-05` Visual review: operator inspects the rendered `README.md` (GitHub markdown) and `docs/installation.md` (mkdocs build output). Confirm the short pointer prompt reads as a self-contained copyable block on both surfaces, the canonical Discovery/Install/Initialize/Verify/Report prose under `docs/installation.md` is clear, and the README link to the canonical section still works.
  - wave: 3
  - depends_on: [t-04]
  - target_files: [docs/installation.md, README.md]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-004]

- [ ] `t-06` Operator-gated acceptance: paste the short pointer prompt into Claude Code on a clean shell and confirm the agent fetches the canonical Installation page and drives a successful end-to-end install (`slipway --version` resolves on PATH). Final human acceptance gate.
  - wave: 4
  - depends_on: [t-05]
  - target_files: [docs/installation.md, README.md]
  - task_kind: verification
  - covers: [REQ-001, REQ-002]
