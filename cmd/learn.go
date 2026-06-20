package cmd

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type learnView struct {
	ExecutionMode   string          `json:"execution_mode"`
	Preview         bool            `json:"preview"`
	AutoApply       bool            `json:"auto_apply"`
	AnalyzedChanges int             `json:"analyzed_changes"`
	Signals         learnSignals    `json:"signals"`
	Proposals       []learnProposal `json:"proposals,omitempty"`
	IntegrityIssues []string        `json:"integrity_issues,omitempty"`
	GeneratedAt     time.Time       `json:"generated_at"`
}

type learnSignals struct {
	ActiveChanges                  int            `json:"active_changes"`
	ArchivedChanges                int            `json:"archived_changes"`
	LifecycleEventCount            int            `json:"lifecycle_event_count"`
	MissingLifecycleLogs           []string       `json:"missing_lifecycle_logs,omitempty"`
	GateBlockedByReason            map[string]int `json:"gate_blocked_by_reason,omitempty"`
	RequiredSkillMissing           map[string]int `json:"required_skill_missing,omitempty"`
	AutoPassByState                map[string]int `json:"auto_pass_by_state,omitempty"`
	GuardrailDomainFrequency       map[string]int `json:"guardrail_domain_frequency,omitempty"`
	ControlBlockerFrequency        map[string]int `json:"control_blocker_frequency,omitempty"`
	EvidenceMissingFrequency       map[string]int `json:"evidence_missing_frequency,omitempty"`
	PlanAuditStalled               int            `json:"plan_audit_stalled"`
	PlanAuditBudgetExhausted       int            `json:"plan_audit_budget_exhausted"`
	PlanAuditIterations            int            `json:"plan_audit_iterations"`
	PlanAuditIterationDistribution map[string]int `json:"plan_audit_iteration_distribution,omitempty"`
	PlanAuditAffectedChanges       []string       `json:"plan_audit_affected_changes,omitempty"`
	ClarificationBlockRate         float64        `json:"clarification_block_rate"`
	GovernanceActionBlockedChanges []string       `json:"governance_action_blocked_changes,omitempty"`
	IntakeMissingChanges           []string       `json:"intake_missing_changes,omitempty"`
	PlanAuditStallRate             float64        `json:"plan_audit_stall_rate"`
	ReviewIntentDrift              int            `json:"review_intent_drift_failures"`
	ReviewIntentDriftFailureRate   float64        `json:"review_intent_drift_failure_rate"`
	ReviewIntentDriftChanges       []string       `json:"review_intent_drift_changes,omitempty"`
	CheckpointOpened               int            `json:"checkpoint_opened"`
	CheckpointResolved             int            `json:"checkpoint_resolved"`
	CheckpointResolvedManual       int            `json:"checkpoint_resolved_manual"`
	CheckpointResolvedAuto         int            `json:"checkpoint_resolved_auto"`
	CheckpointResolutionRate       float64        `json:"checkpoint_resolution_rate"`
	InterruptionCount              int            `json:"interruption_count"`
	InterruptionResumeSuccesses    int            `json:"interruption_resume_successes"`
	InterruptionResumeSuccessRate  float64        `json:"interruption_resume_success_rate"`
	InterruptedChanges             []string       `json:"interrupted_changes,omitempty"`
}

type learnProposal struct {
	ID                    string             `json:"id"`
	ProposalID            string             `json:"proposal_id"`
	Kind                  string             `json:"kind"`
	Summary               string             `json:"summary"`
	Rationale             string             `json:"rationale"`
	Evidence              []string           `json:"evidence,omitempty"`
	Changes               []string           `json:"changes,omitempty"`
	Metrics               map[string]float64 `json:"metrics,omitempty"`
	Hypothesis            string             `json:"hypothesis"`
	RecommendedAction     string             `json:"recommended_action"`
	SuggestedAction       string             `json:"suggested_action"`
	Risk                  string             `json:"risk"`
	Mode                  string             `json:"mode"`
	RequiresHumanApproval bool               `json:"requires_human_approval"`
}

