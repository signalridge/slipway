package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateIncludesRequirementsContractForGovernedChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "validate requirements contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		require.NotNil(t, view.RequirementsContract)
		assert.Equal(t, "valid", view.RequirementsContract.Status)
		assert.Equal(t, artifact.ResolveArtifactPath(bundleDir, slug, "requirements.md"), view.RequirementsContract.Source)
		assert.Contains(t, view.RequirementsContract.Message, "validated")
	})
}

func TestValidateReportsMissingRequirementsContract(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "validate missing requirements contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)
		require.NoError(t, os.Remove(filepath.Join(bundleDir, "requirements.md")))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		require.NotNil(t, view.RequirementsContract)
		assert.Equal(t, "missing", view.RequirementsContract.Status)
		assert.Equal(t, artifact.ResolveArtifactPath(bundleDir, slug, "requirements.md"), view.RequirementsContract.Source)
		assert.Contains(t, view.RequirementsContract.Message, "missing")
	})
}

func TestValidateReportsInvalidRequirementsContract(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "validate invalid requirements contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

No requirement blocks here.
`), 0o644))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		require.NotNil(t, view.RequirementsContract)
		assert.Equal(t, "invalid", view.RequirementsContract.Status)
		assert.Contains(t, view.RequirementsContract.Message, "no Requirement blocks found")
	})
}

func TestValidateOmitsRequirementsContractWhenPresetConfirmationPending(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "validate preset confirmation pending")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = ""
		change.SuggestedWorkflowPreset = model.WorkflowPresetStandard
		require.NoError(t, state.SaveChange(root, change))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		assert.True(t, view.PresetConfirmationPending)
		assert.Nil(t, view.RequirementsContract)
	})
}

func TestValidateDiagnosticsModeOmitsRequirementsContract(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		payload := map[string]any{}
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		_, ok := payload["requirements_contract"]
		assert.False(t, ok)
	})
}

func TestValidateUsesDedicatedWorktreePathInRequirementsContractSource(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L3", "validate worktree requirements contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreePath := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreePath, "-b", branch)

		changeBeforeWT := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&change, worktreePath, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
		require.NoError(t, state.SaveChange(root, change))

		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		require.NotNil(t, view.RequirementsContract)
		assert.Equal(t, artifact.ResolveArtifactPath(bundleDir, slug, "requirements.md"), view.RequirementsContract.Source)
		assert.Contains(t, view.RequirementsContract.Source, worktreePath)
	})
}

func TestValidateOmitsRequirementsContractWhenRequirementsFileIsUnreadable(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "validate unreadable requirements contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)
		reqPath := filepath.Join(bundleDir, "requirements.md")
		require.NoError(t, os.WriteFile(reqPath, []byte(`# Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))
		require.NoError(t, os.Chmod(reqPath, 0))
		t.Cleanup(func() {
			_ = os.Chmod(reqPath, 0o644)
		})

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		assert.Nil(t, view.RequirementsContract)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_coverage_spec_unreadable")
	})
}
