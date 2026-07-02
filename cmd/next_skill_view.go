package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/toolgen"
)

type assembleSkillViewOptions struct {
	IncludeSkillEvidence bool
	IncludeReviewContext bool
	IncludeContextBudget bool
	IncludeAgentContext  bool
}

var fullSkillViewOptions = assembleSkillViewOptions{
	IncludeSkillEvidence: true,
	IncludeReviewContext: true,
	IncludeContextBudget: true,
	IncludeAgentContext:  true,
}

const (
	testDesignTechniqueHintName = "skill:test-design"
	languageTestingHintName     = "capability:language-testing"
	languageTestingCapability   = "language-testing"
	techniqueHintKindCapability = "capability"
)

// assembleSkillView resolves the next skill, builds the skill view with technique hints,
// review context, constraints, and context budget, then applies guards.
// governedChange is the already-loaded executable change from buildNextContextByMode.
// precomputedPassingSkills lets callers reuse skill evidence that was already
// loaded earlier in the same invocation instead of reading verification files
// again.
func assembleSkillView(
	root string,
	view *nextView,
	ref changeRef,
	advanced progression.AdvanceSummary,
	governedChange *model.Change,
	execCtx *executionContext,
	precomputedPassingSkills map[string]model.VerificationRecord,
	artifactProjection *progression.ArtifactProjection,
	autoSkipEvidence bool,
) error {
	options := fullSkillViewOptions
	if autoSkipEvidence {
		options.IncludeSkillEvidence = false
	}
	return assembleSkillViewWithOptions(
		root,
		view,
		ref,
		advanced,
		governedChange,
		execCtx,
		precomputedPassingSkills,
		artifactProjection,
		options,
	)
}