func makeLearnCmd() *cobra.Command {
	var jsonOutput bool
	var preview bool

	cmd := &cobra.Command{
		Use:   "learn",
		Short: desc("learn"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !preview {
				return newInvalidUsageError(
					"learn_apply_unsupported",
					"learn only supports read-only preview proposals",
					"Run `slipway learn --preview` and apply accepted governance changes manually.",
					nil,
				)
			}
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			view, err := buildLearnView(root, time.Now().UTC())
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeJSONResponse(cmd, view)
			}
			return writeLearnText(cmd.OutOrStdout(), view)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().BoolVar(&preview, "preview", true, "Generate read-only governance learning proposals")
	return cmd
}

func buildLearnView(root string, now time.Time) (learnView, error) {
	repoStats, err := state.CollectRepoStats(root, now)
	if err != nil {
		return learnView{}, err
	}

	view := learnView{
		ExecutionMode: "diagnostics",
		Preview:       true,
		AutoApply:     false,
		GeneratedAt:   now,
		Signals: learnSignals{
			ActiveChanges:                  len(repoStats.ActiveChanges),
			GateBlockedByReason:            map[string]int{},
			RequiredSkillMissing:           map[string]int{},
			AutoPassByState:                map[string]int{},
			GuardrailDomainFrequency:       map[string]int{},
			ControlBlockerFrequency:        map[string]int{},
			EvidenceMissingFrequency:       map[string]int{},
			PlanAuditIterationDistribution: map[string]int{},
		},
	}
	for _, issue := range repoStats.ChangeLoadIssues {
		view.IntegrityIssues = append(view.IntegrityIssues, statsIntegrityIssue(issue.Slug, "change_state_load_failed", issue.Err))
	}

	changes := append([]model.Change(nil), repoStats.ActiveChanges...)
	archivedSlugs, err := state.ListArchivedChangeSlugs(root)
	if err != nil {
		return learnView{}, err
	}
	view.Signals.ArchivedChanges = len(archivedSlugs)
	for _, slug := range archivedSlugs {
		change, loadErr := state.LoadArchivedChange(root, slug)
		if loadErr != nil {
			view.IntegrityIssues = append(view.IntegrityIssues, statsIntegrityIssue(slug, "archived_change_load_failed", loadErr))
			continue
		}
		changes = append(changes, change)
	}

	for _, change := range changes {
		analyzeChangeForLearning(root, change, &view)
	}
	view.AnalyzedChanges = len(changes)
	computeLearnDerivedSignals(&view)
	view.Proposals = buildLearnProposals(view.Signals, now)
	finalizeLearnView(&view)
	return view, nil
}

func analyzeChangeForLearning(root string, change model.Change, view *learnView) {
	view.Signals.PlanAuditIterations += change.PlanAuditIterations
	if change.PlanAuditIterations > 0 {
		view.Signals.PlanAuditIterationDistribution[strconv.Itoa(change.PlanAuditIterations)]++
	}
	view.Signals.ReviewIntentDrift += change.ReviewIntentDriftFailures
	if change.ReviewIntentDriftFailures > 0 {
		view.Signals.ReviewIntentDriftChanges = appendUniqueString(view.Signals.ReviewIntentDriftChanges, change.Slug)
	}
	if domain := strings.TrimSpace(change.GuardrailDomain); domain != "" {
		view.Signals.GuardrailDomainFrequency[domain]++
	}
	if !change.InterruptedExecutionAt.IsZero() {
		view.Signals.InterruptedChanges = append(view.Signals.InterruptedChanges, change.Slug)
	}

	events, err := state.ReadLifecycleEvents(root, change)
	if err != nil {
		view.IntegrityIssues = append(view.IntegrityIssues, statsIntegrityIssue(change.Slug, "lifecycle_event_load_failed", err))
		return
	}
	if len(events) == 0 {
		if change.Status == model.ChangeStatusActive {
			view.Signals.MissingLifecycleLogs = append(view.Signals.MissingLifecycleLogs, change.Slug)
		}
		return
	}
	view.Signals.LifecycleEventCount += len(events)
	interrupted := false
	resumed := false
	for _, event := range events {
		if event.EventType == "auto_pass.applied" {
			stateKey := strings.TrimSpace(string(event.BeforeState))
			if stateKey == "" {
				stateKey = strings.TrimSpace(string(event.AfterState))
			}
			if stateKey != "" {
				view.Signals.AutoPassByState[stateKey]++
			}
		}
		switch event.EventType {
		case "checkpoint.opened":
			view.Signals.CheckpointOpened++
		case "checkpoint.resolved":
			view.Signals.CheckpointResolved++
			if lifecycleEventHasSideEffect(event, autoCheckpointAcknowledgedSideEffect) {
				view.Signals.CheckpointResolvedAuto++
			} else {
				view.Signals.CheckpointResolvedManual++
			}
		case "abort.marked":
			if event.Result == "interrupted" || event.Action == "execution_interrupted" {
				interrupted = true
			}
		case "resume.succeeded":
			resumed = true
		}
		if controlID := strings.TrimSpace(event.ControlID); controlID != "" {
			view.Signals.ControlBlockerFrequency[controlID]++
		}
		for _, blocker := range event.Blockers {
			code := strings.TrimSpace(blocker.Code)
			if code == "" {
				continue
			}
			view.Signals.GateBlockedByReason[code]++
			if code == "required_skill_missing" {
				if skillName := strings.TrimSpace(blocker.Detail); skillName != "" {
					view.Signals.RequiredSkillMissing[skillName]++
					if skillName == "intake-clarification" {
						view.Signals.IntakeMissingChanges = appendUniqueString(view.Signals.IntakeMissingChanges, change.Slug)
					}
				}
			}
			if strings.Contains(code, "missing") {
				view.Signals.EvidenceMissingFrequency[code]++
			}
			if code == "governance_action_required" {
				view.Signals.GovernanceActionBlockedChanges = appendUniqueString(view.Signals.GovernanceActionBlockedChanges, change.Slug)
				if controlID := governanceActionControlFromLearnBlocker(blocker); controlID != "" {
					view.Signals.ControlBlockerFrequency[controlID]++
				}
			}
			switch code {
			case "plan_audit_stalled":
				view.Signals.PlanAuditStalled++
				view.Signals.PlanAuditAffectedChanges = appendUniqueString(view.Signals.PlanAuditAffectedChanges, change.Slug)
			case "plan_audit_budget_exhausted":
				view.Signals.PlanAuditBudgetExhausted++
				view.Signals.PlanAuditAffectedChanges = appendUniqueString(view.Signals.PlanAuditAffectedChanges, change.Slug)
			}
		}
	}
	if interrupted {
		view.Signals.InterruptionCount++
		if resumed {
			view.Signals.InterruptionResumeSuccesses++
		}
	}
}

func buildLearnProposals(signals learnSignals, now time.Time) []learnProposal {
	var proposals []learnProposal
	if signals.PlanAuditStalled > 0 || signals.PlanAuditBudgetExhausted > 0 {
		proposals = append(proposals, newLearnProposal(
			now,
			"plan-audit-loop-review",
			"template_adjustment",
			"Review plan-audit prompt and evidence contract for repeated non-improving feedback.",
			"Plan audit should converge quickly; repeated stall or budget exhaustion indicates either overly broad gate criteria or weak planner output.",
			[]string{
				fmt.Sprintf("plan_audit_stalled=%d", signals.PlanAuditStalled),
				fmt.Sprintf("plan_audit_budget_exhausted=%d", signals.PlanAuditBudgetExhausted),
				fmt.Sprintf("plan_audit_iterations=%d", signals.PlanAuditIterations),
			},
			signals.PlanAuditAffectedChanges,
			map[string]float64{
				"plan_audit_stall_rate": signals.PlanAuditStallRate,
			},
			"Plan-audit feedback is not producing measurable progress before the iteration budget is spent.",
			"Tighten plan-audit findings into stable, actionable issue IDs and add examples for acceptable remediation evidence.",
			"Changing audit templates can weaken gate clarity if accepted without sampling affected changes.",
		))
	}
	if clarificationBlocks := signals.GateBlockedByReason["governance_action_required"]; clarificationBlocks > 0 {
		proposals = append(proposals, newLearnProposal(
			now,
			"governance-action-friction-review",
			"preset_review",
			"Inspect recurring governance-action blockers before changing thresholds.",
			"Governance-action blockers are expected for sensitive work, but repetition can reveal unclear intake prompts or missing caller context.",
			[]string{fmt.Sprintf("governance_action_required=%d", clarificationBlocks)},
			signals.GovernanceActionBlockedChanges,
			map[string]float64{"clarification_block_rate": signals.ClarificationBlockRate},
			"Recurring governance-action blockers may indicate missing project context rather than an overly strict control.",
			"Sample blocked changes and decide whether docs, intake classification examples, or project context should be updated.",
			"Do not relax fail-closed guardrail-domain controls based only on aggregate friction.",
		))
	}
	if intakeMissing := signals.RequiredSkillMissing["intake-clarification"]; intakeMissing > 0 {
		proposals = append(proposals, newLearnProposal(
			now,
			"intake-template-friction-review",
			"template_adjustment",
			"Review intake clarification templates for repeated missing clarification evidence.",
			"Repeated intake-clarification stops usually mean the agent lacks enough upfront classification or project context to proceed deterministically.",
			[]string{fmt.Sprintf("required_skill_missing:intake-clarification=%d", intakeMissing)},
			signals.IntakeMissingChanges,
			map[string]float64{"clarification_block_rate": signals.ClarificationBlockRate},
			"Intake is not capturing enough bounded context for deterministic planning handoff.",
			"Add concrete classification examples and require unresolved assumptions to be captured before planning.",
			"Template changes should improve evidence quality without encouraging agents to guess missing risk classification.",
		))
	}
	if signals.ReviewIntentDrift > 0 {
		proposals = append(proposals, newLearnProposal(
			now,
			"intent-drift-retrospective",
			"docs_clarification",
			"Retrospect review intent drift before relaxing review gates.",
			"Intent drift failures usually point to artifact or task scope mismatch rather than a gate that should be disabled.",
			[]string{fmt.Sprintf("review_intent_drift_failures=%d", signals.ReviewIntentDrift)},
			signals.ReviewIntentDriftChanges,
			map[string]float64{"review_intent_drift_failure_rate": signals.ReviewIntentDriftFailureRate},
			"Review failures are detecting real drift between intended scope and delivered evidence.",
			"Compare intent, decision, task, and review evidence on affected changes and update templates only after a human accepts the pattern.",
			"Relaxing review gates here would hide scope drift instead of fixing artifact quality.",
		))
	}
	if len(signals.MissingLifecycleLogs) > 0 {
		proposals = append(proposals, newLearnProposal(
			now,
			"legacy-event-log-gap",
			"docs_clarification",
			"Treat changes without lifecycle logs as legacy observations.",
			"Lifecycle events were added after existing changes; missing logs should not be backfilled with invented state transitions.",
			[]string{
				fmt.Sprintf("missing_lifecycle_logs=%d", len(signals.MissingLifecycleLogs)),
			},
			signals.MissingLifecycleLogs,
			map[string]float64{},
			"Legacy snapshots are useful compatibility data but should not be converted into fabricated audit history.",
			"Use change.yaml snapshots for compatibility, and rely on lifecycle.jsonl for new learning signals going forward.",
			"Backfilling synthetic transitions would make audit history less trustworthy.",
		))
	}
	return proposals
}

func finalizeLearnView(view *learnView) {
	sortStatsStrings(&view.IntegrityIssues)
	sortStatsStrings(&view.Signals.MissingLifecycleLogs)
	sortStatsStrings(&view.Signals.InterruptedChanges)
	sortStatsStrings(&view.Signals.PlanAuditAffectedChanges)
	sortStatsStrings(&view.Signals.GovernanceActionBlockedChanges)
	sortStatsStrings(&view.Signals.IntakeMissingChanges)
	sortStatsStrings(&view.Signals.ReviewIntentDriftChanges)
	if len(view.Signals.GateBlockedByReason) == 0 {
		view.Signals.GateBlockedByReason = nil
	}
	if len(view.Signals.RequiredSkillMissing) == 0 {
		view.Signals.RequiredSkillMissing = nil
	}
	if len(view.Signals.AutoPassByState) == 0 {
		view.Signals.AutoPassByState = nil
	}
	if len(view.Signals.GuardrailDomainFrequency) == 0 {
		view.Signals.GuardrailDomainFrequency = nil
	}
	if len(view.Signals.ControlBlockerFrequency) == 0 {
		view.Signals.ControlBlockerFrequency = nil
	}
	if len(view.Signals.EvidenceMissingFrequency) == 0 {
		view.Signals.EvidenceMissingFrequency = nil
	}
	if len(view.Signals.PlanAuditIterationDistribution) == 0 {
		view.Signals.PlanAuditIterationDistribution = nil
	}
	slices.SortFunc(view.Proposals, func(a, b learnProposal) int {
		return strings.Compare(a.ID, b.ID)
	})
}

func computeLearnDerivedSignals(view *learnView) {
	analyzed := view.AnalyzedChanges
	view.Signals.ClarificationBlockRate = learnRate(view.Signals.RequiredSkillMissing["intake-clarification"], analyzed)
	view.Signals.PlanAuditStallRate = learnRate(view.Signals.PlanAuditStalled, analyzed)
	view.Signals.ReviewIntentDriftFailureRate = learnRate(view.Signals.ReviewIntentDrift, analyzed)
	view.Signals.CheckpointResolutionRate = learnRate(view.Signals.CheckpointResolvedManual, view.Signals.CheckpointOpened)
	view.Signals.InterruptionResumeSuccessRate = learnRate(view.Signals.InterruptionResumeSuccesses, view.Signals.InterruptionCount)
}

func newLearnProposal(
	now time.Time,
	id, kind, summary, rationale string,
	evidence []string,
	changes []string,
	metrics map[string]float64,
	hypothesis, recommendedAction, risk string,
) learnProposal {
	if len(metrics) == 0 {
		metrics = nil
	}
	changes = append([]string(nil), changes...)
	sortStatsStrings(&changes)
	return learnProposal{
		ID:                    id,
		ProposalID:            "learn-" + now.UTC().Format("2006-01-02") + "-" + id,
		Kind:                  kind,
		Summary:               summary,
		Rationale:             rationale,
		Evidence:              evidence,
		Changes:               changes,
		Metrics:               metrics,
		Hypothesis:            hypothesis,
		RecommendedAction:     recommendedAction,
		SuggestedAction:       recommendedAction,
		Risk:                  risk,
		Mode:                  "manual_review",
		RequiresHumanApproval: true,
	}
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func learnRate(numerator, denominator int) float64 {
	if denominator <= 0 || numerator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func lifecycleEventHasSideEffect(event state.LifecycleEvent, kind string) bool {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return false
	}
	for _, effect := range event.SideEffects {
		if strings.TrimSpace(effect.Kind) == kind {
			return true
		}
	}
	return false
}

func governanceActionControlFromLearnBlocker(blocker model.ReasonCode) string {
	detail := strings.TrimSpace(blocker.Detail)
	controlID, _, _ := strings.Cut(detail, ":")
	return strings.TrimSpace(controlID)
}

func writeLearnText(w io.Writer, view learnView) error {
	writer := newFormatWriter(w)
	writer.Writef("Mode: %s\n", view.ExecutionMode)
	writer.Writef("Preview: %t\n", view.Preview)
	writer.Writef("Auto Apply: %t\n", view.AutoApply)
	writer.Writef("Analyzed Changes: %d\n", view.AnalyzedChanges)
	writer.Writef("Lifecycle Events: %d\n", view.Signals.LifecycleEventCount)
	if len(view.Proposals) == 0 {
		writer.Writef("Proposals: none\n")
	} else {
		writer.Writef("Proposals:\n")
		for _, proposal := range view.Proposals {
			writer.Writef("  - %s: %s\n", proposal.ID, proposal.Summary)
			writer.Writef("    Next: %s\n", proposal.SuggestedAction)
		}
	}
	if len(view.IntegrityIssues) > 0 {
		writer.Writef("Integrity Issues: %s\n", strings.Join(view.IntegrityIssues, ", "))
	}
	return writer.Err()
}
