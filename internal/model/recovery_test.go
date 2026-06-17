package model

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBlockerSegments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		spec    string
		code    string
		subject string
		detail  string
		raw     string
	}{
		{
			name:    "three segment stale token",
			spec:    "required_skill_stale:plan-audit:assurance.md",
			code:    "required_skill_stale",
			subject: "plan-audit",
			detail:  "assurance.md",
			raw:     "required_skill_stale:plan-audit:assurance.md",
		},
		{
			name:    "two segment task token",
			spec:    "tasks_plan_changed_since_task_evidence:t-03",
			code:    "tasks_plan_changed_since_task_evidence",
			subject: "t-03",
			detail:  "",
			raw:     "tasks_plan_changed_since_task_evidence:t-03",
		},
		{
			name:    "bare token",
			spec:    "plan_audit_failed",
			code:    "plan_audit_failed",
			subject: "",
			detail:  "",
			raw:     "plan_audit_failed",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ParseBlocker(ReasonCodeFromSpec(tc.spec))
			assert.Equal(t, tc.code, got.Code)
			assert.Equal(t, tc.subject, got.Subject)
			assert.Equal(t, tc.detail, got.Detail)
			assert.Equal(t, tc.raw, got.Raw)
		})
	}
}

func TestParseBlockerIsSingleDecompositionPoint(t *testing.T) {
	t.Parallel()

	// ParseBlocker must accept an already-split ReasonCode and re-derive the same
	// segments, so callers can route either a spec string or a ReasonCode through
	// the one parser.
	rc := NewReasonCode("required_skill_stale", "plan-audit:assurance.md")
	got := ParseBlocker(rc)
	assert.Equal(t, "plan-audit", got.Subject)
	assert.Equal(t, "assurance.md", got.Detail)
}

func TestRemediationTableEntriesAreComplete(t *testing.T) {
	t.Parallel()

	for code, rem := range blockerRemediations {
		_, hasCanonical := canonicalReasonDefinitions[code]
		assert.Truef(t, hasCanonical, "recovery code %q must have a canonical reason message", code)
		assert.NotEmptyf(t, strings.TrimSpace(rem.Remediation), "remediation for %q must be non-empty", code)
		assert.NotEmptyf(t, strings.TrimSpace(string(rem.Class)), "recovery class for %q must be non-empty", code)
		assert.Lessf(t, recoveryClassRank(rem.Class), len(recoveryClassPriority),
			"remediation class %q for %q must be in recoveryClassPriority", rem.Class, code)
	}
}

func TestRecoveryRelevantTokensResolveToRemediation(t *testing.T) {
	t.Parallel()

	// Every recovery-relevant family named in the requirements must render a
	// non-empty remediation. This is derived from canonical reason codes so adding
	// a new scope_contract_*/plan_audit_*/wave_* code without remediation goes red.
	for _, code := range recoveryRelevantCanonicalCodes() {
		rc := NewReasonCode(code, sampleRecoveryDetail(code))
		step, ok := recoveryStepFor(rc)
		require.Truef(t, ok, "token %q must produce a recovery step", rc.Key())
		assert.NotEmptyf(t, strings.TrimSpace(step.Remediation), "token %q must produce a remediation", rc.Key())
		assert.NotEmptyf(t, strings.TrimSpace(step.Command), "token %q must produce a command", rc.Key())
		assert.NotContainsf(t, step.Remediation, "{", "token %q remediation must not leak a placeholder", rc.Key())
		assert.NotContainsf(t, step.Command, "{", "token %q command must not leak a placeholder", rc.Key())
	}
}

