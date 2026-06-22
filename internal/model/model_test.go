package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTaskRunKeyUsesTaskIDWithoutRunVersionSuffix(t *testing.T) {
	t.Parallel()
	key, err := BuildTaskRunKey("task-a")
	require.NoError(t, err)
	assert.Equal(t, "task-a", key)
}

func TestExecutionTaskFreshnessInputsFieldMapOmitsZeroValue(t *testing.T) {
	t.Parallel()

	assert.Empty(t, ExecutionTaskFreshnessInputs{}.FieldMap())
	assert.Equal(t, map[string]string{
		"change_id":           "demo",
		"run_summary_version": "1",
		"task_id":             "task-a",
		"guardrail_domain":    "",
	}, (ExecutionTaskFreshnessInputs{
		ChangeID:          "demo",
		RunSummaryVersion: 1,
		TaskID:            "task-a",
	}).FieldMap())
}

func TestChangeUnmarshalYAMLRequiresCanonicalFields(t *testing.T) {
	t.Parallel()

	var change Change
	require.NoError(t, yaml.Unmarshal([]byte(`
version: 1
slug: canonical-change
status: active
current_state: S1_PLAN
plan_substep: bundle
`), &change))
	change.Normalize()

	assert.Equal(t, "canonical-change", change.Slug)
	assert.Equal(t, ChangeVersion, change.Version)
	assert.Equal(t, ChangeStatusActive, change.Status)
	assert.Equal(t, StateS1Plan, change.CurrentState)
	assert.Equal(t, PlanSubStepBundle, change.PlanSubStep)
	require.NoError(t, change.Validate())
}

func TestChangeNormalizeCanonicalizesRetiredWorkflowStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want WorkflowState
	}{
		{
			name: "s2 execute",
			raw:  "S2_EXECUTE",
			want: StateS2Implement,
		},
		{
			name: "s4 verify",
			raw:  "S4_VERIFY",
			want: StateS3Review,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var change Change
			require.NoError(t, yaml.Unmarshal([]byte(`
version: 1
slug: retired-state-change
status: active
current_state: `+tc.raw+`
`), &change))
			change.Normalize()

			assert.Equal(t, tc.want, change.CurrentState)
			require.NoError(t, change.Validate())
		})
	}
}

func TestChangeValidateRejectsInvalidWorkflowProfile(t *testing.T) {
	t.Parallel()

	change := NewChange("invalid-profile")
	change.WorkflowProfile = WorkflowProfile("slides")

	err := change.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workflow_profile")
}

func TestWorkflowProfileMetaIsValidAndDefaultsExpandedArtifacts(t *testing.T) {
	t.Parallel()

	assert.True(t, WorkflowProfileMeta.IsValid())
	assert.True(t, WorkflowProfileMeta.RequiresCodeQualityReview())
	assert.Equal(t, ArtifactSchemaExpanded, DefaultArtifactSchemaForWorkflowProfile(WorkflowProfileMeta, false, ArtifactSchemaCore))
	assert.Equal(t, ArtifactSchemaCore, DefaultArtifactSchemaForWorkflowProfile(WorkflowProfileDocs, false, ArtifactSchemaExpanded))
}

func TestControlActivationValidateRequiresTriggeredBy(t *testing.T) {
	t.Parallel()

	control := ControlActivation{
		ControlID:    ControlDomainReview,
		Mode:         ControlModeBlocking,
		Scope:        ControlScopeReview,
		Active:       true,
		PolicySource: BuiltinPolicySource,
	}

	err := control.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "triggered_by is required")
}

func TestTraceabilitySummaryValidateRejectsEmptyLinkIDs(t *testing.T) {
	t.Parallel()

	summary := TraceabilitySummary{
		Status: TraceabilityStatusOK,
		Links: []TraceabilityLink{
			{FromID: "REQ-001", ToID: ""},
		},
	}

	err := summary.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "links[0]")
	assert.Contains(t, err.Error(), "to_id is required")
}

func TestTraceabilitySummaryBlockingIntentGapHelpers(t *testing.T) {
	t.Parallel()

	summary := TraceabilitySummary{
		Status: TraceabilityStatusFail,
		Gaps: []TraceabilityGap{
			{ID: "REQ-1", Type: "requirement", Issue: "non-blocking", Blocking: true},
			{ID: "INT-1", Type: "intent", Issue: "missing intent coverage", Blocking: true},
		},
	}

	gap, found := summary.FirstBlockingIntentGap()
	require.True(t, found)
	assert.Equal(t, "INT-1", gap.ID)
	assert.True(t, summary.HasBlockingIntentGap())
}

