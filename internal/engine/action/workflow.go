package action

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/speclane/internal/engine/artifact"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
)

type LoopTrigger string

const (
	LoopTriggerPlanAuditFailed LoopTrigger = "plan_audit_failed"
	LoopTriggerRetry           LoopTrigger = "retry"
	LoopTriggerPivotReroute    LoopTrigger = "pivot_reroute"
	LoopTriggerPivotRescope    LoopTrigger = "pivot_rescope"
	LoopTriggerReviewFailed    LoopTrigger = "review_failed"
	LoopTriggerReviewOverride  LoopTrigger = "review_override"
	LoopTriggerVerifyFailed    LoopTrigger = "verify_failed"
)

type LoopTransitionInput struct {
	Level             model.Level
	CurrentState      model.WorkflowState
	Trigger           LoopTrigger
	AnalyzeLevel      model.Level
	PivotGateApproved bool
}

func LevelPath(level model.Level) []model.WorkflowState {
	switch level {
	case model.LevelL1:
		return []model.WorkflowState{
			model.StateS0Intake,
			model.StateS1Analyze,
			model.StateS6RunWaves,
			model.StateS7Review,
			model.StateS8Verify,
			model.StateDone,
		}
	case model.LevelL2:
		return []model.WorkflowState{
			model.StateS0Intake,
			model.StateS1Analyze,
			model.StateS4SpecBundle,
			model.StateS5PlanAudit,
			model.StateS6RunWaves,
			model.StateS7Review,
			model.StateS8Verify,
			model.StateDone,
		}
	case model.LevelL3:
		return []model.WorkflowState{
			model.StateS0Intake,
			model.StateS1Analyze,
			model.StateS2Discover,
			model.StateS3ScopeConfirmation,
			model.StateS4SpecBundle,
			model.StateS5PlanAudit,
			model.StateS6RunWaves,
			model.StateS7Review,
			model.StateS8Verify,
			model.StateDone,
		}
	default:
		return nil
	}
}

func EvaluateFixedLevelSafety(level model.Level, guardrailDomain string) []string {
	conflicts := []string{}
	if level == model.LevelL1 && strings.TrimSpace(guardrailDomain) != "" {
		conflicts = append(conflicts, "fixed_level_guardrail_conflict")
	}
	return conflicts
}

func ApplyAnalyzeOverrideAdmission(admission *model.AdmissionState, hardConflicts []string) error {
	if admission == nil {
		return fmt.Errorf("admission is required")
	}
	if admission.AdmissionStatus != model.AdmissionStatusActive {
		return fmt.Errorf("analyze override requires admission_status=active")
	}
	admission.CurrentState = model.StateS1Analyze
	admission.RouteSnapshot.BlockingConflicts = normalizeConflicts(hardConflicts)
	return nil
}

func ApplyAnalyzeOverrideChange(change *model.ChangeState, hardConflicts []string) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if change.ChangeStatus != model.ChangeStatusActive {
		return fmt.Errorf("analyze override requires change_status=active")
	}
	change.CurrentState = model.StateS1Analyze
	change.RouteSnapshot.BlockingConflicts = normalizeConflicts(hardConflicts)
	return nil
}

func ResolveLoopTransition(input LoopTransitionInput) (model.WorkflowState, error) {
	switch input.CurrentState {
	case model.StateS5PlanAudit:
		if input.Trigger == LoopTriggerPlanAuditFailed {
			return model.StateS4SpecBundle, nil
		}
	case model.StateS6RunWaves:
		switch input.Trigger {
		case LoopTriggerRetry:
			return model.StateS6RunWaves, nil
		case LoopTriggerPivotReroute:
			return model.StateS1Analyze, nil
		case LoopTriggerPivotRescope:
			return resolveRescopeTransition(input)
		}
	case model.StateS7Review:
		switch input.Trigger {
		case LoopTriggerReviewFailed:
			return model.StateS6RunWaves, nil
		case LoopTriggerPivotReroute:
			return model.StateS1Analyze, nil
		case LoopTriggerPivotRescope:
			return "", fmt.Errorf("rescope requires current_state=S6_RUN_WAVES")
		}
	case model.StateS8Verify:
		switch input.Trigger {
		case LoopTriggerReviewOverride:
			return model.StateS7Review, nil
		case LoopTriggerVerifyFailed:
			return model.StateS6RunWaves, nil
		case LoopTriggerPivotReroute:
			return model.StateS1Analyze, nil
		case LoopTriggerPivotRescope:
			return "", fmt.Errorf("rescope requires current_state=S6_RUN_WAVES")
		}
	}

	return "", fmt.Errorf("unsupported loop transition: state=%s trigger=%s", input.CurrentState, input.Trigger)
}

