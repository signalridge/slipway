package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"

	"github.com/spf13/cobra"
)

// changeRef is the lightweight active-change selector used across cmd/.
type changeRef struct {
	Slug string
}

const governedExecutionMode = string(ctxpack.ExecutionModeGoverned)

type projectRootContextKey struct{}

func setCommandProjectRoot(cmd *cobra.Command, root string) {
	if cmd == nil {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, projectRootContextKey{}, root))
}

func projectRootFromCommand(cmd *cobra.Command) (string, error) {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		if _, err := os.Stat(state.ConfigPath(root)); err == nil {
			return root, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		return "", fmt.Errorf("%w: workspace is not initialized; run `slipway init`", fsutil.ErrProjectRootNotFound)
	}
	return projectRootFromWD()
}

func workspaceRootFromCommandOrWD(cmd *cobra.Command) (string, error) {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		return root, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if normalized, err := state.NormalizePath(wd); err == nil {
		return normalized, nil
	}
	return filepath.Clean(wd), nil
}

func projectRootOverrideFromCommand(cmd *cobra.Command) (string, bool) {
	if cmd != nil {
		if root, ok := cmd.Context().Value(projectRootContextKey{}).(string); ok {
			root = strings.TrimSpace(root)
			if root != "" {
				normalized, err := state.NormalizePath(root)
				if err == nil {
					root = normalized
				} else {
					root = filepath.Clean(root)
				}
				return root, true
			}
		}
	}
	return "", false
}

func projectRootFromWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := fsutil.FindProjectRoot(wd)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(state.ConfigPath(root)); err == nil {
		return root, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return "", fmt.Errorf("%w: workspace is not initialized; run `slipway init`", fsutil.ErrProjectRootNotFound)
}

// invocationWorkspaceRoot resolves the git worktree where the current command
// is running. Adapter prompt/agent paths must follow the active invocation
// workspace, not always the canonical scope root.
func invocationWorkspaceRoot(projectRoot string) string {
	workspaceRoot := projectRoot
	wd, err := os.Getwd()
	if err != nil {
		return workspaceRoot
	}

	resolved, err := state.ResolveGitWorkspaceRoot(wd)
	if err != nil {
		return workspaceRoot
	}
	if resolved == "" {
		return workspaceRoot
	}
	if normalized, err := state.NormalizePath(resolved); err == nil {
		return normalized
	}
	return filepath.Clean(resolved)
}

func invocationWorkspaceRootFromCommand(cmd *cobra.Command, projectRoot string) string {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		return root
	}
	return invocationWorkspaceRoot(projectRoot)
}

func repairRootFromWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := fsutil.FindProjectRoot(wd)
	if err == nil && hasRepairableWorkspaceMarkers(root) {
		return root, nil
	}
	if err == nil {
		err = fmt.Errorf("%w: discovered project root %q has no slipway repair markers", fsutil.ErrProjectRootNotFound, root)
	}

	// Accept the cwd as a repair root when slipway-specific runtime or
	// artifact markers are already present.
	for dir := wd; ; dir = filepath.Dir(dir) {
		if hasRepairableWorkspaceMarkers(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", err
}

func repairRootFromCommand(cmd *cobra.Command) (string, error) {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		if hasRepairableWorkspaceMarkers(root) {
			return root, nil
		}
		return "", fmt.Errorf("%w: provided project root %q has no slipway repair markers", fsutil.ErrProjectRootNotFound, root)
	}
	return repairRootFromWD()
}

