package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeNextJSON(t *testing.T, args []string, v any) {
	t.Helper()
	var out bytes.Buffer
	cmd := makeNextCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
	require.NoError(t, json.Unmarshal(out.Bytes(), v))
}

// writeScaffoldCodebaseMapDocs writes the full durable doc set with
// non-substantive content so AssessCodebaseMapDocs classifies the whole map
// scaffold_only regardless of detected baseline facts.
func writeScaffoldCodebaseMapDocs(t *testing.T, root string) {
	t.Helper()
	dir := state.CodebaseMapDir(root)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, name := range []string{
		"STACK.md", "INTEGRATIONS.md", "ARCHITECTURE.md", "STRUCTURE.md",
		"CONVENTIONS.md", "TESTING.md", "CONCERNS.md",
	} {
		body := "# " + strings.TrimSuffix(name, ".md") + "\n- Placeholder:\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}
}

func TestNextSurfacesScaffoldCodebaseMapStatusOnBothSurfaces(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		slug := createGovernedRequest(t, root, "L2", "surface scaffold codebase map status")
		writeScaffoldCodebaseMapDocs(t, root)

		// Standard next view (diagnostics) carries the freshness status field.
		var fullView nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &fullView)
		assert.Equal(t, slug, fullView.Slug)
		assert.Equal(t, artifact.CodebaseMapStatusScaffoldOnly, fullView.InputContext.CodebaseMapStatus)
		require.NotEmpty(t, fullView.InputContext.CodebaseMapDocStates)
		assert.Equal(t, artifact.CodebaseMapStatusScaffoldOnly, fullView.InputContext.CodebaseMapDocStates["architecture"])

		// Compact handoff/run projection must carry the same field — a next-only
		// assertion would miss a forgotten projection copy at next_handoff.go.
		var handoff nextHandoffView
		decodeNextJSON(t, []string{"--json"}, &handoff)
		assert.Equal(t, artifact.CodebaseMapStatusScaffoldOnly, handoff.InputContext.CodebaseMapStatus)
		require.NotEmpty(t, handoff.InputContext.CodebaseMapDocStates)
		assert.Equal(t, artifact.CodebaseMapStatusScaffoldOnly, handoff.InputContext.CodebaseMapDocStates["architecture"])
	})
}

func TestNextReportsMissingCodebaseMapStatusNotOmitted(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		createGovernedRequest(t, root, "L2", "missing codebase map status")
		require.NoError(t, os.RemoveAll(state.CodebaseMapDir(root)))

		// No map present: the field must report the valid "missing" assessment
		// with per-doc states, NOT an omitted/empty field (#27 empty-map case).
		var fullView nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &fullView)
		assert.Equal(t, artifact.CodebaseMapStatusMissing, fullView.InputContext.CodebaseMapStatus)
		require.NotEmpty(t, fullView.InputContext.CodebaseMapDocStates)
		assert.Equal(t, artifact.CodebaseMapStatusMissing, fullView.InputContext.CodebaseMapDocStates["architecture"])

		var handoff nextHandoffView
		decodeNextJSON(t, []string{"--json"}, &handoff)
		assert.Equal(t, artifact.CodebaseMapStatusMissing, handoff.InputContext.CodebaseMapStatus)
		require.NotEmpty(t, handoff.InputContext.CodebaseMapDocStates)
	})
}

func TestNextIncludesDurableCodebaseMapPathsForGovernedRequests(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))

		slug := createGovernedRequest(t, root, "L2", "durable codebase mapping paths")

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, "artifacts/codebase", view.InputContext.CodebaseMapDir)
		require.Len(t, view.InputContext.CodebaseMapDocs, 7)
		assert.Equal(t, "artifacts/codebase/STACK.md", view.InputContext.CodebaseMapDocs["stack"])
		assert.Equal(t, "artifacts/codebase/ARCHITECTURE.md", view.InputContext.CodebaseMapDocs["architecture"])
		assert.Equal(t, "artifacts/codebase/TESTING.md", view.InputContext.CodebaseMapDocs["testing"])
	})
}

