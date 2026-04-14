package capability

// Skills registered at B4 (change-shape + verification). See
// docs/distillation/catalog.md rows 9, 15, 16, 18, 19, 20, 21.

func multiReviewerCalibration() Skill {
	return Skill{
		ID:                "multi-reviewer-calibration",
		Domain:            DomainReviewQuality,
		Function:          "calibrate multiple reviewers on severity and scope before merging findings",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when more than one reviewer will sign off. Triggers on code-quality-review host or review commands naming multi-reviewer intent.",
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpHost, Value: "code-quality-review",
				Reason: "Code-quality-review host active; calibrate reviewer severity"},
			{Op: OpCommand, Value: "review",
				Reason: "review command invoked; multi-reviewer calibration may apply"},
			{Op: OpUserTextMatches, Values: []string{"second reviewer", "adversarial", "panel review"},
				Reason: "User text signals multiple reviewers"},
		},
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "code-quality-review", Attachment: AttachmentProcedure},
			{Type: BindingCommandManual, Target: "review", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
	}
}

func differentialReview() Skill {
	return Skill{
		ID:                "differential-review",
		Domain:            DomainReviewChangeShape,
		Function:          "review the delta against the baseline rather than the whole file",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when reviewing a diff against a known baseline. Triggers on review command or PR-shaped change context.",
		Evidence:          EvidenceVerdict,
		Triggers: []TriggerClause{
			{Op: OpCommand, Value: "review",
				Reason: "review command invoked; scope reviewer attention to the diff"},
			{Op: OpUserTextMatches, Values: []string{"diff", "pull request", "delta"},
				Reason: "User text names a diff-shaped review"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "review", Attachment: AttachmentProcedure},
			{Type: BindingCommandManual, Target: "review", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
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
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Values: []string{"review", "repair"},
				Reason: "Review or repair command invoked; hunt variants of the fix"},
			{Op: OpUserTextMatches, Values: []string{"variants", "similar bug", "elsewhere"},
				Reason: "User text asks for variant hunting"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "review", Attachment: AttachmentProcedure},
			{Type: BindingCommandManual, Target: "repair", Attachment: AttachmentProcedure},
		},
		ProvenanceRef: "provenance.yaml",
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
		Evidence:          EvidenceVerdict,
		Triggers: []TriggerClause{
			{Op: OpCommand, Value: "validate",
				Reason: "validate command invoked; coverage report applies"},
			{Op: OpHost, Value: "goal-verification",
				Reason: "Verification host active; coverage is a verification input"},
			{Op: OpUserTextMatches, Values: []string{"coverage", "uncovered", "untested"},
				Reason: "User text names coverage"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "validate", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
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
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Value: "validate",
				Reason: "validate command invoked; property tests may apply"},
			{Op: OpHost, Value: "goal-verification",
				Reason: "Verification host active; consider property tests"},
			{Op: OpUserTextMatches, Values: []string{"property test", "invariant", "quickcheck", "hypothesis"},
				Reason: "User text signals property-based testing"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "validate", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
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
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Value: "validate",
				Reason: "validate command invoked; mutation testing may apply"},
			{Op: OpHost, Value: "goal-verification",
				Reason: "Verification host active; mutation testing is a verification booster"},
			{Op: OpUserTextMatches, Values: []string{"mutation testing", "mutmut", "stryker", "pitest"},
				Reason: "User text names a mutation testing tool"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "validate", Attachment: AttachmentToolRecipe},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
	}
}

func performanceProfiling() Skill {
	return Skill{
		ID:                "performance-profiling",
		Domain:            DomainVerification,
		Function:          "profile and attribute performance against a baseline before optimizing",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a change is suspected to affect performance. Triggers on validate or status commands, goal-verification host, or perf-related user text.",
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Values: []string{"validate", "status"},
				Reason: "validate or status command invoked; profiling may apply"},
			{Op: OpHost, Value: "goal-verification",
				Reason: "Verification host active; perf regression may be in scope"},
			{Op: OpUserTextMatches, Values: []string{"perf", "profiling", "slow", "latency", "regression"},
				Reason: "User text signals performance work"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "validate", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
			{Type: BindingCommandManual, Target: "status", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
	}
}
