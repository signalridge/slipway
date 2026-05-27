# Tasks

## Project Context
- Tech Stack: Go, GitHub Actions, MkDocs, GoReleaser, Release Please
- Conventions: local-first Slipway governance, repo-native validation, least-privilege CI permissions, deterministic release automation
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go, YAML, Markdown

## Task List

- [x] `t-01` harden CI long-path and future slug generation
  - wave: 1
  - depends_on: []
  - target_files: [".github/workflows/ci.yml", "internal/model/identity.go", "cmd/new.go", "internal/model/identity_test.go", "cmd/new_test.go"]
  - task_kind: code
  - acceptance: Windows checkout no longer fails on long tracked paths; generated slugs are capped and collision suffixes are preserved by tests.
  - covers: [REQ-001, REQ-005, REQ-006]

- [x] `t-02` harden docs workflow around Pages settings
  - wave: 1
  - depends_on: []
  - target_files: [".github/workflows/docs.yml"]
  - task_kind: ops
  - acceptance: Docs build remains required, Pages upload/deploy is conditional, and the workflow can inspect Pages status.
  - covers: [REQ-002, REQ-006]

- [x] `t-03` update README and Release Please presentation
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: ["README.md", "release-please-config.json"]
  - task_kind: doc
  - acceptance: README includes clinvoker-style header imagery and badges; Release Please copy documents the review/merge/release flow.
  - covers: [REQ-003, REQ-004]

- [x] `t-04` validate locally and monitor remote Actions
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["artifacts/changes/fix-remote-ci-failures-and-align-docs-release-readme-presentation-with-clinvoker/**"]
  - task_kind: verification
  - acceptance: Local validation commands pass; latest remote CI, Docs, Security, and Release Please runs for `main` are green.
  - covers: [REQ-006]