func TestBuildNextContextFallsBackToProjectRootWithoutWorktreeBinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("discovery-no-worktree")
	change.Description = "discovery change without bound worktree"
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	require.NoError(t, state.SaveChange(root, change))

	var view nextView
	loaded, _, err := buildNextContextByMode(root, &view, changeRef{Slug: change.Slug}, "", true)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	expectedRoot, err := state.NormalizePath(root)
	require.NoError(t, err)
	assert.Equal(t, expectedRoot, view.InputContext.WorkspaceRoot)
	assert.Equal(t, "artifacts/changes/"+change.Slug, view.InputContext.ArtifactBundle)
	assert.Equal(t, "artifacts/codebase", view.InputContext.CodebaseMapDir)
}

func TestBuildNextContextIncludesBoundedHandoffContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("docs-handoff")
	change.Description = "document operator runbook"
	change.WorkflowProfile = model.WorkflowProfileDocs
	change.ProjectContext = model.ProjectContext{
		TechStack: "Go",
		TestCmd:   "go test ./...",
	}
	require.NoError(t, state.SaveChange(root, change))

	var view nextView
	loaded, _, err := buildNextContextByMode(root, &view, changeRef{Slug: change.Slug}, "", true)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	require.NotNil(t, view.InputContext.ProjectContext)
	assert.Equal(t, "Go", view.InputContext.ProjectContext.TechStack)
	require.NotNil(t, view.InputContext.HandoffContext)
	assert.Equal(t, "docs", view.InputContext.HandoffContext.WorkflowProfile)
	assert.Equal(t, "bounded_references_only", view.InputContext.HandoffContext.ContextPolicy)
	assert.Equal(t, "artifacts/changes/docs-handoff/change.yaml", view.InputContext.HandoffContext.ChangeAuthority)
	assert.Contains(t, view.InputContext.HandoffContext.LifecycleEventLog, "events/lifecycle.jsonl")
	assert.Contains(t, view.InputContext.HandoffContext.RequiredReads, ".slipway.yaml")
	require.NotNil(t, view.InputContext.HandoffContext.Trace)
	assert.Contains(t, view.InputContext.HandoffContext.Trace.CorrelationID, "next-docs-handoff")
	require.NotNil(t, view.InputContext.HandoffContext.ContextBudget)
	assert.Equal(t, "compact", view.InputContext.HandoffContext.ContextBudget.Mode)
	assert.NotEmpty(t, view.InputContext.HandoffContext.ReadRefs)
	require.NotNil(t, view.InputContext.HandoffContext.Risk)
	assert.Contains(t, view.InputContext.HandoffContext.Risk.Hints[0], "docs profile")
}

func TestBuildNextContextIncludesAdvisoryPolicyPackHandoff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	require.NoError(t, os.MkdirAll(filepath.Join(root, ".slipway", "policies"), 0o755))
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  policy_packs:
    - name: platform
      path: .slipway/policies/platform.yaml
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway", "policies", "platform.yaml"), []byte(`version: 1
advisory_rules:
  - preserve operator runbook links
artifact_requirements:
  - runbook.md must describe rollback
recommended_reviewers:
  - platform
terminology:
  SLA: service level agreement
