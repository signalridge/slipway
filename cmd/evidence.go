package cmd

import (
	"crypto/sha256"
	"encoding/hex"
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
	"gopkg.in/yaml.v3"
)

type evidenceTaskView struct {
	Slug              string                             `json:"slug"`
	TaskID            string                             `json:"task_id"`
	RunSummaryVersion int                                `json:"run_summary_version"`
	Path              string                             `json:"path"`
	Recorded          bool                               `json:"recorded"`
	FreshnessInputs   model.ExecutionTaskFreshnessInputs `json:"freshness_inputs"`
}

type evidenceTaskBatchView struct {
	Slug              string             `json:"slug"`
	RunSummaryVersion int                `json:"run_summary_version"`
	Recorded          bool               `json:"recorded"`
	RecordedCount     int                `json:"recorded_count"`
	Tasks             []evidenceTaskView `json:"tasks"`
}

type evidenceSkillView struct {
	Slug       string   `json:"slug"`
	Skill      string   `json:"skill"`
	SkillName  string   `json:"skill_name"`
	Verdict    string   `json:"verdict"`
	RunVersion int      `json:"run_version"`
	Path       string   `json:"path"`
	Recorded   bool     `json:"recorded"`
	Stamped    bool     `json:"stamped"`
	References []string `json:"references,omitempty"`
}

const (
	taskEvidenceResultFileRemediation = "Record task evidence with `slipway evidence task --result-file <path> --json` after the executor writes compact JSON with task_id, verdict, evidence_ref, changed_files, blockers, and optional session_id; repeat --result-file to import multiple task results atomically."
	maxEvidenceTaskResultFileBytes    = int64(1 << 20)
	maxEvidenceTaskResultFiles        = 256
)

type evidenceSuiteResultView struct {
	Slug              string            `json:"slug"`
	RunSummaryVersion int               `json:"run_summary_version"`
	FullSuiteDigest   string            `json:"full_suite_digest"`
	SASTDigests       map[string]string `json:"sast_digests,omitempty"`
	Path              string            `json:"path"`
	Recorded          bool              `json:"recorded"`
}

func makeEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: desc("evidence"),
	}
	cmd.AddCommand(makeEvidenceTaskCmd())
	cmd.AddCommand(makeEvidenceSkillCmd())
	cmd.AddCommand(makeEvidenceSuiteResultCmd())
	return cmd
}