func TestChangeMarshalUnmarshalRoundTripNewFormat(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	artifactUpdatedAt := createdAt.Add(45 * time.Minute)

	change := NewChange("round-trip-new-format")
	change.Description = "round-trip manifest coverage"
	change.CurrentState = StateS1Plan
	change.PlanSubStep = PlanSubStepAudit
	change.CreatedAt = createdAt
	change.GuardrailDomain = GuardrailDomainAuthAuthZ
	change.WorktreePath = "/tmp/slipway-round-trip"
	change.WorktreeBranch = "feature/round-trip"
	change.Artifacts = map[string]ArtifactState{
		"plan": {
			ID:          "plan",
			Path:        "artifacts/changes/round-trip-new-format/plan.md",
			State:       ArtifactLifecycleApproved,
			ContentHash: strings.Repeat("a", 64),
			UpdatedAt:   artifactUpdatedAt,
		},
	}
	change.ArtifactSchema = ArtifactSchemaExpanded
	change.CustomArtifacts = []ArtifactDefinition{{
		Name:              "security-review",
		Template:          "security-review.md",
		RequiresDiscovery: false,
		DependsOn:         []string{"plan"},
	}}
	change.ContextDependencies = ContextDependencies{
		Requires: []ContextRequirement{
			{Slug: "baseline-auth", Provides: []string{"auth-contract", "session-model"}},
		},
	}
	change.LastAutoPassedStates = []AutoPassedState{{
		State:  StateS3Review,
		Reason: "no_blocking_review_obligations",
	}}
	change.ReviewIntentDriftFailures = 2
	change.EvidenceRefs = map[string]string{
		"plan-audit": "evidence/governance/plan-audit.json",
	}
	change.InterruptedExecutionAt = createdAt.Add(2 * time.Hour)
	change.Normalize()

	raw, err := yaml.Marshal(change)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "version: 1")
	assert.NotContains(t, string(raw), "gates:")
	assert.Contains(t, string(raw), "artifacts:")
	assert.Contains(t, string(raw), "last_auto_passed_states:")
	assert.Contains(t, string(raw), "evidence_refs:")
	assert.Contains(t, string(raw), "review_intent_drift_failures:")
	assert.Contains(t, string(raw), "interrupted_execution_at:")
	// The machine-local absolute worktree path is never persisted to tracked
	// change.yaml; the portable branch metadata still is.
	assert.NotContains(t, string(raw), "worktree_path:")
	assert.Contains(t, string(raw), "worktree_branch:")

	var decoded Change
	require.NoError(t, yaml.Unmarshal(raw, &decoded))
	decoded.Normalize()

	require.NoError(t, decoded.Validate())
	assert.Equal(t, ChangeVersion, decoded.Version)
	assert.Equal(t, change.Slug, decoded.Slug)
	assert.Equal(t, change.Description, decoded.Description)
	assert.Equal(t, change.Status, decoded.Status)
	assert.Equal(t, change.CurrentState, decoded.CurrentState)
	assert.True(t, change.CreatedAt.Equal(decoded.CreatedAt))
	assert.Equal(t, change.GuardrailDomain, decoded.GuardrailDomain)
	assert.Empty(t, decoded.WorktreePath, "worktree_path must not round-trip through tracked change.yaml")
	assert.Equal(t, change.WorktreeBranch, decoded.WorktreeBranch)
	assert.Equal(t, change.ArtifactSchema, decoded.ArtifactSchema)
	assert.Equal(t, change.CustomArtifacts, decoded.CustomArtifacts)
	assert.Equal(t, change.ContextDependencies, decoded.ContextDependencies)
	assert.Equal(t, change.Artifacts, decoded.Artifacts)
	assert.Equal(t, change.LastAutoPassedStates, decoded.LastAutoPassedStates)
	assert.Equal(t, change.EvidenceRefs, decoded.EvidenceRefs)
	assert.Equal(t, change.ReviewIntentDriftFailures, decoded.ReviewIntentDriftFailures)
	assert.True(t, change.InterruptedExecutionAt.Equal(decoded.InterruptedExecutionAt))
}

