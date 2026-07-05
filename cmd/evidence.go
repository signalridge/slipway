package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"github.com/spf13/cobra"
)

type evidenceTaskView struct {
	Slug              string                             `json:"slug"`
	TaskID            string                             `json:"task_id"`
	RunSummaryVersion int                                `json:"run_summary_version"`
	InvocationRoute   *invocationRouteView               `json:"invocation_route,omitempty"`
	Path              string                             `json:"path"`
	Recorded          bool                               `json:"recorded"`
	FreshnessInputs   model.ExecutionTaskFreshnessInputs `json:"freshness_inputs"`
}

type evidenceTaskBatchView struct {
	Slug              string               `json:"slug"`
	RunSummaryVersion int                  `json:"run_summary_version"`
	InvocationRoute   *invocationRouteView `json:"invocation_route,omitempty"`
	Recorded          bool                 `json:"recorded"`
	RecordedCount     int                  `json:"recorded_count"`
	Tasks             []evidenceTaskView   `json:"tasks"`
}

type evidenceSkillView struct {
	Slug            string               `json:"slug"`
	Skill           string               `json:"skill"`
	SkillName       string               `json:"skill_name"`
	Verdict         string               `json:"verdict"`
	RunVersion      int                  `json:"run_version"`
	InvocationRoute *invocationRouteView `json:"invocation_route,omitempty"`
	Path            string               `json:"path"`
	Recorded        bool                 `json:"recorded"`
	Stamped         bool                 `json:"stamped"`
	References      []string             `json:"references,omitempty"`
}

const (
	taskEvidenceResultFileRemediation = "Record task evidence with `slipway evidence task --result-file <path> --json` after the executor writes compact JSON with task_id, verdict, evidence_ref, changed_files, blockers, and optional session_id; repeat --result-file to import multiple task results atomically."
	maxEvidenceTaskResultFileBytes    = int64(1 << 20)
	maxEvidenceTaskResultFiles        = 256
)

func makeEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: desc("evidence"),
		Args:  rejectRetiredEvidenceSubcommands,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Bare `slipway evidence` prints the subcommand help (exit 0). Unknown
			// or retired positional tokens are rejected by the Args validator above.
			return cmd.Help()
		},
	}
	cmd.AddCommand(makeEvidenceTaskCmd())
	cmd.AddCommand(makeEvidenceSkillCmd())
	return cmd
}

// rejectRetiredEvidenceSubcommands fails closed on any positional argument that is
// not a registered `evidence` subcommand. Cobra only auto-rejects unknown
// subcommands on the ROOT command (legacyArgs gates on !HasParent), so a nested
// parent like `evidence` would otherwise accept a stray token and silently no-op
// into its own help with exit 0. That mattered for the retired `suite-result`
// keystone: a stale script still running `slipway evidence suite-result` would get
// exit 0 and believe suite proof was recorded when nothing happened. Fail closed
// instead, and name the replacement for the retired token.
func rejectRetiredEvidenceSubcommands(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	sub := strings.TrimSpace(args[0])
	if sub == "suite-result" {
		return newInvalidUsageError(
			"evidence_suite_result_retired",
			"`slipway evidence suite-result` was retired with the ship-verification merge",
			"The authoritative test suite now runs exactly once inside the terminal ship-verification gate; record it with `slipway evidence skill --skill ship-verification ...`. Review peers no longer consume a shared suite-result keystone.",
			map[string]any{"subcommand": sub},
		)
	}
	return newInvalidUsageError(
		"evidence_unknown_subcommand",
		fmt.Sprintf("unknown command %q for \"slipway evidence\"", sub),
		"Use `slipway evidence skill` or `slipway evidence task`; run `slipway evidence --help` to list subcommands.",
		map[string]any{"subcommand": sub},
	)
}

func makeEvidenceSkillCmd() *cobra.Command {
	var (
		jsonOutput     bool
		changeSlug     string
		skillName      string
		verdictRaw     string
		references     []string
		blockers       []string
		notes          string
		notesFile      string
		refreshCurrent bool
	)

	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Record CLI-stamped governance skill verification evidence",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			readCtx := newStateReadContext(root)
			ref, err := resolveActiveChangeRefWithReadContext(readCtx, changeSlug)
			if err != nil {
				return adaptArchivedEvidenceRemediation(err)
			}

			return withChangeStateLock(root, ref.Slug, "evidence skill", func() error {
				change, err := loadActiveChange(
					root,
					ref.Slug,
					"cannot record skill evidence for governed status %q",
					"Skill evidence can only be recorded for an active governed change.",
				)
				if err != nil {
					return err
				}
				route := commandInvocationRoute(cmd, root, change, strings.TrimSpace(changeSlug) != "")

				skillName = strings.TrimSpace(skillName)
				def, err := validateEvidenceSkillName(root, skillName)
				if err != nil {
					return err
				}
				if err := validateEvidenceSkillStage(root, change, def); err != nil {
					return err
				}

				verdict := strings.TrimSpace(verdictRaw)
				switch verdict {
				case model.VerificationVerdictPass, model.VerificationVerdictFail:
				case "":
					return newInvalidUsageError(
						"evidence_skill_verdict_required",
						"--verdict is required",
						"Pass pass or fail.",
						nil,
					)
				default:
					return newInvalidUsageError(
						"evidence_skill_verdict_invalid",
						fmt.Sprintf("invalid skill verdict: %q", verdict),
						"Pass pass or fail.",
						map[string]any{"verdict": verdict},
					)
				}

				blockerCodes := model.ReasonCodesFromSpecs(blockers)
				for i, blocker := range blockerCodes {
					if err := blocker.Validate(); err != nil {
						return newInvalidUsageError(
							"evidence_skill_blocker_invalid",
							fmt.Sprintf("blocker %d is invalid: %v", i, err),
							"Pass blockers as code or code:detail values.",
							nil,
						)
					}
				}
				if verdict == model.VerificationVerdictPass && len(blockerCodes) > 0 {
					return newInvalidUsageError(
						"evidence_skill_pass_with_blockers",
						"pass verdict cannot include blockers",
						"Use --verdict fail when blockers are present, or remove --blocker values.",
						nil,
					)
				}

				notesText, err := resolveEvidenceSkillNotes(root, change, notes, notesFile)
				if err != nil {
					return err
				}
				runVersion, digestSummary, err := evidenceSkillRunContext(root, change, def)
				if err != nil {
					return err
				}
				if err := validateEvidenceSkillActionable(root, change, def, runVersion, refreshCurrent); err != nil {
					return err
				}
				candidateReferences := trimNonEmptyStrings(references)
				candidateRecord := model.VerificationRecord{
					Verdict:    verdict,
					Blockers:   blockerCodes,
					RunVersion: runVersion,
					References: candidateReferences,
					Notes:      notesText,
				}
				if err := validateSelectedReviewPassContextOrigin(root, change, def, candidateRecord); err != nil {
					return err
				}
				if err := validatePlanDimensionSkillEvidence(root, change, def, candidateRecord); err != nil {
					return err
				}
				references = stringutil.UniqueSorted(candidateReferences)

				if verdict == model.VerificationVerdictPass {
					if err := progression.CheckEvidenceDigestInputsForSkill(root, change, skillName, digestSummary); err != nil {
						return newStateIntegrityError(
							"evidence_skill_digest_input_unavailable",
							fmt.Sprintf("failed to resolve %s evidence digest inputs: %v", skillName, err),
							"Repair the governed inputs for this skill and retry the evidence command.",
							change.Slug,
							map[string]any{"skill": skillName},
						)
					}
				}

				record := model.VerificationRecord{
					Verdict:    verdict,
					Blockers:   blockerCodes,
					Timestamp:  time.Now().UTC(),
					RunVersion: runVersion,
					References: references,
					Notes:      notesText,
				}
				previousRaw, hadPrevious, err := readExistingVerificationRaw(root, change.Slug, skillName)
				if err != nil {
					return newStateIntegrityError(
						"evidence_skill_existing_read_failed",
						fmt.Sprintf("failed to read existing %s verification evidence: %v", skillName, err),
						"Repair the existing verification file before overwriting it.",
						change.Slug,
						map[string]any{"skill": skillName},
					)
				}
				path, err := state.SaveVerification(root, change.Slug, skillName, record)
				if err != nil {
					return newStateIntegrityError(
						"evidence_skill_write_failed",
						fmt.Sprintf("failed to write %s verification evidence: %v", skillName, err),
						"Fix the verification record inputs and rerun `slipway evidence skill`.",
						change.Slug,
						map[string]any{"skill": skillName},
					)
				}

				stamped := false
				if record.IsPassing() {
					if err := progression.StampEvidenceDigestForSkill(root, change, skillName, record, digestSummary); err != nil {
						restoreErr := restoreVerificationRaw(path, previousRaw, hadPrevious)
						return newStateIntegrityError(
							"evidence_skill_digest_stamp_failed",
							fmt.Sprintf("failed to stamp %s evidence digest: %v%s", skillName, err, restoreVerificationSuffix(restoreErr)),
							"Resolve missing or stale digest inputs before recording passing skill evidence.",
							change.Slug,
							map[string]any{"skill": skillName},
						)
					}
					stamped = true
				} else if err := progression.PruneEvidenceDigestForSkill(root, change, skillName); err != nil {
					restoreErr := restoreVerificationRaw(path, previousRaw, hadPrevious)
					return newStateIntegrityError(
						"evidence_skill_digest_prune_failed",
						fmt.Sprintf("failed to prune %s evidence digest: %v%s", skillName, err, restoreVerificationSuffix(restoreErr)),
						"Repair the governed verification store and retry the evidence command.",
						change.Slug,
						map[string]any{"skill": skillName},
					)
				}

				displayPath := state.DisplayPath(root, path)
				change.RecordEvidenceRef(skillName, displayPath)
				if err := state.SaveChange(root, change); err != nil {
					return newStateIntegrityError(
						"evidence_skill_change_save_failed",
						fmt.Sprintf("failed to record %s evidence reference: %v", skillName, err),
						"Repair the governed change state and retry.",
						change.Slug,
						map[string]any{"skill": skillName, "path": displayPath},
					)
				}

				// Materialize execution-summary.yaml from the same public command that
				// owns wave execution evidence (issue #228). The summary was previously
				// only written by advance/next or `slipway repair`, so the public
				// per-task-evidence + wave-orchestration flow left validate blocking on
				// run_summary_missing until an undocumented repair. The owning stage now
				// produces the evidence: once the passing wave-orchestration record and
				// its task evidence are durable at S2_IMPLEMENT, sync writes the summary.
				// This is idempotent (sync only rewrites a changed summary) and any
				// error it returns is surfaced, never swallowed, so a partial or
				// scope-failing run still fails closed instead of recording a clean
				// summary.
				// At S2 this first materializes execution-summary.yaml; at S3 it rebuilds
				// it to fold in the just-attested task so the incomplete_execution_task
				// blocker clears in place (the wave evidence was only recordable at S3
				// while that convergence was pending). Both flow through the owning
				// command so validate/status/next reflect the result immediately, and any
				// error fails closed instead of leaving a stale summary.
				if record.IsPassing() &&
					skillName == progression.SkillWaveOrchestration &&
					(change.CurrentState == model.StateS2Implement || change.CurrentState == model.StateS3Review) {
					if _, err := progression.SyncGovernedWaveExecution(root, change); err != nil {
						return newStateIntegrityError(
							"evidence_skill_execution_summary_sync_failed",
							fmt.Sprintf("failed to materialize execution summary after recording %s evidence: %v", skillName, err),
							"Repair the runtime wave execution evidence and retry the evidence command.",
							change.Slug,
							map[string]any{"skill": skillName},
						)
					}
				}

				if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
					Command:     "evidence skill",
					EventType:   "skill.evidence_recorded",
					Action:      "recorded",
					Result:      "recorded",
					BeforeState: change.CurrentState,
					AfterState:  change.CurrentState,
					SkillID:     skillName,
					EvidenceRefs: map[string]string{
						skillName: displayPath,
					},
					Diagnostics: []string{
						fmt.Sprintf("skill=%s", skillName),
						fmt.Sprintf("verdict=%s", verdict),
						fmt.Sprintf("run_version=%d", runVersion),
						"path=" + displayPath,
					},
				}); err != nil {
					return err
				}

				view := evidenceSkillView{
					Slug:            change.Slug,
					Skill:           skillName,
					SkillName:       skillName,
					Verdict:         verdict,
					RunVersion:      runVersion,
					InvocationRoute: route,
					Path:            displayPath,
					Recorded:        true,
					Stamped:         stamped,
					References:      references,
				}
				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}

				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef("Skill evidence recorded: %s\n", view.Path)
				return writer.Err()
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().StringVar(&skillName, "skill", "", "Governance skill name, for example spec-compliance-review (required)")
	cmd.Flags().StringVar(&verdictRaw, "verdict", "", "Skill verdict: pass or fail (required)")
	cmd.Flags().StringArrayVar(&references, "reference", nil, "Verification reference token; may be repeated")
	cmd.Flags().StringArrayVar(&blockers, "blocker", nil, "Verification blocker as code or code:detail; may be repeated")
	cmd.Flags().StringVar(&notes, "notes", "", "Bounded verification notes")
	cmd.Flags().StringVar(&notesFile, "notes-file", "", "Workspace-relative file containing verification notes")
	cmd.Flags().BoolVar(&refreshCurrent, "refresh-current", false, "Intentionally replace already-current passing evidence for a selected S3 review skill")

	return cmd
}

