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

### Context Ownership

Slipway does not watch or measure your context window. There is no
context-pressure hook, no token-budget estimate, and no `next` context guard:
the host owns the decision of when to compact in place or start a fresh session,
using whatever native signal it has. The SessionStart hook advertises this
contract and points at the optional `slipway handoff` surface for capturing an
advisory continuation narrative. Governed continuity itself does not depend on
that narrative — it comes solely from authoritative lifecycle state resumed via
`slipway status` / `slipway next`, with no agent cooperation required.

The SessionStart hook payload carries two static, host-facing keys:
`slipway_entry_skill` (the entry-skill routing pointer) and
`slipway_context_note` (the note that Slipway does not watch context and that
`slipway handoff` is optional advisory continuity). Both are routing/advisory
context consumed by the host, not user-configured settings.

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
