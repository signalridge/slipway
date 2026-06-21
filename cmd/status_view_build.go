package cmd

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/progression"
	enginestatus "github.com/signalridge/slipway/internal/engine/status"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func buildStatusViewFromChange(root string, change model.Change) (statusView, error) {
	return buildGovernedStatusView(root, change)
}

func buildGovernedStatusView(root string, change model.Change) (statusView, error) {
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
			WorkflowProfile:           profile.WorkflowProfile,
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

	execCtx, err := loadExecutionContext(root, change)
	if err != nil {
		return statusView{}, err
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
	readiness.GateEvaluations = defaultVisibleGateEvaluations(change, readiness.GateEvaluations)
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
	primaryAction, waveBlockers, waveDiagnostics := statusPrimaryAction(root, change, execCtx)
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
	view.WorkflowProfile = profile.WorkflowProfile
	view.WorkflowPreset = presetFields.WorkflowPreset
	view.SuggestedWorkflowPreset = presetFields.SuggestedWorkflowPreset
	view.EffectiveWorkflowPreset = presetFields.EffectiveWorkflowPreset
	view.PresetConfirmationPending = presetFields.PresetConfirmationPending
	view.PresetUpgradeReasons = presetFields.PresetUpgradeReasons
	view.GovernanceForecast = presetFields.GovernanceForecast
	if !change.InterruptedExecutionAt.IsZero() {
		view.InterruptedExecutionAt = change.InterruptedExecutionAt.UTC().Format(time.RFC3339)
	}
	view.NextReadyActions = projectNextReadyActionsWithPrimary(change.CurrentState, primaryAction)
	view.AutoPassedStates = append([]model.AutoPassedState(nil), change.LastAutoPassedStates...)
	view.NeedsDiscovery = profile.NeedsDiscovery
	view.ScopeContract = buildScopeContractView(readiness.ScopeContract)
	if change.CurrentState == model.StateS3Review {
		view.SelectedReviewSkills = selectedReviewSkillsFromReadiness(readiness, change.EffectiveWorkflowProfile())
	}
	if !change.ContextDependencies.IsEmpty() {
		deps := change.ContextDependencies
		view.ContextDependencies = &deps
	}
	view.SelectedPriorContext, view.UnresolvedDependencies = buildSelectedPriorContext(root, change.ContextDependencies)
	applyGovernanceSurfaceToStatus(readiness, &view)
	if readiness.ArtifactProjection != nil && len(readiness.ArtifactProjection.Amendments) > 0 {
		view.ArtifactAmendments = append([]artifact.AmendmentEvent(nil), readiness.ArtifactProjection.Amendments...)
	}
	view.Blockers = model.NormalizeReasonCodes(append(view.Blockers, waveBlockers...))
	applyDoneReadyProjection(change, readiness.GateEvaluations, &view)
	view.Recovery = model.BuildRecovery(view.Blockers)
	view.FreshnessDiagnostics = attachFreshnessDiagnostics(readiness.FreshnessDiagnostics)
	view.Diagnostics = append([]string(nil), projection.Diagnostics...)
	view.Diagnostics = append(view.Diagnostics, waveDiagnostics...)
	timeline, timelineErr := buildStatusTimeline(root, change, 20)
	if timelineErr != nil {
		view.Diagnostics = append(view.Diagnostics, "lifecycle_event_log_unreadable: "+timelineErr.Error())
	} else {
		view.Timeline = timeline
	}
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
		ArtifactDAG:      mapArtifactDAGNodesForGateStatus(projection.ArtifactDAG, projection.GateStatus),
	}
	view.Narrative = buildStatusNarrative(view)
	return view
}

func statusPrimaryAction(root string, change model.Change, execCtx executionContext) (string, []model.ReasonCode, []string) {
	switch change.CurrentState {
	case model.StateS2Implement:
		return projectStatusExecutionAction(root, change, execCtx)
	case model.StateS3Review:
		return statusWaveExecutionRepairAction(root, change, execCtx)
	default:
		return "", nil, nil
	}
}

func projectStatusExecutionAction(root string, change model.Change, execCtx executionContext) (string, []model.ReasonCode, []string) {
	if change.ActiveCheckpoint != nil {
		if err := validateActiveCheckpointAuthority(root, change, execCtx, "status"); err != nil {
			blockers, diagnostics := statusWaveExecutionIssues(err)
			return "repair", blockers, diagnostics
		}
		return `run --resume-response "<response>"`, nil, nil
	}
	if !execCtx.Ready || execCtx.LatestRunVersion < 1 {
		return "run", nil, nil
	}

	resumeWaveIndex, err := loadResumableWaveExecution(root, change, execCtx, "status")
	switch {
	case err != nil:
		blockers, diagnostics := statusWaveExecutionIssues(err)
		return "repair", blockers, diagnostics
	case resumeWaveIndex > 0:
		return "run --resume", nil, nil
	default:
		return "run", nil, nil
	}
}

