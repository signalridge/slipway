package control

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestResolveControlModeNoOverrides(t *testing.T) {
	t.Parallel()
	// Without overrides, built-in defaults apply.
	assert.Equal(t, model.ControlModeBlocking, ResolveControlMode(model.ControlClarification, nil))
	assert.Equal(t, model.ControlModeBlocking, ResolveControlMode(model.ControlResearch, nil))
	assert.Equal(t, model.ControlModeBlocking, ResolveControlMode(model.ControlDomainReview, nil))
	assert.Equal(t, model.ControlModeBlocking, ResolveControlMode(model.ControlIndependentReview, nil))
	assert.Equal(t, model.ControlModeAdvisory, ResolveControlMode(model.ControlWorktreeIsolation, nil))
	assert.Equal(t, model.ControlModeAdvisory, ResolveControlMode(model.ControlRollbackRequired, nil))
}

func TestResolveControlModeWithOverride(t *testing.T) {
	t.Parallel()
	overrides := &ControlOverrides{
		ModeOverrides: map[model.ControlID]model.ControlMode{
			model.ControlIndependentReview: model.ControlModeAdvisory,
		},
	}
	// Overridden control returns advisory.
	assert.Equal(t, model.ControlModeAdvisory, ResolveControlMode(model.ControlIndependentReview, overrides))
	// Non-overridden control returns default.
	assert.Equal(t, model.ControlModeBlocking, ResolveControlMode(model.ControlDomainReview, overrides))
	assert.Equal(t, model.ControlModeAdvisory, ResolveControlMode(model.ControlRollbackRequired, overrides))
}

func TestResolveControlModeReescalateAdvisoryToBlocking(t *testing.T) {
	t.Parallel()
	overrides := &ControlOverrides{
		ModeOverrides: map[model.ControlID]model.ControlMode{
			model.ControlRollbackRequired: model.ControlModeBlocking,
		},
	}
	assert.Equal(t, model.ControlModeBlocking, ResolveControlMode(model.ControlRollbackRequired, overrides))
}

func TestApplyOverridesSkipsInvalidButValidTakesEffect(t *testing.T) {
	t.Parallel()
	base := defaultThresholds()
	overrides := ControlOverrides{
		IndependentReviewBlastRadius: model.SignalLevelMedium,
		WorktreeBlastRadius:          "severe", // invalid
	}
	result := applyOverrides(base, overrides)
	assert.Equal(t, model.SignalLevelMedium, result.independentReviewBlastRadius,
		"valid override should take effect")
	assert.Equal(t, model.SignalLevelHigh, result.worktreeBlastRadius,
		"invalid override should fall back to default")
}

func TestDeriveControlsWithThresholdOverride(t *testing.T) {
	t.Parallel()
	// Default threshold is high. Override to medium. 5 files = medium → should trigger.
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain:    "",
		PlannedTargetFiles: []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
		Traceability:       model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		Overrides: &ControlOverrides{
			IndependentReviewBlastRadius: model.SignalLevelMedium,
			WorktreeBlastRadius:          model.SignalLevelMedium,
		},
		PolicySource: model.OverridePolicySource,
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlIndependentReview,
		"medium threshold + medium blast radius should trigger independent-review")
	assert.Contains(t, controlIDs, model.ControlWorktreeIsolation,
		"medium threshold + medium blast radius should trigger worktree-isolation")
}

func TestDeriveControlsWithOverrides(t *testing.T) {
	t.Parallel()
	// Default: independent-review triggers on blast_radius=high+. Override threshold to medium.
	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain: "",
		PlannedTargetFiles: []string{
			"a.go", "b.go", "c.go", "d.go", "e.go",
			"f.go", "g.go", "h.go", "i.go", "j.go", "k.go",
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		Overrides: &ControlOverrides{
			IndependentReviewBlastRadius: model.SignalLevelMedium,
		},
		PolicySource: model.OverridePolicySource,
	})

	var controlIDs []model.ControlID
	for _, c := range result.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlIndependentReview)

	// Verify policy source propagation.
	for _, c := range result.ActiveControls {
		if c.ControlID == model.ControlIndependentReview {
			assert.Equal(t, model.OverridePolicySource, c.PolicySource)
		}
	}
}
