package control

import "github.com/signalridge/slipway/internal/model"

// ControlOverrides holds project-level threshold and enablement overrides.
type ControlOverrides struct {
	// Threshold overrides
	IndependentReviewBlastRadius model.SignalLevel `yaml:"independent_review_blast_radius,omitempty"`
	WorktreeBlastRadius          model.SignalLevel `yaml:"worktree_blast_radius,omitempty"`

	// Enablement overrides: explicitly disable a built-in control.
	DisabledControls []model.ControlID `yaml:"disabled_controls,omitempty"`

	// ModeOverrides allows per-control mode configuration (blocking/advisory).
	// Controls not listed here use their built-in default mode.
	ModeOverrides map[model.ControlID]model.ControlMode `yaml:"mode_overrides,omitempty"`
}

// defaultControlModes returns the built-in default mode for each control.
// These defaults are the canonical source of truth and must not change
// without careful backward-compatibility consideration.
var defaultControlModes = map[model.ControlID]model.ControlMode{
	model.ControlClarification:     model.ControlModeBlocking,
	model.ControlResearch:          model.ControlModeBlocking,
	model.ControlDomainReview:      model.ControlModeBlocking,
	model.ControlIndependentReview: model.ControlModeBlocking,
	model.ControlWorktreeIsolation: model.ControlModeAdvisory,
	model.ControlRollbackRequired:  model.ControlModeAdvisory,
}

// ResolveControlMode returns the effective mode for a control, considering
// any per-control mode override. If no override is configured, the built-in
// default is returned.
func ResolveControlMode(controlID model.ControlID, overrides *ControlOverrides) model.ControlMode {
	if overrides != nil && len(overrides.ModeOverrides) > 0 {
		if mode, ok := overrides.ModeOverrides[controlID]; ok && mode.IsValid() {
			return mode
		}
	}
	if mode, ok := defaultControlModes[controlID]; ok {
		return mode
	}
	// Fail-closed: unknown controls default to blocking.
	return model.ControlModeBlocking
}
