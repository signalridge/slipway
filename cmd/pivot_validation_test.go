package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePivotPreconditionsAllowsRescopeFromS2ThroughS4(t *testing.T) {
	t.Parallel()

	for _, state := range []model.WorkflowState{
		model.StateS2Execute,
		model.StateS3Review,
		model.StateS4Verify,
	} {
		err := validatePivotPreconditions(
			string(gate.PivotKindRescope),
			state,
		)
		require.NoError(t, err, "state %s", state)
	}
}

func TestValidatePivotPreconditionsRejectsRescopeBeforeExecution(t *testing.T) {
	t.Parallel()

	for _, state := range []model.WorkflowState{model.StateS0Intake, model.StateS1Plan} {
		err := validatePivotPreconditions(
			string(gate.PivotKindRescope),
			state,
		)
		require.Error(t, err, "state %s", state)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "rescope_state_invalid", cliErr.ErrorCode)
		assert.Contains(t, err.Error(), "rescope requires governed S2_EXECUTE, S3_REVIEW, or S4_VERIFY")
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
