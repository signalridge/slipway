package cmd

import (
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type checkpointView struct {
	Slug           string `json:"slug"`
	PausedTaskID   string `json:"paused_task_id"`
	CheckpointType string `json:"checkpoint_type"`
	Set            bool   `json:"set"`
}

func makeCheckpointCmd() *cobra.Command {
	var (
		jsonOutput       bool
		changeSlug       string
		taskID           string
		checkpointType   string
		allowedResponses []string
	)

	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Set an active checkpoint to pause wave execution and request user input",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, "checkpoint", func() error {
				change, err := state.LoadChange(root, ref.Slug)
				if err != nil {
					return err
				}

				// All slipway changes are governed; no level check needed.

				taskID = strings.TrimSpace(taskID)
				if taskID == "" {
					return newInvalidUsageError(
						"checkpoint_task_id_required",
						"--task-id is required",
						"Provide the paused task ID with --task-id.",
						nil,
					)
				}

				checkpointType = strings.TrimSpace(checkpointType)
				if checkpointType == "" {
					checkpointType = string(model.CheckpointHumanVerify)
				}

				cp := model.ActiveCheckpoint{
					PausedTaskID:     taskID,
					CheckpointType:   checkpointType,
					AllowedResponses: allowedResponses,
				}
				if err := cp.Validate(); err != nil {
					return newInvalidUsageError(
						"checkpoint_invalid",
						err.Error(),
						"Fix the checkpoint parameters and retry.",
						nil,
					)
				}

				if change.CurrentState != model.StateS2Execute {
					return newInvalidUsageError(
						"checkpoint_wrong_state",
						fmt.Sprintf("checkpoint requires S2_EXECUTE state, current: %s", change.CurrentState),
						"Checkpoints can only be set during wave execution.",
						nil,
					)
				}

				if change.ActiveCheckpoint != nil {
					return newInvalidUsageError(
						"checkpoint_already_active",
						fmt.Sprintf("checkpoint already active for task %s", change.ActiveCheckpoint.PausedTaskID),
						"Resume the existing checkpoint with `slipway next --resume-response` before setting a new one.",
						nil,
					)
				}

				change.ActiveCheckpoint = &cp
				if err := state.SaveChange(root, change); err != nil {
					return err
				}

				view := checkpointView{
					Slug:           ref.Slug,
					PausedTaskID:   taskID,
					CheckpointType: checkpointType,
					Set:            true,
				}

				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}

				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef(
					"Checkpoint set: task=%s type=%s\nResume with: slipway next --resume-response \"<response>\"\n",
					taskID, checkpointType,
				)
				return writer.Err()
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().StringVar(&taskID, "task-id", "", "ID of the paused task (required)")
	cmd.Flags().StringVar(&checkpointType, "type", "human_verify", "Checkpoint type: human_verify, decision, human_action")
	cmd.Flags().StringSliceVar(&allowedResponses, "allowed-responses", nil, "Allowed response values (required for type=decision)")

	return cmd
}
