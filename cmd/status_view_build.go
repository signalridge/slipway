package cmd

import (
	"path/filepath"
	"slices"

	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/progression"
	enginestatus "github.com/signalridge/slipway/internal/engine/status"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func buildStatusViewFromChange(root string, change model.Change) (statusView, error) {
	return buildGovernedStatusViewWithExecutionContext(root, change, nil)
}

func buildGovernedStatusViewWithExecutionContext(root string, change model.Change, preloadedExecCtx *executionContext) (statusView, error) {
	var blockers []model.ReasonCode
	presetFields, err := buildWorkflowPresetView(root, change)
	if err != nil {
		return statusView{}, err
	}

	// Preset confirmation is a universal early S1 blocker. When pending,
	// return a minimal view with only identity, preset fields, and the single
	// blocker — no progress, artifact_dag, source_state_file, evidence,
	// governance surface, or downstream ready actions. This prevents leaking
	// bundle semantics before the preset is confirmed.
	if change.WorkflowPresetConfirmationPending() {
		profile := buildChangeProfileView(change)
		view := statusView{
			ExecutionMode:             "governed",
			Slug:                      change.Slug,
			Phase:                     model.PhaseFor(change.CurrentState),
			LifecycleStatus:           string(change.Status),
			CurrentState:              change.CurrentState,
			IntakeSubStep:             change.IntakeSubStep,
			PlanSubStep:               change.PlanSubStep,
			PlanningNote:              planningNote(change.CurrentState, change.PlanSubStep),
			QualityMode:               profile.QualityMode,
			NeedsDiscovery:            profile.NeedsDiscovery,
			WorkflowPreset:            presetFields.WorkflowPreset,
			SuggestedWorkflowPreset:   presetFields.SuggestedWorkflowPreset,
			EffectiveWorkflowPreset:   presetFields.EffectiveWorkflowPreset,
			PresetConfirmationPending: presetFields.PresetConfirmationPending,
			PresetUpgradeReasons:      presetFields.PresetUpgradeReasons,
			GovernanceForecast:        presetFields.GovernanceForecast,
			Blockers:                  []model.ReasonCode{model.NewReasonCode("preset_confirmation_required", "")},
			NextReadyActions:          []string{"preset <light|standard|strict>"},
			EvidenceFreshness:         "unknown",
		}
		view.Narrative = buildStatusNarrative(view)
		return view, nil
	}

	var execCtx executionContext
	if preloadedExecCtx != nil {
		execCtx = *preloadedExecCtx
	} else {
		execCtx, err = loadExecutionContext(root, change)
		if err != nil {
			return statusView{}, err
		}
	}
	readiness, err := progression.EvaluateGovernanceReadiness(
		root,
		change,
		progression.GovernanceReadinessOptions{
			IncludeGateEvaluations: true,
			// Status renders artifact-centric context, so it opts into the
			// in-memory projection on top of shared blockers.
			IncludeArtifactProjection: true,
		},
	)
	if err != nil {
		return statusView{}, wrapGovernanceReadinessError("build status view", change.Slug, err)
	}
	blockers = append(blockers, readiness.Blockers...)

	projection, err := enginestatus.BuildProjection(
		root,
		change,
		execCtx.Summary,
		change.EvidenceRefs,
		readiness,
		workflowStateLabel,
	)
	if err != nil {
		return statusView{}, err
	}
	view := buildStatusViewBase(
		root,
		"governed",
		change,
		execCtx.Summary,
		projection,
		blockers,
		governedSourceStateFile(root, change),
	)
	profile := buildChangeProfileView(change)
	view.QualityMode = profile.QualityMode
	view.WorkflowPreset = presetFields.WorkflowPreset
	view.SuggestedWorkflowPreset = presetFields.SuggestedWorkflowPreset
	view.EffectiveWorkflowPreset = presetFields.EffectiveWorkflowPreset
	view.PresetConfirmationPending = presetFields.PresetConfirmationPending
	view.PresetUpgradeReasons = presetFields.PresetUpgradeReasons
	view.GovernanceForecast = presetFields.GovernanceForecast
	view.AutoPassedStates = append([]model.AutoPassedState(nil), change.LastAutoPassedStates...)
	view.NeedsDiscovery = profile.NeedsDiscovery
	if !change.ContextDependencies.IsEmpty() {
		deps := change.ContextDependencies
		view.ContextDependencies = &deps
	}
	view.SelectedPriorContext, view.UnresolvedDependencies = buildSelectedPriorContext(root, change.ContextDependencies)
	applyGovernanceSurfaceToStatus(readiness, &view)
	view.Diagnostics = append([]string(nil), projection.Diagnostics...)
	view.Narrative = buildStatusNarrative(view)
	return view, nil
}

func buildStatusViewBase(
	root string,
	execMode string,
	change model.Change,
	executionSummary *model.ExecutionSummary,
	projection enginestatus.Projection,
	blockers []model.ReasonCode,
	sourceStateFile string,
) statusView {
	view := statusView{
		ExecutionMode:    execMode,
		Slug:             change.Slug,
		Phase:            model.PhaseFor(change.CurrentState),
		LifecycleStatus:  string(change.Status),
		CurrentState:     change.CurrentState,
		IntakeSubStep:    change.IntakeSubStep,
		PlanSubStep:      change.PlanSubStep,
		PlanningNote:     planningNote(change.CurrentState, change.PlanSubStep),
		NextReadyActions: projectNextReadyActions(change.CurrentState),
		SummaryBlockers:  append([]model.ReasonCode(nil), projection.SummaryBlockers...),
		Blockers:         model.NormalizeReasonCodes(blockers),
		Progress:         mapStatusProgress(projection.Progress),
		EvidenceFreshness: projectFreshnessForExecMode(
			root,
			change,
			executionSummary,
			blockers,
		),
		SourceStateFile:  sourceStateFile,
		EvidencePointers: buildEvidencePointers(projection.EvidenceInventory, collectSkillVerificationPointers(root, change.Slug)),
		GateStatus:       projection.GateStatus,
		ArtifactDAG:      mapArtifactDAGNodes(projection.ArtifactDAG),
	}
	view.Narrative = buildStatusNarrative(view)
	return view
}

// governedSourceStateFile returns the display path for the authoritative
// change.yaml, resolving worktree-bound bundles when necessary.
func governedSourceStateFile(root string, change model.Change) string {
	if paths, err := state.ResolveChangePaths(root, change); err == nil {
		return state.DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "change.yaml"))
	}
	return filepath.Join("artifacts", "changes", change.Slug, "change.yaml")
}

