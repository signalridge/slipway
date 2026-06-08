package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	enginestatus "github.com/signalridge/slipway/internal/engine/status"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGovernedStatusViewUsesDiscoveryWorkflowForProgress(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("discovery-change")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	assert.Equal(t, 1, view.Progress.StageIndex)
	assert.Equal(t, 6, view.Progress.StageTotal)
	assert.Equal(t, 20, view.Progress.Percentage)
}

func TestBuildGovernedStatusViewUsesExecutionSummaryForProgress(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("summary-progress")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		CompletedTasks:    []string{"task-a"},
		NonPassTasks:      []string{"task-b"},
		OpenBlockers:      model.ReasonCodesFromSpecs([]string{"task:task-b:lint_failed"}),
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				ChangedFiles: []string{"cmd/status.go"},
				EvidenceRef:  filepath.ToSlash(filepath.Join(state.ChangeDir(root, change.Slug), "evidence", "tasks", "task-a.json")),
				CapturedAt:   time.Now().UTC(),
			},
			{
				TaskID:       "task-b",
				Verdict:      model.TaskVerdictFail,
				ChangedFiles: []string{"cmd/review.go"},
				Blockers:     []model.ReasonCode{model.NewReasonCode("lint_failed", "")},
				CapturedAt:   time.Now().UTC(),
			},
		},
	}))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	assert.Equal(t, 1, view.Progress.TasksCompleted)
	assert.Equal(t, 2, view.Progress.TasksTotal)
	assert.Equal(t, 1, view.Progress.RunSummaryVersion)
	assert.Equal(t, 1, view.Progress.TasksByVerdict["pass"])
	assert.Equal(t, 1, view.Progress.TasksByVerdict["fail"])
}

func TestBuildGovernedStatusViewExposesSummaryBlockersSeparately(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("summary-blockers")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      model.ReasonCodesFromSpecs([]string{"session_isolation_warning:session_id=abc:shared_by=task-a,task-b"}),
		Tasks:             []model.ExecutionTaskSummary{},
	}))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(view.SummaryBlockers), "session_isolation_warning:session_id=abc:shared_by=task-a,task-b")
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "session_isolation_warning:session_id=abc:shared_by=task-a,task-b")
}

func TestBuildGovernedStatusViewPreAuditOmitsShipGateDebt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should omit ship gate debt before verify")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.NotContains(t, view.GateStatus, "G_ship")
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_key_links_missing_target_files")
}

func TestBuildGovernedStatusViewIncludesStaleExecutionEvidenceBlocker(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("stale-execution-summary")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

	staleAt := time.Now().Add(-time.Minute).UTC()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        staleAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:     "task-a",
				Verdict:    model.TaskVerdictPass,
				TaskKind:   model.TaskKindCode,
				CapturedAt: staleAt,
			},
		},
	}))

	change.GuardrailDomain = model.GuardrailDomainExternalAPIContracts
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, "stale", view.EvidenceFreshness)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), state.StaleExecutionEvidenceBlockerToken)
	assert.Contains(t, view.Blockers, model.NewReasonCode(state.StaleExecutionEvidenceBlockerToken, ""))
	require.NotNil(t, view.FreshnessDiagnostics)
	assert.Equal(t, "stale", view.FreshnessDiagnostics.Status)
	require.NotNil(t, view.FreshnessDiagnostics.FirstStaleCause)
	require.NotEmpty(t, view.FreshnessDiagnostics.TaskInputDiffs)
	assert.Equal(t, "task-a", view.FreshnessDiagnostics.TaskInputDiffs[0].TaskID)
	assert.Equal(t, "guardrail_domain", view.FreshnessDiagnostics.TaskInputDiffs[0].Field)
	assert.Contains(t, view.FreshnessDiagnostics.TaskInputDiffs[0].NextAction, "regenerate")
	require.NotNil(t, view.FreshnessDiagnostics.PathAuthority)
	runtimePath := view.FreshnessDiagnostics.PathAuthority.RuntimeEvidencePath
	assert.True(t, filepath.IsAbs(runtimePath))
	assert.True(t, strings.HasSuffix(runtimePath, "/.git/slipway/runtime/changes/stale-execution-summary"), runtimePath)
	assert.Contains(t, view.FreshnessDiagnostics.PathAuthority.GovernedBundlePath, "artifacts/changes/stale-execution-summary")
}

func TestBuildGovernedStatusViewKeepsExecutionSummaryProgressWhenChecklistExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("summary-progress-authority")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`task-a`"+` checklist should not override execution summary
  - target_files: ["cmd/status.go"]
  - task_kind: code
