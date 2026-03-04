package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ctxpack "github.com/signalridge/speclane/internal/engine/context"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type contextOptions struct {
	format string
}

func newContextCmd() *cobra.Command {
	opts := contextOptions{}

	cmd := &cobra.Command{
		Use:   "context",
		Short: "Show compact execution context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			records, err := state.DiscoverActiveRecords(root)
			if err != nil {
				return err
			}

			var pack ctxpack.Pack
			if len(records) != 1 {
				remediation := []string{}
				if len(records) == 0 {
					remediation = append(remediation, "run `spln new` to create an active request")
				} else {
					remediation = append(remediation, "run `spln repair` to resolve active-context ambiguity")
				}
				pack = ctxpack.BuildDiagnosticsPack(remediation)
			} else {
				record := records[0]
				switch record.Lane {
				case state.LaneAdmission:
					admission, err := state.LoadAdmission(root, record.RequestID)
					if err != nil {
						return err
					}
					blockers := append([]string{}, admission.RouteSnapshot.BlockingConflicts...)
					pack = ctxpack.BuildAdmissionPack(
						admission,
						filepath.Join(".spln", "runtime", "admissions", admission.RequestID+".yaml"),
						projectNextReadyActions(admission.CurrentState),
						blockers,
						ctxpack.EvidenceFreshness(projectFreshnessForLane(
							root,
							admission.RequestID,
							admission.LatestFrozenRunSummaryVersion,
							admission.Level,
							admission.LevelSource,
							admission.RouteSnapshot,
							admission.TaskRuns,
							blockers,
							admission.ActionHistory,
						)),
					)
					attachWaveEnvelope(&pack, admission.CurrentState, admission.TaskRuns, admission.LatestFrozenRunSummaryVersion)
					attachCheckpointResume(
						&pack,
						admission.LatestFrozenRunSummaryVersion,
						admission.TaskRuns,
						admission.ActionHistory,
					)
				case state.LaneChange:
					change, err := state.LoadChange(root, record.RequestID)
					if err != nil {
						return err
					}
					blockers := append([]string{}, change.RouteSnapshot.BlockingConflicts...)
					pack = ctxpack.BuildGovernedPack(
						change,
						filepath.Join(".spln", "runtime", "changes", change.RequestID+".yaml"),
						projectNextReadyActions(change.CurrentState),
						blockers,
						"governed request context",
						nil,
						ctxpack.EvidenceFreshness(projectFreshnessForLane(
							root,
							change.RequestID,
							change.LatestFrozenRunSummaryVersion,
							change.Level,
							change.LevelSource,
							change.RouteSnapshot,
							change.TaskRuns,
							blockers,
							change.ActionHistory,
						)),
					)
					attachWaveEnvelope(&pack, change.CurrentState, change.TaskRuns, change.LatestFrozenRunSummaryVersion)
					attachCheckpointResume(
						&pack,
						change.LatestFrozenRunSummaryVersion,
						change.TaskRuns,
						change.ActionHistory,
					)
				default:
					return errors.New("unknown lane mode")
				}
			}

			switch strings.ToLower(strings.TrimSpace(opts.format)) {
			case "", "text":
				printContextText(cmd, pack)
				return nil
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(pack)
			case "yaml":
				b, err := yaml.Marshal(pack)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(b)
				return err
			default:
				return fmt.Errorf("invalid --format %q; expected text|yaml|json", opts.format)
			}
		},
	}

	cmd.Flags().StringVar(&opts.format, "format", "text", "Output format: text|yaml|json")
	return cmd
}

func printContextText(cmd *cobra.Command, pack ctxpack.Pack) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "lane_mode: %s\n", pack.LaneMode)
	fmt.Fprintf(out, "evidence_freshness: %s\n", pack.EvidenceFreshness)
	if pack.Level != "" {
		fmt.Fprintf(out, "level: %s (%s)\n", pack.Level, pack.LevelSource)
	}
	if pack.CurrentState != "" {
		fmt.Fprintf(out, "current_state: %s\n", pack.CurrentState)
	}
	if pack.IntentSummary != "" {
		fmt.Fprintf(out, "intent: %s\n", pack.IntentSummary)
	}
	if len(pack.NextReadyActions) > 0 {
		fmt.Fprintf(out, "next_ready_actions: %s\n", strings.Join(pack.NextReadyActions, ", "))
	}
	if len(pack.Blockers) > 0 {
		fmt.Fprintf(out, "blockers: %s\n", strings.Join(pack.Blockers, ", "))
	}
	if len(pack.Remediation) > 0 {
		fmt.Fprintf(out, "remediation: %s\n", strings.Join(pack.Remediation, "; "))
	}
	if pack.WaveEnvelope != nil {
		fmt.Fprintf(out, "wave_id: %s\n", pack.WaveEnvelope.WaveID)
		fmt.Fprintf(out, "task_id: %s\n", pack.WaveEnvelope.TaskID)
		if len(pack.WaveEnvelope.DependsOn) > 0 {
			fmt.Fprintf(out, "depends_on: %s\n", strings.Join(pack.WaveEnvelope.DependsOn, ", "))
		}
		if len(pack.WaveEnvelope.MustHaves) > 0 {
			fmt.Fprintf(out, "must_haves: %s\n", strings.Join(pack.WaveEnvelope.MustHaves, ", "))
		}
		if pack.WaveEnvelope.CheckpointType != "" {
			fmt.Fprintf(out, "checkpoint_type: %s\n", pack.WaveEnvelope.CheckpointType)
		}
	}
	if pack.CheckpointResume != nil {
		fmt.Fprintf(out, "resume_from: %s\n", pack.CheckpointResume.PriorRunID)
		fmt.Fprintf(out, "paused_task_id: %s\n", pack.CheckpointResume.PausedTaskID)
	}
}

