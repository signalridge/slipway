package progression

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
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
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	snap, err := governance.PreviewGovernanceSnapshot(root, change, bundleDir)
	require.NoError(t, err)
	expectedBlockers := governance.RequiredActionBlockers(change, governance.ResolveRuntimeRequiredActions(root, change, snap))
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
	change.CurrentState = model.StateS4Verify
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

func TestEvaluateGovernanceReadinessDoesNotRetainStaleControlsFromPersistedSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-monotonic-controls")
	change.CurrentState = model.StateS4Verify
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
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))

	shipAuthority, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
		ArtifactReadiness: ArtifactReadiness{Ready: true},
		PassingSkills:     map[string]model.VerificationRecord{},
		SkillBlockers:     []model.ReasonCode{model.NewReasonCode("required_skill_missing", "goal-verification")},
		ReviewSurface:     &ReviewAuthority{},
	})
	require.NoError(t, err)
	assert.Contains(t, shipAuthority.VerifySkillBlockers, model.NewReasonCode("required_skill_missing", "goal-verification"))
}

func TestBuildShipAuthorityUsesCachedReviewAuthorityWhenReviewSurfaceIsHidden(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("ship-hidden-review-surface-cache")
	change.CurrentState = model.StateS4Verify
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
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("before\n"), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", "README.md")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "init")

	bundleDir := filepath.Join(root, "artifacts", "changes", "scope-drift")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("after\n"), 0o644))
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
	assert.NotContains(t, files, "artifacts/changes/scope-drift/tasks.md")
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
		Required:    []string{"intent.md"},
		Blockers:    []model.ReasonCode{model.NewReasonCode("artifact_reader_contract_blocker", "")},
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
			Source:   "stub_projection",
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
	assert.True(t, hasAdvanceReasonCode(readiness.Blockers, "artifact_reader_contract_blocker"))
	assert.Contains(t, readiness.Diagnostics, "artifact_reader_contract_diagnostic")
	require.NotNil(t, readiness.ArtifactProjection)
	require.Len(t, readiness.ArtifactProjection.Nodes, 1)
	assert.Equal(t, "stub_projection", readiness.ArtifactProjection.Nodes[0].Source)
	assert.Contains(t, readiness.Diagnostics, "projection_reader_contract_diagnostic")
}

func initGitWorkspaceForReadinessOptimizationTests(t *testing.T, root string) {
	t.Helper()

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

func gitForReadinessOptimizationTests(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}
