package progression

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvanceGovernedRollsBackBundleScaffoldWhenAuthorityWriteFails(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))

	change := model.NewChange("bundle-transaction-rollback")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	change.WorkflowPreset = model.WorkflowPresetStandard
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	intentPath := filepath.Join(bundleDir, "intent.md")

	writeErr := errors.New("change authority write failed")
	withFailingGovernedTransactionWrite(t, "change.yaml", writeErr)

	_, err = AdvanceGoverned(root, change.Slug)
	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)

	_, statErr := os.Stat(intentPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	reloaded, loadErr := state.LoadChange(root, change.Slug)
	require.NoError(t, loadErr)
	assert.Equal(t, model.PlanSubStepResearch, reloaded.PlanSubStep)
}

func TestAdvanceGovernedTransactionRefreshesWorktreeBinding(t *testing.T) {
	root, worktreeRoot := setupTransactionRepoWithWorktree(t)

	change := model.NewChange("transaction-refreshes-binding")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "transaction-binding"
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, os.Remove(state.WorktreeBindingPath(root, change.Slug)))

	_, err := AdvanceGoverned(root, change.Slug)
	require.NoError(t, err)

	require.FileExists(t, state.WorktreeBindingPath(root, change.Slug))
	loaded, err := state.LoadChange(root, change.Slug)
	require.NoError(t, err)
	wantWorktree, err := state.NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
}

func TestAdvanceGovernedRollsBackWavePlanWhenAuthorityWriteFails(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))

	change := model.NewChange("wave-plan-transaction-rollback")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	change.WorkflowPreset = model.WorkflowPresetLight
	require.NoError(t, state.SaveChange(root, change))
	writeValidPlanningBundleForTransactionTest(t, root, change)

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, record)

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	wavePlanPath := filepath.Join(bundleDir, "verification", state.WavePlanFileName)

	writeErr := errors.New("change authority write failed")
	withFailingGovernedTransactionWrite(t, "change.yaml", writeErr)

	_, err = AdvanceGoverned(root, change.Slug)
	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)

	_, statErr := os.Stat(wavePlanPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	reloaded, loadErr := state.LoadChange(root, change.Slug)
	require.NoError(t, loadErr)
	assert.Equal(t, model.StateS1Plan, reloaded.CurrentState)
	assert.Equal(t, model.PlanSubStepAudit, reloaded.PlanSubStep)
}

func withFailingGovernedTransactionWrite(t *testing.T, suffix string, failErr error) {
	t.Helper()

	original := applyGovernedFileTransaction
	applyGovernedFileTransaction = func(ops []fsutil.FileTransactionOp) error {
		return fsutil.ApplyFileTransactionWithHooks(ops, fsutil.FileTransactionHooks{
			WriteFile: func(path string, data []byte, perm os.FileMode) error {
				if strings.HasSuffix(filepath.ToSlash(path), suffix) {
					return failErr
				}
				return fsutil.WriteFileAtomic(path, data, perm)
			},
		})
	}
	t.Cleanup(func() {
		applyGovernedFileTransaction = original
	})
}

func setupTransactionRepoWithWorktree(t *testing.T) (repoRoot string, worktreePath string) {
	t.Helper()

	root := t.TempDir()
	runTransactionGit(t, root, "init", "-b", "main")
	runTransactionGit(t, root, "config", "user.email", "transaction@example.com")
	runTransactionGit(t, root, "config", "user.name", "Transaction Test")
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))
	runTransactionGit(t, root, "add", ".slipway.yaml")
	runTransactionGit(t, root, "commit", "-m", "init")

	worktreeRoot := filepath.Join(t.TempDir(), "transaction-binding")
	runTransactionGit(t, root, "worktree", "add", worktreeRoot, "-b", "transaction-binding")
	return root, worktreeRoot
}

func runTransactionGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) // #nosec G204 -- test helper executes fixed git commands with test-controlled args.
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, string(out))
}

func writeValidPlanningBundleForTransactionTest(t *testing.T, root string, change model.Change) {
	t.Helper()

	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, change.WorkflowPreset))
	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

## Requirements

### Requirement: Transactional transition
REQ-001: The system MUST keep transition files all-or-nothing.

#### Scenario: Rollback
GIVEN a transition file is written
WHEN a later authority write fails
THEN the first file is rolled back.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision

## Alternatives Considered
- Use per-call rollback.
- Use a shared file transaction.

## Selected Approach
Use a shared file transaction.

## Interfaces and Data Flow
Progression builds file operations and applies them together.

## Rollout and Rollback
Revert the transaction wrapper if needed.

## Risk
Rollback failure must be visible.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

## Task List

- [ ] `+"`t-01`"+` implement transaction
  - target_files: ["internal/fsutil/transaction.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
}