func assembleSkillViewWithOptions(
	root string,
	view *nextView,
	ref changeRef,
	advanced progression.AdvanceSummary,
	governedChange *model.Change,
	execCtx *executionContext,
	precomputedPassingSkills map[string]model.VerificationRecord,
	artifactProjection *progression.ArtifactProjection,
	options assembleSkillViewOptions,
) error {
	// Build a synthetic Change for skill resolution when no governed change exists.
	resolveChange := model.Change{CurrentState: view.CurrentState}
	if governedChange != nil {
		resolveChange = *governedChange
	}
	reviewSelection := skill.ReviewSkillSelection{}
	var selectedReviewSkills []string
	if resolveChange.CurrentState == model.StateS3Review {
		if governedChange != nil {
			var err error
			reviewSelection, selectedReviewSkills, err = selectedReviewSkillsForChange(root, *governedChange)
			if err != nil {
				return err
			}
		} else {
			selectedReviewSkills = skill.SelectedReviewSkills(reviewSelection)
		}
	}
	// ResolveNextSkill now returns a skill set. This single-skill view uses the
	// first selected peer as the initial display, then S3_REVIEW resolution below
	// advances to the first selected peer that still needs evidence without
	// treating any peer as a predecessor of another.
	nextSkillName, nextState := progression.PrimaryNextSkillWithReviewSelection(resolveChange, reviewSelection)
	displaySkillName := nextSkillName
	resolutionReason := ""
	blockingResolution := false

	var evidenceMap map[string]model.VerificationRecord
	if governedChange != nil && nextSkillName != "" {
		if precomputedPassingSkills != nil {
			evidenceMap = precomputedPassingSkills
		} else {
			var evalErr error
			var latestRunVersion int
			if execCtx != nil {
				latestRunVersion = execCtx.LatestRunVersion
			} else {
				var err error
				latestRunVersion, err = state.LatestRelevantExecutionRunVersion(root, *governedChange)
				if err != nil {
					return err
				}
			}
			planningSubSteps := readOnlyRequiredSkillInputs(*governedChange)
			evidenceMap, _, evalErr = progression.EvaluateRequiredSkillsForChangeWithReviewSelection(
				root,
				*governedChange,
				view.CurrentState,
				latestRunVersion,
				reviewSelection,
				planningSubSteps...,
			)
			if evalErr != nil {
				return wrapRequiredSkillsEvaluationError("evaluate next skill evidence", ref.Slug, evalErr)
			}
		}
	}

	if options.IncludeSkillEvidence && evidenceMap != nil {
		staleSkills := requiredSkillStaleSet(view.Blockers)
		if advanced.Action == "blocked" {
			for skillName := range requiredSkillStaleSet(advanced.Blockers) {
				staleSkills[skillName] = true
			}
		}
		requiredSkillEvidence, err := buildRequiredSkillEvidence(root, *governedChange, view.CurrentState, execCtx, precomputedPassingSkills, staleSkills, reviewSelection)
		if err != nil {
			return wrapRequiredSkillsEvaluationError("evaluate required skill evidence", ref.Slug, err)
		}
		view.SkillEvidence = requiredSkillEvidence
	}

	blockersForResolution := append([]model.ReasonCode(nil), view.Blockers...)
	if advanced.Action == "blocked" {
		blockersForResolution = append(blockersForResolution, advanced.Blockers...)
	}
	if governedChange != nil && governedChange.CurrentState == model.StateS3Review {
		staleTarget, staleAvailable, err := progression.StaleEvidenceRepairAvailable(root, *governedChange, blockersForResolution)
		if err != nil {
			return err
		}
		if staleAvailable {
			reviewAlignmentBlockers := append([]model.ReasonCode(nil), staleTarget.Blockers...)
			reviewAlignmentBlockers = append(reviewAlignmentBlockers, model.NewReasonCode("review_alignment_required", staleTarget.SkillName))
			view.Blockers = appendReasonCodes(view.Blockers, reviewAlignmentBlockers)
			blockersForResolution = append(blockersForResolution, reviewAlignmentBlockers...)
		}
	}
	if len(selectedReviewSkills) > 0 && hasS3TaskPlanDriftReexecutionRoot(blockersForResolution) {
		// The S3 added-task drift root's only valid next action is reopening execution
		// (recovery names `slipway fix --start-reexecution`); no selected review/ship
		// skill applies. firstPendingSelectedReviewSkill would otherwise pick the first
		// stale review peer and loop the agent back through a review of the stale plan.
		// Leaving next_skill empty makes assembleSkillView return before any review/ship
		// handoff or review-batch is built, so the whole next payload stays consistent
		// with the reexecution recovery and blockers it already surfaces.
		nextSkillName = ""
		displaySkillName = ""
	} else if len(selectedReviewSkills) > 0 {
		nextSkillName = firstPendingSelectedReviewSkill(selectedReviewSkills, evidenceMap, blockersForResolution)
		displaySkillName = nextSkillName
		resolutionReason = ""
		blockingResolution = false
		if nextSkillName == "" {
			if repairSkill := reviewAlignmentSkillForBlockers(selectedReviewSkills, blockersForResolution); repairSkill != "" {
				nextSkillName = repairSkill
				blockingResolution = true
				resolutionReason = fmt.Sprintf("review alignment required for stale upstream evidence; rerun %s", repairSkill)
				view.Warnings = append(view.Warnings, resolutionReason)
			} else if shipSkill := nextS3ShipAuthoritySkill(evidenceMap, blockersForResolution); shipSkill != "" {
				nextSkillName = shipSkill
				displaySkillName = shipSkill
				blockingResolution = hasRequiredSkillBlockerFor(blockersForResolution, shipSkill)
			} else if actionableSkill, reason := resolveActionableBlockingSkill(nextSkillName, evidenceMap, blockersForResolution); actionableSkill != "" {
				nextSkillName = actionableSkill
				blockingResolution = true
				if reason != "" {
					resolutionReason = reason
					view.Warnings = append(view.Warnings, reason)
				}
			}
		}
	} else if actionableSkill, reason := resolveActionableBlockingSkill(nextSkillName, evidenceMap, blockersForResolution); actionableSkill != "" {
		nextSkillName = actionableSkill
		blockingResolution = true
		if reason != "" {
			resolutionReason = reason
			view.Warnings = append(view.Warnings, reason)
		}
	} else if skillHasPassingEvidence(evidenceMap, nextSkillName) {
		nextSkillName = ""
	}
	if !blockingResolution && displaySkillName != "" && displaySkillName != nextSkillName && hasRequiredSkillBlockerFor(blockersForResolution, nextSkillName) {
		blockingResolution = true
		resolutionReason = fmt.Sprintf("blocking skill evidence supersedes already-passing display skill: %s", nextSkillName)
	}

	// atBundlePlanAuditHandoff records that plan-audit is being surfaced from the
	// S1_PLAN/bundle authoring step rather than S1_PLAN/audit. plan-audit evidence
	// is rejected by `slipway evidence skill` until the substep advances to audit
	// (evidence_skill_wrong_plan_substep), so the bundle handoff must NOT advertise
	// write_evidence / evidence_record as the immediate action — doing so produces a
	// dead-end handoff (issue #229). The authoritative next action at bundle is to
	// run the lifecycle into S1_PLAN/audit first.
	atBundlePlanAuditHandoff := false
	if nextSkillName == "" && governedChange != nil &&
		governedChange.CurrentState == model.StateS1Plan &&
		governedChange.PlanSubStep == model.PlanSubStepBundle &&
		advanced.Action != "blocked" &&
		len(view.Blockers) == 0 {
		nextSkillName = progression.SkillPlanAudit
		nextState = string(model.StateS1Plan)
		atBundlePlanAuditHandoff = true
		view.Warnings = append(view.Warnings, "S1_PLAN/bundle is a machine authoring step; ensure bundle artifacts are complete, then run `slipway run --json` to enter S1_PLAN/audit. plan-audit evidence cannot be recorded until the substep advances to audit.")
	}

	if nextSkillName == "" {
		view.NextSkill = nil
		blockers := append([]model.ReasonCode(nil), view.Blockers...)
		if advanced.Action == "blocked" {
			blockers = append(blockers, advanced.Blockers...)
		}
		// A genuine dead-end on the gate that owns advancement out of this
		// (state, plan substep) must keep the no-skill step from advertising
		// ready to advance. Folding the captured dead-end codes into the blocker
		// set here makes the readiness guard below skip the no_skill_required /
		// run_slipway_run_to_advance advertisement and surfaces the dead-end so
		// confirmation and recovery read governance-blocked (#382). Pacing blocks
		// were already filtered out at capture time, so they keep riding the
		// normal handoff/run guidance instead of surfacing here.
		blockers = append(blockers, view.ownedAdvanceGateDeadEndBlockers...)
		if len(blockers) == 0 {
			if governedChange != nil {
				canAdvance, advanceBlockers := noSkillStateAdvanceReadiness(root, *governedChange)
				if len(advanceBlockers) > 0 {
					blockers = append(blockers, advanceBlockers...)
				} else if canAdvance {
					blockers = append(blockers, model.NewReasonCode("run_slipway_run_to_advance", string(view.CurrentState)))
				}
			}
			blockers = append(blockers, model.NewReasonCode("no_skill_required", string(view.CurrentState)))
		}
		view.Blockers = model.NormalizeReasonCodes(blockers)
		return nil
	}

	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return wrapRequiredSkillsEvaluationError("load governance registry", ref.Slug, err)
	}

	verificationDir := state.DisplayPath(root, filepath.Dir(state.VerificationFilePath(root, ref.Slug, nextSkillName)))

	ns := &nextSkillView{
		Name:                 nextSkillName,
		Subagent:             subagentDirectiveForSkill(view.config, nextSkillName),
		SelectedReviewSkills: append([]string(nil), selectedReviewSkills...),
		VerificationDir:      verificationDir,
		State:                nextState,
	}
	if displaySkillName != "" && displaySkillName != nextSkillName {
		ns.DisplayName = displaySkillName
		if blockingResolution {
			ns.BlockingName = nextSkillName
		}
		if resolutionReason != "" {
			ns.ResolutionReason = resolutionReason
		} else {
			ns.ResolutionReason = "passing skill evidence advances display skill"
		}
	}

	if governedChange != nil && model.WorkflowState(nextState) == model.StateS1Plan {
		mapStatus := view.InputContext.CodebaseMapStatus
		// Re-source the empty-map technique hint from the workspace-bound status
		// field (REQ-009) rather than a second HasEmptyCodebaseMap(root, …) probe.
		// The old probe re-joined worktree-relative doc paths against the
		// invocation root, so under `slipway next --change <slug>` from the root
		// checkout it read the wrong map and could contradict codebase_map_status.
		// Reading the one AssessCodebaseMapDocs(paths.WorkspaceRoot) assessment
		// keeps the hint, the status field, and the advisory consistent. The hint
		// fires for missing/scaffold_only, matching the old probe's truth set.
		if codebaseMapStatusHasNoDurableDocs(mapStatus) {
			ns.TechniqueHints = append(ns.TechniqueHints, techniqueHint{
				Name:   "skill:codebase-mapping",
				Reason: "No durable codebase-map documents found. Run the slipway-codebase-mapping skill to write source-backed artifacts/codebase/* documents before planning; `slipway codebase-map` only scaffolds the empty document set.",
			})
		}
		// Codebase-map advisories (non-blocking; surfaced via view.Warnings, which
		// the compact handoff projection copies). The consume-time advisory takes
		// priority for the map-consuming planning skills (research/plan-audit) when
		// the map is scaffold_only/baseline. The broader discovery advisory covers
		// the gap it leaves — chiefly a fully missing map for a discovery-scoped
		// change — and routes the host to the slipway-codebase-mapping skill. The
		// two are mutually exclusive, so at most one codebase_map_advisory fires and
		// neither ever blocks progression.
		if advisory := codebaseMapConsumeAdvisory(mapStatus, nextSkillName); advisory != "" {
			view.Warnings = append(view.Warnings, advisory)
		} else if advisory := codebaseMapDiscoveryAdvisory(mapStatus, nextSkillName, governedChange.NeedsDiscovery); advisory != "" {
			view.Warnings = append(view.Warnings, advisory)
		}
	}

	// Codebase-map relevance self-check (#80): a populated/partial map reflects
	// content presence, not scope relevance, and can be consumed stale at any
	// durable-map consumer — including wave-orchestration at S2_IMPLEMENT, the exact
	// handoff issue #80 reproduces — so this fires independent of lifecycle state.
	// It is disjoint by status from the S1 consume/discovery advisories above
	// (those own scaffold_only/baseline/missing), so it never double-fires.
	if governedChange != nil {
		if advisory := codebaseMapRelevanceAdvisory(view.InputContext.CodebaseMapStatus, nextSkillName); advisory != "" {
			view.Warnings = append(view.Warnings, advisory)
		}
	}

	// Auto capability resolver: attach catalog-skill hints on top of the
	// kernel's host selection. Never changes the next skill chosen by
	// ResolveNextSkill; only enriches TechniqueHints.
	ns.TechniqueHints = appendCatalogHints(ns.TechniqueHints, nextSkillName, governedChange, view)
	ns.TechniqueHints = appendWorkflowProfileTechniqueHints(ns.TechniqueHints, nextSkillName, governedChange)
	ns.TechniqueHints = appendLanguageTestingHints(ns.TechniqueHints, root, governedChange)

	if options.IncludeReviewContext && (nextSkillName == progression.SkillSpecComplianceReview || nextSkillName == progression.SkillCodeQualityReview) {
		ns.ReviewContext = buildReviewContext(governedChange, artifactProjection, false, nextSkillName)
		if governedChange != nil {
			ns.RequiredTokens = progression.RequiredReviewLayerTokensForSkill(*governedChange, artifactProjection, false, nextSkillName)
		}
	}

	if def, ok := skill.LookupDefinitionInRegistry(registry, nextSkillName); ok {
		ns.SkillConstraints = buildSkillConstraints(root, def, governedChange, view.planLocked)
	}

	view.NextSkill = ns
	if governedChange != nil && governedChange.CurrentState == model.StateS3Review && len(selectedReviewSkills) > 0 {
		view.ReviewBatch = buildReviewBatchView(reviewBatchBuildInput{
			root:                 root,
			registry:             registry,
			change:               governedChange,
			artifactProjection:   artifactProjection,
			view:                 view,
			selectedReviewSkills: selectedReviewSkills,
			evidenceMap:          evidenceMap,
			blockers:             blockersForResolution,
			verificationDir:      verificationDir,
			nextState:            nextState,
			includeReviewContext: options.IncludeReviewContext,
			planLocked:           view.planLocked,
		})
	}
	if options.IncludeContextBudget {
		view.ContextBudget = estimateContextBudget(root, ns, view.InputContext)
	}
	if options.IncludeAgentContext {
		view.Constraints = deriveAgentConstraints(registry, nextSkillName)
		if atBundlePlanAuditHandoff {
			// At S1_PLAN/bundle the authoritative next action is `slipway run` into
			// S1_PLAN/audit, not recording evidence: `slipway evidence skill --skill
			// plan-audit` fails closed with evidence_skill_wrong_plan_substep until the
			// substep advances. Drop write_evidence / evidence_record so the handoff
			// does not advertise an action the evidence command rejects (issue #229).
			// The audit-substep handoff keeps the registry constraints unchanged.
			view.Constraints.AllowedOperations = withoutOperation(view.Constraints.AllowedOperations, "write_evidence")
			view.Constraints.RequiredOutputs = withoutOperation(view.Constraints.RequiredOutputs, "evidence_record")
		}
	}

	if advanced.Action == "blocked" {
		view.Blockers = appendReasonCodes(view.Blockers, advanced.Blockers)
	}
	applyContextBudgetGuard(view)

	return nil
}

