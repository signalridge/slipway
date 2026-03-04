package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/signalridge/speclane/internal/engine/action"
	"github.com/signalridge/speclane/internal/engine/artifact"
	"github.com/signalridge/speclane/internal/engine/gate"
	"github.com/signalridge/speclane/internal/engine/wave"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type doOptions struct {
	resumeResponse string
}

type doCheckpoint struct {
	ResumeSignal      string   `json:"resume_signal"`
	ExpectedResponses []string `json:"expected_responses"`
	Blockers          []string `json:"blockers,omitempty"`
}

type doView struct {
	LaneMode         string         `json:"lane_mode"`
	RequestID        string         `json:"request_id"`
	CurrentState     string         `json:"current_state"`
	DoneReady        bool           `json:"done_ready"`
	NextReadyActions []string       `json:"next_ready_actions,omitempty"`
	Blockers         []string       `json:"blockers,omitempty"`
	Checkpoint       *doCheckpoint  `json:"checkpoint,omitempty"`
	Remediation      []string       `json:"remediation,omitempty"`
	Details          map[string]any `json:"details,omitempty"`
}

func newDoCmd() *cobra.Command {
	opts := doOptions{}

	cmd := &cobra.Command{
		Use:   "do",
		Short: "Execute one next action for the active request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceStateLock(root, "do", func() error {
				active, err := ensureRequestScopedActive(root)
				if err != nil {
					return err
				}

				var view doView
				switch active.Mode {
				case state.ActiveResolutionModeAdmissionOnly:
					view, err = runAdmissionDo(root, active.RequestID, opts.resumeResponse)
				case state.ActiveResolutionModeGoverned:
					view, err = runGovernedDo(root, active.RequestID, opts.resumeResponse)
				default:
					err = fmt.Errorf("unsupported active mode %q", active.Mode)
				}
				if err != nil {
					return err
				}
				return printDoJSON(cmd, view)
			})
		},
	}

	cmd.Flags().StringVar(&opts.resumeResponse, "resume-response", "", "Checkpoint continuation response text")
	return cmd
}

