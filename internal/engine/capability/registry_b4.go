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
		Evidence:          EvidenceArtifact, // Explicit-focus backing for `--focus calibration` on review
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
		Evidence:          EvidenceArtifact, // Suggested-only on review / repair (§5.2). No public focus selector.
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
		Evidence:          EvidenceVerdict, // Host-embedded on goal-verification; also suggested on validate so
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
		Summary:           "Use when invariants are clearer than example cases. Triggers on validate command or property-oriented user text.",
		Evidence:          EvidenceArtifact, // Explicit-focus backing for `--focus property` on validate
		// (resolved via surfaces.go). It remains validate-only until a different
		// routed discoverability path is intentionally added.
		Bindings: nil,
		HydrateReferences: []HydrateReference{
			{Name: "design.md", Reason: "How to pick properties, strategy families, and review heuristics that are worth testing"},
			{Name: "generating.md", Reason: "Write generators, edge bias, and library choices that exercise the property space"},
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
		Summary:           "Use when test strength is in doubt. Triggers on validate command or user text naming mutation testing.",
		Evidence:          EvidenceArtifact, // Explicit-focus backing for `--focus mutation` on validate.
		Bindings:          nil,
		HydrateReferences: []HydrateReference{
			{Name: "optimization-strategies.md", Reason: "Make mutation runs finish in bounded time"},
			{Name: "configuration.md", Reason: "Pick the right mutators and exclusions for the target"},
		},
	}
}
