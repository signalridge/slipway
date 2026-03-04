package model

import (
	"fmt"
	"time"
)

type AdmissionState struct {
	RequestID                     string              `yaml:"request_id" json:"request_id"`
	AdmissionStatus               AdmissionStatus     `yaml:"admission_status" json:"admission_status"`
	CurrentState                  WorkflowState       `yaml:"current_state" json:"current_state"`
	Level                         Level               `yaml:"level,omitempty" json:"level,omitempty"`
	LevelSource                   LevelSource         `yaml:"level_source,omitempty" json:"level_source,omitempty"`
	LevelHistory                  []LevelHistoryEvent `yaml:"level_history" json:"level_history"`
	LastLevelUpdateAt             *time.Time          `yaml:"last_level_update_at,omitempty" json:"last_level_update_at,omitempty"`
	RouteSnapshot                 RouteSnapshot       `yaml:"route_snapshot" json:"route_snapshot"`
	IntakeAssessment              IntakeAssessment    `yaml:"intake_assessment" json:"intake_assessment"`
	LatestFrozenRunSummaryVersion int                 `yaml:"latest_frozen_run_summary_version" json:"latest_frozen_run_summary_version"`
	TaskRuns                      map[string]TaskRun  `yaml:"task_runs,omitempty" json:"task_runs,omitempty"`
	EvidenceRefs                  map[string]string   `yaml:"evidence_refs" json:"evidence_refs"`
	ActionHistory                 []ActionEvent       `yaml:"action_history,omitempty" json:"action_history,omitempty"`
	SealedAt                      *time.Time          `yaml:"sealed_at,omitempty" json:"sealed_at,omitempty"`
}

func NewAdmissionState(requestID string) AdmissionState {
	return AdmissionState{
		RequestID:                     requestID,
		AdmissionStatus:               AdmissionStatusActive,
		CurrentState:                  StateS0Intake,
		LevelHistory:                  []LevelHistoryEvent{},
		LatestFrozenRunSummaryVersion: 0,
		TaskRuns:                      map[string]TaskRun{},
		EvidenceRefs:                  map[string]string{},
	}
}

func (s *AdmissionState) Normalize(maxLevelHistoryEntries int) {
	if s.LevelHistory == nil {
		s.LevelHistory = []LevelHistoryEvent{}
	}
	if s.EvidenceRefs == nil {
		s.EvidenceRefs = map[string]string{}
	}
	if s.TaskRuns == nil {
		s.TaskRuns = map[string]TaskRun{}
	}
	s.LevelHistory = TruncateLevelHistory(s.LevelHistory, maxLevelHistoryEntries)
}

func (s AdmissionState) Validate() error {
	if !IsUUIDv7(s.RequestID) {
		return fmt.Errorf("request_id must be UUIDv7: %q", s.RequestID)
	}
	if !s.AdmissionStatus.IsValid() {
		return fmt.Errorf("invalid admission_status: %q", s.AdmissionStatus)
	}
	if s.Level != "" && !s.Level.IsValid() {
		return fmt.Errorf("invalid level: %q", s.Level)
	}
	if s.LevelSource != "" && !s.LevelSource.IsValid() {
		return fmt.Errorf("invalid level_source: %q", s.LevelSource)
	}
	if s.LatestFrozenRunSummaryVersion < 0 {
		return fmt.Errorf("latest_frozen_run_summary_version must be >= 0: %d", s.LatestFrozenRunSummaryVersion)
	}
	if s.IntakeAssessment.IsExecutable {
		emptyScores := s.RouteSnapshot.Scores == (Scores{})
		if emptyScores &&
			s.RouteSnapshot.GuardrailDomain == "" &&
			len(s.RouteSnapshot.RoutingRationale) == 0 &&
			len(s.RouteSnapshot.BlockingConflicts) == 0 {
			return fmt.Errorf("route_snapshot is required for executable requests")
		}
	}
	if err := s.RouteSnapshot.Scores.Validate(); err != nil {
		return fmt.Errorf("route_snapshot.scores invalid: %w", err)
	}
	if err := ValidateTaskRunMap(s.TaskRuns); err != nil {
		return err
	}
	if err := validateEvidenceOwnership(s.TaskRuns, s.EvidenceRefs); err != nil {
		return err
	}
	return nil
}

func (s AdmissionState) MarshalYAML() (interface{}, error) {
	normalized := s
	normalized.Normalize(0)
	type alias AdmissionState
	return alias(normalized), nil
}

func (s *AdmissionState) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type alias AdmissionState
	var parsed alias
	if err := unmarshal(&parsed); err != nil {
		return err
	}
	*s = AdmissionState(parsed)
	s.Normalize(0)
	return nil
}
