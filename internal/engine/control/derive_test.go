package control

import (
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveControls_NoDomainSmallChangeNoDiscovery(t *testing.T) {
	t.Parallel()
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "",
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"a.go", "b.go"}},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	assert.Empty(t, result.ActiveControls, "no-domain small change with no discovery should produce no controls")
	assert.Empty(t, result.NewActivations)
}

func TestDeriveControls_NeedsDiscoveryFalseSkipsExploration(t *testing.T) {
	t.Parallel()
	// Exploration control derives solely from NeedsDiscovery. With NeedsDiscovery=false,
	// exploration is NOT activated regardless of other signals.
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "",
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"a.go"}},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.NotContains(t, controlIDs, model.ControlResearch,
		"exploration control should NOT activate when NeedsDiscovery=false")
}

func TestDeriveControls_DiscoveryRequiredNoGuardrailDomain(t *testing.T) {
	t.Parallel()
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "",
		NeedsDiscovery:  true,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"a.go"}},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlResearch, "exploration control should activate when NeedsDiscovery=true")
	assert.NotContains(t, controlIDs, model.ControlDomainReview, "domain-review should not activate without a guardrail domain")
}

func TestDeriveControls_GuardrailDomainSmallBlastRadius(t *testing.T) {
	t.Parallel()
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "auth_authz",
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"auth.go"}},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlDomainReview, "domain triggers domain-review")
	assert.Contains(t, controlIDs, model.ControlIndependentReview, "domain triggers independent-review even at low blast radius")
	assert.Contains(t, controlIDs, model.ControlWorktreeIsolation, "domain triggers worktree-isolation even at low blast radius")

	// Provenance validation.
	for _, c := range result.ActiveControls {
		require.NoError(t, c.Validate())
	}
}

func TestDeriveControls_PreExecutionBlastRadiusFromPlannedTargetFiles(t *testing.T) {
	t.Parallel()
	// ExecutionRunVersion=0 means pre-execution: blast radius from
	// tasks.md target_files, not task run metadata.
	// Default threshold is now high (10+ files). 5 files = medium → no controls.
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain:     "",
		NeedsDiscovery:      false,
		ExecutionRunVersion: 0,
		PlannedTargetFiles:  []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
		Traceability:        model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.NotContains(t, controlIDs, model.ControlIndependentReview, "medium blast radius no longer triggers independent-review (threshold=high)")
	assert.NotContains(t, controlIDs, model.ControlWorktreeIsolation, "medium blast radius no longer triggers worktree-isolation (threshold=high)")
	assert.Equal(t, model.SignalLevelMedium, result.Summary.BlastRadius, "5 target files should yield medium blast radius")
	assert.Equal(t, "tasks_checklist.target_files", result.Observations[0].Source)
}

func TestDeriveControls_PreExecutionBlastRadiusFallsBackToPlannedTargetFiles(t *testing.T) {
	t.Parallel()
	// 5 files = medium, but default threshold is now high. Use 11 files to trigger.
	files := make([]string, 11)
	for i := range files {
		files[i] = filepath.Join("cmd", "file"+string(rune('a'+i))+".go")
	}
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain:     "",
		NeedsDiscovery:      false,
		ExecutionRunVersion: 0,
		PlannedTargetFiles:  files,
		Traceability:        model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlIndependentReview, "high blast radius triggers independent-review")
	assert.Contains(t, controlIDs, model.ControlWorktreeIsolation, "high blast radius triggers worktree-isolation")
	assert.Equal(t, model.SignalLevelHigh, result.Summary.BlastRadius)
}

func TestDeriveControls_PreExecutionIgnoresTaskRunTargetFilesWithoutFrozenRunSummary(t *testing.T) {
	t.Parallel()
	// Use 11 files to exceed the high threshold (default).
	files := make([]string, 11)
	for i := range files {
		files[i] = string(rune('a'+i)) + ".go"
	}
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain:     "",
		NeedsDiscovery:      false,
		ExecutionRunVersion: 0,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"stale.go"}},
		},
		PlannedTargetFiles: files,
		Traceability:       model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlIndependentReview, "planned target files should drive pre-execution blast radius")
	assert.Contains(t, controlIDs, model.ControlWorktreeIsolation, "planned target files should drive pre-execution blast radius")
	assert.Equal(t, model.SignalLevelHigh, result.Summary.BlastRadius, "pre-execution blast radius must ignore stale task-run target files")
	assert.Equal(t, "tasks_checklist.target_files", result.Observations[0].Source)
}

