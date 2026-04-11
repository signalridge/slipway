# ADR: Retire `sync` As A Product Verb

The old `sync` surface had a hard contract split: runtime behavior was already
read-only validation of the active change's `requirements.md`, while generated
adapter metadata still described a merge-style mutation. We resolved that split
by retiring `sync` as a product verb and making the real contract explicit as
`validate-requirements`: a read-only validation command with no merge side
effects. This keeps command behavior, Cobra help, toolgen registry metadata,
generated adapter skills, and user docs aligned with the code that actually
ships. If Slipway ever needs a true merge/apply verb later, it should be added
as a new command with its own explicit mutation contract instead of overloading
`validate-requirements`.
