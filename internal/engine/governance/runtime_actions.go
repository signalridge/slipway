package governance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

const (
	skillSpecComplianceReview = "spec-compliance-review"
	skillIndependentReview    = "independent-review"
	skillSecurityReview       = "security-review"
	skillIntakeClarification  = "intake-clarification"

	decisionRollbackTemplateText  = "Describe rollout sequencing, safeguards, and how the change would be rolled back if verification fails."
	assuranceRollbackTemplateText = "Summarize rollback constraints, prerequisites, and verification status when rollback planning is required."
)

// ResolveRuntimeRequiredActions resolves the runtime governance action queue.
// researchEvidenceStale is supplied by the progression caller (digest-freshness
// state lives in progression, which imports governance and never the reverse) and
// reports whether a present research-orchestration record is certified stale, so
// the research action reflects effective freshness rather than mere structural
// presence (#377).
func ResolveRuntimeRequiredActions(root string, change model.Change, snap model.GovernanceSnapshot, researchEvidenceStale bool) []RequiredAction {
	verifications := loadRuntimeVerificationState(root, change)
	executionSummaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		verifications.diagnostics = append(verifications.diagnostics, fmt.Sprintf("runtime_execution_summary_invalid:%v", err))
		// Fail closed for execution-summary-bound checks by treating invalid
		// summary state the same as unavailable summary state. This keeps
		// review-side runtime evidence unsatisfied until the summary is repaired,
		// without silently relying on an implicit zero-value struct.
	}
	// Intake clarification is a pre-execution scope-confirmation proof. It must
	// remain satisfiable before the first execution summary exists, so it is not
	// bound to run_summary_version.
	intakeConfirmed, _, scopeIssues := verifications.skillSatisfied(
		skillIntakeClarification,
		executionSummaryCtx.LatestRunVersion,
		false,
		"intake-clarification confirms the scope for research",
	)
	// Review-side skills validate execution outputs. They are intentionally tied
	// to the latest execution summary so stale review evidence cannot satisfy the
	// current governed run.
	domainReviewDone, domainSatisfiedBy, domainIssues := verifications.skillSatisfied(
		skillSpecComplianceReview,
		executionSummaryCtx.LatestRunVersion,
		true,
		"spec-compliance-review provides the domain-aware review evidence for domain-review",
	)
	independentReviewDone, independentSatisfiedBy, independentIssues := verifications.skillSatisfied(
		skillIndependentReview,
		executionSummaryCtx.LatestRunVersion,
		true,
		"independent-review provides the selected independent review evidence for independent-review",
	)
	securityReviewDone, securitySatisfiedBy, securityIssues := verifications.skillSatisfied(
		skillSecurityReview,
		executionSummaryCtx.LatestRunVersion,
		true,
		"security-review provides the selected security review evidence for security-review",
	)
	if len(executionSummaryCtx.Issues) > 0 {
		blockingExecutionIssues := runtimeBlockingExecutionSummaryIssues(change, executionSummaryCtx.Issues)
		if len(blockingExecutionIssues) > 0 {
			domainReviewDone = false
			domainSatisfiedBy = nil
			domainIssues = stringutil.UniqueSorted(append(domainIssues, blockingExecutionIssues...))
			independentReviewDone = false
			independentSatisfiedBy = nil
			independentIssues = stringutil.UniqueSorted(append(independentIssues, blockingExecutionIssues...))
			securityReviewDone = false
			securitySatisfiedBy = nil
			securityIssues = stringutil.UniqueSorted(append(securityIssues, blockingExecutionIssues...))
		}
	}

	worktreeValidation, err := state.ValidateChangeWorktree(root, change)
	worktreeIssues := []string{}
	worktreeSatisfied := false
	if err != nil {
		worktreeIssues = []string{"worktree_validation_error:" + err.Error()}
	} else {
		worktreeIssues = append(worktreeIssues, model.ReasonMessages(worktreeValidation.Blockers)...)
		worktreeSatisfied = len(worktreeValidation.Blockers) == 0 && strings.TrimSpace(worktreeValidation.NormalizedPath) != ""
	}

	researchOK := researchStructureSatisfied(root, change)

	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:               snap.ActiveControls,
		CurrentState:                 change.CurrentState,
		HasBlockingOpenQuestions:     snap.Traceability.HasBlockingIntentGap(),
		IntentExists:                 artifactExistsInBundle(root, change, "intent.md"),
		ScopeConfirmed:               changeScopeConfirmed(change, intakeConfirmed),
		ResearchStructureOK:          researchOK,
		ResearchEvidenceStale:        researchEvidenceStale,
		DomainReviewDone:             domainReviewDone,
		DomainReviewSatisfiedBy:      domainSatisfiedBy,
		IndependentReviewDone:        independentReviewDone,
		IndependentReviewSatisfiedBy: independentSatisfiedBy,
		SecurityReviewDone:           securityReviewDone,
		SecurityReviewSatisfiedBy:    securitySatisfiedBy,
		WorktreePreflightDone:        worktreeSatisfied,
		RollbackSectionExists:        hasRollbackDocumentation(root, change),
	})
	for idx := range actions {
		if actions[idx].Satisfied {
			continue
		}
		issues := diagnosticsForUnsatisfiedAction(
			actions[idx].ControlID,
			verifications,
			scopeIssues,
			domainIssues,
			independentIssues,
			securityIssues,
			worktreeIssues,
		)
		if len(issues) == 0 {
			continue
		}
		actions[idx].Description = fmt.Sprintf("%s [diagnostics: %s]", actions[idx].Description, strings.Join(issues, ", "))
	}
	return actions
}

