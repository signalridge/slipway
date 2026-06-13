# Conventions

Re-authored for change
`resolve-github-issues-195-and-196-make-status-expose-done-re`
(GitHub issues #195 and #196).

- Keep active lifecycle state authoritative; status can project readiness, but
  must not persist done-ready as a new terminal state.
- Prefer optional additive JSON fields for new status facts when existing fields
  have established meanings.
- Reuse canonical reason codes and recovery summaries where possible.
- Preserve existing delete/repair recovery for genuinely broken active state.
- Use command/view tests in `cmd/` before broad repository verification.