// withoutOperation returns values with every occurrence of op removed, preserving
// order. An emptied list collapses to nil so it serializes as a JSON null rather
// than carrying a stale entry.
func withoutOperation(values []string, op string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == op {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// codebaseMapStatusHasNoDurableDocs reports whether a whole-map status means no
// durable codebase-map documents exist (missing or scaffold_only), driving the
// empty-map technique hint. baseline/partial/populated return false. This mirrors
// the truth set of the retired progression.HasEmptyCodebaseMap probe without its
// independent filesystem read (REQ-009).
func codebaseMapStatusHasNoDurableDocs(status string) bool {
	switch status {
	case artifact.CodebaseMapStatusMissing, artifact.CodebaseMapStatusScaffoldOnly:
		return true
	default:
		return false
	}
}

// codebaseMapConsumeAdvisory returns a non-blocking consume-time advisory when a
// map-consuming planning skill (research-orchestration or plan-audit) is next and
// the codebase map is non-durable (scaffold_only or baseline). It returns "" for
// populated/partial/missing maps and for non-consuming skills. The wording adds
// consume-time framing rather than restating the empty-map hint's "no durable
// docs" text, so for scaffold_only — where both the hint and the advisory fire —
// the two stay complementary, not contradictory. partial carries no CONSUME
// advisory here because it is a durable status owned by
// codebaseMapRelevanceAdvisory instead (populated/partial); for a partial map
// that advisory routes the host to codebase_map_doc_states for the per-doc gaps.
func codebaseMapConsumeAdvisory(status, nextSkillName string) string {
	switch nextSkillName {
	case progression.SkillResearchOrchestration, progression.SkillPlanAudit:
	default:
		return ""
	}
	switch status {
	case artifact.CodebaseMapStatusScaffoldOnly, artifact.CodebaseMapStatusBaseline:
		return fmt.Sprintf(
			"codebase_map_advisory: %s is consuming a non-durable codebase map (status: %s); refine artifacts/codebase with source-backed findings before relying on it as reviewed context, or inspect input_context.codebase_map_doc_states for per-doc gaps.",
			nextSkillName, status,
		)
	default:
		return ""
	}
}

// codebaseMapRelevanceAdvisory returns a non-blocking advisory when a durable-map
// consumer is next and the codebase map is durable (populated or partial). The
// consumers are research-orchestration and plan-audit (S1_PLAN) and
// wave-orchestration (S2_IMPLEMENT) — the same set the codebase-mapping skill names
// as SHOULD-consume — so the advisory fires at the exact handoff issue #80
// reproduces: a stale populated map consumed at wave-orchestration. The map status
// reflects content presence, not scope relevance — a map authored for a prior
// change still reads `populated` — so Slipway cannot tell whether the map matches
// THIS change. The engine only surfaces the trigger; the host AI owns the semantic
// relevance judgment and the inline refresh (re-author the relevant docs in
// artifacts/codebase in place; the assessment re-reads them on every run). For a
// partial map the advisory also routes the host to codebase_map_doc_states to
// complete the unfinished docs. It is disjoint by status from the non-durable
// consume advisory (scaffold_only/baseline), so at most one codebase_map_advisory
// fires.
func codebaseMapRelevanceAdvisory(status, nextSkillName string) string {
	switch nextSkillName {
	case progression.SkillResearchOrchestration, progression.SkillPlanAudit, progression.SkillWaveOrchestration:
	default:
		return ""
	}
	switch status {
	case artifact.CodebaseMapStatusPopulated, artifact.CodebaseMapStatusPartial:
		advisory := fmt.Sprintf(
			"codebase_map_advisory: %s is consuming a codebase map whose status (%s) reflects content presence, not scope relevance — it may have been authored for a prior change. Judge whether its affected seams, blast radius, and concerns match THIS change's scope, and re-author any stale sections in artifacts/codebase inline before relying on it.",
			nextSkillName, status,
		)
		// A partial map mixes durable and non-durable docs; route the host to the
		// per-doc states so it completes the unfinished set, matching the consuming
		// skills' guidance. A populated map has no per-doc gaps, so it is omitted.
		if status == artifact.CodebaseMapStatusPartial {
			advisory += " It is partial: inspect input_context.codebase_map_doc_states and complete any scaffold_only/baseline/missing doc."
		}
		return advisory + " Advisory only — this does not block progression."
	default:
		return ""
	}
}

// codebaseMapDiscoveryAdvisory returns a non-blocking discovery-phase advisory
// nudging the host to populate a durable codebase map before planning. It fires
// only for discovery-scoped changes (needs_discovery) while a planning skill that
// consumes the map (research-orchestration or plan-audit) is next and the map is
// non-durable. The call site gives the narrower consume-time advisory priority,
// so in practice this carries the case that one omits: a fully missing map — the
// "nothing but templates" state this nudge exists to break. It routes to the
// slipway-codebase-mapping skill (which writes source-backed content) rather than
// `slipway codebase-map` (which only scaffolds the document set). Non-discovery
// changes get nothing: a change that opted out of discovery should not be nagged
// to map the repository.
func codebaseMapDiscoveryAdvisory(status, nextSkillName string, needsDiscovery bool) string {
	if !needsDiscovery {
		return ""
	}
	switch nextSkillName {
	case progression.SkillResearchOrchestration, progression.SkillPlanAudit:
	default:
		return ""
	}
	switch status {
	case artifact.CodebaseMapStatusMissing, artifact.CodebaseMapStatusScaffoldOnly, artifact.CodebaseMapStatusBaseline:
		return fmt.Sprintf(
			"codebase_map_advisory: no durable codebase map (status: %s) for this discovery-scoped change; run the slipway-codebase-mapping skill to write source-backed artifacts/codebase/* documents before %s consumes it for planning. Advisory only — this does not block progression.",
			status, nextSkillName,
		)
	default:
		return ""
	}
}

func resolveActionableBlockingSkill(
	current string,
	evidenceMap map[string]model.VerificationRecord,
	blockers []model.ReasonCode,
) (string, string) {
	current = strings.TrimSpace(current)
	if current != "" && !skillHasPassingEvidence(evidenceMap, current) {
		return "", ""
	}
	for _, blocker := range blockers {
		if !isRequiredSkillBlocker(blocker.Code) {
			continue
		}
		skillName := blockerSkillName(blocker)
		if skillName == "" || (current != "" && skillName == current) {
			continue
		}
		if current == "" {
			return skillName, ""
		}
		return skillName, fmt.Sprintf("blocking skill evidence supersedes already-passing display skill: %s", skillName)
	}
	return "", ""
}

func selectedReviewSkillsForChange(root string, change model.Change) (skill.ReviewSkillSelection, []string, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return skill.ReviewSkillSelection{}, nil, err
	}
	snap, err := governance.PreviewGovernanceSnapshot(root, change, paths.GovernedBundleDir)
	if err != nil {
		return skill.ReviewSkillSelection{}, nil, err
	}
	selection, selected := selectedReviewSkillsFromControls(snap.ActiveControls, change.EffectiveWorkflowProfile())
	return selection, selected, nil
}

func selectedReviewSkillsFromReadiness(readiness progression.GovernanceReadiness, profile model.WorkflowProfile) []string {
	_, selected := selectedReviewSkillsFromControls(readiness.ActiveControls, profile)
	return selected
}

func selectedReviewSkillsFromControls(activeControls []model.ControlActivation, profile model.WorkflowProfile) (skill.ReviewSkillSelection, []string) {
	selection := progression.ReviewSkillSelectionFromControls(activeControls)
	return selection, skill.SelectedReviewSkillsForWorkflowProfile(selection, profile)
}

func firstPendingSelectedReviewSkill(
	selectedReviewSkills []string,
	evidenceMap map[string]model.VerificationRecord,
	blockers []model.ReasonCode,
) string {
	if len(selectedReviewSkills) == 0 {
		return ""
	}
	for _, skillName := range selectedReviewSkills {
		if hasRequiredSkillBlockerFor(blockers, skillName) {
			return skillName
		}
	}
	for _, skillName := range selectedReviewSkills {
		if !skillHasPassingEvidence(evidenceMap, skillName) {
			return skillName
		}
	}
	return ""
}

type reviewBatchBuildInput struct {
	root                 string
	registry             []skill.Definition
	change               *model.Change
	artifactProjection   *progression.ArtifactProjection
	view                 *nextView
	selectedReviewSkills []string
	evidenceMap          map[string]model.VerificationRecord
	blockers             []model.ReasonCode
	verificationDir      string
	nextState            string
	includeReviewContext bool
	planLocked           bool
}

func buildReviewBatchView(input reviewBatchBuildInput) *reviewBatchView {
	if input.change == nil || len(input.selectedReviewSkills) == 0 {
		return nil
	}
	pending := pendingReviewBatchSkills(input.selectedReviewSkills, input.evidenceMap, input.blockers)
	if len(pending) == 0 {
		if repairSkill := reviewAlignmentSkillForBlockers(input.selectedReviewSkills, input.blockers); repairSkill != "" {
			pending = []string{repairSkill}
		}
	}
	if len(pending) < 2 {
		return nil
	}
	nextState := strings.TrimSpace(input.nextState)
	if nextState == "" {
		nextState = string(model.StateS3Review)
	}
	batch := &reviewBatchView{
		Mode:            "parallel",
		Subagent:        subagentDirectiveForSlot(input.view.config, model.SubagentSlotReview),
		VerificationDir: input.verificationDir,
		State:           nextState,
		Skills:          make([]reviewBatchSkillView, 0, len(pending)),
	}
	for _, skillName := range pending {
		item := reviewBatchSkillView{
			Name:           skillName,
			RequiredTokens: progression.RequiredReviewLayerTokensForSkill(*input.change, input.artifactProjection, false, skillName),
		}
		if input.includeReviewContext &&
			(skillName == progression.SkillSpecComplianceReview || skillName == progression.SkillCodeQualityReview) {
			item.ReviewContext = buildReviewContext(input.change, input.artifactProjection, false, skillName)
		}
		item.TechniqueHints = appendCatalogHints(item.TechniqueHints, skillName, input.change, input.view)
		item.TechniqueHints = appendWorkflowProfileTechniqueHints(item.TechniqueHints, skillName, input.change)
		item.TechniqueHints = appendLanguageTestingHints(item.TechniqueHints, input.root, input.change)
		if def, ok := skill.LookupDefinitionInRegistry(input.registry, skillName); ok {
			item.SkillConstraints = buildSkillConstraints(input.root, def, input.change, input.planLocked)
		}
		batch.Skills = append(batch.Skills, item)
	}
	return batch
}

func subagentDirectiveForSkill(cfg model.Config, skillName string) *subagentDirective {
	slot, ok := subagentSlotForSkill(skillName)
	if !ok {
		return nil
	}
	return subagentDirectiveForSlot(cfg, slot)
}

func subagentSlotForSkill(skillName string) (model.SubagentSlotName, bool) {
	switch strings.TrimSpace(skillName) {
	case progression.SkillPlanAudit:
		return model.SubagentSlotPlanAudit, true
	case progression.SkillSpecComplianceReview,
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
		progression.SkillSecurityReview:
		return model.SubagentSlotReview, true
	case progression.SkillShipVerification:
		return model.SubagentSlotVerify, true
	default:
		return "", false
	}
}

func subagentDirectiveForSlot(cfg model.Config, slot model.SubagentSlotName) *subagentDirective {
	resolved := cfg.ResolveSubagent(slot)
	if resolved.IsZero() {
		return nil
	}
	return cloneSubagentDirective(&resolved)
}

func cloneSubagentDirective(in *subagentDirective) *subagentDirective {
	if in == nil {
		return nil
	}
	out := *in
	if in.EngineBoundary != nil {
		engineBoundary := *in.EngineBoundary
		out.EngineBoundary = &engineBoundary
	}
	return &out
}

func pendingReviewBatchSkills(
	selectedReviewSkills []string,
	evidenceMap map[string]model.VerificationRecord,
	blockers []model.ReasonCode,
) []string {
	pending := make([]string, 0, len(selectedReviewSkills))
	seen := map[string]bool{}
	add := func(skillName string) {
		skillName = strings.TrimSpace(skillName)
		if skillName == "" || seen[skillName] {
			return
		}
		seen[skillName] = true
		pending = append(pending, skillName)
	}
	for _, skillName := range selectedReviewSkills {
		if hasRequiredSkillBlockerFor(blockers, skillName) {
			add(skillName)
		}
	}
	for _, skillName := range selectedReviewSkills {
		if !skillHasPassingEvidence(evidenceMap, skillName) {
			add(skillName)
		}
	}
	return pending
}

// hasS3TaskPlanDriftReexecutionRoot reports the S3 added-task drift root whose only
// valid next action is reopening execution (recovery names `slipway fix
// --start-reexecution`). When present, no review/ship skill handoff must be offered:
// rerunning a review re-certifies against the stale plan and loops — the dead end
// reexecution exists to break.
func hasS3TaskPlanDriftReexecutionRoot(blockers []model.ReasonCode) bool {
	for _, b := range blockers {
		if strings.TrimSpace(b.Code) == "s3_task_plan_drift_requires_reexecution" {
			return true
		}
	}
	return false
}

func nextS3ShipAuthoritySkill(
	evidenceMap map[string]model.VerificationRecord,
	blockers []model.ReasonCode,
) string {
	if hasS3TaskPlanDriftReexecutionRoot(blockers) {
		return ""
	}
	if hasRequiredSkillBlockerFor(blockers, progression.SkillShipVerification) ||
		!skillHasPassingEvidence(evidenceMap, progression.SkillShipVerification) ||
		hasShipVerificationHandoffBlocker(blockers) {
		return progression.SkillShipVerification
	}
	return ""
}

// hasShipVerificationHandoffBlocker reports an active G_ship gate blocker that a
// fresh ship-verification handoff resolves. It keeps `next` from stranding the
// operator when a passing-but-malformed ship record leaves the gate blocked yet
// the generic required-skill check sees a passing record.
func hasShipVerificationHandoffBlocker(blockers []model.ReasonCode) bool {
	for _, blocker := range blockers {
		if progression.IsShipVerificationHandoffBlockerCode(blocker.Code) {
			return true
		}
	}
	return false
}

func reviewAlignmentSkillForBlockers(selectedReviewSkills []string, blockers []model.ReasonCode) string {
	if hasS3TaskPlanDriftReexecutionRoot(blockers) {
		return ""
	}
	for _, blocker := range blockers {
		switch strings.TrimSpace(blocker.Code) {
		case "review_alignment_required":
			if skillName := reviewAlignmentSkillForTarget(selectedReviewSkills, blocker.Detail); skillName != "" {
				return skillName
			}
		case "required_skill_stale":
			target := blockerSkillName(blocker)
			if !stringInSlice(selectedReviewSkills, target) {
				if skillName := reviewAlignmentSkillForTarget(selectedReviewSkills, target); skillName != "" {
					return skillName
				}
			}
		}
	}
	return ""
}

func reviewAlignmentSkillForTarget(selectedReviewSkills []string, target string) string {
	target = strings.TrimSpace(target)
	if stringInSlice(selectedReviewSkills, target) {
		return target
	}
	switch target {
	case progression.SkillPlanAudit:
		if stringInSlice(selectedReviewSkills, progression.SkillSpecComplianceReview) {
			return progression.SkillSpecComplianceReview
		}
	case progression.SkillWaveOrchestration:
		if stringInSlice(selectedReviewSkills, progression.SkillCodeQualityReview) {
			return progression.SkillCodeQualityReview
		}
	}
	if len(selectedReviewSkills) > 0 {
		return strings.TrimSpace(selectedReviewSkills[0])
	}
	return ""
}

func hasRequiredSkillBlockerFor(blockers []model.ReasonCode, skillName string) bool {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return false
	}
	for _, blocker := range blockers {
		if !isRequiredSkillBlocker(blocker.Code) {
			continue
		}
		if blockerSkillName(blocker) == skillName {
			return true
		}
	}
	return false
}