func runAdmissionDo(root, requestID, resumeResponse string) (doView, error) {
	admission, err := state.LoadAdmission(root, requestID)
	if err != nil {
		return doView{}, err
	}
	if admission.AdmissionStatus != model.AdmissionStatusActive {
		return doView{}, fmt.Errorf("request is not active; status=%s", admission.AdmissionStatus)
	}

	switch admission.CurrentState {
	case model.StateS1Analyze:
		admission.CurrentState = model.StateS6RunWaves
	case model.StateS6RunWaves:
		postWaveTaskRuns, overlapConflicts, hasOverlap := applyPostWaveOverlapConflicts(
			admission.TaskRuns,
			admission.LatestFrozenRunSummaryVersion,
		)
		if hasOverlap {
			admission.TaskRuns = postWaveTaskRuns
			admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
				Action:    "do",
				State:     admission.CurrentState,
				Timestamp: time.Now().UTC(),
				Details: map[string]string{
					"post_wave_file_conflict": "true",
				},
			})
			if err := state.SaveAdmission(root, admission); err != nil {
				return doView{}, err
			}
			return doView{
				LaneMode:         "admission_only",
				RequestID:        requestID,
				CurrentState:     string(admission.CurrentState),
				DoneReady:        false,
				NextReadyActions: projectNextReadyActions(admission.CurrentState),
				Blockers:         overlapConflicts,
				Remediation: []string{
					"resolve overlap via serialized retry: run `spln do --resume-response retry`",
				},
			}, nil
		}

		checkpoint, err := resolveCheckpointContinuation(
			admission.TaskRuns,
			admission.LatestFrozenRunSummaryVersion,
			resumeResponse,
		)
		if err != nil {
			return doView{}, err
		}
		details := map[string]string{}
		if checkpoint != nil {
			details["resume_signal"] = checkpoint.ResumeSignal
			details["user_response_payload"] = strings.TrimSpace(resumeResponse)
			continuationRef, err := writeContinuationEvidence(root, admission.RequestID, checkpoint, resumeResponse)
			if err != nil {
				return doView{}, err
			}
			details["continuation_evidence_ref"] = continuationRef
			if shouldPauseOnResponse(resumeResponse) {
				admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
					Action:    "do",
					State:     admission.CurrentState,
					Timestamp: time.Now().UTC(),
					Details:   details,
				})
				if err := state.SaveAdmission(root, admission); err != nil {
					return doView{}, err
				}
				return doView{
					LaneMode:         "admission_only",
					RequestID:        requestID,
					CurrentState:     string(admission.CurrentState),
					DoneReady:        false,
					NextReadyActions: projectNextReadyActions(admission.CurrentState),
					Blockers:         []string{"checkpoint_waiting_for_operator"},
					Checkpoint:       checkpoint,
				}, nil
			}
		}

		runSummaryVersion := wave.NextRunSummaryVersion(admission.LatestFrozenRunSummaryVersion)
		taskID := nextL1TaskID(admission.RequestID, admission.TaskRuns)
		evidenceRef, err := writeTaskEvidence(
			root,
			admission.RequestID,
			runSummaryVersion,
			taskID,
			admission.Level,
			admission.LevelSource,
			admission.RouteSnapshot,
		)
		if err != nil {
			return doView{}, err
		}

		run := model.TaskRun{
			TaskID:            taskID,
			RunSummaryVersion: runSummaryVersion,
			TaskKind:          model.TaskKindCode,
			Verdict:           model.TaskVerdictPass,
			TargetFiles:       []string{},
			ChangedFiles:      []string{},
			EvidenceRef:       evidenceRef,
			Blockers:          []string{},
		}
		taskRuns, err := model.InsertTaskRun(admission.TaskRuns, run)
		if err != nil {
			return doView{}, err
		}
		admission.TaskRuns = taskRuns

		summary := wave.RunSummary{
			RequestID:         admission.RequestID,
			RunSummaryVersion: runSummaryVersion,
			CompletedTasks:    []string{taskID},
			NonPassTasks:      []string{},
			CarriedDebt:       []string{},
			EvidenceSet:       []string{evidenceRef},
			OpenBlockers:      []string{},
			FrozenAt:          time.Now().UTC(),
		}
		if _, err := wave.PersistFrozenRunSummary(root, summary); err != nil {
			return doView{}, err
		}
		if err := wave.UpdateFrozenRunSummaryPointer(root, admission.RequestID, runSummaryVersion); err != nil {
			return doView{}, err
		}

		admission.LatestFrozenRunSummaryVersion = runSummaryVersion
		auto := action.RunL1DoAutoChecks(admission)
		admission.CurrentState = auto.NextState
		admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
			Action:    "do",
			State:     admission.CurrentState,
			Timestamp: time.Now().UTC(),
			Details:   details,
		})
		if err := state.SaveAdmission(root, admission); err != nil {
			return doView{}, err
		}

		remediation := []string{}
		if auto.DoneReady {
			remediation = append(remediation, "run `spln done` to finalize and archive")
		}
		return doView{
			LaneMode:         "admission_only",
			RequestID:        requestID,
			CurrentState:     string(admission.CurrentState),
			DoneReady:        auto.DoneReady,
			NextReadyActions: projectNextReadyActions(admission.CurrentState),
			Blockers:         auto.Blockers,
			Checkpoint:       checkpoint,
			Remediation:      remediation,
			Details: map[string]any{
				"run_summary_version": runSummaryVersion,
				"task_id":             taskID,
			},
		}, nil
	case model.StateS7Review:
		admission.CurrentState = model.StateS8Verify
	case model.StateS8Verify:
		return doView{
			LaneMode:         "admission_only",
			RequestID:        requestID,
			CurrentState:     string(admission.CurrentState),
			DoneReady:        true,
			NextReadyActions: projectNextReadyActions(admission.CurrentState),
			Remediation:      []string{"run `spln done` to finalize and archive"},
		}, nil
	default:
		return doView{}, fmt.Errorf("unsupported state for admission lane do: %s", admission.CurrentState)
	}

	admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
		Action:    "do",
		State:     admission.CurrentState,
		Timestamp: time.Now().UTC(),
	})
	if err := state.SaveAdmission(root, admission); err != nil {
		return doView{}, err
	}
	return doView{
		LaneMode:         "admission_only",
		RequestID:        requestID,
		CurrentState:     string(admission.CurrentState),
		DoneReady:        admission.CurrentState == model.StateS8Verify,
		NextReadyActions: projectNextReadyActions(admission.CurrentState),
		Blockers:         append([]string{}, admission.RouteSnapshot.BlockingConflicts...),
	}, nil
}