func TestInScopeProducedBlockersResolveToCanonicalRecovery(t *testing.T) {
	t.Parallel()

	// This list is intentionally derived from known validate/next/run/done
	// producers, not from canonicalReasonDefinitions. It catches a real blocker
	// that can reach an in-scope user surface before it is added to the canonical
	// reason and remediation tables.
	for _, spec := range inScopeProducedRecoverySpecs() {
		producedCode, _, _ := strings.Cut(spec, ":")
		producedCode = normalizeReasonCode(producedCode)
		definition, hasCanonical := canonicalReasonDefinitions[producedCode]
		require.Truef(t, hasCanonical, "produced blocker %q must have a canonical reason message", producedCode)

		rc := ReasonCodeFromSpec(spec)
		require.Equalf(t, producedCode, rc.Code, "produced blocker %q must not collapse to %q", spec, unknownReasonCode)

		step, ok := recoveryStepFor(rc)
		require.Truef(t, ok, "produced blocker %q must produce a recovery step", rc.Key())
		assert.NotEmptyf(t, strings.TrimSpace(step.Remediation), "produced blocker %q must produce a remediation", rc.Key())
		assert.NotEmptyf(t, strings.TrimSpace(step.Command), "produced blocker %q must produce a command", rc.Key())
		assert.NotContainsf(t, step.Remediation, "{", "produced blocker %q remediation must not leak a placeholder", rc.Key())
		assert.NotContainsf(t, step.Command, "{", "produced blocker %q command must not leak a placeholder", rc.Key())
		assert.NotEqualf(t, testHumanizeReasonCode(rc.Code), definition.Message,
			"produced blocker %q must not render through humanize fallthrough", rc.Key())
	}
}

func inScopeProducedRecoverySpecs() []string {
	return []string{
		"research_structure_invalid:section \"Findings\" must have non-empty content",
		"research_section_placeholder:## Alternatives Considered",
		"decision_structure_invalid:missing required heading \"## Selected Approach\"",
		"decision_section_placeholder:## Selected Approach",
		"decision_status_rejected:superseded",
		"decision_status_unknown:retired ish",
		"decision_contract_path_invalid:permission denied",
		"decision_contract_unreadable",
		"assurance_contract_missing",
		"assurance_contract_path_invalid:permission denied",
		"assurance_contract_unreadable",
		"assurance_section_placeholder:## Scope Summary",
		"non_pass_task:t-01",
		"incomplete_execution_task:t-19",
		"high_risk_check_missing:external_api_contracts.safety_baseline",
		"high_risk_check_failed:external_api_contracts.safety_baseline",
		"closeout_goal_verification_reuse_invalid:goal-verification evidence was produced before final-closeout input changed; rerun goal-verification, then rerun final-closeout",
		"manifest_r0_invalid:manifest_missing",
		"manifest_r0_invalid:manifest_parse_invalid",
		"manifest_r0_invalid:manifest_slug_mismatch",
		"manifest_r0_invalid:manifest_base_ref_missing",
		"sensitive_evidence_missing:schema_migration:db/migrations/001_create_users.sql",
		"worktree_metadata_persist_failed:permission denied",
		"closeout_reviewer_independence_missing",
		"closeout_chain_order_invalid",
		"context_origin_handle_invalid",
		"cross_stage_context_not_distinct:spec-compliance-review|code-quality-review",
		"plan_audit_origin_invalid",
		"wave_test_impl_not_distinct",
		"degraded_dispatch_justification_missing",
	}
}

func TestReviewSetRecoveryTextNamesSelectedSetAndOptionalSecurity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec string
		want []string
	}{
		{
			name: "missing selected security review",
			spec: "required_skill_missing:security-review",
			want: []string{
				"selected review-set",
				"security-review only when selected",
			},
		},
		{
			name: "closeout chain order selected review set",
			spec: "closeout_chain_order_invalid",
			want: []string{
				"selected review-set",
				"independent-review",
				"security-review when the security control selected it",
				"precedes goal-verification",
				"precedes final-closeout",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step, ok := recoveryStepFor(ReasonCodeFromSpec(tt.spec))
			require.True(t, ok)
			for _, want := range tt.want {
				assert.Contains(t, step.Remediation, want)
			}
			assert.NotContains(t, step.Remediation, "{")
		})
	}
}

