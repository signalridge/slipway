// Package progression encapsulates state machine advancement logic for the
// slipway engine, independent of CLI framework types.
package progression

import (
	"errors"

	"github.com/signalridge/slipway/internal/model"
)

// AdvanceSummary captures what happened during a state advance attempt.
type AdvanceSummary struct {
	Action           string                  `json:"action"`
	FromState        model.WorkflowState     `json:"from_state"`
	ToState          model.WorkflowState     `json:"to_state,omitempty"`
	Message          string                  `json:"message"`
	Blockers         []model.ReasonCode      `json:"blockers,omitempty"`
	AutoPassedStates []model.AutoPassedState `json:"auto_passed_states,omitempty"`
}

// WaveSyncResult captures what happened during a wave synchronization attempt.
type WaveSyncResult struct {
	Updated  bool
	Blockers []model.ReasonCode
}

// Sentinel errors for the progression engine.
var (
	ErrGateBlocked = errors.New("gate evaluation blocked")
	ErrNoNextState = errors.New("no next state defined")
)
