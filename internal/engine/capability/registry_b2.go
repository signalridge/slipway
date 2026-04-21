package capability

// B2 scale-foundation skills.

func contextAssembly() Skill {
	return Skill{
		ID:                "context-assembly",
		Domain:            DomainIntake,
		Function:          "assemble product, codebase, and risk context before planning or review",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a task needs grounded context before planning or review. Triggers on research or plan-audit hosts, unclear context, or action-scoped hydration cues.",
		Evidence:          EvidenceArtifact,
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "research-orchestration", Attachment: AttachmentProcedure},
			{Type: BindingHostEmbedded, Target: "plan-audit", Attachment: AttachmentPosture},
			{Type: BindingTechniqueHint, Target: "research-orchestration", Attachment: AttachmentProcedure},
		},
		HydrateReferences: []HydrateReference{
			{Name: "codebase-map.md", Reason: "Ground brownfield context before planning"},
		},
	}
}

func rootCauseTracing() Skill {
	return Skill{
		ID:                "root-cause-tracing",
		Domain:            DomainDebugging,
		Function:          "trace root cause before attempting fixes; branch competing hypotheses when traces disagree",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a fix is being considered before the root cause is documented. Triggers on repair, wave-orchestration host, or debugging-centric user text.",
		Evidence:          EvidenceArtifact,
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "wave-orchestration", Attachment: AttachmentProcedure},
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentProcedure},
			{Type: BindingTechniqueHint, Target: "wave-orchestration", Attachment: AttachmentProcedure},
		},
		HydrateReferences: []HydrateReference{
			{Name: "root-cause-tracing.md", Reason: "Trace error origins, named failure patterns, and debugging anti-patterns before proposing fixes"},
			{Name: "condition-based-waiting.md", Reason: "Replace sleep/retry guards with condition-based waits in flaky tests"},
			{Name: "hypothesis-testing.md", Reason: "Structure competing hypotheses, defense layers, and their falsification experiments"},
		},
	}
}

func securityReview() Skill {
	return Skill{
		ID:                "security-review",
		Domain:            DomainReviewSecurity,
		Function:          "secure-default and framework-aware security review",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentChecklist,
		Summary:           "Use when reviewing security-relevant code. Triggers on review command, security-classified guardrail, or changes to auth/crypto/input paths.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "spec-compliance-review", Attachment: AttachmentChecklist},
			{Type: BindingHostEmbedded, Target: "code-quality-review", Attachment: AttachmentChecklist},
		},
		HydrateReferences: []HydrateReference{
			{Name: "authentication.md", Reason: "Password storage / session / MFA / recovery secure-default rules"},
			{Name: "authorization.md", Reason: "Resource-boundary re-check, IDOR, multi-tenant isolation"},
			{Name: "injection.md", Reason: "Per-sink parameterization, deserialization-as-injection"},
			{Name: "xss.md", Reason: "Context-aware encoding and framework escape-hatch review cues"},
			{Name: "ssrf.md", Reason: "Fetcher allow/deny-list, metadata endpoints, DNS rebinding"},
			{Name: "infrastructure-docker.md", Reason: "Container hardening, K8s securityContext, image supply chain"},
		},
	}
}

func specTrace() Skill {
	return Skill{
		ID:                "spec-trace",
		Domain:            DomainReviewChangeShape,
		Function:          "bidirectional spec-to-code and code-to-spec trace review",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentChecklist,
		Summary:           "Use when verifying that implementation mirrors the approved plan. Triggers on spec-compliance host or validate/review commands.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "spec-compliance-review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentReportSchema},
		},
	}
}
