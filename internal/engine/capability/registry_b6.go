package capability

// B6 test-design skills.

func testDesign() Skill {
	return Skill{
		ID:                "test-design",
		Domain:            DomainVerification,
		Function:          "language-agnostic test design, case enumeration, and test-double judgment",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when designing meaningful test cases, test doubles, properties, or fixtures. Triggers on wave-orchestration host or testing-quality user text.",
		Evidence:          EvidenceArtifact,
		Bindings: []Binding{
			{Type: BindingTechniqueHint, Target: "wave-orchestration", Attachment: AttachmentProcedure},
		},
		HydrateReferences: []HydrateReference{
			{Name: "test-doubles.md", Reason: "Choose real dependencies, fakes, spies, stubs, mocks, and injected time or IO per boundary"},
			{Name: "behavior-vs-implementation.md", Reason: "Assert observable behavior and reject tautologies or internal-call coupling"},
			{Name: "case-enumeration.md", Reason: "Derive equivalence, boundary, decision-table, state, pairwise, negative, and MC/DC cases with oracles"},
			{Name: "property-reasoning.md", Reason: "Frame invariants, generators, shrinking, and stateful properties without weak assertions"},
			{Name: "test-data.md", Reason: "Select fixtures, factories, builders, and deterministic non-sensitive datasets"},
		},
	}
}
