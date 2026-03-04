package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/speclane/internal/bootstrap"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommandRejectsNonSplnIntentWithoutState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"how do i set this up?"})
		err := cmd.Execute()
		require.Error(t, err)
		var cliErr *CLIError
		require.True(t, errors.As(err, &cliErr))
		assert.Equal(t, "non_spln_intent", cliErr.ErrorCode)
		assert.Equal(t, categoryPrecondition, cliErr.Category)

		admissionFiles, err := os.ReadDir(filepath.Join(root, ".spln", "runtime", "admissions"))
		require.NoError(t, err)
		assert.Len(t, admissionFiles, 0)
		changeFiles, err := os.ReadDir(filepath.Join(root, ".spln", "runtime", "changes"))
		require.NoError(t, err)
		assert.Len(t, changeFiles, 0)
	})
}

func TestNewCommandUnclearLowConfidenceClarificationRejected(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"unclear scope maybe something later"})
		err := cmd.Execute()
		require.Error(t, err)

		var cliErr *CLIError
		require.True(t, errors.As(err, &cliErr))
		assert.Equal(t, "non_spln_intent", cliErr.ErrorCode)

		admissionFiles, readErr := os.ReadDir(filepath.Join(root, ".spln", "runtime", "admissions"))
		require.NoError(t, readErr)
		assert.Len(t, admissionFiles, 0)
	})
}

func TestNewCommandL1AdmissionOnly(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "admissions"))
		assert.True(t, model.IsUUIDv7(requestID))
		admission, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL1, admission.Level)
		assert.Equal(t, model.StateS6RunWaves, admission.CurrentState)
		assert.Equal(t, model.AdmissionStatusActive, admission.AdmissionStatus)

		changeFiles, err := os.ReadDir(filepath.Join(root, ".spln", "runtime", "changes"))
		require.NoError(t, err)
		assert.Len(t, changeFiles, 0)
	})
}

func TestNewCommandMultilingualExecutableAccepted(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"请 update auth middleware timeout strategy"})
		require.NoError(t, cmd.Execute())

		records, err := state.DiscoverActiveRecords(root)
		require.NoError(t, err)
		require.Len(t, records, 1)
	})
}

func TestNewCommandL2CreatesGovernedStateAndBundle(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, cmd.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))
		assert.True(t, model.IsUUIDv7(requestID))
		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL2, change.Level)
		assert.Equal(t, model.StateS4SpecBundle, change.CurrentState)
		assert.Equal(t, model.ChangeStatusActive, change.ChangeStatus)

		admission, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.AdmissionStatusSealedHandoff, admission.AdmissionStatus)
		assert.Equal(t, model.StateS1Analyze, admission.CurrentState)

		_, err = os.Stat(filepath.Join(root, "aircraft", "changes", change.Slug, "change.yaml"))
		require.NoError(t, err)
	})
}

func TestNewCommandL3CreatesGovernedDiscoverLanding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"--level", "L3", "introduce guarded auth changes"})
		require.NoError(t, cmd.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))
		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL3, change.Level)
		assert.Equal(t, model.StateS2Discover, change.CurrentState)

		admission, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.AdmissionStatusSealedHandoff, admission.AdmissionStatus)
		assert.Equal(t, model.StateS1Analyze, admission.CurrentState)
	})
}

func TestNewCommandFixedConflictFailsBeforeStateCreation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"--level", "L1", "update auth policy"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fixed level blocked")

		admissionFiles, err := os.ReadDir(filepath.Join(root, ".spln", "runtime", "admissions"))
		require.NoError(t, err)
		assert.Len(t, admissionFiles, 0)
	})
}

func TestNewCommandUsesConfigDefaultLevelWhenFlagOmitted(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cfg := model.DefaultConfig()
		cfg.Defaults.LevelMode = model.LevelMode(model.LevelL2)
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"implement service guardrails"})
		require.NoError(t, cmd.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))
		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL2, change.Level)
	})
}

func TestNewCommandInvalidConfigLevelModeFallsBackToAuto(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cfgPath := filepath.Join(root, ".spln", "config.yaml")
		raw, err := os.ReadFile(cfgPath)
		require.NoError(t, err)
		invalid := strings.ReplaceAll(string(raw), "level_mode: auto", "level_mode: INVALID")
		require.NoError(t, os.WriteFile(cfgPath, []byte(invalid), 0o644))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"not sure refactor service modules"})
		require.NoError(t, cmd.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))
		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL3, change.Level)
	})
}

func TestNewCommandExecutableUnknownsRouteToL3(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newNewCmd()
		cmd.SetArgs([]string{"not sure fix login timeout behavior"})
		require.NoError(t, cmd.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))
		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL3, change.Level)
		assert.Equal(t, model.StateS2Discover, change.CurrentState)
	})
}

func withWorkspace(t *testing.T, root string, fn func()) {
	t.Helper()
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	defer func() {
		_ = os.Chdir(previousWD)
	}()
	fn()
}

func singleRequestID(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return entries[0].Name()[:len(entries[0].Name())-len(".yaml")]
}
