# Stack

Re-authored for change
`resolve-github-issue-185-prevent-s4-goal-verification-from-s`
(GitHub issue #185).

- Language: Go.
- CLI framework: Cobra command tree under `cmd/`.
- Persistence: YAML governed bundle authority under
  `artifacts/changes/<slug>/change.yaml`, verification YAML under
  `artifacts/changes/<slug>/verification/`, and runtime task evidence under
  `.git/slipway/runtime/changes/<slug>/`.
- Serialization: `gopkg.in/yaml.v3` for strict authority decoding and
  `encoding/json` via `model.ComputeInputHash` for canonical digest payloads.
- Tests: Go `testing` plus `testify`.
- Expected checks: focused progression tests, full Go test suite, `git diff
  --check`, and Slipway `validate --json`.
