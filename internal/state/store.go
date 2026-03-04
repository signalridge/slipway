package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/signalridge/speclane/internal/fsutil"
	"github.com/signalridge/speclane/internal/model"
	"gopkg.in/yaml.v3"
)

type Lane string

const (
	LaneAdmission Lane = "admission"
	LaneChange    Lane = "change"
)

type ActiveResolutionMode string

const (
	ActiveResolutionModeAdmissionOnly ActiveResolutionMode = "admission_only"
	ActiveResolutionModeGoverned      ActiveResolutionMode = "governed"
)

type ActiveRecord struct {
	RequestID string `json:"request_id" yaml:"request_id"`
	Lane      Lane   `json:"lane" yaml:"lane"`
	Path      string `json:"path" yaml:"path"`
}

type ActiveResolution struct {
	RequestID string               `json:"request_id" yaml:"request_id"`
	Mode      ActiveResolutionMode `json:"mode" yaml:"mode"`
	Record    ActiveRecord         `json:"record" yaml:"record"`
}

var (
	ErrNoActiveRequest          = errors.New("no active request")
	ErrMultipleActiveRequests   = errors.New("multiple active requests")
	ErrSameRequestDualActive    = errors.New("same-request dual-active handoff fault")
	ErrSealedAdmissionImmutable = errors.New("sealed admission snapshot is immutable")
)

func AdmissionsDir(root string) string {
	return filepath.Join(root, ".spln", "runtime", "admissions")
}

func ChangesDir(root string) string {
	return filepath.Join(root, ".spln", "runtime", "changes")
}

func AdmissionPath(root, requestID string) string {
	return filepath.Join(AdmissionsDir(root), requestID+".yaml")
}

func ChangePath(root, requestID string) string {
	return filepath.Join(ChangesDir(root), requestID+".yaml")
}

func LoadAdmission(root, requestID string) (model.AdmissionState, error) {
	b, err := os.ReadFile(AdmissionPath(root, requestID))
	if err != nil {
		return model.AdmissionState{}, err
	}
	var st model.AdmissionState
	if err := yaml.Unmarshal(b, &st); err != nil {
		return model.AdmissionState{}, err
	}
	st.Normalize(model.DefaultConfig().Execution.MaxLevelHistoryEntries)
	if err := st.Validate(); err != nil {
		return model.AdmissionState{}, err
	}
	return st, nil
}

func SaveAdmission(root string, st model.AdmissionState) error {
	if st.RequestID == "" {
		return errors.New("request_id is required")
	}
	path := AdmissionPath(root, st.RequestID)

	if existing, err := LoadAdmission(root, st.RequestID); err == nil {
		if existing.AdmissionStatus == model.AdmissionStatusSealedHandoff {
			incoming := st
			incoming.Normalize(model.DefaultConfig().Execution.MaxLevelHistoryEntries)
			if !reflect.DeepEqual(existing, incoming) {
				return ErrSealedAdmissionImmutable
			}
			return nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	st.Normalize(model.DefaultConfig().Execution.MaxLevelHistoryEntries)
	if err := st.Validate(); err != nil {
		return err
	}
	b, err := yaml.Marshal(st)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, b, 0o644)
}

func LoadChange(root, requestID string) (model.ChangeState, error) {
	b, err := os.ReadFile(ChangePath(root, requestID))
	if err != nil {
		return model.ChangeState{}, err
	}
	var st model.ChangeState
	if err := yaml.Unmarshal(b, &st); err != nil {
		return model.ChangeState{}, err
	}
	st.Normalize(model.DefaultConfig().Execution.MaxLevelHistoryEntries)
	if err := st.Validate(); err != nil {
		return model.ChangeState{}, err
	}
	return st, nil
}

func SaveChange(root string, st model.ChangeState) error {
	if st.RequestID == "" {
		return errors.New("request_id is required")
	}
	st.Normalize(model.DefaultConfig().Execution.MaxLevelHistoryEntries)
	if err := st.Validate(); err != nil {
		return err
	}
	b, err := yaml.Marshal(st)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(ChangePath(root, st.RequestID), b, 0o644)
}

// DiscoverActiveRecords is diagnostics-oriented and may return 0..N active records.
func DiscoverActiveRecords(root string) ([]ActiveRecord, error) {
	records := make([]ActiveRecord, 0)

	admissionFiles, err := listYAMLFiles(AdmissionsDir(root))
	if err != nil {
		return nil, err
	}
	for _, path := range admissionFiles {
		requestID := strings.TrimSuffix(filepath.Base(path), ".yaml")
		st, err := LoadAdmission(root, requestID)
		if err != nil {
			return nil, fmt.Errorf("load admission %q: %w", requestID, err)
		}
		if st.AdmissionStatus == model.AdmissionStatusActive {
			records = append(records, ActiveRecord{
				RequestID: st.RequestID,
				Lane:      LaneAdmission,
				Path:      path,
			})
		}
	}

	changeFiles, err := listYAMLFiles(ChangesDir(root))
	if err != nil {
		return nil, err
	}
	for _, path := range changeFiles {
		requestID := strings.TrimSuffix(filepath.Base(path), ".yaml")
		st, err := LoadChange(root, requestID)
		if err != nil {
			return nil, fmt.Errorf("load change %q: %w", requestID, err)
		}
		if st.ChangeStatus == model.ChangeStatusActive {
			records = append(records, ActiveRecord{
				RequestID: st.RequestID,
				Lane:      LaneChange,
				Path:      path,
			})
		}
	}

	slices.SortFunc(records, func(a, b ActiveRecord) int {
		if a.RequestID != b.RequestID {
			if a.RequestID < b.RequestID {
				return -1
			}
			return 1
		}
		if a.Lane < b.Lane {
			return -1
		}
		if a.Lane > b.Lane {
			return 1
		}
		return 0
	})

	return records, nil
}

// ResolveActiveRequest enforces MVP single-active invariant for request-scoped commands.
func ResolveActiveRequest(root string) (ActiveResolution, error) {
	records, err := DiscoverActiveRecords(root)
	if err != nil {
		return ActiveResolution{}, err
	}
	if len(records) == 0 {
		return ActiveResolution{}, ErrNoActiveRequest
	}
	if len(records) == 1 {
		record := records[0]
		mode := ActiveResolutionModeAdmissionOnly
		if record.Lane == LaneChange {
			mode = ActiveResolutionModeGoverned
		}
		return ActiveResolution{
			RequestID: record.RequestID,
			Mode:      mode,
			Record:    record,
		}, nil
	}

	unique := map[string]struct{}{}
	for _, record := range records {
		unique[record.RequestID] = struct{}{}
	}
	if len(unique) == 1 {
		return ActiveResolution{}, ErrSameRequestDualActive
	}
	return ActiveResolution{}, ErrMultipleActiveRequests
}

func listYAMLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(files)
	return files, nil
}