func TestChangeUnmarshalYAMLParsesInterruptedExecutionAt(t *testing.T) {
	t.Parallel()

	var change Change
	require.NoError(t, yaml.Unmarshal([]byte(`
version: 1
slug: unified-runtime-field
status: active
current_state: S2_IMPLEMENT
interrupted_execution_at: 2026-04-10T12:00:00Z
`), &change))
	change.Normalize()

	expected := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	assert.True(t, change.InterruptedExecutionAt.Equal(expected))
}

func TestChangeValidateRejectsUnsupportedVersion(t *testing.T) {
	t.Parallel()

	change := NewChange("unsupported-version")
	change.Version = ChangeVersion + 1

	err := change.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported change version")
}

func TestNormalizeReasonCodesSortsGateReasons(t *testing.T) {
	t.Parallel()

	reasonCodes := NormalizeReasonCodes([]ReasonCode{
		NewReasonCode("verification_evidence_missing", ""),
		NewReasonCode("required_skill_missing", "final-closeout"),
	})

	require.Len(t, reasonCodes, 2)
	assert.Equal(t, "required_skill_missing", reasonCodes[0].Code)
	assert.Equal(t, ReasonSeverityError, reasonCodes[0].Severity)
	assert.Equal(t, "verification_evidence_missing", reasonCodes[1].Code)
}

func TestWaveReasonCodesCarryRemediation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code   string
		detail string
	}{
		{"wave_orchestration_stale_task_evidence", "task-a"},
		{"wave_orchestration_run_summary_version_invalid", ""},
		{"missing_task_evidence_for_run_summary", ""},
		{"wave_plan_missing", "slug-a"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()
			reason := NewReasonCode(tc.code, tc.detail)
			assert.Equal(t, tc.code, reason.Code)
			assert.Equal(t, tc.detail, reason.Detail)
			assert.Equal(t, ReasonSeverityError, reason.Severity)
			definition, ok := canonicalReasonDefinitions[reason.Code]
			require.True(t, ok)
			assert.NotEqualf(t, testHumanizeReasonCode(reason.Code), definition.Message,
				"wave reason code %q must carry an actionable remediation, not a humanized fallback", tc.code)
		})
	}
}

func TestRequiredSkillStaleCarriesRemediation(t *testing.T) {
	t.Parallel()

	reason := ReasonCodeFromSpec("required_skill_stale:plan-audit:tasks.md")

	assert.Equal(t, "required_skill_stale", reason.Code)
	assert.Equal(t, "plan-audit:tasks.md", reason.Detail)
	assert.Equal(t, ReasonSeverityError, reason.Severity)
	definition, ok := canonicalReasonDefinitions[reason.Code]
	require.True(t, ok)
	assert.NotEqual(t, testHumanizeReasonCode(reason.Code), definition.Message)
}

func TestPhaseForMapsWorkflowStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		state WorkflowState
		phase UserPhase
	}{
		// New simplified states
		{name: "s0_intake", state: StateS0Intake, phase: PhaseIntake},
		{name: "s1_plan", state: StateS1Plan, phase: PhasePlanning},
		{name: "s2_implement", state: StateS2Implement, phase: PhaseBuilding},
		{name: "s3_review", state: StateS3Review, phase: PhaseReviewing},
		{name: "s4_verify", state: StateS3Review, phase: PhaseReviewing},
		{name: "done", state: StateDone, phase: PhaseDone},
		// Unknown states fall through to PhaseIntake (default)
		{name: "unknown falls back to intake", state: WorkflowState("UNKNOWN"), phase: PhaseIntake},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.phase, PhaseFor(tc.state))
		})
	}
}

func TestChangePhaseUsesCurrentStateMapping(t *testing.T) {
	t.Parallel()

	change := NewChange("phase-mapping")
	change.CurrentState = StateS3Review
	change.PlanSubStep = PlanSubStepNone

	assert.Equal(t, PhaseReviewing, change.Phase())
}

func TestConfigUnknownTopLevelPreserved(t *testing.T) {
	t.Parallel()
	raw := []byte(`
defaults:
  artifact_schema: expanded
execution:
  lock_wait_timeout_seconds: 15
custom_block:
  enabled: true
`)

	cfg, err := ParseConfigYAML(raw)
	require.NoError(t, err)
	require.Contains(t, cfg.UnknownTopLevel, "custom_block")

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "custom_block:")
	assert.Contains(t, string(out), "enabled: true")
}

