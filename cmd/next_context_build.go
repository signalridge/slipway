package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// codebaseMapStatusForContext returns the freshness status and per-doc states
// for a change's codebase map, reusing artifact.AssessCodebaseMapDocs against
// the bound worktree (paths.WorkspaceRoot) so the next/run handoff and the
// consume-time advisory all read one assessment (REQ-009). It short-circuits to
// "missing" when the worktree's codebase map directory is absent, skipping the
// bounded repo walk AssessCodebaseMapDocs would otherwise run on every
// next/run invocation. An absent map dir always assesses "missing", so the
// short-circuit result is identical to the full assessment. docs supplies the
// per-doc keys (the same short keys as codebase_map_docs) for the missing case.
func codebaseMapStatusForContext(workspaceRoot string, docs map[string]string) (string, map[string]string) {
	dir := state.CodebaseMapDir(workspaceRoot)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return artifact.CodebaseMapStatusMissing, missingCodebaseMapDocStates(docs)
	}
	assessment, err := artifact.AssessCodebaseMapDocs(workspaceRoot)
	if err != nil {
		// Never drop the default freshness signal: report the empty-map state
		// rather than an omitted field when the assessment cannot complete.
		return artifact.CodebaseMapStatusMissing, missingCodebaseMapDocStates(docs)
	}
	return assessment.Status, assessment.DocStates
}

func missingCodebaseMapDocStates(docs map[string]string) map[string]string {
	if len(docs) == 0 {
		return nil
	}
	states := make(map[string]string, len(docs))
	for key := range docs {
		states[key] = artifact.CodebaseMapStatusMissing
	}
	return states
}

// buildNextContextByMode populates change-specific fields on the view
// (state, lifecycle, artifacts, and execution resume context).
// Returns the loaded Change plus its execution context so downstream next-path
// consumers can reuse the same execution-summary read.
func buildNextContextByMode(
	root string,
	view *nextView,
	ref changeRef,
) (*model.Change, *executionContext, error) {
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return nil, nil, err
	}

	view.CurrentState = change.CurrentState
	view.PlanSubStep = change.PlanSubStep
	view.PlanningNote = planningNote(change.CurrentState, change.PlanSubStep)
	view.LifecycleStatus = string(change.Status)
	view.InputContext.Description = change.Description

	view.ExecutionMode = governedExecutionMode
	profile := buildChangeProfileView(change)
	view.QualityMode = profile.QualityMode
	view.WorkflowProfile = profile.WorkflowProfile
	view.NeedsDiscovery = profile.NeedsDiscovery
	view.InputContext.Slug = change.Slug
	if paths, err := state.ResolveChangePaths(root, change); err == nil {
		view.InputContext.WorkspaceRoot = paths.WorkspaceRoot
		view.InputContext.ArtifactBundle = state.DisplayPath(root, paths.GovernedBundleDir)
		view.InputContext.CodebaseMapDir = state.DisplayPath(paths.WorkspaceRoot, paths.CodebaseMapDir)
		view.InputContext.CodebaseMapDocs = artifact.CodebaseMapDisplayDocs(paths.WorkspaceRoot, paths.CodebaseMapDir)
		view.InputContext.CodebaseMapStatus, view.InputContext.CodebaseMapDocStates = codebaseMapStatusForContext(paths.WorkspaceRoot, view.InputContext.CodebaseMapDocs)
		view.InputContext.HandoffContext = buildHandoffContext(root, change, paths)
	} else {
		return nil, nil, err
	}
	if !change.ProjectContext.IsZero() {
		projectContext := change.ProjectContext
		view.InputContext.ProjectContext = &projectContext
	}
	if !change.ContextDependencies.IsEmpty() {
		deps := change.ContextDependencies
		view.InputContext.ContextDependencies = &deps
	}
	view.InputContext.SelectedPriorContext, view.InputContext.UnresolvedDependencies = buildSelectedPriorContext(root, change.ContextDependencies)

	execCtx, err := loadExecutionContext(root, change)
	if err != nil {
		return nil, nil, err
	}
	if err := buildExecutionResumeContext(root, change, execCtx, view); err != nil {
		return nil, nil, err
	}
	return &change, &execCtx, nil
}