func recoveryRelevantCanonicalCodes() []string {
	exact := map[string]bool{
		"archive_failed":                           true,
		"assurance_structure_invalid":              true,
		"assurance_contract_missing":               true,
		"assurance_contract_path_invalid":          true,
		"assurance_contract_unreadable":            true,
		"assurance_section_placeholder":            true,
		"artifact_not_ready":                       true,
		"artifact_reconcile_failed":                true,
		"artifact_schema_missing":                  true,
		"artifact_validation_failed":               true,
		"change_not_active":                        true,
		"closeout_assurance_attestation_missing":   true,
		"closeout_goal_verification_reuse_invalid": true,
		"dedicated_worktree_branch_mismatch":       true,
		"dedicated_worktree_metadata_required":     true,
		"dedicated_worktree_path_invalid":          true,
		"dedicated_worktree_required":              true,
		"decision_contract_path_invalid":           true,
		"decision_contract_unreadable":             true,
		"decision_section_placeholder":             true,
		"decision_status_rejected":                 true,
		"decision_status_unknown":                  true,
		"decision_structure_invalid":               true,
		"governance_action_required":               true,
		"governed_bundle_path_invalid":             true,
		"high_risk_check_failed":                   true,
		"high_risk_check_missing":                  true,
		"incomplete_execution_task":                true,
		"intake_clarification_incomplete":          true,
		"intake_confirmation_incomplete":           true,
		"intake_substep_invalid":                   true,
		"lifecycle_event_write_failed":             true,
		"list_changes_failed":                      true,
		"load_change_failed":                       true,
		"manifest_r0_invalid":                      true,
		"missing_discovery_evidence":               true,
		"missing_required_artifact":                true,
		"missing_task_evidence_for_run_summary":    true,
		"missing_worktree_branch":                  true,
		"missing_worktree_path":                    true,
		"non_pass_task":                            true,
		"not_done_ready":                           true,
		"preset_confirmation_required":             true,
		"research_structure_invalid":               true,
		"research_section_placeholder":             true,
		"run_slipway_done_to_finalize":             true,
		"run_slipway_run_to_advance":               true,
		"ship_gate_blocked":                        true,
		"tasks_checklist_invalid_format":           true,
		"unknown_reason_code":                      true,
		"verification_evidence_missing":            true,
		"worktree_metadata_persist_failed":         true,
		"worktree_validation_error":                true,
	}
	prefixes := []string{
		"plan_audit_",
		"plan_checker_",
		"plan_dimension_",
		"required_artifact_",
		"required_skill_",
		"review_layer_",
		"scope_contract_",
		"sensitive_evidence_",
		"stale_execution_",
		"stale_planning_",
		"tasks_checklist_",
		"tasks_plan_",
		"wave_",
	}
	codes := make([]string, 0, len(canonicalReasonDefinitions))
	for code := range canonicalReasonDefinitions {
		if exact[code] || hasAnyPrefix(code, prefixes) {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	return codes
}

func hasAnyPrefix(code string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(code, prefix) {
			return true
		}
	}
	return false
}

func sampleRecoveryDetail(code string) string {
	switch code {
	case "assurance_structure_invalid":
		return "missing required Evidence section: closeout:assurance_complete=pass"
	case "assurance_contract_path_invalid", "decision_contract_path_invalid":
		return "permission denied"
	case "assurance_section_placeholder":
		return "## Scope Summary"
	case "closeout_assurance_attestation_missing":
		return "final-closeout must record closeout:assurance_complete=pass on standard/strict"
	case "closeout_goal_verification_reuse_invalid":
		return "goal-verification evidence was produced before final-closeout input changed; rerun goal-verification, then rerun final-closeout"
	case "governance_action_required":
		return "domain-review:run domain-aware review"
	case "governed_bundle_path_invalid":
		return "../outside"
	case "high_risk_check_failed", "high_risk_check_missing":
		return "external_api_contracts.safety_baseline"
	case "incomplete_execution_task":
		return "t-19"
	case "missing_required_artifact", "required_artifact_schema_missing", "required_artifact_unreadable":
		return "decision.md"
	case "required_artifact_dependency_missing":
		return "decision.md->requirements.md"
	case "missing_task_evidence_for_run_summary":
		return "run_summary_version=1"
	case "non_pass_task":
		return "t-01"
	case "plan_audit_budget_exhausted":
		return "checker iteration budget exhausted before plan audit passed (rescope is S2_EXECUTE-only)"
	case "plan_audit_iteration":
		return "1/2"
	case "plan_checker_feedback_required":
		return "rerun_plan_audit_with_blocker_feedback"
	case "plan_dimension_scope_out_of_bounds_target":
		return "t-01:../outside.go"
	case "plan_dimension_dependency_unknown", "plan_dimension_coverage_unknown_requirement":
		return "t-01->t-99"
	case "plan_dimension_coverage_missing_requirement", "plan_dimension_coverage_requirement_id_missing":
		return "REQ-001"
	case "review_layer_missing", "review_layer_failed":
		return "IR1"
	case "required_skill_stale":
		return "plan-audit:assurance.md"
	case "required_skill_blockers_present", "required_skill_missing", "required_skill_not_passed", "required_skill_not_ready":
		return "plan-audit"
	case "decision_section_placeholder":
		return "## Selected Approach"
	case "decision_structure_invalid":
		return "missing required heading \"## Selected Approach\""
	case "research_structure_invalid":
		return "section \"Findings\" must have non-empty content"
	case "research_section_placeholder":
		return "## Alternatives Considered"
	case "ship_gate_blocked":
		return "required_skill_missing:final-closeout"
	case "scope_contract_changed_files_missing", "scope_contract_missing", "wave_orchestration_stale_task_evidence":
		return "t-01"
	case "scope_contract_drift":
		return "cmd/next.go"
	case "sensitive_evidence_missing":
		return "schema_migration:db/migrations/001_create_users.sql"
	case "tasks_checklist_duplicate_task_id":
		return "t-01"
	case "tasks_checklist_task_id_missing":
		return "index_0"
	case "tasks_plan_changed_since_task_evidence":
		return "t-03"
	case "worktree_validation_error":
		return "missing branch"
	case "worktree_metadata_persist_failed":
		return "permission denied"
	case "wave_plan_missing":
		return "change-slug"
	default:
		return ""
	}
}

func TestSensitiveEvidenceRecoveryPointsToEvidenceTaskWithoutBypass(t *testing.T) {
	t.Parallel()

	step, ok := recoveryStepFor(NewReasonCode("sensitive_evidence_missing", "schema_migration:db/migrations/001_create_users.sql"))
	require.True(t, ok)
	assert.Equal(t, "slipway run", step.Command)
	assert.Contains(t, step.Remediation, "S2_EXECUTE")
	assert.Contains(t, step.Remediation, "slipway evidence task")
	assert.Contains(t, step.Remediation, "migration-applied")
	assert.Contains(t, step.Remediation, "db/migrations/001_create_users.sql")
	assert.NotContains(t, step.Remediation, "GSD_SKIP_SCHEMA_CHECK")
	assert.NotContains(t, step.Remediation, "bypass")
}

func TestRecoveryStepFillsSubjectIntoCommand(t *testing.T) {
	t.Parallel()

	rc := ReasonCodeFromSpec("required_skill_stale:plan-audit:assurance.md")
	step, ok := recoveryStepFor(rc)
	require.True(t, ok)
	assert.Equal(t, "slipway run", step.Command)
	assert.NotContains(t, step.Remediation, "{subject}")
	assert.NotContains(t, step.Remediation, "{detail}")
}

func TestRecoveryStepFallsBackWhenSubjectMissing(t *testing.T) {
	t.Parallel()

	// A subjectless stale token must not emit a command with an empty placeholder.
	rc := ReasonCodeFromSpec("required_skill_stale")
	step, ok := recoveryStepFor(rc)
	require.True(t, ok)
	assert.NotContains(t, step.Command, "{subject}")
	assert.Equal(t, "slipway run", step.Command, "must fall back when the subject is missing")
}

func TestBuildRecoveryNilOnCleanState(t *testing.T) {
	t.Parallel()

	// Informational, non-actionable blockers must not produce a recovery object.
	blockers := []ReasonCode{NewReasonCode("no_skill_required", "S1_PLAN")}
	assert.Nil(t, BuildRecovery(blockers))
	assert.Nil(t, BuildRecovery(nil))
}

func TestBuildRecoveryWorktreeBranchMismatchRoutesToRun(t *testing.T) {
	t.Parallel()

	// #86: a bound-worktree branch mismatch must route to `slipway run` (which
	// reconciles the recorded branch), not the hollow `slipway repair` dead-end.
	got := BuildRecovery([]ReasonCode{
		NewReasonCode("dedicated_worktree_branch_mismatch", ""),
	})
	require.NotNil(t, got)
	assert.Equal(t, "slipway run", got.PrimaryCommand)
	assert.NotContains(t, got.PrimaryAction, "slipway repair")
	assert.Contains(t, got.PrimaryAction, "slipway run")
}

func TestBuildRecoveryOrphanedChangeBundleRoutesToDelete(t *testing.T) {
	t.Parallel()

	// #129: a governed bundle missing its change.yaml authority must route to the
	// public `slipway delete --change <slug>` surface (with the slug filled in),
	// not a dead-end integrity error.
	got := BuildRecovery([]ReasonCode{
		NewReasonCode("orphaned_change_bundle", "abandoned-change"),
	})
	require.NotNil(t, got)
	assert.Equal(t, "slipway delete --change abandoned-change", got.PrimaryCommand)
	assert.Equal(t, RecoveryClassDiscardChange, got.RecoveryClass)
	require.Len(t, got.Steps, 1)
	assert.Equal(t, "abandoned-change", got.Steps[0].Subject)
	assert.Contains(t, got.Steps[0].Remediation, "slipway delete --change abandoned-change")
}

func TestBuildRecoveryStaleRuntimeBindingRoutesToDelete(t *testing.T) {
	t.Parallel()

	// #129: when only git-local runtime state remains after a bundle was fully
	// removed, recovery must still route through the public delete surface.
	got := BuildRecovery([]ReasonCode{
		NewReasonCode("stale_runtime_binding", "abandoned-change"),
	})
	require.NotNil(t, got)
	assert.Equal(t, "slipway delete --change abandoned-change", got.PrimaryCommand)
	assert.Equal(t, RecoveryClassDiscardChange, got.RecoveryClass)
	require.Len(t, got.Steps, 1)
	assert.Equal(t, "abandoned-change", got.Steps[0].Subject)
	assert.Contains(t, got.Steps[0].Remediation, "slipway delete --change abandoned-change")
}

func TestBuildRecoverySelectsPrimaryByStagePriority(t *testing.T) {
	t.Parallel()

	// reopen_evidence (root-most) must win over refresh_execution (later stage),
	// regardless of blocker order.
	blockers := []ReasonCode{
		NewReasonCode("stale_execution_evidence", ""),
		NewReasonCode("stale_planning_evidence", ""),
		NewReasonCode("no_skill_required", "S2_EXECUTE"),
	}
	got := BuildRecovery(blockers)
	require.NotNil(t, got)
	assert.Equal(t, RecoveryClassReopenEvidence, got.RecoveryClass)
	assert.NotEmpty(t, got.PrimaryCommand)
	for _, step := range got.Steps {
		assert.NotEqual(t, "no_skill_required", step.Code, "informational blocker must not appear as a step")
	}
}

func TestBuildRecoveryPrioritizesStaleEvidenceRecoveryOverDerivedShipBlockers(t *testing.T) {
	t.Parallel()

	got := BuildRecovery([]ReasonCode{
		NewReasonCode("high_risk_check_missing", "external_api_contracts.safety_baseline"),
		NewReasonCode("closeout_assurance_attestation_missing",
			"final-closeout must record closeout:assurance_complete=pass on standard/strict"),
		NewReasonCode("required_skill_stale", "code-quality-review:CLAUDE.md"),
		NewReasonCode("stale_evidence_recovery_available", "S3_REVIEW"),
		NewReasonCode("run_slipway_run_to_advance", "S4_VERIFY"),
		NewReasonCode("verification_evidence_missing", ""),
	})
	require.NotNil(t, got)
	assert.Equal(t, "slipway run", got.PrimaryCommand)
	assert.Equal(t, RecoveryClassReopenEvidence, got.RecoveryClass)
	assert.Contains(t, got.PrimaryAction, "earliest affected authority")
}

func TestRecoveryTokensUseCanonicalMessages(t *testing.T) {
	t.Parallel()

	specs := []string{
		"governance_action_required:domain-review:run domain-aware review",
		"preset_confirmation_required",
		"tasks_plan_changed_since_task_evidence:t-03",
	}
	for _, spec := range specs {
		rc := ReasonCodeFromSpec(spec)
		definition, ok := canonicalReasonDefinitions[rc.Code]
		require.True(t, ok)
		assert.NotEqualf(t, testHumanizeReasonCode(rc.Code), definition.Message,
			"message for %q must be canonical, not the humanize fallthrough", spec)
	}
}

func TestArtifactAuthoringBlockersRouteToInstructions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		spec    string
		command string
	}{
		{
			name:    "decision structure",
			spec:    "decision_structure_invalid:missing required heading \"## Selected Approach\"",
			command: "slipway instructions decision",
		},
		{
			name:    "decision placeholder",
			spec:    "decision_section_placeholder:## Selected Approach",
			command: "slipway instructions decision",
		},
		{
			name:    "research structure",
			spec:    "research_structure_invalid:section \"Findings\" must have non-empty content",
			command: "slipway instructions research",
		},
		{
			name:    "research placeholder",
			spec:    "research_section_placeholder:## Alternatives Considered",
			command: "slipway instructions research",
		},
		{
			name:    "assurance placeholder",
			spec:    "assurance_section_placeholder:## Scope Summary",
			command: "slipway instructions assurance",
		},
		{
			name:    "missing custom artifact",
			spec:    "missing_required_artifact:my-widget.md",
			command: "slipway instructions my-widget.md",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			step, ok := recoveryStepFor(ReasonCodeFromSpec(tc.spec))
			require.True(t, ok)
			assert.Equal(t, tc.command, step.Command)
			assert.NotContains(t, step.Remediation, "{")
		})
	}
}

