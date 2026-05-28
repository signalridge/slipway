package cmd

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type closeoutFreshnessStats struct {
	Missing []string `json:"missing,omitempty"`
	Stale   []string `json:"stale,omitempty"`
	Fresh   []string `json:"fresh,omitempty"`
}

type statsView struct {
	ExecutionMode         string                 `json:"execution_mode"`
	ActiveCount           int                    `json:"active_count"`
	MissingReviewEvidence []string               `json:"missing_review_evidence,omitempty"`
	StaleRunSummaries     []string               `json:"stale_run_summaries,omitempty"`
	IntegrityIssues       []string               `json:"integrity_issues,omitempty"`
	ArchiveCount          int                    `json:"archive_count"`
	CodebaseMap           state.CodebaseMapStats `json:"codebase_map"`
	CloseoutFreshness     closeoutFreshnessStats `json:"closeout_freshness"`
}

func makeStatsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:    "stats",
		Short:  desc("stats"),
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}

			view, err := buildStatsView(root, time.Now().UTC())
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeJSONResponse(cmd, view)
			}
			return writeStatsText(cmd.OutOrStdout(), view)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	return cmd
}

func buildStatsView(root string, now time.Time) (statsView, error) {
	repoStats, err := state.CollectRepoStats(root, now)
	if err != nil {
		return statsView{}, err
	}

	view := statsView{
		ExecutionMode: "diagnostics",
		ActiveCount:   len(repoStats.ActiveChanges),
		ArchiveCount:  repoStats.ArchiveCount,
		CodebaseMap:   repoStats.CodebaseMap,
	}
	for _, issue := range repoStats.ChangeLoadIssues {
		view.IntegrityIssues = append(view.IntegrityIssues, statsIntegrityIssue(issue.Slug, "change_state_load_failed", issue.Err))
	}

	for _, change := range repoStats.ActiveChanges {
		readiness, err := progression.EvaluateGovernanceReadiness(
			root,
			change,
			progression.GovernanceReadinessOptions{
				// Stats only requests the optional surfaces it actually summarizes.
				// It does not need artifact projection because it never renders an
				// artifact-centric view.
				IncludeReviewSurface: change.CurrentState == model.StateS3Review || change.CurrentState == model.StateS4Verify,
				IncludeShipSurface:   change.CurrentState == model.StateS4Verify,
			},
		)
		if err != nil {
			view.IntegrityIssues = append(view.IntegrityIssues, statsIntegrityIssueFromError(wrapGovernanceReadinessError("evaluate stats readiness", change.Slug, err)))
			continue
		}
		if statsExecutionSummaryStale(readiness) || statsExecutionSummaryMissing(change, readiness) {
			view.StaleRunSummaries = append(view.StaleRunSummaries, change.Slug)
		}

		if readiness.ReviewSurface != nil && hasMissingReviewEvidenceBlockers(readiness.ReviewSurface.SkillBlockers) {
			view.MissingReviewEvidence = append(view.MissingReviewEvidence, change.Slug)
		}
		presetPolicy, err := governance.ResolvePresetPolicy(root, change)
		if err != nil {
			return statsView{}, err
		}
		if !presetPolicy.CloseoutRefreshRequired || change.CurrentState != model.StateS4Verify {
			continue
		}

		shipAuthority := readiness.ShipSurface
		if shipAuthority == nil {
			continue
		}
		switch {
		case hasRequiredSkillReason(shipAuthority.VerifySkillBlockers, "required_skill_missing", progression.SkillFinalCloseout):
			view.CloseoutFreshness.Missing = append(view.CloseoutFreshness.Missing, change.Slug)
		// CloseoutFreshness is intentionally broader than StaleRunSummaries: it
		// reflects ship-readiness for refreshed closeout evidence, not only the
		// execution-summary authority.
		case statsExecutionSummaryStale(readiness) ||
			hasAnyRequiredSkillBlocker(shipAuthority.VerifySkillBlockers, progression.SkillGoalVerification, progression.SkillFinalCloseout):
			view.CloseoutFreshness.Stale = append(view.CloseoutFreshness.Stale, change.Slug)
		default:
			view.CloseoutFreshness.Fresh = append(view.CloseoutFreshness.Fresh, change.Slug)
		}
	}

	sortStatsStrings(&view.MissingReviewEvidence)
	sortStatsStrings(&view.StaleRunSummaries)
	sortStatsStrings(&view.IntegrityIssues)
	sortStatsStrings(&view.CloseoutFreshness.Missing)
	sortStatsStrings(&view.CloseoutFreshness.Stale)
	sortStatsStrings(&view.CloseoutFreshness.Fresh)
	return view, nil
}

