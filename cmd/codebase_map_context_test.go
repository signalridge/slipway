package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextIncludesDurableCodebaseMapPathsForGovernedRequests(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))

		slug := createGovernedRequest(t, root, "L2", "durable codebase mapping paths")

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
	})
}

func TestBuildNextContextLeavesGateStatusToReadinessEvaluation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
	})
}

func TestNextUsesRepoScopedCodebaseMapPathsForDedicatedWorktree(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L3", "repo-scoped codebase map in worktree")
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

		expectedRoot, err := state.NormalizePath(root)
		require.NoError(t, err)
		assert.Equal(t, filepath.ToSlash(filepath.Join(expectedRoot, "artifacts", "codebase")), view.InputContext.CodebaseMapDir)
		assert.Equal(t, filepath.ToSlash(filepath.Join(expectedRoot, "artifacts", "codebase", "STACK.md")), view.InputContext.CodebaseMapDocs["stack"])
		assert.Equal(t, filepath.ToSlash(filepath.Join(expectedRoot, "artifacts", "codebase", "ARCHITECTURE.md")), view.InputContext.CodebaseMapDocs["architecture"])
	})
}

func TestBuildNextContextIncludesSelectedArchivedDependencyContext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		archived := model.NewChange("baseline-auth")
		archived.CurrentState = model.StateS4Verify
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
	})
}