func TestReasonCodeJSONShapeHasNoPresentationFields(t *testing.T) {
	t.Parallel()

	// Read-only/additive invariant: the persisted ReasonCode must not gain
	// recovery/remediation presentation fields.
	rc := NewReasonCode("required_skill_stale", "plan-audit:assurance.md")
	raw, err := json.Marshal(rc)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	assert.Equal(t, []string{"code", "detail", "message", "severity"}, got,
		"ReasonCode JSON must carry no presentation fields")
}

func TestPlanAuditRecoveryDoesNotRecommendRescopeInS1(t *testing.T) {
	t.Parallel()

	// rescope is S2_EXECUTE-only (gate.EvaluateGPivot), but these blockers are
	// produced on the S1 plan-audit path. The recovery command must therefore not
	// hand the operator a `pivot --rescope` the gate will reject; reroute is valid
	// from S1 through S4.
	for _, code := range []string{"plan_audit_budget_exhausted", "plan_checker_loop_terminated"} {
		rc := NewReasonCode(code, "")
		step, ok := recoveryStepFor(rc)
		require.Truef(t, ok, "%s must produce a recovery step", code)
		assert.Equalf(t, "slipway pivot --reroute", step.Command, "%s must recommend reroute, not rescope", code)
		assert.NotContainsf(t, step.Command, "rescope", "%s command must not point at S1-invalid rescope", code)
		assert.NotContainsf(t, step.Remediation, "--rescope", "%s remediation must not point at S1-invalid rescope", code)
	}
}

