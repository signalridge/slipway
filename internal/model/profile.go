package model

// QualityMode controls planning and closeout style (discuss, full).
// It must NOT drive governance posture, control modes, or auto-pass logic.
// Those responsibilities belong exclusively to WorkflowPreset.
type QualityMode string

const (
	QualityModeStandard QualityMode = "standard"
	QualityModeDiscuss  QualityMode = "discuss"
	QualityModeFull     QualityMode = "full"
)

// WorkflowPreset controls governance posture: auto-pass, plan-audit caps,
// control mode overrides, and review gates. It must NOT be conflated with
// QualityMode, which only affects planning/closeout style.
type WorkflowPreset string

const (
	WorkflowPresetLight    WorkflowPreset = "light"
	WorkflowPresetStandard WorkflowPreset = "standard"
	WorkflowPresetStrict   WorkflowPreset = "strict"
)

func (m QualityMode) IsValid() bool {
	switch m {
	case QualityModeStandard, QualityModeDiscuss, QualityModeFull:
		return true
	default:
		return false
	}
}

func (p WorkflowPreset) IsValid() bool {
	switch p {
	case WorkflowPresetLight, WorkflowPresetStandard, WorkflowPresetStrict:
		return true
	default:
		return false
	}
}

func (c Change) EffectiveQualityMode() QualityMode {
	if c.QualityMode.IsValid() {
		return c.QualityMode
	}
	return QualityModeStandard
}

func (c Change) RequiresCloseoutRefresh() bool {
	return c.EffectiveQualityMode() == QualityModeFull
}

func (c Change) WorkflowPresetConfirmationPending() bool {
	return !c.WorkflowPreset.IsValid() && c.SuggestedWorkflowPreset.IsValid()
}

// Rank returns the strictness rank of this preset (light=0, standard=1, strict=2).
// Unknown or empty presets default to standard (1).
func (p WorkflowPreset) Rank() int {
	switch p {
	case WorkflowPresetLight:
		return 0
	case WorkflowPresetStandard:
		return 1
	case WorkflowPresetStrict:
		return 2
	default:
		return 1
	}
}

func (c Change) ConfirmedWorkflowPreset() WorkflowPreset {
	if c.WorkflowPreset.IsValid() {
		return c.WorkflowPreset
	}
	if c.WorkflowPresetConfirmationPending() {
		return ""
	}
	return WorkflowPresetStandard
}
