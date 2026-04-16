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
		Triggers: []TriggerClause{
			{Op: OpCommand, Values: []string{"review", "validate"},
				Reason: "review or validate command invoked; enumerate threats against the change"},
			{Op: OpUserTextMatches, Values: []string{"threat model", "attack surface", "adversary", "trust boundary"},
				Reason: "User text asks for threat modeling"},
		},
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentProcedure},
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentProcedure},
			{Type: BindingExportOnly, Target: "using-slipway-catalog", Attachment: AttachmentReportSchema},
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
		Summary:           "Use when running SAST tooling against the change. Triggers on review, validate, or repair commands and user text naming SAST tools.",
		Evidence:          EvidenceArtifact,
		Triggers: []TriggerClause{
			{Op: OpCommand, Values: []string{"review", "validate", "repair"},
				Reason: "Review/validate/repair command invoked; SAST may apply"},
			{Op: OpUserTextMatches, Values: []string{"codeql", "semgrep", "sast", "sarif"},
				Reason: "User text names a SAST tool"},
		},
		// Explicit-focus backing for `--focus sast` on review / validate /
		// repair (resolved via surfaces.go). Command-auto bindings also let
		// the skill appear in suggested_capabilities[] when SAST triggers fire.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentToolRecipe},
			{Type: BindingCommandAuto, Target: "validate", Attachment: AttachmentToolRecipe},
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentToolRecipe},
		},
		HydrateReferences: []HydrateReference{
			{Name: "codeql-ruleset-catalog.md", Reason: "Pick a CodeQL query pack by threat model and language"},
			{Name: "codeql-language-details.md", Reason: "Language-specific CodeQL build and analysis caveats"},
			{Name: "codeql-threat-models.md", Reason: "Threat-model selection for CodeQL run scoping"},
			{Name: "codeql-performance-tuning.md", Reason: "Scan-time / memory knobs for large repos"},
			{Name: "codeql-build-fixes.md", Reason: "Common build failures that block the CodeQL database"},
			{Name: "semgrep-rulesets.md", Reason: "Semgrep ruleset selection and risk coverage"},
			{Name: "semgrep-scan-modes.md", Reason: "Full / diff / supply-chain scan-mode selection"},
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
		Evidence:          EvidenceVerdict,
		Triggers: []TriggerClause{
			{Op: OpChangedFilesInclude, Values: []string{".github/workflows/*", ".github/workflows/**/*"},
				Reason: "GitHub Actions workflow changed"},
			{Op: OpCommand, Values: []string{"review", "repair"},
				Reason: "Review or repair command invoked; workflow surface may be in scope"},
		},
		// Suggested-only on review / repair (§5.2). Command-auto feeds the
		// suggested_capabilities[] channel when workflow paths change.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentToolRecipe},
		},
		HydrateReferences: []HydrateReference{
			{Name: "pwn-request.md", Reason: "pull_request_target fork-code execution vector"},
			{Name: "comment-triggered-commands.md", Reason: "comment-triggered workflow command injection"},
			{Name: "expression-injection.md", Reason: "github.event.* interpolation injection into run blocks"},
			{Name: "ai-prompt-injection-via-ci.md", Reason: "agentic action prompt-injection via CI-provided input"},
			{Name: "credential-escalation.md", Reason: "GITHUB_TOKEN and secret scope escalation paths"},
			{Name: "permissions-and-secrets.md", Reason: "least-privilege permissions and secret scoping rules"},
			{Name: "runner-infrastructure.md", Reason: "self-hosted runner isolation and ephemerality gates"},
			{Name: "supply-chain.md", Reason: "action pinning, reusable-workflow, and dependency surface"},
			{Name: "real-world-attacks.md", Reason: "awesome-go / trivy exploit case studies and detection signals"},
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
		Evidence:          EvidenceVerdict,
		Triggers: []TriggerClause{
			{Op: OpChangedFilesInclude, Values: []string{
				"go.mod", "go.sum",
				"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
				"Cargo.toml", "Cargo.lock",
				"requirements*.txt", "pyproject.toml", "poetry.lock", "uv.lock",
			},
				Reason: "Dependency manifest or lockfile changed"},
			{Op: OpCommand, Values: []string{"review", "repair"},
				Reason: "Review or repair command invoked; dependency surface may apply"},
		},
		// Suggested-only on review / repair (§5.2). The status
		// --view=supply-chain-audit surface was removed (§5.5), and status no
		// longer carries this skill as a suggested surface.
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentChecklist},
			{Type: BindingCommandAuto, Target: "repair", Attachment: AttachmentToolRecipe},
		},
		HydrateReferences: []HydrateReference{
			{Name: "results-template.md", Reason: "Audit report schema for supply-chain findings"},
			{Name: "dependency-management-best-practices.md", Reason: "Pinning, review cadence, and lockfile discipline"},
			{Name: "vulnerability-assessment-guide.md", Reason: "CVE triage and severity assignment under time pressure"},
			{Name: "license-compatibility-matrix.md", Reason: "License compatibility rules per distribution target"},
		},
	}
}