func makeEvidenceTaskCmd() *cobra.Command {
	var (
		jsonOutput    bool
		changeSlug    string
		taskID        string
		runSummary    int
		taskKindRaw   string
		verdictRaw    string
		evidenceRef   string
		resultFiles   []string
		changedFiles  []string
		targetFiles   []string
		blockerSpecs  []string
		capturedAtRaw string
		sessionID     string
	)

	cmd := &cobra.Command{
		Use:   "task",
		Short: "Record runtime task evidence for the active execution run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			readCtx := newStateReadContext(root)
			ref, err := resolveActiveChangeRefWithReadContext(readCtx, changeSlug)
			if err != nil {
				return adaptArchivedEvidenceRemediation(err)
			}

			return withChangeStateLock(root, ref.Slug, "evidence task", func() error {
				change, err := loadActiveChangeWithReadContext(
					readCtx,
					ref.Slug,
					"cannot record task evidence for governed status %q",
					"Task evidence can only be recorded for an active governed change.",
				)
				if err != nil {
					return err
				}
				route := commandInvocationRouteWithReadContext(cmd, readCtx, change, strings.TrimSpace(changeSlug) != "")
				if change.CurrentState != model.StateS2Implement && change.CurrentState != model.StateS3Review {
					remediation := evidenceTaskWrongStateRemediation(root, change)
					return newInvalidUsageError(
						"evidence_task_wrong_state",
						fmt.Sprintf("task evidence requires S2_IMPLEMENT or S3_REVIEW state, current: %s", change.CurrentState),
						remediation,
						nil,
					)
				}

				resultFiles = normalizeEvidenceTaskResultFileArgs(resultFiles)
				if len(resultFiles) > 0 {
					if err := rejectEvidenceTaskResultFileLedgerFlags(cmd); err != nil {
						return err
					}
					return recordEvidenceTaskResultFiles(cmd, root, change, resultFiles, route, jsonOutput)
				}

				taskID = strings.TrimSpace(taskID)
				if err := validateEvidenceTaskID(taskID); err != nil {
					return newInvalidUsageError(
						"evidence_task_id_invalid",
						err.Error(),
						"Use a task ID from the current tasks.md-derived wave projection without path separators.",
						map[string]any{"task_id": taskID},
					)
				}

				wavePlan, err := loadCurrentWavePlanForCommand(root, change)
				if err != nil {
					if errors.Is(err, state.ErrWavePlanCacheUnreadable) {
						return newWavePlanCacheUnreadableError(root, change, "task evidence", err)
					}
					return newStateIntegrityError(
						"evidence_task_wave_plan_unavailable",
						fmt.Sprintf("task evidence requires a current S2 wave projection for %q: %v", change.Slug, err),
						"Fix tasks.md so Slipway can derive the current S2 wave projection, then rerun `slipway implement` before recording task evidence.",
						change.Slug,
						map[string]any{"path": state.WavePlanPathForRead(root, change.Slug)},
					)
				}
				planTask, ok := findEvidenceWavePlanTask(wavePlan, taskID)
				if !ok {
					addedAtReview, addedErr := taskPlannedButNotInWavePlan(root, change, wavePlan, taskID)
					if addedErr != nil {
						return newStateIntegrityError(
							"evidence_task_wave_plan_unavailable",
							fmt.Sprintf("cannot classify task %q against the current tasks.md projection: %v", taskID, addedErr),
							"Fix tasks.md so Slipway can derive the current tasks projection, then retry recording task evidence.",
							change.Slug,
							map[string]any{"task_id": taskID},
						)
					}
					if addedAtReview {
						// tasks.md names this task but the materialized wave projection does
						// not: it was added at S3_REVIEW after S2 execution. Its evidence
						// cannot be recorded in place and `slipway run` will not
						// re-materialize a settled review, so the scope contract that demands
						// it and this command contradict each other (#352). Reopening
						// execution re-materializes the wave plan WITH the added task and
						// makes its evidence recordable.
						return newInvalidUsageError(
							"evidence_task_unknown",
							fmt.Sprintf("task %q is in tasks.md but not the current wave projection; it was added after S2 execution", taskID),
							"This task was added to tasks.md after S2 execution, so the current wave projection does not contain it and its evidence cannot be recorded in place. Reopen execution to re-materialize the wave plan with the added task, then record its evidence: `slipway fix --start-reexecution`. If the work belongs on an existing task, remove the added task from tasks.md.",
							map[string]any{
								"task_id":                  taskID,
								"remediation_command_hint": "slipway fix --start-reexecution",
							},
						)
					}
					return newInvalidUsageError(
						"evidence_task_unknown",
						fmt.Sprintf("task %q is not present in the current wave plan", taskID),
						"Use a task ID from the current tasks.md-derived wave projection and retry.",
						map[string]any{"task_id": taskID},
					)
				}
				if err := assertTaskEvidenceConvergenceAtReview(root, change, taskID); err != nil {
					return err
				}

				if runSummary < 1 {
					return newInvalidUsageError(
						"evidence_task_run_summary_version_invalid",
						"--run-summary-version must be >= 1",
						"Pass the current wave-orchestration run_version as --run-summary-version; the first task-evidence run version is 1.",
						nil,
					)
				}
				if err := validateEvidenceTaskRunSummaryVersion(root, change, runSummary); err != nil {
					return err
				}

				taskKind := model.TaskKind(strings.TrimSpace(taskKindRaw))
				if taskKind == "" {
					return newInvalidUsageError(
						"evidence_task_kind_required",
						"--task-kind is required",
						"Pass one of: code, test, doc, ops, verification, investigation, other.",
						nil,
					)
				}
				if !taskKind.IsValid() {
					return newInvalidUsageError(
						"evidence_task_kind_invalid",
						fmt.Sprintf("invalid task_kind: %q", taskKind),
						"Pass one of: code, test, doc, ops, verification, investigation, other.",
						map[string]any{"task_kind": string(taskKind)},
					)
				}
				if planTask.TaskKind != "" && taskKind != planTask.TaskKind {
					return newInvalidUsageError(
						"evidence_task_kind_mismatch",
						fmt.Sprintf("task %q has task_kind=%q in the current wave projection, got %q", taskID, planTask.TaskKind, taskKind),
						"Use the task_kind recorded in tasks.md for the current task.",
						map[string]any{"expected": string(planTask.TaskKind), "got": string(taskKind)},
					)
				}

				verdict := model.TaskVerdict(strings.TrimSpace(verdictRaw))
				if verdict == "" {
					return newInvalidUsageError(
						"evidence_task_verdict_required",
						"--verdict is required",
						"Pass a valid task verdict such as pass or fail.",
						nil,
					)
				}
				if !verdict.IsValid() {
					return newInvalidUsageError(
						"evidence_task_verdict_invalid",
						fmt.Sprintf("invalid task verdict: %q", verdict),
						"Pass one of: pass, fail, blocked, incomplete, timeout.",
						map[string]any{"verdict": string(verdict)},
					)
				}

				evidenceRef = strings.TrimSpace(evidenceRef)
				if evidenceRef == "" {
					return newInvalidUsageError(
						"evidence_task_ref_required",
						"--evidence-ref is required",
						"Provide a stable transcript, command, artifact, or note reference for this task.",
						nil,
					)
				}

				commandCapturedAt := time.Now().UTC()
				capturedAt, err := parseEvidenceTaskCapturedAt(capturedAtRaw, commandCapturedAt)
				if err != nil {
					return newInvalidUsageError(
						"evidence_task_captured_at_invalid",
						err.Error(),
						"Pass --captured-at as RFC3339Nano, or omit it so Slipway records the current UTC time.",
						nil,
					)
				}

				blockers := model.ReasonCodesFromSpecs(blockerSpecs)
				for i, blocker := range blockers {
					if err := blocker.Validate(); err != nil {
						return newInvalidUsageError(
							"evidence_task_blocker_invalid",
							fmt.Sprintf("blocker %d is invalid: %v", i, err),
							"Pass blockers as code or code:detail values.",
							nil,
						)
					}
				}

				changedFiles, err = normalizeEvidencePaths(changedFiles)
				if err != nil {
					return newInvalidUsageError(
						"evidence_task_changed_file_invalid",
						err.Error(),
						"Pass workspace-relative changed file paths without absolute paths, empty segments, or parent traversal.",
						nil,
					)
				}
				targetFiles, err = normalizeEvidencePaths(targetFiles)
				if err != nil {
					return newInvalidUsageError(
						"evidence_task_target_file_invalid",
						err.Error(),
						"Pass workspace-relative target file paths without absolute paths, empty segments, or parent traversal.",
						nil,
					)
				}
				if len(targetFiles) == 0 {
					targetFiles, err = normalizeEvidencePaths(planTask.TargetFiles)
					if err != nil {
						return newStateIntegrityError(
							"evidence_task_wave_plan_target_invalid",
							fmt.Sprintf("current wave projection target_files for task %q are invalid: %v", taskID, err),
							"Fix the task target_files in tasks.md before recording task evidence.",
							change.Slug,
							map[string]any{"task_id": taskID},
						)
					}
				}

				payload := progression.TaskEvidencePayload{
					TaskID:            taskID,
					RunSummaryVersion: runSummary,
					TaskKind:          taskKind,
					Verdict:           verdict,
					ChangedFiles:      changedFiles,
					TargetFiles:       targetFiles,
					EvidenceRef:       evidenceRef,
					Blockers:          blockers,
					CapturedAt:        capturedAt.Format(time.RFC3339Nano),
					FreshnessInputs:   state.ExpectedExecutionTaskFreshnessInputs(change, runSummary, taskID, wavePlan.TasksPlanHash),
					SessionID:         strings.TrimSpace(sessionID),
				}

				path := filepath.Join(state.EvidenceTasksDir(root, change.Slug), taskID+".json")
				if err := writeEvidenceTaskPayload(path, payload); err != nil {
					return err
				}
				if _, _, _, err := progression.ParseTaskEvidence(root, path, runSummary); err != nil {
					_ = os.Remove(path)
					return newStateIntegrityError(
						"evidence_task_written_invalid",
						fmt.Sprintf("written task evidence failed parser validation: %v", err),
						"Rerun `slipway evidence task`; Slipway removed the invalid evidence file.",
						change.Slug,
						map[string]any{"path": state.DisplayPath(root, path)},
					)
				}

				if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
					Command:     "evidence task",
					EventType:   "task_evidence.recorded",
					Action:      "recorded",
					Result:      string(verdict),
					BeforeState: change.CurrentState,
					AfterState:  change.CurrentState,
					Diagnostics: []string{
						fmt.Sprintf("task_id=%s", taskID),
						fmt.Sprintf("run_summary_version=%d", runSummary),
						"path=" + state.DisplayPath(root, path),
					},
				}); err != nil {
					return err
				}

				view := evidenceTaskView{
					Slug:              change.Slug,
					TaskID:            taskID,
					RunSummaryVersion: runSummary,
					InvocationRoute:   route,
					Path:              state.DisplayPath(root, path),
					Recorded:          true,
					FreshnessInputs:   payload.FreshnessInputs,
				}
				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}

				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef("Task evidence recorded: %s\n", view.Path)
				return writer.Err()
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().StringVar(&taskID, "task-id", "", "Manual flag mode only: task ID from the current tasks.md-derived wave projection")
	cmd.Flags().IntVar(&runSummary, "run-summary-version", 0, "Manual flag mode only: run summary version to attribute task evidence to (>= 1; the first task-evidence run version is 1 -- pass the current wave-orchestration run_version)")
	cmd.Flags().StringVar(&taskKindRaw, "task-kind", "", "Manual flag mode only: task kind: code, test, doc, ops, verification, investigation, other")
	cmd.Flags().StringVar(&verdictRaw, "verdict", "", "Manual flag mode only: task verdict: pass, fail, blocked, incomplete, timeout")
	cmd.Flags().StringVar(&evidenceRef, "evidence-ref", "", "Manual flag mode only: stable transcript, command, artifact, or note reference")
	cmd.Flags().StringArrayVar(&resultFiles, "result-file", nil, "executor result JSON with task_id, verdict, evidence_ref, changed_files, blockers, and optional session_id; may be repeated for atomic batch import; cannot be combined with manual task flags")
	cmd.Flags().StringArrayVar(&changedFiles, "changed-file", nil, "Manual flag mode only: changed file path for this task; may be repeated")
	cmd.Flags().StringArrayVar(&targetFiles, "target-file", nil, "Manual flag mode only: target file path for this task; may be repeated")
	cmd.Flags().StringArrayVar(&blockerSpecs, "blocker", nil, "Manual flag mode only: task blocker as code or code:detail; may be repeated")
	cmd.Flags().StringVar(&capturedAtRaw, "captured-at", "", "Manual flag mode only: evidence timestamp in RFC3339Nano; defaults to now")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Manual flag mode only: optional executor session identifier")

	return cmd
}

