package progression

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateGovernanceReadinessExposesPassingSkillsForActivePlanningSubStep(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-passing-skills")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	writeVerificationForTest(t, root, change.Slug, "plan-audit", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	})
	writeVerificationForTest(t, root, change.Slug, "research-orchestration", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC().Add(time.Second),
	})

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	assert.Equal(t, model.VerificationVerdictPass, readiness.PassingSkills["plan-audit"].Verdict)
	assert.NotContains(t, readiness.PassingSkills, "research-orchestration")
}

func TestRequiredReviewLayerTokensForSkillUsesArtifactScope(t *testing.T) {
	t.Parallel()

	change := model.NewChange("artifact-scoped-review-tokens")
	change.GuardrailDomain = "external_api_contracts"

	manifestOnly := &ArtifactProjection{
		Nodes: []ArtifactProjectionNode{{
			Name:  "change.yaml",
			State: string(model.ArtifactLifecycleDraft),
		}},
	}
	assert.ElementsMatch(t,
		[]string{"layer:R0=pass"},
		RequiredReviewLayerTokensForSkill(change, manifestOnly, false, SkillSpecComplianceReview),
	)

	decisionScope := &ArtifactProjection{
		Nodes: []ArtifactProjectionNode{{
			Name:  "decision.md",
			State: string(model.ArtifactLifecycleDraft),
		}},
	}
	assert.ElementsMatch(t,
		[]string{"layer:R0=pass", "layer:R3=pass"},
		RequiredReviewLayerTokensForSkill(change, decisionScope, false, SkillSpecComplianceReview),
	)
	assert.ElementsMatch(t,
		[]string{"layer:IR1=pass", "layer:IR3=pass"},
		RequiredReviewLayerTokensForSkill(change, decisionScope, false, SkillCodeQualityReview),
	)
}

func TestEvaluateGovernanceReadinessSkipsGateEvaluationsUnlessRequested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-gates-opt-in")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	assert.Nil(t, readiness.GateEvaluations)

	readiness, err = EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{
		IncludeGateEvaluations: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, readiness.GateEvaluations)
}

func TestEvaluateGovernanceReadinessDoesNotPersistGovernanceSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-no-snapshot-persist")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	snapshotPath := state.GovernanceSnapshotCachePath(root, change.Slug)
	_, err := os.Stat(snapshotPath)
	require.True(t, os.IsNotExist(err), "test setup must start without a persisted snapshot")

	_, err = EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)

	_, err = os.Stat(snapshotPath)
	assert.True(t, os.IsNotExist(err), "shared readiness should not persist governance snapshots on read paths")
}

func TestEvaluateGovernanceReadinessRetainsRequiredActionBlockersWhenSnapshotCachePathIsBroken(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-required-actions-preview")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	snap, err := governance.PreviewGovernanceSnapshot(root, change, bundleDir)
	require.NoError(t, err)
	expectedBlockers := governance.RequiredActionBlockers(change, governance.ResolveRuntimeRequiredActions(root, change, snap, false))
	require.NotEmpty(t, expectedBlockers, "test setup must exercise required-action blockers")

	snapshotDir := filepath.Dir(state.GovernanceSnapshotCachePath(root, change.Slug))
	require.NoError(t, os.MkdirAll(filepath.Dir(snapshotDir), 0o755))
	require.NoError(t, os.WriteFile(snapshotDir, []byte("block snapshot dir creation"), 0o644))

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	for _, blocker := range expectedBlockers {
		assert.Contains(t, readiness.Blockers, model.ReasonCodeFromSpec(blocker))
	}
	assert.NotContains(t, readiness.Diagnostics, "governance_snapshot_unavailable")
}

func TestEvaluateGovernanceReadinessKeepsReviewSurfaceOptInAtVerify(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-review-surface-opt-in")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	readOnly, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)

	withSurface, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{
		IncludeReviewSurface: true,
	})
	require.NoError(t, err)

	require.NotNil(t, withSurface.ReviewSurface)
	require.NotEmpty(t, withSurface.ReviewSurface.Blockers, "test setup must exercise review blockers")
	assert.Nil(t, readOnly.ReviewSurface, "review surface should stay opt-in even when review blockers are shared")
	assert.ElementsMatch(t, withSurface.Blockers, readOnly.Blockers)
}

