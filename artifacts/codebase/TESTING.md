# Testing

- Test layout: this adapter change is primarily covered in
  `internal/toolgen`. Product docs and generated-surface inventory are checked
  by docs/surface-manifest tests. Evidence:
  `internal/toolgen/toolgen_test.go`,
  `internal/toolgen/adapter_contract_test.go`,
  `internal/toolgen/surface_manifest_test.go`.
- Registry and selection tests currently pin the five supported tools and the
  hardcoded `all` expansion; these must change when new IDs are added.
  Evidence: `internal/toolgen/toolgen_test.go:61-78`,
  `internal/toolgen/toolgen.go:711-719`.
- File generation tests assert every selected tool gets sentinels, skills,
  command prompts or command skills, settings/hook behavior, and skill indexes.
  Add Pi and any other selected tools to these expectations rather than only
  smoke-testing registry membership. Evidence:
  `internal/toolgen/toolgen_test.go:403-575`,
  `internal/toolgen/toolgen_test.go:943-983`.
- Frozen adapter contracts pin command roots, styles, extensions, trigger
  strings, settings paths, and hook launcher behavior. New adapters need frozen
  rows so path or invocation drift is caught. Evidence:
  `internal/toolgen/adapter_contract_test.go:38-159`,
  `internal/toolgen/adapter_contract_test.go:161-176`.
- Ownership/refresh coverage already includes manifest-backed refusal for
  modified generated files and preservation of legacy/user prompt files without
  ownership proof. New roots such as `.pi`, `.qwen`, `.kiro`, `.github`,
  `.windsurf`, and `.kilocode` need explicit auto-detect and refresh
  assertions. Evidence:
  `internal/toolgen/toolgen_test.go:1028-1069`,
  `internal/toolgen/toolgen_test.go:1147-1165`,
  `internal/toolgen/toolgen_test.go:1880-1905`.
- Surface manifest tests derive adapter rows from `Registry()` and fail if the
  committed `docs/SURFACE-MANIFEST.json` is stale or docs tokens are missing.
  Any added adapter requires regenerating the manifest and updating docs first.
  Evidence: `internal/toolgen/surface_manifest.go:31-56`,
  `internal/toolgen/surface_manifest_test.go:15-57`,
  `internal/toolgen/surface_manifest_test.go:134-160`.
- Verification commands: targeted proof should include
  `go test ./internal/toolgen/...`; final proof should include `go test ./...`
  unless an unrelated harness issue blocks it.
