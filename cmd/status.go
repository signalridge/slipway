package cmd

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type statusView struct {
	LaneMode          string              `json:"lane_mode"`
	RequestID         string              `json:"request_id,omitempty"`
	LifecycleStatus   string              `json:"lifecycle_status,omitempty"`
	Level             model.Level         `json:"level,omitempty"`
	LevelSource       model.LevelSource   `json:"level_source,omitempty"`
	CurrentState      model.WorkflowState `json:"current_state,omitempty"`
	NextReadyActions  []string            `json:"next_ready_actions,omitempty"`
	Blockers          []string            `json:"blockers,omitempty"`
	EvidenceFreshness string              `json:"evidence_freshness"`
	SourceStateFile   string              `json:"source_state_file,omitempty"`
	Diagnostics       []string            `json:"diagnostics,omitempty"`
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
					SourceStateFile: filepath.Join(".spln", "runtime", "admissions", admission.RequestID+".yaml"),
				}
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
					SourceStateFile: filepath.Join(".spln", "runtime", "changes", change.RequestID+".yaml"),
				}
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
