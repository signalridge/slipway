package cmd

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
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
	RepairBatchID                         string   `json:"repair_batch_id"`
	CollectAllSelectedReviewFindingsFirst bool     `json:"collect_all_selected_review_findings_first"`
	RequiresFreshContext                  bool     `json:"requires_fresh_context"`
	FindingCollection                     string   `json:"finding_collection"`
	Dispatch                              string   `json:"dispatch"`
	RepairBrief                           string   `json:"repair_brief"`
	ContextReference                      string   `json:"context_reference"`
	RecordEvidence                        []string `json:"record_evidence"`
	AfterRepair                           []string `json:"after_repair"`
	Prohibited                            []string `json:"prohibited"`
}

func makeFixCmd() *cobra.Command {
	var changeSlug string
	var jsonOutput bool
	var reviewer string
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
			view, err := buildFixViewForSlug(root, ref.Slug, reviewer)
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
	return cmd
}

func buildFixViewForSlug(root, slug, reviewerFilter string) (fixView, error) {
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

	selected := selectedReviewSkillsFromReadiness(readiness)
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

	profile := buildChangeProfileView(change)
	return fixView{
		Slug:                 slug,
		ExecutionMode:        governedExecutionMode,
		QualityMode:          profile.QualityMode,
		CurrentState:         string(change.CurrentState),
		SelectedReviewSkills: selected,
		RepairTargets:        targets,
		Contract:             reviewFixContract(slug, selected),
		Blockers:             model.NormalizeReasonCodes(readiness.Blockers),
	}, nil
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

func reviewFixContract(slug string, selected []string) fixRepairContract {
	batchID := "s3-review-repair:" + strings.TrimSpace(slug)
	return fixRepairContract{
		RepairBatchID:                         batchID,
		CollectAllSelectedReviewFindingsFirst: true,
		RequiresFreshContext:                  true,
		FindingCollection:                     "Collect all selected S3 reviewer findings first, then consolidate by root cause before dispatching repair.",
		Dispatch:                              "Spawn a fresh-context repair subagent with the consolidated repair brief; pass paths and blocker facts, not the host conversation.",
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
