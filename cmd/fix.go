package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/wave"
	"github.com/spf13/cobra"
)

type fixView struct {
	Slug                 string             `json:"slug"`
	ExecutionMode        string             `json:"execution_mode"`
	QualityMode          string             `json:"quality_mode,omitempty"`
	CurrentState         string             `json:"current_state"`
	SelectedReviewSkills []string           `json:"selected_review_skills,omitempty"`
	RepairTargets        []fixRepairTarget  `json:"repair_targets,omitempty"`
	Contract             fixRepairContract  `json:"contract"`
	Blockers             []model.ReasonCode `json:"blockers,omitempty"`
}

type fixRepairTarget struct {
	Reviewer     string             `json:"reviewer,omitempty"`
	Kind         string             `json:"kind"`
	Reason       string             `json:"reason"`
	Detail       string             `json:"detail,omitempty"`
	EvidencePath string             `json:"evidence_path,omitempty"`
	Blockers     []model.ReasonCode `json:"blockers,omitempty"`
}

type fixRepairContract struct {
	RepairBatchID                         string             `json:"repair_batch_id"`
	CollectAllSelectedReviewFindingsFirst bool               `json:"collect_all_selected_review_findings_first"`
	RequiresFreshContext                  bool               `json:"requires_fresh_context"`
	Subagent                              *subagentDirective `json:"subagent,omitempty"`
	FindingCollection                     string             `json:"finding_collection"`
	Dispatch                              string             `json:"dispatch"`
	RepairBrief                           string             `json:"repair_brief"`
	ContextReference                      string             `json:"context_reference"`
	RecordEvidence                        []string           `json:"record_evidence"`
	AfterRepair                           []string           `json:"after_repair"`
	Prohibited                            []string           `json:"prohibited"`
}

func makeFixCmd() *cobra.Command {
	var changeSlug string
	var jsonOutput bool
	var reviewer string
	var startReexecution bool
	var discardPriorEvidence bool
	cmd := &cobra.Command{
		Use:   "fix",
		Short: desc("fix"),
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
			view, err := buildFixViewForSlug(root, ref.Slug, reviewer, startReexecution, discardPriorEvidence)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeJSONResponse(cmd, view)
			}
			return writeFixText(cmd.OutOrStdout(), view)
		},
	}

	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().StringVar(&reviewer, "reviewer", "", "Limit repair target discovery to one selected review skill")
	cmd.Flags().BoolVar(&startReexecution, "start-reexecution", false, "Start a fresh S2 execution run for review-driven implementation repairs")
	cmd.Flags().BoolVar(&discardPriorEvidence, "discard-prior-evidence", false, "Confirm that --start-reexecution may discard prior task evidence instead of using S3 in-place convergence")
	return cmd
}

