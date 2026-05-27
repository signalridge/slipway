# Testing

- Test layout: cmd/*_test.go covers CLI contracts; internal/**/*_test.go covers state, artifact, progression, governance, and template behavior.
- Coverage hotspots: next/run/status JSON contracts, governed lifecycle gates, archive migration, worktree binding, and generated skill/template drift.
- Coverage gaps: End-to-end governed workflow tests are intentionally heavier and should use explicit timeouts.
- Verification commands: go test -timeout=20m ./... -count=1; go build ./...
- Fixture patterns: Tests commonly create temp workspaces, seed governed bundles, write verification YAML, and assert JSON command output.
- Notes: Prefer focused regression tests before full-suite verification.