func TestEvaluateGovernanceReadinessFailsClosedOnMalformedVerificationWithoutRequiredSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-malformed-verification-no-required-skills")
	change.CurrentState = model.StateDone
	change.Status = model.ChangeStatusDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	verificationDir := state.VerificationDir(root, change.Slug)
	require.NoError(t, os.MkdirAll(verificationDir, 0o755))
	brokenPath := filepath.Join(verificationDir, "broken.yaml")
	require.NoError(t, os.WriteFile(brokenPath, []byte("not valid yaml: [[["), 0o644))

	_, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.Error(t, err)
	var loadErr *state.VerificationLoadError
	require.ErrorAs(t, err, &loadErr)
	normalizedBrokenPath, normalizeErr := state.NormalizePath(brokenPath)
	if normalizeErr == nil {
		brokenPath = normalizedBrokenPath
	}
	assert.Equal(t, brokenPath, loadErr.Path)
}

func TestEvaluateGovernanceReadinessUsesPreloadedVerificationRecords(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-preloaded-verification-records")
	change.CurrentState = model.StateDone
	change.Status = model.ChangeStatusDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	verificationDir := state.VerificationDir(root, change.Slug)
	require.NoError(t, os.MkdirAll(verificationDir, 0o755))
	brokenPath := filepath.Join(verificationDir, "broken.yaml")
	require.NoError(t, os.WriteFile(brokenPath, []byte("not valid yaml: [[["), 0o644))

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{
		VerificationRecords: map[string]model.VerificationRecord{},
	})
	require.NoError(t, err)
	assert.Empty(t, readiness.PassingSkills)

	_, err = EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.Error(t, err)
	var loadErr *state.VerificationLoadError
	require.ErrorAs(t, err, &loadErr)
	normalizedBrokenPath, normalizeErr := state.NormalizePath(brokenPath)
	if normalizeErr == nil {
		brokenPath = normalizedBrokenPath
	}
	assert.Equal(t, brokenPath, loadErr.Path)
}

