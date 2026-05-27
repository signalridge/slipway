package capability

// defaultSkills returns the shipped catalog registration list. Foundation
// skills come first, followed by later domain batches.
func defaultSkills() []Skill {
	return []Skill{
		// B1 foundation set
		independentReview(),
		// B2 scale foundation
		contextAssembly(),
		rootCauseTracing(),
		securityReview(),
		specTrace(),
		// B3 security cluster
		threatModeling(),
		sastOrchestration(),
		ghaSecurityReview(),
		supplyChainAudit(),
		// B4 change-shape + verification
		multiReviewerCalibration(),
		variantAnalysis(),
		coverageAnalysis(),
		propertyTesting(),
		mutationTesting(),
		// B5 repair/CI + ops
		ciTriage(),
		reviewCommentTriage(),
		gitRecovery(),
		incidentResponse(),
	}
}

func independentReview() Skill {
	return Skill{
		ID:                "independent-review",
		Domain:            DomainReviewQuality,
		Function:          "fresh-context code review with explicit verdict contract and reviewer-handoff discipline",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when performing code review with a verdict contract. Triggers on review host or the `review` command surface.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "spec-compliance-review", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "code-quality-review", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "code-quality-review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentReportSchema},
		},
	}
}
