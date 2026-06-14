# Concerns

Re-authored for change
`resolve-github-issue-155-knuth-invariant-overwrite-only-own`
(GitHub issue #155).

- False-negative freshness risk: if the classifier treats human prose as
  scaffold/default, stale plan or research evidence may be accepted. The
  implementation must include unknown non-empty prose in the digest.
- False-positive reopen risk: if authoring comments or empty scaffold-only
  sections stay in the digest, engine-owned scaffold refreshes can reopen
  downstream stages without a material change.
- Contract drift risk: template-derived defaults and hand-maintained phrase
  lists can diverge. Prefer deriving scaffold defaults from embedded templates
  where possible, and keep any literal known defaults narrow and tested.
- Artifact-stage risk: `intent.md` is engine-scaffolded at creation, while
  `requirements.md`, `research.md`, `decision.md`, `tasks.md`, and
  `assurance.md` are skill-authored/deferred. A one-size-fits-all overwrite
  rule would be too broad.
- Recovery risk: stale recovery removes verification records from the target
  stage onward. Digest materiality changes must be covered before relying on
  `stale_evidence_recovery.go` to reopen correctly.