`), 0o644))

	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		CompletedTasks:    []string{"task-a"},
		NonPassTasks:      []string{"task-b"},
		OpenBlockers:      model.ReasonCodesFromSpecs([]string{"task:task-b:lint_failed"}),
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				ChangedFiles: []string{"cmd/status.go"},
				CapturedAt:   time.Now().UTC(),
			},
			{
				TaskID:       "task-b",
				Verdict:      model.TaskVerdictFail,
				ChangedFiles: []string{"cmd/review.go"},
				Blockers:     []model.ReasonCode{model.NewReasonCode("lint_failed", "")},
				CapturedAt:   time.Now().UTC(),
			},
		},
	}))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	assert.Equal(t, 2, view.Progress.TasksTotal)
	assert.Equal(t, 1, view.Progress.RunSummaryVersion)
	assert.Equal(t, 1, view.Progress.TasksByVerdict["pass"])
	assert.Equal(t, 1, view.Progress.TasksByVerdict["fail"])
}

func TestBuildGovernedStatusViewDoesNotUseChecklistProgressWhenExecutionSummaryIsNotReady(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("summary-progress-not-ready")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`task-a`"+` checklist must not stand in for execution summary
  - target_files: ["cmd/status.go"]
  - task_kind: code

- [ ] `+"`task-b`"+` still pending
  - target_files: ["cmd/status.go"]
  - task_kind: verification
`), 0o644))

	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks:             []model.ExecutionTaskSummary{},
	}))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	assert.Equal(t, 0, view.Progress.TasksCompleted)
	assert.Equal(t, 0, view.Progress.TasksTotal)
	assert.Equal(t, 1, view.Progress.RunSummaryVersion)
	assert.Equal(t, "unknown", view.EvidenceFreshness)
}

func TestBuildGovernedStatusViewIgnoresExecutionSummaryOutsideExecutionStates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("stale-plan-summary")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepValidate
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 7,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:      "task-a",
				Verdict:     model.TaskVerdictPass,
				TaskKind:    model.TaskKindCode,
				EvidenceRef: filepath.ToSlash(filepath.Join(state.ChangeDir(root, change.Slug), "evidence", "tasks", "task-a.json")),
				CapturedAt:  time.Now().UTC(),
			},
		},
	}))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	assert.Equal(t, 0, view.Progress.RunSummaryVersion, "plan-state status should ignore stale execution summaries")
	assert.Equal(t, "unknown", view.EvidenceFreshness)
	assert.Empty(t, view.EvidencePointers.TaskEvidence)
}

func TestBuildGovernedStatusViewKeepsPlanAuditInNormalFlow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("plan-finalized")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	assert.Equal(t, 20, view.Progress.Percentage)
	assert.Contains(t, view.NextReadyActions, "next")
}

func TestBuildGovernedStatusViewOnlyRequiresActivePlanningSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("plan-audit-only")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:plan-audit")
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:research-orchestration")
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:scope-confirmation")
}

func TestBuildGovernedStatusViewExposesPlanningRecoveryState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("plan-validate")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepValidate

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, model.PlanSubStepValidate, view.PlanSubStep)
	assert.Contains(t, view.PlanningNote, "recovery-only")
	require.NotNil(t, view.Progress)
	assert.Equal(t, "S1_PLAN/validate", view.Progress.StageName)
	assert.Contains(t, view.Narrative, "recovery-only")
}

func TestBuildGovernedStatusViewDoesNotLeakBundleBlockersBeforeWorktreeBinding(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("execute-no-worktree")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS2Execute

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	// At S2_EXECUTE with NeedsDiscovery=true and no worktree bound,
	// the worktree gate should be the primary blocker.
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "dedicated_worktree_metadata_required")
}

func TestBuildGovernedStatusViewIncludesQualityMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("quality-change")
	change.QualityMode = model.QualityModeDiscuss

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, "discuss", view.QualityMode)
}

func TestBuildGovernedStatusViewIncludesWorkflowPresetAndForecast(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("preset-status")
	change.CurrentState = model.StateS1Plan
	change.WorkflowPreset = model.WorkflowPresetLight
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, "light", view.WorkflowPreset)
	assert.Equal(t, "light", view.EffectiveWorkflowPreset)
	require.NotNil(t, view.GovernanceForecast)
	assert.Equal(t, "light", view.GovernanceForecast.DownstreamLevel)
}

func TestBuildGovernedStatusViewIncludesTaskChecklistAdvisories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("status-advisories")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.ArtifactSchema = model.ArtifactSchemaCore
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Intent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "requirements.md"), []byte(`## Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - depends_on: []
  - target_files: [cmd/status.go]
