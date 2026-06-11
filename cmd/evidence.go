package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
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
	Path              string                             `json:"path"`
	Recorded          bool                               `json:"recorded"`
	FreshnessInputs   model.ExecutionTaskFreshnessInputs `json:"freshness_inputs"`
}

type evidenceSkillView struct {
	Slug       string `json:"slug"`
	SkillName  string `json:"skill_name"`
	Verdict    string `json:"verdict"`
	RunVersion int    `json:"run_version"`
	Path       string `json:"path"`
	Recorded   bool   `json:"recorded"`
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
		jsonOutput   bool
		changeSlug   string
		skillName    string
		verdictRaw   string
		references   []string
		blockerSpecs []string
		notes        string
		notesFile    string
	)

	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Record governance skill verification evidence for the active change",
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
				if skillName == "" {
					return newInvalidUsageError(
						"evidence_skill_required",
						"--skill is required",
						"Pass the governance skill name returned by `slipway next --json`.",
						nil,
					)
				}
				def, err := evidenceSkillDefinition(root, skillName)
				if err != nil {
					return err
				}
				if err := validateEvidenceSkillState(change, def); err != nil {
					return err
				}

				verdict := strings.TrimSpace(verdictRaw)
				switch verdict {
				case model.VerificationVerdictPass, model.VerificationVerdictFail:
				case "":
					return newInvalidUsageError(
						"evidence_skill_verdict_required",
						"--verdict is required",
						"Pass --verdict pass or --verdict fail.",
						nil,
					)
				default:
					return newInvalidUsageError(
						"evidence_skill_verdict_invalid",
						fmt.Sprintf("invalid skill verdict: %q", verdict),
						"Pass --verdict pass or --verdict fail.",
						map[string]any{"verdict": verdict},
					)
				}

				blockers := model.ReasonCodesFromSpecs(blockerSpecs)
				for i, blocker := range blockers {
					if err := blocker.Validate(); err != nil {
						return newInvalidUsageError(
							"evidence_skill_blocker_invalid",
							fmt.Sprintf("blocker %d is invalid: %v", i, err),
							"Pass blockers as code or code:detail values.",
							nil,
						)
					}
				}

				recordNotes, err := evidenceSkillNotes(notes, notesFile)
				if err != nil {
					return err
				}
				runVersion, err := evidenceSkillRunVersion(root, change, def)
				if err != nil {
					return err
				}
				if err := validateEvidenceSkillActionable(root, change, def, runVersion); err != nil {
					return err
				}
				var summary *model.ExecutionSummary
				if verdict == model.VerificationVerdictPass {
					summary, err = state.LoadOptionalRelevantExecutionSummary(root, change)
					if err != nil {
						return newStateIntegrityError(
							"evidence_skill_digest_input_failed",
							fmt.Sprintf("failed to load digest inputs for %s evidence: %v", skillName, err),
							"Repair execution evidence and retry the skill evidence command.",
							change.Slug,
							map[string]any{"skill": skillName},
						)
					}
					if err := progression.CheckEvidenceDigestInputsForSkill(root, change, skillName, summary); err != nil {
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
					Blockers:   blockers,
					Timestamp:  time.Now().UTC(),
					RunVersion: runVersion,
					References: stringutil.UniqueSorted(trimNonEmptyStrings(references)),
					Notes:      recordNotes,
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
						"Repair the governed bundle path or verification record and retry.",
						change.Slug,
						map[string]any{"skill": skillName},
					)
				}
				if record.IsPassing() {
					if err := progression.StampEvidenceDigestForSkill(root, change, skillName, record, summary); err != nil {
						restoreErr := restoreVerificationRaw(path, previousRaw, hadPrevious)
						return newStateIntegrityError(
							"evidence_skill_digest_stamp_failed",
							fmt.Sprintf("failed to stamp %s evidence digest: %v%s", skillName, err, restoreVerificationSuffix(restoreErr)),
							"Repair the governed inputs for this skill and retry the evidence command.",
							change.Slug,
							map[string]any{"skill": skillName},
						)
					}
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

				if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
					Command:     "evidence skill",
					EventType:   "skill.evidence_recorded",
					SkillID:     skillName,
					Action:      "recorded",
					Result:      "recorded",
					BeforeState: change.CurrentState,
					AfterState:  change.CurrentState,
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
					Slug:       change.Slug,
					SkillName:  skillName,
					Verdict:    verdict,
					RunVersion: runVersion,
					Path:       displayPath,
					Recorded:   true,
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
	cmd.Flags().StringVar(&skillName, "skill", "", "Governance skill name returned by slipway next (required)")
	cmd.Flags().StringVar(&verdictRaw, "verdict", "", "Skill verdict: pass or fail (required)")
	cmd.Flags().StringArrayVar(&references, "reference", nil, "Stable evidence reference; may be repeated")
	cmd.Flags().StringArrayVar(&blockerSpecs, "blocker", nil, "Skill blocker as code or code:detail; may be repeated")
	cmd.Flags().StringVar(&notes, "notes", "", "Inline verification notes")
	cmd.Flags().StringVar(&notesFile, "notes-file", "", "Path to verification notes file")

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

func evidenceSkillDefinition(root, skillName string) (skill.Definition, error) {
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return skill.Definition{}, err
	}
	def, ok := skill.LookupDefinitionInRegistry(registry, skillName)
	if !ok {
		return skill.Definition{}, newInvalidUsageError(
			"evidence_skill_unknown",
			fmt.Sprintf("unknown governance skill: %s", skillName),
			"Use a governance skill name returned by `slipway next --json`.",
			map[string]any{"skill": skillName},
		)
	}
	return def, nil
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

func validateEvidenceSkillActionable(root string, change model.Change, def skill.Definition, runVersion int) error {
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
		passing, err := currentPassingEvidenceSkills(root, change, model.StateS3Review, runVersion)
		if err != nil {
			return "", err
		}
		if _, ok := passing[progression.SkillSpecComplianceReview]; !ok {
			return progression.SkillSpecComplianceReview, nil
		}
		if change.EffectiveWorkflowProfile().RequiresCodeQualityReview() {
			if _, ok := passing[progression.SkillCodeQualityReview]; !ok {
				return progression.SkillCodeQualityReview, nil
			}
		}
		return "", nil
	case model.StateS4Verify:
		passing, err := currentPassingEvidenceSkills(root, change, model.StateS4Verify, runVersion)
		if err != nil {
			return "", err
		}
		if _, ok := passing[progression.SkillGoalVerification]; !ok {
			return progression.SkillGoalVerification, nil
		}
		policy, err := governance.ResolvePresetPolicy(root, change)
		if err != nil {
			return "", err
		}
		if progression.FinalCloseoutEvidenceRequired(policy) {
			if _, ok := passing[progression.SkillFinalCloseout]; !ok {
				return progression.SkillFinalCloseout, nil
			}
		}
		return "", nil
	default:
		nextSkill, _ := progression.ResolveNextSkill(change)
		if nextSkill == progression.SkillWorktreePreflight {
			return "", nil
		}
		return nextSkill, nil
	}
}

