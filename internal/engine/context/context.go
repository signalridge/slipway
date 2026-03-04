package context

import (
	"fmt"
	"strings"
	"time"

	"github.com/signalridge/speclane/internal/model"
)

type LaneMode string

const (
	LaneModeAdmissionOnly LaneMode = "admission_only"
	LaneModeGoverned      LaneMode = "governed"
	LaneModeDiagnostics   LaneMode = "diagnostics"
)

type EvidenceFreshness string

const (
	EvidenceFreshnessFresh   EvidenceFreshness = "fresh"
	EvidenceFreshnessStale   EvidenceFreshness = "stale"
	EvidenceFreshnessUnknown EvidenceFreshness = "unknown"
)

type Pack struct {
	LaneMode          LaneMode                `json:"lane_mode" yaml:"lane_mode"`
	SourceStateFile   string                  `json:"source_state_file,omitempty" yaml:"source_state_file,omitempty"`
	Level             model.Level             `json:"level,omitempty" yaml:"level,omitempty"`
	LevelSource       model.LevelSource       `json:"level_source,omitempty" yaml:"level_source,omitempty"`
	CurrentState      model.WorkflowState     `json:"current_state,omitempty" yaml:"current_state,omitempty"`
	CurrentAction     string                  `json:"current_action,omitempty" yaml:"current_action,omitempty"`
	NextAction        string                  `json:"next_action,omitempty" yaml:"next_action,omitempty"`
	IntentSummary     string                  `json:"intent_summary,omitempty" yaml:"intent_summary,omitempty"`
	ScopeFiles        []string                `json:"scope_files,omitempty" yaml:"scope_files,omitempty"`
	RecentDecisions   []string                `json:"recent_decisions,omitempty" yaml:"recent_decisions,omitempty"`
	NextReadyActions  []string                `json:"next_ready_actions,omitempty" yaml:"next_ready_actions,omitempty"`
	Blockers          []string                `json:"blockers,omitempty" yaml:"blockers,omitempty"`
	EvidenceFreshness EvidenceFreshness       `json:"evidence_freshness" yaml:"evidence_freshness"`
	WaveEnvelope      *WaveEnvelope           `json:"wave_envelope,omitempty" yaml:"wave_envelope,omitempty"`
	CheckpointResume  *CheckpointResumeBundle `json:"checkpoint_resume,omitempty" yaml:"checkpoint_resume,omitempty"`
	Remediation       []string                `json:"remediation,omitempty" yaml:"remediation,omitempty"`
}

type WaveEnvelope struct {
	WaveID         string         `json:"wave_id" yaml:"wave_id"`
	TaskID         string         `json:"task_id" yaml:"task_id"`
	DependsOn      []string       `json:"depends_on" yaml:"depends_on"`
	TargetFiles    []string       `json:"target_files,omitempty" yaml:"target_files,omitempty"`
	TaskKind       model.TaskKind `json:"task_kind,omitempty" yaml:"task_kind,omitempty"`
	Autonomous     bool           `json:"autonomous" yaml:"autonomous"`
	CheckpointType string         `json:"checkpoint_type,omitempty" yaml:"checkpoint_type,omitempty"`
	MustHaves      []string       `json:"must_haves" yaml:"must_haves"`
}

type CheckpointResumeBundle struct {
	PriorRunID          string   `json:"prior_run_id" yaml:"prior_run_id"`
	PausedTaskID        string   `json:"paused_task_id" yaml:"paused_task_id"`
	CheckpointType      string   `json:"checkpoint_type" yaml:"checkpoint_type"`
	UserResponsePayload string   `json:"user_response_payload,omitempty" yaml:"user_response_payload,omitempty"`
	PauseBlockers       []string `json:"pause_blockers,omitempty" yaml:"pause_blockers,omitempty"`
}

type SubagentContext struct {
	SessionID      string   `json:"session_id" yaml:"session_id"`
	TaskID         string   `json:"task_id" yaml:"task_id"`
	TaskScopeFiles []string `json:"task_scope_files,omitempty" yaml:"task_scope_files,omitempty"`
	TechniqueHints []string `json:"technique_hints,omitempty" yaml:"technique_hints,omitempty"`
	Pack           Pack     `json:"pack" yaml:"pack"`
}

type EvidenceFreshnessInput struct {
	EvidenceInputHash      string    `json:"evidence_input_hash"`
	CurrentInputHash       string    `json:"current_input_hash"`
	EvidenceTimestamp      time.Time `json:"evidence_timestamp"`
	LatestRelevantUpdateAt time.Time `json:"latest_relevant_update_at"`
}

func BuildAdmissionPack(
	admission model.AdmissionState,
	sourceStateFile string,
	nextReadyActions []string,
	blockers []string,
	freshness EvidenceFreshness,
) Pack {
	return Pack{
		LaneMode:          LaneModeAdmissionOnly,
		SourceStateFile:   sourceStateFile,
		Level:             admission.Level,
		LevelSource:       admission.LevelSource,
		CurrentState:      admission.CurrentState,
		CurrentAction:     string(admission.CurrentState),
		NextAction:        firstOrEmpty(nextReadyActions),
		IntentSummary:     summarizeIntent(admission.IntakeAssessment),
		ScopeFiles:        clone(admission.IntakeAssessment.ChangeTargets),
		RecentDecisions:   clone(admission.RouteSnapshot.RoutingRationale),
		NextReadyActions:  clone(nextReadyActions),
		Blockers:          clone(blockers),
		EvidenceFreshness: normalizeFreshness(freshness),
	}
}