func runGovernedDo(root, requestID, resumeResponse string) (doView, error) {
	change, err := state.LoadChange(root, requestID)
	if err != nil {
		return doView{}, err
	}
	if change.ChangeStatus != model.ChangeStatusActive {
		return doView{}, fmt.Errorf("request is not active; status=%s", change.ChangeStatus)
	}

	blockers := []string{}
	details := map[string]any{}

	switch change.CurrentState {
	case model.StateS1Analyze:
		if change.Level == model.LevelL3 {
			change.CurrentState = model.StateS2Discover
		} else {
			change.CurrentState = model.StateS4SpecBundle
		}
	case model.StateS2Discover:
		content, err := ensureDiscoverArtifact(root, change.Slug)
		if err != nil {
			return doView{}, err
		}
		if err := action.RunS2Discover(&change, content); err != nil {
			return doView{}, err
		}
	case model.StateS3ScopeConfirmation:
		worktreePath, err := os.Getwd()
		if err != nil {
			return doView{}, err
		}
		branch, err := currentBranch(worktreePath)
		if err != nil {
			return doView{}, err
		}
		if err := state.PersistScopeWorktreeMetadata(&change, worktreePath, branch); err != nil {
			return doView{}, err
		}
		worktreeReasons, err := state.ValidateWorktreeAuthenticityReasons(
			root,
			change.WorktreePath,
			change.WorktreeBranch,
		)
		if err != nil {
			return doView{}, err
		}

		_, skillBlockers, err := evaluateRequiredSkills(
			root,
			change.RequestID,
			change.Level,
			model.StateS3ScopeConfirmation,
			change.LatestFrozenRunSummaryVersion,
			false,
		)
		if err != nil {
			return doView{}, err
		}

		explorePath := filepath.Join(root, "aircraft", "changes", change.Slug, "explore.md")
		exploreRaw, err := os.ReadFile(explorePath)
		if err != nil {
			blockers = append(blockers, "missing_explore_md")
			break
		}
		scopeEvidenceOK := len(skillBlockers) == 0
		eval := gate.EvaluateGScope(
			change,
			string(exploreRaw),
			scopeEvidenceOK,
			scopeEvidenceOK,
			worktreeReasons,
		)
		eval.Reasons = uniqueSorted(append(eval.Reasons, skillBlockers...))
		change.Gates[string(gate.GateScope)] = model.GateRecord{
			GateID:    string(gate.GateScope),
			Status:    eval.Status,
			Decision:  gateDecisionFromStatus(eval.Status),
			Reasons:   append([]string{}, eval.Reasons...),
			UpdatedAt: time.Now().UTC(),
		}
		if eval.Status == model.GateStatusApproved {
			change.CurrentState = model.StateS4SpecBundle
		} else {
			blockers = append(blockers, eval.Reasons...)
		}
	case model.StateS4SpecBundle:
		if err := action.RunS4SpecBundle(root, &change); err != nil {
			blockers = append(blockers, err.Error())
		}
	case model.StateS5PlanAudit:
		_, skillBlockers, err := evaluateRequiredSkills(
			root,
			change.RequestID,
			change.Level,
			model.StateS5PlanAudit,
			change.LatestFrozenRunSummaryVersion,
			false,
		)
		if err != nil {
			return doView{}, err
		}
		blockersForGate := append([]string{}, change.RouteSnapshot.BlockingConflicts...)
		blockersForGate = append(blockersForGate, skillBlockers...)
		eval := gate.EvaluateGPlan(true, len(skillBlockers) == 0, blockersForGate)
		change.Gates[string(gate.GatePlan)] = model.GateRecord{
			GateID:    string(gate.GatePlan),
			Status:    eval.Status,
			Decision:  gateDecisionFromStatus(eval.Status),
			Reasons:   append([]string{}, eval.Reasons...),
			UpdatedAt: time.Now().UTC(),
		}
		if eval.Status == model.GateStatusApproved {
			change.CurrentState = model.StateS6RunWaves
		} else {
			next, loopErr := action.ResolveLoopTransition(action.LoopTransitionInput{
				Level:        change.Level,
				CurrentState: model.StateS5PlanAudit,
				Trigger:      action.LoopTriggerPlanAuditFailed,
			})
			if loopErr != nil {
				return doView{}, loopErr
			}
			change.CurrentState = next
			blockers = append(blockers, eval.Reasons...)
		}
	case model.StateS6RunWaves:
		postWaveTaskRuns, overlapConflicts, hasOverlap := applyPostWaveOverlapConflicts(
			change.TaskRuns,
			change.LatestFrozenRunSummaryVersion,
		)
		if hasOverlap {
			change.TaskRuns = postWaveTaskRuns
			change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
				Action:    "do",
				State:     change.CurrentState,
				Timestamp: time.Now().UTC(),
				Details: map[string]string{
					"post_wave_file_conflict": "true",
				},
			})
			if err := state.SaveChange(root, change); err != nil {
				return doView{}, err
			}
			return doView{
				LaneMode:         "governed",
				RequestID:        requestID,
				CurrentState:     string(change.CurrentState),
				DoneReady:        false,
				NextReadyActions: projectNextReadyActions(change.CurrentState),
				Blockers:         overlapConflicts,
				Remediation: []string{
					"resolve overlap via serialized retry: run `spln do --resume-response retry`",
				},
			}, nil
		}

		checkpoint, err := resolveCheckpointContinuation(
			change.TaskRuns,
			change.LatestFrozenRunSummaryVersion,
			resumeResponse,
		)
		if err != nil {
			return doView{}, err
		}
		if checkpoint != nil {
			details["resume_signal"] = checkpoint.ResumeSignal
			details["user_response_payload"] = strings.TrimSpace(resumeResponse)
			continuationRef, err := writeContinuationEvidence(root, change.RequestID, checkpoint, resumeResponse)
			if err != nil {
				return doView{}, err
			}
			details["continuation_evidence_ref"] = continuationRef
			if shouldPauseOnResponse(resumeResponse) {
				change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
					Action:    "do",
					State:     change.CurrentState,
					Timestamp: time.Now().UTC(),
					Details: map[string]string{
						"resume_signal":         checkpoint.ResumeSignal,
						"user_response_payload": strings.TrimSpace(resumeResponse),
					},
				})
				if err := state.SaveChange(root, change); err != nil {
					return doView{}, err
				}
				return doView{
					LaneMode:         "governed",
					RequestID:        requestID,
					CurrentState:     string(change.CurrentState),
					DoneReady:        false,
					NextReadyActions: projectNextReadyActions(change.CurrentState),
					Blockers:         []string{"checkpoint_waiting_for_operator"},
					Checkpoint:       checkpoint,
				}, nil
			}
		}

		runSummaryVersion := wave.NextRunSummaryVersion(change.LatestFrozenRunSummaryVersion)
		taskID := fmt.Sprintf("gov-%s-%02d", shortRequestID(change.RequestID), runSummaryVersion)
		evidenceRef, err := writeTaskEvidence(
			root,
			change.RequestID,
			runSummaryVersion,
			taskID,
			change.Level,
			change.LevelSource,
			change.RouteSnapshot,
		)
		if err != nil {
			return doView{}, err
		}

		run := model.TaskRun{
			TaskID:            taskID,
			RunSummaryVersion: runSummaryVersion,
			TaskKind:          model.TaskKindCode,
			Verdict:           model.TaskVerdictPass,
			TargetFiles:       []string{},
			ChangedFiles:      []string{},
			EvidenceRef:       evidenceRef,
			Blockers:          []string{},
		}
		taskRuns, err := model.InsertTaskRun(change.TaskRuns, run)
		if err != nil {
			return doView{}, err
		}
		change.TaskRuns = taskRuns

		summary := wave.RunSummary{
			RequestID:         change.RequestID,
			RunSummaryVersion: runSummaryVersion,
			CompletedTasks:    []string{taskID},
			NonPassTasks:      []string{},
			CarriedDebt:       []string{},
			EvidenceSet:       []string{evidenceRef},
			OpenBlockers:      []string{},
			FrozenAt:          time.Now().UTC(),
		}
		if _, err := wave.PersistFrozenRunSummary(root, summary); err != nil {
			return doView{}, err
		}
		if err := wave.UpdateFrozenRunSummaryPointer(root, change.RequestID, runSummaryVersion); err != nil {
			return doView{}, err
		}
		change.LatestFrozenRunSummaryVersion = runSummaryVersion
		change.CurrentState = model.StateS7Review
		details["run_summary_version"] = runSummaryVersion
		details["task_id"] = taskID
	case model.StateS7Review:
		if change.LatestFrozenRunSummaryVersion < 1 {
			return doView{}, fmt.Errorf("review requires frozen run summary; run `spln do` from S6 first")
		}
		change.CurrentState = model.StateS8Verify
	case model.StateS8Verify:
		artifactReady := true
		unresolved := append([]string{}, change.RouteSnapshot.BlockingConflicts...)
		assurancePath := filepath.Join(root, "aircraft", "changes", change.Slug, "assurance.md")
		if b, err := os.ReadFile(assurancePath); err != nil {
			artifactReady = false
			unresolved = append(unresolved, "assurance_read_failed")
		} else if err := artifact.ValidateAssuranceStructure(string(b)); err != nil {
			artifactReady = false
			unresolved = append(unresolved, "assurance_structure_invalid:"+err.Error())
		}

		closeoutRequired := len(change.RouteSnapshot.BlockingConflicts) > 0
		verificationSkills, verificationBlockers, err := evaluateRequiredSkills(
			root,
			change.RequestID,
			change.Level,
			model.StateS8Verify,
			change.LatestFrozenRunSummaryVersion,
			closeoutRequired,
		)
		if err != nil {
			return doView{}, err
		}
		unresolved = append(unresolved, verificationBlockers...)

		manifestPath := filepath.Join(root, "aircraft", "changes", change.Slug, "change.yaml")
		manifestR0Valid, manifestReasons := validateChangeManifestR0(
			manifestPath,
			change.RequestID,
			change.Slug,
			change.Level,
		)
		unresolved = append(unresolved, manifestReasons...)

		checks := extractHighRiskChecks(verificationSkills, change.EvidenceRefs)

		eval := gate.EvaluateGShip(
			change,
			artifactReady,
			len(verificationBlockers) == 0,
			manifestR0Valid,
			unresolved,
			checks,
		)
		change.Gates[string(gate.GateShip)] = model.GateRecord{
			GateID:    string(gate.GateShip),
			Status:    eval.Status,
			Decision:  gateDecisionFromStatus(eval.Status),
			Reasons:   append([]string{}, eval.Reasons...),
			UpdatedAt: time.Now().UTC(),
		}
		if eval.Status == model.GateStatusBlocked {
			next, loopErr := action.ResolveLoopTransition(action.LoopTransitionInput{
				Level:        change.Level,
				CurrentState: model.StateS8Verify,
				Trigger:      action.LoopTriggerVerifyFailed,
			})
			if loopErr != nil {
				return doView{}, loopErr
			}
			change.CurrentState = next
			blockers = append(blockers, eval.Reasons...)
		}
	default:
		return doView{}, fmt.Errorf("unsupported state for governed lane do: %s", change.CurrentState)
	}

	change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
		Action:    "do",
		State:     change.CurrentState,
		Timestamp: time.Now().UTC(),
		Details:   stringifyDetails(details),
	})
	if err := state.SaveChange(root, change); err != nil {
		return doView{}, err
	}

	doneReady := change.CurrentState == model.StateS8Verify
	if doneReady {
		blockers = append(blockers, change.RouteSnapshot.BlockingConflicts...)
	}

	return doView{
		LaneMode:         "governed",
		RequestID:        requestID,
		CurrentState:     string(change.CurrentState),
		DoneReady:        doneReady,
		NextReadyActions: projectNextReadyActions(change.CurrentState),
		Blockers:         blockers,
		Details:          details,
	}, nil
}

