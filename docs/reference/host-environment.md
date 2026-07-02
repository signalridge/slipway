# Host Environment Variables

Slipway keeps repository policy, runtime host facts, and secrets on separate
surfaces. Use `.slipway.yaml` for version-controlled repo policy. Use
environment variables for facts supplied by the current host session and for
secrets that must never be written to project config.

The current machine-readable authority is:

```bash
slipway config list --env
slipway config list --env --json
```

The JSON form includes each variable's scope, secret status, value syntax,
accepted values when constrained, examples, and unset behavior.

## Runtime Host Scope

Runtime-host variables describe the current AI host session. They have no
`.slipway.yaml` key because they describe host capability or identity, not repo
policy.

### Declaring Subagent Capability

Some governed skills require fresh subagent or delegated execution to preserve
independent review evidence. When a skill says the host must declare subagent
capability, the host-facing knob is:

```bash
SLIPWAY_HOST_CAPABILITIES=subagent
```

Accepted `SLIPWAY_HOST_CAPABILITIES` tokens:

| Token | Meaning |
| --- | --- |
| `subagent` | Declares subagent dispatch capability available. |
| `delegation` | Alias that also satisfies the subagent capability. |
| `none` | Explicitly declares the capability unavailable. |
| `unavailable` | Explicitly declares the capability unavailable. |

The value is a comma, semicolon, space, tab, newline, or carriage-return
separated token list. Capability token matching is case-insensitive. Empty or
unset does not mean unavailable; it means unknown. In that state `next`, `run`,
or `validate` can surface a delegation authorization prerequisite plus named
fallback options. Any other non-empty token still declares the host capability
space; if it does not satisfy the required capability, Slipway treats that
capability as unavailable rather than unknown.

When a host intentionally runs degraded same-context evidence, declare the
fallback separately and record fallback evidence with the governed skill:

```bash
SLIPWAY_HOST_CAPABILITY_FALLBACKS=same_context_degraded
```

Skill-specific manual fallbacks include `manual_plan_audit`,
`manual_spec_compliance_review`, `manual_code_quality_review`,
`manual_security_review`, `manual_independent_review`, and
`manual_ship_verification`. Fallback token matching is case-insensitive, and
unrecognized fallback tokens are ignored. Fallback selection does not replace
evidence requirements; it only makes the degradation explicit.

### Context Budget And Pressure

`SLIPWAY_CONTEXT_WINDOW_TOKENS` overrides the assumed context-window size used
by `next` diagnostics and context-pressure hooks. It must be a positive integer.
Malformed, zero, or negative values are ignored and Slipway falls back to the
built-in `200000` token window.

`SLIPWAY_CONTEXT_METRICS_PATH` points the context-pressure hook at a JSON metrics
file written by the host. Supported metric shapes are:

- `tokens_used` with `context_window` or `context_window_size`
- `used_pct`
- `used_percentage`
- `remaining_percentage`

Metrics older than the freshness window are ignored. When the path is unset,
Slipway checks sanitized session temp metric paths and then the transcript tail.

### Handoff Owner

`SLIPWAY_SESSION_OWNER` sets the owner label recorded in handoff notes. When it
is unset, empty, or whitespace-only, Slipway falls back to `USER`, then
`USERNAME`, then machine hostname, then `unknown`.

## Repo Policy And Secret Scope

Some repo policy has both a file key and an environment override. For GitHub
Enterprise API routing, environment values win over file config, and file config
wins over defaults:

| Environment | File key | Notes |
| --- | --- | --- |
| `SLIPWAY_GITHUB_API_URL` | `github.api_url` | HTTPS GitHub REST/GraphQL API base URL. Defaults to `https://api.github.com`. |
| `SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS` | `github.api_allowed_base_urls` | Comma, semicolon, space, tab, newline, or carriage-return separated HTTPS allowlist for override hosts. |

Secrets stay environment-only:

| Environment | Notes |
| --- | --- |
| `GH_TOKEN` | Ambient token for `https://api.github.com`; preferred over `GITHUB_TOKEN`. |
| `GITHUB_TOKEN` | Ambient fallback token for `https://api.github.com`. |
| `SLIPWAY_GITHUB_API_TOKEN` | Token for an allowlisted override host only. |

Ambient `GH_TOKEN` and `GITHUB_TOKEN` are not sent to override hosts.
`SLIPWAY_GITHUB_API_TOKEN` is sent only after the override host is allowlisted
and operator-confirmed.