func hasRepairableWorkspaceMarkers(root string) bool {
	for _, marker := range []string{
		state.GitStateDir(root),
		filepath.Join(root, "artifacts", "changes"),
		filepath.Join(root, "artifacts", "codebase"),
	} {
		info, err := os.Stat(marker)
		if err == nil && info.IsDir() {
			return true
		}
	}
	for _, marker := range []string{
		state.ConfigPath(root),
		state.ScopeMarkerPath(root),
	} {
		info, err := os.Stat(marker)
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func loadConfigAtRoot(root string) (model.Config, error) {
	cfgPath := state.ConfigPath(root)
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		return model.Config{}, newStateIntegrityError(
			"config_parse_failure",
			fmt.Sprintf("failed to load .slipway.yaml: %v", err),
			"Run `slipway repair` to back up broken config and rewrite deterministic defaults.",
			"",
			map[string]any{"path": cfgPath},
		)
	}
	return cfg, nil
}

func wrapRequiredSkillsEvaluationError(operation, slug string, err error) error {
	if err == nil {
		return nil
	}

	errorCode := "required_skills_evaluation_failed"
	remediation := "Run `slipway repair` to restore generated skills, then fix malformed governance skill or evidence files."
	details := map[string]any{
		"operation": operation,
	}

	var registryErr *skill.GovernanceRegistryError
	if errors.As(err, &registryErr) {
		errorCode = "skill_registry_invalid"
		remediation = "Run `slipway repair` to restore generated skills or fix malformed governance skill frontmatter."
		if registryErr.Path != "" {
			details["path"] = registryErr.Path
		}
	}

	return newStateIntegrityError(
		errorCode,
		fmt.Sprintf("%s: %v", operation, err),
		remediation,
		slug,
		details,
	)
}

func governanceReadinessErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var registryErr *skill.GovernanceRegistryError
	if errors.As(err, &registryErr) {
		return "skill_registry_invalid"
	}
	var verificationErr *state.VerificationLoadError
	if errors.As(err, &verificationErr) {
		return "verification_load_failed"
	}
	var summaryErr *state.ExecutionSummaryLoadError
	if errors.As(err, &summaryErr) {
		return "execution_summary_load_failed"
	}
	return "governance_readiness_failed"
}

func wrapGovernanceReadinessError(operation, slug string, err error) error {
	if err == nil {
		return nil
	}

	if governanceReadinessErrorCode(err) == "skill_registry_invalid" {
		return wrapRequiredSkillsEvaluationError(operation, slug, err)
	}

	errorCode := governanceReadinessErrorCode(err)
	remediation := "Run `slipway repair` to restore canonical governance files, then fix malformed verification, bundle, or worktree authority data."
	if errorCode == "verification_load_failed" {
		remediation = "Run `slipway repair` to inspect authoritative verification files, then fix malformed verification records or restore unreadable verification files."
	}
	if errorCode == "execution_summary_load_failed" {
		remediation = "Run `slipway repair` to restore execution-summary authority, then fix malformed execution summary files."
	}
	details := map[string]any{
		"operation": operation,
	}
	var verificationErr *state.VerificationLoadError
	if errors.As(err, &verificationErr) && strings.TrimSpace(verificationErr.Path) != "" {
		details["path"] = verificationErr.Path
	}
	var summaryErr *state.ExecutionSummaryLoadError
	if errors.As(err, &summaryErr) && strings.TrimSpace(summaryErr.Path) != "" {
		details["path"] = summaryErr.Path
	}

	return newStateIntegrityError(
		errorCode,
		fmt.Sprintf("%s: %v", operation, err),
		remediation,
		slug,
		details,
	)
}

func readOnlyRequiredSkillInputs(change model.Change) []model.PlanSubStep {
	if change.CurrentState != model.StateS1Plan {
		return nil
	}
	if change.PlanSubStep == model.PlanSubStepNone {
		return nil
	}
	return []model.PlanSubStep{change.PlanSubStep}
}

func workflowStateLabel(currentState model.WorkflowState, intakeSubStep model.IntakeSubStep, planSubStep model.PlanSubStep) string {
	if currentState == model.StateS0Intake && intakeSubStep != "" {
		return fmt.Sprintf("%s/%s", currentState, intakeSubStep)
	}
	if currentState != model.StateS1Plan || planSubStep == model.PlanSubStepNone {
		return string(currentState)
	}
	return fmt.Sprintf("%s/%s", currentState, planSubStep)
}

func planningNote(currentState model.WorkflowState, planSubStep model.PlanSubStep) string {
	if currentState == model.StateS1Plan && planSubStep == model.PlanSubStepValidate {
		return "This is a recovery-only planning state entered after post-audit machine validation failed."
	}
	return ""
}

