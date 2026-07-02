# Concerns

- JSON compatibility: `config list --env --json` is public. Additive
  `omitempty` fields are acceptable; removing or renaming existing fields is not.
- Contract drift: the hidden-contract bug recurs if future env vars can be added
  with only a one-line description. Tests should fail when scoped variables with
  meaningful format, accepted values, or fallback semantics omit structured
  wiring metadata.
- Over-documentation risk: not every env var has enumerated accepted values.
  The schema should distinguish accepted token lists from free-form value syntax
  and unset behavior.
- Security boundary: secret env vars must not imply values can be written to
  `.slipway.yaml`; catalog wording and tests should preserve env-only handling.
- Skill boundary: generated workflow skills should continue to name capability
  prerequisites and evidence fallbacks, but host-integration wiring should stay
  in catalog/docs surfaces to avoid duplicating host manuals across skills.
