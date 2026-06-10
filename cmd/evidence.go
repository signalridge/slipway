package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	Path              string                             `json:"path"`
	Recorded          bool                               `json:"recorded"`
	FreshnessInputs   model.ExecutionTaskFreshnessInputs `json:"freshness_inputs"`
}

type evidenceSkillView struct {
	Slug       string   `json:"slug"`
	Skill      string   `json:"skill"`
	RunVersion int      `json:"run_version"`
	Path       string   `json:"path"`
	Recorded   bool     `json:"recorded"`
	Stamped    bool     `json:"stamped"`
	References []string `json:"references,omitempty"`
}

func makeEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: desc("evidence"),
	}
	cmd.AddCommand(makeEvidenceTaskCmd())
	cmd.AddCommand(makeEvidenceSkillCmd())
	return cmd
}

func makeEvidenceSkillCmd() *cobra.Command {
	var (
		jsonOutput bool
		changeSlug string
		skillName  string
		verdictRaw string
		references []string
		blockers   []string
		notes      string
		notesFile  string
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
			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
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

				skillName = strings.TrimSpace(skillName)
				def, err := validateEvidenceSkillName(root, skillName)
				if err != nil {
					return err
				}
				if err := validateEvidenceSkillStage(change, def); err != nil {
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

				notesText, err := resolveEvidenceSkillNotes(root, notes, notesFile)
				if err != nil {
					return err
				}
				runVersion, err := evidenceSkillRunVersion(root, change, def)
				if err != nil {
					return err
				}
				references = stringutil.UniqueSorted(trimNonEmptyStrings(references))
				record := model.VerificationRecord{
					Verdict:    verdict,
					Blockers:   blockerCodes,
					Timestamp:  time.Now().UTC(),
					RunVersion: runVersion,
					References: references,
					Notes:      notesText,
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
					summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
					if err != nil {
						return newStateIntegrityError(
							"evidence_skill_execution_summary_load_failed",
							fmt.Sprintf("failed to load execution summary for %q: %v", change.Slug, err),
							"Repair execution-summary.yaml or record non-passing skill evidence.",
							change.Slug,
							nil,
						)
					}
					if err := progression.StampEvidenceDigestForSkill(root, change, skillName, record, summary); err != nil {
						return newStateIntegrityError(
							"evidence_skill_digest_stamp_failed",
							fmt.Sprintf("failed to stamp %s evidence digest: %v", skillName, err),
							"Resolve missing or stale digest inputs before recording passing skill evidence.",
							change.Slug,
							map[string]any{"skill": skillName},
						)
					}
					stamped = true
				}

				if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
					Command:     "evidence skill",
					EventType:   "skill.evidence_recorded",
					Action:      "recorded",
					Result:      verdict,
					BeforeState: change.CurrentState,
					AfterState:  change.CurrentState,
					SkillID:     skillName,
					Diagnostics: []string{
						fmt.Sprintf("skill=%s", skillName),
						fmt.Sprintf("run_version=%d", runVersion),
						"path=" + state.DisplayPath(root, path),
					},
				}); err != nil {
					return err
				}

				view := evidenceSkillView{
					Slug:       change.Slug,
					Skill:      skillName,
					RunVersion: runVersion,
					Path:       state.DisplayPath(root, path),
					Recorded:   true,
					Stamped:    stamped,
					References: references,
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
			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, "evidence task", func() error {
				change, err := loadActiveChange(
					root,
					ref.Slug,
					"cannot record task evidence for governed status %q",
					"Task evidence can only be recorded for an active governed change.",
				)
				if err != nil {
					return err
				}
				if change.CurrentState != model.StateS2Execute {
					remediation := "Record task evidence only during wave execution."
					if change.CurrentState == model.StateS4Verify {
						remediation = progression.S4VerificationRecoveryRemediation()
					}
					return newInvalidUsageError(
						"evidence_task_wrong_state",
						fmt.Sprintf("task evidence requires S2_EXECUTE state, current: %s", change.CurrentState),
						remediation,
						nil,
					)
				}

				taskID = strings.TrimSpace(taskID)
				if err := validateEvidenceTaskID(taskID); err != nil {
					return newInvalidUsageError(
						"evidence_task_id_invalid",
						err.Error(),
						"Use a task ID from the current governed wave plan without path separators.",
						map[string]any{"task_id": taskID},
					)
				}
				if runSummary < 1 {
					return newInvalidUsageError(
						"evidence_task_run_summary_version_invalid",
						"--run-summary-version must be >= 1",
						"Pass the current wave-orchestration run_version as --run-summary-version.",
						nil,
					)
				}
				if err := validateEvidenceTaskRunSummaryVersion(root, change, runSummary); err != nil {
					return err
				}

				wavePlan, err := state.LoadWavePlanForChange(root, change)
				if err != nil {
					return newStateIntegrityError(
						"evidence_task_wave_plan_missing",
						fmt.Sprintf("task evidence requires wave-plan.yaml for %q: %v", change.Slug, err),
						"Run `slipway run` through plan-audit so Slipway materializes wave-plan.yaml before recording task evidence.",
						change.Slug,
						map[string]any{"path": state.WavePlanPathForRead(root, change.Slug)},
					)
				}
				planTask, ok := findEvidenceWavePlanTask(wavePlan, taskID)
				if !ok {
					return newInvalidUsageError(
						"evidence_task_unknown",
						fmt.Sprintf("task %q is not present in the current wave plan", taskID),
						"Use a task ID from tasks.md / wave-plan.yaml and retry.",
						map[string]any{"task_id": taskID},
					)
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
						fmt.Sprintf("task %q has task_kind=%q in wave-plan.yaml, got %q", taskID, planTask.TaskKind, taskKind),
						"Use the task_kind recorded in wave-plan.yaml.",
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
							fmt.Sprintf("wave-plan.yaml target_files for task %q are invalid: %v", taskID, err),
							"Regenerate wave-plan.yaml from a valid tasks.md plan before recording task evidence.",
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
	cmd.Flags().StringVar(&taskID, "task-id", "", "Task ID from wave-plan.yaml (required)")
	cmd.Flags().IntVar(&runSummary, "run-summary-version", 0, "Current wave-orchestration run_version (required)")
	cmd.Flags().StringVar(&taskKindRaw, "task-kind", "", "Task kind: code, test, doc, ops, verification, investigation, other (required)")
	cmd.Flags().StringVar(&verdictRaw, "verdict", "", "Task verdict: pass, fail, blocked, incomplete, timeout (required)")
	cmd.Flags().StringVar(&evidenceRef, "evidence-ref", "", "Stable transcript, command, artifact, or note reference (required)")
	cmd.Flags().StringArrayVar(&changedFiles, "changed-file", nil, "Changed file path for this task; may be repeated")
	cmd.Flags().StringArrayVar(&targetFiles, "target-file", nil, "Target file path for this task; may be repeated")
	cmd.Flags().StringArrayVar(&blockerSpecs, "blocker", nil, "Task blocker as code or code:detail; may be repeated")
	cmd.Flags().StringVar(&capturedAtRaw, "captured-at", "", "Evidence timestamp in RFC3339Nano; defaults to now")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Optional executor session identifier")

	return cmd
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

func evidenceSkillRunVersion(root string, change model.Change, def skill.Definition) (int, error) {
	if !def.RunSummaryBound {
		return 0, nil
	}
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return 0, newStateIntegrityError(
			"evidence_skill_execution_summary_load_failed",
			fmt.Sprintf("failed to load execution summary for %q: %v", change.Slug, err),
			"Repair execution-summary.yaml before recording run-summary-bound skill evidence.",
			change.Slug,
			nil,
		)
	}
	if summary == nil || summary.RunSummaryVersion < 1 {
		return 0, newInvalidUsageError(
			"evidence_skill_run_summary_missing",
			fmt.Sprintf("%s evidence requires execution-summary.yaml with run_summary_version >= 1", def.Name),
			"Run wave execution first so Slipway can bind this verification to the current execution summary.",
			map[string]any{"skill": def.Name},
		)
	}
	return summary.RunSummaryVersion, nil
}

func validateEvidenceSkillStage(change model.Change, def skill.Definition) error {
	if change.CurrentState != def.State {
		return newInvalidUsageError(
			"evidence_skill_wrong_state",
			fmt.Sprintf("%s evidence requires %s state, current: %s", def.Name, def.State, change.CurrentState),
			fmt.Sprintf("Run the lifecycle to %s before recording %s evidence.", def.State, def.Name),
			map[string]any{
				"skill":    def.Name,
				"expected": def.State,
				"current":  change.CurrentState,
			},
		)
	}
	if def.State == model.StateS1Plan && def.PlanSubStep != model.PlanSubStepNone && change.PlanSubStep != def.PlanSubStep {
		return newInvalidUsageError(
			"evidence_skill_wrong_plan_substep",
			fmt.Sprintf("%s evidence requires S1_PLAN/%s, current substep: %s", def.Name, def.PlanSubStep, change.PlanSubStep),
			fmt.Sprintf("Run the lifecycle to S1_PLAN/%s before recording %s evidence.", def.PlanSubStep, def.Name),
			map[string]any{
				"skill":    def.Name,
				"expected": def.PlanSubStep,
				"current":  change.PlanSubStep,
			},
		)
	}
	return nil
}

func resolveEvidenceSkillNotes(root, notes, notesFile string) (string, error) {
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
	path := filepath.Join(root, filepath.FromSlash(model.NormalizePublicPath(notesFile)))
	raw, err := os.ReadFile(path) // #nosec G304 -- path is validated as workspace-relative before reading.
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_skill_notes_file_read_failed",
			fmt.Sprintf("failed to read notes file %q: %v", notesFile, err),
			"Write the delegated review or verification notes to the referenced workspace-relative path and retry.",
			"",
			map[string]any{"path": notesFile},
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
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}
