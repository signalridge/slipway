# Concerns

Re-authored for change
`resolve-github-issue-151-thin-host-disk-handoff-return-contr`
(GitHub issue #151).

- Load-bearing invariant: Slipway CLI owns evidence freshness, run versions,
  timestamps, and pass/fail stamping. A subagent's return line must remain a
  claim until the host records or validates evidence through supported Slipway
  paths.
- Context risk: updating only one host would satisfy the issue's minimum
  acceptance but leave the named remaining heavy stages inconsistent. The plan
  should cover all five named hosts while keeping implementation at the template
  contract layer.
- Overengineering risk: adding a new runtime evidence-ingest subsystem would
  widen the change beyond the issue's acceptance signal. Prefer explicit host
  contracts and tests unless implementation proves a runtime gap is necessary.
- Test risk: phrase-only tests can pass while the fail-closed boundary regresses.
  Regression coverage should assert the three-part contract: disk artifact
  handoff, short confirmation, and CLI-owned stamping/freshness.