func skillHasPassingEvidence(evidenceMap map[string]model.VerificationRecord, skillName string) bool {
	if len(evidenceMap) == 0 {
		return false
	}
	rec, ok := evidenceMap[skillName]
	return ok && rec.IsPassing()
}

func isRequiredSkillBlocker(code string) bool {
	return progression.IsRequiredSkillBlockerCode(code)
}

// requiredSkillStaleSet collects the skills carrying a required_skill_stale
// digest-drift blocker, keyed by the parsed blocker subject (e.g. "plan-audit"
// from "required_skill_stale:plan-audit:assurance.md").
func requiredSkillStaleSet(blockers []model.ReasonCode) map[string]bool {
	out := map[string]bool{}
	for _, blocker := range blockers {
		if strings.TrimSpace(blocker.Code) != "required_skill_stale" {
			continue
		}
		if name := blockerSkillName(blocker); name != "" {
			out[name] = true
		}
	}
	return out
}

// blockerSkillName returns the skill-name subject of a required-skill blocker,
// delegating to model.ParseBlocker so prefix-token decomposition lives in one
// place (the blocker subject, e.g. "plan-audit" from
// "required_skill_stale:plan-audit:assurance.md").
func blockerSkillName(blocker model.ReasonCode) string {
	return model.ParseBlocker(blocker).Subject
}