func buildFixViewForSlug(root, slug, reviewerFilter string, startReexecution, discardPriorEvidence bool) (fixView, error) {
	if discardPriorEvidence && !startReexecution {
		return fixView{}, newInvalidUsageError(
			"discard_prior_evidence_requires_reexecution",
			"--discard-prior-evidence is only valid with --start-reexecution",
			"Use `slipway fix --start-reexecution --discard-prior-evidence` only when intentionally opening a fresh execution boundary; otherwise omit the flag.",
			nil,
		)
	}
	change, err := loadActiveChange(
		root,
		slug,
		"fix requires active change; current status=%s",
		"Only active changes can run review finding fixes.",
	)
	if err != nil {
		return fixView{}, err
	}
	if change.CurrentState != model.StateS3Review {
		currentCommand := primaryCommandForState(change.CurrentState)
		return fixView{}, newGovernanceBlockedError(
			"fix_state_invalid",
			fmt.Sprintf("slipway fix can only run while current_state is %s; current_state=%s", model.StateS3Review, change.CurrentState),
			fmt.Sprintf("Use `slipway %s` for the current state; `slipway fix` is only for S3 review findings.", currentCommand),
			slug,
			map[string]any{
				"current_state":  change.CurrentState,
				"expected_state": model.StateS3Review,
				"next_command":   "slipway " + currentCommand,
			},
		)
	}

	reviewState := model.StateS3Review
	readiness, err := progression.EvaluateGovernanceReadiness(
		root,
		change,
		progression.GovernanceReadinessOptions{
			WorkflowStateOverride: &reviewState,
			IncludeReviewSurface:  true,
		},
	)
	if err != nil {
		return fixView{}, wrapGovernanceReadinessError("evaluate review fix prerequisites", change.Slug, err)
	}

	selected := selectedReviewSkillsFromReadiness(readiness, change.EffectiveWorkflowProfile())
	if reviewerFilter = strings.TrimSpace(reviewerFilter); reviewerFilter != "" && !stringInSlice(selected, reviewerFilter) {
		return fixView{}, newInvalidUsageError(
			"reviewer_not_selected",
			fmt.Sprintf("reviewer %q is not in the selected S3 review set", reviewerFilter),
			"Choose one of the selected_review_skills from `slipway fix --json`, or omit `--reviewer`.",
			map[string]any{"selected_review_skills": selected},
		)
	}

	verifications, err := state.ListVerificationsForChange(root, change)
	if err != nil {
		return fixView{}, err
	}
	targets := reviewFixTargets(root, change, selected, reviewerFilter, verifications, readiness.Blockers)

	if startReexecution {
		if convergence, guardErr := s3ReviewTaskConvergencePending(root, change); guardErr != nil {
			return fixView{}, guardErr
		} else if convergence.Pending && !discardPriorEvidence {
			return fixView{}, newCLIErrorWithReasons(
				categoryPrecondition,
				"fix_start_reexecution_inplace_convergence_available",
				"tasks.md has review-discovered task-plan amendments that Slipway can absorb in place at S3_REVIEW",
				"Run `slipway run` to re-materialize the current wave projection at the same run_summary_version. Record task evidence only for newly added tasks surfaced as incomplete; edited already-evidenced tasks stay frozen and are re-certified through review evidence before wave-orchestration is re-recorded. `slipway fix --start-reexecution` would bump the run version and clear existing task evidence; pass `--discard-prior-evidence` only when that discard is intentional.",
				change.Slug,
				s3InPlaceConvergenceReasonCodes(convergence.ReasonSubjects()),
				map[string]any{
					"added_tasks":                convergence.AddedTasks,
					"changed_tasks":              convergence.ChangedTasks,
					"removed_tasks":              convergence.RemovedTasks,
					"drifted_tasks":              convergence.ReasonSubjects(),
					"remediation_command_hint":   "slipway run",
					"destructive_effect_blocked": "would bump run_summary_version and clear existing task evidence",
					"override_flag":              "--discard-prior-evidence",
				},
			)
		}
		change, err = startFixReexecution(root, change)
		if err != nil {
			return fixView{}, err
		}
	}
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return fixView{}, err
	}

	profile := buildChangeProfileView(change)
	return fixView{
		Slug:                 slug,
		ExecutionMode:        governedExecutionMode,
		QualityMode:          profile.QualityMode,
		CurrentState:         string(change.CurrentState),
		SelectedReviewSkills: selected,
		RepairTargets:        targets,
		Contract:             reviewFixContract(slug, cfg),
		Blockers:             model.NormalizeReasonCodes(readiness.Blockers),
	}, nil
}

func startFixReexecution(root string, change model.Change) (model.Change, error) {
	nextRunVersion, err := nextExecutionRunSummaryVersion(root, change)
	if err != nil {
		return model.Change{}, err
	}

	reexecution := change
	reexecution.TransitionTo(model.StateS2Implement)
	reexecution.ClearAutoPassHistory()

	wavePlanOp, err := materializeReexecutionWavePlanOp(root, reexecution, nextRunVersion)
	if err != nil {
		return model.Change{}, err
	}
	changeOps, err := state.SaveChangeTransactionOps(root, reexecution)
	if err != nil {
		return model.Change{}, err
	}
	transactionOps := make([]fsutil.FileTransactionOp, 0, len(changeOps)+2)
	transactionOps = append(transactionOps, fsutil.RemoveAllTransactionOp(state.EvidenceTasksDir(root, change.Slug)))
	transactionOps = append(transactionOps, wavePlanOp)
	transactionOps = append(transactionOps, changeOps...)
	if err := fsutil.ApplyFileTransaction(transactionOps); err != nil {
		return model.Change{}, err
	}
	if err := appendCLILifecycleEvent(root, reexecution, state.LifecycleEvent{
		Command:     "fix",
		EventType:   "execution.reopened",
		Action:      "started_reexecution",
		Result:      "advanced",
		BeforeState: change.CurrentState,
		AfterState:  reexecution.CurrentState,
		Diagnostics: []string{
			fmt.Sprintf("run_summary_version=%d", nextRunVersion),
		},
	}); err != nil {
		return model.Change{}, err
	}
	return reexecution, nil
}

