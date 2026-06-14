package cmd

import (
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
)

type governanceSurfaceView struct {
	GovernanceSignals *governanceSignalView
	ActiveControls    []governanceControlView
	RequiredActions   []governanceActionView
	Blockers          []model.ReasonCode
	Diagnostics       []string
}

func buildGovernanceSurfaceViewWithReadiness(readiness progression.GovernanceReadiness) governanceSurfaceView {
	surface := governanceSurfaceView{
		RequiredActions: governanceActionViews(readiness.RequiredActions),
		Blockers:        append([]model.ReasonCode(nil), readiness.Blockers...),
		Diagnostics:     append([]string(nil), readiness.Diagnostics...),
	}
	if readiness.SignalSummary != nil {
		surface.GovernanceSignals = &governanceSignalView{
			Domains:     append([]string(nil), readiness.SignalSummary.Domains...),
			BlastRadius: string(readiness.SignalSummary.BlastRadius),
		}
	}

	for _, ctrl := range readiness.ActiveControls {
		surface.ActiveControls = append(surface.ActiveControls, governanceControlView{
			ControlID:    string(ctrl.ControlID),
			Mode:         string(ctrl.Mode),
			Scope:        string(ctrl.Scope),
			TriggeredBy:  ctrl.TriggeredBy,
			PolicySource: ctrl.PolicySource,
		})
	}
	return surface
}

func governanceActionViews(actions []governance.RequiredAction) []governanceActionView {
	if len(actions) == 0 {
		return nil
	}
	out := make([]governanceActionView, 0, len(actions))
	for _, action := range actions {
		out = append(out, governanceActionView{
			ControlID:   string(action.ControlID),
			Mode:        string(action.Mode),
			Description: action.Description,
			Satisfied:   action.Satisfied,
			SatisfiedBy: governanceActionSatisfactionViews(action.SatisfiedBy),
		})
	}
	return out
}

func governanceActionSatisfactionViews(src []governance.SatisfiedBy) []governanceActionSatisfactionView {
	if len(src) == 0 {
		return nil
	}
	out := make([]governanceActionSatisfactionView, 0, len(src))
	for _, item := range src {
		out = append(out, governanceActionSatisfactionView{
			Kind:        item.Kind,
			Name:        item.Name,
			EvidenceRef: item.EvidenceRef,
			Reason:      item.Reason,
		})
	}
	return out
}

func applyGovernanceSurfaceToStatus(readiness progression.GovernanceReadiness, view *statusView) {
	surface := buildGovernanceSurfaceViewWithReadiness(readiness)
	view.GovernanceSignals = surface.GovernanceSignals
	view.ActiveControls = surface.ActiveControls
	view.RequiredActions = surface.RequiredActions
}

func applyGovernanceSurfaceToValidate(readiness progression.GovernanceReadiness, view *validateView) {
	surface := buildGovernanceSurfaceViewWithReadiness(readiness)
	view.RequiredActions = surface.RequiredActions
}

func applyGovernanceSurfaceToNext(readiness progression.GovernanceReadiness, view *nextView) {
	surface := buildGovernanceSurfaceViewWithReadiness(readiness)
	view.GovernanceSignals = surface.GovernanceSignals
	view.ActiveControls = surface.ActiveControls
	view.RequiredActions = surface.RequiredActions
}