func TestCloseoutAttestationMissingResolvesToRecovery(t *testing.T) {
	t.Parallel()

	// closeout_assurance_attestation_missing is a real G_ship blocker but was not
	// canonicalized or in the remediation table, so BuildRecovery silently skipped
	// it on real S4 output. It must now render a step with a remediation/command.
	rc := NewReasonCode("closeout_assurance_attestation_missing",
		"final-closeout must record closeout:assurance_complete=pass on standard/strict")
	step, ok := recoveryStepFor(rc)
	require.True(t, ok, "closeout_assurance_attestation_missing must produce a recovery step")
	assert.NotEmpty(t, step.Remediation)
	assert.NotEmpty(t, step.Command)
	assert.Contains(t, step.Remediation, "final-closeout")
	assert.NotContains(t, step.Remediation, "{", "static remediation must not leak a placeholder")
	assert.Empty(t, step.Subject, "opaque prose detail must not become a synthetic subject")
	assert.Equal(t,
		[]string{"final-closeout must record closeout:assurance_complete=pass on standard/strict"},
		step.Details,
		"the colon-bearing attestation token must stay intact in Details")
	// Canonical message, not the old humanize fallback.
	definition, ok := canonicalReasonDefinitions[rc.Code]
	require.True(t, ok)
	assert.NotEqual(t, testHumanizeReasonCode(rc.Code), definition.Message,
		"message must be the written canonical sentence")
}