`), 0o644))

	change := model.NewChange("policy-pack-handoff")
	require.NoError(t, state.SaveChange(root, change))

	var view nextView
	loaded, _, err := buildNextContextByMode(root, &view, changeRef{Slug: change.Slug}, "", true)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	require.NotNil(t, view.InputContext.HandoffContext)
	require.Len(t, view.InputContext.HandoffContext.PolicyPacks, 1)
	pack := view.InputContext.HandoffContext.PolicyPacks[0]
	assert.Equal(t, "platform", pack.Name)
	assert.Equal(t, "advisory", pack.Mode)
	assert.Equal(t, "1", pack.SchemaVersion)
	assert.Contains(t, pack.Path, ".slipway/policies/platform.yaml")
	assert.Contains(t, pack.AdvisoryRules, "preserve operator runbook links")
	assert.Contains(t, pack.ArtifactRequirements, "runbook.md must describe rollback")
	assert.Contains(t, pack.RecommendedReviewers, "platform")
	assert.Contains(t, pack.Terminology, "SLA=service level agreement")
	assert.Contains(t, view.InputContext.HandoffContext.RequiredReads, "artifacts/changes/policy-pack-handoff/change.yaml")
	assert.Contains(t, view.InputContext.HandoffContext.RequiredReads, ".slipway/policies/platform.yaml")

	foundRef := false
	for _, ref := range view.InputContext.HandoffContext.ReadRefs {
		if ref.Kind == "policy_pack" && strings.Contains(ref.Path, ".slipway/policies/platform.yaml") {
			foundRef = true
		}
	}
	assert.True(t, foundRef, "expected policy_pack read ref")
}

func TestBuildNextContextLeavesGateStatusToReadinessEvaluation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "gate status warning on malformed verification")
	verificationPath := state.VerificationFilePath(root, slug, "plan-audit")
	require.NoError(t, os.MkdirAll(filepath.Dir(verificationPath), 0o755))
	require.NoError(t, os.WriteFile(verificationPath, []byte("verdict: ["), 0o644))

	var view nextView
	loaded, _, err := buildNextContextByMode(root, &view, changeRef{Slug: slug}, "", true)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Nil(t, view.InputContext.GateStatus)
	assert.Empty(t, strings.Join(view.Warnings, "\n"))
}

func TestNextUsesWorktreeScopedCodebaseMapPathsForDedicatedWorktree(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L3", "worktree-scoped codebase map in worktree")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Advance to audit substep where worktree gate binds the worktree.
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		worktreePath := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreePath, "-b", branch)

		// Bind worktree and relocate bundle manually before running next.
		changeBeforeWT := change // value copy before worktree binding
		change.WorktreePath = worktreePath
		change.WorktreeBranch = branch
		require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
		require.NoError(t, state.SaveChange(root, change))

		writeWorktreePreflightEvidence(t, root, slug, worktreePath, branch)

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		assert.Equal(t, "artifacts/codebase", view.InputContext.CodebaseMapDir)
		assert.Equal(t, "artifacts/codebase/STACK.md", view.InputContext.CodebaseMapDocs["stack"])
		assert.Equal(t, "artifacts/codebase/ARCHITECTURE.md", view.InputContext.CodebaseMapDocs["architecture"])
	})
}

func TestBuildNextContextIncludesSelectedArchivedDependencyContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	archived := model.NewChange("baseline-auth")
	archived.CurrentState = model.StateS3Review
	archived.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, archived))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", archived.Slug), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "changes", archived.Slug, "change.yaml"), []byte("id: x"), 0o644))
	_, err := state.ArchiveChange(root, archived, model.ChangeStatusDone)
	require.NoError(t, err)

	change := model.NewChange("consumer")
	change.Description = "consumer change"
	change.ContextDependencies = model.ContextDependencies{
		Requires: []model.ContextRequirement{
			{Slug: "baseline-auth", Provides: []string{"auth-contract"}},
			{Slug: "missing-auth", Provides: []string{"session-model"}},
		},
	}
	require.NoError(t, state.SaveChange(root, change))

	var view nextView
	loaded, _, err := buildNextContextByMode(root, &view, changeRef{Slug: change.Slug}, "", true)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	require.NotNil(t, view.InputContext.ContextDependencies)
	assert.Equal(t, change.ContextDependencies, *view.InputContext.ContextDependencies)
	require.Len(t, view.InputContext.SelectedPriorContext, 1)
	assert.Equal(t, "baseline-auth", view.InputContext.SelectedPriorContext[0].Slug)
	assert.Equal(t, "artifacts/changes/archived/baseline-auth/change.yaml", view.InputContext.SelectedPriorContext[0].SourceStateFile)
	assert.Equal(t, []string{"requires:auth-contract"}, view.InputContext.SelectedPriorContext[0].SelectedBecause)
	require.Len(t, view.InputContext.UnresolvedDependencies, 1)
	assert.Equal(t, "missing-auth", view.InputContext.UnresolvedDependencies[0].Slug)
	assert.Equal(t, []string{"session-model"}, view.InputContext.UnresolvedDependencies[0].Provides)
	assert.Equal(t, "archive_not_found", view.InputContext.UnresolvedDependencies[0].Reason)
}