func runtimeBlockingExecutionSummaryIssues(change model.Change, issues []string) []string {
	if len(issues) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(issues))
	for _, issue := range issues {
		reasons := model.ReasonCodesFromSpecs([]string{issue})
		if len(reasons) == 0 {
			filtered = append(filtered, issue)
			continue
		}
		if runtimeIssueAbsorbedByReview(change, reasons[0].Code) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return stringutil.UniqueSorted(filtered)
}

func runtimeIssueAbsorbedByReview(change model.Change, code string) bool {
	if change.CurrentState != model.StateS3Review {
		return false
	}
	switch strings.TrimSpace(code) {
	case "tasks_plan_changed_since_task_evidence":
		return true
	default:
		return false
	}
}

func RequiredActionBlockers(change model.Change, actions []RequiredAction) []string {
	blocking := UnsatisfiedBlockingActions(actions)
	blockers := make([]string, 0, len(blocking))
	for _, action := range blocking {
		if !actionBlocksCurrentState(change, action) {
			continue
		}
		blockers = append(blockers, fmt.Sprintf("governance_action_required:%s: %s", action.ControlID, action.Description))
	}
	return blockers
}

func actionBlocksCurrentState(change model.Change, action RequiredAction) bool {
	if action.ControlID == model.ControlResearch && change.CurrentState == model.StateS1Plan {
		// Research is actionable at S1_PLAN, but hard-blocking here deadlocks both
		// discovery and non-discovery paths before research is possible.
		return false
	}

	switch action.Scope {
	case model.ControlScopeDiscovery:
		// S0_INTAKE handles clarification; discovery scope blocks from S1_PLAN onward.
		if change.CurrentState == model.StateS0Intake {
			return false
		}
		if change.CurrentState == model.StateS1Plan {
			return !change.NeedsDiscovery
		}
		return change.CurrentState == model.StateS2Implement ||
			change.CurrentState == model.StateS3Review
	case model.ControlScopeExecution:
		// Execution-scope controls (e.g. worktree-isolation) only block from
		// S2_IMPLEMENT onward. S0_INTAKE and S1_PLAN are pre-execution phases;
		// the worktree gate lives at S2_IMPLEMENT/preflight per design.
		return change.CurrentState == model.StateS2Implement ||
			change.CurrentState == model.StateS3Review
	case model.ControlScopeReview:
		return change.CurrentState == model.StateS3Review
	case model.ControlScopeRelease:
		return change.CurrentState == model.StateS3Review
	default:
		return false
	}
}

func changeScopeConfirmed(change model.Change, intakeConfirmed bool) bool {
	switch change.CurrentState {
	case model.StateS0Intake:
		// Scope confirmation happens during S0_INTAKE/confirm.
		return intakeConfirmed
	case model.StateS1Plan:
		// If we've reached S1_PLAN, intake was completed.
		return true
	default:
		return true
	}
}

func researchStructureSatisfied(root string, change model.Change) bool {
	if !change.NeedsDiscovery {
		return true // non-discovery changes have no research.md obligation
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(paths.GovernedBundleDir, "research.md"))
	if err != nil {
		return false // research.md not yet created
	}
	return len(artifact.ResearchStructureBlockers(string(data))) == 0
}

func artifactExistsInBundle(root string, change model.Change, artifactName string) bool {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(paths.GovernedBundleDir, artifactName))
	return err == nil
}