func TestBuildRecoveryCoversRealValidationAndShipBlockers(t *testing.T) {
	t.Parallel()

	blockers := []ReasonCode{
		NewReasonCode("missing_required_artifact", "decision.md"),
		NewReasonCode("plan_dimension_dependency_unknown", "t-01->t-99"),
		NewReasonCode("review_layer_missing", "IR1"),
		NewReasonCode("tasks_checklist_empty", ""),
	}
	got := BuildRecovery(blockers)
	require.NotNil(t, got)
	assert.ElementsMatch(t,
		[]string{
			"missing_required_artifact",
			"plan_dimension_dependency_unknown",
			"review_layer_missing",
			"tasks_checklist_empty",
		},
		recoveryCodes(got.Steps),
		"real validate/done blockers must not be silently skipped")
}

func TestBuildRecoveryPrioritizesVerificationBeforeCloseout(t *testing.T) {
	t.Parallel()

	got := BuildRecovery([]ReasonCode{
		NewReasonCode("closeout_assurance_attestation_missing",
			"final-closeout must record closeout:assurance_complete=pass on standard/strict"),
		NewReasonCode("closeout_goal_verification_reuse_invalid",
			"goal-verification evidence was produced before final-closeout input changed; rerun goal-verification, then rerun final-closeout"),
		NewReasonCode("verification_evidence_missing", "goal-verification"),
	})
	require.NotNil(t, got)
	assert.Equal(t, RecoveryClassRerunSkill, got.RecoveryClass)
	assert.Contains(t, got.PrimaryAction, "goal-verification",
		"goal-verification recovery must precede final-closeout when both S4 blockers are present")
}