func resolveCheckpointContinuation(
	taskRuns map[string]model.TaskRun,
	latestRunSummaryVersion int,
	resumeResponse string,
) (*doCheckpoint, error) {
	if latestRunSummaryVersion < 1 {
		return nil, nil
	}
	needsCheckpoint := false
	blockers := []string{}
	for _, run := range taskRuns {
		if run.RunSummaryVersion != latestRunSummaryVersion {
			continue
		}
		if run.Verdict != model.TaskVerdictPass || len(run.Blockers) > 0 {
			needsCheckpoint = true
			blockers = append(blockers, run.Blockers...)
			blockers = append(blockers, "non_pass_task:"+run.TaskID)
		}
	}
	if !needsCheckpoint {
		return nil, nil
	}

	checkpoint := &doCheckpoint{
		ResumeSignal:      "checkpoint_response_required",
		ExpectedResponses: []string{"retry", "skip", "approved", "pivot", "abort"},
		Blockers:          uniqueSorted(blockers),
	}

	response := strings.ToLower(strings.TrimSpace(resumeResponse))
	if response == "" {
		return checkpoint, fmt.Errorf(
			"S6 checkpoint continuation requires --resume-response; expected one of: %s",
			strings.Join(checkpoint.ExpectedResponses, ", "),
		)
	}
	if !slices.Contains(checkpoint.ExpectedResponses, response) {
		return checkpoint, fmt.Errorf(
			"invalid --resume-response %q; expected one of: %s",
			resumeResponse,
			strings.Join(checkpoint.ExpectedResponses, ", "),
		)
	}
	return checkpoint, nil
}

