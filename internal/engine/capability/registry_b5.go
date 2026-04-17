package capability

// B5 repair/CI + ops skills.

func ciTriage() Skill {
	return Skill{
		ID:                "ci-triage",
		Domain:            DomainRepairCI,
		Function:          "triage failing CI runs to root cause before retrying",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when CI is failing and a retry is being considered. Triggers on repair command or user text naming CI failures.",
		Evidence:          EvidenceArtifact,		// Suggested-only on repair (§5.2). No public explicit selector.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentProcedure},
		},
	}
}

func reviewCommentTriage() Skill {
	return Skill{
		ID:                "review-comment-triage",
		Domain:            DomainRepairCI,
		Function:          "triage reviewer comments into accept, push-back, or defer with a written disposition",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when addressing reviewer comments on an open PR. Triggers on repair command or user text naming PR review comments.",
		Evidence:          EvidenceArtifact,		// Suggested-only on repair (§5.2). No public explicit selector.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentProcedure},
		},
	}
}

func gitRecovery() Skill {
	return Skill{
		ID:                "git-recovery",
		Domain:            DomainRepairCI,
		Function:          "recover git state without destroying unsaved work or bypassing hooks",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when git state is entangled and a destructive operation is being considered. Triggers on repair command or user text naming git recovery.",
		Evidence:          EvidenceArtifact,		// Suggested-only on repair (§5.2). Host-embedded attachment
		// on worktree-preflight remains so preflight flows still route recovery.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "worktree-preflight", Attachment: AttachmentProcedure},
		},
	}
}

func incidentResponse() Skill {
	return Skill{
		ID:                "incident-response",
		Domain:            DomainOpsDiagnostics,
		Function:          "incident-response posture: contain, diagnose, communicate, and write up",
		Tier:              TierT3,
		PrimaryAttachment: AttachmentReportSchema,
		Summary:           "Use when a production incident is suspected or active. Triggers on status or health commands or user text naming an incident.",
		Evidence:          EvidenceArtifact,		Bindings: []Binding{
			{Type: BindingCommandView, Target: "status", Attachment: AttachmentReportSchema},
			{Type: BindingCommandView, Target: "health", Attachment: AttachmentReportSchema},
			{Type: BindingExportOnly, Target: "using-slipway-catalog", Attachment: AttachmentReportSchema},
		},
		HydrateReferences: []HydrateReference{
			{Name: "incident-response-framework.md", Reason: "Core roles, phase gates, decision authority"},
			{Name: "incident-severity-matrix.md", Reason: "SEV1-4 triage criteria and escalation thresholds"},
			{Name: "communication-templates.md", Reason: "Status-page and stakeholder message templates"},
			{Name: "sla-management-guide.md", Reason: "SLA clock rules, breach thresholds, credit calculation"},
			{Name: "rca-frameworks-guide.md", Reason: "Postmortem frameworks and action-item authoring"},
			{Name: "regulatory-deadlines.md", Reason: "GDPR/HIPAA/PCI notification windows and wording"},
		},
	}
}
