package cmd

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// buildNextContextByMode populates change-specific fields on the view
// (state, lifecycle, artifacts, checkpoints).
// Returns the loaded Change plus its execution context so downstream next-path
// consumers can reuse the same execution-summary read.
func buildNextContextByMode(root string, view *nextView, ref changeRef, resumeResponse string, preview bool) (*model.Change, *executionContext, error) {
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
	view.NeedsDiscovery = profile.NeedsDiscovery
	view.InputContext.Slug = change.Slug
	if paths, err := state.ResolveChangePaths(root, change); err == nil {
		view.InputContext.WorkspaceRoot = paths.WorkspaceRoot
		view.InputContext.ArtifactBundle = state.DisplayPath(root, paths.GovernedBundleDir)
		view.InputContext.CodebaseMapDir = state.DisplayPath(paths.WorkspaceRoot, paths.CodebaseMapDir)
		view.InputContext.CodebaseMapDocs = artifact.CodebaseMapDisplayDocs(paths.WorkspaceRoot, paths.CodebaseMapDir)
	} else {
		return nil, nil, err
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
	if err := buildResumeCheckpoint(root, &change, execCtx, view, resumeResponse, preview); err != nil {
		return nil, nil, err
	}
	return &change, &execCtx, nil
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

	if readiness.ArtifactProjection == nil || len(readiness.ArtifactProjection.Nodes) == 0 {
		return
	}
	view.InputContext.ArtifactStatus = make(map[string]string, len(readiness.ArtifactProjection.Nodes))
	for _, node := range readiness.ArtifactProjection.Nodes {
		artifactID := strings.TrimSuffix(node.Name, filepath.Ext(node.Name))
		view.InputContext.ArtifactStatus[artifactID] = node.State
	}
}

// buildResumeCheckpoint handles active checkpoint validation and wave execution
// resume checkpoint construction for governed changes.
func buildResumeCheckpoint(root string, change *model.Change, execCtx executionContext, view *nextView, resumeResponse string, preview bool) error {
	if !preview {
		if change.ActiveCheckpoint != nil {
			if err := validateResumeResponse(change.ActiveCheckpoint, resumeResponse); err != nil {
				return err
			}
			view.InputContext.ResumeCheckpoint = &resumeCheckpoint{
				RunSummaryVersion:   execCtx.LatestRunVersion,
				PausedTaskID:        change.ActiveCheckpoint.PausedTaskID,
				CheckpointType:      change.ActiveCheckpoint.CheckpointType,
				UserResponsePayload: resumeResponse,
			}
			// Non-preview `next` consumes the checkpoint only after the full
			// next view succeeds so failed readiness/projection passes preserve
			// the pending resume contract.
			view.consumeActiveCheckpoint = true
		} else if strings.TrimSpace(resumeResponse) != "" {
			return newPreconditionError(
				"no_active_checkpoint",
				"--resume-response provided but no active checkpoint exists",
				"Remove --resume-response when no checkpoint is pending.",
				"",
				nil,
			)
		}
	} else if change.ActiveCheckpoint != nil {
		view.InputContext.ResumeCheckpoint = &resumeCheckpoint{
			RunSummaryVersion: execCtx.LatestRunVersion,
			PausedTaskID:      change.ActiveCheckpoint.PausedTaskID,
			CheckpointType:    change.ActiveCheckpoint.CheckpointType,
		}
	}

	if change.CurrentState == model.StateS2Execute && execCtx.Ready && view.InputContext.ResumeCheckpoint == nil {
		completed := progression.BuildResumeCompletedTasks(*execCtx.Summary)
		hasCompletedTasks := len(execCtx.Summary.CompletedTasks) > 0
		if hasCompletedTasks {
			ids := make([]string, 0, len(completed))
			for id := range completed {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			freshness := projectFreshnessForExecMode(
				root,
				*change,
				execCtx.Summary,
				nil,
			)
			view.InputContext.ResumeCheckpoint = &resumeCheckpoint{
				RunSummaryVersion: execCtx.LatestRunVersion,
				CompletedTaskIDs:  ids,
				Freshness:         freshness,
			}
		}
	}
	return nil
}
