package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextBlocksL3AdvanceWithoutWorktreePreflightEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L3", "l3 worktree gate")

		// Advance to S2_EXECUTE where the worktree gate is now checked.
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS2Execute, view.CurrentState)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "dedicated_worktree_metadata_required")
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillWorktreePreflight, view.NextSkill.Name)

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Execute, change.CurrentState)
	})
}

func TestNextL3PreviewDoesNotRequireResearchContractBeforeResearchPhase(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		change := model.NewChange("l3-s1-preview")
		change.Description = "preview should not require research contract at S1 research"
		change.NeedsDiscovery = true
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepResearch
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS1Plan, view.CurrentState)
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "research_contract_missing")
	})
}

func TestNextL3AdvancesAfterDedicatedWorktreePreflightEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)
		slug := createGovernedRequest(t, root, "L3", "l3 worktree advance")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Advance to audit substep and bind the worktree before running next.
		// GovernedBundleBlockers validates the worktree at step 2, so the
		// worktree must be bound to the change before the advance can proceed.
		worktreePath := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreePath, "-b", branch)

		normalizedWT, normErr := state.NormalizePath(worktreePath)
		require.NoError(t, normErr)

		changeBeforeWT := change // value copy before worktree binding
		change.PlanSubStep = model.PlanSubStepAudit
		change.WorktreePath = normalizedWT
		change.WorktreeBranch = branch
		require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
		require.NoError(t, state.SaveChange(root, change))

		writeWorktreePreflightEvidence(t, root, slug, normalizedWT, branch)

		// Write passing plan-audit evidence so advance can proceed.
		writeSkillVerification(t, root, slug, progression.SkillPlanAudit, model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: time.Now().UTC(),
		})

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, normalizedWT, change.WorktreePath)
		assert.Equal(t, branch, change.WorktreeBranch)
	})
}

func TestNextL3AdvanceCreatesResearchArtifactAtScopeEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)

	change := model.NewChange("l3-scope-artifact")
	change.Description = "scope entry should have canonical research artifact"
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	require.NoError(t, state.SaveChange(root, change))

	worktreePath := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGit(t, root, "worktree", "add", worktreePath, "-b", branch)
	writeWorktreePreflightEvidence(t, root, change.Slug, worktreePath, branch)

	view, err := buildNextView(root, changeRef{Slug: change.Slug}, "", false, true, false)
	require.NoError(t, err)

	assert.Equal(t, model.StateS1Plan, view.CurrentState)

	// Research artifact should be created at the project root bundle dir
	// (not worktree) at S1_PLAN/research entry for discovery changes.
	researchPath := filepath.Join(root, "artifacts", "changes", change.Slug, "research.md")
	_, err = os.Stat(researchPath)
	require.NoError(t, err)
}

func TestNextUsesDedicatedWorktreePathsAfterPreflight(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)
		slug := createGovernedRequest(t, root, "L3", "l3 worktree paths")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreePath := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreePath, "-b", branch)

		// Bind worktree and advance to audit substep.
		changeBeforeWT := change // value copy before worktree binding
		change.PlanSubStep = model.PlanSubStepAudit
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
		normalizedWT, normErr := state.NormalizePath(worktreePath)
		require.NoError(t, normErr)
		assert.Equal(t, normalizedWT, view.InputContext.WorkspaceRoot)
		assert.Equal(t, filepath.ToSlash(filepath.Join(normalizedWT, "artifacts", "changes", slug)), view.InputContext.ArtifactBundle)
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, filepath.ToSlash(filepath.Join(normalizedWT, "artifacts", "changes", slug, "verification")), view.NextSkill.VerificationDir)
	})
}

