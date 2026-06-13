# Concerns

Re-authored for change
`resolve-github-issues-195-and-196-make-status-expose-done-re`
(GitHub issues #195 and #196).

- Lifecycle ambiguity risk: status must make done-ready obvious without saying
  the change is already done. `change.status` stays active until finalization.
- False-corruption risk: an archived change with no active bundle must not be
  reported as `change_state_load_failed` or remediated with generic repair.
- Real-corruption risk: if neither active nor archived authority can be loaded,
  existing delete/repair diagnostics should still surface.
- Compatibility risk: adding optional JSON fields is lower risk than changing
  existing `lifecycle_status` semantics from `active` to `done_ready`.
- Text/JSON drift risk: if only JSON is changed, operators using text status
  still see the ambiguous handoff. Text rendering should show the same
  done-ready/archived facts.