type preparedEvidenceTaskResult struct {
	payload    progression.TaskEvidencePayload
	path       string
	resultFile string
	view       evidenceTaskView
}

func normalizeEvidenceTaskResultFileArgs(resultFiles []string) []string {
	normalized := make([]string, 0, len(resultFiles))
	for _, resultFile := range resultFiles {
		if trimmed := strings.TrimSpace(resultFile); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

// loadActiveChangeWithReadContext mirrors loadActiveChange but resolves the
// change through the shared read-context, reusing the change already loaded
// during active-ref resolution instead of reading change.yaml again uncached.
func loadActiveChangeWithReadContext(readCtx *stateReadContext, slug, inactiveMessage, remediation string) (model.Change, error) {
	change, err := readCtx.loadChange(slug)
	if err != nil {
		return model.Change{}, newChangeStateLoadFailedError(slug, err)
	}
	if change.Status != model.ChangeStatusActive {
		return model.Change{}, newPreconditionError(
			"not_active",
			fmt.Sprintf(inactiveMessage, change.Status),
			remediation,
			slug,
			map[string]any{
				"status": string(change.Status),
			},
		)
	}
	return change, nil
}

func recordEvidenceTaskResultFiles(
	cmd *cobra.Command,
	root string,
	change model.Change,
	resultFiles []string,
	route *invocationRouteView,
	jsonOutput bool,
) error {
	if len(resultFiles) == 0 {
		return newInvalidUsageError(
			"evidence_task_result_file_required",
			"--result-file requires a path",
			"Pass one or more workspace-relative executor result JSON file paths.",
			nil,
		)
	}
	if len(resultFiles) > maxEvidenceTaskResultFiles {
		return newInvalidUsageError(
			"evidence_task_result_file_batch_too_large",
			fmt.Sprintf("--result-file may be repeated at most %d times, got %d", maxEvidenceTaskResultFiles, len(resultFiles)),
			"Split very large task result imports into smaller batches.",
			map[string]any{"max_files": maxEvidenceTaskResultFiles, "got": len(resultFiles)},
		)
	}

	wavePlan, err := loadCurrentWavePlanForCommand(root, change)
	if err != nil {
		if errors.Is(err, state.ErrWavePlanCacheUnreadable) {
			return newWavePlanCacheUnreadableError(root, change, "task evidence", err)
		}
		return newStateIntegrityError(
			"evidence_task_wave_plan_unavailable",
			fmt.Sprintf("task evidence requires a current S2 wave projection for %q: %v", change.Slug, err),
			"Fix tasks.md so Slipway can derive the current S2 wave projection, then rerun `slipway implement` before recording task evidence.",
			change.Slug,
			map[string]any{"path": state.WavePlanPathForRead(root, change.Slug)},
		)
	}
	runSummary, existingTaskEvidencePaths, err := deriveEvidenceTaskRunSummaryVersion(root, change, wavePlan)
	if err != nil {
		return err
	}
	if err := validateEvidenceTaskRunSummaryVersion(root, change, runSummary); err != nil {
		return err
	}

	prepared, err := prepareEvidenceTaskResultFiles(root, change, wavePlan, runSummary, resultFiles, time.Now().UTC(), existingTaskEvidencePaths)
	if err != nil {
		return err
	}
	for _, item := range prepared {
		if err := assertTaskEvidenceConvergenceAtReview(root, change, item.payload.TaskID); err != nil {
			return err
		}
	}
	if err := writePreparedEvidenceTaskResults(prepared); err != nil {
		return err
	}
	for _, item := range prepared {
		if _, _, _, err := progression.ParseTaskEvidence(root, item.path, runSummary); err != nil {
			return newStateIntegrityError(
				"evidence_task_written_invalid",
				fmt.Sprintf("written task evidence failed parser validation: %v", err),
				"Rerun `slipway evidence task`; the batch transaction wrote invalid evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, item.path)},
			)
		}
	}

	taskIDs := make([]string, 0, len(prepared))
	paths := make([]string, 0, len(prepared))
	taskVerdicts := make([]string, 0, len(prepared))
	views := make([]evidenceTaskView, 0, len(prepared))
	for _, item := range prepared {
		item.view.InvocationRoute = route
		taskIDs = append(taskIDs, item.payload.TaskID)
		paths = append(paths, item.view.Path)
		taskVerdicts = append(taskVerdicts, fmt.Sprintf("%s:%s", item.payload.TaskID, item.payload.Verdict))
		views = append(views, item.view)
	}
	if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
		Command:     "evidence task",
		EventType:   "task_evidence.recorded",
		Action:      "recorded",
		Result:      evidenceTaskBatchEventResult(prepared),
		BeforeState: change.CurrentState,
		AfterState:  change.CurrentState,
		Diagnostics: []string{
			fmt.Sprintf("task_ids=%s", strings.Join(taskIDs, ",")),
			fmt.Sprintf("task_verdicts=%s", strings.Join(taskVerdicts, ",")),
			fmt.Sprintf("run_summary_version=%d", runSummary),
			fmt.Sprintf("paths=%s", strings.Join(paths, ",")),
		},
	}); err != nil {
		return err
	}

	if len(views) == 1 {
		if jsonOutput {
			return encodeJSONResponse(cmd, views[0])
		}
		writer := newFormatWriter(cmd.OutOrStdout())
		writer.Writef("Task evidence recorded: %s\n", views[0].Path)
		return writer.Err()
	}

	view := evidenceTaskBatchView{
		Slug:              change.Slug,
		RunSummaryVersion: runSummary,
		InvocationRoute:   route,
		Recorded:          true,
		RecordedCount:     len(views),
		Tasks:             views,
	}
	if jsonOutput {
		return encodeJSONResponse(cmd, view)
	}
	writer := newFormatWriter(cmd.OutOrStdout())
	writer.Writef("Task evidence batch recorded: %d tasks\n", len(views))
	for _, task := range views {
		writer.Writef("- %s: %s\n", task.TaskID, task.Path)
	}
	return writer.Err()
}

func evidenceTaskBatchEventResult(prepared []preparedEvidenceTaskResult) string {
	if len(prepared) == 0 {
		return ""
	}
	result := string(prepared[0].payload.Verdict)
	for _, item := range prepared[1:] {
		if string(item.payload.Verdict) != result {
			return "mixed"
		}
	}
	return result
}

