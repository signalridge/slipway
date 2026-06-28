# Architecture

- Question: Which public lifecycle surfaces need additive route, freshness, and
  host-capability metadata so agents can recover from non-success states without
  parsing prose or relying on private resolver knowledge?
- Route entry points: `CLIError` now has an optional `invocation_route`
  payload (`cmd/errors.go:46`), and common route builders cover diagnostic
  paths for `no_active`, `multi_active`, `explicit_missing`, and
  `bound_elsewhere` (`cmd/common.go:56`, `cmd/common.go:80`,
  `cmd/common.go:91`, `cmd/common.go:102`, `cmd/common.go:113`). Existing
  successful route assembly remains centralized in `buildInvocationRouteView`
  (`cmd/common.go:1184`), with local archived worktrees distinguished as
  `archived_local` (`cmd/common.go:1211`).
- Command collaborators: `status` attaches route data to diagnostic summaries
  and explicit-missing errors (`cmd/status.go:322`, `cmd/status.go:603`);
  `validate` carries route data from resolution errors into diagnostics
  (`cmd/validate.go:180`); `next` and `done` reuse active-change resolution and
  CLIError route payloads rather than adding separate route grammars.
- Freshness entry points: `nextView` and default handoff views now expose
  execution, governance, and overall readiness freshness (`cmd/next.go:55`,
  `cmd/next_handoff.go:22`), while `doneView` reports the same pre-archive
  concepts plus diagnostics (`cmd/done.go:32`). Shared projection helpers live
  in `cmd/status_view_build.go:206`, `cmd/status_view_build.go:238`, and
  `cmd/status_view_build.go:266`.
- Human status rendering remains the text projection of the same readiness
  model: it falls back to the legacy single value only when split fields are
  absent, then prints explicit execution/governance/overall labels
  (`cmd/status_render.go:145`, `cmd/status_render.go:157`).
- Host capability authority moves from resolver-only hardcoding to registry
  metadata. `Skill.HostCapabilities` and validation live in
  `internal/engine/capability/registry.go:88` and
  `internal/engine/capability/registry.go:249`; the independent-review
  contract is declared by the default registry
  (`internal/engine/capability/registry_default.go:47`); the resolver consumes
  registry data through `ResolveHostCapabilityRequirementFromRegistry`
  (`internal/engine/capability/resolver.go:96`).
- Blast radius: the repair stays in public command JSON/text projections,
  capability registry/template metadata, and tests. It does not change
  lifecycle mutation ordering, durable state formats, or GitHub protection
  settings.
