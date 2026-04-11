package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRequirementsCommandValidatesRequirements(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "sync requirements")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)

		// Write requirements.md in the change bundle (flat).
		reqContent := "# Requirements\n\n## Requirements\n\n### Requirement: Auth\nREQ-001: Auth MUST remain deterministic.\n"
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(reqContent), 0o644))

		out := bytes.NewBuffer(nil)
		cmd := makeValidateRequirementsCmd()
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		view := validateRequirementsView{}
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Valid)
		assert.Equal(t, slug, view.Slug)
		assert.Contains(t, view.Message, "validated")

		// Verify NO published directory is created.
		_, err = os.Stat(filepath.Join(root, "artifacts", "requirements", slug))
		assert.True(t, os.IsNotExist(err))
	})
}

func runValidateRequirementsCommand(t *testing.T, slug string) validateRequirementsView {
	t.Helper()

	out := bytes.NewBuffer(nil)
	cmd := makeValidateRequirementsCmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--json", "--change", slug})
	require.NoError(t, cmd.Execute())

	view := validateRequirementsView{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &view))
	return view
}

func TestValidateRequirementsCommandMissingRequirementsReportsFailure(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "sync no requirements")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)

		// Remove the scaffolded requirements.md so there's nothing to sync.
		_ = os.Remove(filepath.Join(bundleDir, "requirements.md"))

		out := bytes.NewBuffer(nil)
		cmd := makeValidateRequirementsCmd()
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		view := validateRequirementsView{}
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.False(t, view.Valid)
		assert.Contains(t, view.Message, "missing")
	})
}

func TestValidateRequirementsCommandRejectsInvalidRequirements(t *testing.T) {
	tests := []struct {
		name        string
		description string
		content     string
		wantMessage string
	}{
		{
			name:        "malformed requirements",
			description: "sync malformed requirements",
			content:     "# Requirements\n\n## Requirements\n\nNo requirement blocks here.\n",
			wantMessage: "not well-formed",
		},
		{
			name:        "missing stable ids",
			description: "sync missing stable requirement ids",
			content: `# Requirements

## Requirements

### Requirement: Auth
Auth MUST remain deterministic.
`,
			wantMessage: "stable REQ-* IDs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			withWorkspace(t, root, func() {
				require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

				slug := createGovernedRequest(t, root, "L2", tt.description)
				change, err := state.LoadChange(root, slug)
				require.NoError(t, err)

				bundleDir, err := state.GovernedBundleDir(root, change)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(tt.content), 0o644))

				view := runValidateRequirementsCommand(t, slug)
				assert.False(t, view.Valid)
				assert.Contains(t, view.Message, tt.wantMessage)
			})
		})
	}
}

func TestValidateRequirementsCommandAcceptsAllGovernedChanges(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		_ = createActiveNonDiscoveryChange(t, root, "simple governed task")

		cmd := makeValidateRequirementsCmd()
		cmd.SetArgs([]string{"--json"})
		err := cmd.Execute()
		// May succeed or report missing requirements, but should not fail with a governed-mode precondition.
		if err != nil {
			cliErr := asCLIError(err)
			if cliErr != nil {
				assert.NotEqual(t, "validate_requirements_requires_governed", cliErr.ErrorCode)
			}
		}
	})
}

func TestValidateRequirementsCommandUsesDedicatedWorktreeBundleForL3(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		initGitRepoForWorktreeTests(t, root)
		slug := createGovernedRequest(t, root, "L3", "sync l3 worktree")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreePath := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreePath, "-b", branch)
		writeWorktreePreflightEvidence(t, root, slug, worktreePath, branch)

		next := makeNextCmd()
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)

		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)

		reqContent := "# Requirements\n\n### Requirement: Auth\nREQ-001: Auth MUST remain deterministic.\n"
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(reqContent), 0o644))

		view := runValidateRequirementsCommand(t, slug)
		assert.True(t, view.Valid)
		assert.Equal(t, slug, view.Slug)
	})
}