func attachWaveEnvelope(
	pack *ctxpack.Pack,
	currentState model.WorkflowState,
	taskRuns map[string]model.TaskRun,
	latestRunSummaryVersion int,
) {
	if pack == nil || currentState != model.StateS6RunWaves || latestRunSummaryVersion < 1 {
		return
	}
	taskID := ""
	var taskKind model.TaskKind
	targetFiles := []string{}
	keys := make([]string, 0, len(taskRuns))
	for key := range taskRuns {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		run := taskRuns[key]
		if run.RunSummaryVersion != latestRunSummaryVersion {
			continue
		}
		taskID = run.TaskID
		taskKind = run.TaskKind
		targetFiles = append([]string{}, run.TargetFiles...)
		break
	}
	if taskID == "" {
		return
	}
	checkpointType := detectCheckpointType(taskRuns, latestRunSummaryVersion)
	pack.WaveEnvelope = &ctxpack.WaveEnvelope{
		WaveID:         fmt.Sprintf("rv%d", latestRunSummaryVersion),
		TaskID:         taskID,
		DependsOn:      []string{},
		TargetFiles:    targetFiles,
		TaskKind:       taskKind,
		Autonomous:     true,
		CheckpointType: checkpointType,
		MustHaves:      defaultMustHaves(taskKind),
	}
}

func detectCheckpointType(taskRuns map[string]model.TaskRun, latestRunSummaryVersion int) string {
	for _, run := range taskRuns {
		if run.RunSummaryVersion != latestRunSummaryVersion {
			continue
		}
		if run.Verdict != model.TaskVerdictPass || len(run.Blockers) > 0 {
			return "checkpoint_response_required"
		}
	}
	return ""
}

func defaultMustHaves(taskKind model.TaskKind) []string {
	switch taskKind {
	case model.TaskKindReview:
		return []string{"review_verdict_recorded", "blockers_accounted"}
	case model.TaskKindVerification:
		return []string{"verification_passes", "goals_met"}
	default:
		return []string{"changes_applied", "evidence_recorded"}
	}
}

type continuationEvidenceRecord struct {
	ResumeSignal        string   `json:"resume_signal"`
	UserResponsePayload string   `json:"user_response_payload"`
	Blockers            []string `json:"blockers"`
}

func attachCheckpointResume(
	pack *ctxpack.Pack,
	latestRunSummaryVersion int,
	taskRuns map[string]model.TaskRun,
	actionHistory []model.ActionEvent,
) {
	if pack == nil || latestRunSummaryVersion < 1 {
		return
	}
	continuationRef, signalFromAction, responseFromAction := latestContinuationReference(actionHistory)
	if continuationRef == "" {
		return
	}
	record, err := readContinuationEvidenceRecord(continuationRef)
	if err != nil {
		return
	}

	pausedTaskID := pausedTaskForCheckpoint(taskRuns, latestRunSummaryVersion)
	checkpointType := strings.TrimSpace(record.ResumeSignal)
	if checkpointType == "" {
		checkpointType = strings.TrimSpace(signalFromAction)
	}
	if checkpointType == "" {
		checkpointType = "checkpoint_response_required"
	}
	userResponse := strings.TrimSpace(record.UserResponsePayload)
	if userResponse == "" {
		userResponse = strings.TrimSpace(responseFromAction)
	}

	bundle := ctxpack.BuildCheckpointResumeBundle(
		fmt.Sprintf("rv%d", latestRunSummaryVersion),
		pausedTaskID,
		checkpointType,
		userResponse,
		record.Blockers,
	)
	_ = ctxpack.AttachCheckpointResume(pack, bundle)
	if pack.WaveEnvelope != nil && strings.TrimSpace(pack.WaveEnvelope.CheckpointType) == "" {
		pack.WaveEnvelope.CheckpointType = checkpointType
	}
}

func latestContinuationReference(actionHistory []model.ActionEvent) (continuationRef, signal, response string) {
	for i := len(actionHistory) - 1; i >= 0; i-- {
		event := actionHistory[i]
		if event.Action != "do" {
			continue
		}
		ref := strings.TrimSpace(event.Details["continuation_evidence_ref"])
		if ref == "" {
			continue
		}
		return ref, strings.TrimSpace(event.Details["resume_signal"]), strings.TrimSpace(event.Details["user_response_payload"])
	}
	return "", "", ""
}

func readContinuationEvidenceRecord(path string) (continuationEvidenceRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return continuationEvidenceRecord{}, err
	}
	record := continuationEvidenceRecord{}
	if err := json.Unmarshal(raw, &record); err != nil {
		return continuationEvidenceRecord{}, err
	}
	return record, nil
}

func pausedTaskForCheckpoint(taskRuns map[string]model.TaskRun, latestRunSummaryVersion int) string {
	type candidate struct {
		key string
		run model.TaskRun
	}
	candidates := []candidate{}
	for key, run := range taskRuns {
		if run.RunSummaryVersion != latestRunSummaryVersion {
			continue
		}
		candidates = append(candidates, candidate{key: key, run: run})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].key < candidates[j].key
	})
	for _, item := range candidates {
		if item.run.Verdict != model.TaskVerdictPass || len(item.run.Blockers) > 0 {
			return item.run.TaskID
		}
	}
	return candidates[0].run.TaskID
}
