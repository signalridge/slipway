# Concerns

- Public JSON compatibility risk: route and freshness fields are public command
  surface. The repair must add fields without removing or renaming existing
  fields; `nextView`, `doneView`, status/validate diagnostics, and CLIError
  therefore keep legacy `evidence_freshness` while adding split fields
  (`cmd/next.go:55`, `cmd/done.go:32`, `cmd/errors.go:46`).
- Lifecycle execution safety risk: diagnostic route kinds must not imply
  executable lifecycle authority. No-active, multi-active, explicit-missing,
  and bound-elsewhere routes explicitly set local/effective lifecycle execution
  disabled unless the existing successful explicit route permits it
  (`cmd/common.go:56`, `cmd/common.go:113`, `cmd/common.go:1184`).
- Freshness truthfulness risk: host capability blockers are appended late in
  `next` construction, so overall readiness freshness must be recomputed after
  those blockers are added (`cmd/next.go:500`,
  `cmd/status_view_build.go:244`). Otherwise `next`/`run` could report stale or
  misleading readiness.
- Human status risk: printing one line as `Evidence Freshness: fresh` can hide
  stale governance evidence. Text output must show execution, governance, and
  overall readiness separately (`cmd/status_render.go:145`,
  `cmd/status_render.go:157`).
- Host capability fail-closed risk: independent-review must continue to block
  when the host cannot provide fresh subagent review. Registry metadata can make
  the requirement discoverable, but fallback remains explicit and bounded
  (`internal/engine/capability/registry_default.go:47`,
  `internal/engine/capability/resolver.go:140`).
- Template drift risk: generated skills are public agent contracts. The
  independent-review source template and registry must stay mirrored through
  `TestFrontmatterMirrorsRegistryHostCapabilities`
  (`internal/engine/capability/gates_test.go:76`).