func resolveActiveChangeRef(root string, explicitSlug string) (changeRef, error) {
	// When --change is provided, load that specific change.
	if strings.TrimSpace(explicitSlug) != "" {
		return resolveExplicitChange(root, strings.TrimSpace(explicitSlug))
	}

	// Worktree-based resolution.
	worktreePath, err := currentWorktreeRoot()
	if err != nil {
		return changeRef{}, wrapResolutionError(err)
	}
	if worktreePath != "" {
		change, err := state.FindActiveChangeForWorktree(root, worktreePath)
		if err != nil {
			return changeRef{}, wrapResolutionError(err)
		}
		return changeRef{Slug: change.Slug}, nil
	}

	change, err := state.FindActiveChange(root)
	if err != nil {
		return changeRef{}, wrapResolutionError(err)
	}
	return changeRef{Slug: change.Slug}, nil
}

func addChangeSelectorFlags(cmd *cobra.Command, target *string, usage string) {
	cmd.Flags().StringVar(target, "change", "", usage)
}

// resolveExplicitChange loads a change by slug and verifies it is active.
func resolveExplicitChange(root string, slug string) (changeRef, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return changeRef{}, newPreconditionError(
				"no_active_change",
				fmt.Sprintf("no change found for slug %q", slug),
				"Check the slug with `slipway status`.",
				slug,
				nil,
			)
		}
		return changeRef{}, newStateIntegrityError(
			"change_state_load_failed",
			fmt.Sprintf("failed to load change state for %q: %v", slug, err),
			"Run `slipway repair` to inspect or repair change state files.",
			slug,
			map[string]any{
				"path": filepath.Join("artifacts", "changes", slug, "change.yaml"),
			},
		)
	}
	if change.Status != model.ChangeStatusActive {
		return changeRef{}, newPreconditionError(
			"not_active",
			fmt.Sprintf("change %q is not active; current status=%s", slug, change.Status),
			"Use `slipway status` to choose an active change, or inspect `artifacts/changes/<slug>/change.yaml` for state.",
			slug,
			map[string]any{
				"status": string(change.Status),
			},
		)
	}
	return changeRef{Slug: change.Slug}, nil
}

func loadChangeBySlug(root, slug string) (model.Change, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Change{}, newPreconditionError(
				"change_not_found",
				fmt.Sprintf("no change found for slug %q", slug),
				"Check the slug with `slipway status`.",
				slug,
				nil,
			)
		}
		return model.Change{}, newStateIntegrityError(
			"change_state_load_failed",
			fmt.Sprintf("failed to load change state for %q: %v", slug, err),
			"Run `slipway repair` to inspect or repair change state files.",
			slug,
			map[string]any{
				"path": filepath.Join("artifacts", "changes", slug, "change.yaml"),
			},
		)
	}
	return change, nil
}

func wrapResolutionError(err error) error {
	if errors.Is(err, state.ErrNoActiveChange) {
		return newPreconditionError(
			"no_active_change",
			"no active change; start one with `slipway new`",
			"Use `slipway new` to create a governed change.",
			"",
			nil,
		)
	}
	if errors.Is(err, state.ErrMultipleActiveChanges) {
		return newPreconditionError(
			"active_context_ambiguous",
			"active change context is ambiguous; use `--change <slug>` or run `slipway status`",
			"Specify change explicitly with `--change <slug>`, or run `slipway status` for diagnostics.",
			"",
			nil,
		)
	}
	return err
}