func BuildGovernedPack(
	change model.ChangeState,
	sourceStateFile string,
	nextReadyActions []string,
	blockers []string,
	intentSummary string,
	scopeFiles []string,
	freshness EvidenceFreshness,
) Pack {
	if strings.TrimSpace(intentSummary) == "" {
		intentSummary = "governed execution context"
	}
	return Pack{
		LaneMode:          LaneModeGoverned,
		SourceStateFile:   sourceStateFile,
		Level:             change.Level,
		LevelSource:       change.LevelSource,
		CurrentState:      change.CurrentState,
		CurrentAction:     string(change.CurrentState),
		NextAction:        firstOrEmpty(nextReadyActions),
		IntentSummary:     intentSummary,
		ScopeFiles:        clone(scopeFiles),
		RecentDecisions:   clone(change.RouteSnapshot.RoutingRationale),
		NextReadyActions:  clone(nextReadyActions),
		Blockers:          clone(blockers),
		EvidenceFreshness: normalizeFreshness(freshness),
	}
}

func BuildDiagnosticsPack(remediation []string) Pack {
	return Pack{
		LaneMode:          LaneModeDiagnostics,
		EvidenceFreshness: EvidenceFreshnessUnknown,
		Remediation:       clone(remediation),
	}
}

func AttachCheckpointResume(pack *Pack, bundle CheckpointResumeBundle) error {
	if pack == nil {
		return fmt.Errorf("pack is required")
	}
	pack.CheckpointResume = &bundle
	return nil
}

func BuildCheckpointResumeBundle(
	priorRunID string,
	pausedTaskID string,
	checkpointType string,
	userResponsePayload string,
	pauseBlockers []string,
) CheckpointResumeBundle {
	return CheckpointResumeBundle{
		PriorRunID:          strings.TrimSpace(priorRunID),
		PausedTaskID:        strings.TrimSpace(pausedTaskID),
		CheckpointType:      strings.TrimSpace(checkpointType),
		UserResponsePayload: strings.TrimSpace(userResponsePayload),
		PauseBlockers:       clone(pauseBlockers),
	}
}

func InjectSubagentContext(
	pack Pack,
	envelope WaveEnvelope,
	techniqueHints []string,
) (SubagentContext, error) {
	sessionID, err := model.NewRequestID()
	if err != nil {
		return SubagentContext{}, err
	}
	if strings.TrimSpace(envelope.TaskID) == "" {
		return SubagentContext{}, fmt.Errorf("wave envelope task_id is required")
	}
	if envelope.TaskKind != "" && !envelope.TaskKind.IsValid() {
		return SubagentContext{}, fmt.Errorf("invalid task_kind %q", envelope.TaskKind)
	}

	// Exclude unrelated context for subagent execution and keep only task-scoped files.
	pack.ScopeFiles = clone(envelope.TargetFiles)
	pack.WaveEnvelope = &envelope

	return SubagentContext{
		SessionID:      sessionID,
		TaskID:         envelope.TaskID,
		TaskScopeFiles: clone(envelope.TargetFiles),
		TechniqueHints: clone(techniqueHints),
		Pack:           pack,
	}, nil
}

func EvaluateEvidenceFreshness(
	hasActiveContext bool,
	inputs []EvidenceFreshnessInput,
) EvidenceFreshness {
	if !hasActiveContext {
		return EvidenceFreshnessUnknown
	}
	if len(inputs) == 0 {
		return EvidenceFreshnessUnknown
	}

	evaluated := false
	for _, item := range inputs {
		if item.EvidenceInputHash != "" && item.CurrentInputHash != "" {
			evaluated = true
			if item.EvidenceInputHash != item.CurrentInputHash {
				return EvidenceFreshnessStale
			}
		}
		if !item.EvidenceTimestamp.IsZero() && !item.LatestRelevantUpdateAt.IsZero() {
			evaluated = true
			if item.EvidenceTimestamp.Before(item.LatestRelevantUpdateAt) {
				return EvidenceFreshnessStale
			}
		}
	}
	if !evaluated {
		return EvidenceFreshnessUnknown
	}
	return EvidenceFreshnessFresh
}

func summarizeIntent(assessment model.IntakeAssessment) string {
	if strings.TrimSpace(assessment.IntendedDelta) != "" {
		return assessment.IntendedDelta
	}
	if len(assessment.ChangeTargets) > 0 {
		return "update " + strings.Join(assessment.ChangeTargets, ", ")
	}
	if strings.TrimSpace(assessment.AcceptanceAnchor) != "" {
		return assessment.AcceptanceAnchor
	}
	return "execution context"
}

func normalizeFreshness(value EvidenceFreshness) EvidenceFreshness {
	switch value {
	case EvidenceFreshnessFresh, EvidenceFreshnessStale, EvidenceFreshnessUnknown:
		return value
	default:
		return EvidenceFreshnessUnknown
	}
}

func clone(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func firstOrEmpty(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
