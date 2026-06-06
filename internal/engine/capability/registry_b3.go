package capability

// B3 security-cluster skills.

func threatModeling() Skill {
	return Skill{
		ID:                "threat-modeling",
		Domain:            DomainReviewSecurity,
		Function:          "structured threat enumeration with ownership map and mitigation trace",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when a change alters the trust boundary or asset surface. Triggers on review or validate commands, security-classified guardrails, or user text naming threats.",
		Evidence:          EvidenceArtifact,
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentProcedure},
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentProcedure},
			{Type: BindingExportOnly, Target: "skill-index", Attachment: AttachmentReportSchema},
		},
	}
}

func sastOrchestration() Skill {
	return Skill{
		ID:                "sast-orchestration",
		Domain:            DomainReviewSecurity,
		Function:          "run SAST tooling (CodeQL/Semgrep) with SARIF triage",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentToolRecipe,
		Summary:           "Use when running SAST tooling against the change. Triggers on review or validate commands and user text naming SAST tools.",
		Evidence:          EvidenceArtifact, // Explicit-focus backing for `--focus sast` on review /
		// validate (resolved via surfaces.go). repair is intentionally excluded:
		// it performs bounded local-state integrity only and never runs scanners.
		// Command-auto bindings also let the skill appear in suggested_capabilities[]
		// when SAST triggers fire.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentToolRecipe},
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentToolRecipe},
		},
		HydrateReferences: []HydrateReference{
			{Name: "codeql-ruleset-catalog.md", Reason: "Pick a CodeQL query pack, threat model, language caveat, and build-fix path"},
			{Name: "semgrep-scan-modes.md", Reason: "Full / diff / supply-chain scan-mode and ruleset-family selection"},
			{Name: "sarif-merge.md", Reason: "Deterministic multi-tool SARIF merge contract"},
			{Name: "sarif-jq-queries.md", Reason: "Ad-hoc triage queries over SARIF output"},
		},
	}
}

func ghaSecurityReview() Skill {
	return Skill{
		ID:                "gha-security-review",
		Domain:            DomainReviewSecurity,
		Function:          "review GitHub Actions workflows for privilege, pinning, and agentic-action risk",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentChecklist,
		Summary:           "Use when reviewing GitHub Actions workflows. Triggers on review or repair commands or on changes to .github/workflows paths.",
		Evidence:          EvidenceVerdict, // Suggested-only on review / repair (§5.2). Command-auto feeds the
		// suggested_capabilities[] channel when workflow paths change.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentToolRecipe},
		},
		HydrateReferences: []HydrateReference{
			{Name: "pwn-request.md", Reason: "pull_request_target fork-code execution vector"},
			{Name: "comment-triggered-commands.md", Reason: "comment-triggered workflow command injection"},
			{Name: "expression-injection.md", Reason: "github.event.* interpolation injection into run blocks"},
			{Name: "permissions-and-secrets.md", Reason: "least-privilege permissions and secret scoping rules"},
		},
	}
}

func supplyChainAudit() Skill {
	return Skill{
		ID:                "supply-chain-audit",
		Domain:            DomainReviewSecurity,
		Function:          "audit third-party dependencies for CVE, provenance, and pinning risk",
		Tier:              TierT2,
		PrimaryAttachment: AttachmentChecklist,
		Summary:           "Use when dependency manifests or lockfiles change. Triggers on review or repair commands or on changes to package/lock files.",
		Evidence:          EvidenceVerdict, // Suggested-only on review / repair (§5.2). The status
		// --view=supply-chain-audit surface was removed (§5.5), and status no
		// longer carries this skill as a suggested surface.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentToolRecipe},
		},
		HydrateReferences: []HydrateReference{
			{Name: "results-template.md", Reason: "Audit report schema for supply-chain findings"},
			{Name: "vulnerability-assessment-guide.md", Reason: "CVE triage and severity assignment under time pressure"},
		},
	}
}