type runtimeVerificationState struct {
	bySkill      map[string]model.VerificationRecord
	evidenceRefs map[string]string
	diagnostics  []string
}

func loadRuntimeVerificationState(root string, change model.Change) runtimeVerificationState {
	verifications, err := state.ListVerificationsForChange(root, change)
	st := runtimeVerificationState{
		bySkill:      verifications,
		evidenceRefs: verificationEvidenceRefs(root, change, verifications),
	}
	if err != nil {
		st.bySkill = map[string]model.VerificationRecord{}
		st.evidenceRefs = map[string]string{}
		st.diagnostics = []string{"runtime_verification_load_failed"}
	}
	return st
}

func verificationEvidenceRefs(root string, change model.Change, verifications map[string]model.VerificationRecord) map[string]string {
	if len(verifications) == 0 {
		return nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return nil
	}
	refs := make(map[string]string, len(verifications))
	for skillName := range verifications {
		refs[skillName] = state.DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "verification", skillName+".yaml"))
	}
	return refs
}

func (s runtimeVerificationState) skillSatisfied(skillName string, latestRunVersion int, requireRunSummary bool, reason string) (bool, []SatisfiedBy, []string) {
	rec, ok := s.bySkill[skillName]
	if !ok {
		return false, nil, nil
	}
	if !rec.IsPassing() {
		return false, nil, []string{"runtime_verification_not_passed:" + skillName}
	}
	if requireRunSummary && latestRunVersion < 1 {
		return false, nil, []string{"runtime_verification_not_ready:" + skillName + ":run_summary_missing"}
	}
	if latestRunVersion > 0 && rec.RunVersion != latestRunVersion {
		return false, nil, []string{fmt.Sprintf("runtime_verification_not_ready:%s:run_version_mismatch(got=%d,want=%d)", skillName, rec.RunVersion, latestRunVersion)}
	}
	return true, []SatisfiedBy{{
		Kind:        "skill_evidence",
		Name:        skillName,
		EvidenceRef: s.evidenceRefs[skillName],
		Reason:      reason,
	}}, nil
}

func hasRollbackDocumentation(root string, change model.Change) bool {
	return artifactSectionHasSubstantiveContent(
		root,
		change,
		"decision.md",
		"## Rollout and Rollback",
		decisionRollbackTemplateText,
	) && artifactSectionHasSubstantiveContent(
		root,
		change,
		"assurance.md",
		"## Rollback Readiness",
		assuranceRollbackTemplateText,
	)
}

func diagnosticsForUnsatisfiedAction(
	controlID model.ControlID,
	evidence runtimeVerificationState,
	scopeIssues []string,
	domainIssues []string,
	independentIssues []string,
	securityIssues []string,
	worktreeIssues []string,
) []string {
	switch controlID {
	case model.ControlResearch:
		return stringutil.UniqueSorted(append(scopeIssues, evidence.diagnostics...))
	case model.ControlDomainReview:
		return stringutil.UniqueSorted(append(domainIssues, evidence.diagnostics...))
	case model.ControlIndependentReview:
		return stringutil.UniqueSorted(append(independentIssues, evidence.diagnostics...))
	case model.ControlSecurityReview:
		return stringutil.UniqueSorted(append(securityIssues, evidence.diagnostics...))
	case model.ControlWorktreeIsolation:
		return stringutil.UniqueSorted(worktreeIssues)
	default:
		return nil
	}
}

func artifactSectionHasSubstantiveContent(root string, change model.Change, artifactName, heading string, templateBody string) bool {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return false
	}
	content, err := os.ReadFile(filepath.Join(paths.GovernedBundleDir, artifactName)) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return false
	}
	body := extractMarkdownSectionBody(string(content), heading)
	if strings.TrimSpace(body) == "" {
		return false
	}
	if normalizeSectionBody(body) == normalizeSectionBody(templateBody) {
		return false
	}
	// This runtime helper is only invoked for decision.md and assurance.md (via
	// hasRollbackDocumentation), so the placeholder gate stays scoped to
	// decision.md. requirements.md/tasks.md substance is NOT gated here — it is
	// enforced by the governed validation path (the progression substance gate
	// and the validate requirements/tasks contracts), so generalizing this gate
	// to those artifacts would be dead, unreachable code (issue #91).
	if artifactName == "decision.md" && artifact.LooksLikeTemplatePlaceholder(body) {
		return false
	}
	return true
}

func normalizeSectionBody(body string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(body)), " ")
}