func applyPostWaveOverlapConflicts(
	taskRuns map[string]model.TaskRun,
	latestRunSummaryVersion int,
) (map[string]model.TaskRun, []string, bool) {
	if latestRunSummaryVersion < 1 || len(taskRuns) < 2 {
		return taskRuns, nil, false
	}

	type indexedRun struct {
		Key string
		Run model.TaskRun
	}

	latest := []indexedRun{}
	results := []wave.TaskResult{}
	for key, run := range taskRuns {
		if run.RunSummaryVersion != latestRunSummaryVersion {
			continue
		}
		if run.Verdict != model.TaskVerdictPass {
			continue
		}
		latest = append(latest, indexedRun{Key: key, Run: run})
		results = append(results, wave.TaskResult{
			TaskID:       run.TaskID,
			ChangedFiles: append([]string{}, run.ChangedFiles...),
		})
	}
	if len(latest) < 2 {
		return taskRuns, nil, false
	}

	fileOwner := map[string]string{}
	conflictTasks := map[string]struct{}{}
	for _, result := range results {
		for _, file := range result.ChangedFiles {
			if owner, exists := fileOwner[file]; exists && owner != result.TaskID {
				conflictTasks[owner] = struct{}{}
				conflictTasks[result.TaskID] = struct{}{}
				continue
			}
			fileOwner[file] = result.TaskID
		}
	}
	if len(conflictTasks) == 0 {
		return taskRuns, nil, false
	}

	conflicts := wave.DetectPostWaveFileOverlap(results)
	updated := map[string]model.TaskRun{}
	for key, run := range taskRuns {
		updated[key] = run
	}
	for _, item := range latest {
		if _, conflicted := conflictTasks[item.Run.TaskID]; !conflicted {
			continue
		}
		run := updated[item.Key]
		run.Verdict = model.TaskVerdictBlocked
		run.Blockers = uniqueSorted(append(run.Blockers, conflicts...))
		updated[item.Key] = run
	}
	return updated, conflicts, true
}

