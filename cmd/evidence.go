package cmd

import (
	"crypto/sha256"
	"encoding/hex"
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

				taskID = strings.TrimSpace(taskID)
				if err := validateEvidenceTaskID(taskID); err != nil {
					return newInvalidUsageError(
						"evidence_task_id_invalid",
						err.Error(),
						"Use a task ID from the current tasks.md-derived wave projection without path separators.",
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
	cmd.Flags().StringVar(&taskID, "task-id", "", "Task ID from the current tasks.md-derived wave projection (required)")
	cmd.Flags().IntVar(&runSummary, "run-summary-version", 0, "Run summary version to attribute this task evidence to (>= 1; the first task-evidence run version is 1 -- pass the current wave-orchestration run_version) (required)")
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

func evidenceSkillRunContext(root string, change model.Change, def skill.Definition) (int, *model.ExecutionSummary, error) {
	if !def.RunSummaryBound {
		return 0, nil, nil
	}
	execCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return 0, nil, err
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
	dir := state.EvidenceTasksDir(root, change.Slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, newInvalidUsageError(
				"evidence_skill_run_summary_missing",
				"wave-orchestration evidence requires task evidence before execution-summary.yaml exists",
				"Record task evidence with `slipway evidence task --run-summary-version <n>` before recording wave-orchestration evidence.",
				map[string]any{"skill": progression.SkillWaveOrchestration},
			)
		}
		return 0, newStateIntegrityError(
			"evidence_skill_task_evidence_load_failed",
			fmt.Sprintf("failed to read task evidence for %q: %v", change.Slug, err),
			"Repair the runtime task evidence directory before recording wave-orchestration evidence.",
			change.Slug,
			map[string]any{"path": state.DisplayPath(root, dir)},
		)
	}

	versions := map[int]bool{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway runtime evidence authority.
		if err != nil {
			return 0, newStateIntegrityError(
				"evidence_skill_task_evidence_load_failed",
				fmt.Sprintf("failed to read task evidence %q: %v", state.DisplayPath(root, path), err),
				"Repair the runtime task evidence file before recording wave-orchestration evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, path)},
			)
		}
		var payload struct {
			RunSummaryVersion int `json:"run_summary_version"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return 0, newStateIntegrityError(
				"evidence_skill_task_evidence_invalid",
				fmt.Sprintf("failed to parse task evidence %q: %v", state.DisplayPath(root, path), err),
				"Regenerate task evidence with `slipway evidence task` before recording wave-orchestration evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, path)},
			)
		}
		if payload.RunSummaryVersion < 1 {
			return 0, newStateIntegrityError(
				"evidence_skill_task_evidence_invalid",
				fmt.Sprintf("task evidence %q has invalid run_summary_version %d", state.DisplayPath(root, path), payload.RunSummaryVersion),
				"Regenerate task evidence with a run_summary_version >= 1 before recording wave-orchestration evidence.",
				change.Slug,
				map[string]any{"path": state.DisplayPath(root, path)},
			)
		}
		versions[payload.RunSummaryVersion] = true
	}
	if len(versions) == 0 {
		return 0, newInvalidUsageError(
			"evidence_skill_run_summary_missing",
			"wave-orchestration evidence requires task evidence before execution-summary.yaml exists",
			"Record task evidence with `slipway evidence task --run-summary-version <n>` before recording wave-orchestration evidence.",
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
	if tasks, issues, err := progression.LoadExecutionTasksFromEvidence(root, change.Slug, runVersion); err != nil {
		return 0, newStateIntegrityError(
			"evidence_skill_task_evidence_load_failed",
			fmt.Sprintf("failed to load task evidence for run_summary_version=%d: %v", runVersion, err),
			"Repair runtime task evidence before recording wave-orchestration evidence.",
			change.Slug,
			map[string]any{"run_summary_version": runVersion},
		)
	} else if len(issues) > 0 {
		return 0, newStateIntegrityError(
			"evidence_skill_task_evidence_invalid",
			fmt.Sprintf("task evidence for run_summary_version=%d is invalid: %s", runVersion, strings.Join(issues, "; ")),
			"Regenerate invalid task evidence before recording wave-orchestration evidence.",
			change.Slug,
			map[string]any{"run_summary_version": runVersion},
		)
	} else if len(tasks) == 0 {
		return 0, newInvalidUsageError(
			"evidence_skill_run_summary_missing",
			"wave-orchestration evidence requires task evidence before execution-summary.yaml exists",
			"Record task evidence with `slipway evidence task --run-summary-version <n>` before recording wave-orchestration evidence.",
			map[string]any{"skill": progression.SkillWaveOrchestration},
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
