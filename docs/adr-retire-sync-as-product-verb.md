# ADR: Retire `sync` And `validate-requirements` As Product Verbs

The old `sync` surface had a hard contract split: runtime behavior was already
read-only validation of the active change's `requirements.md`, while generated
adapter metadata still described a merge-style mutation. We first retired
`sync` as a product verb, but exposing the checker as `validate-requirements`
left the public surface with two similarly named validation commands that
answered different questions.

Slipway now retires both legacy verbs as public product surfaces. The
requirements checker remains in the runtime, but it is no longer a standalone
top-level command. Instead, `slipway validate` owns the public read-only
readiness surface and includes a nested `requirements_contract` summary when
the governed bundle can be evaluated cleanly.

This keeps Cobra help, toolgen registry metadata, generated command prompt
surfaces, and stable docs aligned with the code that actually ships, while
avoiding a second near-duplicate validation verb. If Slipway ever needs a true
merge/apply command later, it should be added as a new explicit mutation
surface rather than reviving either retired verb.