func prepareEvidenceTaskResultFiles(
	root string,
	change model.Change,
	wavePlan model.WavePlan,
	runSummary int,
	resultFiles []string,
	commandCapturedAt time.Time,
	existingTaskEvidencePaths []string,
) ([]preparedEvidenceTaskResult, error) {
	seenTasks := map[string]string{}
	prepared := make([]preparedEvidenceTaskResult, 0, len(resultFiles))
	for _, resultFile := range resultFiles {
		result, err := loadEvidenceTaskResultFile(root, change, resultFile)
		if err != nil {
			return nil, err
		}
		taskID := strings.TrimSpace(result.TaskID)
		if err := validateEvidenceTaskID(taskID); err != nil {
			return nil, newInvalidUsageError(
				"evidence_task_id_invalid",
				err.Error(),
				"Use a task ID from the current tasks.md-derived wave projection without path separators.",
				map[string]any{"task_id": taskID, "result_file": resultFile},
			)
		}
		if previous, ok := seenTasks[taskID]; ok {
			return nil, newInvalidUsageError(
				"evidence_task_result_file_duplicate_task",
				fmt.Sprintf("multiple result files target task %q", taskID),
				"Pass at most one result file per task in a single batch.",
				map[string]any{"task_id": taskID, "first_result_file": previous, "duplicate_result_file": resultFile},
			)
		}
		seenTasks[taskID] = resultFile

		sessionID := strings.TrimSpace(result.SessionID)
		planTask, ok := findEvidenceWavePlanTask(wavePlan, taskID)
		if !ok {
			addedAtReview, addedErr := taskPlannedButNotInWavePlan(root, change, wavePlan, taskID)
			if addedErr != nil {
				return nil, newStateIntegrityError(
					"evidence_task_wave_plan_unavailable",
					fmt.Sprintf("cannot classify task %q against the current tasks.md projection: %v", taskID, addedErr),
					"Fix tasks.md so Slipway can derive the current tasks projection, then retry recording task evidence.",
					change.Slug,
					map[string]any{"task_id": taskID},
				)
			}
			if addedAtReview {
				// tasks.md names this task but the materialized wave projection does
				// not: it was added at S3_REVIEW after S2 execution, so its evidence
				// cannot be recorded in place and `slipway run` will not re-materialize
				// a settled review. Reopening execution re-materializes the wave plan
				// WITH the added task and makes its evidence recordable (#352).
				return nil, newInvalidUsageError(
					"evidence_task_unknown",
					fmt.Sprintf("task %q is in tasks.md but not the current wave projection; it was added after S2 execution", taskID),
					"This task was added to tasks.md after S2 execution, so the current wave projection does not contain it and its evidence cannot be recorded in place. Reopen execution to re-materialize the wave plan with the added task, then record its evidence: `slipway fix --start-reexecution`. If the work belongs on an existing task, remove the added task from tasks.md.",
					map[string]any{
						"task_id":                  taskID,
						"result_file":              resultFile,
						"remediation_command_hint": "slipway fix --start-reexecution",
					},
				)
			}
			return nil, newInvalidUsageError(
				"evidence_task_unknown",
				fmt.Sprintf("task %q is not present in the current wave plan", taskID),
				"Use a task ID from the current tasks.md-derived wave projection and retry.",
				map[string]any{"task_id": taskID, "result_file": resultFile},
			)
		}

		taskKind := planTask.TaskKind
		if taskKind == "" {
			taskKind = model.TaskKindOther
		}
		verdict := model.TaskVerdict(strings.TrimSpace(string(result.Verdict)))
		if verdict == "" {
			return nil, newInvalidUsageError(
				"evidence_task_verdict_required",
				"result-file task evidence requires verdict",
				"Write executor result JSON with a valid task verdict such as pass or fail.",
				map[string]any{"task_id": taskID, "result_file": resultFile},
			)
		}
		if !verdict.IsValid() {
			return nil, newInvalidUsageError(
				"evidence_task_verdict_invalid",
				fmt.Sprintf("invalid task verdict: %q", verdict),
				"Pass one of: pass, fail, blocked, incomplete, timeout.",
				map[string]any{"verdict": string(verdict), "task_id": taskID, "result_file": resultFile},
			)
		}

		evidenceRef := strings.TrimSpace(result.EvidenceRef)
		if evidenceRef == "" {
			return nil, newInvalidUsageError(
				"evidence_task_ref_required",
				"result-file task evidence requires evidence_ref",
				"Provide a stable transcript, command, artifact, or note reference for this task.",
				map[string]any{"task_id": taskID, "result_file": resultFile},
			)
		}

		blockers := model.ReasonCodesFromSpecs(result.Blockers)
		for i, blocker := range blockers {
			if err := blocker.Validate(); err != nil {
				return nil, newInvalidUsageError(
					"evidence_task_blocker_invalid",
					fmt.Sprintf("blocker %d is invalid: %v", i, err),
					"Pass blockers as code or code:detail values.",
					map[string]any{"task_id": taskID, "result_file": resultFile},
				)
			}
		}

		changedFiles, err := normalizeEvidencePaths(result.ChangedFiles)
		if err != nil {
			return nil, newInvalidUsageError(
				"evidence_task_changed_file_invalid",
				err.Error(),
				"Pass workspace-relative changed file paths without absolute paths, empty segments, or parent traversal.",
				map[string]any{"task_id": taskID, "result_file": resultFile},
			)
		}
		if len(changedFiles) == 0 {
			return nil, newInvalidUsageError(
				"evidence_task_changed_file_required",
				"result-file task evidence requires at least one changed_files entry",
				"Write executor result JSON with changed_files containing the workspace-relative files changed by the task.",
				map[string]any{"task_id": taskID, "result_file": resultFile},
			)
		}

		targetFiles, err := normalizeEvidencePaths(planTask.TargetFiles)
		if err != nil {
			return nil, newStateIntegrityError(
				"evidence_task_wave_plan_target_invalid",
				fmt.Sprintf("current wave projection target_files for task %q are invalid: %v", taskID, err),
				"Fix the task target_files in tasks.md before recording task evidence.",
				change.Slug,
				map[string]any{"task_id": taskID, "result_file": resultFile},
			)
		}

		payload := progression.TaskEvidencePayload{
			TaskID:            taskID,
			RunSummaryVersion: runSummary,
			TaskKind:          taskKind,
			Verdict:           verdict,
			ChangedFiles:      changedFiles,
			TargetFiles:       targetFiles,
			EvidenceRef:       evidenceRef,
			Blockers:          blockers,
			CapturedAt:        commandCapturedAt.Format(time.RFC3339Nano),
			FreshnessInputs:   state.ExpectedExecutionTaskFreshnessInputs(change, runSummary, taskID, wavePlan.TasksPlanHash),
			SessionID:         sessionID,
		}
		path := filepath.Join(state.EvidenceTasksDir(root, change.Slug), taskID+".json")
		prepared = append(prepared, preparedEvidenceTaskResult{
			payload:    payload,
			path:       path,
			resultFile: resultFile,
			view: evidenceTaskView{
				Slug:              change.Slug,
				TaskID:            taskID,
				RunSummaryVersion: runSummary,
				Path:              state.DisplayPath(root, path),
				Recorded:          true,
				FreshnessInputs:   payload.FreshnessInputs,
			},
		})
	}
	if err := rejectDuplicateEvidenceTaskResultSessions(root, change, runSummary, prepared, existingTaskEvidencePaths); err != nil {
		return nil, err
	}
	return prepared, nil
}

type evidenceTaskResultSessionOwner struct {
	taskID     string
	resultFile string
}

func rejectDuplicateEvidenceTaskResultSessions(root string, change model.Change, runSummary int, prepared []preparedEvidenceTaskResult, existingTaskEvidencePaths []string) error {
	replacedTasks := map[string]struct{}{}
	for _, item := range prepared {
		replacedTasks[item.payload.TaskID] = struct{}{}
	}

	sessionOwners, err := existingEvidenceTaskResultSessionOwners(root, change, runSummary, replacedTasks, existingTaskEvidencePaths)
	if err != nil {
		return err
	}
	for _, item := range prepared {
		sessionID := strings.TrimSpace(item.payload.SessionID)
		if sessionID == "" {
			continue
		}
		if previous, ok := sessionOwners[sessionID]; ok && previous.taskID != item.payload.TaskID {
			return newInvalidUsageError(
				"evidence_task_result_file_duplicate_session",
				fmt.Sprintf("multiple task results use session_id %q", sessionID),
				"Use a distinct session_id per task executor in the active run, or omit session_id when no executor identity is available.",
				map[string]any{
					"session_id":            sessionID,
					"first_task_id":         previous.taskID,
					"duplicate_task_id":     item.payload.TaskID,
					"first_result_file":     previous.resultFile,
					"duplicate_result_file": item.resultFile,
				},
			)
		}
		sessionOwners[sessionID] = evidenceTaskResultSessionOwner{
			taskID:     item.payload.TaskID,
			resultFile: item.resultFile,
		}
	}
	return nil
}

func existingEvidenceTaskResultSessionOwners(
	root string,
	change model.Change,
	runSummary int,
	replacedTasks map[string]struct{},
	paths []string,
) (map[string]evidenceTaskResultSessionOwner, error) {
	sessionOwners := map[string]evidenceTaskResultSessionOwner{}
	for _, path := range paths {
		task, _, sessionID, err := progression.ParseTaskEvidence(root, path, runSummary)
		if err != nil {
			var versionMismatch progression.TaskEvidenceRunVersionMismatchError
			if errors.As(err, &versionMismatch) {
				continue
			}
			return nil, newStateIntegrityError(
				"evidence_task_existing_evidence_invalid",
				fmt.Sprintf("failed to parse existing task evidence %q: %v", state.DisplayPath(root, path), err),
				"Repair or remove malformed task evidence before importing task result evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, path)},
			)
		}
		if _, replaced := replacedTasks[task.TaskID]; replaced || strings.TrimSpace(sessionID) == "" {
			continue
		}
		sessionOwners[strings.TrimSpace(sessionID)] = evidenceTaskResultSessionOwner{
			taskID:     task.TaskID,
			resultFile: state.DisplayPath(root, path),
		}
	}
	return sessionOwners, nil
}

func writePreparedEvidenceTaskResults(prepared []preparedEvidenceTaskResult) error {
	ops := make([]fsutil.FileTransactionOp, 0, len(prepared))
	for _, item := range prepared {
		raw, err := marshalEvidenceTaskPayload(item.payload)
		if err != nil {
			return err
		}
		ops = append(ops, fsutil.WriteFileTransactionOp(item.path, raw, 0o644))
	}
	return fsutil.ApplyFileTransaction(ops)
}

