package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type reviewOptions struct {
	changedOnly bool
	all         bool
	artifact    string
}

type reviewView struct {
	RequestID    string   `json:"request_id"`
	LaneMode     string   `json:"lane_mode"`
	CurrentState string   `json:"current_state"`
	Verdict      string   `json:"verdict"`
	Blockers     []string `json:"blockers,omitempty"`
}

func newReviewCmd() *cobra.Command {
	opts := reviewOptions{}
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run review flow for current execution artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.artifact != "" {
				return fmt.Errorf(
					"`--artifact` is not supported in MVP; use default changed-only review or --all",
				)
			}

			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceStateLock(root, "review", func() error {
				active, err := ensureRequestScopedActive(root)
				if err != nil {
					return err
				}

				var view reviewView
				switch active.Mode {
				case state.ActiveResolutionModeAdmissionOnly:
					admission, err := state.LoadAdmission(root, active.RequestID)
					if err != nil {
						return err
					}
					if admission.AdmissionStatus != model.AdmissionStatusActive {
						return fmt.Errorf("review requires active request; current status=%s", admission.AdmissionStatus)
					}

					if err := ensureReviewEntryState(admission.CurrentState, admission.LatestFrozenRunSummaryVersion); err != nil {
						return err
					}
					if admission.CurrentState == model.StateS6RunWaves || admission.CurrentState == model.StateS8Verify {
						admission.CurrentState = model.StateS7Review
					}

					verdict, blockers := evaluateReviewVerdict(admission.TaskRuns, admission.LatestFrozenRunSummaryVersion)
					if verdict == "fail" {
						admission.CurrentState = model.StateS6RunWaves
					}
					admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
						Action:    "review",
						State:     admission.CurrentState,
						Timestamp: time.Now().UTC(),
						Details: map[string]string{
							"verdict": verdict,
						},
					})
					if err := state.SaveAdmission(root, admission); err != nil {
						return err
					}
					view = reviewView{
						RequestID:    active.RequestID,
						LaneMode:     "admission_only",
						CurrentState: string(admission.CurrentState),
						Verdict:      verdict,
						Blockers:     blockers,
					}
				case state.ActiveResolutionModeGoverned:
					change, err := state.LoadChange(root, active.RequestID)
					if err != nil {
						return err
					}
					if change.ChangeStatus != model.ChangeStatusActive {
						return fmt.Errorf("review requires active request; current status=%s", change.ChangeStatus)
					}

					if err := ensureReviewEntryState(change.CurrentState, change.LatestFrozenRunSummaryVersion); err != nil {
						return err
					}
					if change.CurrentState == model.StateS6RunWaves || change.CurrentState == model.StateS8Verify {
						change.CurrentState = model.StateS7Review
					}

					verdict, blockers := evaluateReviewVerdict(change.TaskRuns, change.LatestFrozenRunSummaryVersion)
					if verdict == "fail" {
						change.CurrentState = model.StateS6RunWaves
					}
					change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
						Action:    "review",
						State:     change.CurrentState,
						Timestamp: time.Now().UTC(),
						Details: map[string]string{
							"verdict": verdict,
						},
					})
					if err := state.SaveChange(root, change); err != nil {
						return err
					}
					view = reviewView{
						RequestID:    active.RequestID,
						LaneMode:     "governed",
						CurrentState: string(change.CurrentState),
						Verdict:      verdict,
						Blockers:     blockers,
					}
				default:
					return fmt.Errorf("unsupported active mode %q", active.Mode)
				}

				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			})
		},
	}

	cmd.Flags().BoolVar(&opts.changedOnly, "changed-only", true, "Review only changed/stale units")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Run full review")
	cmd.Flags().StringVar(&opts.artifact, "artifact", "", "Artifact path (unsupported in MVP)")
	return cmd
}

func ensureReviewEntryState(current model.WorkflowState, latestSummary int) error {
	switch current {
	case model.StateS7Review:
		return nil
	case model.StateS6RunWaves:
		if latestSummary < 1 {
			return fmt.Errorf("review from S6 requires frozen run summary; complete `spln do` first")
		}
		return nil
	case model.StateS8Verify:
		return nil
	default:
		return fmt.Errorf("review is allowed only in S6/S7/S8")
	}
}

func evaluateReviewVerdict(taskRuns map[string]model.TaskRun, latestSummary int) (string, []string) {
	if latestSummary < 1 {
		return "fail", []string{"missing_frozen_run_summary"}
	}
	blockers := []string{}
	for key, run := range taskRuns {
		if run.RunSummaryVersion != latestSummary {
			continue
		}
		if run.Verdict != model.TaskVerdictPass {
			blockers = append(blockers, "non_pass_task:"+run.TaskID)
		}
		if len(run.Blockers) > 0 {
			blockers = append(blockers, "task_blockers:"+key)
		}
	}
	if len(blockers) > 0 {
		return "fail", uniqueSorted(blockers)
	}
	return "pass", nil
}
