package capability

// defaultSkills returns the shipped catalog registration list. Foundation
// skills come first, followed by later domain batches.
func defaultSkills() []Skill {
	return []Skill{
		// B1 foundation set
		scopeClarification(),
		planAuthoring(),
		tddProof(),
		freshVerificationEvidence(),
		independentReview(),
		// B2 scale foundation
		contextAssembly(),
		parallelExecutorContract(),
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
		differentialReview(),
		variantAnalysis(),
		coverageAnalysis(),
		propertyTesting(),
		mutationTesting(),
		performanceProfiling(),
		// B5 repair/CI + ops
		ciTriage(),
		reviewCommentTriage(),
		gitRecovery(),
		incidentResponse(),
	}
}

func scopeClarification() Skill {
	return Skill{
		ID:                "scope-clarification",
		Domain:            DomainIntake,
		Function:          "converge intent and scope before planning begins",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentPosture,
		Summary:           "Use when user intent or scope is ambiguous before planning. Triggers on intake host, unclear acceptance, or open clarifying questions.",
		Evidence:          EvidenceChecklist,
		Triggers: []TriggerClause{
			{
				Op: OpAllOf,
				Children: []TriggerClause{
					{Op: OpHost, Value: "intake-clarification"},
				},
				Reason: "Intake host active; anchor scope posture before questions",
			},
			{
				Op:     OpUserTextMatches,
				Values: []string{"underspecified", "unclear scope", "ambiguous", "not sure what"},
				Reason: "User text signals underspecified intent",
			},
		},
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "intake-clarification", Attachment: AttachmentPosture},
			{Type: BindingHostEmbedded, Target: "intake-clarification", Attachment: AttachmentChecklist},
			{Type: BindingTechniqueHint, Target: "intake-clarification", Attachment: AttachmentPosture},
		},
		ProvenanceRef: "provenance.yaml",
	}
}

func planAuthoring() Skill {
	return Skill{
		ID:                "plan-authoring",
		Domain:            DomainIntake,
		Function:          "turn requirements into bounded, auditable implementation tasks",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when drafting or auditing a plan bundle. Triggers on plan-audit host or on `plan` / `next` work that requires bounded execution-ready tasks.",
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{
				Op:     OpHost,
				Values: []string{"plan-audit"},
				Reason: "Plan audit host active; enforce bounded-task and execution-contract shape",
			},
			{
				Op:     OpChangedFilesInclude,
				Values: []string{"docs/plans/*.md", "docs/plans/**/*.md"},
				Reason: "Plan bundle touched; plan-authoring procedure applies",
			},
		},
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "plan-audit", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "plan-audit", Attachment: AttachmentChecklist},
			{Type: BindingExportOnly, Target: "using-slipway-catalog", Attachment: AttachmentProcedure},
		},
		ProvenanceRef: "provenance.yaml",
	}
}

func tddProof() Skill {
	return Skill{
		ID:                "tdd-proof",
		Domain:            DomainExecution,
		Function:          "enforce RED-GREEN-REFACTOR and test-first proof for guardrail work",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when executing guardrail-domain work. Triggers on tdd-governance host or on execution covered by a guardrail domain.",
		Evidence:          EvidenceVerdict,
		Triggers: []TriggerClause{
			{
				Op: OpAllOf,
				Children: []TriggerClause{
					{Op: OpHost, Values: []string{"tdd-governance", "wave-orchestration"}},
				},
				Reason: "Execution host active; inject TDD procedure",
			},
			{
				Op:     OpBlockerReason,
				Values: []string{"guardrail_domain_requires_tdd", "missing_red_proof"},
				Reason: "Blocker cites missing TDD proof",
			},
		},
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "tdd-governance", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "wave-orchestration", Attachment: AttachmentProcedure},
			{Type: BindingTechniqueHint, Target: "tdd-governance", Attachment: AttachmentProcedure},
		},
		ProvenanceRef: "provenance.yaml",
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
		Triggers: []TriggerClause{
			{
				Op:     OpHost,
				Values: []string{"goal-verification", "final-closeout", "tdd-governance"},
				Reason: "Verification or closeout host active; enforce fresh-evidence checklist",
			},
			{
				Op:     OpBlockerReason,
				Values: []string{"stale_verification_evidence", "missing_fresh_run"},
				Reason: "Blocker cites stale or missing fresh verification",
			},
		},
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "goal-verification", Attachment: AttachmentReportSchema},
			{Type: BindingHostEmbedded, Target: "final-closeout", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "tdd-governance", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
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
		Triggers: []TriggerClause{
			{
				Op:     OpHost,
				Values: []string{"spec-compliance-review", "code-quality-review"},
				Reason: "Review host active; anchor fresh-context review discipline",
			},
			{
				Op:     OpCommand,
				Value:  "review",
				Reason: "review command invoked; attach independent-review procedure",
			},
		},
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "spec-compliance-review", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "code-quality-review", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "code-quality-review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentReportSchema},
		},
		ProvenanceRef: "provenance.yaml",
	}
}
