package capability

// B4 change-shape + verification skills.

func multiReviewerCalibration() Skill {
	return Skill{
		ID:                "multi-reviewer-calibration",
		Domain:            DomainReviewQuality,
		Function:          "calibrate multiple reviewers on severity and scope before merging findings",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when more than one reviewer will sign off. Triggers on code-quality-review host or review commands naming multi-reviewer intent.",
		Evidence:          EvidenceArtifact,		// Explicit-focus backing for `--focus calibration` on review
		// (resolved via surfaces.go). Host attachment on code-quality-review
		// stays so calibration still rides host-embedded flows without
		// needing a public selector.
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "code-quality-review", Attachment: AttachmentProcedure},
		},
		HydrateReferences: []HydrateReference{
			{Name: "review-dimensions.md", Reason: "Dimensions for reviewer-severity calibration"},
		},
	}
}

func variantAnalysis() Skill {
	return Skill{
		ID:                "variant-analysis",
		Domain:            DomainReviewChangeShape,
		Function:          "hunt additional variants of a known bug across the codebase",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a bug has been fixed in one place and variants elsewhere are plausible. Triggers on review or repair commands or user text asking for similar-bug hunts.",
		Evidence:          EvidenceArtifact,		// Suggested-only on review / repair (§5.2). No public focus selector.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentProcedure},
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentProcedure},
		},
	}
}

func coverageAnalysis() Skill {
	return Skill{
		ID:                "coverage-analysis",
		Domain:            DomainVerification,
		Function:          "evaluate test coverage against the change surface with a reproducible report",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentChecklist,
		Summary:           "Use when a change needs coverage evaluation. Triggers on validate command, goal-verification host, or coverage-related user text.",
		Evidence:          EvidenceVerdict,		// Host-embedded on goal-verification; also suggested on validate so
		// verification flows keep coverage as a recommendation without a
		// public focus selector.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
		},
	}
}

func propertyTesting() Skill {
	return Skill{
		ID:                "property-testing",
		Domain:            DomainVerification,
		Function:          "write property-based tests that specify invariants, not examples",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when invariants are clearer than example cases. Triggers on validate command, goal-verification host, or property-oriented user text.",
		Evidence:          EvidenceArtifact,		// Explicit-focus backing for `--focus property` on validate
		// (resolved via surfaces.go). Host attachment on goal-verification
		// keeps it available from verification flows.
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
		},
		HydrateReferences: []HydrateReference{
			{Name: "design.md", Reason: "How to pick properties that are worth testing"},
			{Name: "generating.md", Reason: "Write generators that exercise the property space"},
			{Name: "strategies.md", Reason: "Core property strategies (idempotence, roundtrip, oracle, invariants)"},
			{Name: "libraries.md", Reason: "Choose an appropriate property-testing library"},
			{Name: "interpreting-failures.md", Reason: "Read shrunk counterexamples and extract real bugs"},
		},
	}
}

func mutationTesting() Skill {
	return Skill{
		ID:                "mutation-testing",
		Domain:            DomainVerification,
		Function:          "run mutation testing to score the test suite, not the implementation",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentToolRecipe,
		Summary:           "Use when test strength is in doubt. Triggers on validate command, goal-verification host, or user text naming mutation testing.",
		Evidence:          EvidenceArtifact,		// Explicit-focus backing for `--focus mutation` on validate.
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
		},
		HydrateReferences: []HydrateReference{
			{Name: "optimization-strategies.md", Reason: "Make mutation runs finish in bounded time"},
			{Name: "configuration.md", Reason: "Pick the right mutators and exclusions for the target"},
		},
	}
}

func performanceProfiling() Skill {
	return Skill{
		ID:                "performance-profiling",
		Domain:            DomainVerification,
		Function:          "profile and attribute performance against a baseline before optimizing",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a change is suspected to affect performance. Triggers on validate command, goal-verification host, or perf-related user text.",
		Evidence:          EvidenceArtifact,		// Suggested-only on validate (§5.2). The status
		// --view=performance-profiling surface was removed (§5.5), and status no
		// longer carries this skill as a suggested surface.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
		},
	}
}