func buildRequiredSkillEvidence(
	root string,
	change model.Change,
	workflowState model.WorkflowState,
	execCtx *executionContext,
	precomputedPassingSkills map[string]model.VerificationRecord,
	staleSkills map[string]bool,
	reviewSelection skill.ReviewSkillSelection,
) ([]skillEvidenceEntry, error) {
	var latestRunVersion int
	var err error
	if execCtx != nil {
		latestRunVersion = execCtx.LatestRunVersion
	} else {
		latestRunVersion, err = state.LatestRelevantExecutionRunVersion(root, change)
		if err != nil {
			return nil, err
		}
	}
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return nil, err
	}
	var planSubSteps []model.PlanSubStep
	if workflowState == model.StateS1Plan && change.PlanSubStep != model.PlanSubStepNone {
		planSubSteps = []model.PlanSubStep{change.PlanSubStep}
	}
	required := skill.RequiredSkillsForStateWithRegistryWithReviewSelection(
		registry,
		change.NeedsDiscovery,
		workflowState,
		reviewSelection,
		planSubSteps...,
	)
	required = skill.FilterRequiredSkillsForWorkflowProfileWithReviewSelection(required, change.EffectiveWorkflowProfile(), reviewSelection)
	evidence := make([]skillEvidenceEntry, 0, len(required))
	if precomputedPassingSkills != nil {
		for _, skillName := range required {
			entry := skillEvidenceEntry{
				SkillName: skillName,
				Status:    "missing",
			}
			if rec, ok := precomputedPassingSkills[skillName]; ok {
				entry.HasEvidence = true
				entry.Verdict = rec.Verdict
				entry.Status = "passing"
			}
			// A digest-drift blocker supersedes missing/passing: the recorded
			// verdict exists but its certified inputs are stale.
			if staleSkills[skillName] {
				entry.HasEvidence = true
				entry.Status = "stale"
			}
			evidence = append(evidence, entry)
		}
		return evidence, nil
	}
	verifications, err := state.ListVerificationsForChange(root, change)
	if err != nil {
		return nil, err
	}
	for _, skillName := range required {
		entry := skillEvidenceEntry{
			SkillName: skillName,
			Status:    "missing",
		}
		rec, ok := verifications[skillName]
		if !ok {
			evidence = append(evidence, entry)
			continue
		}
		entry.HasEvidence = true
		entry.Verdict = rec.Verdict
		entry.Status = "not_ready"
		if def, ok := skill.LookupDefinitionInRegistry(registry, skillName); ok {
			switch {
			case def.RunSummaryBound && latestRunVersion < 1:
				entry.Status = "stale"
			case def.RunSummaryBound && latestRunVersion > 0 && rec.RunVersion != latestRunVersion:
				entry.Status = "stale"
			case rec.IsPassing():
				entry.Status = "passing"
			case rec.Verdict == model.VerificationVerdictFail:
				entry.Status = "failed"
			case len(rec.Blockers) > 0:
				entry.Status = "blocked"
			}
		} else if rec.IsPassing() {
			entry.Status = "passing"
		}
		// Digest drift (certified inputs changed after the verdict) is stale even
		// when the recorded verdict itself passes.
		if staleSkills[skillName] {
			entry.Status = "stale"
		}
		evidence = append(evidence, entry)
	}
	return evidence, nil
}

