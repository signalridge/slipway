package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/signalridge/speclane/internal/engine/action"
	"github.com/signalridge/speclane/internal/engine/gate"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type doneView struct {
	RequestID string `json:"request_id"`
	LaneMode  string `json:"lane_mode"`
	Status    string `json:"status"`
	Archived  bool   `json:"archived"`
}

func newDoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done",
		Short: "Finalize a done-ready request and archive it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceStateLock(root, "done", func() error {
				active, err := ensureRequestScopedActive(root)
				if err != nil {
					return err
				}

				var view doneView
				switch active.Mode {
				case state.ActiveResolutionModeAdmissionOnly:
					admission, err := state.LoadAdmission(root, active.RequestID)
					if err != nil {
						return err
					}
					if admission.AdmissionStatus != model.AdmissionStatusActive {
						return fmt.Errorf("cannot finalize non-active admission status %q", admission.AdmissionStatus)
					}
					if !action.CanFinalizeDone(admission.CurrentState) {
						return fmt.Errorf("admission request is not done-ready; expected S8_VERIFY")
					}

					admission.AdmissionStatus = model.AdmissionStatusDone
					admission.CurrentState = model.StateDone
					admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
						Action:    "done",
						State:     model.StateDone,
						Timestamp: time.Now().UTC(),
					})
					if err := state.SaveAdmission(root, admission); err != nil {
						return err
					}
					if err := state.ArchiveDirectAdmission(root, admission); err != nil {
						return err
					}
					view = doneView{
						RequestID: active.RequestID,
						LaneMode:  "admission_only",
						Status:    string(model.AdmissionStatusDone),
						Archived:  true,
					}
				case state.ActiveResolutionModeGoverned:
					change, err := state.LoadChange(root, active.RequestID)
					if err != nil {
						return err
					}
					if change.ChangeStatus != model.ChangeStatusActive {
						return fmt.Errorf("cannot finalize non-active governed status %q", change.ChangeStatus)
					}
					if !action.CanFinalizeDone(change.CurrentState) {
						return fmt.Errorf("governed request is not done-ready; expected S8_VERIFY")
					}
					ship, exists := change.Gates[string(gate.GateShip)]
					if !exists || ship.Status != model.GateStatusApproved {
						return fmt.Errorf("governed done requires approved G_ship gate")
					}

					var admission *model.AdmissionState
					if ad, err := state.LoadAdmission(root, active.RequestID); err == nil {
						admission = &ad
					}
					if _, err := state.ArchiveGoverned(root, change, admission, model.ChangeStatusDone); err != nil {
						return err
					}
					view = doneView{
						RequestID: active.RequestID,
						LaneMode:  "governed",
						Status:    string(model.ChangeStatusDone),
						Archived:  true,
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
	return cmd
}