func TestDeriveControls_PostExecutionBlastRadiusFromChangedFiles(t *testing.T) {
	t.Parallel()
	// ExecutionRunVersion=1 means post-execution: blast radius from ChangedFiles.
	// 15 changed files => high blast radius.
	changedFiles := make([]string, 15)
	for i := range changedFiles {
		changedFiles[i] = filepath.Join("pkg", "file"+string(rune('a'+i))+".go")
	}
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain:     "",
		NeedsDiscovery:      false,
		ExecutionRunVersion: 1,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", RunSummaryVersion: 1, ChangedFiles: changedFiles},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlIndependentReview, "high blast radius triggers independent-review")
	assert.Contains(t, controlIDs, model.ControlWorktreeIsolation, "high blast radius triggers worktree-isolation")
	assert.Equal(t, model.SignalLevelHigh, result.Summary.BlastRadius, "15 changed files should yield high blast radius")
}

func TestDeriveControls_RollbackRequiredFromDomainMapping(t *testing.T) {
	t.Parallel()
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: model.GuardrailDomainSchemaDataMigration,
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"migrate.sql"}},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlRollbackRequired, "schema_data_migration domain triggers rollback-required")

	// Verify advisory mode.
	for _, c := range result.ActiveControls {
		if c.ControlID == model.ControlRollbackRequired {
			assert.Equal(t, model.ControlModeAdvisory, c.Mode, "rollback-required should be advisory mode")
			break
		}
	}
}

func TestDeriveControls_TraceabilityDrivenClarification(t *testing.T) {
	t.Parallel()
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "",
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"a.go"}},
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusFail,
			Gaps: []model.TraceabilityGap{
				{
					ID:       "intent-open-questions",
					Type:     "intent",
					Issue:    "blocking open questions remain unresolved",
					Blocking: true,
				},
			},
		},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlClarification, "blocking intent gap triggers clarification control")
}

func TestDeriveControls_DisabledControlOverrideWithMonotonicPreservation(t *testing.T) {
	t.Parallel()
	existing := []model.ControlActivation{
		{
			ControlID:    model.ControlDomainReview,
			Mode:         model.ControlModeBlocking,
			Scope:        model.ControlScopeReview,
			Active:       true,
			TriggeredBy:  []string{"auth_authz"},
			PolicySource: model.BuiltinPolicySource,
		},
	}

	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "auth_authz",
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"auth.go"}},
		},
		Traceability:     model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		ExistingControls: existing,
		Overrides: &ControlOverrides{
			DisabledControls: []model.ControlID{model.ControlDomainReview},
		},
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	// DisabledControls removes new candidate, but monotonic merge preserves existing.
	assert.Contains(t, controlIDs, model.ControlDomainReview,
		"existing active control persists due to monotonic rule even when disabled")

	// domain-review should NOT appear in NewActivations (it was existing, not newly added).
	var newIDs []model.ControlID
	for _, c := range result.NewActivations {
		newIDs = append(newIDs, c.ControlID)
	}
	assert.NotContains(t, newIDs, model.ControlDomainReview,
		"domain-review should not be a new activation since it came from existing controls")
}

func TestDeriveControls_ModeOverrideAdvisory(t *testing.T) {
	t.Parallel()
	// Override independent-review to advisory. It should still activate but as advisory.
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "auth_authz",
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"auth.go"}},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		Overrides: &ControlOverrides{
			ModeOverrides: map[model.ControlID]model.ControlMode{
				model.ControlIndependentReview: model.ControlModeAdvisory,
			},
		},
	})

	var found bool
	for _, c := range result.ActiveControls {
		if c.ControlID == model.ControlIndependentReview {
			assert.Equal(t, model.ControlModeAdvisory, c.Mode,
				"independent-review should be advisory when overridden")
			found = true
		}
		// Other controls should retain their defaults.
		if c.ControlID == model.ControlDomainReview {
			assert.Equal(t, model.ControlModeBlocking, c.Mode,
				"domain-review should remain blocking (not overridden)")
		}
	}
	assert.True(t, found, "independent-review should still be activated")
}