// appendCatalogHints runs the capability resolver against the current host
// and returns technique-hint entries derived from exported support attachments.
// The resolver is read-only with respect to kernel progression.
func appendCatalogHints(
	existing []techniqueHint,
	hostSkill string,
	governedChange *model.Change,
	view *nextView,
) []techniqueHint {
	sig := capability.Signals{Host: hostSkill}
	if governedChange != nil {
		sig.Paths = append(sig.Paths, governedChange.WorktreePath)
	}
	if view != nil {
		for _, rc := range view.Blockers {
			sig.Blockers = append(sig.Blockers, rc.Code)
		}
	}

	reg := capability.DefaultRegistry()
	resolution := capability.Resolve(reg, sig)
	for _, support := range resolution.Supports {
		if !toolgen.ShouldExportAsHostSkill(support.SkillID) {
			continue
		}
		existing = append(existing, techniqueHint{
			Name:              supportHintName(support.SkillID),
			Reason:            fmt.Sprintf("[%s] %s", support.Kind, support.Reason),
			HydrateReferences: normalizeHydrateKeys(capability.HydrateReferenceKeysForSkill(reg, support.SkillID)),
		})
	}
	return existing
}

func noSkillStateAdvanceReadiness(root string, change model.Change) (bool, []model.ReasonCode) {
	switch change.CurrentState {
	case model.StateS0Intake:
		blockers := progression.IntakeAdvanceBlockers(root, change)
		return len(blockers) == 0, blockers
	case model.StateS1Plan:
		// S1 research reaches the no-skill branch only after its evidence passed
		// and the display skill was cleared.
		switch change.PlanSubStep {
		case model.PlanSubStepResearch, model.PlanSubStepValidate:
			return true, nil
		case model.PlanSubStepAudit:
			result := progression.ValidatePlanningReadiness(root, change)
			if len(result.Blockers) > 0 {
				return false, result.Blockers
			}
			return true, nil
		default:
			return false, nil
		}
	case model.StateS3Review:
		return true, nil
	default:
		return false, nil
	}
}

