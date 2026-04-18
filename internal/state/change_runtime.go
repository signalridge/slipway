package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

const ChangeRuntimeStateFileName = "runtime-state.yaml"

// ChangeRuntimeStateLoadError is retained for backward compatibility with
// diagnostic/health callers that check for legacy sidecar load failures.
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

// legacyRuntimeState is the on-disk shape of the legacy runtime-state.yaml
// sidecar. Kept only for migration reads.
type legacyRuntimeState struct {
	CurrentState              model.WorkflowState            `yaml:"current_state,omitempty"`
	Status                    model.ChangeStatus             `yaml:"status,omitempty"`
	Artifacts                 map[string]model.ArtifactState `yaml:"artifacts,omitempty"`
	EvidenceRefs              map[string]string              `yaml:"evidence_refs,omitempty"`
	LastAutoPassedStates      []model.AutoPassedState        `yaml:"last_auto_passed_states,omitempty"`
	ReviewIntentDriftFailures int                            `yaml:"review_intent_drift_failures,omitempty"`
	InterruptedExecutionAt    time.Time                      `yaml:"interrupted_execution_at,omitempty"`
}

// loadLegacyRuntimeStateFromPath reads and parses a legacy runtime-state.yaml.
func loadLegacyRuntimeStateFromPath(path string) (legacyRuntimeState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return legacyRuntimeState{}, fs.ErrNotExist
		}
		return legacyRuntimeState{}, err
	}

	var runtime legacyRuntimeState
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&runtime); err != nil {
		return legacyRuntimeState{}, err
	}
	if err := runtime.validate(); err != nil {
		return legacyRuntimeState{}, err
	}
	return runtime, nil
}

func (r legacyRuntimeState) validate() error {
	for key, artifact := range r.Artifacts {
		if artifact.ID == "" {
			return fmt.Errorf("artifacts[%q] is missing id", key)
		}
		if !artifact.State.IsValid() {
			return fmt.Errorf("artifacts[%q] has invalid state: %q", key, artifact.State)
		}
	}
	for i, autoPassed := range r.LastAutoPassedStates {
		if err := autoPassed.Validate(); err != nil {
			return fmt.Errorf("last_auto_passed_states[%d]: %w", i, err)
		}
	}
	if r.ReviewIntentDriftFailures < 0 {
		return fmt.Errorf("review_intent_drift_failures must be >= 0")
	}
	return nil
}

// mergeLegacyRuntimeState checks for a legacy runtime-state.yaml sidecar and
// merges its fields into the change without deleting the sidecar file. Used on
// the load path so health diagnostics can still observe the sidecar.
func mergeLegacyRuntimeState(bundleDir string, change *model.Change) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}

	path := filepath.Join(bundleDir, ChangeRuntimeStateFileName)
	runtime, err := loadLegacyRuntimeStateFromPath(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return &ChangeRuntimeStateLoadError{Path: path, Err: err}
	}

	// Merge legacy fields into the change only if the change doesn't already
	// have them populated (the unified change.yaml is authoritative).
	if len(change.Artifacts) == 0 && len(runtime.Artifacts) > 0 {
		change.Artifacts = runtime.Artifacts
	}
	if len(change.EvidenceRefs) == 0 && len(runtime.EvidenceRefs) > 0 {
		change.EvidenceRefs = runtime.EvidenceRefs
	}
	if len(change.LastAutoPassedStates) == 0 && len(runtime.LastAutoPassedStates) > 0 {
		change.LastAutoPassedStates = append([]model.AutoPassedState(nil), runtime.LastAutoPassedStates...)
	}
	if change.ReviewIntentDriftFailures == 0 && runtime.ReviewIntentDriftFailures != 0 {
		change.ReviewIntentDriftFailures = runtime.ReviewIntentDriftFailures
	}
	if change.InterruptedExecutionAt.IsZero() && !runtime.InterruptedExecutionAt.IsZero() {
		change.InterruptedExecutionAt = runtime.InterruptedExecutionAt
	}

	change.Normalize()
	return nil
}

// deleteLegacyRuntimeState removes a legacy runtime-state.yaml if it exists.
func deleteLegacyRuntimeState(bundleDir string) error {
	path := filepath.Join(bundleDir, ChangeRuntimeStateFileName)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// legacyRuntimeStateExists reports whether a legacy sidecar file is present.
func legacyRuntimeStateExists(bundleDir string) bool {
	path := filepath.Join(bundleDir, ChangeRuntimeStateFileName)
	_, err := os.Stat(path)
	return err == nil
}