func TestDeriveControls_ModeOverrideReescalateRollback(t *testing.T) {
	t.Parallel()
	// Override rollback-required from advisory (default) to blocking.
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: model.GuardrailDomainSchemaDataMigration,
		NeedsDiscovery:  false,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"migrate.sql"}},
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		Overrides: &ControlOverrides{
			ModeOverrides: map[model.ControlID]model.ControlMode{
				model.ControlRollbackRequired: model.ControlModeBlocking,
			},
		},
	})

	for _, c := range result.ActiveControls {
		if c.ControlID == model.ControlRollbackRequired {
			assert.Equal(t, model.ControlModeBlocking, c.Mode,
				"rollback-required should be re-escalated to blocking when overridden")
			return
		}
	}
	t.Fatal("rollback-required should be activated")
}

func TestDeriveControls_NoOverridePreservesDefaults(t *testing.T) {
	t.Parallel()
	// Without any overrides, all modes match the built-in defaults.
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: model.GuardrailDomainSchemaDataMigration,
		NeedsDiscovery:  true,
		TaskResults: map[string]model.TaskRun{
			"t1": {TaskID: "t1", TargetFiles: []string{"migrate.sql"}},
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusFail,
			Gaps: []model.TraceabilityGap{
				{ID: "intent-oq", Type: "intent", Issue: "test", Blocking: true},
			},
		},
	})

	for _, c := range result.ActiveControls {
		switch c.ControlID {
		case model.ControlRollbackRequired:
			assert.Equal(t, model.ControlModeAdvisory, c.Mode, "rollback-required default is advisory")
		case model.ControlClarification, model.ControlResearch,
			model.ControlDomainReview, model.ControlIndependentReview:
			assert.Equal(t, model.ControlModeBlocking, c.Mode, "%s default is blocking", c.ControlID)
		case model.ControlWorktreeIsolation:
			assert.Equal(t, model.ControlModeAdvisory, c.Mode, "%s default is advisory", c.ControlID)
		}
	}
}

func TestDeriveControls_PostExecEmptyTaskRunsFallsBackToPlannedTargetFiles(t *testing.T) {
	t.Parallel()
	result := DeriveControls(DeriveControlsInput{
		ExecutionRunVersion: 1,
		TaskResults:         map[string]model.TaskRun{}, // empty — execution summary lost
		PlannedTargetFiles:  []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
		Traceability:        model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var blastObs *model.SignalObservation
	for _, obs := range result.Observations {
		if obs.Signal == model.SignalBlastRadius {
			blastObs = &obs
			break
		}
	}
	require.NotNil(t, blastObs, "blast radius observation must exist")
	assert.Equal(t, model.SignalLevelMedium, blastObs.Level, "5 planned files should be medium, not low")
	assert.Contains(t, blastObs.Source, "fallback")
}

func TestDeriveControls_PostExecNoDataDegradesToMedium(t *testing.T) {
	t.Parallel()
	result := DeriveControls(DeriveControlsInput{
		ExecutionRunVersion: 1,
		TaskResults:         map[string]model.TaskRun{}, // empty
		PlannedTargetFiles:  nil,                        // also empty
		Traceability:        model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	var blastObs *model.SignalObservation
	for _, obs := range result.Observations {
		if obs.Signal == model.SignalBlastRadius {
			blastObs = &obs
			break
		}
	}
	require.NotNil(t, blastObs, "blast radius observation must exist")
	assert.Equal(t, model.SignalLevelMedium, blastObs.Level, "no data at all must degrade to medium, not low")
	assert.Contains(t, blastObs.Source, "degrade_medium")
}