// loadActiveChange loads a change by slug and verifies it is active.
func loadActiveChange(root, slug, inactiveMessage, remediation string) (model.Change, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return model.Change{}, newStateIntegrityError(
			"change_state_load_failed",
			fmt.Sprintf("failed to load change state for %q: %v", slug, err),
			"Run `slipway repair` to inspect or repair change state files.",
			slug,
			map[string]any{
				"path": filepath.Join("artifacts", "changes", slug, "change.yaml"),
			},
		)
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

// currentWorktreeRoot returns the git worktree root for the current working directory.
func currentWorktreeRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		// Only fall back to CWD when git reports "not a git repository".
		// Other errors (git binary missing, permission denied) should propagate.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 &&
			strings.Contains(string(exitErr.Stderr), "not a git repository") {
			return os.Getwd()
		}
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func projectNextReadyActions(currentState model.WorkflowState) []string {
	return projectNextReadyActionsWithPrimary(currentState, "")
}

func projectNextReadyActionsWithPrimary(currentState model.WorkflowState, primary string) []string {
	actions := []string{}
	if currentState == model.StateDone {
		return actions
	}
	if primary = strings.TrimSpace(primary); primary != "" {
		actions = append(actions, primary)
	} else {
		switch currentState {
		case model.StateS2Execute:
			actions = append(actions, "run")
		default:
			actions = append(actions, "next")
		}
	}
	if currentState == model.StateS4Verify {
		actions = append(actions, "done")
	}
	if currentState == model.StateS2Execute || currentState == model.StateS3Review || currentState == model.StateS4Verify {
		actions = append(actions, "pivot")
	}
	if currentState == model.StateS2Execute {
		actions = append(actions, "abort")
	}
	actions = append(actions, "cancel")
	return actions
}

func projectFreshnessForExecMode(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	blockers []model.ReasonCode,
) string {
	if !state.ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return string(ctxpack.EvidenceFreshnessUnknown)
	}
	if hasFreshnessBlocker(blockers) {
		return string(ctxpack.EvidenceFreshnessStale)
	}
	return string(state.ExecutionSummaryFreshness(root, change, summary))
}

func hasFreshnessBlocker(blockers []model.ReasonCode) bool {
	for _, blocker := range blockers {
		if blocker.Code == state.StaleExecutionEvidenceBlockerToken {
			return true
		}
	}
	return false
}

// executionContext holds the resolved execution summary state used by status,
// validate, review, next, and stats commands.
type executionContext struct {
	Summary          *model.ExecutionSummary
	LatestRunVersion int
	Ready            bool
	// SummaryBlockers are the summary-level OpenBlockers from execution-summary.yaml.
	// These include wave-level blockers, parse issues, and session isolation warnings
	// that are not captured at the task level.
	SummaryBlockers []model.ReasonCode
}

type waveExecutionContext struct {
	Plan model.WavePlan
	Runs []model.WaveRun
}

func loadResumableWaveExecution(
	root string,
	change model.Change,
	execCtx executionContext,
	operation string,
) (*waveExecutionContext, int, error) {
	if change.CurrentState != model.StateS2Execute || !execCtx.Ready {
		return nil, 0, nil
	}

	waveCtx, err := loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, operation)
	if err != nil || waveCtx == nil {
		return nil, 0, err
	}
	return waveCtx, state.ResumeWaveIndex(waveCtx.Plan, waveCtx.Runs), nil
}

func validateActiveCheckpointAuthority(
	root string,
	change model.Change,
	execCtx executionContext,
	operation string,
) error {
	if change.ActiveCheckpoint == nil || change.CurrentState != model.StateS2Execute {
		return nil
	}

	var plan model.WavePlan
	if execCtx.Ready && execCtx.LatestRunVersion > 0 {
		waveCtx, err := loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, operation)
		if err != nil {
			return err
		}
		if waveCtx == nil {
			return nil
		}
		plan = waveCtx.Plan
	} else {
		loadedPlan, err := state.LoadWavePlanForChange(root, change)
		if err != nil {
			errorCode := "wave_plan_load_failed"
			message := fmt.Sprintf("%s failed to load wave-plan.yaml for %q: %v", operation, change.Slug, err)
			if errors.Is(err, fs.ErrNotExist) {
				errorCode = "wave_plan_missing"
				message = fmt.Sprintf("%s requires wave-plan.yaml for active checkpoint %q, but it is missing", operation, change.Slug)
			}
			return newStateIntegrityError(
				errorCode,
				message,
				"Run `slipway repair` to restore wave execution artifacts before continuing.",
				change.Slug,
				map[string]any{
					"path": state.WavePlanPathForRead(root, change.Slug),
				},
			)
		}
		plan = loadedPlan
	}

	return validateActiveCheckpointWavePlan(root, change, plan, operation)
}