func TestBuildRecoveryPrioritizesMissingArtifactsByAuthoringOrder(t *testing.T) {
	t.Parallel()

	got := BuildRecovery([]ReasonCode{
		NewReasonCode("missing_required_artifact", "assurance.md"),
		NewReasonCode("missing_required_artifact", "tasks.md"),
		NewReasonCode("missing_required_artifact", "decision.md"),
		NewReasonCode("missing_required_artifact", "requirements.md"),
	})
	require.NotNil(t, got)
	assert.Equal(t, "slipway instructions requirements.md", got.PrimaryCommand)
	assert.Contains(t, got.PrimaryAction, "requirements.md",
		"missing artifact recovery must start at the earliest plan authoring input, not the alphabetically first downstream artifact")
}

func TestReadyStatesSurfaceAdvanceRecovery(t *testing.T) {
	t.Parallel()

	// A ready-to-advance or ready-to-finalize state is not blocked, but its single
	// trustworthy next action is still surfaced as the primary command, and the two
	// ready advisories are handled symmetrically.
	advance := BuildRecovery([]ReasonCode{
		NewReasonCode("run_slipway_run_to_advance", "S2_EXECUTE"),
		NewReasonCode("no_skill_required", "S2_EXECUTE"),
	})
	require.NotNil(t, advance)
	assert.Equal(t, "slipway run", advance.PrimaryCommand)
	assert.Equal(t, RecoveryClassAdvance, advance.RecoveryClass)

	finalize := BuildRecovery([]ReasonCode{NewReasonCode("run_slipway_done_to_finalize", "")})
	require.NotNil(t, finalize)
	assert.Equal(t, "slipway done", finalize.PrimaryCommand)
}

