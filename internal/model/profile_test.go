package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveQualityModeDefaultsToStandard(t *testing.T) {
	t.Parallel()

	change := Change{}

	assert.Equal(t, QualityModeStandard, change.EffectiveQualityMode())
}

func TestConfirmedWorkflowPresetDefaultsToStandard(t *testing.T) {
	t.Parallel()

	change := Change{}

	assert.Equal(t, WorkflowPresetStandard, change.ConfirmedWorkflowPreset())
	assert.False(t, change.WorkflowPresetConfirmationPending())
}

func TestWorkflowPresetConfirmationPendingRequiresSuggestionWithoutConfirmedPreset(t *testing.T) {
	t.Parallel()

	change := Change{
		SuggestedWorkflowPreset: WorkflowPresetLight,
	}

	assert.True(t, change.WorkflowPresetConfirmationPending())
	assert.Empty(t, change.WorkflowPreset)
}