func buildHandoffContext(root string, change model.Change, paths state.ResolvedChangePaths) *handoffContextView {
	changeAuthority := state.DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "change.yaml"))
	configPath, err := state.ConfigPathForChange(root, change)
	if err != nil {
		configPath = ""
	}
	lifecycleLog, err := state.LifecycleEventLogPath(root, change)
	if err != nil {
		lifecycleLog = ""
	}
	requiredReads := []string{changeAuthority}
	readRefs := []handoffReadRef{{
		Kind:   "authority",
		Path:   changeAuthority,
		Reason: "current change lifecycle/runtime authority",
	}}
	if paths.CodebaseMapDir != "" {
		displayPath := state.DisplayPath(root, paths.CodebaseMapDir)
		requiredReads = append(requiredReads, displayPath)
		readRefs = append(readRefs, handoffReadRef{
			Kind:   "artifact",
			Path:   displayPath,
			Reason: "durable codebase map for bounded repository context",
		})
	}
	if configPath != "" {
		displayPath := state.DisplayPath(root, configPath)
		requiredReads = append(requiredReads, displayPath)
		readRefs = append(readRefs, handoffReadRef{
			Kind:   "config",
			Path:   displayPath,
			Reason: "project-local governance and handoff configuration",
		})
	}
	eventLog := displayOptionalPath(root, lifecycleLog)
	if eventLog != "" {
		readRefs = append(readRefs, handoffReadRef{
			Kind:   "trace",
			Path:   eventLog,
			Reason: "append-only lifecycle audit trace for this change",
		})
	}
	policyPacks, policyRefs, policyReads := buildPolicyPackHandoff(root, configPath)
	readRefs = append(readRefs, policyRefs...)
	requiredReads = append(requiredReads, policyReads...)
	return &handoffContextView{
		WorkflowProfile:   string(change.EffectiveWorkflowProfile()),
		ContextPolicy:     "bounded_references_only",
		Trace:             buildHandoffTrace(change, eventLog),
		ContextBudget:     &handoffBudgetHintView{Mode: "compact", MaxInlineBytes: 12000},
		ReadRefs:          readRefs,
		PolicyPacks:       policyPacks,
		Risk:              buildHandoffRisk(change, nil),
		ChangeAuthority:   changeAuthority,
		LifecycleEventLog: eventLog,
		ConfigPath:        displayOptionalPath(root, configPath),
		RequiredReads:     requiredReads,
	}
}

func buildPolicyPackHandoff(root, configPath string) ([]handoffPolicyPack, []handoffReadRef, []string) {
	if strings.TrimSpace(configPath) == "" {
		return nil, nil, nil
	}
	cfg, err := model.LoadConfig(configPath)
	if err != nil || len(cfg.Governance.PolicyPacks) == 0 {
		return nil, nil, nil
	}

	packs := make([]handoffPolicyPack, 0, len(cfg.Governance.PolicyPacks))
	refs := make([]handoffReadRef, 0, len(cfg.Governance.PolicyPacks))
	reads := make([]string, 0, len(cfg.Governance.PolicyPacks))
	for _, pack := range cfg.Governance.PolicyPacks {
		packPath := strings.TrimSpace(pack.Path)
		if packPath == "" {
			continue
		}
		if !filepath.IsAbs(packPath) {
			packPath = filepath.Join(filepath.Dir(configPath), packPath)
		}
		displayPath := state.DisplayPath(root, packPath)
		mode := string(pack.Mode)
		if mode == "" {
			mode = string(model.ControlModeAdvisory)
		}
		view := handoffPolicyPack{
			Name: strings.TrimSpace(pack.Name),
			Path: displayPath,
			Mode: mode,
		}
		if parsed, loadErr := governance.LoadAdvisoryPolicyPack(pack.Name, packPath); loadErr == nil {
			view.SchemaVersion = parsed.SchemaVersion
			view.AdvisoryRules = append([]string(nil), parsed.AdvisoryRules...)
			view.ArtifactRequirements = append([]string(nil), parsed.ArtifactRequirements...)
			view.RecommendedReviewers = append([]string(nil), parsed.RecommendedReviewers...)
			view.Terminology = append([]string(nil), parsed.Terminology...)
		}
		packs = append(packs, view)
		reads = append(reads, displayPath)
		refs = append(refs, handoffReadRef{
			Kind:   "policy_pack",
			Path:   displayPath,
			Reason: "project-local advisory governance policy pack",
		})
	}
	return packs, refs, reads
}

func buildHandoffTrace(change model.Change, eventLog string) *handoffTraceView {
	return &handoffTraceView{
		CorrelationID: "next-" + strings.ToLower(strings.TrimSpace(change.Slug)) + "-" + strings.ToLower(string(change.CurrentState)),
		EventLog:      eventLog,
	}
}

func buildHandoffRisk(change model.Change, controls []string) *handoffRiskView {
	risk := &handoffRiskView{
		GuardrailDomain: strings.TrimSpace(change.GuardrailDomain),
		WorkflowProfile: string(change.EffectiveWorkflowProfile()),
		Controls:        append([]string(nil), controls...),
		Hints:           workflowProfileRiskHints(change.EffectiveWorkflowProfile()),
	}
	if risk.GuardrailDomain == "" && len(risk.Controls) == 0 && len(risk.Hints) == 0 {
		return nil
	}
	return risk
}

