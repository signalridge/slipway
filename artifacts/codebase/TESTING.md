# Testing

- Question: Which tests prove additive public surface repair without weakening
  lifecycle fail-closed behavior?
- Route coverage: `cmd/active_change_resolution_test.go` now verifies
  bound-elsewhere CLIError route data, no-active status/validate/next/done
  routes, multi-active status summary routes, explicit-missing routes, and
  `archived_local` routing (`cmd/active_change_resolution_test.go:40`,
  `cmd/active_change_resolution_test.go:123`,
  `cmd/active_change_resolution_test.go:173`,
  `cmd/active_change_resolution_test.go:297`).
- Freshness coverage: next/run/validate handoff views assert split freshness and
  blocked overall readiness for host-capability blockers
  (`cmd/progression_next_test.go:1239`, `cmd/progression_next_test.go:1275`,
  `cmd/progression_next_test.go:1291`); done JSON asserts pre-archive freshness
  and diagnostics (`cmd/lifecycle_commands_test.go:128`); status text asserts
  split labels and absence of the old single-line misleading claim
  (`cmd/status_render_test.go:60`).
- Capability coverage: registry behavior and bounded alias handling are tested
  in `TestResolveHostCapabilityRequirementUsesRegistryContract`
  (`internal/engine/capability/resolver_test.go:175`), and template/registry
  drift is guarded by `TestFrontmatterMirrorsRegistryHostCapabilities`
  (`internal/engine/capability/gates_test.go:76`).
- Integration commands already run for this repair: focused `cmd` route and
  freshness tests, focused capability tests, `go test ./cmd -count=1`,
  `go test ./internal/engine/capability ./internal/tmpl ./internal/toolgen
  -count=1`, `git diff --check`, `go test ./... -count=1`,
  `just coverage-gate`, and the state-read performance baseline check.
- Final verification must be refreshed after any remaining artifact/evidence
  edits: at minimum rerun validation/status/next and rerun the full suite or the
  relevant gates if code changes after this research evidence.
