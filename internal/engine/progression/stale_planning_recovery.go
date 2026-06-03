package progression

import (
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

const StalePlanningRecoveryTarget = "S1_PLAN/audit"

func StalePlanningRecoveryAvailable(change model.Change, blockers []model.ReasonCode) bool {
	if !stalePlanningRecoveryState(change.CurrentState) {
		return false
	}
	for _, blocker := range blockers {
		if blocker.Code == state.StalePlanningEvidenceBlockerToken {
			return true
		}
	}
	return false
}

func stalePlanningRecoveryIssueAvailable(change model.Change, issues []string) bool {
	if !stalePlanningRecoveryState(change.CurrentState) {
		return false
	}
	for _, issue := range issues {
		code, _, _ := strings.Cut(strings.TrimSpace(issue), ":")
		if code == state.StalePlanningEvidenceBlockerToken {
			return true
		}
	}
	return false
}

func stalePlanningRecoveryState(workflowState model.WorkflowState) bool {
	return workflowState == model.StateS3Review || workflowState == model.StateS4Verify
}