func materializeReexecutionWavePlanOp(
	root string,
	change model.Change,
	runSummaryVersion int,
) (fsutil.FileTransactionOp, error) {
	_, op, err := state.MaterializeWavePlanTransactionOpAtRunSummaryVersion(
		root,
		change,
		time.Now().UTC(),
		runSummaryVersion,
	)
	return op, err
}

func nextExecutionRunSummaryVersion(root string, change model.Change) (int, error) {
	latest := 0
	if plan, err := state.LoadOptionalWavePlanForChange(root, change); err != nil {
		return 0, err
	} else if plan != nil && plan.RunSummaryVersion > latest {
		latest = plan.RunSummaryVersion
	}
	execCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return 0, err
	}
	if execCtx.LatestRunVersion > latest {
		latest = execCtx.LatestRunVersion
	}
	if latest < 1 {
		return 1, nil
	}
	return latest + 1, nil
}

type s3ReviewTaskConvergence struct {
	Pending      bool
	AddedTasks   []string
	ChangedTasks []string
	RemovedTasks []string
}

func (c s3ReviewTaskConvergence) ReasonSubjects() []string {
	subjects := append([]string{}, c.AddedTasks...)
	subjects = append(subjects, c.ChangedTasks...)
	subjects = append(subjects, c.RemovedTasks...)
	slices.Sort(subjects)
	if len(subjects) == 0 && c.Pending {
		return []string{"tasks.md"}
	}
	return subjects
}

func s3ReviewTaskConvergencePending(root string, change model.Change) (s3ReviewTaskConvergence, error) {
	if change.CurrentState != model.StateS3Review {
		return s3ReviewTaskConvergence{}, nil
	}
	drift, err := state.CurrentTasksPlanDriftFromWavePlan(root, change)
	if err != nil || !drift.HasWavePlan {
		return s3ReviewTaskConvergence{}, err
	}
	if !drift.Drifted() {
		return s3ReviewTaskConvergence{}, nil
	}
	plan := drift.Plan
	current, err := currentTaskPlanByIDForFix(root, change)
	if err != nil {
		return s3ReviewTaskConvergence{}, err
	}

	persisted := map[string]model.WavePlanTask{}
	var changed []string
	var removed []string
	for _, plannedWave := range plan.Waves {
		for _, task := range plannedWave.Tasks {
			taskID := strings.TrimSpace(task.TaskID)
			if taskID == "" {
				continue
			}
			persisted[taskID] = task
			currentTask, ok := current[taskID]
			if !ok {
				removed = append(removed, taskID)
			} else if !sameFixTaskProjection(task, currentTask) {
				changed = append(changed, taskID)
			}
		}
	}

	var added []string
	for taskID := range current {
		if _, ok := persisted[taskID]; !ok {
			added = append(added, taskID)
		}
	}
	slices.Sort(added)
	slices.Sort(changed)
	slices.Sort(removed)
	return s3ReviewTaskConvergence{Pending: true, AddedTasks: added, ChangedTasks: changed, RemovedTasks: removed}, nil
}

func s3InPlaceConvergenceReasonCodes(taskIDs []string) []model.ReasonCode {
	reasons := make([]model.ReasonCode, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID = strings.TrimSpace(taskID); taskID != "" {
			reasons = append(reasons, model.NewReasonCode("s3_task_plan_drift_requires_inplace_convergence", taskID))
		}
	}
	return model.NormalizeReasonCodes(reasons)
}