func TestNextMovesGovernedBundleIntoDedicatedWorktreeAfterPreflight(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)
	slug := createGovernedRequest(t, root, "L3", "l3 worktree bundle migration")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	projectBundle := filepath.Join(root, "artifacts", "changes", slug)
	_, err = os.Stat(filepath.Join(projectBundle, "intent.md"))
	require.NoError(t, err)

	worktreePath := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGit(t, root, "worktree", "add", worktreePath, "-b", branch)

	// Bind worktree, advance to audit, and relocate bundle manually.
	changeBeforeWT := change // value copy before worktree binding
	change.PlanSubStep = model.PlanSubStepAudit
	change.WorktreePath = worktreePath
	change.WorktreeBranch = branch
	require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
	require.NoError(t, state.SaveChange(root, change))

	normalizedWT, normErr := state.NormalizePath(worktreePath)
	require.NoError(t, normErr)
	worktreeBundle := filepath.Join(normalizedWT, "artifacts", "changes", slug)
	_, err = os.Stat(filepath.Join(worktreeBundle, "intent.md"))
	require.NoError(t, err)
	_, err = os.Stat(projectBundle)
	assert.True(t, os.IsNotExist(err))
}

func TestValidateBlocksL3WorktreePreflightWhenEvidenceTargetsMainWorkspace(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)
		slug := createGovernedRequest(t, root, "L3", "l3 invalid main worktree")

		// Advance to S2_EXECUTE where the worktree gate is now checked.
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		branch := currentGitBranch(t, root)
		writeWorktreePreflightEvidence(t, root, slug, root, branch)

		cmd := makeValidateCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS2Execute, view.CurrentState)
		assert.False(t, view.CanAdvance)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), state.WorktreeReasonDedicatedRequired)
	})
}

// TestNextL3WorktreePreflightEvidenceUnblocksS2Execute is a regression test
// for the deadlock where GovernedBundleBlockers() called ValidateChangeWorktree()
// before the explicit DeriveWorktreeBlockers + ApplyWorktreeMetadata path had
// a chance to consume worktree-preflight evidence and persist metadata.
// Without the fix, this test would hang at S2_EXECUTE with
// blocker=dedicated_worktree_metadata_required and next_skill=worktree-preflight
// simultaneously.
func TestNextL3WorktreePreflightEvidenceUnblocksS2Execute(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)
	slug := createGovernedRequest(t, root, "L3", "l3 worktree deadlock regression")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	// Create a real worktree so the preflight evidence points to a valid path.
	worktreePath := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGit(t, root, "worktree", "add", worktreePath, "-b", branch)
	normalizedWT, normErr := state.NormalizePath(worktreePath)
	require.NoError(t, normErr)

	// Place the change at S2_EXECUTE with NO worktree bound — this is the
	// state after S1_PLAN completes when worktree gate is at S2.
	change.CurrentState = model.StateS2Execute
	change.IntakeSubStep = ""
	change.PlanSubStep = model.PlanSubStepNone
	// WorktreePath deliberately left empty.
	require.NoError(t, state.SaveChange(root, change))

	// Write passing worktree-preflight evidence referencing the real worktree.
	writeWorktreePreflightEvidence(t, root, slug, normalizedWT, branch)

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
	require.NoError(t, err)

	// Key assertion: the advance must NOT deadlock. The worktree metadata
	// should be persisted from the preflight evidence.
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "dedicated_worktree_metadata_required",
		"GovernedBundleBlockers must skip worktree check when worktree is unbound at S2_EXECUTE so the explicit derive/apply path can consume evidence")

	change, err = state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, normalizedWT, change.WorktreePath,
		"worktree metadata should be persisted from preflight evidence")
	assert.Equal(t, branch, change.WorktreeBranch)
}

func initGitRepoForWorktreeTests(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Slipway Test")
	runGit(t, root, "commit", "--allow-empty", "-m", "init")
}

func currentGitBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "resolve branch failed: %s", string(out))
	return strings.TrimSpace(string(out))
}

func writeWorktreePreflightEvidence(t *testing.T, root, slug, worktreePath, branch string) {
	t.Helper()
	writeSkillVerification(t, root, slug, "worktree-preflight", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
		References: []string{
			"worktree_path:" + worktreePath,
			"worktree_branch:" + branch,
			"baseline_verify_cmd:go test ./...",
		},
	})
}
