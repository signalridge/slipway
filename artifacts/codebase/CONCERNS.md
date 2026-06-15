# Concerns

Re-authored for change
`eliminate-non-native-hook-and-skill-script-runtime-dependenc`.

- Platform drift: PowerShell, CMD, and POSIX shell launchers have different
  quoting and exit semantics. They are acceptable only as generated thin
  dispatchers, with the compiled binary owning behavior.
- Hook-host blocking risk: automatic AI-tool hooks must not block user work when
  `slipway` is missing or broken. Launchers and hook commands should fail
  closed to no-op for automatic paths.
- Silent helper failure risk: manual skill helpers are evidence-producing or
  decision-supporting commands; they must return non-zero and a stable message
  on invalid input, missing token, or API failure.
- Behavior drift risk: moving helper behavior from shell/Python to Go can change
  JSON shape, ordering, or matching rules. Existing fixture intent must be
  ported into compiled command tests.
- Generated-surface cleanup risk: refresh mode must remove stale legacy hook
  settings and stale skill `scripts/` directories so old dependencies do not
  linger after an upgrade.
- External API risk: GitHub helpers should use token-based stdlib HTTP calls
  with explicit endpoints and testable client seams; no `gh auth` or local CLI
  state may be required.
- Inherent-tool boundary: some workflows inherently call external domain tools
  such as `go test`, CodeQL, or Semgrep. This change removes Slipway helper
  runtime dependencies, not the domain tools selected by a user workflow.