func currentTaskPlanByIDForFix(root string, change model.Change) (map[string]wave.TaskNode, error) {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md")) // #nosec G304 -- governed bundle path is resolved by Slipway state authority.
	if err != nil {
		return nil, err
	}
	plan, err := wave.ParseTaskPlan(string(raw))
	if err != nil {
		return nil, err
	}
	byID := make(map[string]wave.TaskNode, len(plan.Tasks))
	for _, task := range plan.Tasks {
		if taskID := strings.TrimSpace(task.TaskID); taskID != "" {
			byID[taskID] = task
		}
	}
	return byID, nil
}

func sameFixTaskProjection(persisted model.WavePlanTask, current wave.TaskNode) bool {
	return strings.TrimSpace(persisted.TaskID) == strings.TrimSpace(current.TaskID) &&
		strings.TrimSpace(persisted.Objective) == strings.TrimSpace(current.Objective) &&
		persisted.TaskKind == current.TaskKind &&
		slices.Equal(normalizeFixStringList(persisted.DependsOn), normalizeFixStringList(current.DependsOn)) &&
		slices.Equal(normalizeFixPathList(persisted.TargetFiles), normalizeFixPathList(current.TargetFiles))
}

func normalizeFixStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	return out
}

func normalizeFixPathList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := model.NormalizePublicPath(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	return out
}

func reviewFixTargets(
	root string,
	change model.Change,
	selected []string,
	reviewerFilter string,
	verifications map[string]model.VerificationRecord,
	readinessBlockers []model.ReasonCode,
) []fixRepairTarget {
	targets := []fixRepairTarget{}
	addTarget := func(target fixRepairTarget) {
		if reviewerFilter != "" && target.Reviewer != "" && target.Reviewer != reviewerFilter {
			return
		}
		targets = append(targets, target)
	}

	for _, skillName := range selected {
		record, ok := verifications[skillName]
		if !ok {
			continue
		}
		if record.IsPassing() {
			continue
		}
		target := fixRepairTarget{
			Reviewer:     skillName,
			Kind:         "review_finding",
			Reason:       "reviewer_recorded_non_passing_evidence",
			EvidencePath: state.DisplayPath(root, state.VerificationFilePath(root, change.Slug, skillName)),
			Blockers:     model.NormalizeReasonCodes(record.Blockers),
		}
		if record.Verdict != "" {
			target.Detail = string(record.Verdict)
		}
		addTarget(target)
	}

	for _, blocker := range model.NormalizeReasonCodes(readinessBlockers) {
		if target, ok := reviewFixTargetForBlocker(root, change, selected, blocker); ok {
			addTarget(target)
		}
	}

	slices.SortFunc(targets, func(a, b fixRepairTarget) int {
		if c := strings.Compare(a.Reviewer, b.Reviewer); c != 0 {
			return c
		}
		if c := strings.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return strings.Compare(a.Detail, b.Detail)
	})
	return compactFixRepairTargets(targets)
}

func reviewFixTargetForBlocker(root string, change model.Change, selected []string, blocker model.ReasonCode) (fixRepairTarget, bool) {
	code := strings.TrimSpace(blocker.Code)
	detail := strings.TrimSpace(blocker.Detail)
	switch code {
	case "review_alignment_required":
		reviewer := reviewAlignmentSkillForTarget(selected, detail)
		if reviewer == "" {
			reviewer = detail
		}
		return fixRepairTarget{
			Reviewer: reviewer,
			Kind:     "alignment",
			Reason:   code,
			Detail:   detail,
			Blockers: []model.ReasonCode{blocker},
		}, true
	case "scope_contract_drift":
		return fixRepairTarget{
			Kind:     "scope_amendment",
			Reason:   code,
			Detail:   detail,
			Blockers: []model.ReasonCode{blocker},
		}, true
	case "required_skill_not_passed", "required_skill_blockers_present":
		if !stringInSlice(selected, detail) {
			return fixRepairTarget{}, false
		}
		return fixRepairTarget{
			Reviewer:     detail,
			Kind:         "review_finding",
			Reason:       code,
			Detail:       detail,
			EvidencePath: state.DisplayPath(root, state.VerificationFilePath(root, change.Slug, detail)),
			Blockers:     []model.ReasonCode{blocker},
		}, true
	default:
		return fixRepairTarget{}, false
	}
}