func TestBuildRecoveryGroupsBlockersByCodeAndSubject(t *testing.T) {
	t.Parallel()

	// Many stale artifacts under one skill must collapse into a single step that
	// lists the artifacts in Details; a second skill stays a distinct step.
	blockers := []ReasonCode{
		NewReasonCode("required_skill_stale", "code-quality-review:CLAUDE.md"),
		NewReasonCode("required_skill_stale", "code-quality-review:README.md"),
		NewReasonCode("required_skill_stale", "code-quality-review:cmd/next.go"),
		NewReasonCode("required_skill_stale", "code-quality-review:CLAUDE.md"), // duplicate
		NewReasonCode("required_skill_stale", "plan-audit:tasks.md"),
	}
	got := BuildRecovery(blockers)
	require.NotNil(t, got)
	require.Len(t, got.Steps, 2, "one step per (code, subject), not per blocker")

	var cqr *RecoveryStep
	for i := range got.Steps {
		if got.Steps[i].Subject == "code-quality-review" {
			cqr = &got.Steps[i]
		}
	}
	require.NotNil(t, cqr, "the code-quality-review group must be one step")
	assert.Equal(t, []string{"CLAUDE.md", "README.md", "cmd/next.go"}, cqr.Details,
		"details are de-duplicated and sorted")
	assert.Equal(t, "slipway run", cqr.Command)
	assert.NotContains(t, cqr.Remediation, "{", "remediation must not embed a per-detail placeholder")
}

func TestGovernanceActionRemediationStaysCleanWithoutDetail(t *testing.T) {
	t.Parallel()

	// A subjectless/detailless token must not leak the artifacts of an empty
	// placeholder substitution (empty quotes, dangling separator, double space).
	step, ok := recoveryStepFor(NewReasonCode("governance_action_required", ""))
	require.True(t, ok)
	assert.NotContains(t, step.Remediation, "''")
	assert.NotContains(t, step.Remediation, ": .")
	assert.NotContains(t, step.Remediation, "  ")
	assert.NotContains(t, step.Remediation, "{")
}

func TestDecisionContractUnreadableRoutesToRepair(t *testing.T) {
	t.Parallel()

	step, ok := recoveryStepFor(NewReasonCode("decision_contract_unreadable", ""))
	require.True(t, ok)
	assert.Equal(t, "slipway repair", step.Command)
}

func TestMissingTargetFilesRecoveryAppliesToEveryTaskKind(t *testing.T) {
	t.Parallel()

	rc := NewReasonCode("plan_dimension_key_links_missing_target_files", "t-01")
	step, ok := recoveryStepFor(rc)
	require.True(t, ok)
	assert.Equal(t, "plan_dimension_key_links_missing_target_files", rc.Code)
	assert.Equal(t, "t-01", rc.Detail)
	assert.Contains(t, step.Remediation, "every task")
	assert.NotContains(t, step.Remediation, "code task")
}

func recoveryCodes(steps []RecoveryStep) []string {
	codes := make([]string, 0, len(steps))
	for _, step := range steps {
		codes = append(codes, step.Code)
	}
	return codes
}

// recoveryStepFor resolves a single blocker into a RecoveryStep. It is a small
// single-blocker wrapper used only by tests that do not need group collapse.
// The second return is false for blockers with no recovery-relevant
// remediation (e.g. informational no_skill_required), which are skipped.
func recoveryStepFor(rc ReasonCode) (RecoveryStep, bool) {
	rc.Normalize()
	if _, ok := blockerRemediations[rc.Code]; !ok {
		return RecoveryStep{}, false
	}
	return recoveryStepForGroup([]ReasonCode{rc}), true
}
