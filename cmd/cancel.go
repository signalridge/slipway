package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type cancelView struct {
	RequestID       string `json:"request_id"`
	LaneMode        string `json:"lane_mode"`
	Status          string `json:"status"`
	Archived        bool   `json:"archived"`
	InterruptPIDs   []int  `json:"interrupt_pids,omitempty"`
	ForceKilledPIDs []int  `json:"force_killed_pids,omitempty"`
}

func newCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an active request and archive terminal state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceStateLock(root, "cancel", func() error {
				active, err := ensureRequestScopedActive(root)
				if err != nil {
					return err
				}

				var view cancelView
				cfg, err := loadConfigAtRoot(root)
				if err != nil {
					return err
				}
				interrupted, forceKilled, err := preemptInFlightTasks(
					root,
					active.RequestID,
					time.Duration(cfg.Execution.CancelGracePeriodSeconds)*time.Second,
				)
				if err != nil {
					return err
				}
				preemptionEvidenceRef, err := writeCancelPreemptionEvidence(root, active.RequestID, interrupted, forceKilled)
				if err != nil {
					return err
				}
				switch active.Mode {
				case state.ActiveResolutionModeAdmissionOnly:
					admission, err := state.LoadAdmission(root, active.RequestID)
					if err != nil {
						return err
					}
					if admission.AdmissionStatus != model.AdmissionStatusActive {
						return fmt.Errorf("cannot cancel non-active admission status %q", admission.AdmissionStatus)
					}

					admission.AdmissionStatus = model.AdmissionStatusCancelled
					details := map[string]string{
						"interrupt_pids":    strings.TrimSpace(fmt.Sprintf("%v", interrupted)),
						"force_killed_pids": strings.TrimSpace(fmt.Sprintf("%v", forceKilled)),
					}
					if strings.TrimSpace(preemptionEvidenceRef) != "" {
						key := fmt.Sprintf("cancel_preemption_%d", time.Now().UTC().UnixNano())
						admission.EvidenceRefs[key] = preemptionEvidenceRef
						details["preemption_evidence_ref"] = preemptionEvidenceRef
					}
					admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
						Action:    "cancel",
						State:     admission.CurrentState,
						Timestamp: time.Now().UTC(),
						Details:   details,
					})
					if err := state.SaveAdmission(root, admission); err != nil {
						return err
					}
					if err := state.ArchiveDirectAdmission(root, admission); err != nil {
						return err
					}
					view = cancelView{
						RequestID:       active.RequestID,
						LaneMode:        "admission_only",
						Status:          string(model.AdmissionStatusCancelled),
						Archived:        true,
						InterruptPIDs:   interrupted,
						ForceKilledPIDs: forceKilled,
					}
				case state.ActiveResolutionModeGoverned:
					change, err := state.LoadChange(root, active.RequestID)
					if err != nil {
						return err
					}
					if change.ChangeStatus != model.ChangeStatusActive {
						return fmt.Errorf("cannot cancel non-active governed status %q", change.ChangeStatus)
					}

					change.ChangeStatus = model.ChangeStatusCancelled
					details := map[string]string{
						"interrupt_pids":    strings.TrimSpace(fmt.Sprintf("%v", interrupted)),
						"force_killed_pids": strings.TrimSpace(fmt.Sprintf("%v", forceKilled)),
					}
					if strings.TrimSpace(preemptionEvidenceRef) != "" {
						key := fmt.Sprintf("cancel_preemption_%d", time.Now().UTC().UnixNano())
						change.EvidenceRefs[key] = preemptionEvidenceRef
						details["preemption_evidence_ref"] = preemptionEvidenceRef
					}
					change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
						Action:    "cancel",
						State:     change.CurrentState,
						Timestamp: time.Now().UTC(),
						Details:   details,
					})
					if err := state.SaveChange(root, change); err != nil {
						return err
					}

					var admission *model.AdmissionState
					if ad, err := state.LoadAdmission(root, active.RequestID); err == nil {
						admission = &ad
					}
					if _, err := state.ArchiveGoverned(root, change, admission, model.ChangeStatusCancelled); err != nil {
						return err
					}
					view = cancelView{
						RequestID:       active.RequestID,
						LaneMode:        "governed",
						Status:          string(model.ChangeStatusCancelled),
						Archived:        true,
						InterruptPIDs:   interrupted,
						ForceKilledPIDs: forceKilled,
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

func preemptInFlightTasks(root, requestID string, grace time.Duration) ([]int, []int, error) {
	pids, err := loadActiveTaskPIDs(root, requestID)
	if err != nil || len(pids) == 0 {
		return nil, nil, err
	}
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGINT)
	}
	if grace < 0 {
		grace = 0
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if len(filterAlivePIDs(pids)) == 0 {
			_ = clearActiveTaskPIDs(root, requestID)
			return pids, nil, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	stillAlive := filterAlivePIDs(pids)
	for _, pid := range stillAlive {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	_ = clearActiveTaskPIDs(root, requestID)
	return pids, stillAlive, nil
}

func loadActiveTaskPIDs(root, requestID string) ([]int, error) {
	path := filepath.Join(root, ".spln", "runtime", "task_pids", requestID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pids []int
	if err := json.Unmarshal(raw, &pids); err != nil {
		return nil, err
	}
	slices.Sort(pids)
	return pids, nil
}

func clearActiveTaskPIDs(root, requestID string) error {
	path := filepath.Join(root, ".spln", "runtime", "task_pids", requestID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func filterAlivePIDs(pids []int) []int {
	alive := []int{}
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		if err := syscall.Kill(pid, 0); err == nil {
			alive = append(alive, pid)
		}
	}
	return alive
}

func writeCancelPreemptionEvidence(root, requestID string, interrupted, forceKilled []int) (string, error) {
	if len(interrupted) == 0 && len(forceKilled) == 0 {
		return "", nil
	}
	dir := filepath.Join(root, ".spln", "evidence", "tasks", requestID, "cancel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	payload := map[string]any{
		"request_id":        requestID,
		"timestamp":         time.Now().UTC().Format(time.RFC3339Nano),
		"interrupt_pids":    append([]int(nil), interrupted...),
		"force_killed_pids": append([]int(nil), forceKilled...),
		"outcome":           "cancelled_or_aborted",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", time.Now().UTC().UnixNano()))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