`), 0o644))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	diagnostics := strings.Join(view.Diagnostics, "\n")
	assert.Contains(t, diagnostics, "plan_dimension_context_missing_task_kind_warning:t-01")
	assert.Contains(t, diagnostics, "plan_dimension_coverage_missing_requirement_warning:REQ-001")
}

func TestBuildGovernedStatusViewIncludesTaskChecklistBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("status-checklist-blockers")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.ArtifactSchema = model.ArtifactSchemaCore
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Intent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "requirements.md"), []byte(`## Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - depends_on: [t-99]
  - target_files: [cmd/status.go]
`), 0o644))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_dependency_unknown:t-01->t-99")
}

func TestBuildGovernedStatusViewIncludesAutoPassedStates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("autopass-status")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	change.LastAutoPassedStates = []model.AutoPassedState{
		{State: model.StateS3Review, Reason: "no_blocking_review_obligations"},
	}
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.Len(t, view.AutoPassedStates, 1)
	assert.Equal(t, model.StateS3Review, view.AutoPassedStates[0].State)
}

func TestBuildGovernedStatusViewUsesResolvedWorktreeEvidencePaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("worktree-evidence")
	worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)

	normalizedWT, err := state.NormalizePath(worktreeRoot)
	require.NoError(t, err)
	change.WorktreePath = normalizedWT
	change.WorktreeBranch = branch
	require.NoError(t, state.SaveChange(root, change))
	writeSkillVerification(t, root, change.Slug, "plan-audit", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	})

	loaded, err := state.LoadChange(root, change.Slug)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, loaded)
	require.NoError(t, err)

	assert.Equal(t, state.DisplayPath(root, filepath.Join(normalizedWT, "artifacts", "changes", change.Slug, "change.yaml")), view.SourceStateFile)
	require.Contains(t, view.EvidencePointers.NonTaskEvidence, "skill.plan-audit")
	assert.Equal(
		t,
		state.DisplayPath(root, filepath.Join(normalizedWT, "artifacts", "changes", change.Slug, "verification", "plan-audit.yaml")),
		view.EvidencePointers.NonTaskEvidence["skill.plan-audit"],
	)
}

func TestForecastDownstreamLevelNeverBelowEffectivePreset(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	// Set min_preset to standard so effective_preset is upgraded.
	cfgPath := state.ConfigPath(root)
	cfg, err := model.LoadConfig(cfgPath)
	require.NoError(t, err)
	cfg.Governance.MinPreset = model.WorkflowPresetStandard
	require.NoError(t, model.SaveConfig(cfgPath, cfg))

	change := model.NewChange("forecast-floor")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS1Plan
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, "light", view.WorkflowPreset, "confirmed preset remains light")
	assert.Equal(t, "standard", view.EffectiveWorkflowPreset, "effective preset upgraded by min_preset")
	require.NotNil(t, view.GovernanceForecast)
	assert.NotEqual(t, "light", view.GovernanceForecast.DownstreamLevel,
		"forecast downstream_level must not be lower than effective_preset")
}

func TestBuildGovernedStatusViewPendingPresetShowsPresetInNextActions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	change := model.NewChange("pending-hint")
	change.SuggestedWorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS1Plan
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.True(t, view.PresetConfirmationPending)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Contains(t, view.NextReadyActions[0], "preset",
		"first next-ready action should mention preset when confirmation is pending")

	assert.Nil(t, view.Progress,
		"progress must not appear when preset is pending")
	assert.Nil(t, view.ArtifactDAG,
		"artifact_dag must not appear when preset is pending")
	assert.Empty(t, view.SourceStateFile,
		"source_state_file must not appear when preset is pending")
	assert.Len(t, view.NextReadyActions, 1,
		"only preset action should be in next_ready_actions when pending")
}

func TestBuildGovernedStatusViewUsesResumeResponseForActiveCheckpoint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should suggest checkpoint resume-response")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.ActiveCheckpoint = &model.ActiveCheckpoint{
		PausedTaskID:    "task-02",
		PausedWaveIndex: 2,
		PausedAt:        time.Now().UTC(),
		CheckpointType:  string(model.CheckpointHumanVerify),
	}
	require.NoError(t, state.SaveChange(root, change))
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending checkpointed wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code
`)))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Equal(t, `run --resume-response "<response>"`, view.NextReadyActions[0])
	assert.Contains(t, renderStatusText(view), `slipway run --resume-response "<response>"`)
}

func TestBuildGovernedStatusViewSuggestsRepairForActiveCheckpointWhenWavePlanIsMissingBeforeExecutionSummaryReady(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should fail closed for checkpoint resume when pre-summary wave plan is missing")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.ActiveCheckpoint = &model.ActiveCheckpoint{
		PausedTaskID:    "task-02",
		PausedWaveIndex: 2,
		PausedAt:        time.Now().UTC(),
		CheckpointType:  string(model.CheckpointHumanVerify),
	}
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Equal(t, "repair", view.NextReadyActions[0])
	assert.Contains(t, strings.Join(view.Diagnostics, "\n"), "wave-plan.yaml")

	found := false
	for _, blocker := range view.Blockers {
		if blocker.Code == "wave_plan_missing" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected status blockers to include wave_plan_missing")
}

