package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePivotPreconditionsAllowsRescopeOnlyAtExecute(t *testing.T) {
	t.Parallel()

	err := validatePivotPreconditions(
		string(gate.PivotKindRescope),
		model.StateS2Execute,
	)
	require.NoError(t, err)
}

func TestValidatePivotPreconditionsRejectsNonPivotState(t *testing.T) {
	t.Parallel()

	err := validatePivotPreconditions(
		string(gate.PivotKindRescope),
		model.StateS1Plan,
	)
	require.Error(t, err)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "rescope_state_invalid", cliErr.ErrorCode)
	assert.Contains(t, err.Error(), "rescope requires governed S2_EXECUTE")
}

func TestValidatePivotPreconditionsRejectsRescopeOutsideExecute(t *testing.T) {
	t.Parallel()

	for _, state := range []model.WorkflowState{model.StateS3Review, model.StateS4Verify} {
		err := validatePivotPreconditions(
			string(gate.PivotKindRescope),
			state,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rescope requires governed S2_EXECUTE")
	}
}

func TestValidatePivotPreconditionsAllowsGovernedRerouteInS1ThroughS4(t *testing.T) {
	t.Parallel()

	for _, state := range []model.WorkflowState{
		model.StateS1Plan,
		model.StateS2Execute,
		model.StateS3Review,
		model.StateS4Verify,
	} {
		err := validatePivotPreconditions(
			string(gate.PivotKindReroute),
			state,
		)
		require.NoError(t, err, "state %s", state)
	}
}
