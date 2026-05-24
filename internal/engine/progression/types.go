// Package progression encapsulates state machine advancement logic for the
// slipway engine, independent of CLI framework types.
package progression

import (
	"errors"
	"time"

	"github.com/signalridge/slipway/internal/model"
)

// SideEffect records a runtime-owned mutation that occurred during advance.
type SideEffect struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// SkillEvidenceTrace records verified skill evidence consumed by a mutating
// progression step. The event log writes this as skill.evidence_recorded.
type SkillEvidenceTrace struct {
	SkillName   string    `json:"skill_name"`
	RunVersion  int       `json:"run_version"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
	References  []string  `json:"references,omitempty"`
	EvidenceRef string    `json:"evidence_ref,omitempty"`
}

// AdvanceSummary captures what happened during a state advance attempt.
// All fields are structured and machine-readable. The Message field is
// human-only and non-authoritative — JSON callers should use the structured
// fields instead.
type AdvanceSummary struct {
	Action           string                  `json:"action"`
	FromState        model.WorkflowState     `json:"from_state"`
	ToState          model.WorkflowState     `json:"to_state,omitempty"`
	FromSubStep      string                  `json:"from_substep,omitempty"`
	ToSubStep        string                  `json:"to_substep,omitempty"`
	Reason           string                  `json:"reason,omitempty"`
	RecoveryOnly     bool                    `json:"recovery_only,omitempty"`
	Signals          map[string]bool         `json:"signals,omitempty"`
	SideEffects      []SideEffect            `json:"side_effects,omitempty"`
	SkillEvidence    []SkillEvidenceTrace    `json:"skill_evidence,omitempty"`
	ClearedFields    []string                `json:"cleared_fields,omitempty"`
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
