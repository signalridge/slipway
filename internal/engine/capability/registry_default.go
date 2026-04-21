package capability

// defaultSkills returns the shipped catalog registration list. Foundation
// skills come first, followed by later domain batches.
func defaultSkills() []Skill {
	return []Skill{
		// B1 foundation set
		freshVerificationEvidence(),
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

func freshVerificationEvidence() Skill {
	return Skill{
		ID:                "fresh-verification-evidence",
		Domain:            DomainExecution,
		Function:          "block completion claims without fresh commands and fresh proof",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentChecklist,
		Summary:           "Use when a change is approaching a verify/closeout gate. Triggers on goal-verification, final-closeout, or any completion-adjacent step.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentReportSchema},
			{Type: BindingHostEmbedded, Target: "final-closeout", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "tdd-governance", Attachment: AttachmentChecklist},
		},
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