func TestBuildGovernedStatusViewSuggestsRepairForActiveCheckpointWhenWaveRunsAreMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should fail closed for checkpoint resume when wave runs are missing")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.ActiveCheckpoint = &model.ActiveCheckpoint{
		PausedTaskID:    "task-02",
		PausedWaveIndex: 2,
		PausedAt:        time.Now().UTC(),
		CheckpointType:  string(model.CheckpointHumanVerify),
	}
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "task-01")
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending checkpointed wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code
`)))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Equal(t, "repair", view.NextReadyActions[0])
	assert.Contains(t, strings.Join(view.Diagnostics, "\n"), "wave run evidence")

	found := false
	for _, blocker := range view.Blockers {
		if blocker.Code == "wave_runs_missing" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected status blockers to include wave_runs_missing")
}

func TestBuildGovernedStatusViewUsesRunResumeForIncompleteWaveExecution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should suggest run resume")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "task-01")
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code
`)))
	materializeWaveExecutionForSummary(t, root, slug)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Equal(t, "run --resume", view.NextReadyActions[0])
	assert.Contains(t, renderStatusText(view), "slipway run --resume")
}

func TestBuildGovernedStatusViewSurfacesInterruptedExecutionContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should surface interrupted execution context")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.InterruptedExecutionAt = time.Date(2026, time.April, 11, 10, 30, 0, 0, time.UTC)
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "task-01")
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code
`)))
	materializeWaveExecutionForSummary(t, root, slug)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Equal(t, "2026-04-11T10:30:00Z", view.InterruptedExecutionAt)
	assert.Equal(t, "run --resume", view.NextReadyActions[0])
	assert.Contains(t, view.Narrative, "interrupted at 2026-04-11T10:30:00Z")
	assert.Contains(t, renderStatusText(view), "Interrupted Execution: 2026-04-11T10:30:00Z")
}

func TestBuildGovernedStatusViewSuggestsRepairWhenWaveRunsAreIncomplete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should fail closed for incomplete wave evidence")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "task-01")
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: code
`)))

	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	runs, err := state.BuildWaveRuns(plan, summary.RunSummaryVersion, summary.Tasks)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.NoError(t, state.SaveWaveRuns(root, slug, summary.RunSummaryVersion, runs[:1]))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Equal(t, "repair", view.NextReadyActions[0])
	assert.Contains(t, strings.Join(view.Diagnostics, "\n"), "incomplete wave run evidence")

	found := false
	for _, blocker := range view.Blockers {
		if blocker.Code == "wave_runs_incomplete" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected status blockers to include wave_runs_incomplete")
}

func TestBuildGovernedStatusViewSuggestsRepairWhenWaveRunsAreMissingDuringVerify(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status should surface missing wave runs during verify")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "task-01")
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed only wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: verification
`)))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, view.NextReadyActions)
	assert.Equal(t, "repair", view.NextReadyActions[0])
	assert.Contains(t, strings.Join(view.Diagnostics, "\n"), "wave run evidence")

	found := false
	for _, blocker := range view.Blockers {
		if blocker.Code == "wave_runs_missing" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected status blockers to include wave_runs_missing")
}

func TestComputeProgressExcludesPassWithBlockersFromCompleted(t *testing.T) {
	t.Parallel()

	change := model.NewChange("progress")
	change.CurrentState = model.StateS2Execute
	summary := &model.ExecutionSummary{
		RunSummaryVersion: 1,
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "a", Verdict: model.TaskVerdictPass},
			{TaskID: "b", Verdict: model.TaskVerdictPass, Blockers: model.ReasonCodesFromSpecs([]string{"post_wave_file_conflict:x"})},
			{TaskID: "c", Verdict: model.TaskVerdictFail},
		},
	}

	projection, err := enginestatus.BuildProjection(t.TempDir(), change, summary, nil, progression.GovernanceReadiness{}, workflowStateLabel)
	require.NoError(t, err)
	progress := projection.Progress
	require.NotNil(t, progress)
	assert.Equal(t, 1, progress.TasksCompleted, "only pass with no blockers counts as completed")
	assert.Equal(t, 3, progress.TasksTotal)
}

func TestBuildStatusNarrativeMentionsSelectivePriorContext(t *testing.T) {
	t.Parallel()

	narrative := buildStatusNarrative(statusView{
		CurrentState: model.StateS2Execute,
		SelectedPriorContext: []selectedPriorContextView{{
			Slug: "baseline-auth",
		}},
	})

	assert.Contains(t, narrative, "Prior archived context was loaded selectively")
}