func validateActiveCheckpointWavePlan(root string, change model.Change, plan model.WavePlan, operation string) error {
	if change.ActiveCheckpoint == nil {
		return nil
	}

	expectedWaveIndex := plan.WaveIndexForTask(change.ActiveCheckpoint.PausedTaskID)
	if expectedWaveIndex == 0 {
		return newStateIntegrityError(
			"checkpoint_task_missing_from_wave_plan",
			fmt.Sprintf("%s found active checkpoint task %q is not present in wave-plan.yaml for %q", operation, change.ActiveCheckpoint.PausedTaskID, change.Slug),
			"Run `slipway repair` to clear the stale checkpoint before resuming execution.",
			change.Slug,
			map[string]any{
				"path":    state.WavePlanPathForRead(root, change.Slug),
				"task_id": change.ActiveCheckpoint.PausedTaskID,
			},
		)
	}
	if change.ActiveCheckpoint.PausedWaveIndex != expectedWaveIndex {
		return newStateIntegrityError(
			"checkpoint_wave_index_drift",
			fmt.Sprintf("%s found checkpoint wave index drift for %q: task %q belongs to wave %d, checkpoint points at wave %d", operation, change.Slug, change.ActiveCheckpoint.PausedTaskID, expectedWaveIndex, change.ActiveCheckpoint.PausedWaveIndex),
			"Run `slipway repair` to rewrite the checkpoint wave index before resuming execution.",
			change.Slug,
			map[string]any{
				"path":                  state.WavePlanPathForRead(root, change.Slug),
				"task_id":               change.ActiveCheckpoint.PausedTaskID,
				"expected_wave_index":   expectedWaveIndex,
				"checkpoint_wave_index": change.ActiveCheckpoint.PausedWaveIndex,
			},
		)
	}
	return nil
}

// loadExecutionContext loads the execution summary for a change and extracts
// the unified readiness and blocker surface. This is the single call site for
// execution-summary authority consumption.
func loadExecutionContext(root string, change model.Change) (executionContext, error) {
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return executionContext{}, newStateIntegrityError(
			"execution_summary_load_failed",
			fmt.Sprintf("failed to load execution summary for %q: %v", change.Slug, err),
			"Run `slipway repair` to inspect or repair execution summary files.",
			change.Slug,
			map[string]any{
				"path": state.ExecutionSummaryPathForRead(root, change.Slug),
			},
		)
	}
	ctx := executionContext{Summary: summary}
	if summary != nil {
		ctx.LatestRunVersion = summary.RunSummaryVersion
		ctx.Ready = state.ExecutionSummaryReady(summary)
		if len(summary.OpenBlockers) > 0 {
			ctx.SummaryBlockers = append(ctx.SummaryBlockers, summary.OpenBlockers...)
		}
	}
	return ctx, nil
}

func loadAuthoritativeWaveExecution(
	root string,
	change model.Change,
	runVersion int,
	operation string,
) (*waveExecutionContext, error) {
	if runVersion < 1 {
		return nil, nil
	}

	plan, err := state.LoadWavePlanForChange(root, change)
	if err != nil {
		errorCode := "wave_plan_load_failed"
		message := fmt.Sprintf("%s failed to load wave-plan.yaml for %q: %v", operation, change.Slug, err)
		if errors.Is(err, fs.ErrNotExist) {
			errorCode = "wave_plan_missing"
			message = fmt.Sprintf("%s requires wave-plan.yaml for %q, but it is missing", operation, change.Slug)
		}
		return nil, newStateIntegrityError(
			errorCode,
			message,
			"Run `slipway repair` to restore wave execution artifacts before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WavePlanPathForRead(root, change.Slug),
			},
		)
	}

	runs, err := state.LoadOptionalWaveRuns(root, change.Slug, runVersion)
	if err != nil {
		return nil, newStateIntegrityError(
			"wave_runs_load_failed",
			fmt.Sprintf("%s failed to load wave run evidence for %q: %v", operation, change.Slug, err),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug, runVersion),
			},
		)
	}
	if len(plan.Waves) > 0 && len(runs) == 0 {
		return nil, newStateIntegrityError(
			"wave_runs_missing",
			fmt.Sprintf("%s requires wave run evidence for %q, but none was found for rv%d", operation, change.Slug, runVersion),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug, runVersion),
			},
		)
	}
	if len(runs) > 0 && len(runs) < len(plan.Waves) {
		return nil, newStateIntegrityError(
			"wave_runs_incomplete",
			fmt.Sprintf("%s found incomplete wave run evidence for %q: %d of %d waves are present for rv%d", operation, change.Slug, len(runs), len(plan.Waves), runVersion),
			"Run `slipway repair` to reconstruct the missing wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug, runVersion),
			},
		)
	}
	if len(runs) > len(plan.Waves) {
		return nil, newStateIntegrityError(
			"wave_runs_invalid_count",
			fmt.Sprintf("%s found more wave runs than planned waves for %q (%d > %d)", operation, change.Slug, len(runs), len(plan.Waves)),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug, runVersion),
			},
		)
	}
	for _, run := range runs {
		if run.RunSummaryVersion == runVersion {
			continue
		}
		return nil, newStateIntegrityError(
			"wave_run_version_mismatch",
			fmt.Sprintf("%s found wave evidence version drift for %q: wave %d points at rv%d, expected rv%d", operation, change.Slug, run.WaveIndex, run.RunSummaryVersion, runVersion),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug, runVersion),
			},
		)
	}
	if linkageIssues := state.WaveTaskLinkageIssues(plan, runs); len(linkageIssues) > 0 {
		return nil, newStateIntegrityError(
			"wave_task_linkage_mismatch",
			fmt.Sprintf("%s found wave/task linkage mismatch for %q: %s", operation, change.Slug, strings.Join(linkageIssues, "; ")),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path":   state.WaveEvidenceDir(root, change.Slug, runVersion),
				"issues": linkageIssues,
			},
		)
	}

	return &waveExecutionContext{
		Plan: plan,
		Runs: runs,
	}, nil
}

