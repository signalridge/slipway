package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestForecastDownstreamLevel_StandardFullDoesNotEscalateToStrict(t *testing.T) {
	t.Parallel()

	// standard --full: closeout is required but the effective preset is
	// standard. The forecast must stay "standard", not escalate to "strict".
	// CloseoutRefreshRequired is a separate dimension from governance posture.
	policy := governance.PresetPolicy{
		EffectivePreset:         model.WorkflowPresetStandard,
		CloseoutRefreshRequired: true,
	}

	level := forecastDownstreamLevel(policy, nil)
	assert.Equal(t, "standard", level,
		"standard --full must forecast standard, not strict")
}

func TestForecastDownstreamLevel_LightFullDoesNotEscalateToStrict(t *testing.T) {
	t.Parallel()

	// light --full: lighter governance with closeout required.
	policy := governance.PresetPolicy{
		EffectivePreset:         model.WorkflowPresetLight,
		CloseoutRefreshRequired: true,
	}

	level := forecastDownstreamLevel(policy, nil)
	assert.Equal(t, "light", level,
		"light --full must forecast light, not strict")
}

func TestForecastDownstreamLevel_StrictAlwaysForecastsStrict(t *testing.T) {
	t.Parallel()

	policy := governance.PresetPolicy{
		EffectivePreset:         model.WorkflowPresetStrict,
		CloseoutRefreshRequired: true,
	}

	level := forecastDownstreamLevel(policy, nil)
	assert.Equal(t, "strict", level)
}

func TestForecastDownstreamLevel_BlockingReviewEscalatesLightToStandard(t *testing.T) {
	t.Parallel()

	policy := governance.PresetPolicy{
		EffectivePreset: model.WorkflowPresetLight,
	}
	controls := []model.ControlActivation{
		{
			ControlID: model.ControlIndependentReview,
			Mode:      model.ControlModeBlocking,
			Scope:     model.ControlScopeReview,
			Active:    true,
		},
	}

	level := forecastDownstreamLevel(policy, controls)
	assert.Equal(t, "standard", level,
		"blocking review obligation should escalate light to standard")
}

func TestForecastDownstreamLevel_AdvisoryControlDoesNotEscalate(t *testing.T) {
	t.Parallel()

	policy := governance.PresetPolicy{
		EffectivePreset: model.WorkflowPresetLight,
	}
	controls := []model.ControlActivation{
		{
			ControlID: model.ControlIndependentReview,
			Mode:      model.ControlModeAdvisory,
			Scope:     model.ControlScopeReview,
			Active:    true,
		},
	}

	level := forecastDownstreamLevel(policy, controls)
	assert.Equal(t, "light", level,
		"advisory controls should not escalate forecast")
}
