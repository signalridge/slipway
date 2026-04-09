package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type cancelView struct {
	Slug            string `json:"slug"`
	ExecutionMode   string `json:"execution_mode"`
	Status          string `json:"status"`
	Archived        bool   `json:"archived"`
	InterruptPIDs   []int  `json:"interrupt_pids,omitempty"`
	ForceKilledPIDs []int  `json:"force_killed_pids,omitempty"`
}

func makeCancelCmd() *cobra.Command {
	var changeSlug string
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an active change and archive terminal state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			active, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, active.Slug, "cancel", func() error {
				cfg, err := loadConfigAtRoot(root)
				if err != nil {
					return err
				}
				interrupted, forceKilled, err := preemptInFlightTasks(
					root,
					active.Slug,
					time.Duration(cfg.Execution.CancelGracePeriodSeconds)*time.Second,
				)
				if err != nil {
					return err
				}
				preemptionEvidenceRef, err := writeCancelPreemptionEvidence(root, active.Slug, interrupted, forceKilled)
				if err != nil {
					return err
				}

				change, err := loadActiveChange(
					root,
					active.Slug,
					"cannot cancel non-active change status %q",
					"Only active changes can be cancelled.",
				)
				if err != nil {
					return err
				}

				change.Status = model.ChangeStatusCancelled
				details := map[string]string{
					"interrupt_pids":    strings.TrimSpace(fmt.Sprintf("%v", interrupted)),
					"force_killed_pids": strings.TrimSpace(fmt.Sprintf("%v", forceKilled)),
				}
				if strings.TrimSpace(preemptionEvidenceRef) != "" {
					key := fmt.Sprintf("cancel_preemption_%d", time.Now().UTC().UnixNano())
					change.EvidenceRefs[key] = preemptionEvidenceRef
					details["preemption_evidence_ref"] = preemptionEvidenceRef
				}
				if err := state.SaveChange(root, change); err != nil {
					return err
				}

				execMode := governedExecutionMode
				if _, err := state.ArchiveChange(root, change, model.ChangeStatusCancelled); err != nil {
					return err
				}

				view := cancelView{
					Slug:            active.Slug,
					ExecutionMode:   execMode,
					Status:          string(model.ChangeStatusCancelled),
					Archived:        true,
					InterruptPIDs:   interrupted,
					ForceKilledPIDs: forceKilled,
				}

				return encodeJSONResponse(cmd, view)
			})
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func loadActiveTaskPIDs(root, slug string) ([]int, error) {
	path := state.TaskPIDFilePath(root, slug)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
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

func clearActiveTaskPIDs(root, slug string) error {
	path := state.TaskPIDFilePath(root, slug)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func writeCancelPreemptionEvidence(root, slug string, interrupted, forceKilled []int) (string, error) {
	if len(interrupted) == 0 && len(forceKilled) == 0 {
		return "", nil
	}
	dir := filepath.Join(state.ChangeDir(root, slug), "evidence", "tasks", "cancel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	payload := map[string]any{
		"slug":              slug,
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
