package model

import (
	"fmt"
	"time"
)

type ChangeState struct {
	RequestID                     string                   `yaml:"request_id" json:"request_id"`
	Slug                          string                   `yaml:"slug" json:"slug"`
	ChangeStatus                  ChangeStatus             `yaml:"change_status" json:"change_status"`
	CurrentState                  WorkflowState            `yaml:"current_state" json:"current_state"`
	Level                         Level                    `yaml:"level,omitempty" json:"level,omitempty"`
	LevelSource                   LevelSource              `yaml:"level_source,omitempty" json:"level_source,omitempty"`
	LevelHistory                  []LevelHistoryEvent      `yaml:"level_history" json:"level_history"`
	LastLevelUpdateAt             *time.Time               `yaml:"last_level_update_at,omitempty" json:"last_level_update_at,omitempty"`
	RouteSnapshot                 RouteSnapshot            `yaml:"route_snapshot" json:"route_snapshot"`
	LatestFrozenRunSummaryVersion int                      `yaml:"latest_frozen_run_summary_version" json:"latest_frozen_run_summary_version"`
	WorktreePath                  string                   `yaml:"worktree_path,omitempty" json:"worktree_path,omitempty"`
	WorktreeBranch                string                   `yaml:"worktree_branch,omitempty" json:"worktree_branch,omitempty"`
	Gates                         map[string]GateRecord    `yaml:"gates,omitempty" json:"gates,omitempty"`
	Artifacts                     map[string]ArtifactState `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	TaskRuns                      map[string]TaskRun       `yaml:"task_runs,omitempty" json:"task_runs,omitempty"`
	EvidenceRefs                  map[string]string        `yaml:"evidence_refs" json:"evidence_refs"`
	ActionHistory                 []ActionEvent            `yaml:"action_history,omitempty" json:"action_history,omitempty"`
}

func NewChangeState(requestID, slug string) ChangeState {
	return ChangeState{
		RequestID:                     requestID,
		Slug:                          slug,
		ChangeStatus:                  ChangeStatusActive,
		CurrentState:                  StateS4SpecBundle,
		LevelHistory:                  []LevelHistoryEvent{},
		LatestFrozenRunSummaryVersion: 0,
		Gates:                         map[string]GateRecord{},
		Artifacts:                     map[string]ArtifactState{},
		TaskRuns:                      map[string]TaskRun{},
		EvidenceRefs:                  map[string]string{},
	}
}

func (s *ChangeState) Normalize(maxLevelHistoryEntries int) {
	if s.LevelHistory == nil {
		s.LevelHistory = []LevelHistoryEvent{}
	}
	if s.EvidenceRefs == nil {
		s.EvidenceRefs = map[string]string{}
	}
	if s.TaskRuns == nil {
		s.TaskRuns = map[string]TaskRun{}
	}
	if s.Gates == nil {
		s.Gates = map[string]GateRecord{}
	}
	if s.Artifacts == nil {
		s.Artifacts = map[string]ArtifactState{}
	}
	s.LevelHistory = TruncateLevelHistory(s.LevelHistory, maxLevelHistoryEntries)
}

func (s ChangeState) Validate() error {
	if !IsUUIDv7(s.RequestID) {
		return fmt.Errorf("request_id must be UUIDv7: %q", s.RequestID)
	}
	if s.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if !s.ChangeStatus.IsValid() {
		return fmt.Errorf("invalid change_status: %q", s.ChangeStatus)
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
	if err := s.RouteSnapshot.Scores.Validate(); err != nil {
		return fmt.Errorf("route_snapshot.scores invalid: %w", err)
	}
	for key, gate := range s.Gates {
		if gate.GateID == "" {
			return fmt.Errorf("gates[%q] is missing gate_id", key)
		}
		if !gate.Status.IsValid() {
			return fmt.Errorf("gates[%q] has invalid status: %q", key, gate.Status)
		}
		if gate.Decision != "" && !gate.Decision.IsValid() {
			return fmt.Errorf("gates[%q] has invalid decision: %q", key, gate.Decision)
		}
	}
	if err := ValidateTaskRunMap(s.TaskRuns); err != nil {
		return err
	}
	if err := validateEvidenceOwnership(s.TaskRuns, s.EvidenceRefs); err != nil {
		return err
	}
	return nil
}

func (s ChangeState) MarshalYAML() (interface{}, error) {
	normalized := s
	normalized.Normalize(0)
	type alias ChangeState
	return alias(normalized), nil
}

func (s *ChangeState) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type alias ChangeState
	var parsed alias
	if err := unmarshal(&parsed); err != nil {
		return err
	}
	*s = ChangeState(parsed)
	s.Normalize(0)
	return nil
}

func validateEvidenceOwnership(taskRuns map[string]TaskRun, evidenceRefs map[string]string) error {
	if evidenceRefs == nil {
		return nil
	}

	taskEvidence := map[string]struct{}{}
	for key, run := range taskRuns {
		if run.EvidenceRef == "" {
			continue
		}
		taskEvidence[run.EvidenceRef] = struct{}{}
		if _, exists := evidenceRefs[run.EvidenceRef]; exists {
			return fmt.Errorf(
				"evidence ownership conflict: task_runs[%q].evidence_ref=%q duplicated in evidence_refs key",
				key,
				run.EvidenceRef,
			)
		}
	}
	for _, ref := range evidenceRefs {
		if _, exists := taskEvidence[ref]; exists {
			return fmt.Errorf("evidence ownership conflict: task evidence_ref %q duplicated in evidence_refs values", ref)
		}
	}
	return nil
}