func TestEvaluateGovernanceReadinessDoesNotRetainStaleControlsFromPersistedSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-monotonic-controls")
	change.CurrentState = model.StateS3Review
	change.GuardrailDomain = string(model.GuardrailDomainAuthAuthZ)
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	_, err = governance.RecomputeGovernanceSnapshot(root, change, bundleDir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  disabled_controls:
    - independent-review
`), 0o644))

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)

	var controlIDs []model.ControlID
	for _, control := range readiness.ActiveControls {
		controlIDs = append(controlIDs, control.ControlID)
	}
	assert.NotContains(t, controlIDs, model.ControlIndependentReview)

	foundBlocker := false
	for _, blocker := range readiness.Blockers {
		if blocker.Code == "governance_action_required" && strings.HasPrefix(blocker.Detail, "independent-review:") {
			foundBlocker = true
			break
		}
	}
	assert.False(t, foundBlocker, "read-only readiness should reflect fresh preview controls, not preserve stale independent-review blockers from persisted snapshot")
}

func TestBuildShipAuthorityUsesStructuredVerifySkillBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("ship-structured-skill-blockers")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	shipAuthority, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
		ArtifactReadiness: ArtifactReadiness{Ready: true},
		PassingSkills:     map[string]model.VerificationRecord{},
		SkillBlockers:     []model.ReasonCode{model.NewReasonCode("required_skill_missing", "code-quality-review")},
		ReviewSurface:     &ReviewAuthority{},
	})
	require.NoError(t, err)
	assert.Contains(t, shipAuthority.VerifySkillBlockers, model.NewReasonCode("required_skill_missing", "code-quality-review"))
}

func TestBuildShipAuthorityUsesCachedReviewAuthorityWhenReviewSurfaceIsHidden(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("ship-hidden-review-surface-cache")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	cachedReviewAuthority := ReviewAuthority{
		Blockers: []model.ReasonCode{model.NewReasonCode("cached_review_blocker", "")},
	}
	shipAuthority, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
		ArtifactReadiness: ArtifactReadiness{Ready: true},
		PassingSkills:     map[string]model.VerificationRecord{},
		reviewAuthority:   &cachedReviewAuthority,
	})
	require.NoError(t, err)
	assert.Equal(t, cachedReviewAuthority.Blockers, shipAuthority.ReviewAuthority.Blockers)
}

func TestArtifactScopeForReviewNilProjectionFallback(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"change"}, artifactScopeForReview(nil, false))
	assert.Nil(t, artifactScopeForReview(nil, true))
}

func TestEvaluateReviewLayerBlockersTreatsProjectedChangeYamlAsManifest(t *testing.T) {
	t.Parallel()

	change := model.NewChange("manifest-review-scope")
	change.GuardrailDomain = string(model.GuardrailDomainAuthAuthZ)

	blockers := EvaluateReviewLayerBlockersFromNamedEvidence(
		change,
		model.VerificationRecord{
			Verdict: model.VerificationVerdictPass,
			References: []string{
				"layer:R0=pass",
				"layer:IR1=pass",
				"layer:IR3=pass",
			},
		},
		model.VerificationRecord{},
		&ArtifactProjection{
			Nodes: []ArtifactProjectionNode{{
				Name:     "change.yaml",
				State:    string(model.ArtifactLifecycleDraft),
				Required: true,
			}},
		},
		false,
	)

	assert.Empty(t, blockers, "projected change.yaml should keep manifest-only review requirements")
}

func TestEvaluateReviewLayerBlockersMergesImplementationReviewEvidence(t *testing.T) {
	t.Parallel()

	change := model.NewChange("merged-review-layers")
	change.GuardrailDomain = string(model.GuardrailDomainExternalAPIContracts)
	projection := &ArtifactProjection{
		Nodes: []ArtifactProjectionNode{{
			Name:     "decision.md",
			State:    string(model.ArtifactLifecycleDraft),
			Required: true,
		}},
	}
	specEvidence := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		References: []string{"layer:R0=pass", "layer:R3=pass"},
	}
	codeEvidence := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		References: []string{"layer:IR1=pass", "layer:IR3=pass"},
	}

	assert.Contains(t,
		model.ReasonSpecs(EvaluateReviewLayerBlockersFromNamedEvidence(
			change,
			specEvidence,
			model.VerificationRecord{Verdict: model.VerificationVerdictPass},
			projection,
			false,
		)),
		"review_layer_missing:IR1",
	)
	assert.Empty(t,
		EvaluateReviewLayerBlockersFromNamedEvidence(change, specEvidence, codeEvidence, projection, false),
	)
}

func TestEvaluateReviewLayerBlockersRequiresImplementationEvidenceProvenance(t *testing.T) {
	t.Parallel()

	change := model.NewChange("review-layer-provenance")
	change.GuardrailDomain = string(model.GuardrailDomainExternalAPIContracts)
	projection := &ArtifactProjection{
		Nodes: []ArtifactProjectionNode{{
			Name:     "decision.md",
			State:    string(model.ArtifactLifecycleDraft),
			Required: true,
		}},
	}
	specEvidenceWithImplementationRefs := model.VerificationRecord{
		Verdict: model.VerificationVerdictPass,
		References: []string{
			"layer:R0=pass",
			"layer:R3=pass",
			"layer:IR1=pass",
			"layer:IR3=pass",
		},
	}
	codeEvidenceWithoutRefs := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		References: nil,
	}

	blockers := model.ReasonSpecs(EvaluateReviewLayerBlockersFromNamedEvidence(
		change,
		specEvidenceWithImplementationRefs,
		codeEvidenceWithoutRefs,
		projection,
		false,
	))
	assert.Contains(t, blockers, "review_layer_missing:IR1")
	assert.Contains(t, blockers, "review_layer_missing:IR3")
}

func TestScopeContractWorkspaceChangedFilesIncludesWorktreeDiffAndExcludesBundle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "codebase"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("before\n"), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", "README.md")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "init")

	bundleDir := filepath.Join(root, "artifacts", "changes", "scope-drift")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("after\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "codebase", "STRUCTURE.md"), []byte("# Structure\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "untracked.go"), []byte("package cmd\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".golangci.yml"), []byte("run:\n  timeout: 2m\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway.yaml"), []byte("version: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(state.LocalStateGitIgnoreBlock()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	files := scopeContractWorkspaceChangedFiles(state.ResolvedChangePaths{
		WorkspaceRoot:     root,
		GovernedBundleDir: bundleDir,
	})

	assert.Contains(t, files, "README.md")
	assert.Contains(t, files, ".golangci.yml")
	assert.Contains(t, files, "cmd/untracked.go")
	assert.NotContains(t, files, ".slipway.yaml")
	assert.NotContains(t, files, ".gitignore")
	assert.NotContains(t, files, "artifacts/codebase/STRUCTURE.md")
	assert.NotContains(t, files, "artifacts/changes/scope-drift/tasks.md")
}

func TestEvaluateGovernanceReadinessDisclosesExemptContextFilesWithoutScopeDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)

	const inScopeFile = "cmd/next.go"
	const exemptFile = "artifacts/codebase/ARCHITECTURE.md"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "codebase"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, inScopeFile), []byte("package cmd\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, exemptFile), []byte("# Architecture\n"), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", inScopeFile, exemptFile)
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "init")

	change := model.NewChange("scope-contract-exempt-disclosure")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetLight
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, "intent.md"),
		[]byte("# Intent\n\n## Summary\nDisclose exempted context files.\n"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, "requirements.md"),
		[]byte("# Requirements\n\n### Requirement: Exempt disclosure\nREQ-001: Disclose.\n"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, "decision.md"),
		[]byte("# Decision\n\n## Selected Approach\nDisclose.\n"),
		0o644,
	))
	writeTasksAndMaterializeWavePlan(t, root, change, "# Tasks\n\n"+
		"- [x] `t-01` Implement in-scope change\n"+
		"  - target_files: [\""+inScopeFile+"\"]\n"+
		"  - task_kind: code\n")

	recordedAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	})

	freshnessInputs := expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "t-01")
	taskEvidence := map[string]any{
		"task_id":             "t-01",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{inScopeFile},
		"target_files":        []string{inScopeFile},
		"blockers":            []model.ReasonCode{},
		"evidence_ref":        "test:t-01",
		"captured_at":         recordedAt.Format(time.RFC3339Nano),
		"freshness_inputs":    freshnessInputs,
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, change.Slug), "t-01.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        recordedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "t-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{inScopeFile},
			TargetFiles:  []string{inScopeFile},
			EvidenceRef:  "test:t-01",
			CapturedAt:   recordedAt,
		}},
	}
	summary.SyncDerivedFields()
	summary.Normalize()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	// Dirty both a tracked in-scope file and a tracked exempt context-artifact
	// file so the scope-contract evaluation observes both as workspace changes.
	require.NoError(t, os.WriteFile(filepath.Join(root, inScopeFile), []byte("package cmd // changed\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, exemptFile), []byte("# Architecture changed\n"), 0o644))

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	require.NotNil(t, readiness.ScopeContract)

	assert.Equal(t, scopecontract.StatusPass, readiness.ScopeContract.Status,
		"exempting context-artifact files is a disclosure change and must keep the contract passing")
	assert.Contains(t, readiness.ScopeContract.ExemptContextFiles, exemptFile,
		"dirty context-artifact file must be disclosed in ExemptContextFiles")
	assert.NotContains(t, readiness.ScopeContract.ChangedFiles, exemptFile,
		"exempted context-artifact file must stay out of ChangedFiles")
	assert.NotContains(t, readiness.ScopeContract.OutOfScopeFiles, exemptFile,
		"exempted context-artifact file must not be reported as out-of-scope drift")
}

func TestWorkspaceChangedFilesForDoneArchiveExcludesBundleAndGeneratedLocalState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("before\n"), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", "README.md")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "init")

	bundleDir := filepath.Join(root, "artifacts", "changes", "done-dirty")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("after\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "untracked.go"), []byte("package cmd\n"), 0o644))
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "codebase"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "codebase", "STRUCTURE.md"), []byte("# Structure\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(state.LocalStateGitIgnoreBlock()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	files := WorkspaceChangedFilesForDoneArchive(state.ResolvedChangePaths{
		WorkspaceRoot:     root,
		GovernedBundleDir: bundleDir,
	})

	assert.Contains(t, files, "README.md")
	assert.Contains(t, files, "cmd/untracked.go")
	assert.Contains(t, files, "artifacts/codebase/STRUCTURE.md")
	assert.NotContains(t, files, "artifacts/changes/done-dirty/tasks.md")
	assert.NotContains(t, files, ".gitignore")
	assert.NotContains(t, files, ".slipway.yaml")
}

func TestWorkspaceChangedFilesForDoneArchiveKeepsRealLocalGovernanceFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("before\n"), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", "README.md")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "init")

	bundleDir := filepath.Join(root, "artifacts", "changes", "done-dirty")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n\n"+state.LocalStateGitIgnoreBlock()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway.yaml"), []byte("context:\n  tech_stack: Go\n"), 0o644))

	files := WorkspaceChangedFilesForDoneArchive(state.ResolvedChangePaths{
		WorkspaceRoot:     root,
		GovernedBundleDir: bundleDir,
	})

	assert.Contains(t, files, ".gitignore")
	assert.Contains(t, files, ".slipway.yaml")
}

func TestWorkspaceChangedFilesForDoneArchiveSkipsManagedGitIgnoreMigration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	legacyGitIgnore := "build/\n\n" +
		"# Slipway local state (managed)\n" +
		"/old-slipway-local-state/\n" +
		"# End Slipway local state\n\n" +
		"dist/\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(legacyGitIgnore), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", ".gitignore")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "init")
	_, err := state.EnsureLocalStateGitIgnore(root)
	require.NoError(t, err)

	files := WorkspaceChangedFilesForDoneArchive(state.ResolvedChangePaths{
		WorkspaceRoot:     root,
		GovernedBundleDir: filepath.Join(root, "artifacts", "changes", "done-dirty"),
	})

	assert.NotContains(t, files, ".gitignore")
}

func TestScopeContractUntrackedChangedFileKeepsRealRootDotfiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	assert.False(t, scopeContractUntrackedChangedFile(root, ".slipway.yaml"))
	assert.True(t, scopeContractUntrackedChangedFile(root, ".golangci.yml"))
	assert.True(t, scopeContractUntrackedChangedFile(root, ".gitignore"))

	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(state.LocalStateGitIgnoreBlock()), 0o644))
	assert.False(t, scopeContractUntrackedChangedFile(root, ".gitignore"))

	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n\n"+state.LocalStateGitIgnoreBlock()), 0o644))
	assert.True(t, scopeContractUntrackedChangedFile(root, ".gitignore"))
}

type stubArtifactReadinessReader struct {
	calls int
}

func (s *stubArtifactReadinessReader) Evaluate(root string, change model.Change) (ArtifactReadiness, error) {
	s.calls++
	return ArtifactReadiness{
		Ready:       false,
		Blockers:    []model.ReasonCode{model.NewReasonCode("missing_required_artifact", "tasks.md")},
		Diagnostics: []string{"artifact_reader_contract_diagnostic"},
	}, nil
}

type stubArtifactProjectionReader struct {
	calls int
}

func (s *stubArtifactProjectionReader) Project(root string, change model.Change) (ArtifactProjection, error) {
	s.calls++
	return ArtifactProjection{
		Nodes: []ArtifactProjectionNode{{
			Name:     "intent.md",
			State:    string(model.ArtifactLifecycleDraft),
			Ready:    true,
			Required: true,
		}},
		Diagnostics: []string{"projection_reader_contract_diagnostic"},
	}, nil
}

func TestEvaluateGovernanceReadinessRoutesArtifactStateThroughReaderContracts(t *testing.T) {
	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-reader-contracts")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, state.SaveChange(root, change))

	readinessReader := &stubArtifactReadinessReader{}
	projectionReader := &stubArtifactProjectionReader{}

	readiness, err := evaluateGovernanceReadinessBaseWithReaders(
		root,
		change,
		GovernanceReadinessOptions{IncludeArtifactProjection: true},
		governanceReadinessReaders{
			artifactReadiness:  readinessReader,
			artifactProjection: projectionReader,
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, readinessReader.calls)
	assert.Equal(t, 1, projectionReader.calls)
	assert.True(t, hasAdvanceReasonCode(readiness.Blockers, "missing_required_artifact"))
	assert.Contains(t, readiness.Diagnostics, "artifact_reader_contract_diagnostic")
	require.NotNil(t, readiness.ArtifactProjection)
	require.Len(t, readiness.ArtifactProjection.Nodes, 1)
	assert.Contains(t, readiness.Diagnostics, "projection_reader_contract_diagnostic")
}

func initGitWorkspaceForReadinessOptimizationTests(t *testing.T, root string) {
	t.Helper()

	cleanupGitWorkspaceBeforeTempDir(t, root)
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Slipway Test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}
}

func cleanupGitWorkspaceBeforeTempDir(t *testing.T, root string) {
	t.Helper()

	t.Cleanup(func() {
		var err error
		for attempt := 0; attempt < 5; attempt++ {
			err = os.RemoveAll(root)
			if err == nil {
				return
			}
			time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
		}
		require.NoError(t, err)
	})
}

func gitForReadinessOptimizationTests(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}

// installSnapshotBuildCounter wraps the package-level governance-snapshot builder
// seam with a counter and returns a pointer to the count. The original builder is
// restored on cleanup. Callers must NOT run in parallel: swapping the shared seam
// is only race-free during the sequential (non-parallel) test phase.
func installSnapshotBuildCounter(t *testing.T) *int {
	t.Helper()

	original := previewGovernanceSnapshotForReadiness
	t.Cleanup(func() { previewGovernanceSnapshotForReadiness = original })

	var builds int
	previewGovernanceSnapshotForReadiness = func(root string, change model.Change, bundleDir string) (model.GovernanceSnapshot, error) {
		builds++
		return original(root, change, bundleDir)
	}
	return &builds
}

func TestEvaluateGovernanceReadinessBuildsGovernanceSnapshotOnceThroughReviewAuthority(t *testing.T) {
	// No t.Parallel(): this test swaps the package-level snapshot-builder seam to
	// count materializations, so it must run in the non-parallel phase.
	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-authority-snapshot-once")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	builds := installSnapshotBuildCounter(t)

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, *builds,
		"S3 readiness must build the governance snapshot exactly once and thread it into the review-authority path instead of rebuilding it")

	reused, ok := readiness.cachedReviewAuthority()
	require.True(t, ok, "S3 readiness must cache the review authority built from the threaded snapshot")

	// The autopass path (EvaluateReviewAuthority, nil prebuilt) must still rebuild
	// the snapshot and produce an identical authority outcome, proving the reuse
	// optimization is observably behavior-preserving.
	*builds = 0
	rebuilt, err := EvaluateReviewAuthority(root, change)
	require.NoError(t, err)
	assert.Positive(t, *builds,
		"EvaluateReviewAuthority (autopass path) must rebuild the snapshot when no prebuilt snapshot is threaded (nil => rebuild)")

	assert.ElementsMatch(t, rebuilt.SelectedReviewSkills, reused.SelectedReviewSkills,
		"reused-snapshot selected review skills must match the rebuilt selection")
	assert.ElementsMatch(t, rebuilt.Blockers, reused.Blockers,
		"reused-snapshot review authority outcome must match the rebuilt outcome")
}

func TestReviewAuthorityReusesPrebuiltSnapshotOnlyOnChangeIdentityMatch(t *testing.T) {
	// No t.Parallel(): swaps the package-level snapshot-builder seam.
	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("review-authority-prebuilt-identity-guard")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	policy, err := governance.ResolvePresetPolicy(root, change)
	require.NoError(t, err)
	paths, err := state.ResolveChangePaths(root, change)
	require.NoError(t, err)
	snap, err := governance.PreviewGovernanceSnapshot(root, change, paths.GovernedBundleDir)
	require.NoError(t, err)

	builds := installSnapshotBuildCounter(t)

	// Matching change identity => reuse, no rebuild.
	*builds = 0
	matched, err := evaluateReviewAuthorityWithPolicyAndRecords(root, change, policy, nil,
		&prebuiltGovernanceSnapshot{change: change, snapshot: snap})
	require.NoError(t, err)
	assert.Zero(t, *builds,
		"a prebuilt snapshot matching the change identity must be reused without rebuilding the governance snapshot")

	// Mismatched change identity => fall back to a rebuild rather than silently
	// reusing a snapshot built for a different change.
	*builds = 0
	fallback, err := evaluateReviewAuthorityWithPolicyAndRecords(root, change, policy, nil,
		&prebuiltGovernanceSnapshot{change: model.NewChange("some-other-change"), snapshot: model.GovernanceSnapshot{}})
	require.NoError(t, err)
	assert.Positive(t, *builds,
		"a prebuilt snapshot for a different change must fall back to a rebuild, not be silently reused")

	assert.ElementsMatch(t, fallback.SelectedReviewSkills, matched.SelectedReviewSkills,
		"reused-snapshot selection must equal the rebuilt selection")
	assert.ElementsMatch(t, fallback.Blockers, matched.Blockers,
		"reused-snapshot review authority outcome must equal the rebuilt outcome")
}