func compactFixRepairTargets(targets []fixRepairTarget) []fixRepairTarget {
	if len(targets) == 0 {
		return nil
	}
	out := make([]fixRepairTarget, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range targets {
		key := strings.Join([]string{target.Reviewer, target.Kind, target.Reason, target.Detail}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	return out
}

func reviewFixContract(slug string, cfg model.Config) fixRepairContract {
	batchID := "s3-review-repair:" + strings.TrimSpace(slug)
	return fixRepairContract{
		RepairBatchID:                         batchID,
		CollectAllSelectedReviewFindingsFirst: true,
		RequiresFreshContext:                  true,
		Subagent:                              subagentDirectiveForSlot(cfg, model.SubagentSlotFix),
		FindingCollection:                     "Collect all selected S3 reviewer findings first, then consolidate by root cause before dispatching repair.",
		Dispatch:                              "Use contract.subagent when present to dispatch the repair session; otherwise default to a native fresh-context repair subagent. Pass paths and blocker facts, not the host conversation.",
		RepairBrief:                           "One repair brief covers the open finding set for this repair_batch_id. Do not repair one reviewer finding while the selected review batch is still collecting findings.",
		ContextReference:                      model.ContextOriginReferencePrefix + model.StageContextFix + "=<repair-subagent-handle>",
		RecordEvidence: []string{
			"After the repair, rerun the affected selected reviewer(s) under their own fresh review contexts.",
			"Record the affected reviewer with `slipway evidence skill --skill <reviewer> --verdict pass --reference \"context_origin:stage=review=<reviewer-handle>\" --reference \"context_origin:stage=fix=<repair-subagent-handle>\" ...`.",
			"Include the repair_batch_id in reviewer notes so rereview can close the consolidated repair batch.",
		},
		AfterRepair: []string{
			"Run `slipway review --json` to re-evaluate S3 convergence.",
			"If a reviewer explicitly records `new_change_required:intent_conflict`, open a new governed change instead of continuing this fix.",
		},
		Prohibited: []string{
			"Do not repair individual review findings before collecting the selected review batch findings.",
			"Do not fix review findings inline in the host context.",
			"Do not use `slipway repair` for review findings; repair is local integrity only.",
			"Do not hand-edit verification YAML.",
		},
	}
}

func writeFixText(w io.Writer, view fixView) error {
	writer := newFormatWriter(w)
	writer.Writef("Change: %s\n", view.Slug)
	writer.Writef("State: %s\n", view.CurrentState)
	writer.Writef("Repair batch: %s\n", view.Contract.RepairBatchID)
	writer.Writef("Collect all findings first: %t\n", view.Contract.CollectAllSelectedReviewFindingsFirst)
	writer.Writef("Fresh context required: %t\n", view.Contract.RequiresFreshContext)
	if len(view.SelectedReviewSkills) > 0 {
		writer.Writef("Selected reviewers: %s\n", strings.Join(view.SelectedReviewSkills, ", "))
	}
	if len(view.RepairTargets) == 0 {
		writer.Writef("Repair targets: none\n")
		writer.Writef("Run `slipway review --json` to refresh review convergence before fixing.\n")
		return writer.Err()
	}
	writer.Writef("Repair targets:\n")
	for _, target := range view.RepairTargets {
		subject := target.Reviewer
		if subject == "" {
			subject = target.Detail
		}
		if subject == "" {
			subject = target.Kind
		}
		writer.Writef("- %s: %s", subject, target.Kind)
		if target.Reason != "" {
			writer.Writef(" (%s)", target.Reason)
		}
		writer.Writef("\n")
		if target.EvidencePath != "" {
			writer.Writef("  evidence: %s\n", target.EvidencePath)
		}
		for _, blocker := range target.Blockers {
			for _, spec := range model.ReasonSpecs([]model.ReasonCode{blocker}) {
				writer.Writef("  blocker: %s\n", spec)
			}
		}
	}
	writer.Writef("Contract:\n")
	writer.Writef("- %s\n", view.Contract.FindingCollection)
	writer.Writef("- %s\n", view.Contract.Dispatch)
	writer.Writef("- %s\n", view.Contract.RepairBrief)
	writer.Writef("- record reference: %s\n", view.Contract.ContextReference)
	for _, step := range view.Contract.AfterRepair {
		writer.Writef("- %s\n", step)
	}
	return writer.Err()
}