func validateEvidenceSkillName(root, skillName string) (skill.Definition, error) {
	if skillName == "" {
		return skill.Definition{}, newInvalidUsageError(
			"evidence_skill_required",
			"--skill is required",
			"Pass a governance skill name such as plan-audit or spec-compliance-review.",
			nil,
		)
	}
	if strings.ContainsAny(skillName, `/\`) || skillName == "." || skillName == ".." {
		return skill.Definition{}, newInvalidUsageError(
			"evidence_skill_invalid",
			fmt.Sprintf("skill must be a safe governance skill name: %q", skillName),
			"Pass a governance skill name without path separators.",
			map[string]any{"skill": skillName},
		)
	}
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return skill.Definition{}, newStateIntegrityError(
			"evidence_skill_registry_load_failed",
			fmt.Sprintf("failed to load governance skill registry: %v", err),
			"Repair generated governance skill metadata and retry.",
			"",
			nil,
		)
	}
	def, ok := skill.LookupDefinitionInRegistry(registry, skillName)
	if !ok {
		return skill.Definition{}, newInvalidUsageError(
			"evidence_skill_unknown",
			fmt.Sprintf("unknown governance skill %q", skillName),
			"Use a governance skill registered in the active Slipway skill registry.",
			map[string]any{"skill": skillName},
		)
	}
	return def, nil
}

func evidenceSkillRunContext(root string, change model.Change, def skill.Definition) (int, *model.ExecutionSummary, error) {
	if !def.RunSummaryBound {
		return 0, nil, nil
	}
	execCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return 0, nil, err
	}
	// wave-orchestration evidence must prove every planned task has fresh task
	// evidence before its own record may be written — the "wave evidence is
	// recorded last, after every task" rule (issue #95). This holds at S2_IMPLEMENT
	// and, symmetrically, while completing an in-place S3_REVIEW convergence (a task
	// folded into tasks.md whose Door 1 evidence must precede the Door 2 wave
	// re-attestation). Without the S3 arm, Door 2 could write a misleading passing
	// wave record before the folded task's evidence existed.
	requiresWaveTaskCompleteness, err := waveOrchestrationEvidenceRequiresTaskCompleteness(root, change, def)
	if err != nil {
		return 0, nil, err
	}
	if requiresWaveTaskCompleteness {
		runVersion, taskErr := waveOrchestrationTaskEvidenceRunVersion(root, change)
		if taskErr != nil {
			return 0, nil, taskErr
		}
		if runVersion > execCtx.LatestRunVersion {
			return runVersion, &model.ExecutionSummary{
				Version:           model.ExecutionSummaryVersion,
				RunSummaryVersion: runVersion,
				CapturedAt:        time.Now().UTC(),
			}, nil
		}
	}
	if execCtx.LatestRunVersion >= 1 {
		return execCtx.LatestRunVersion, execCtx.Summary, nil
	}
	if requiresWaveTaskCompleteness {
		runVersion, err := waveOrchestrationTaskEvidenceRunVersion(root, change)
		if err != nil {
			return 0, nil, err
		}
		return runVersion, &model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: runVersion,
			CapturedAt:        time.Now().UTC(),
		}, nil
	}
	return 0, nil, newPreconditionError(
		"evidence_skill_run_summary_missing",
		fmt.Sprintf("skill %s requires a ready execution summary", def.Name),
		"Run `slipway run` until execution-summary.yaml is ready, then record the skill evidence.",
		change.Slug,
		map[string]any{"skill": def.Name},
	)
}

// waveOrchestrationEvidenceRequiresTaskCompleteness reports whether recording the
// given skill's evidence must first prove every planned task has fresh task
// evidence (the IncompleteExecutionTaskBlockers gate). It is true for
// wave-orchestration at S2_IMPLEMENT and, symmetrically, while an in-place
// S3_REVIEW convergence is still pending — keeping the S3 forward exit (Door 2)
// under the SAME completeness ordering S2 enforces, so a folded task's evidence
// (Door 1) must exist before the wave run is re-attested.
func waveOrchestrationEvidenceRequiresTaskCompleteness(root string, change model.Change, def skill.Definition) (bool, error) {
	if def.Name != progression.SkillWaveOrchestration {
		return false, nil
	}
	if change.CurrentState == model.StateS2Implement {
		return true, nil
	}
	return reviewWaveConvergenceReRecordAllowed(root, change, def.Name)
}

func waveOrchestrationTaskEvidenceRunVersion(root string, change model.Change) (int, error) {
	versions, scanErr := scanEvidenceTaskRunSummaryVersions(root, change.Slug)
	if scanErr != nil {
		switch scanErr.Kind {
		case evidenceTaskRunSummaryVersionsMissingDir:
			return 0, newInvalidUsageError(
				"evidence_skill_run_summary_missing",
				"wave-orchestration evidence requires task evidence before execution-summary.yaml exists",
				taskEvidenceResultFileRemediation,
				map[string]any{"skill": progression.SkillWaveOrchestration},
			)
		case evidenceTaskRunSummaryVersionsReadDir, evidenceTaskRunSummaryVersionsReadFile:
			return 0, newStateIntegrityError(
				"evidence_skill_task_evidence_load_failed",
				fmt.Sprintf("failed to read task evidence %q: %v", state.DisplayPath(root, scanErr.Path), scanErr.Err),
				"Repair the runtime task evidence before recording wave-orchestration evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		case evidenceTaskRunSummaryVersionsParseFile:
			return 0, newStateIntegrityError(
				"evidence_skill_task_evidence_invalid",
				fmt.Sprintf("failed to parse task evidence %q: %v", state.DisplayPath(root, scanErr.Path), scanErr.Err),
				"Regenerate task evidence with `slipway evidence task` before recording wave-orchestration evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		case evidenceTaskRunSummaryVersionsInvalidVersion:
			return 0, newStateIntegrityError(
				"evidence_skill_task_evidence_invalid",
				fmt.Sprintf("task evidence %q has invalid run_summary_version %d", state.DisplayPath(root, scanErr.Path), scanErr.RunSummaryVersion),
				"Regenerate task evidence with a run_summary_version >= 1 before recording wave-orchestration evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		}
	}
	if len(versions) == 0 {
		return 0, newInvalidUsageError(
			"evidence_skill_run_summary_missing",
			"wave-orchestration evidence requires task evidence before execution-summary.yaml exists",
			taskEvidenceResultFileRemediation,
			map[string]any{"skill": progression.SkillWaveOrchestration},
		)
	}
	if len(versions) > 1 {
		return 0, newInvalidUsageError(
			"evidence_skill_task_evidence_run_summary_ambiguous",
			"task evidence contains multiple run_summary_version values",
			"Re-record task evidence for a single wave-orchestration run_version before recording wave-orchestration evidence.",
			map[string]any{"skill": progression.SkillWaveOrchestration},
		)
	}

	runVersion := 0
	for version := range versions {
		runVersion = version
	}
	tasks, issues, err := progression.LoadExecutionTasksFromEvidence(root, change.Slug, runVersion)
	if err != nil {
		return 0, newStateIntegrityError(
			"evidence_skill_task_evidence_load_failed",
			fmt.Sprintf("failed to load task evidence for run_summary_version=%d: %v", runVersion, err),
			"Repair runtime task evidence before recording wave-orchestration evidence.",
			change.Slug,
			map[string]any{"run_summary_version": runVersion},
		)
	}
	if len(issues) > 0 {
		return 0, newStateIntegrityError(
			"evidence_skill_task_evidence_invalid",
			fmt.Sprintf("task evidence for run_summary_version=%d is invalid: %s", runVersion, strings.Join(issues, "; ")),
			"Regenerate invalid task evidence before recording wave-orchestration evidence.",
			change.Slug,
			map[string]any{"run_summary_version": runVersion},
		)
	}
	if len(tasks) == 0 {
		return 0, newInvalidUsageError(
			"evidence_skill_run_summary_missing",
			"wave-orchestration evidence requires task evidence before execution-summary.yaml exists",
			taskEvidenceResultFileRemediation,
			map[string]any{"skill": progression.SkillWaveOrchestration},
		)
	}
	wavePlan, err := loadCurrentWavePlanForCommand(root, change)
	if err != nil {
		if errors.Is(err, state.ErrWavePlanCacheUnreadable) {
			return 0, newWavePlanCacheUnreadableError(root, change, "wave-orchestration evidence", err)
		}
		return 0, newStateIntegrityError(
			"evidence_skill_wave_plan_unavailable",
			fmt.Sprintf("wave-orchestration evidence requires a current S2 wave projection for %q: %v", change.Slug, err),
			"Fix tasks.md so Slipway can derive the current S2 wave projection before recording wave-orchestration evidence.",
			change.Slug,
			map[string]any{"path": state.WavePlanPathForRead(root, change.Slug)},
		)
	}
	runs := make(map[string]model.TaskRun, len(tasks))
	for _, task := range tasks {
		runs[task.TaskID] = model.TaskRun{
			TaskID:  task.TaskID,
			Verdict: task.Verdict,
		}
	}
	if blockers := progression.IncompleteExecutionTaskBlockers(wavePlan, runs); len(blockers) > 0 {
		return 0, newInvalidUsageError(
			"evidence_skill_task_evidence_incomplete",
			"wave-orchestration evidence requires current task evidence for every planned task",
			"Record task evidence for every planned task in the active execution run before recording wave-orchestration evidence.",
			map[string]any{
				"skill":               progression.SkillWaveOrchestration,
				"run_summary_version": runVersion,
				"blockers":            blockers,
			},
		)
	}
	return runVersion, nil
}

func validateEvidenceSkillStage(root string, change model.Change, def skill.Definition) error {
	if def.DiscoveryOnly && !change.NeedsDiscovery {
		return newInvalidUsageError(
			"evidence_skill_not_applicable",
			fmt.Sprintf("skill %s applies only to discovery changes", def.Name),
			"Record evidence for the skill currently returned by `slipway next --json`.",
			map[string]any{"skill": def.Name},
		)
	}
	if change.CurrentState != def.State {
		// In-place stale re-cert: when this skill is the engine-flagged recoverable
		// stale-repair target, allow recording its fresh evidence at the current
		// state even though the change has advanced past the skill's owning state.
		// This mirrors the wrong-substep exception below (1431) and closes the
		// otherwise-circular dead end where an upstream skill (e.g. intake-clarification,
		// owned by S0_INTAKE) goes stale after the change has advanced to S1_PLAN:
		// `evidence skill` rejected it (wrong state), `intake`/`run` would not reopen
		// to S0, so there was no public path to re-certify. Fail-closed: this opens
		// ONLY for the skill the engine itself reports as a recoverable stale target,
		// ONLY while it is stale (gated on staleEvidenceSkillRefreshRequired).
		refreshRequired, err := staleEvidenceSkillRefreshRequired(root, change, def.Name)
		if err != nil {
			return err
		}
		// In-place review convergence: wave-orchestration is S2-owned, but folding a
		// task into tasks.md at S3 requires re-attesting the wave run so the fresh
		// wave record post-dates the folded task's evidence (symmetric to S2, where
		// wave evidence is always recorded last). This opens the door only while the
		// summary still carries an incomplete_execution_task blocker; it never grants
		// a bypass — the rebuilt summary fails closed on missing dispatch/handle
		// evidence for the folded task.
		convergence, err := reviewWaveConvergenceReRecordAllowed(root, change, def.Name)
		if err != nil {
			return err
		}
		if !refreshRequired && !convergence {
			return newInvalidUsageError(
				"evidence_skill_wrong_state",
				fmt.Sprintf("%s evidence requires %s state, current: %s", def.Name, def.State, change.CurrentState),
				evidenceSkillWrongStateRemediation(root, change, def),
				map[string]any{
					"skill":          def.Name,
					"expected":       def.State,
					"current":        change.CurrentState,
					"required_state": string(def.State),
					"current_state":  string(change.CurrentState),
				},
			)
		}
	}
	if def.State == model.StateS1Plan && def.PlanSubStep != model.PlanSubStepNone && change.PlanSubStep != def.PlanSubStep {
		refreshRequired, err := staleEvidenceSkillRefreshRequired(root, change, def.Name)
		if err != nil {
			return err
		}
		if refreshRequired {
			return nil
		}
		return newInvalidUsageError(
			"evidence_skill_wrong_plan_substep",
			fmt.Sprintf("%s evidence requires S1_PLAN/%s, current substep: %s", def.Name, def.PlanSubStep, change.PlanSubStep),
			fmt.Sprintf("Run the lifecycle to S1_PLAN/%s before recording %s evidence.", def.PlanSubStep, def.Name),
			map[string]any{
				"skill":            def.Name,
				"expected":         def.PlanSubStep,
				"current":          change.PlanSubStep,
				"required_state":   string(def.State),
				"required_substep": string(def.PlanSubStep),
				"current_state":    string(change.CurrentState),
				"current_substep":  string(change.PlanSubStep),
			},
		)
	}
	return nil
}

func staleEvidenceSkillRefreshRequired(root string, change model.Change, skillName string) (bool, error) {
	skillName = strings.TrimSpace(skillName)
	target, ok, err := progression.StaleEvidenceRepairAvailable(root, change, nil)
	if err != nil {
		return false, err
	}
	if ok && strings.TrimSpace(target.SkillName) == skillName {
		return true, nil
	}
	return readinessRequiredSkillStaleRefreshRequired(root, change, skillName)
}

func readinessRequiredSkillStaleRefreshRequired(root string, change model.Change, skillName string) (bool, error) {
	readiness, err := progression.EvaluateGovernanceReadiness(root, change, progression.GovernanceReadinessOptions{
		IncludeGateEvaluations: true,
	})
	if err != nil {
		return false, err
	}
	return hasRecoverableRequiredSkillStaleForSkill(readinessBlockers(readiness), skillName), nil
}

func readinessBlockers(readiness progression.GovernanceReadiness) []model.ReasonCode {
	blockers := append([]model.ReasonCode{}, readiness.SkillBlockers...)
	blockers = append(blockers, readiness.Blockers...)
	for _, evaluation := range readiness.GateEvaluations {
		blockers = append(blockers, evaluation.ReasonCodes...)
	}
	return model.NormalizeReasonCodes(blockers)
}

func hasRecoverableRequiredSkillStaleForSkill(blockers []model.ReasonCode, skillName string) bool {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return false
	}
	for _, blocker := range blockers {
		if strings.TrimSpace(blocker.Code) != "required_skill_stale" {
			continue
		}
		parsed := model.ParseBlocker(blocker)
		if strings.TrimSpace(parsed.Subject) != skillName {
			continue
		}
		if recoverableRequiredSkillStaleDetail(parsed.Detail) {
			return true
		}
	}
	return false
}

func recoverableRequiredSkillStaleDetail(detail string) bool {
	detail = strings.TrimSpace(detail)
	switch {
	case detail == "":
		return false
	case detail == "input_digest_unavailable" || strings.HasSuffix(detail, ":input_digest_unavailable"):
		return false
	case detail == "input_digest_missing" || strings.HasSuffix(detail, ":input_digest_missing"):
		return false
	default:
		return true
	}
}

func currentS3ReviewAlignmentActive(root string, change model.Change) (bool, error) {
	if change.CurrentState != model.StateS3Review {
		return false, nil
	}
	_, staleAvailable, err := progression.StaleEvidenceRepairAvailable(root, change, nil)
	if err != nil || !staleAvailable {
		return false, err
	}
	return true, nil
}

func selectedReviewContextOriginRefreshRequired(root string, change model.Change, skillName string) (bool, error) {
	return progression.SelectedReviewContextOriginInvalid(root, change, skillName)
}

func validateSelectedReviewPassContextOrigin(
	root string,
	change model.Change,
	def skill.Definition,
	record model.VerificationRecord,
) error {
	if change.CurrentState != model.StateS3Review || !record.IsPassing() {
		return nil
	}
	_, selectedReviewSkills, err := selectedReviewSkillsForChange(root, change)
	if err != nil {
		return err
	}
	if !stringInSlice(selectedReviewSkills, def.Name) {
		return nil
	}
	if _, ok := model.ExactlyOneReviewContextOriginHandleFromVerification(record); ok {
		return nil
	}
	notesPath := "artifacts/changes/" + change.Slug + "/verification/" + def.Name + "-notes.md"
	return newInvalidUsageError(
		"evidence_skill_review_context_origin_missing",
		fmt.Sprintf("selected S3 review evidence for %s must include exactly one context_origin:stage=review=<handle> reference", def.Name),
		fmt.Sprintf(
			"Re-run %s in a fresh subagent, or explicitly select a degraded fallback, then record evidence with --reference \"context_origin:stage=review=<handle>\" and --notes-file %s. Fallback mode is not a substitute for the review-stage context handle; record it as an additional reference such as --reference \"fallback:same_context_degraded\".",
			def.Name,
			notesPath,
		),
		map[string]any{
			"skill":                  def.Name,
			"selected_review_skills": selectedReviewSkills,
			"state":                  string(change.CurrentState),
			"notes_file":             notesPath,
		},
	)
}

func validatePlanDimensionSkillEvidence(
	root string,
	change model.Change,
	def skill.Definition,
	record model.VerificationRecord,
) error {
	if !record.IsPassing() {
		return nil
	}
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return err
	}
	if policy.EffectivePreset == model.WorkflowPresetLight {
		return nil
	}

	required := def.Name == progression.SkillPlanAudit
	if !required && def.Name == progression.SkillSpecComplianceReview && change.CurrentState == model.StateS3Review {
		_, selectedReviewSkills, err := selectedReviewSkillsForChange(root, change)
		if err != nil {
			return err
		}
		required = stringInSlice(selectedReviewSkills, def.Name)
	}
	if !required {
		return nil
	}

	workspaceRoot, err := state.WorkspaceRootForChange(root, change)
	if err != nil {
		return newStateIntegrityError(
			"evidence_skill_plan_dimension_workspace_resolve_failed",
			fmt.Sprintf("failed to resolve plan-dimension evidence workspace for %q: %v", change.Slug, err),
			"Repair the governed change worktree binding and retry.",
			change.Slug,
			map[string]any{"skill": def.Name},
		)
	}
	_, blockers := model.RequiredPlanDimensionAttestationBlockers(workspaceRoot, record)
	if len(blockers) == 0 {
		return nil
	}
	return newInvalidUsageError(
		"evidence_skill_plan_dimension_attestation_invalid",
		fmt.Sprintf("passing %s evidence must include passing plan-dimension attestations", def.Name),
		planDimensionEvidenceRemediation(change, def.Name),
		map[string]any{
			"skill":      def.Name,
			"blockers":   model.ReasonSpecs(blockers),
			"references": record.References,
		},
	)
}

func planDimensionEvidenceRemediation(change model.Change, skillName string) string {
	notesPath := "artifacts/changes/" + change.Slug + "/verification/" + skillName + "-notes.md"
	return fmt.Sprintf(
		"Re-run %s and record explicit dimension evidence, for example --reference \"dim:decision_soundness=pass:<repo-path-outside-artifacts>\" --reference \"dim:consistency=pass:<repo-path-or-artifact-line>\" --notes-file %s. Use --verdict fail with matching --blocker values when a dimension fails.",
		skillName,
		notesPath,
	)
}

func evidenceTaskWrongStateRemediation(_ string, _ model.Change) string {
	// S2_IMPLEMENT and S3_REVIEW are both recordable states (S3 completes the
	// in-place review convergence for a folded-in task), so this remediation is
	// only reached from other states.
	return "Record task evidence during wave execution (S2_IMPLEMENT), or to complete an in-place review convergence (S3_REVIEW)."
}

// assertTaskEvidenceConvergenceAtReview enforces the S3_REVIEW recording
// contract: at review, task evidence may only COMPLETE the in-place convergence
// — record proof for a task the re-materialized wave plan surfaced as incomplete
// (a folded-in task with no evidence yet at the active run). An already-evidenced
// task is frozen at review: its plan/code drift is realigned through the
// diff-scoped reviewers, never by restamping task evidence (which would forge
// fresh freshness state at review). It is a no-op outside S3_REVIEW.
func assertTaskEvidenceConvergenceAtReview(root string, change model.Change, taskID string) error {
	if change.CurrentState != model.StateS3Review {
		return nil
	}
	taskID = strings.TrimSpace(taskID)
	path := filepath.Join(state.EvidenceTasksDir(root, change.Slug), taskID+".json")
	switch _, err := os.Stat(path); {
	case err == nil:
		return newInvalidUsageError(
			"evidence_task_already_recorded_at_review",
			fmt.Sprintf("task %q already has recorded evidence; at S3_REVIEW task evidence is recordable only for a task the plan surfaced as incomplete", taskID),
			postReviewReplacementEvidenceRemediation(root, change, "task evidence"),
			map[string]any{"task_id": taskID, "state": string(change.CurrentState)},
		)
	case os.IsNotExist(err):
		return nil
	default:
		return newStateIntegrityError(
			"evidence_task_review_state_unreadable",
			fmt.Sprintf("cannot determine task evidence state for %q: %v", taskID, err),
			"Repair the governed runtime directory and retry.",
			change.Slug,
			map[string]any{"task_id": taskID, "path": state.DisplayPath(root, path)},
		)
	}
}

// reviewWaveConvergenceReRecordAllowed reports whether wave-orchestration evidence
// may be re-recorded at S3_REVIEW to complete an in-place convergence. When a task
// is folded into tasks.md at review, the re-materialized wave plan surfaces it as
// incomplete_execution_task and the existing wave record predates the folded
// task's freshly recorded task evidence. Re-attesting the wave run — a fresh wave
// record that accounts for the new task's dispatch — is the symmetric forward exit:
// it is the same "wave-orchestration evidence is recorded last, after every task's
// evidence" rule S2 already enforces, applied to the convergence. It carries no
// bypass: the host must still supply dispatch/handle evidence for every planned
// task, and the folded task's own evidence must exist, or the rebuilt summary stays
// incomplete and fails closed. It opens ONLY at S3_REVIEW, ONLY for
// wave-orchestration, and ONLY while the persisted execution summary still carries
// an incomplete_execution_task blocker (the engine's own folded-task signal).
func reviewWaveConvergenceReRecordAllowed(root string, change model.Change, skillName string) (bool, error) {
	if change.CurrentState != model.StateS3Review {
		return false, nil
	}
	if strings.TrimSpace(skillName) != progression.SkillWaveOrchestration {
		return false, nil
	}
	summary, err := state.LoadOptionalExecutionSummary(root, change.Slug)
	if err != nil {
		return false, err
	}
	return progression.ExecutionSummaryHasIncompleteTask(summary), nil
}

func evidenceSkillWrongStateRemediation(root string, change model.Change, def skill.Definition) string {
	switch change.CurrentState {
	case model.StateS3Review:
		if def.State == model.StateS2Implement {
			return postReviewReplacementEvidenceRemediation(root, change, def.Name+" evidence")
		}
	}
	return fmt.Sprintf("Run the lifecycle to %s before recording %s evidence.", def.State, def.Name)
}

func postReviewReplacementEvidenceRemediation(root string, change model.Change, surface string) string {
	reviewSkills := selectedReviewSkillsForRemediation(root, change)
	return fmt.Sprintf(
		"%s is S2-only after wave execution. For review-driven repairs or tests, record fresh proof for %s evidence, then rerun %s. If the work needs a fresh execution run (for example a task added to tasks.md at review that the wave projection does not contain), reopen execution with `slipway fix --start-reexecution`.",
		surface,
		strings.Join(reviewSkills, ", "),
		progression.SkillShipVerification,
	)
}

func selectedReviewSkillsForRemediation(root string, change model.Change) []string {
	if strings.TrimSpace(root) != "" {
		_, selected, err := selectedReviewSkillsForChange(root, change)
		if err == nil && len(selected) > 0 {
			return selected
		}
	}
	return skill.SelectedReviewSkills(skill.ReviewSkillSelection{})
}

func resolveEvidenceSkillNotes(root string, change model.Change, notes, notesFile string) (string, error) {
	notes = strings.TrimSpace(notes)
	notesFile = strings.TrimSpace(notesFile)
	if notes != "" && notesFile != "" {
		return "", newInvalidUsageError(
			"evidence_skill_notes_conflict",
			"pass either --notes or --notes-file, not both",
			"Use --notes for a bounded inline summary or --notes-file for a disk handoff.",
			nil,
		)
	}
	if notesFile == "" {
		return notes, nil
	}
	if err := validateEvidencePath(notesFile); err != nil {
		return "", newInvalidUsageError(
			"evidence_skill_notes_file_invalid",
			err.Error(),
			"Pass a workspace-relative notes file path without absolute paths, empty segments, or parent traversal.",
			nil,
		)
	}
	workspaceRoot, err := state.WorkspaceRootForChange(root, change)
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_skill_notes_workspace_resolve_failed",
			fmt.Sprintf("failed to resolve notes workspace for %q: %v", change.Slug, err),
			"Repair the governed change worktree binding and retry.",
			change.Slug,
			map[string]any{"path": notesFile},
		)
	}
	path := filepath.Join(workspaceRoot, filepath.FromSlash(model.NormalizePublicPath(notesFile)))
	raw, err := os.ReadFile(path) // #nosec G304 -- path is validated as workspace-relative before reading.
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_skill_notes_file_read_failed",
			fmt.Sprintf("failed to read notes file %q: %v", notesFile, err),
			"Write the delegated review or verification notes to the referenced workspace-relative path and retry.",
			change.Slug,
			map[string]any{
				"path":           notesFile,
				"resolved_path":  state.DisplayPath(root, path),
				"workspace_root": state.DisplayPath(root, workspaceRoot),
			},
		)
	}
	return strings.TrimSpace(string(raw)), nil
}

func trimNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func validateEvidenceTaskID(taskID string) error {
	if err := model.ValidateTaskID(taskID); err != nil {
		return err
	}
	if strings.ContainsAny(taskID, `/\`) || taskID == "." || taskID == ".." {
		return fmt.Errorf("task_id must be a safe flat filename: %q", taskID)
	}
	return nil
}

func validateEvidenceTaskRunSummaryVersion(root string, change model.Change, runSummary int) error {
	record, found, err := progression.LatestPassingWaveEvidence(root, change.Slug)
	if err != nil {
		return newStateIntegrityError(
			"evidence_task_wave_evidence_load_failed",
			fmt.Sprintf("failed to load wave-orchestration evidence for %q: %v", change.Slug, err),
			"Repair or remove malformed wave-orchestration evidence before recording task evidence.",
			change.Slug,
			nil,
		)
	}
	if !found || record.RunVersion < 1 || record.RunVersion == runSummary {
		return nil
	}
	if plan, err := state.LoadOptionalWavePlanForChange(root, change); err == nil &&
		plan != nil &&
		plan.RunSummaryVersion == runSummary &&
		plan.RunSummaryVersion > record.RunVersion {
		return nil
	}
	return newInvalidUsageError(
		"evidence_task_run_summary_version_mismatch",
		fmt.Sprintf("--run-summary-version %d does not match existing wave-orchestration run_version %d", runSummary, record.RunVersion),
		"Use the run_version from the active wave-orchestration evidence, or clear stale execution evidence before starting a new run.",
		map[string]any{
			"expected": record.RunVersion,
			"got":      runSummary,
		},
	)
}

func parseEvidenceTaskCapturedAt(raw string, commandCapturedAt time.Time) (time.Time, error) {
	commandCapturedAt = commandCapturedAt.UTC()
	if commandCapturedAt.IsZero() {
		commandCapturedAt = time.Now().UTC()
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return commandCapturedAt, nil
	}
	capturedAt, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("captured_at must be RFC3339Nano: %w", err)
	}
	capturedAt = capturedAt.UTC()
	if capturedAt.After(commandCapturedAt) {
		return time.Time{}, fmt.Errorf("captured_at must not be in the future")
	}
	return capturedAt, nil
}

type evidenceTaskResultFile struct {
	TaskID       string            `json:"task_id"`
	Verdict      model.TaskVerdict `json:"verdict"`
	EvidenceRef  string            `json:"evidence_ref"`
	ChangedFiles []string          `json:"changed_files"`
	Blockers     []string          `json:"blockers"`
	SessionID    string            `json:"session_id"`
}

func rejectEvidenceTaskResultFileLedgerFlags(cmd *cobra.Command) error {
	for _, flagName := range []string{
		"task-id",
		"run-summary-version",
		"task-kind",
		"verdict",
		"evidence-ref",
		"changed-file",
		"target-file",
		"blocker",
		"captured-at",
		"session-id",
	} {
		if cmd.Flags().Changed(flagName) {
			return newInvalidUsageError(
				"evidence_task_result_file_mixed_mode",
				fmt.Sprintf("--result-file cannot be combined with --%s", flagName),
				"Use --result-file by itself for compact executor results; Slipway derives ledger fields from the current wave plan and reads task-owned verdict, evidence_ref, changed_files, blockers, and session_id from each result file.",
				map[string]any{"flag": "--" + flagName},
			)
		}
	}
	return nil
}

func loadEvidenceTaskResultFile(root string, change model.Change, resultFile string) (evidenceTaskResultFile, error) {
	resultFile = strings.TrimSpace(resultFile)
	if resultFile == "" {
		return evidenceTaskResultFile{}, newInvalidUsageError(
			"evidence_task_result_file_required",
			"--result-file requires a path",
			"Pass a workspace-relative executor result JSON file path.",
			nil,
		)
	}
	path, err := resolveEvidenceTaskResultPath(root, change, resultFile)
	if err != nil {
		return evidenceTaskResultFile{}, err
	}
	// The result path is resolved and scoped before open. The file content is
	// still fully revalidated after open, so a concurrent replacement can only
	// produce invalid or out-of-scope evidence, not trusted ledger fields.
	file, err := os.Open(path) // #nosec G304 -- path is validated, symlink-resolved, and scoped to the workspace root before opening.
	if err != nil {
		return evidenceTaskResultFile{}, newStateIntegrityError(
			"evidence_task_result_file_read_failed",
			fmt.Sprintf("failed to read task result file %q: %v", resultFile, err),
			"Write the executor result JSON file and retry `slipway evidence task --result-file <path>`.",
			change.Slug,
			map[string]any{"path": resultFile},
		)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return evidenceTaskResultFile{}, newStateIntegrityError(
			"evidence_task_result_file_read_failed",
			fmt.Sprintf("failed to stat task result file %q: %v", resultFile, err),
			"Write the executor result JSON file and retry `slipway evidence task --result-file <path>`.",
			change.Slug,
			map[string]any{"path": resultFile},
		)
	}
	if info.Size() > maxEvidenceTaskResultFileBytes {
		return evidenceTaskResultFile{}, newInvalidUsageError(
			"evidence_task_result_file_too_large",
			fmt.Sprintf("task result file %q is too large: %d bytes exceeds %d bytes", resultFile, info.Size(), maxEvidenceTaskResultFileBytes),
			"Write a compact executor result JSON file with only task_id, verdict, evidence_ref, changed_files, blockers, and optional session_id.",
			map[string]any{
				"path":      resultFile,
				"size":      info.Size(),
				"max_bytes": maxEvidenceTaskResultFileBytes,
			},
		)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxEvidenceTaskResultFileBytes+1))
	if err != nil {
		return evidenceTaskResultFile{}, newStateIntegrityError(
			"evidence_task_result_file_read_failed",
			fmt.Sprintf("failed to read task result file %q: %v", resultFile, err),
			"Write the executor result JSON file and retry `slipway evidence task --result-file <path>`.",
			change.Slug,
			map[string]any{"path": resultFile},
		)
	}
	if int64(len(raw)) > maxEvidenceTaskResultFileBytes {
		return evidenceTaskResultFile{}, newInvalidUsageError(
			"evidence_task_result_file_too_large",
			fmt.Sprintf("task result file %q is too large: exceeds %d bytes", resultFile, maxEvidenceTaskResultFileBytes),
			"Write a compact executor result JSON file with only task_id, verdict, evidence_ref, changed_files, blockers, and optional session_id.",
			map[string]any{
				"path":      resultFile,
				"max_bytes": maxEvidenceTaskResultFileBytes,
			},
		)
	}
	// Keep this deny-list aligned with evidenceTaskResultFile: that struct must
	// remain executor-only, while Slipway owns every durable ledger field.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return evidenceTaskResultFile{}, newInvalidUsageError(
			"evidence_task_result_file_invalid",
			fmt.Sprintf("task result file is not valid JSON: %v", err),
			"Write executor result JSON with task_id, verdict, evidence_ref, changed_files, blockers, and optional session_id.",
			map[string]any{"path": resultFile},
		)
	}
	for _, field := range []string{
		"run_summary_version",
		"task_kind",
		"target_files",
		"captured_at",
		"freshness_inputs",
		"input_hash",
	} {
		if _, ok := envelope[field]; ok {
			return evidenceTaskResultFile{}, newInvalidUsageError(
				"evidence_task_result_file_ledger_field",
				fmt.Sprintf("task result file must not include Slipway-owned ledger field %q", field),
				"Remove ledger-owned fields from the executor result; Slipway derives run_summary_version, task_kind, target_files, captured_at, and freshness_inputs.",
				map[string]any{"field": field},
			)
		}
	}

	var result evidenceTaskResultFile
	if err := json.Unmarshal(raw, &result); err != nil {
		return evidenceTaskResultFile{}, newInvalidUsageError(
			"evidence_task_result_file_invalid",
			fmt.Sprintf("task result file has invalid schema: %v", err),
			"Write executor result JSON with task_id, verdict, evidence_ref, changed_files, blockers, and optional session_id.",
			map[string]any{"path": resultFile},
		)
	}
	result.TaskID = strings.TrimSpace(result.TaskID)
	result.EvidenceRef = strings.TrimSpace(result.EvidenceRef)
	result.SessionID = strings.TrimSpace(result.SessionID)
	return result, nil
}

func resolveEvidenceTaskResultPath(root string, change model.Change, resultFile string) (string, error) {
	if filepath.IsAbs(resultFile) || model.PublicPathIsAbs(resultFile) {
		return "", newInvalidUsageError(
			"evidence_task_result_file_path_invalid",
			fmt.Sprintf("path must be workspace-relative: %q", resultFile),
			"Pass a workspace-relative result file path without absolute paths, empty segments, parent traversal, or symlink escapes.",
			map[string]any{"path": resultFile},
		)
	}
	if err := validateEvidencePath(resultFile); err != nil {
		return "", newInvalidUsageError(
			"evidence_task_result_file_path_invalid",
			err.Error(),
			"Pass a workspace-relative result file path without absolute paths, empty segments, parent traversal, or symlink escapes.",
			map[string]any{"path": resultFile},
		)
	}
	workspaceRoot, err := state.WorkspaceRootForChange(root, change)
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_task_result_file_workspace_resolve_failed",
			fmt.Sprintf("failed to resolve workspace for %q: %v", change.Slug, err),
			"Repair the governed change worktree binding and retry.",
			change.Slug,
			map[string]any{"path": resultFile},
		)
	}
	resolvedRoot, err := fsutil.RealExistingPath(workspaceRoot)
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_task_result_file_workspace_resolve_failed",
			fmt.Sprintf("failed to resolve workspace root for %q: %v", change.Slug, err),
			"Repair the governed change worktree binding and retry.",
			change.Slug,
			map[string]any{"path": resultFile},
		)
	}
	path := filepath.Join(workspaceRoot, filepath.FromSlash(model.NormalizePublicPath(resultFile)))
	resolvedPath, err := fsutil.RealExistingPath(path)
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_task_result_file_read_failed",
			fmt.Sprintf("failed to resolve task result file %q: %v", resultFile, err),
			"Write the executor result JSON file inside the workspace and retry `slipway evidence task --result-file <path>`.",
			change.Slug,
			map[string]any{
				"path":           resultFile,
				"resolved_path":  state.DisplayPath(root, path),
				"workspace_root": state.DisplayPath(root, workspaceRoot),
			},
		)
	}
	if !fsutil.PathWithin(resolvedRoot, resolvedPath) {
		return "", newInvalidUsageError(
			"evidence_task_result_file_path_invalid",
			fmt.Sprintf("task result file %q resolves outside the workspace", resultFile),
			"Pass a workspace-relative result file path that does not traverse or symlink outside the workspace.",
			map[string]any{
				"path":           resultFile,
				"resolved_path":  state.DisplayPath(root, resolvedPath),
				"workspace_root": state.DisplayPath(root, resolvedRoot),
			},
		)
	}
	return resolvedPath, nil
}

func deriveEvidenceTaskRunSummaryVersion(root string, change model.Change, wavePlan model.WavePlan) (int, []string, error) {
	if wavePlan.RunSummaryVersion < 1 {
		return 0, nil, newStateIntegrityError(
			"evidence_task_run_summary_version_unavailable",
			fmt.Sprintf("current wave plan for %q has invalid run_summary_version %d", change.Slug, wavePlan.RunSummaryVersion),
			"Rematerialize the S2 wave plan before importing task result evidence.",
			change.Slug,
			map[string]any{"run_summary_version": wavePlan.RunSummaryVersion},
		)
	}
	versions, paths, err := existingEvidenceTaskRunSummaryVersions(root, change.Slug)
	if err != nil {
		return 0, nil, err
	}
	if len(versions) > 1 {
		return 0, nil, newInvalidUsageError(
			"evidence_task_run_summary_version_ambiguous",
			"task evidence contains multiple run_summary_version values",
			"Clear, repair, or re-record task evidence so every task belongs to the active execution run before importing more results.",
			map[string]any{
				"slug":                     change.Slug,
				"active_run_summary":       wavePlan.RunSummaryVersion,
				"remediation_command_hint": "slipway fix --start-reexecution",
			},
		)
	}
	for version := range versions {
		if version > wavePlan.RunSummaryVersion {
			return 0, nil, newInvalidUsageError(
				"evidence_task_run_summary_version_ambiguous",
				"task evidence contains a run_summary_version newer than the active wave plan",
				"Clear, repair, or re-record task evidence so every task belongs to the active execution run before importing more results.",
				map[string]any{
					"slug":                     change.Slug,
					"active_run_summary":       wavePlan.RunSummaryVersion,
					"existing_run_summary":     version,
					"remediation_command_hint": "slipway fix --start-reexecution",
				},
			)
		}
	}
	return wavePlan.RunSummaryVersion, paths, nil
}

// existingEvidenceTaskRunSummaryVersions enumerates the evidence-tasks directory
// once, returning both the set of existing run_summary_version values and the
// sorted list of task-evidence file paths it walked so the same listing can feed
// the session-owner scan without walking the directory a second time.
func existingEvidenceTaskRunSummaryVersions(root, slug string) (map[int]struct{}, []string, error) {
	dir := state.EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, newStateIntegrityError(
			"evidence_task_existing_evidence_load_failed",
			fmt.Sprintf("failed to read existing task evidence %q: %v", state.DisplayPath(root, dir), err),
			"Repair the runtime task evidence before importing task result evidence.",
			slug,
			map[string]any{"path": state.DisplayPath(root, dir)},
		)
	}
	paths := evidenceTaskJSONPaths(dir, entries)
	versions, scanErr := runSummaryVersionsFromPaths(paths)
	if scanErr != nil {
		switch scanErr.Kind {
		case evidenceTaskRunSummaryVersionsReadFile:
			return nil, nil, newStateIntegrityError(
				"evidence_task_existing_evidence_load_failed",
				fmt.Sprintf("failed to read existing task evidence %q: %v", state.DisplayPath(root, scanErr.Path), scanErr.Err),
				"Repair the runtime task evidence before importing task result evidence.",
				slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		case evidenceTaskRunSummaryVersionsParseFile:
			return nil, nil, newStateIntegrityError(
				"evidence_task_existing_evidence_invalid",
				fmt.Sprintf("failed to parse existing task evidence %q: %v", state.DisplayPath(root, scanErr.Path), scanErr.Err),
				"Repair or remove malformed task evidence before importing task result evidence.",
				slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		case evidenceTaskRunSummaryVersionsInvalidVersion:
			return nil, nil, newStateIntegrityError(
				"evidence_task_existing_evidence_invalid",
				fmt.Sprintf("existing task evidence %q has invalid run_summary_version %d", state.DisplayPath(root, scanErr.Path), scanErr.RunSummaryVersion),
				"Repair or remove invalid task evidence before importing task result evidence.",
				slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		}
	}
	return versions, paths, nil
}

const (
	evidenceTaskRunSummaryVersionsMissingDir     = "missing_dir"
	evidenceTaskRunSummaryVersionsReadDir        = "read_dir"
	evidenceTaskRunSummaryVersionsReadFile       = "read_file"
	evidenceTaskRunSummaryVersionsParseFile      = "parse_file"
	evidenceTaskRunSummaryVersionsInvalidVersion = "invalid_version"
)

type evidenceTaskRunSummaryVersionsError struct {
	Kind              string
	Path              string
	Err               error
	RunSummaryVersion int
}

func scanEvidenceTaskRunSummaryVersions(root, slug string) (map[int]struct{}, *evidenceTaskRunSummaryVersionsError) {
	dir := state.EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &evidenceTaskRunSummaryVersionsError{
				Kind: evidenceTaskRunSummaryVersionsMissingDir,
				Path: dir,
				Err:  err,
			}
		}
		return nil, &evidenceTaskRunSummaryVersionsError{
			Kind: evidenceTaskRunSummaryVersionsReadDir,
			Path: dir,
			Err:  err,
		}
	}
	return runSummaryVersionsFromPaths(evidenceTaskJSONPaths(dir, entries))
}

// evidenceTaskJSONPaths projects the already-enumerated directory entries into the
// sorted list of task-evidence file paths, preserving os.ReadDir ordering and the
// directory/.json filtering both evidence-tasks scans applied independently before.
func evidenceTaskJSONPaths(dir string, entries []os.DirEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	return paths
}

// runSummaryVersionsFromPaths collects the set of run_summary_version values from
// the given task-evidence file paths. It is the per-file version scan shared by the
// wave-orchestration path and the single-walk result-import path.
func runSummaryVersionsFromPaths(paths []string) (map[int]struct{}, *evidenceTaskRunSummaryVersionsError) {
	versions := map[int]struct{}{}
	for _, path := range paths {
		raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway runtime evidence authority.
		if err != nil {
			return nil, &evidenceTaskRunSummaryVersionsError{
				Kind: evidenceTaskRunSummaryVersionsReadFile,
				Path: path,
				Err:  err,
			}
		}
		var payload struct {
			RunSummaryVersion int `json:"run_summary_version"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, &evidenceTaskRunSummaryVersionsError{
				Kind: evidenceTaskRunSummaryVersionsParseFile,
				Path: path,
				Err:  err,
			}
		}
		if payload.RunSummaryVersion < 1 {
			return nil, &evidenceTaskRunSummaryVersionsError{
				Kind:              evidenceTaskRunSummaryVersionsInvalidVersion,
				Path:              path,
				RunSummaryVersion: payload.RunSummaryVersion,
			}
		}
		versions[payload.RunSummaryVersion] = struct{}{}
	}
	return versions, nil
}

