package capability

// defaultSkills returns the shipped catalog registration list. Foundation
// skills come first, followed by later domain batches.
func defaultSkills() []Skill {
	return []Skill{
		// foundation set
		independentReview(),
		// scale foundation
		contextAssembly(),
		rootCauseTracing(),
		securityReview(),
		specTrace(),
		// security cluster
		threatModeling(),
		sastOrchestration(),
		ghaSecurityReview(),
		supplyChainAudit(),
		// change-shape + verification
		multiReviewerCalibration(),
		variantAnalysis(),
		coverageAnalysis(),
		propertyTesting(),
		mutationTesting(),
		// test-design
		testDesign(),
		// repair/CI + ops
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
		Function:          "workflow-owned S3 fresh-context code review with explicit verdict contract and reviewer-handoff discipline",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a fresh-context S3 code review with an explicit verdict contract is needed. Triggers on the workflow-owned S3 review host or the `slipway review` command surface.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentReportSchema},
		},
		HostCapabilities: []HostCapabilityContract{
			{
				Capability:          "subagent",
				Required:            true,
				FallbackModes:       []string{"manual_independent_review", "same_context_degraded"},
				EvidenceRequirement: "record independent-review evidence from a fresh independent reviewer context",
				Remediation:         "Run independent-review in a host with subagent capability, or explicitly select manual_independent_review / same_context_degraded fallback and record fresh reviewer evidence with context_origin:stage=review=<handle> plus a fallback:<mode> reference when degraded.",
			},
		},
	}
}
