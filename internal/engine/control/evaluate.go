package control

import (
	"github.com/signalridge/slipway/internal/model"
)

// mergeResult holds the output of monotonic control merge.
type mergeResult struct {
	ActiveControls []model.ControlActivation
}

// mergeMonotonic ensures controls are never silently removed.
// Existing active controls are always preserved (monotonic guarantee), but
// their policy metadata (Mode, Scope, TriggeredBy, PolicySource) is
// refreshed from matching candidates so that builtin policy changes take
// effect on recompute.
func mergeMonotonic(existing, candidates []model.ControlActivation) mergeResult {
	existingSet := map[model.ControlID]model.ControlActivation{}
	for _, ctrl := range existing {
		if ctrl.Active {
			existingSet[ctrl.ControlID] = ctrl
		}
	}

	candidateSet := map[model.ControlID]model.ControlActivation{}
	for _, ctrl := range candidates {
		candidateSet[ctrl.ControlID] = ctrl
	}

	var active []model.ControlActivation

	// Keep all existing active controls, refreshing policy metadata from
	// current candidates when available.
	for _, ctrl := range existing {
		if ctrl.Active {
			if candidate, ok := candidateSet[ctrl.ControlID]; ok {
				ctrl.Mode = candidate.Mode
				ctrl.Scope = candidate.Scope
				ctrl.TriggeredBy = candidate.TriggeredBy
				ctrl.PolicySource = candidate.PolicySource
			}
			active = append(active, ctrl)
		}
	}

	// Add new candidates that weren't already active.
	for _, ctrl := range candidates {
		if _, exists := existingSet[ctrl.ControlID]; !exists {
			active = append(active, ctrl)
		}
	}

	return mergeResult{
		ActiveControls: active,
	}
}

// controlThresholds defines the trigger thresholds for each control.
type controlThresholds struct {
	independentReviewBlastRadius model.SignalLevel
	worktreeBlastRadius          model.SignalLevel
}

func defaultThresholds() controlThresholds {
	return controlThresholds{
		independentReviewBlastRadius: model.SignalLevelHigh,
		worktreeBlastRadius:          model.SignalLevelHigh,
	}
}

// filterDisabledControls removes candidates whose ControlID is in the disabled list.
func filterDisabledControls(candidates []model.ControlActivation, disabled []model.ControlID) []model.ControlActivation {
	disabledSet := map[model.ControlID]bool{}
	for _, id := range disabled {
		disabledSet[id] = true
	}
	var filtered []model.ControlActivation
	for _, c := range candidates {
		if !disabledSet[c.ControlID] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func applyOverrides(base controlThresholds, overrides ControlOverrides) controlThresholds {
	if overrides.IndependentReviewBlastRadius != "" && overrides.IndependentReviewBlastRadius.IsValid() {
		base.independentReviewBlastRadius = overrides.IndependentReviewBlastRadius
	}
	if overrides.WorktreeBlastRadius != "" && overrides.WorktreeBlastRadius.IsValid() {
		base.worktreeBlastRadius = overrides.WorktreeBlastRadius
	}
	return base
}