func findEvidenceWavePlanTask(plan model.WavePlan, taskID string) (model.WavePlanTask, bool) {
	for _, wave := range plan.Waves {
		for _, task := range wave.Tasks {
			if strings.TrimSpace(task.TaskID) == taskID {
				return task, true
			}
		}
	}
	return model.WavePlanTask{}, false
}

// taskPlannedButNotInWavePlan reports whether taskID is declared by the current
// tasks.md but absent from the materialized wave projection — i.e. it was added at
// S3_REVIEW after S2 execution. Such a task cannot be evidenced in place, so the
// public refresh is to reopen execution (`slipway fix --start-reexecution`). It is
// restricted to S3_REVIEW, where the added task is the genuine dead end and the
// reexecution route is valid.
func taskPlannedButNotInWavePlan(root string, change model.Change, wavePlan model.WavePlan, taskID string) (bool, error) {
	if change.CurrentState != model.StateS3Review {
		return false, nil
	}
	if _, ok := findEvidenceWavePlanTask(wavePlan, taskID); ok {
		return false, nil
	}
	plannedIDs, err := state.CurrentTasksPlanTaskIDs(root, change)
	if err != nil {
		return false, err
	}
	taskID = strings.TrimSpace(taskID)
	for _, id := range plannedIDs {
		if strings.TrimSpace(id) == taskID {
			return true, nil
		}
	}
	return false, nil
}

