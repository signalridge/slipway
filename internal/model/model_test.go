package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTaskRunKeyRejectsEmbeddedDelimiter(t *testing.T) {
	t.Parallel()
	_, err := BuildTaskRunKey("task-a__rvshadow", 1)
	require.Error(t, err)
}

func TestChangeUnmarshalYAMLRequiresCanonicalFields(t *testing.T) {
	t.Parallel()

	var change Change
	require.NoError(t, yaml.Unmarshal([]byte(`
slug: canonical-change
status: active
current_state: S1_PLAN
plan_substep: bundle
`), &change))

	assert.Equal(t, "canonical-change", change.Slug)
	assert.Equal(t, ChangeStatusActive, change.Status)
	assert.Equal(t, StateS1Plan, change.CurrentState)
	assert.Equal(t, PlanSubStepBundle, change.PlanSubStep)
	require.NoError(t, change.Validate())
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
			Version:     2,
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
	change.EvidenceRefs = map[string]string{
		"plan-audit": "evidence/governance/plan-audit.json",
	}
	change.ActiveCheckpoint = &ActiveCheckpoint{
		PausedTaskID:     "task-01",
		CheckpointType:   string(CheckpointDecision),
		AllowedResponses: []string{"approved", "needs_changes"},
	}
	change.Normalize()

	raw, err := yaml.Marshal(change)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "gates:")
	assert.NotContains(t, string(raw), "\nartifacts:")
	assert.NotContains(t, string(raw), "evidence_refs:")
	assert.NotContains(t, string(raw), "last_auto_passed_states:")
	assert.NotContains(t, string(raw), "review_intent_drift_failures:")

	var decoded Change
	require.NoError(t, yaml.Unmarshal(raw, &decoded))
	decoded.Normalize()

	require.NoError(t, decoded.Validate())
	assert.Equal(t, change.Slug, decoded.Slug)
	assert.Equal(t, change.Description, decoded.Description)
	assert.Equal(t, change.Status, decoded.Status)
	assert.Equal(t, change.CurrentState, decoded.CurrentState)
	assert.True(t, change.CreatedAt.Equal(decoded.CreatedAt))
	assert.Equal(t, change.GuardrailDomain, decoded.GuardrailDomain)
	assert.Equal(t, change.WorktreePath, decoded.WorktreePath)
	assert.Equal(t, change.WorktreeBranch, decoded.WorktreeBranch)
	assert.Equal(t, change.ArtifactSchema, decoded.ArtifactSchema)
	assert.Equal(t, change.CustomArtifacts, decoded.CustomArtifacts)
	assert.Equal(t, change.ContextDependencies, decoded.ContextDependencies)
	assert.Empty(t, decoded.Artifacts)
	assert.Empty(t, decoded.EvidenceRefs)
	assert.Empty(t, decoded.LastAutoPassedStates)
	assert.Zero(t, decoded.ReviewIntentDriftFailures)
	require.NotNil(t, decoded.ActiveCheckpoint)
	assert.Equal(t, *change.ActiveCheckpoint, *decoded.ActiveCheckpoint)
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
		{name: "s2_execute", state: StateS2Execute, phase: PhaseBuilding},
		{name: "s3_review", state: StateS3Review, phase: PhaseReviewing},
		{name: "s4_verify", state: StateS4Verify, phase: PhaseReviewing},
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
    rollback-required: blocking
  disabled_controls:
    - research
`)

	cfg, err := ParseConfigYAML(raw)
	require.NoError(t, err)
	assert.Equal(t, WorkflowPresetLight, cfg.Governance.DefaultPreset)
	assert.Equal(t, WorkflowPresetStandard, cfg.Governance.MinPreset)
	assert.Equal(t, ControlModeAdvisory, cfg.Governance.Controls[ControlIndependentReview])
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
		},
	}

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "governance:")
	assert.Contains(t, string(out), "default_preset: light")
	assert.Contains(t, string(out), "min_preset: standard")
	assert.Contains(t, string(out), "independent-review: advisory")

	// Re-parse and verify.
	cfg2, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, WorkflowPresetLight, cfg2.Governance.DefaultPreset)
	assert.Equal(t, WorkflowPresetStandard, cfg2.Governance.MinPreset)
	assert.Equal(t, ControlModeAdvisory, cfg2.Governance.Controls[ControlIndependentReview])
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
			WorktreeBlastRadius: SignalLevelMedium,
		},
	}

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "governance:")
	assert.Contains(t, string(out), "worktree_blast_radius: medium")

	// Re-parse and verify thresholds survive.
	cfg2, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, SignalLevelMedium, cfg2.Governance.Thresholds.WorktreeBlastRadius)
}

func TestConfigGovernanceThresholdsOnlyRoundTrip(t *testing.T) {
	t.Parallel()
	// Thresholds-only governance (no controls, no disabled_controls) must persist.
	cfg := DefaultConfig()
	cfg.Governance = ConfigGovernance{
		Thresholds: ConfigGovernanceThresholds{
			IndependentReviewBlastRadius: SignalLevelMedium,
			WorktreeBlastRadius:          SignalLevelLow,
		},
	}

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "governance:")

	cfg2, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, SignalLevelMedium, cfg2.Governance.Thresholds.IndependentReviewBlastRadius)
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
    independent_review_blast_radius: low
`)
	cfg, err := ParseConfigYAML(raw)
	require.NoError(t, err)
	assert.Equal(t, SignalLevelMedium, cfg.Governance.Thresholds.WorktreeBlastRadius)
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

func TestChangeAuthorityOmitsRuntimeSidecarFields(t *testing.T) {
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
	assert.NotContains(t, string(b), "\nartifacts:")
	assert.NotContains(t, string(b), "evidence_refs:")
	assert.NotContains(t, string(b), "last_auto_passed_states:")
	assert.NotContains(t, string(b), "review_intent_drift_failures:")
}

func TestActiveCheckpointRequiresPausedTaskID(t *testing.T) {
	t.Parallel()

	cp := ActiveCheckpoint{
		CheckpointType: string(CheckpointHumanVerify),
	}
	require.ErrorContains(t, cp.Validate(), "paused_task_id is required")

	cp.PausedTaskID = "   "
	require.ErrorContains(t, cp.Validate(), "paused_task_id is required")

	cp.PausedTaskID = "task-01"
	require.NoError(t, cp.Validate())
}
