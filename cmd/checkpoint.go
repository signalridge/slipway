package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type checkpointView struct {
	Slug            string `json:"slug"`
	PausedTaskID    string `json:"paused_task_id"`
	PausedWaveIndex int    `json:"paused_wave_index,omitempty"`
	CheckpointType  string `json:"checkpoint_type"`
	Set             bool   `json:"set"`
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
		Short: desc("checkpoint"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
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
						"Resume the existing checkpoint with `slipway run --resume-response` before setting a new one.",
						nil,
					)
				}

				execCtx, err := loadExecutionContext(root, change)
				if err != nil {
					return err
				}

				var wavePlan model.WavePlan
				currentWaveIndex := 0
				if execCtx.Ready && execCtx.LatestRunVersion > 0 {
					waveCtx, err := loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, "checkpoint")
					if err != nil {
						return err
					}
					if waveCtx != nil {
						wavePlan = waveCtx.Plan
						currentWaveIndex = state.ResumeWaveIndex(wavePlan, waveCtx.Runs)
					}
				} else {
					wavePlan, err = state.LoadWavePlanForChange(root, change)
					if err != nil {
						if errors.Is(err, fs.ErrNotExist) {
							return newStateIntegrityError(
								"wave_plan_missing",
								"checkpoint requires wave-plan.yaml but it is missing",
								"Run `slipway repair` to restore execution plan artifacts before setting a checkpoint.",
								change.Slug,
								map[string]any{"path": state.WavePlanPathForRead(root, change.Slug)},
							)
						}
						return err
					}
					currentWaveIndex = state.ResumeWaveIndex(wavePlan, nil)
				}
				if currentWaveIndex == 0 {
					return newInvalidUsageError(
						"checkpoint_unavailable",
						"all planned waves have already passed; no active wave can be checkpointed",
						"Use `slipway next` or `slipway done` to continue the lifecycle instead of setting a checkpoint.",
						nil,
					)
				}

				pausedWaveIndex := wavePlan.WaveIndexForTask(taskID)
				if pausedWaveIndex == 0 {
					return newInvalidUsageError(
						"checkpoint_task_unknown",
						fmt.Sprintf("task %q is not present in the current wave plan", taskID),
						"Use a task id from tasks.md / wave-plan.yaml and retry.",
						nil,
					)
				}
				if pausedWaveIndex != currentWaveIndex {
					return newInvalidUsageError(
						"checkpoint_task_not_in_current_wave",
						fmt.Sprintf("task %q belongs to wave %d, but the current incomplete wave is %d", taskID, pausedWaveIndex, currentWaveIndex),
						"Choose a task from the current incomplete wave before setting a checkpoint.",
						nil,
					)
				}

				beforeChange := change
				change.ActiveCheckpoint = &cp
				change.ActiveCheckpoint.PausedWaveIndex = pausedWaveIndex
				change.ActiveCheckpoint.PausedAt = time.Now().UTC()
				if err := state.SaveChange(root, change); err != nil {
					return err
				}
				if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
					Command:     "checkpoint",
					EventType:   "checkpoint.opened",
					Action:      "opened",
					Reason:      checkpointType,
					Result:      "pending_response",
					BeforeState: beforeChange.CurrentState,
					AfterState:  change.CurrentState,
					Diagnostics: []string{
						fmt.Sprintf("task_id=%s", taskID),
						fmt.Sprintf("wave_index=%d", pausedWaveIndex),
						fmt.Sprintf("checkpoint_type=%s", checkpointType),
					},
				}); err != nil {
					return err
				}

				view := checkpointView{
					Slug:            ref.Slug,
					PausedTaskID:    taskID,
					PausedWaveIndex: pausedWaveIndex,
					CheckpointType:  checkpointType,
					Set:             true,
				}

				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}

				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef(
					"Checkpoint set: task=%s wave=%d type=%s\nResume with: slipway run --resume-response \"<response>\"\n",
					taskID, pausedWaveIndex, checkpointType,
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