func statusWaveExecutionRepairAction(root string, change model.Change, execCtx executionContext) (string, []model.ReasonCode, []string) {
	if !execCtx.Ready || execCtx.LatestRunVersion < 1 {
		return "", nil, nil
	}
	if _, err := loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, "status"); err != nil {
		blockers, diagnostics := statusWaveExecutionIssues(err)
		return "repair", blockers, diagnostics
	}
	return "", nil, nil
}

func statusWaveExecutionIssues(err error) ([]model.ReasonCode, []string) {
	if err == nil {
		return nil, nil
	}

	diagnostics := []string{err.Error()}
	if cliErr := asCLIError(err); cliErr != nil {
		if cliErr.Remediation != "" {
			diagnostics = append(diagnostics, cliErr.Remediation)
		}
		if len(cliErr.Reasons) > 0 {
			return append([]model.ReasonCode(nil), cliErr.Reasons...), diagnostics
		}
		return []model.ReasonCode{statusReasonFromCLIError(cliErr)}, diagnostics
	}

	return []model.ReasonCode{model.NewReasonCode("wave_execution_unavailable", err.Error())}, diagnostics
}

func statusReasonFromCLIError(cliErr *CLIError) model.ReasonCode {
	code := strings.TrimSpace(cliErr.ErrorCode)
	detail := strings.TrimSpace(cliErr.Message)
	if model.IsCanonicalReasonCode(code) {
		return model.NewReasonCode(code, detail)
	}
	if code != "" {
		if detail != "" {
			detail = code + ": " + detail
		} else {
			detail = code
		}
	}
	return model.NewReasonCode("wave_execution_unavailable", detail)
}

// governedSourceStateFile returns the display path for the authoritative
// change.yaml, resolving worktree-bound bundles when necessary.
func governedSourceStateFile(root string, change model.Change) string {
	if paths, err := state.ResolveChangePaths(root, change); err == nil {
		return state.DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "change.yaml"))
	}
	return filepath.Join("artifacts", "changes", change.Slug, "change.yaml")
}

func buildStatusTimeline(root string, change model.Change, limit int) ([]statusTimelineEvent, error) {
	events, err := state.ReadLifecycleEvents(root, change)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	events = markDuplicateTransitionReplays(events)
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	timeline := make([]statusTimelineEvent, 0, len(events))
	for _, event := range events {
		entry := statusTimelineEvent{
			EventID:   event.EventID,
			Command:   event.Command,
			EventType: event.EventType,
			Result:    event.Result,
			FromState: event.BeforeState.Canonical(),
			ToState:   event.AfterState.Canonical(),
			GateID:    event.GateID,
			ControlID: event.ControlID,
			SkillID:   event.SkillID,
			Blockers:  append([]model.ReasonCode(nil), event.Blockers...),
		}
		if !event.OccurredAt.IsZero() {
			entry.OccurredAt = event.OccurredAt.UTC().Format(time.RFC3339)
		}
		timeline = append(timeline, entry)
	}
	return timeline, nil
}

func markDuplicateTransitionReplays(events []state.LifecycleEvent) []state.LifecycleEvent {
	if len(events) == 0 {
		return nil
	}
	presented := make([]state.LifecycleEvent, 0, len(events))
	var lastTransition state.LifecycleEvent
	hasLastTransition := false
	for _, event := range events {
		next := event
		if hasLastTransition && duplicateLifecycleTransition(lastTransition, event) {
			next.EventType = "state.transition.replayed"
			next.Result = "replayed"
			presented = append(presented, next)
			continue
		}
		presented = append(presented, next)
		if event.EventType == "state.transitioned" {
			lastTransition = event
			hasLastTransition = true
		}
	}
	return presented
}

func duplicateLifecycleTransition(previous, current state.LifecycleEvent) bool {
	if previous.EventType != "state.transitioned" || current.EventType != "state.transitioned" {
		return false
	}
	if previous.Command != current.Command ||
		previous.Action != current.Action ||
		previous.Reason != current.Reason ||
		previous.Result != current.Result ||
		previous.BeforeState.Canonical() != current.BeforeState.Canonical() ||
		previous.AfterState.Canonical() != current.AfterState.Canonical() ||
		previous.BeforeSubStep != current.BeforeSubStep ||
		previous.AfterSubStep != current.AfterSubStep ||
		previous.GateID != current.GateID ||
		previous.ControlID != current.ControlID ||
		previous.SkillID != current.SkillID {
		return false
	}
	if !lifecycleEvidenceRefsEqual(previous.EvidenceRefs, current.EvidenceRefs) {
		return false
	}
	return lifecycleSideEffectsEqual(previous.SideEffects, current.SideEffects)
}

func lifecycleEvidenceRefsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		if right[key] != leftValue {
			return false
		}
	}
	return true
}

func lifecycleSideEffectsEqual(left, right []state.LifecycleSideEffect) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Kind != right[i].Kind || left[i].Detail != right[i].Detail {
			return false
		}
	}
	return true
}

