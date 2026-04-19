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

func TestExecuteGovernedEntryDoesNotShortCircuitToAdvisory(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		stdout, _, err := runRootCommand([]string{"new", "how do i set this up?"})
		require.NoError(t, err)
		assert.NotContains(t, stdout, "no governed workflow created")
		if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
			payload := decodeJSONMap(t, stdout)
			assert.Equal(t, "governed", payload["mode"])
			assert.Equal(t, true, payload["workflow_created"])
		}

		changes, err := state.ListChanges(root)
		require.NoError(t, err)
		require.Len(t, changes, 1)
		assert.Equal(t, model.ChangeStatusActive, changes[0].Status)
	})
}

func TestExecuteFailureEnvelopeInvalidUsage(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		createIntakeChangeFixture(t, root, "fix login timeout")

		_, stderr, err := runRootCommand([]string{"pivot", "--invalid-flag"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryInvalidUsage, payload.Category)
		assert.Equal(t, exitCodeInvalidUsage, payload.ExitCode)
	})
}

func TestExecuteFailureEnvelopeStateIntegrityForCorruptConfig(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createIntakeChangeFixture(t, root, "fix login timeout")
		require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("defaults: ["), 0o644))

		_, stderr, err := runRootCommand([]string{"next"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryStateIntegrity, payload.Category)
		assert.Equal(t, exitCodeStateIntegrity, payload.ExitCode)
		assert.Contains(t, payload.Remediation, "slipway repair")
	})
}

func TestExecuteFailureEnvelopeStateIntegrityForMalformedGovernanceSkill(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "review malformed governance skill")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		skillPath := filepath.Join(root, ".codex", "skills", "slipway-code-quality-review", "SKILL.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(skillPath, []byte("---\nname: code-quality-review\ndescription: [\n---\n"), 0o644))

		_, stderr, err := runRootCommand([]string{"next", "--json"})
		require.Error(t, err)

		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryStateIntegrity, payload.Category)
		assert.Equal(t, exitCodeStateIntegrity, payload.ExitCode)
		assert.Equal(t, "skill_registry_invalid", payload.ErrorCode)
		assert.Contains(t, payload.Message, "evaluate next skill evidence")
		assert.Contains(t, payload.Message, "parse skill frontmatter")
		assert.Contains(t, payload.Remediation, "repair")
	})
}

func assertMalformedGovernanceSkillCommandFailsStateIntegrity(t *testing.T, command, description, wantMessage string) {
	t.Helper()

	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", description)
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		if command == "review" {
			writePassingExecutionSummary(t, root, slug, 1, "t-01")
		}

		skillPath := filepath.Join(root, ".codex", "skills", "slipway-code-quality-review", "SKILL.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(skillPath, []byte("---\nname: code-quality-review\ndescription: [\n---\n"), 0o644))

		_, stderr, err := runRootCommand([]string{command})
		require.Error(t, err)

		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryStateIntegrity, payload.Category)
		assert.Equal(t, exitCodeStateIntegrity, payload.ExitCode)
		assert.Equal(t, "skill_registry_invalid", payload.ErrorCode)
		assert.Contains(t, payload.Message, wantMessage)
		assert.Contains(t, payload.Message, "parse skill frontmatter")
		assert.Contains(t, payload.Remediation, "repair")
	})
}

func TestExecuteFailureEnvelopeStateIntegrityForMalformedGovernanceSkillStateCommands(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		description string
		wantMessage string
	}{
		{
			name:        "review",
			command:     "review",
			description: "review malformed governance skill through review",
			wantMessage: "evaluate review prerequisites",
		},
		{
			name:        "validate",
			command:     "validate",
			description: "review malformed governance skill through validate",
			wantMessage: "validate readiness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMalformedGovernanceSkillCommandFailsStateIntegrity(t, tt.command, tt.description, tt.wantMessage)
		})
	}
}

func TestExecuteFailureEnvelopeToolAmbiguityForNextJSON(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex", "claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "next json ambiguity contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		_, stderr, err := runRootCommand([]string{"next", "--json"})
		require.Error(t, err)

		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryPrecondition, payload.Category)
		assert.Equal(t, exitCodePrecondition, payload.ExitCode)
		assert.Equal(t, "tool_ambiguity", payload.ErrorCode)
		assert.Contains(t, payload.Remediation, "SLIPWAY_TOOL=<tool>")

		rawDetected, ok := payload.Details["detected_adapters"].([]any)
		require.True(t, ok, "detected_adapters must be a JSON array")
		detected := make([]string, 0, len(rawDetected))
		for _, item := range rawDetected {
			detected = append(detected, item.(string))
		}
		assert.ElementsMatch(t, []string{"claude", "codex"}, detected)
	})
}

func TestExecuteFailureEnvelopeGovernanceBlocked(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createIntakeChangeFixture(t, root, "fix login timeout")

		_, stderr, err := runRootCommand([]string{"done"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryGovernanceBlocked, payload.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, payload.ExitCode)
	})
}

func TestExecuteFailureEnvelopeUnknownHelpTopic(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		_, stderr, err := runRootCommand([]string{"help", "help"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryInvalidUsage, payload.Category)
		assert.Equal(t, exitCodeInvalidUsage, payload.ExitCode)
		assert.Equal(t, "unknown_help_topic", payload.ErrorCode)
	})
}

func TestExecuteFailureEnvelopeStateLockTimeout(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createIntakeChangeFixture(t, root, "fix login timeout")

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		cfg.Execution.LockStaleAfterSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		lockPath := state.ChangeStateLockPath(root, slug)
		stopLockHolder := startStateLockHolder(t, lockPath)
		defer stopLockHolder()

		_, stderr, err := runRootCommand([]string{"next"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryPrecondition, payload.Category)
		assert.Equal(t, exitCodePrecondition, payload.ExitCode)
		assert.Equal(t, "state_lock_timeout", payload.ErrorCode)
	})
}

func TestTypedCLIErrorHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          *CLIError
		wantCategory FailureCategory
		wantExitCode int
	}{
		{
			name:         "invalid usage",
			err:          newInvalidUsageError("invalid_format", "bad format", "use json", nil),
			wantCategory: categoryInvalidUsage,
			wantExitCode: exitCodeInvalidUsage,
		},
		{
			name:         "precondition",
			err:          newPreconditionError("no_active_change", "no request", "create one", "req-1", map[string]any{"scope": "active"}),
			wantCategory: categoryPrecondition,
			wantExitCode: exitCodePrecondition,
		},
		{
			name:         "state integrity",
			err:          newStateIntegrityError("broken_state", "state broken", "run repair", "", nil),
			wantCategory: categoryStateIntegrity,
			wantExitCode: exitCodeStateIntegrity,
		},
		{
			name:         "governance blocked",
			err:          newGovernanceBlockedError("not_done_ready", "blocked", "finish review", "", nil),
			wantCategory: categoryGovernanceBlocked,
			wantExitCode: exitCodeGovernanceBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.err)
			assert.Equal(t, tt.wantCategory, tt.err.Category)
			assert.Equal(t, tt.wantExitCode, tt.err.ExitCode)
		})
	}
}

func runRootCommand(args []string) (string, string, error) {
	rootCmd = newRootCmd()

	var out bytes.Buffer
	var errOut bytes.Buffer

	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs(args)
	err := Execute()
	return out.String(), errOut.String(), err
}