func statsIntegrityIssue(slug, code string, err error) string {
	if err == nil {
		return fmt.Sprintf("%s:%s", slug, code)
	}
	return fmt.Sprintf("%s:%s:%v", slug, code, err)
}

func statsIntegrityIssueFromError(err error) string {
	if err == nil {
		return ""
	}
	if cliErr := asCLIError(err); cliErr != nil {
		slug := strings.TrimSpace(cliErr.Slug)
		if slug == "" {
			slug = "repo"
		}
		issue := fmt.Sprintf("%s:%s:%s", slug, cliErr.ErrorCode, cliErr.Message)
		if cliErr.Remediation != "" {
			issue += " | remediation: " + cliErr.Remediation
		}
		return issue
	}
	return statsIntegrityIssue("repo", governanceReadinessErrorCode(err), err)
}

func statsExecutionSummaryStale(readiness progression.GovernanceReadiness) bool {
	return readiness.EvidenceFreshness == "stale"
}

func statsExecutionSummaryMissing(change model.Change, readiness progression.GovernanceReadiness) bool {
	return requiresFrozenRunSummary(change.CurrentState) && !state.ExecutionSummaryReady(readiness.ExecutionSummary)
}

func requiresFrozenRunSummary(currentState model.WorkflowState) bool {
	switch currentState {
	case model.StateS3Review, model.StateS4Verify:
		return true
	default:
		return false
	}
}

func hasMissingReviewEvidenceBlockers(blockers []model.ReasonCode) bool {
	return hasRequiredSkillReason(blockers, "required_skill_missing", progression.SkillSpecComplianceReview) ||
		hasRequiredSkillReason(blockers, "required_skill_missing", progression.SkillCodeQualityReview)
}

func hasRequiredSkillReason(blockers []model.ReasonCode, code, skillName string) bool {
	return hasReason(blockers, code, skillName)
}

func hasAnyRequiredSkillBlocker(blockers []model.ReasonCode, skillNames ...string) bool {
	for _, skillName := range skillNames {
		if hasReason(blockers, "required_skill_missing", skillName) ||
			hasReason(blockers, "required_skill_not_ready", skillName) ||
			hasReason(blockers, "required_skill_not_passed", skillName) ||
			hasReason(blockers, "required_skill_blockers_present", skillName) {
			return true
		}
	}
	return false
}

func hasReason(reasons []model.ReasonCode, code, detail string) bool {
	for _, reason := range reasons {
		if reason.Code != code {
			continue
		}
		if detail == "" || strings.Contains(reason.Detail, detail) {
			return true
		}
	}
	return false
}

func sortStatsStrings(values *[]string) {
	if len(*values) == 0 {
		*values = nil
		return
	}
	slices.Sort(*values)
}

func writeStatsText(w io.Writer, view statsView) error {
	writer := newFormatWriter(w)
	writer.Writef("Mode: %s\n", view.ExecutionMode)
	writer.Writef("Active Changes: %d\n", view.ActiveCount)
	writer.Writef("Archive Count: %d\n", view.ArchiveCount)
	writer.Writef("Codebase Map Freshness: %s\n", view.CodebaseMap.Freshness)
	if len(view.MissingReviewEvidence) > 0 {
		writer.Writef("Missing Review Evidence: %s\n", strings.Join(view.MissingReviewEvidence, ", "))
	}
	if len(view.StaleRunSummaries) > 0 {
		writer.Writef("Stale Run Summaries: %s\n", strings.Join(view.StaleRunSummaries, ", "))
	}
	if len(view.IntegrityIssues) > 0 {
		writer.Writef("Integrity Issues: %s\n", strings.Join(view.IntegrityIssues, ", "))
	}
	if len(view.CloseoutFreshness.Missing) > 0 || len(view.CloseoutFreshness.Stale) > 0 {
		writer.Writef("Closeout Freshness:\n")
		if len(view.CloseoutFreshness.Missing) > 0 {
			writer.Writef("  missing: %s\n", strings.Join(view.CloseoutFreshness.Missing, ", "))
		}
		if len(view.CloseoutFreshness.Stale) > 0 {
			writer.Writef("  stale:   %s\n", strings.Join(view.CloseoutFreshness.Stale, ", "))
		}
	}
	return writer.Err()
}
