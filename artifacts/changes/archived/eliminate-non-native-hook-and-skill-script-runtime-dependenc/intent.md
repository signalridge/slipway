# Intent

## Summary
Eliminate non-native hook and generated skill-script runtime dependencies while preserving the best authenticated backend for manual GitHub helpers.
## Complexity Assessment
complex
The change crosses generated adapter surfaces, hidden hook commands, skill
script guidance, helper execution, docs, health diagnostics, and tests. It must
preserve governed lifecycle behavior while removing shell/Python script payloads
from generated hooks and skill helper execution paths. Manual GitHub helper
commands may use `gh` when that is the best authenticated backend, with a token
API fallback and explicit fail-closed behavior when no authenticated backend is
available.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Move generated hook behavior behind compiled Slipway commands so hooks do not
  require bash, PowerShell, Python, jq, gh, or a Go toolchain at runtime.
- Replace generated hook settings that invoke `.sh` files with dependency-free
  Slipway binary entrypoints or generated platform-native launchers that fail
  closed to no-op when the binary is unavailable.
- Inventory skill scripts and remove non-native runtime assumptions from
  exported skill guidance by replacing executable helper paths with Slipway
  binary commands where practical.
- Prefer the best explicit backend for each manual helper: local helpers stay
  inside the Slipway binary, GitHub helpers prefer authenticated `gh` and fall
  back to token-backed API, and domain tools such as `go test`, CodeQL, or
  Semgrep remain explicit operator dependencies only where the workflow domain
  requires them.
- Update toolgen contracts, generated adapter surfaces, docs, and health/tests
  so the dependency-free runtime contract is enforced rather than implied.

## Out of Scope
- Maintaining backward compatibility with legacy `bash ".*/hooks/*.sh"`
  registrations.
- Maintaining duplicated business logic across shell, PowerShell, Python, or
  other script runtimes. Thin generated platform launchers are allowed when
  they only locate/execute the Slipway binary and contain no product logic.
- Installing third-party tools implicitly from hooks or helper commands. Skills
  may document explicit operator installation/authentication steps for necessary
  manual dependencies.
- Changing unrelated governed lifecycle semantics.

## Constraints
- Automatic hook execution should depend on the compiled Slipway binary and the
  operating system only.
- Manual GitHub helper execution should prefer authenticated `gh`, fall back to
  `GH_TOKEN`/`GITHUB_TOKEN` API when `gh` is unavailable or reports an
  auth-required error, and fail closed when neither backend is available. It
  must not make unauthenticated GitHub calls or install tools implicitly.
- Platform-specific launcher files are acceptable when rendered from templates
  and kept as thin invocation shims rather than separate implementations.
- If the Slipway binary is unavailable from an automatic hook path, the hook must
  not block the host AI tool.
- Manual skill-helper failures must be explicit; agents must not treat an
  unavailable helper as successful evidence.
- Durable edits belong in `internal/tmpl/templates`, `internal/toolgen`, `cmd`,
  docs, and tests; generated `.codex`/`.claude` outputs are refreshed from
  source templates.

## Acceptance Signals
- Generated registered hooks no longer invoke `bash` or `.sh` as the canonical
  path.
- Hook behavior has Go tests that do not require bash.
- Skill helper guidance no longer presents shell/Python helpers as required
  runtime paths; helpers needed for supported workflows are exposed through the
  Slipway binary.
- GitHub helper guidance and tests document backend selection: `--backend auto`
  prefers `gh`, `--backend api` requires a token, `--backend gh` requires GitHub
  CLI, and no unauthenticated fallback is attempted.
- Health/toolgen tests detect stale legacy hook registrations or non-native
  helper runtime assumptions.
- Targeted tests plus repo-wide `go test -count=1 ./...` pass.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Which current generated hook surfaces are registered by settings versus
  only emitted as files, and what is the smallest no-compat replacement surface?
- [x] Which skill scripts are executable workflow helpers versus optional
  references, and which should become Slipway binary subcommands in this change?

## Deferred Ideas
- Publishing package-manager-specific installation repairs.
- Remote CI follow-up after local governed completion unless explicitly
  requested.

## Approved Summary
Approved by user instruction on 2026-06-14 after rescope: use the best backend
choice rather than forcing a Go-only GitHub implementation. Automatic hooks
remain dependency-free and fail-open when the Slipway binary is unavailable.
Generated executable helper scripts remain removed. Manual GitHub helpers use
`gh` as the preferred authenticated backend, fall back to token-backed API when
`gh` is unavailable or reports an auth-required error, and fail closed with
remediation when neither backend is available. Necessary domain dependencies such
as the Go toolchain for
`find-polluter-go` remain explicit and fail closed.
