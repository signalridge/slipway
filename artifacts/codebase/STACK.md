# Stack

Re-authored for change
`resolve-github-issues-195-and-196-make-status-expose-done-re`
(GitHub issues #195 and #196).

- Language: Go.
- CLI framework: Cobra command tree under `cmd/`.
- Persistence: active governed bundle authority under
  `artifacts/changes/<slug>/change.yaml`; archived terminal authority under
  `artifacts/changes/archived/<slug>/change.yaml`.
- Serialization: `encoding/json` for command output and `gopkg.in/yaml.v3` for
  change/verification files.
- Tests: Go `testing` plus `testify`.