func workflowProfileRiskHints(profile model.WorkflowProfile) []string {
	switch profile.Effective() {
	case model.WorkflowProfileDocs:
		return []string{"docs profile emphasizes artifact consistency; sensitive-domain controls still override profile shortcuts"}
	case model.WorkflowProfileResearch:
		return []string{"research profile emphasizes discovery evidence and does not imply implementation approval"}
	case model.WorkflowProfileConfig:
		return []string{"config profile emphasizes rollback, safety, and supply-chain context"}
	case model.WorkflowProfileMeta:
		return []string{"meta profile changes Slipway governance surfaces; preserve schema and generated surface compatibility"}
	default:
		return nil
	}
}

func displayOptionalPath(root, target string) string {
	if strings.TrimSpace(target) == "" {
		return ""
	}
	return state.DisplayPath(root, target)
}

func applyReadinessToNextContext(view *nextView, readiness progression.GovernanceReadiness) {
	if view == nil {
		return
	}

	gateStatus := gateStatusFromEvaluations(readiness.GateEvaluations)
	if len(gateStatus) > 0 {
		view.InputContext.GateStatus = make(map[string]string, len(gateStatus))
		for id, gate := range gateStatus {
			view.InputContext.GateStatus[id] = string(gate.Status)
		}
	}

	if view.InputContext.HandoffContext != nil {
		view.InputContext.HandoffContext.Risk = buildHandoffRiskForReadiness(view.InputContext.HandoffContext.Risk, readiness.ActiveControls)
	}

	if readiness.ArtifactProjection == nil || len(readiness.ArtifactProjection.Nodes) == 0 {
		return
	}
	view.InputContext.ArtifactStatus = make(map[string]string, len(readiness.ArtifactProjection.Nodes))
	for _, node := range readiness.ArtifactProjection.Nodes {
		artifactID := strings.TrimSuffix(node.Name, filepath.Ext(node.Name))
		view.InputContext.ArtifactStatus[artifactID] = node.State
	}
}

func buildHandoffRiskForReadiness(existing *handoffRiskView, controls []model.ControlActivation) *handoffRiskView {
	controlIDs := make([]string, 0, len(controls))
	for _, control := range controls {
		if !control.Active || strings.TrimSpace(string(control.ControlID)) == "" {
			continue
		}
		controlIDs = append(controlIDs, string(control.ControlID))
	}
	slices.Sort(controlIDs)
	if existing == nil {
		if len(controlIDs) == 0 {
			return nil
		}
		return &handoffRiskView{Controls: controlIDs}
	}
	existing.Controls = controlIDs
	return existing
}

// buildExecutionResumeContext attaches resumable wave execution progress for
// governed changes without advancing or repairing runtime state.
func buildExecutionResumeContext(
	root string,
	change model.Change,
	execCtx executionContext,
	view *nextView,
) error {
	completedTaskIDs, freshness, resumeWaveIndex, err := buildExecutionResumeProgress(root, change, execCtx)
	if err != nil {
		return err
	}

	if change.CurrentState == model.StateS2Implement && execCtx.Ready {
		if len(completedTaskIDs) > 0 {
			view.InputContext.ExecutionResume = &executionResumeContext{
				RunSummaryVersion: execCtx.LatestRunVersion,
				CompletedTaskIDs:  completedTaskIDs,
				Freshness:         freshness,
				ResumeWaveIndex:   resumeWaveIndex,
			}
		}
	}
	return nil
}

func buildExecutionResumeProgress(
	root string,
	change model.Change,
	execCtx executionContext,
) ([]string, string, int, error) {
	if !execCtx.Ready || execCtx.Summary == nil || len(execCtx.Summary.CompletedTasks) == 0 {
		return nil, "", 0, nil
	}

	completed := progression.BuildResumeCompletedTasks(*execCtx.Summary)
	ids := make([]string, 0, len(completed))
	for id := range completed {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	freshness := projectFreshnessForExecMode(
		root,
		change,
		execCtx.Summary,
		execCtx.SummaryBlockers,
	)

	resumeWaveIndex := 0
	if execCtx.LatestRunVersion > 0 {
		waveCtx, err := loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, "next")
		if err != nil {
			if !resumableWavePlanHasStructuralDrift(root, change) {
				return nil, "", 0, err
			}
			waveCtx = nil
		}
		if waveCtx != nil {
			resumeWaveIndex = state.ResumeWaveIndex(waveCtx.Plan, waveCtx.Runs)
		}
	}

	return ids, freshness, resumeWaveIndex, nil
}
