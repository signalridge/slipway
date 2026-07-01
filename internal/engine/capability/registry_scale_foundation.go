package capability

// scale-foundation skills.

func contextAssembly() Skill {
	return Skill{
		ID:                "context-assembly",
		Domain:            DomainIntake,
		Function:          "assemble product, codebase, and risk context before planning or review",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a task needs grounded context before planning or review. Triggers on the research or plan-audit hosts, unclear context, or a user asking how something works or for background.",
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
		Summary:           "Use when a fix is being considered before the root cause is documented. Triggers on fix, wave-orchestration host, or debugging-centric user text.",
		Evidence:          EvidenceArtifact,
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "wave-orchestration", Attachment: AttachmentProcedure},
			{Type: BindingCommandAuto, Target: "fix", Attachment: AttachmentProcedure},
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
		Function:          "workflow-owned S3 secure-default, boundary- and framework-aware security review",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentChecklist,
		Summary:           "Use when running the S3 security review for auth/authz, injection, secrets, SSRF, and insecure defaults. Triggers on the workflow-owned S3 review host, the `slipway review` command, a security-classified guardrail, or security-review control selected by blast-radius policy.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentChecklist},
		},
		HostCapabilities: []HostCapabilityContract{
			{
				Capability:          "subagent",
				Required:            true,
				FallbackModes:       []string{"manual_security_review", "same_context_degraded"},
				EvidenceRequirement: "record security-review evidence from a fresh independent reviewer context",
				Remediation:         "Run security-review in a host with subagent capability, or explicitly select manual_security_review / same_context_degraded fallback and record fresh reviewer evidence with context_origin:stage=review=<handle> plus a fallback:<mode> reference when degraded.",
			},
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
		Summary:           "Use when tracing the approved plan and code in both directions (plan line to code, diff hunk back to plan line) to catch drift. Triggers on the spec-compliance-review stage, `slipway review` (auto-attached), or `slipway validate --focus spec-trace`.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingHostEmbedded, Target: "spec-compliance-review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentReportSchema},
		},
	}
}