func currentPassingEvidenceSkills(
	root string,
	change model.Change,
	workflowState model.WorkflowState,
	runVersion int,
) (map[string]model.VerificationRecord, error) {
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return nil, err
	}
	passing, _, err := progression.EvaluateRequiredSkillsForChange(
		root,
		change,
		workflowState,
		runVersion,
		progression.FinalCloseoutEvidenceRequired(policy),
	)
	return passing, err
}

func validateEvidenceSkillState(change model.Change, def skill.Definition) error {
	if def.DiscoveryOnly && !change.NeedsDiscovery {
		return newInvalidUsageError(
			"evidence_skill_not_applicable",
			fmt.Sprintf("skill %s applies only to discovery changes", def.Name),
			"Record evidence for the skill currently returned by `slipway next --json`.",
			map[string]any{"skill": def.Name},
		)
	}
	if change.CurrentState != def.State {
		return newInvalidUsageError(
			"evidence_skill_wrong_state",
			fmt.Sprintf("skill %s requires %s state, current: %s", def.Name, def.State, change.CurrentState),
			"Run `slipway next --json` and record evidence only for the current actionable skill.",
			map[string]any{
				"skill":          def.Name,
				"required_state": string(def.State),
				"current_state":  string(change.CurrentState),
			},
		)
	}
	if def.State == model.StateS1Plan &&
		def.PlanSubStep != model.PlanSubStepNone &&
		change.PlanSubStep != def.PlanSubStep {
		return newInvalidUsageError(
			"evidence_skill_wrong_substep",
			fmt.Sprintf("skill %s requires %s/%s, current: %s/%s", def.Name, def.State, def.PlanSubStep, change.CurrentState, change.PlanSubStep),
			"Run `slipway next --json` and record evidence only for the current actionable skill.",
			map[string]any{
				"skill":            def.Name,
				"required_state":   string(def.State),
				"required_substep": string(def.PlanSubStep),
				"current_state":    string(change.CurrentState),
				"current_substep":  string(change.PlanSubStep),
			},
		)
	}
	return nil
}

func evidenceSkillRunVersion(root string, change model.Change, def skill.Definition) (int, error) {
	if !def.RunSummaryBound {
		return 0, nil
	}
	if def.Name == progression.SkillWaveOrchestration {
		runVersion, err := progression.LatestTaskEvidenceRunVersion(root, change.Slug)
		if err != nil {
			return 0, err
		}
		if runVersion < 1 {
			return 0, newPreconditionError(
				"evidence_skill_run_summary_missing",
				fmt.Sprintf("skill %s requires runtime task evidence", def.Name),
				"Record task evidence with `slipway evidence task`, then record the wave-orchestration skill evidence.",
				change.Slug,
				map[string]any{"skill": def.Name},
			)
		}
		return runVersion, nil
	}
	execCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return 0, err
	}
	if execCtx.LatestRunVersion < 1 {
		return 0, newPreconditionError(
			"evidence_skill_run_summary_missing",
			fmt.Sprintf("skill %s requires a ready execution summary", def.Name),
			"Run `slipway run` until execution-summary.yaml is ready, then record the skill evidence.",
			change.Slug,
			map[string]any{"skill": def.Name},
		)
	}
	return execCtx.LatestRunVersion, nil
}

func evidenceSkillNotes(notes, notesFile string) (string, error) {
	notes = strings.TrimSpace(notes)
	notesFile = strings.TrimSpace(notesFile)
	if notes != "" && notesFile != "" {
		return "", newInvalidUsageError(
			"evidence_skill_notes_conflict",
			"pass either --notes or --notes-file, not both",
			"Use --notes for short inline text or --notes-file for a verification notes artifact.",
			nil,
		)
	}
	if notesFile == "" {
		return notes, nil
	}
	raw, err := os.ReadFile(notesFile) // #nosec G304 -- notes-file is an explicit local operator input.
	if err != nil {
		return "", newInvalidUsageError(
			"evidence_skill_notes_unreadable",
			fmt.Sprintf("failed to read --notes-file %q: %v", notesFile, err),
			"Pass a readable notes file path or use --notes.",
			map[string]any{"notes_file": notesFile},
		)
	}
	return strings.TrimSpace(string(raw)), nil
}

func trimNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