func supportHintName(skillID string) string {
	return "skill:" + skillID
}

func appendWorkflowProfileTechniqueHints(existing []techniqueHint, hostSkill string, governedChange *model.Change) []techniqueHint {
	if governedChange == nil {
		return existing
	}
	reg := capability.DefaultRegistry()
	addHint := func(skillID, reason string) {
		if !toolgen.ShouldExportAsHostSkill(skillID) {
			return
		}
		existing = append(existing, techniqueHint{
			Name:              supportHintName(skillID),
			Reason:            reason,
			HydrateReferences: normalizeHydrateKeys(capability.HydrateReferenceKeysForSkill(reg, skillID)),
		})
	}
	switch governedChange.EffectiveWorkflowProfile() {
	case model.WorkflowProfileDocs:
		if hostSkill == progression.SkillSpecComplianceReview || hostSkill == progression.SkillShipVerification {
			addHint("spec-trace", "[workflow-profile:docs] verify rendered docs, links, and requirement references instead of code-only quality signals")
		}
	case model.WorkflowProfileResearch:
		if hostSkill == progression.SkillResearchOrchestration || hostSkill == progression.SkillShipVerification {
			addHint("codebase-mapping", "[workflow-profile:research] keep discovery evidence bounded and cite only the artifacts needed for the research answer")
		}
	case model.WorkflowProfileConfig:
		addHint("supply-chain-audit", "[workflow-profile:config] inspect dependency, build, and rollback implications before treating config changes as low risk")
	case model.WorkflowProfileMeta:
		addHint("spec-trace", "[workflow-profile:meta] preserve generated-surface and schema compatibility for Slipway governance changes")
	}
	return existing
}