func buildStatusNarrative(view statusView) string {
	stateLabel := workflowStateLabel(view.CurrentState, view.IntakeSubStep, view.PlanSubStep)
	recoverySuffix := ""
	if note := planningNote(view.CurrentState, view.PlanSubStep); note != "" {
		recoverySuffix = " " + note
	}
	switch {
	case len(view.Blockers) > 0:
		return "Blocked in " + stateLabel + "." + recoverySuffix + " Resolve the current blockers before running the next lifecycle action."
	case len(view.SelectedPriorContext) > 0:
		return "Active in " + stateLabel + "." + recoverySuffix + " Prior archived context was loaded selectively to support the next action."
	default:
		return "Active in " + stateLabel + "." + recoverySuffix + " Continue with the next lifecycle action."
	}
}

func buildMultiChangeSummaryView(changes []model.Change) multiChangeSummaryView {
	entries := make([]multiChangeSummaryEntry, 0, len(changes))
	for _, change := range changes {
		entry := multiChangeSummaryEntry{
			Slug:         change.Slug,
			Description:  change.Description,
			CurrentState: string(change.CurrentState),
			WorktreePath: change.WorktreePath,
		}
		entry.ExecMode = governedExecutionMode
		entries = append(entries, entry)
	}

	return multiChangeSummaryView{
		ExecutionMode: "multi_active",
		ActiveCount:   len(entries),
		ActiveChanges: entries,
		Hint:          "Use `--change <slug>` to interact with a specific change.",
	}
}

func buildEvidencePointers(inventory enginestatus.EvidenceInventory, extraNonTask map[string]string) statusEvidencePointers {
	taskPointers := map[string]string{}
	for _, ref := range inventory.TaskEvidence {
		taskPointers[ref.Key] = ref.Path
	}

	combinedNonTask := map[string]string{}
	for _, ref := range inventory.NonTaskEvidence {
		combinedNonTask[ref.Key] = ref.Path
	}
	for key, value := range extraNonTask {
		combinedNonTask[key] = value
	}

	keys := make([]string, 0, len(combinedNonTask))
	for key := range combinedNonTask {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	nonTaskPointers := map[string]string{}
	for _, key := range keys {
		nonTaskPointers[key] = combinedNonTask[key]
	}

	return statusEvidencePointers{TaskEvidence: taskPointers, NonTaskEvidence: nonTaskPointers}
}

func gateStatusFromEvaluations(evaluations map[gate.GateID]gate.GateEvaluation) map[string]model.GateRecord {
	return enginestatus.GateStatusFromEvaluations(evaluations)
}

func collectSkillVerificationPointers(root, slug string) map[string]string {
	records, err := state.ListVerifications(root, slug)
	if err != nil {
		return nil
	}
	keys := make([]string, 0, len(records))
	for skillName := range records {
		keys = append(keys, skillName)
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		return nil
	}
	out := make(map[string]string, len(keys))
	for _, skillName := range keys {
		key := "skill." + skillName
		out[key] = state.DisplayPath(root, state.VerificationFilePath(root, slug, skillName))
	}
	return out
}

func mapStatusProgress(progress *enginestatus.Progress) *statusProgress {
	if progress == nil {
		return nil
	}
	mapped := &statusProgress{
		Percentage:        progress.Percentage,
		StageIndex:        progress.StageIndex,
		StageTotal:        progress.StageTotal,
		StageName:         progress.StageName,
		TasksCompleted:    progress.TasksCompleted,
		TasksTotal:        progress.TasksTotal,
		RunSummaryVersion: progress.RunSummaryVersion,
	}
	if len(progress.TasksByVerdict) > 0 {
		mapped.TasksByVerdict = make(map[string]int, len(progress.TasksByVerdict))
		for key, value := range progress.TasksByVerdict {
			mapped.TasksByVerdict[key] = value
		}
	}
	return mapped
}

func mapArtifactDAGNodes(nodes []enginestatus.ArtifactNode) []artifactDAGNode {
	if len(nodes) == 0 {
		return nil
	}
	mapped := make([]artifactDAGNode, 0, len(nodes))
	for _, node := range nodes {
		mapped = append(mapped, artifactDAGNode{
			Name:      node.Name,
			State:     node.State,
			DependsOn: append([]string(nil), node.DependsOn...),
			Ready:     node.Ready,
		})
	}
	return mapped
}