func shouldPauseOnResponse(resumeResponse string) bool {
	response := strings.ToLower(strings.TrimSpace(resumeResponse))
	return response == "pivot" || response == "abort"
}

func printDoJSON(cmd *cobra.Command, view doView) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(view)
}

func nextL1TaskID(requestID string, taskRuns map[string]model.TaskRun) string {
	short := shortRequestID(requestID)
	prefix := "l1-" + short + "-"
	maxN := 0
	for _, run := range taskRuns {
		if !strings.HasPrefix(run.TaskID, prefix) {
			continue
		}
		part := strings.TrimPrefix(run.TaskID, prefix)
		var n int
		if _, err := fmt.Sscanf(part, "%d", &n); err == nil && n > maxN {
			maxN = n
		}
	}
	return fmt.Sprintf("%s%02d", prefix, maxN+1)
}

func shortRequestID(requestID string) string {
	if len(requestID) <= 8 {
		return requestID
	}
	return requestID[:8]
}

func ensureDiscoverArtifact(root, slug string) (string, error) {
	path := filepath.Join(root, "aircraft", "changes", slug, "explore.md")
	if _, err := os.Stat(path); err == nil {
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", readErr
		}
		return string(b), nil
	}

	content := strings.Join([]string{
		"## Objectives",
		"- Establish discovery objectives",
		"",
		"## Unknowns",
		"- Identify unknowns",
		"",
		"## Assumptions",
		"- Record assumptions",
		"",
		"## Scope Boundaries",
		"- Define in-scope and out-of-scope boundaries",
		"",
		"## Validation Plan",
		"- Define validation approach",
		"",
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return content, nil
}

func currentBranch(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func writeTaskEvidence(
	root, requestID string,
	runSummaryVersion int,
	taskID string,
	level model.Level,
	levelSource model.LevelSource,
	routeSnapshot model.RouteSnapshot,
) (string, error) {
	if err := maybeRunOpportunisticEvidenceGC(root); err != nil {
		return "", err
	}

	baseDir := filepath.Join(
		root,
		".spln",
		"evidence",
		"tasks",
		requestID,
		fmt.Sprintf("rv%d", runSummaryVersion),
	)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	inputHash, err := computeTaskEvidenceInputHash(
		requestID,
		runSummaryVersion,
		taskID,
		level,
		levelSource,
		routeSnapshot,
	)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(map[string]any{
		"task_id":             taskID,
		"run_summary_version": runSummaryVersion,
		"input_hash":          inputHash,
		"captured_at":         time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", err
	}

	for i := 0; ; i++ {
		name := taskID + ".json"
		if i > 0 {
			name = fmt.Sprintf("%s--%d.json", taskID, i+1)
		}
		path := filepath.Join(baseDir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			return "", err
		}
		return path, nil
	}
}

func maybeRunOpportunisticEvidenceGC(root string) error {
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return err
	}

	freeMB, err := freeDiskMB(root)
	if err != nil {
		// Best-effort telemetry signal only; write path continues on stat failure.
		return nil
	}
	if freeMB >= int64(cfg.Execution.EvidenceGCLowDiskFreeMB) {
		return nil
	}

	_, err = state.RunEvidenceRetentionGC(root, cfg.Execution.EvidenceRetentionDays, time.Now().UTC())
	return err
}

func freeDiskMB(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	freeBytes := int64(stat.Bavail) * int64(stat.Bsize)
	return freeBytes / (1024 * 1024), nil
}

func writeContinuationEvidence(
	root string,
	requestID string,
	checkpoint *doCheckpoint,
	resumeResponse string,
) (string, error) {
	if checkpoint == nil {
		return "", nil
	}
	path := filepath.Join(
		root,
		".spln",
		"evidence",
		"tasks",
		requestID,
		"continuation",
		fmt.Sprintf("%d.json", time.Now().UTC().UnixNano()),
	)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	payload := map[string]any{
		"resume_signal":         checkpoint.ResumeSignal,
		"expected_responses":    checkpoint.ExpectedResponses,
		"user_response_payload": strings.TrimSpace(resumeResponse),
		"blockers":              checkpoint.Blockers,
		"timestamp":             time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func gateDecisionFromStatus(status model.GateStatus) model.GateDecision {
	switch status {
	case model.GateStatusApproved:
		return model.GateDecisionApprove
	case model.GateStatusBlocked:
		return model.GateDecisionReject
	default:
		return model.GateDecisionConditionalApprove
	}
}

func stringifyDetails(details map[string]any) map[string]string {
	if len(details) == 0 {
		return nil
	}
	out := map[string]string{}
	keys := make([]string, 0, len(details))
	for key := range details {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		out[key] = fmt.Sprintf("%v", details[key])
	}
	return out
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}