func appendLanguageTestingHints(existing []techniqueHint, root string, governedChange *model.Change) []techniqueHint {
	if governedChange == nil || !hasTestDesignHint(existing) {
		return existing
	}

	languages := normalizeLanguageHints(governedChange.ProjectContext.Languages)
	if len(languages) == 0 {
		languages = stackLanguageHints(languageHintStackRoot(root, *governedChange))
	}
	if len(languages) == 0 {
		return existing
	}

	seen := map[string]struct{}{}
	for _, hint := range existing {
		if hint.Name == languageTestingHintName && hint.Language != "" {
			seen[strings.ToLower(hint.Language)] = struct{}{}
		}
	}
	for _, language := range languages {
		key := strings.ToLower(language)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		existing = append(existing, techniqueHint{
			Name:       languageTestingHintName,
			Kind:       techniqueHintKindCapability,
			Capability: languageTestingCapability,
			Language:   language,
			Optional:   true,
			Reason: fmt.Sprintf(
				"[idiom:%s] If an installed language testing skill exists, use it for idiomatic test APIs, framework conventions, and assertion style.",
				language,
			),
		})
	}
	return existing
}

func hasTestDesignHint(hints []techniqueHint) bool {
	for _, hint := range hints {
		if hint.Name == testDesignTechniqueHintName {
			return true
		}
	}
	return false
}

func languageHintStackRoot(root string, governedChange model.Change) string {
	workspaceRoot, err := state.WorkspaceRootForChange(root, governedChange)
	if err != nil || strings.TrimSpace(workspaceRoot) == "" {
		return root
	}
	return workspaceRoot
}

func stackLanguageHints(root string) []string {
	data, err := os.ReadFile(filepath.Join(state.CodebaseMapDir(root), "STACK.md"))
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- Languages:") {
			continue
		}
		return normalizeLanguageHints(strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "- Languages:")), ","))
	}
	return nil
}

func normalizeLanguageHints(values []string) []string {
	languages := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		language := strings.TrimSpace(value)
		if language == "" {
			continue
		}
		key := strings.ToLower(language)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		languages = append(languages, language)
	}
	return languages
}
