package cmd

import (
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/model"
)

func validatePivotPreconditions(kind string, currentState model.WorkflowState) error {
	if kind == string(gate.PivotKindReroute) && currentState == model.StateS1Plan {
		return nil
	}

	if kind == string(gate.PivotKindRescope) && !isPivotState(currentState) {
		return newGovernanceBlockedError(
			"rescope_state_invalid",
			"rescope requires governed S2_EXECUTE, S3_REVIEW, or S4_VERIFY",
			"Rescope is available once wave execution has begun (S2_EXECUTE, S3_REVIEW, or S4_VERIFY); it resets the change to S0_INTAKE for re-planning. Before execution, use governed reroute from S1_PLAN.",
			"",
			nil,
		)
	}

	if !isPivotState(currentState) {
		remediation := "Advance change to S2_EXECUTE or later before pivoting, or use governed reroute from S1_PLAN."
		return newGovernanceBlockedError(
			"pivot_state_invalid",
			"pivot is allowed only in S1_PLAN reroute or S2/S3/S4",
			remediation,
			"",
			nil,
		)
	}

	return nil
}

func isPivotState(stateID model.WorkflowState) bool {
	return stateID == model.StateS2Execute ||
		stateID == model.StateS3Review ||
		stateID == model.StateS4Verify
}