func buildStatusNarrative(view statusView) string {
	stateLabel := workflowStateLabel(view.CurrentState, view.IntakeSubStep, view.PlanSubStep)
	recoverySuffix := ""
	if note := planningNote(view.CurrentState, view.PlanSubStep); note != "" {
		recoverySuffix = " " + note
	}
	interruptSuffix := ""
	if view.InterruptedExecutionAt != "" {
		interruptSuffix = " The last governed execution was interrupted at " + view.InterruptedExecutionAt + "."
	}
	switch {
	case view.Archived:
		if view.ArchivePath != "" {
			return "Archived change is " + view.LifecycleStatus + ". Active lifecycle actions are complete; inspect archived evidence at " + view.ArchivePath + "."
		}
		return "Archived change is " + view.LifecycleStatus + ". Active lifecycle actions are complete."
	case view.DoneReady:
		return "Done-ready in " + stateLabel + ". All governance gates passed; run `slipway done` to finalize."
	case len(view.Blockers) > 0:
		return "Blocked in " + stateLabel + "." + recoverySuffix + interruptSuffix + " Resolve the current blockers before running the next lifecycle action."
	case len(view.SelectedPriorContext) > 0:
		return "Active in " + stateLabel + "." + recoverySuffix + interruptSuffix + " Prior archived context was loaded selectively to support the next action."
	default:
		return "Active in " + stateLabel + "." + recoverySuffix + interruptSuffix + " Continue with the next lifecycle action."
	}
}

func applyDoneReadyProjection(change model.Change, evaluations map[gate.GateID]gate.GateEvaluation, view *statusView) {
	if view == nil || change.CurrentState != model.StateS3Review {
		return
	}
	for _, blocker := range view.Blockers {
		if blocker.Severity == model.ReasonSeverityError {
			return
		}
	}
	evaluation, ok := evaluations[gate.GateShip]
	if !ok || evaluation.Status != model.GateStatusApproved {
		return
	}
	view.DoneReady = true
	view.Blockers = appendReasonCodes(
		view.Blockers,
		[]model.ReasonCode{model.NewReasonCode("run_slipway_done_to_finalize", "")},
	)
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

func defaultVisibleGateEvaluations(change model.Change, evaluations map[gate.GateID]gate.GateEvaluation) map[gate.GateID]gate.GateEvaluation {
	if len(evaluations) == 0 {
		return nil
	}
	if defaultSurfaceShowsShipGate(change.CurrentState) {
		return evaluations
	}

	visible := make(map[gate.GateID]gate.GateEvaluation, len(evaluations))
	for id, evaluation := range evaluations {
		if id == gate.GateShip {
			continue
		}
		visible[id] = evaluation
	}
	if len(visible) == 0 {
		return nil
	}
	return visible
}

func defaultSurfaceShowsShipGate(state model.WorkflowState) bool {
	switch state {
	case model.StateS3Review, model.StateDone:
		return true
	default:
		return false
	}
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
		CurrentWaveIndex:  progress.CurrentWaveIndex,
		CompletedWaves:    progress.CompletedWaves,
		TotalWaves:        progress.TotalWaves,
		TasksCompleted:    progress.TasksCompleted,
		TasksTotal:        progress.TasksTotal,
		RunSummaryVersion: progress.RunSummaryVersion,
	}
	if len(progress.WavesByVerdict) > 0 {
		mapped.WavesByVerdict = make(map[string]int, len(progress.WavesByVerdict))
		for key, value := range progress.WavesByVerdict {
			mapped.WavesByVerdict[key] = value
		}
	}
	if len(progress.TasksByVerdict) > 0 {
		mapped.TasksByVerdict = make(map[string]int, len(progress.TasksByVerdict))
		for key, value := range progress.TasksByVerdict {
			mapped.TasksByVerdict[key] = value
		}
	}
	return mapped
}

func mapArtifactDAGNodesForGateStatus(nodes []enginestatus.ArtifactNode, gateStatus map[string]model.GateRecord) []artifactDAGNode {
	if len(nodes) == 0 {
		return nil
	}
	mapped := make([]artifactDAGNode, 0, len(nodes))
	for _, node := range nodes {
		blockingReason := artifactNodeBlockingReason(node, gateStatus)
		mapped = append(mapped, artifactDAGNode{
			Name:           node.Name,
			State:          node.State,
			DependsOn:      append([]string(nil), node.DependsOn...),
			Ready:          node.Ready,
			Blocking:       blockingReason != "",
			BlockingReason: blockingReason,
		})
	}
	return mapped
}

func artifactNodeBlockingReason(node enginestatus.ArtifactNode, gateStatus map[string]model.GateRecord) string {
	if node.Ready {
		return ""
	}
	gateIDs := make([]string, 0, len(gateStatus))
	for gateID := range gateStatus {
		gateIDs = append(gateIDs, gateID)
	}
	slices.Sort(gateIDs)
	for _, gateID := range gateIDs {
		record := gateStatus[gateID]
		if record.Status != model.GateStatusBlocked {
			continue
		}
		for _, reason := range record.ReasonCodes {
			switch reason.Code {
			case "artifact_not_ready":
				return gateID
			case "missing_required_artifact":
				if strings.TrimSpace(reason.Detail) == "" || strings.TrimSpace(reason.Detail) == node.Name {
					return gateID + ":" + reason.Code
				}
			}
		}
	}
	return ""
}