func TestConfigRejectsRemovedAgentsSurface(t *testing.T) {
	t.Parallel()
	raw := []byte(`
agents:
  mappings:
    wave-orchestration: slipway-orchestrator
`)

	_, err := ParseConfigYAML(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "top-level agents configuration has been removed")
	assert.Contains(t, err.Error(), "next_skill.name")
}

func TestConfigGovernanceParsing(t *testing.T) {
	t.Parallel()
	raw := []byte(`
defaults:
  artifact_schema: expanded
execution:
  lock_wait_timeout_seconds: 15
governance:
  default_preset: light
  min_preset: standard
  controls:
    independent-review: advisory
    security-review: blocking
    rollback-required: blocking
  disabled_controls:
    - research
`)

	cfg, err := ParseConfigYAML(raw)
	require.NoError(t, err)
	assert.Equal(t, WorkflowPresetLight, cfg.Governance.DefaultPreset)
	assert.Equal(t, WorkflowPresetStandard, cfg.Governance.MinPreset)
	assert.Equal(t, ControlModeAdvisory, cfg.Governance.Controls[ControlIndependentReview])
	assert.Equal(t, ControlModeBlocking, cfg.Governance.Controls[ControlSecurityReview])
	assert.Equal(t, ControlModeBlocking, cfg.Governance.Controls[ControlRollbackRequired])
	assert.Contains(t, cfg.Governance.DisabledControls, ControlResearch)
}

func TestConfigGovernanceRoundTrip(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Governance = ConfigGovernance{
		DefaultPreset: WorkflowPresetLight,
		MinPreset:     WorkflowPresetStandard,
		Controls: map[ControlID]ControlMode{
			ControlIndependentReview: ControlModeAdvisory,
			ControlSecurityReview:    ControlModeBlocking,
		},
	}

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "governance:")
	assert.Contains(t, string(out), "default_preset: light")
	assert.Contains(t, string(out), "min_preset: standard")
	assert.Contains(t, string(out), "independent-review: advisory")
	assert.Contains(t, string(out), "security-review: blocking")

	// Re-parse and verify.
	cfg2, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, WorkflowPresetLight, cfg2.Governance.DefaultPreset)
	assert.Equal(t, WorkflowPresetStandard, cfg2.Governance.MinPreset)
	assert.Equal(t, ControlModeAdvisory, cfg2.Governance.Controls[ControlIndependentReview])
	assert.Equal(t, ControlModeBlocking, cfg2.Governance.Controls[ControlSecurityReview])
}

func TestConfigGovernancePolicyPacksAreAdvisoryOnly(t *testing.T) {
	t.Parallel()
	raw := []byte(`
governance:
  policy_packs:
    - name: platform
      path: .slipway/policies/platform.yaml
`)

	cfg, err := ParseConfigYAML(raw)
	require.NoError(t, err)
	require.Len(t, cfg.Governance.PolicyPacks, 1)
	assert.Equal(t, "platform", cfg.Governance.PolicyPacks[0].Name)
	assert.Equal(t, ControlModeAdvisory, cfg.Governance.PolicyPacks[0].Mode)

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "policy_packs:")
	assert.Contains(t, string(out), "mode: advisory")
}

func TestConfigGovernancePolicyPacksRejectBlockingMode(t *testing.T) {
	t.Parallel()
	raw := []byte(`
governance:
  policy_packs:
    - name: platform
      path: .slipway/policies/platform.yaml
      mode: blocking
`)

	_, err := ParseConfigYAML(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mode must be advisory")
}

func TestConfigGovernanceEmptyNotSerialized(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "governance:")
}

func TestConfigGovernanceInvalidControlSettingsRejectedOnParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "invalid control id",
			yaml: `
governance:
  controls:
    nonexistent: advisory
`,
			errContains: "unknown control_id",
		},
		{
			name: "invalid mode",
			yaml: `
governance:
  controls:
    worktree-isolation: severe
`,
			errContains: "invalid mode",
		},
		{
			name: "invalid disabled control",
			yaml: `
governance:
  disabled_controls:
    - nonexistent
`,
			errContains: "unknown control_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseConfigYAML([]byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestConfigArtifactSchemaValidationRejectedOnParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "invalid defaults artifact schema",
			yaml: `
defaults:
  artifact_schema: invalid
`,
			errContains: "artifact_schema",
		},
		{
			name: "custom schema requires custom artifacts",
			yaml: `
defaults:
  artifact_schema: custom
`,
			errContains: "custom_artifacts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseConfigYAML([]byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestConfigGovernanceThresholdsRoundTrip(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Governance = ConfigGovernance{
		Thresholds: ConfigGovernanceThresholds{
			SecurityReviewBlastRadius: SignalLevelMedium,
			WorktreeBlastRadius:       SignalLevelMedium,
		},
	}

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "governance:")
	assert.Contains(t, string(out), "security_review_blast_radius: medium")
	assert.Contains(t, string(out), "worktree_blast_radius: medium")

	// Re-parse and verify thresholds survive.
	cfg2, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, SignalLevelMedium, cfg2.Governance.Thresholds.SecurityReviewBlastRadius)
	assert.Equal(t, SignalLevelMedium, cfg2.Governance.Thresholds.WorktreeBlastRadius)
}

