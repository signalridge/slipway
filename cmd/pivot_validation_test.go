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
	assert.Contains(t, err.Error(), "pivot is allowed only in S1_PLAN reroute or S2/S3/S4")
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
