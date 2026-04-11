package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

const ChangeRuntimeStateFileName = "runtime-state.yaml"

var writeRuntimeStateFileAtomic = fsutil.WriteFileAtomic

type ChangeRuntimeStateLoadError struct {
	Path string
	Err  error
}

func (e *ChangeRuntimeStateLoadError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path == "" {
		return fmt.Sprintf("load change runtime state: %v", e.Err)
	}
	return fmt.Sprintf("load change runtime state %q: %v", e.Path, e.Err)
}

func (e *ChangeRuntimeStateLoadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type ChangeRuntimeState struct {
	CurrentState              model.WorkflowState            `yaml:"current_state,omitempty"`
	Status                    model.ChangeStatus             `yaml:"status,omitempty"`
	Artifacts                 map[string]model.ArtifactState `yaml:"artifacts,omitempty"`
	EvidenceRefs              map[string]string              `yaml:"evidence_refs,omitempty"`
	LastAutoPassedStates      []model.AutoPassedState        `yaml:"last_auto_passed_states,omitempty"`
	ReviewIntentDriftFailures int                            `yaml:"review_intent_drift_failures,omitempty"`
	InterruptedExecutionAt    time.Time                      `yaml:"interrupted_execution_at,omitempty"`
}

func (s *ChangeRuntimeState) normalize() {
	if s.Artifacts == nil {
		s.Artifacts = map[string]model.ArtifactState{}
	}
	if s.EvidenceRefs == nil {
		s.EvidenceRefs = map[string]string{}
	}
	if !s.InterruptedExecutionAt.IsZero() {
		s.InterruptedExecutionAt = s.InterruptedExecutionAt.Round(0).UTC()
	}
}

func (s ChangeRuntimeState) empty() bool {
	return len(s.Artifacts) == 0 &&
		len(s.EvidenceRefs) == 0 &&
		len(s.LastAutoPassedStates) == 0 &&
		s.ReviewIntentDriftFailures == 0 &&
		s.InterruptedExecutionAt.IsZero()
}

func (s ChangeRuntimeState) Validate() error {
	if s.CurrentState != "" && !s.CurrentState.IsValid() {
		return fmt.Errorf("current_state has invalid value %q", s.CurrentState)
	}
	if s.Status != "" && !s.Status.IsValid() {
		return fmt.Errorf("status has invalid value %q", s.Status)
	}
	for key, artifact := range s.Artifacts {
		if artifact.ID == "" {
			return fmt.Errorf("artifacts[%q] is missing id", key)
		}
		if !artifact.State.IsValid() {
			return fmt.Errorf("artifacts[%q] has invalid state: %q", key, artifact.State)
		}
	}
	for i, autoPassed := range s.LastAutoPassedStates {
		if err := autoPassed.Validate(); err != nil {
			return fmt.Errorf("last_auto_passed_states[%d]: %w", i, err)
		}
	}
	if s.ReviewIntentDriftFailures < 0 {
		return fmt.Errorf("review_intent_drift_failures must be >= 0")
	}
	return nil
}

func buildChangeRuntimeState(change model.Change) (ChangeRuntimeState, error) {
	runtime := ChangeRuntimeState{
		CurrentState:              change.CurrentState,
		Status:                    change.Status,
		Artifacts:                 change.Artifacts,
		EvidenceRefs:              change.EvidenceRefs,
		LastAutoPassedStates:      append([]model.AutoPassedState(nil), change.LastAutoPassedStates...),
		ReviewIntentDriftFailures: change.ReviewIntentDriftFailures,
		InterruptedExecutionAt:    change.InterruptedExecutionAt,
	}
	runtime.normalize()
	if err := runtime.Validate(); err != nil {
		return ChangeRuntimeState{}, err
	}
	return runtime, nil
}

func marshalChangeRuntimeState(change model.Change) ([]byte, bool, error) {
	runtime, err := buildChangeRuntimeState(change)
	if err != nil {
		return nil, false, err
	}
	if runtime.empty() {
		return nil, true, nil
	}
	raw, err := yaml.Marshal(runtime)
	if err != nil {
		return nil, false, err
	}
	return raw, false, nil
}

func loadChangeRuntimeStateFromPath(path string) (ChangeRuntimeState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			state := ChangeRuntimeState{}
			state.normalize()
			return state, nil
		}
		return ChangeRuntimeState{}, err
	}

	var runtime ChangeRuntimeState
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&runtime); err != nil {
		return ChangeRuntimeState{}, err
	}
	runtime.normalize()
	if err := runtime.Validate(); err != nil {
		return ChangeRuntimeState{}, err
	}
	return runtime, nil
}

func saveChangeRuntimeStateToBundleDir(bundleDir string, change model.Change) error {
	path := filepath.Join(bundleDir, ChangeRuntimeStateFileName)
	raw, empty, err := marshalChangeRuntimeState(change)
	if err != nil {
		return err
	}
	return persistPreparedRuntimeState(path, raw, empty)
}

func loadAndApplyChangeRuntimeState(bundleDir string, change *model.Change) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}

	path := filepath.Join(bundleDir, ChangeRuntimeStateFileName)
	runtime, err := loadChangeRuntimeStateFromPath(path)
	if err != nil {
		return &ChangeRuntimeStateLoadError{Path: path, Err: err}
	}
	change.Artifacts = runtime.Artifacts
	change.EvidenceRefs = runtime.EvidenceRefs
	change.LastAutoPassedStates = append([]model.AutoPassedState(nil), runtime.LastAutoPassedStates...)
	change.ReviewIntentDriftFailures = runtime.ReviewIntentDriftFailures
	change.InterruptedExecutionAt = runtime.InterruptedExecutionAt
	change.Normalize()
	return change.Validate()
}