func TestConfigGovernanceThresholdsOnlyRoundTrip(t *testing.T) {
	t.Parallel()
	// Thresholds-only governance (no controls, no disabled_controls) must persist.
	cfg := DefaultConfig()
	cfg.Governance = ConfigGovernance{
		Thresholds: ConfigGovernanceThresholds{
			IndependentReviewBlastRadius: SignalLevelMedium,
			SecurityReviewBlastRadius:    SignalLevelHigh,
			WorktreeBlastRadius:          SignalLevelLow,
		},
	}

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "governance:")

	cfg2, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, SignalLevelMedium, cfg2.Governance.Thresholds.IndependentReviewBlastRadius)
	assert.Equal(t, SignalLevelHigh, cfg2.Governance.Thresholds.SecurityReviewBlastRadius)
	assert.Equal(t, SignalLevelLow, cfg2.Governance.Thresholds.WorktreeBlastRadius)
}

func TestConfigGovernanceInvalidThresholdRejectedOnParse(t *testing.T) {
	t.Parallel()
	raw := []byte(`
defaults:
  artifact_schema: expanded
governance:
  thresholds:
    worktree_blast_radius: severe
`)
	_, err := ParseConfigYAML(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "severe")
}

func TestConfigGovernanceValidThresholdAcceptedOnParse(t *testing.T) {
	t.Parallel()
	raw := []byte(`
defaults:
  artifact_schema: expanded
governance:
  thresholds:
    worktree_blast_radius: medium
    security_review_blast_radius: high
    independent_review_blast_radius: low
`)
	cfg, err := ParseConfigYAML(raw)
	require.NoError(t, err)
	assert.Equal(t, SignalLevelMedium, cfg.Governance.Thresholds.WorktreeBlastRadius)
	assert.Equal(t, SignalLevelHigh, cfg.Governance.Thresholds.SecurityReviewBlastRadius)
	assert.Equal(t, SignalLevelLow, cfg.Governance.Thresholds.IndependentReviewBlastRadius)
}

func TestConfigRejectsDeletedExecutionFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		yaml  string
		field string
	}{
		{"per_task_review", "execution:\n  per_task_review: true\n", "per_task_review"},
		{"max_artifact_amendments", "execution:\n  max_artifact_amendments: 5\n", "max_artifact_amendments"},
		{"commit_strategy", "execution:\n  commit_strategy: per_task\n", "commit_strategy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseConfigYAML([]byte(tt.yaml))
			require.Error(t, err, "config with deleted field %q should be rejected", tt.field)
			assert.Contains(t, err.Error(), tt.field)
		})
	}
}

func TestChangeAuthorityIncludesRuntimeFields(t *testing.T) {
	t.Parallel()
	change := Change{
		Slug:         "change-test",
		Status:       ChangeStatusActive,
		CurrentState: StateS1Plan,
		PlanSubStep:  PlanSubStepResearch,
		Artifacts: map[string]ArtifactState{
			"intent": {ID: "intent", State: ArtifactLifecycleDraft},
		},
		EvidenceRefs: map[string]string{
			"plan-audit": "artifacts/changes/change-test/verification/plan-audit.yaml",
		},
		LastAutoPassedStates: []AutoPassedState{{
			State:  StateS3Review,
			Reason: "no_blocking_review_obligations",
		}},
		ReviewIntentDriftFailures: 2,
	}
	b, err := yaml.Marshal(change)
	require.NoError(t, err)
	assert.Contains(t, string(b), "artifacts:")
	assert.Contains(t, string(b), "evidence_refs:")
	assert.Contains(t, string(b), "last_auto_passed_states:")
	assert.Contains(t, string(b), "review_intent_drift_failures:")
}