// encodeJSONResponse encodes v as indented JSON to the command's stdout.
func encodeJSONResponse(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// filterAlivePIDs returns only PIDs that are still running.
func filterAlivePIDs(pids []int) []int {
	alive := []int{}
	for _, pid := range pids {
		if isPIDAlive(pid) {
			alive = append(alive, pid)
		}
	}
	return alive
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

// rejectIfConflictingChange enforces the single-active-change contract before
// creating a new change.
func rejectIfConflictingChange(root string) error {
	allChanges, err := state.ListChangesForCreateGuard(root)
	if err != nil {
		return err
	}

	var activeChanges []model.Change
	for _, ch := range allChanges {
		if ch.Status == model.ChangeStatusActive {
			activeChanges = append(activeChanges, ch)
		}
	}
	if len(activeChanges) == 0 {
		return nil
	}

	currentWT, wtErr := currentWorktreeRoot()
	if wtErr != nil {
		return fmt.Errorf("cannot determine worktree for conflict check: %w", wtErr)
	}

	ch := activeChanges[0]
	if strings.TrimSpace(ch.WorktreePath) == "" {
		remediation := "Run `slipway next` to bind the existing change to a dedicated worktree, or close it before creating a new change."
		return newPreconditionError(
			"active_change_exists",
			fmt.Sprintf("active governed change %s is not yet bound to a worktree; finish, cancel, or bind it before creating a new one", ch.Slug),
			remediation,
			ch.Slug,
			nil,
		)
	}

	if currentWT != "" && ch.WorktreePath != "" {
		normalizedCurrent, err1 := state.NormalizePath(currentWT)
		normalizedExisting, err2 := state.NormalizePath(ch.WorktreePath)
		if err1 == nil && err2 == nil && normalizedCurrent == normalizedExisting {
			return newPreconditionError(
				"active_change_exists",
				fmt.Sprintf("active governed change %s is bound to this worktree; finish or cancel it before creating a new one", ch.Slug),
				"Run `slipway done` or `slipway cancel` before creating a new change.",
				ch.Slug,
				nil,
			)
		}
	}

	worktreeHint := strings.TrimSpace(state.DisplayPath(root, ch.WorktreePath))
	if worktreeHint == "" {
		worktreeHint = ch.WorktreePath
	}
	remediation := "Resume, finish, or cancel the existing change before creating a new change."
	if worktreeHint != "" {
		remediation = fmt.Sprintf(
			"Switch to %s to continue that change, or run `slipway done` / `slipway cancel` there before creating a new change.",
			worktreeHint,
		)
	}
	return newPreconditionError(
		"active_change_exists",
		fmt.Sprintf("active governed change %s already exists; finish or cancel it before creating a new one", ch.Slug),
		remediation,
		ch.Slug,
		nil,
	)
}