func normalizeEvidencePaths(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, rawPath := range paths {
		trimmed := model.NormalizePublicPath(rawPath)
		if trimmed == "" {
			continue
		}
		if err := validateEvidencePath(rawPath); err != nil {
			return nil, err
		}
		out = append(out, trimmed)
	}
	out = stringutil.UniqueSorted(out)
	return out, nil
}

func validateEvidencePath(path string) error {
	trimmed := model.NormalizePublicPath(path)
	if trimmed == "" {
		return nil
	}
	if model.PublicPathIsAbs(path) {
		return fmt.Errorf("path must be workspace-relative: %q", path)
	}
	for _, segment := range strings.Split(strings.ReplaceAll(path, "\\", "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("path must not contain empty, current, or parent segments: %q", path)
		}
	}
	if strings.ContainsAny(trimmed, "*?[") {
		if _, err := filepath.Match(filepath.FromSlash(trimmed), ""); err != nil {
			return fmt.Errorf("path glob is invalid: %q: %w", path, err)
		}
	}
	return nil
}

func writeEvidenceTaskPayload(path string, payload progression.TaskEvidencePayload) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	raw, err := marshalEvidenceTaskPayload(payload)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}

func marshalEvidenceTaskPayload(payload progression.TaskEvidencePayload) ([]byte, error) {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	return raw, nil
}