func makeEvidenceSuiteResultCmd() *cobra.Command {
	var (
		jsonOutput      bool
		changeSlug      string
		fullSuiteDigest string
		fullSuiteProof  string
		sastDigests     []string
		sastProofs      []string
	)

	cmd := &cobra.Command{
		Use:   "suite-result",
		Short: "Record CLI-owned suite-result verification evidence",
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

			return withChangeStateLock(root, ref.Slug, "evidence suite-result", func() error {
				change, err := loadActiveChange(
					root,
					ref.Slug,
					"cannot record suite-result evidence for governed status %q",
					"Suite-result evidence can only be recorded for an active governed change.",
				)
				if err != nil {
					return err
				}
				if change.CurrentState != model.StateS3Review {
					return newPreconditionError(
						"evidence_suite_result_wrong_state",
						fmt.Sprintf("cannot record suite-result evidence while change is in %s", change.CurrentState),
						"Run S2 execution to completion, advance to S3 review, then record suite-result evidence.",
						change.Slug,
						map[string]any{
							"current_state":  change.CurrentState,
							"required_state": model.StateS3Review,
						},
					)
				}

				fullSuiteDigest, err = resolveSuiteResultFullSuiteDigest(root, change, fullSuiteDigest, fullSuiteProof)
				if err != nil {
					return err
				}

				summary, err := state.LoadExecutionSummaryForChange(root, change)
				if err != nil {
					return newStateIntegrityError(
						"evidence_suite_result_run_summary_missing",
						fmt.Sprintf("failed to load execution summary for suite-result evidence: %v", err),
						"Materialize execution-summary.yaml from S2 execution evidence before recording suite-result evidence.",
						change.Slug,
						nil,
					)
				}

				sastDigestMap, err := resolveSuiteResultSASTDigests(root, change, sastDigests, sastProofs)
				if err != nil {
					return err
				}
				record := model.SuiteResult{
					Version:           model.SuiteResultVersion,
					RunSummaryVersion: summary.RunSummaryVersion,
					FullSuiteDigest:   fullSuiteDigest,
					SASTDigests:       sastDigestMap,
					CapturedAt:        time.Now().UTC(),
				}
				if err := record.Validate(); err != nil {
					return newInvalidUsageError(
						"evidence_suite_result_invalid",
						fmt.Sprintf("invalid suite-result evidence: %v", err),
						"Pass a non-empty full-suite digest and non-empty name=digest SAST digests.",
						nil,
					)
				}

				governedBundleDir, err := state.GovernedBundleDir(root, change)
				if err != nil {
					return newStateIntegrityError(
						"evidence_suite_result_bundle_resolve_failed",
						fmt.Sprintf("failed to resolve governed bundle for suite-result evidence: %v", err),
						"Repair the governed change worktree binding and retry.",
						change.Slug,
						nil,
					)
				}
				verificationDir := filepath.Join(governedBundleDir, "verification")
				if err := os.MkdirAll(verificationDir, 0o755); err != nil { // #nosec G301 -- governed verification artifact directories are intentionally user-readable/searchable.
					return err
				}
				path := filepath.Join(verificationDir, "suite-result.yaml")
				raw, err := yaml.Marshal(record)
				if err != nil {
					return err
				}
				if err := os.WriteFile(path, raw, 0o644); err != nil { // #nosec G306 -- governed verification artifacts are intentionally user-readable.
					return newStateIntegrityError(
						"evidence_suite_result_write_failed",
						fmt.Sprintf("failed to write suite-result evidence: %v", err),
						"Fix the verification record inputs and rerun `slipway evidence suite-result`.",
						change.Slug,
						nil,
					)
				}

				displayPath := state.DisplayPath(root, path)
				if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
					Command:     "evidence suite-result",
					EventType:   "suite_result.evidence_recorded",
					Action:      "recorded",
					Result:      "recorded",
					BeforeState: change.CurrentState,
					AfterState:  change.CurrentState,
					EvidenceRefs: map[string]string{
						"suite-result": displayPath,
					},
					Diagnostics: []string{
						fmt.Sprintf("run_summary_version=%d", summary.RunSummaryVersion),
						"path=" + displayPath,
					},
				}); err != nil {
					return err
				}

				view := evidenceSuiteResultView{
					Slug:              change.Slug,
					RunSummaryVersion: summary.RunSummaryVersion,
					FullSuiteDigest:   fullSuiteDigest,
					SASTDigests:       sastDigestMap,
					Path:              displayPath,
					Recorded:          true,
				}
				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}

				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef("Suite-result evidence recorded: %s\n", view.Path)
				return writer.Err()
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().StringVar(&fullSuiteDigest, "full-suite-digest", "", "Digest of the fresh full-suite proof artifact or transcript")
	cmd.Flags().StringVar(&fullSuiteProof, "full-suite-proof", "", "Workspace-relative full-suite proof artifact or transcript to hash")
	cmd.Flags().StringArrayVar(&sastDigests, "sast-digest", nil, "Guardrail SAST digest as name=digest; may be repeated")
	cmd.Flags().StringArrayVar(&sastProofs, "sast-proof", nil, "Guardrail SAST proof as name=workspace-relative-path; may be repeated")

	return cmd
}

func resolveSuiteResultFullSuiteDigest(root string, change model.Change, digest, proofPath string) (string, error) {
	digest = strings.TrimSpace(digest)
	proofPath = strings.TrimSpace(proofPath)
	if digest != "" && proofPath != "" {
		return "", newInvalidUsageError(
			"evidence_suite_result_full_suite_conflict",
			"pass either --full-suite-digest or --full-suite-proof, not both",
			"Use --full-suite-proof when Slipway should hash a workspace-relative proof file, or --full-suite-digest when an external system already computed the digest.",
			nil,
		)
	}
	if digest != "" {
		return digest, nil
	}
	if proofPath == "" {
		return "", newInvalidUsageError(
			"evidence_suite_result_full_suite_required",
			"--full-suite-digest or --full-suite-proof is required",
			"Pass the digest of the fresh full-suite proof, or pass a workspace-relative proof file for Slipway to hash.",
			nil,
		)
	}
	return digestSuiteResultProofFile(root, change, proofPath, "full-suite")
}

func resolveSuiteResultSASTDigests(root string, change model.Change, digestValues, proofValues []string) (map[string]string, error) {
	digests := make(map[string]string, len(digestValues)+len(proofValues))
	for i, value := range digestValues {
		name, digest, err := parseSuiteResultNameValue(value, "--sast-digest", i)
		if err != nil {
			return nil, err
		}
		if _, exists := digests[name]; exists {
			return nil, duplicateSuiteResultSASTInputError(name)
		}
		digests[name] = digest
	}
	for i, value := range proofValues {
		name, proofPath, err := parseSuiteResultNameValue(value, "--sast-proof", i)
		if err != nil {
			return nil, err
		}
		if _, exists := digests[name]; exists {
			return nil, duplicateSuiteResultSASTInputError(name)
		}
		digest, err := digestSuiteResultProofFile(root, change, proofPath, "SAST proof "+name)
		if err != nil {
			return nil, err
		}
		digests[name] = digest
	}
	if len(digests) == 0 {
		return nil, nil
	}
	return digests, nil
}

