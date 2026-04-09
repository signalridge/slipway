package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePivotPreconditionsRejectsAdmissionRescope(t *testing.T) {
	t.Parallel()

	err := validatePivotPreconditions(
		string(gate.PivotKindRescope),
		model.StateS2Execute,
	)
	require.NoError(t, err)
}

func TestValidatePivotPreconditionsRejectsNonPivotState(t *testing.T) {
	t.Parallel()

	// S1_PLAN rescope is rejected because S1_PLAN is not a pivot state for rescope
	err := validatePivotPreconditions(
		string(gate.PivotKindRescope),
		model.StateS1Plan,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pivot is allowed only in S1_PLAN reroute or S2/S3/S4")
}

func TestValidatePivotPreconditionsAllowsGovernedRescopeInS5(t *testing.T) {
	t.Parallel()

	err := validatePivotPreconditions(
		string(gate.PivotKindRescope),
		model.StateS2Execute,
	)
	require.NoError(t, err)
}

func TestValidatePivotPreconditionsAllowsGovernedRerouteInS4(t *testing.T) {
	t.Parallel()

	err := validatePivotPreconditions(
		string(gate.PivotKindReroute),
		model.StateS1Plan,
	)
	require.NoError(t, err)
}
