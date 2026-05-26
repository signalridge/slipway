package cmd

import (
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

type governanceForecastView struct {
	Authoritative   bool     `json:"authoritative"`
	ConfirmedPreset string   `json:"confirmed_preset,omitempty"`
	EffectivePreset string   `json:"effective_preset,omitempty"`
	DownstreamLevel string   `json:"downstream_level,omitempty"`
	Reasons         []string `json:"reasons,omitempty"`
}

type workflowPresetView struct {
	WorkflowPreset            string                  `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset   string                  `json:"suggested_workflow_preset,omitempty"`
	EffectiveWorkflowPreset   string                  `json:"effective_workflow_preset,omitempty"`
	PresetConfirmationPending bool                    `json:"preset_confirmation_pending,omitempty"`
	PresetUpgradeReasons      []string                `json:"preset_upgrade_reasons,omitempty"`
	GovernanceForecast        *governanceForecastView `json:"governance_forecast,omitempty"`
}

func buildWorkflowPresetView(root string, change model.Change) (workflowPresetView, error) {
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return workflowPresetView{}, err
	}

	view := workflowPresetView{
		WorkflowPreset:            string(policy.ConfirmedPreset),
		SuggestedWorkflowPreset:   string(policy.SuggestedPreset),
		EffectiveWorkflowPreset:   string(policy.EffectivePreset),
		PresetConfirmationPending: policy.PendingConfirmation,
		PresetUpgradeReasons:      append([]string(nil), policy.UpgradeReasons...),
	}

	if change.CurrentState != model.StateS1Plan {
		return view, nil
	}

	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return view, nil
	}
	snap, err := governance.PreviewGovernanceSnapshot(root, change, paths.GovernedBundleDir)
	if err != nil {
		return view, nil
	}

	reasons := make([]string, 0, 4)
	if len(snap.Summary.Domains) > 0 {
		reasons = append(reasons, "guardrail_domain="+strings.Join(snap.Summary.Domains, ","))
	}
	reasons = append(reasons, "blast_radius="+string(snap.Summary.BlastRadius))
	if change.NeedsDiscovery {
		reasons = append(reasons, "needs_discovery=true")
	}
	reasons = append(reasons, policy.UpgradeReasons...)

	view.GovernanceForecast = &governanceForecastView{
		Authoritative:   false,
		ConfirmedPreset: string(policy.ConfirmedPreset),
		EffectivePreset: string(policy.EffectivePreset),
		DownstreamLevel: forecastDownstreamLevel(policy, snap.ActiveControls),
		Reasons:         reasons,
	}
	return view, nil
}

func forecastDownstreamLevel(policy governance.PresetPolicy, controls []model.ControlActivation) string {
	// Start from effective_preset as a floor. The forecast must never
	// contradict the effective preset — if a fail-closed upgrade moved the
	// change upward, the downstream level must reflect that.
	level := policy.EffectivePreset
	if !level.IsValid() {
		level = model.WorkflowPresetStandard
	}

	// CloseoutRefreshRequired alone (e.g. via --full) does NOT escalate the
	// forecast to strict. Closeout strictness is a separate dimension from
	// overall governance posture. Only escalate when the effective preset
	// itself is strict (which already sets level = strict as the floor above).
	for _, ctrl := range controls {
		if !ctrl.Active || ctrl.Mode != model.ControlModeBlocking {
			continue
		}
		switch ctrl.Scope {
		case model.ControlScopeExecution, model.ControlScopeReview, model.ControlScopeRelease:
			if model.WorkflowPresetStandard.Rank() > level.Rank() {
				level = model.WorkflowPresetStandard
			}
		}
	}
	return string(level)
}

func renderWorkflowPresetLines(fields workflowPresetView) []string {
	lines := make([]string, 0, 4)
	switch {
	case fields.PresetConfirmationPending:
		lines = append(lines, "Preset: pending confirmation")
		if fields.SuggestedWorkflowPreset != "" {
			lines = append(lines, fmt.Sprintf("AI Suggestion: %s", fields.SuggestedWorkflowPreset))
		}
		lines = append(lines, "Next: run `slipway preset <light|standard|strict>` before continuing artifact authoring")
	default:
		if fields.WorkflowPreset != "" {
			lines = append(lines, fmt.Sprintf("Preset: %s (confirmed)", fields.WorkflowPreset))
		}
		if fields.EffectiveWorkflowPreset != "" && fields.EffectiveWorkflowPreset != fields.WorkflowPreset {
			lines = append(lines, fmt.Sprintf("Effective: %s", fields.EffectiveWorkflowPreset))
		}
	}
	if len(fields.PresetUpgradeReasons) > 0 {
		lines = append(lines, fmt.Sprintf("Policy: %s", strings.Join(fields.PresetUpgradeReasons, ", ")))
	}
	if fields.GovernanceForecast != nil {
		lines = append(lines, fmt.Sprintf("Forecast: %s (downstream)", fields.GovernanceForecast.DownstreamLevel))
	}
	return lines
}