func resolveRescopeTransition(input LoopTransitionInput) (model.WorkflowState, error) {
	if input.CurrentState != model.StateS6RunWaves {
		return "", fmt.Errorf("rescope requires current_state=S6_RUN_WAVES")
	}
	if !input.PivotGateApproved {
		return "", fmt.Errorf("rescope requires approved pivot")
	}
	if input.Level != model.LevelL2 && input.Level != model.LevelL3 {
		return "", fmt.Errorf("rescope requires governed level")
	}
	if !input.AnalyzeLevel.IsValid() {
		return "", fmt.Errorf("rescope requires valid analyze level")
	}

	if input.AnalyzeLevel != input.Level {
		return model.StateS1Analyze, nil
	}
	if input.Level == model.LevelL2 {
		return model.StateS4SpecBundle, nil
	}
	return model.StateS3ScopeConfirmation, nil
}

func normalizeConflicts(conflicts []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		conflict = strings.TrimSpace(conflict)
		if conflict == "" {
			continue
		}
		if _, exists := seen[conflict]; exists {
			continue
		}
		seen[conflict] = struct{}{}
		normalized = append(normalized, conflict)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

type L1AutoCheckResult struct {
	NextState model.WorkflowState `json:"next_state"`
	DoneReady bool                `json:"done_ready"`
	Blockers  []string            `json:"blockers"`
}

func RunL1DoAutoChecks(admission model.AdmissionState) L1AutoCheckResult {
	result := L1AutoCheckResult{
		NextState: model.StateS6RunWaves,
		Blockers:  []string{},
	}

	latest := admission.LatestFrozenRunSummaryVersion
	if latest < 1 {
		result.Blockers = append(result.Blockers, "missing_frozen_run_summary")
		return result
	}

	latestRuns := make([]model.TaskRun, 0)
	for key, run := range admission.TaskRuns {
		if run.RunSummaryVersion != latest {
			result.Blockers = append(result.Blockers, "run_summary_mismatch:"+key)
			continue
		}
		latestRuns = append(latestRuns, run)
	}
	if len(latestRuns) == 0 {
		result.Blockers = append(result.Blockers, "no_tasks_for_latest_run_summary")
		return result
	}

	// S7 lightweight check: run-summary consistency + all pass.
	for _, run := range latestRuns {
		if run.Verdict != model.TaskVerdictPass {
			result.Blockers = append(result.Blockers, "non_pass_task:"+run.TaskID)
		}
	}

	// S8 lightweight check: pass tasks need evidence and no unresolved blockers.
	for _, run := range latestRuns {
		if run.Verdict == model.TaskVerdictPass && strings.TrimSpace(run.EvidenceRef) == "" {
			result.Blockers = append(result.Blockers, "missing_evidence_ref:"+run.TaskID)
		}
		if len(run.Blockers) > 0 {
			result.Blockers = append(result.Blockers, "unresolved_blockers:"+run.TaskID)
		}
	}

	if len(result.Blockers) == 0 {
		result.DoneReady = true
		result.NextState = model.StateS8Verify
	}
	return result
}

func CanFinalizeDone(state model.WorkflowState) bool {
	return state == model.StateS8Verify
}

func RunS2Discover(change *model.ChangeState, discoveryContent string) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if change.CurrentState != model.StateS2Discover {
		return fmt.Errorf("S2 discover requires current_state=S2_DISCOVER")
	}
	if strings.TrimSpace(discoveryContent) == "" {
		return fmt.Errorf("discovery content is required")
	}
	change.CurrentState = model.StateS3ScopeConfirmation
	return nil
}

func RunS3ScopeConfirmation(
	change *model.ChangeState,
	repoRoot string,
	worktreePath string,
	worktreeBranch string,
) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if change.CurrentState != model.StateS3ScopeConfirmation {
		return fmt.Errorf("scope confirmation requires current_state=S3_SCOPE_CONFIRMATION")
	}
	if err := state.PersistScopeWorktreeMetadata(change, worktreePath, worktreeBranch); err != nil {
		return err
	}
	if err := state.ValidateWorktreeAuthenticity(repoRoot, worktreePath, worktreeBranch); err != nil {
		return err
	}
	change.CurrentState = model.StateS4SpecBundle
	return nil
}

func RunS4SpecBundle(root string, change *model.ChangeState) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if change.CurrentState != model.StateS4SpecBundle {
		return fmt.Errorf("spec bundle requires current_state=S4_SPEC_BUNDLE")
	}

	base := filepath.Join(root, "aircraft", "changes", change.Slug)
	required := []string{"change.yaml", "proposal.md", "spec.md", "design.md", "tasks.md", "assurance.md"}
	if change.Level == model.LevelL3 {
		required = append(required, "explore.md")
	}

	for _, file := range required {
		path := filepath.Join(base, file)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("missing required artifact: %s", file)
			}
			return err
		}
	}

	if change.Level == model.LevelL3 {
		explorePath := filepath.Join(base, "explore.md")
		content, err := os.ReadFile(explorePath)
		if err != nil {
			return err
		}
		if err := artifact.ValidateExploreStructure(string(content)); err != nil {
			return err
		}
	}

	change.CurrentState = model.StateS5PlanAudit
	return nil
}
