package capability

// B5 repair/CI + ops skills.

func ciTriage() Skill {
	return Skill{
		ID:                "ci-triage",
		Domain:            DomainRepairCI,
		Function:          "triage failing CI runs to root cause before retrying",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when CI, a build, or a pipeline is failing and a retry is being considered. Triggers on the `slipway fix` command or user text naming CI/build/pipeline failures.",
		Evidence:          EvidenceArtifact, // Suggested-only on fix (§5.2). No public explicit selector.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "fix", Attachment: AttachmentProcedure},
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
		Summary:           "Use when addressing reviewer comments on an open PR. Triggers on fix command or user text naming PR review comments.",
		Evidence:          EvidenceArtifact, // Suggested-only on fix (§5.2). No public explicit selector.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "fix", Attachment: AttachmentProcedure},
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
		Summary:           "Use when git state is entangled and a destructive operation is being considered (git reset --hard, rebase, force-push, --no-verify, detached HEAD). Triggers on destructive-operation user text.",
		Evidence:          EvidenceArtifact, // Host-embedded attachment on worktree-preflight remains so preflight flows still route recovery.
		Bindings: []Binding{
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
		Summary:           "Use when a production incident is suspected or active. Triggers on `slipway status --focus incident` / `slipway health --focus incident` or user text naming an incident.",
		// Public status/health exposure is owned by surfaces.go, not catalog bindings.
		Evidence: EvidenceArtifact,
		Bindings: []Binding{
			{Type: BindingExportOnly, Target: "skill-index", Attachment: AttachmentReportSchema},
		},
		HydrateReferences: []HydrateReference{
			{Name: "incident-response-framework.md", Reason: "Core roles, phase gates, decision authority"},
			{Name: "incident-severity-matrix.md", Reason: "SEV1-4 triage criteria and escalation thresholds"},
		},
	}
}
