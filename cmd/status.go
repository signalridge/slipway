package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type statusView struct {
	LaneMode          string                      `json:"lane_mode"`
	RequestID         string                      `json:"request_id,omitempty"`
	LifecycleStatus   string                      `json:"lifecycle_status,omitempty"`
	Level             model.Level                 `json:"level,omitempty"`
	LevelSource       model.LevelSource           `json:"level_source,omitempty"`
	CurrentState      model.WorkflowState         `json:"current_state,omitempty"`
	NextReadyActions  []string                    `json:"next_ready_actions,omitempty"`
	Blockers          []string                    `json:"blockers,omitempty"`
	GateStatus        map[string]model.GateRecord `json:"gate_status,omitempty"`
	EvidencePointers  statusEvidencePointers      `json:"evidence_pointers,omitempty"`
	EvidenceFreshness string                      `json:"evidence_freshness"`
	SourceStateFile   string                      `json:"source_state_file,omitempty"`
	Diagnostics       []string                    `json:"diagnostics,omitempty"`
}

type statusEvidencePointers struct {
	TaskEvidence    map[string]string `json:"task_evidence,omitempty"`
	NonTaskEvidence map[string]string `json:"non_task_evidence,omitempty"`
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show lifecycle status, blockers, and next actions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			records, err := state.DiscoverActiveRecords(root)
			if err != nil {
				return err
			}
			if len(records) != 1 {
				view := statusView{
					LaneMode:          "diagnostics",
					EvidenceFreshness: "unknown",
				}
				if len(records) == 0 {
					view.Diagnostics = []string{"no active request; run `spln new`"}
				} else {
					view.Diagnostics = []string{"multiple active requests detected; run `spln repair`"}
				}
				return printStatusJSON(cmd, view)
			}

			record := records[0]
			if record.Lane == state.LaneAdmission {
				admission, err := state.LoadAdmission(root, record.RequestID)
				if err != nil {
					return err
				}
				blockers := append([]string{}, admission.RouteSnapshot.BlockingConflicts...)
				view := statusView{
					LaneMode:         "admission_only",
					RequestID:        admission.RequestID,
					LifecycleStatus:  string(admission.AdmissionStatus),
					Level:            admission.Level,
					LevelSource:      admission.LevelSource,
					CurrentState:     admission.CurrentState,
					NextReadyActions: projectNextReadyActions(admission.CurrentState),
					Blockers:         blockers,
					EvidenceFreshness: projectFreshnessForLane(
						root,
						admission.RequestID,
						admission.LatestFrozenRunSummaryVersion,
						admission.Level,
						admission.LevelSource,
						admission.RouteSnapshot,
						admission.TaskRuns,
						blockers,
						admission.ActionHistory,
					),
					SourceStateFile:  filepath.Join(".spln", "runtime", "admissions", admission.RequestID+".yaml"),
					EvidencePointers: buildEvidencePointers(admission.TaskRuns, admission.EvidenceRefs),
				}
				view.EvidencePointers.NonTaskEvidence = mergeStringMap(
					view.EvidencePointers.NonTaskEvidence,
					collectSkillEvidencePointers(root, admission.RequestID),
				)
				return printStatusJSON(cmd, view)
			}

			if record.Lane == state.LaneChange {
				change, err := state.LoadChange(root, record.RequestID)
				if err != nil {
					return err
				}
				blockers := append([]string{}, change.RouteSnapshot.BlockingConflicts...)
				view := statusView{
					LaneMode:         "governed",
					RequestID:        change.RequestID,
					LifecycleStatus:  string(change.ChangeStatus),
					Level:            change.Level,
					LevelSource:      change.LevelSource,
					CurrentState:     change.CurrentState,
					NextReadyActions: projectNextReadyActions(change.CurrentState),
					Blockers:         blockers,
					EvidenceFreshness: projectFreshnessForLane(
						root,
						change.RequestID,
						change.LatestFrozenRunSummaryVersion,
						change.Level,
						change.LevelSource,
						change.RouteSnapshot,
						change.TaskRuns,
						blockers,
						change.ActionHistory,
					),
					SourceStateFile:  filepath.Join(".spln", "runtime", "changes", change.RequestID+".yaml"),
					GateStatus:       copyGateStatus(change.Gates),
					EvidencePointers: buildEvidencePointers(change.TaskRuns, change.EvidenceRefs),
				}
				view.EvidencePointers.NonTaskEvidence = mergeStringMap(
					view.EvidencePointers.NonTaskEvidence,
					collectSkillEvidencePointers(root, change.RequestID),
				)
				return printStatusJSON(cmd, view)
			}

			return errors.New("unknown lane mode")
		},
	}
	return cmd
}

func printStatusJSON(cmd *cobra.Command, view statusView) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(view)
}

func buildEvidencePointers(taskRuns map[string]model.TaskRun, nonTask map[string]string) statusEvidencePointers {
	taskPointers := map[string]string{}
	taskKeys := make([]string, 0, len(taskRuns))
	for key := range taskRuns {
		taskKeys = append(taskKeys, key)
	}
	slices.Sort(taskKeys)
	for _, key := range taskKeys {
		run := taskRuns[key]
		if run.EvidenceRef == "" {
			continue
		}
		taskPointers[key] = run.EvidenceRef
	}

	nonTaskPointers := map[string]string{}
	nonTaskKeys := make([]string, 0, len(nonTask))
	for key := range nonTask {
		nonTaskKeys = append(nonTaskKeys, key)
	}
	slices.Sort(nonTaskKeys)
	for _, key := range nonTaskKeys {
		nonTaskPointers[key] = nonTask[key]
	}

	return statusEvidencePointers{
		TaskEvidence:    taskPointers,
		NonTaskEvidence: nonTaskPointers,
	}
}

func copyGateStatus(gates map[string]model.GateRecord) map[string]model.GateRecord {
	if len(gates) == 0 {
		return nil
	}
	keys := make([]string, 0, len(gates))
	for key := range gates {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	copied := make(map[string]model.GateRecord, len(gates))
	for _, key := range keys {
		copied[key] = gates[key]
	}
	return copied
}

func collectSkillEvidencePointers(root, requestID string) map[string]string {
	base := filepath.Join(root, ".spln", "evidence", "skills", requestID)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		keys = append(keys, entry.Name())
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		return nil
	}
	out := make(map[string]string, len(keys))
	for _, name := range keys {
		key := "skill." + strings.TrimSuffix(name, ".json")
		out[key] = filepath.Join(".spln", "evidence", "skills", requestID, name)
	}
	return out
}

func mergeStringMap(base map[string]string, add map[string]string) map[string]string {
	if len(base) == 0 && len(add) == 0 {
		return nil
	}
	merged := map[string]string{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range add {
		merged[key] = value
	}
	return merged
}