func readExistingVerificationRaw(root, slug, skillName string) ([]byte, bool, error) {
	path := state.VerificationFilePath(root, slug, skillName)
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state before rollback capture.
	if err == nil {
		return raw, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, err
}

func restoreVerificationRaw(path string, previousRaw []byte, hadPrevious bool) error {
	if hadPrevious {
		return fsutil.WriteFileAtomic(path, previousRaw, 0o644)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func restoreVerificationSuffix(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("; rollback failed: %v", err)
}

func validateEvidenceSkillActionable(root string, change model.Change, def skill.Definition, runVersion int, refreshCurrent bool) error {
	if change.CurrentState == model.StateS3Review {
		reviewSelection, selectedReviewSkills, err := selectedReviewSkillsForChange(root, change)
		if err != nil {
			return err
		}
		passing, err := currentPassingEvidenceSkillsWithReviewSelection(root, change, model.StateS3Review, runVersion, reviewSelection)
		if err != nil {
			return err
		}
		if stringInSlice(selectedReviewSkills, def.Name) {
			if _, ok := passing[def.Name]; ok {
				refreshRequired, err := selectedReviewContextOriginRefreshRequired(root, change, def.Name)
				if err != nil {
					return err
				}
				if refreshRequired {
					return nil
				}
				repairActive, err := currentS3ReviewAlignmentActive(root, change)
				if err != nil {
					return err
				}
				if repairActive {
					return nil
				}
				if refreshCurrent {
					return nil
				}
				return newInvalidUsageError(
					"evidence_skill_not_current",
					fmt.Sprintf("skill %s already has passing evidence for the current review set", def.Name),
					"Run `slipway next --json` and record evidence only for a selected review skill that is still missing or stale. If an operator intentionally reran this selected review skill, rerun with `--refresh-current` to replace the current evidence.",
					map[string]any{"skill": def.Name},
				)
			}
			return nil
		}
		if skill.IsReviewSkill(def.Name) {
			return newInvalidUsageError(
				"evidence_skill_not_current",
				fmt.Sprintf("skill %s is not currently recordable", def.Name),
				"Run `slipway next --json` and record evidence only for a selected review skill.",
				map[string]any{"skill": def.Name},
			)
		}
		// Wave-orchestration is not a review skill, so the actionable-skill ordering
		// below would reject it at review. Re-recording it IS the current action when
		// a folded-in task is converging in place: allow it while that convergence is
		// pending (gated identically to the wrong-state door above).
		if convergence, err := reviewWaveConvergenceReRecordAllowed(root, change, def.Name); err != nil {
			return err
		} else if convergence {
			return nil
		}
		actionable, err := currentActionableEvidenceSkill(root, change, runVersion)
		if err != nil {
			return err
		}
		if actionable == def.Name {
			return nil
		}
		if actionable != "" {
			return newInvalidUsageError(
				"evidence_skill_predecessor_required",
				fmt.Sprintf("skill %s cannot be recorded before %s passes", def.Name, actionable),
				"Record evidence only for the current actionable skill returned by `slipway next --json`.",
				map[string]any{
					"skill":          def.Name,
					"required_first": actionable,
				},
			)
		}
		return nil
	}

	refreshRequired, err := staleEvidenceSkillRefreshRequired(root, change, def.Name)
	if err != nil {
		return err
	}
	if refreshRequired {
		return nil
	}

	actionable, err := currentActionableEvidenceSkill(root, change, runVersion)
	if err != nil {
		return err
	}
	if actionable == def.Name {
		return nil
	}
	if actionable != "" {
		return newInvalidUsageError(
			"evidence_skill_predecessor_required",
			fmt.Sprintf("skill %s cannot be recorded before %s passes", def.Name, actionable),
			"Record evidence only for the current actionable skill returned by `slipway next --json`.",
			map[string]any{
				"skill":          def.Name,
				"required_first": actionable,
			},
		)
	}
	return newInvalidUsageError(
		"evidence_skill_not_current",
		fmt.Sprintf("skill %s is not currently recordable", def.Name),
		"Run `slipway next --json` and record evidence only for the current actionable skill.",
		map[string]any{"skill": def.Name},
	)
}

func currentActionableEvidenceSkill(root string, change model.Change, runVersion int) (string, error) {
	switch change.CurrentState {
	case model.StateS3Review:
		reviewSelection, selectedReviewSkills, err := selectedReviewSkillsForChange(root, change)
		if err != nil {
			return "", err
		}
		passing, err := currentPassingEvidenceSkillsWithReviewSelection(root, change, model.StateS3Review, runVersion, reviewSelection)
		if err != nil {
			return "", err
		}
		for _, skillName := range selectedReviewSkills {
			if _, ok := passing[skillName]; !ok {
				return skillName, nil
			}
		}
		// ship-verification is the single always-required terminal S3 skill; it
		// runs last, after the selected review set, before the governed ship
		// decision.
		if _, ok := passing[progression.SkillShipVerification]; !ok {
			return progression.SkillShipVerification, nil
		}
		return "", nil
	default:
		// S3_REVIEW is handled by the explicit case above; the remaining states
		// resolve a single skill, so the conventional primary is the full skill
		// set here.
		nextSkill, _ := progression.PrimaryNextSkill(change)
		if nextSkill == progression.SkillWorktreePreflight {
			return "", nil
		}
		return nextSkill, nil
	}
}

func currentPassingEvidenceSkillsWithReviewSelection(
	root string,
	change model.Change,
	workflowState model.WorkflowState,
	runVersion int,
	reviewSelection skill.ReviewSkillSelection,
) (map[string]model.VerificationRecord, error) {
	passing, _, err := progression.EvaluateRequiredSkillsForChangeWithReviewSelection(
		root,
		change,
		workflowState,
		runVersion,
		reviewSelection,
	)
	return passing, err
}

func stringInSlice(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