func parseSuiteResultNameValue(value, flag string, index int) (string, string, error) {
	name, digest, ok := strings.Cut(value, "=")
	name = strings.TrimSpace(name)
	digest = strings.TrimSpace(digest)
	if !ok || name == "" || digest == "" {
		return "", "", newInvalidUsageError(
			"evidence_suite_result_named_value_invalid",
			fmt.Sprintf("invalid %s %d: %q", flag, index, value),
			"Pass values as name=value, for example credentials.safety_baseline=sha256:... or credentials.safety_baseline=verification/logs/sast.txt.",
			nil,
		)
	}
	return name, digest, nil
}

func duplicateSuiteResultSASTInputError(name string) error {
	return newInvalidUsageError(
		"evidence_suite_result_sast_duplicate",
		fmt.Sprintf("duplicate SAST digest name %q", name),
		"Pass each SAST digest name only once, using either --sast-digest or --sast-proof.",
		nil,
	)
}

func digestSuiteResultProofFile(root string, change model.Change, proofPath, label string) (string, error) {
	if err := validateEvidencePath(proofPath); err != nil {
		return "", newInvalidUsageError(
			"evidence_suite_result_proof_path_invalid",
			err.Error(),
			"Pass a workspace-relative proof file path without absolute paths, empty segments, or parent traversal.",
			nil,
		)
	}
	workspaceRoot, err := state.WorkspaceRootForChange(root, change)
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_suite_result_workspace_resolve_failed",
			fmt.Sprintf("failed to resolve proof workspace for %q: %v", change.Slug, err),
			"Repair the governed change worktree binding and retry.",
			change.Slug,
			map[string]any{"path": proofPath},
		)
	}
	path := filepath.Join(workspaceRoot, filepath.FromSlash(model.NormalizePublicPath(proofPath)))
	raw, err := os.ReadFile(path) // #nosec G304 -- path is validated as workspace-relative before reading.
	if err != nil {
		return "", newStateIntegrityError(
			"evidence_suite_result_proof_read_failed",
			fmt.Sprintf("failed to read %s proof file %q: %v", label, proofPath, err),
			"Write the proof transcript to the referenced workspace-relative path and retry.",
			change.Slug,
			map[string]any{
				"path":           proofPath,
				"resolved_path":  state.DisplayPath(root, path),
				"workspace_root": state.DisplayPath(root, workspaceRoot),
			},
		)
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
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
				if err := validateEvidenceSkillActionable(root, change, def, runVersion); err != nil {
					return err
				}
				references = stringutil.UniqueSorted(trimNonEmptyStrings(references))

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
				if record.IsPassing() &&
					skillName == progression.SkillWaveOrchestration &&
					change.CurrentState == model.StateS2Implement {
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
					Slug:       change.Slug,
					Skill:      skillName,
					SkillName:  skillName,
					Verdict:    verdict,
					RunVersion: runVersion,
					Path:       displayPath,
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
				if change.CurrentState != model.StateS2Implement {
					remediation := evidenceTaskWrongStateRemediation(root, change)
					return newInvalidUsageError(
						"evidence_task_wrong_state",
						fmt.Sprintf("task evidence requires S2_IMPLEMENT state, current: %s", change.CurrentState),
						remediation,
						nil,
					)
				}

				resultFiles = normalizeEvidenceTaskResultFileArgs(resultFiles)
				if len(resultFiles) > 0 {
					if err := rejectEvidenceTaskResultFileLedgerFlags(cmd); err != nil {
						return err
					}
					return recordEvidenceTaskResultFiles(cmd, root, change, resultFiles, jsonOutput)
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
					return newInvalidUsageError(
						"evidence_task_unknown",
						fmt.Sprintf("task %q is not present in the current wave plan", taskID),
						"Use a task ID from the current tasks.md-derived wave projection and retry.",
						map[string]any{"task_id": taskID},
					)
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
	payload progression.TaskEvidencePayload
	path    string
	view    evidenceTaskView
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

func recordEvidenceTaskResultFiles(
	cmd *cobra.Command,
	root string,
	change model.Change,
	resultFiles []string,
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
		return newStateIntegrityError(
			"evidence_task_wave_plan_unavailable",
			fmt.Sprintf("task evidence requires a current S2 wave projection for %q: %v", change.Slug, err),
			"Fix tasks.md so Slipway can derive the current S2 wave projection, then rerun `slipway implement` before recording task evidence.",
			change.Slug,
			map[string]any{"path": state.WavePlanPathForRead(root, change.Slug)},
		)
	}
	runSummary, err := deriveEvidenceTaskRunSummaryVersion(root, change, wavePlan)
	if err != nil {
		return err
	}
	if err := validateEvidenceTaskRunSummaryVersion(root, change, runSummary); err != nil {
		return err
	}

	prepared, err := prepareEvidenceTaskResultFiles(root, change, wavePlan, runSummary, resultFiles, time.Now().UTC())
	if err != nil {
		return err
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

		planTask, ok := findEvidenceWavePlanTask(wavePlan, taskID)
		if !ok {
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
			SessionID:         strings.TrimSpace(result.SessionID),
		}
		path := filepath.Join(state.EvidenceTasksDir(root, change.Slug), taskID+".json")
		prepared = append(prepared, preparedEvidenceTaskResult{
			payload: payload,
			path:    path,
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
	return prepared, nil
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
	if def.Name == progression.SkillWaveOrchestration && change.CurrentState == model.StateS2Implement {
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
	if def.Name == progression.SkillWaveOrchestration && change.CurrentState == model.StateS2Implement {
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

func evidenceTaskWrongStateRemediation(root string, change model.Change) string {
	switch change.CurrentState {
	case model.StateS3Review:
		return postReviewReplacementEvidenceRemediation(root, change, "task evidence")
	default:
		return "Record task evidence only during wave execution."
	}
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
		"%s is S2-only after wave execution. For review-driven repairs or tests, record fresh proof for %s evidence, then rerun %s and %s.",
		surface,
		strings.Join(reviewSkills, ", "),
		progression.SkillGoalVerification,
		progression.SkillFinalCloseout,
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

func deriveEvidenceTaskRunSummaryVersion(root string, change model.Change, wavePlan model.WavePlan) (int, error) {
	if wavePlan.RunSummaryVersion < 1 {
		return 0, newStateIntegrityError(
			"evidence_task_run_summary_version_unavailable",
			fmt.Sprintf("current wave plan for %q has invalid run_summary_version %d", change.Slug, wavePlan.RunSummaryVersion),
			"Rematerialize the S2 wave plan before importing task result evidence.",
			change.Slug,
			map[string]any{"run_summary_version": wavePlan.RunSummaryVersion},
		)
	}
	versions, err := existingEvidenceTaskRunSummaryVersions(root, change.Slug)
	if err != nil {
		return 0, err
	}
	if len(versions) > 1 {
		return 0, newInvalidUsageError(
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
			return 0, newInvalidUsageError(
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
	return wavePlan.RunSummaryVersion, nil
}

func existingEvidenceTaskRunSummaryVersions(root, slug string) (map[int]struct{}, error) {
	versions, scanErr := scanEvidenceTaskRunSummaryVersions(root, slug)
	if scanErr != nil {
		switch scanErr.Kind {
		case evidenceTaskRunSummaryVersionsMissingDir:
			return nil, nil
		case evidenceTaskRunSummaryVersionsReadDir, evidenceTaskRunSummaryVersionsReadFile:
			return nil, newStateIntegrityError(
				"evidence_task_existing_evidence_load_failed",
				fmt.Sprintf("failed to read existing task evidence %q: %v", state.DisplayPath(root, scanErr.Path), scanErr.Err),
				"Repair the runtime task evidence before importing task result evidence.",
				slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		case evidenceTaskRunSummaryVersionsParseFile:
			return nil, newStateIntegrityError(
				"evidence_task_existing_evidence_invalid",
				fmt.Sprintf("failed to parse existing task evidence %q: %v", state.DisplayPath(root, scanErr.Path), scanErr.Err),
				"Repair or remove malformed task evidence before importing task result evidence.",
				slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		case evidenceTaskRunSummaryVersionsInvalidVersion:
			return nil, newStateIntegrityError(
				"evidence_task_existing_evidence_invalid",
				fmt.Sprintf("existing task evidence %q has invalid run_summary_version %d", state.DisplayPath(root, scanErr.Path), scanErr.RunSummaryVersion),
				"Repair or remove invalid task evidence before importing task result evidence.",
				slug,
				map[string]any{"path": state.DisplayPath(root, scanErr.Path)},
			)
		}
	}
	return versions, nil
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

	versions := map[int]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
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

func validateEvidenceSkillActionable(root string, change model.Change, def skill.Definition, runVersion int) error {
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
				return newInvalidUsageError(
					"evidence_skill_not_current",
					fmt.Sprintf("skill %s already has passing evidence for the current review set", def.Name),
					"Run `slipway next --json` and record evidence only for a selected review skill that is still missing or stale.",
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
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return nil, err
	}
	passing, _, err := progression.EvaluateRequiredSkillsForChangeWithReviewSelection(
		root,
		change,
		workflowState,
		runVersion,
		progression.FinalCloseoutEvidenceRequired(policy),
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
