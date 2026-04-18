package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresetCommandConfirmsPendingPresetAndScaffoldsBundle(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())

		var buf bytes.Buffer
		cmd := makePresetCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "strict"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.Equal(t, model.WorkflowPresetStrict, change.WorkflowPreset)
		assert.False(t, change.WorkflowPresetConfirmationPending())
		assert.Equal(t, "strict", payload["workflow_preset"])
		assert.FileExists(t, filepath.Join(root, "artifacts", "changes", slug, "intent.md"))
		assert.FileExists(t, filepath.Join(root, "artifacts", "changes", slug, "requirements.md"))
		assert.FileExists(t, filepath.Join(root, "artifacts", "changes", slug, "tasks.md"))
		assert.FileExists(t, filepath.Join(root, "artifacts", "changes", slug, "assurance.md"))
	})
}

func TestPresetCommandPendingFromDocSeedsArtifactsFromIntentSections(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		require.NoError(t, os.WriteFile(docPath, []byte(`# Session timeout

## In Scope
- expire idle sessions after 15 minutes
- preserve MFA enforcement for admin sessions

## Constraints
- keep existing middleware contract

## Acceptance Criteria
- verify idle sessions expire after 15 minutes
`), 0o644))

		create := makeNewCmd()
		create.SetArgs([]string{"--from-doc", docPath, "update auth middleware timeout strategy"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.True(t, change.WorkflowPresetConfirmationPending())

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"strict"})
		require.NoError(t, cmd.Execute())

		requirementsRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "requirements.md"))
		require.NoError(t, err)
		assert.Contains(t, string(requirementsRaw), "15 minutes")
		assert.Contains(t, string(requirementsRaw), "preserve mfa enforcement")

		decisionRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "decision.md"))
		require.NoError(t, err)
		assert.Contains(t, string(decisionRaw), "keep existing middleware contract")

		tasksRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "tasks.md"))
		require.NoError(t, err)
		assert.Contains(t, string(tasksRaw), "15 minutes")
	})
}

func TestPresetCommandPendingFromLongDocRetainsAcceptanceSections(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		doc := `# Session timeout

## In Scope
- expire idle sessions after 15 minutes

## Constraints
- keep existing middleware contract

` + strings.Repeat("Constraint rationale that should not erase later acceptance content.\n", 120) + `
## Acceptance Criteria
- verify idle sessions expire after 15 minutes
`
		require.NoError(t, os.WriteFile(docPath, []byte(doc), 0o644))

		create := makeNewCmd()
		create.SetArgs([]string{"--from-doc", docPath, "update auth middleware timeout strategy"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.True(t, change.WorkflowPresetConfirmationPending())

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"strict"})
		require.NoError(t, cmd.Execute())

		tasksRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "tasks.md"))
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(string(tasksRaw)), "verify idle sessions expire after 15 minutes")
	})
}

func TestPresetCommandPrefersCanonicalIntentSectionsOverSourceDocument(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		require.NoError(t, os.WriteFile(docPath, []byte(`# Session timeout

## In Scope
- expire idle sessions after 15 minutes

## Constraints
- keep existing middleware contract
`), 0o644))

		create := makeNewCmd()
		create.SetArgs([]string{"--from-doc", docPath, "update auth middleware timeout strategy"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		intentRaw, err := os.ReadFile(intentPath)
		require.NoError(t, err)

		const oldCanonicalScope = "## In Scope\n- expire idle sessions after 15 minutes\n\n## Out of Scope"
		const newCanonicalScope = "## In Scope\n- expire idle sessions after 30 minutes\n\n## Out of Scope"
		intentUpdated := strings.Replace(string(intentRaw), oldCanonicalScope, newCanonicalScope, 1)
		require.NoError(t, os.WriteFile(intentPath, []byte(intentUpdated), 0o644))

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"strict"})
		require.NoError(t, cmd.Execute())

		requirementsRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "requirements.md"))
		require.NoError(t, err)
		assert.Contains(t, string(requirementsRaw), "30 minutes")
		assert.NotContains(t, string(requirementsRaw), "15 minutes")
	})
}

func TestPresetCommandRejectsInvalidPreset(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"fast"})
		err := cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "invalid_preset", cliErr.ErrorCode)
	})
}

