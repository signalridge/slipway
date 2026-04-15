package capability

// Skills registered at B5 (repair/CI + ops). See docs/distillation/catalog.md
// rows 22, 23, 24, 25.

func ciTriage() Skill {
	return Skill{
		ID:                "ci-triage",
		Domain:            DomainRepairCI,
		Function:          "triage failing CI runs to root cause before retrying",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when CI is failing and a retry is being considered. Triggers on repair or status commands, or user text naming CI failures.",
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Values: []string{"repair", "status"},
				Reason: "repair or status command invoked; CI failures may be in scope"},
			{Op: OpUserTextMatches, Values: []string{"ci failing", "ci broken", "build failing", "pipeline failing"},
				Reason: "User text names a CI failure"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "repair", Attachment: AttachmentProcedure},
			{Type: BindingCommandManual, Target: "status", Attachment: AttachmentChecklist},
		},
		ProvenanceRef: "provenance.yaml",
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
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Value: "repair",
				Reason: "repair command invoked; reviewer comments may be in scope"},
			{Op: OpUserTextMatches, Values: []string{"review comment", "pr comment", "address comment"},
				Reason: "User text names PR review comments"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "repair", Attachment: AttachmentProcedure},
		},
		ProvenanceRef: "provenance.yaml",
	}
}

func gitRecovery() Skill {
	return Skill{
		ID:                "git-recovery",
		Domain:            DomainRepairCI,
		Function:          "recover git state without destroying unsaved work or bypassing hooks",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when git state is entangled and a destructive operation is being considered. Triggers on repair or status commands or user text naming git recovery.",
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Values: []string{"repair", "status"},
				Reason: "repair or status command invoked; git recovery may be in scope"},
			{Op: OpBlockerReason, Values: []string{"worktree_dirty", "branch_diverged", "detached_head"},
				Reason: "Blocker cites an entangled git state"},
			{Op: OpUserTextMatches, Values: []string{"git reset", "git rebase", "--no-verify", "force push", "detached head"},
				Reason: "User text names a destructive or high-risk git operation"},
		},
		Bindings: []Binding{
			{Type: BindingCommandManual, Target: "repair", Attachment: AttachmentProcedure},
			{Type: BindingCommandManual, Target: "status", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "worktree-preflight", Attachment: AttachmentProcedure},
		},
		ProvenanceRef: "provenance.yaml",
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
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Values: []string{"status", "health"},
				Reason: "status or health command invoked; incident may be in scope"},
			{Op: OpUserTextMatches, Values: []string{"incident", "outage", "page", "sev1", "sev2"},
				Reason: "User text names an incident"},
		},
		Bindings: []Binding{
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
		ProvenanceRef: "provenance.yaml",
	}
}