func TestPresetCommandCanUpgradeExistingConfirmedPresetDuringPlanning(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"--preset", "light", "fix login timeout"})
		require.NoError(t, create.Execute())

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"strict"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.Equal(t, model.WorkflowPresetStrict, change.WorkflowPreset)
		assert.False(t, change.WorkflowPresetConfirmationPending())
	})
}

func TestPresetCommandRejectsDowngradeAfterLeavingS1Plan(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"--preset", "strict", "refactor auth"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Simulate having advanced past S1_PLAN.
		change.CurrentState = model.StateS3Review
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"light"})
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "preset_downgrade_rejected", cliErr.ErrorCode)

		// Verify the original preset was not changed.
		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowPresetStrict, reloaded.WorkflowPreset)
	})
}

func TestPresetCommandAllowsUpgradeAfterLeavingS1Plan(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"--preset", "light", "fix login timeout"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Simulate having advanced past S1_PLAN.
		change.CurrentState = model.StateS2Execute
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"strict"})
		require.NoError(t, cmd.Execute())

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowPresetStrict, reloaded.WorkflowPreset)
	})
}

func TestPresetCommandRequiresActiveChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"light"})
		err := cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "no_active_change", cliErr.ErrorCode)
	})
}

func TestPresetCommandPreservesChangeFileAfterConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"light"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		raw, err := os.ReadFile(state.BundleChangeFilePath(root, slug))
		require.NoError(t, err)
		assert.Contains(t, string(raw), "workflow_preset: light")
		assert.NotContains(t, string(raw), "suggested_workflow_preset:")
	})
}

func TestPresetCommandScaffoldFailureRollsBackToPendingConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"implement auth timeout strategy"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Advance past intake so preset scaffold would attempt governed bundle creation.
		change.CurrentState = model.StateS1Plan
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepResearch
		require.NoError(t, state.SaveChange(root, change))

		// Make the bundle directory unwritable so scaffold fails.
		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.Chmod(bundleDir, 0o555))
		defer func() { _ = os.Chmod(bundleDir, 0o755) }()

		cmd := makePresetCmd()
		cmd.SetArgs([]string{"light"})
		err = cmd.Execute()
		require.Error(t, err)

		_ = os.Chmod(bundleDir, 0o755)
		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, reloaded.WorkflowPresetConfirmationPending(),
			"failed scaffold must revert the change to pending preset confirmation")
		assert.Empty(t, reloaded.WorkflowPreset)
	})
}

func TestPresetCommandReScaffoldFailurePreservesConfirmedPreset(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"implement auth timeout strategy"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		presetCmd := makePresetCmd()
		presetCmd.SetArgs([]string{"strict"})
		require.NoError(t, presetCmd.Execute())

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.WorkflowPresetStrict, change.WorkflowPreset)
		require.False(t, change.WorkflowPresetConfirmationPending())

		// Advance past intake to S1_PLAN so re-scaffold triggers.
		change.CurrentState = model.StateS1Plan
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepResearch
		require.NoError(t, state.SaveChange(root, change))

		// Remove a governed artifact then make bundle dir unwritable.
		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		_ = os.Remove(filepath.Join(bundleDir, "requirements.md"))
		require.NoError(t, os.Chmod(bundleDir, 0o555))
		defer func() { _ = os.Chmod(bundleDir, 0o755) }()

		retryCmd := makePresetCmd()
		retryCmd.SetArgs([]string{"strict"})
		err = retryCmd.Execute()
		require.Error(t, err, "re-scaffold should fail because bundle dir is not writable")

		_ = os.Chmod(bundleDir, 0o755)
		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowPresetStrict, reloaded.WorkflowPreset,
			"confirmed preset must be preserved on re-scaffold failure")
		assert.False(t, reloaded.WorkflowPresetConfirmationPending(),
			"change must NOT revert to pending when preset was already confirmed")
		assert.Empty(t, reloaded.SuggestedWorkflowPreset)
	})
}

func TestRestorePresetOnScaffoldFailureReturnsRestoreError(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	badRoot := filepath.Join(tmp, "root-file")
	require.NoError(t, os.WriteFile(badRoot, []byte("not-a-directory"), 0o644))

	change := model.NewChange("preset-restore-error")
	change.WorkflowPreset = model.WorkflowPresetLight

	err := restorePresetOnScaffoldFailure(badRoot, &change, "", "")
	require.Error(t, err)
	assert.Empty(t, change.WorkflowPreset)
	assert.Equal(t, model.WorkflowPresetLight, change.SuggestedWorkflowPreset)
}
