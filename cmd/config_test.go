package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runConfigCmd executes a fresh `slipway config` command rooted at root with the
// given args, capturing stdout and stderr separately. It returns the RunE error
// (nil on success) so callers can assert both output shape and exit semantics.
func runConfigCmd(t *testing.T, root string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := commandForRoot(t, root, makeConfigCmd())
	cmd.SetArgs(args)
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestConfigListEnumeratesCatalogKeys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Bare `config` and `config list` both enumerate every catalog key.
		for _, args := range [][]string{{}, {"list"}} {
			out, errOut, err := runConfigCmd(t, root, args...)
			require.NoError(t, err, "config %v", args)
			assert.Empty(t, errOut, "config %v wrote to stderr", args)
			for _, entry := range model.ConfigCatalog() {
				assert.Contains(t, out, entry.Name, "config %v missing key %q", args, entry.Name)
			}
			// Spot-check that type/scope columns are rendered, not just names.
			assert.Contains(t, out, "execution.lock_wait_timeout_seconds")
			assert.Contains(t, out, "bool")
		}
	})
}

func TestConfigListJSONShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		out, errOut, err := runConfigCmd(t, root, "list", "--json")
		require.NoError(t, err)
		assert.Empty(t, errOut, "--json must not write to stderr")

		var entries []model.ConfigCatalogEntry
		require.NoError(t, json.Unmarshal([]byte(out), &entries), "list --json must emit valid JSON on stdout")
		require.NotEmpty(t, entries)

		byName := map[string]model.ConfigCatalogEntry{}
		for _, e := range entries {
			byName[e.Name] = e
		}
		entry, ok := byName["defaults.artifact_schema"]
		require.True(t, ok, "expected defaults.artifact_schema in JSON catalog")
		assert.Equal(t, "string", entry.Type)
		assert.Equal(t, "defaults", entry.Scope)
		assert.Equal(t, []string{"core", "expanded", "custom"}, entry.AllowedValues)

		entry, ok = byName["execution.auto"]
		require.True(t, ok, "expected execution.auto in JSON catalog")
		assert.Equal(t, "bool", entry.Type)
		assert.Equal(t, "false", entry.Default, "false is a real default value and must not be omitted")
	})
}

func TestConfigHelpDisclosesSetRewrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		out, errOut, err := runConfigCmd(t, root, "--help")
		require.NoError(t, err)
		assert.Empty(t, errOut)
		assert.Contains(t, out, "rewrites .slipway.yaml as deterministic YAML")
		assert.Contains(t, out, "comments and the")
		assert.Contains(t, out, "original key ordering are not preserved")
	})
}

func TestConfigGetHappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 42
		cfg.Defaults.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		out, errOut, err := runConfigCmd(t, root, "get", "execution.lock_wait_timeout_seconds")
		require.NoError(t, err)
		assert.Empty(t, errOut)
		assert.Equal(t, "42", strings.TrimSpace(out))

		out, _, err = runConfigCmd(t, root, "get", "defaults.artifact_schema")
		require.NoError(t, err)
		assert.Equal(t, "expanded", strings.TrimSpace(out))
	})
}

func TestConfigGetJSONShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 7
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		out, errOut, err := runConfigCmd(t, root, "get", "execution.lock_wait_timeout_seconds", "--json")
		require.NoError(t, err)
		assert.Empty(t, errOut, "get --json must not write to stderr")

		var payload struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &payload), "get --json must emit valid JSON on stdout")
		assert.Equal(t, "execution.lock_wait_timeout_seconds", payload.Key)
		assert.Equal(t, "7", payload.Value)
	})
}

func TestConfigJSONWarnsUnknownTopLevelOnStderr(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`unknown_block:
  retained: true
execution:
  lock_wait_timeout_seconds: 37
`), 0o644))

		out, errOut, err := runConfigCmd(t, root, "get", "execution.lock_wait_timeout_seconds", "--json")
		require.NoError(t, err)
		assert.Contains(t, errOut, "warning:")
		assert.Contains(t, errOut, "unknown_block")

		var payload configGetView
		require.NoError(t, json.Unmarshal([]byte(out), &payload), "warning must not corrupt JSON stdout")
		assert.Equal(t, "execution.lock_wait_timeout_seconds", payload.Key)
		assert.Equal(t, "37", payload.Value, "valid config remainder must still load")
	})
}

func TestConfigGetUnknownKeyErrorsToStderrNonZero(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		out, _, err := runConfigCmd(t, root, "get", "execution.does_not_exist")
		require.Error(t, err, "unknown key must be a non-zero exit, not a stdout message")

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.NotEqual(t, 0, cliErr.ExitCode)
		// Assert the unknown-key identity via STABLE structured fields, not Message
		// prose: the stable error code plus the offending key carried in Details.
		assert.Equal(t, "config_key_unknown", cliErr.ErrorCode)
		assert.Equal(t, "execution.does_not_exist", cliErr.Details["key"], "error must name the unknown key in Details")
		// Nothing valid should be printed to stdout for the unknown key.
		assert.Empty(t, strings.TrimSpace(out), "unknown-key get must not print a value to stdout")

		// And the same with --json: still a non-zero error, no stray stdout JSON value.
		jsonOut, _, jsonErr := runConfigCmd(t, root, "get", "execution.does_not_exist", "--json")
		require.Error(t, jsonErr)
		assert.Empty(t, strings.TrimSpace(jsonOut))
	})
}

func TestConfigSectionKeysRejectedAsUnknown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		before, err := os.ReadFile(state.ConfigPath(root))
		require.NoError(t, err)

		for _, key := range []string{"execution", "governance.thresholds", "context"} {
			t.Run("get/"+key, func(t *testing.T) {
				out, _, err := runConfigCmd(t, root, "get", key)
				require.Error(t, err, "section key must be a non-zero exit, not a rendered struct")
				cliErr := asCLIError(err)
				require.NotNil(t, cliErr)
				assert.Equal(t, "config_key_unknown", cliErr.ErrorCode)
				assert.Equal(t, key, cliErr.Details["key"])
				assert.Empty(t, strings.TrimSpace(out), "section-key get must not print a value to stdout")
			})

			t.Run("set/"+key, func(t *testing.T) {
				_, _, err := runConfigCmd(t, root, "set", key, "true")
				require.Error(t, err, "section key must be rejected as unknown, not parsed as a value")
				cliErr := asCLIError(err)
				require.NotNil(t, cliErr)
				assert.Equal(t, "config_key_unknown", cliErr.ErrorCode)
				assert.Equal(t, key, cliErr.Details["key"])
			})
		}

		after, err := os.ReadFile(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "section-key set must leave the config file unchanged")
	})
}

func TestConfigSetHappyPathRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		_, errOut, err := runConfigCmd(t, root, "set", "execution.lock_wait_timeout_seconds", "55")
		require.NoError(t, err)
		assert.Empty(t, errOut)

		// Persisted and reloadable via the model loader.
		reloaded, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, 55, reloaded.Execution.LockWaitTimeoutSeconds)

		// And visible through `config get` (full round-trip through the command).
		out, _, err := runConfigCmd(t, root, "get", "execution.lock_wait_timeout_seconds")
		require.NoError(t, err)
		assert.Equal(t, "55", strings.TrimSpace(out))
	})
}

func TestConfigSetEchoesPersistedEffectiveValue(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		tests := []struct {
			name  string
			key   string
			value string
			want  string
		}{
			{
				name:  "non-positive timeout normalizes to default",
				key:   "execution.lock_wait_timeout_seconds",
				value: "0",
				want:  "10",
			},
			{
				name:  "non-positive audit iterations normalize to default",
				key:   "execution.max_plan_audit_iterations",
				value: "0",
				want:  "3",
			},
			{
				name:  "boolean input renders canonically",
				key:   "execution.auto",
				value: "TRUE",
				want:  "true",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				out, errOut, err := runConfigCmd(t, root, "set", tt.key, tt.value)
				require.NoError(t, err)
				assert.Empty(t, errOut)
				assert.Equal(t, "set "+tt.key+" = "+tt.want, strings.TrimSpace(out))

				getOut, getErrOut, err := runConfigCmd(t, root, "get", tt.key)
				require.NoError(t, err)
				assert.Empty(t, getErrOut)
				assert.Equal(t, tt.want, strings.TrimSpace(getOut))
			})
		}
	})
}

func TestConfigSetPreservesOtherKeys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Seed a config that carries a governance key alongside execution keys.
		cfg := model.DefaultConfig()
		cfg.Governance.DefaultPreset = model.WorkflowPresetStrict
		cfg.Execution.LockWaitTimeoutSeconds = 11
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		// Set a single unrelated key.
		_, _, err := runConfigCmd(t, root, "set", "execution.max_plan_audit_iterations", "5")
		require.NoError(t, err)

		reloaded, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		// The set key changed...
		assert.Equal(t, 5, reloaded.Execution.MaxPlanAuditIterations)
		// ...and the pre-existing keys in OTHER sections survived.
		assert.Equal(t, model.WorkflowPresetStrict, reloaded.Governance.DefaultPreset)
		assert.Equal(t, 11, reloaded.Execution.LockWaitTimeoutSeconds)
	})
}

func TestConfigSetInvalidRejectsWithFileUnchanged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Seed a known config and capture its exact bytes.
		cfg := model.DefaultConfig()
		cfg.Defaults.ArtifactSchema = model.ArtifactSchemaCore
		cfg.Governance.DefaultPreset = model.WorkflowPresetStandard
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		before, err := os.ReadFile(state.ConfigPath(root))
		require.NoError(t, err)

		// Invalid enum value must be rejected via Config.Validate().
		_, _, setErr := runConfigCmd(t, root, "set", "defaults.artifact_schema", "bogus")
		require.Error(t, setErr, "invalid value must be a non-zero exit")
		cliErr := asCLIError(setErr)
		require.NotNil(t, cliErr)
		assert.NotEqual(t, 0, cliErr.ExitCode)
		assert.Equal(t, "config_value_invalid", cliErr.ErrorCode)
		assert.Equal(t, "defaults.artifact_schema", cliErr.Details["key"])

		after, err := os.ReadFile(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "invalid set must leave the config file byte-for-byte unchanged")

		// Unknown key on set is also rejected, file still unchanged. An unknown key
		// reports the same stable code as `config get` (config_key_unknown), not
		// config_value_invalid, so the read/write paths agree on what is unknown.
		_, _, unknownErr := runConfigCmd(t, root, "set", "execution.nope", "1")
		require.Error(t, unknownErr)
		unknownCLI := asCLIError(unknownErr)
		require.NotNil(t, unknownCLI)
		assert.Equal(t, "config_key_unknown", unknownCLI.ErrorCode)
		assert.Equal(t, "execution.nope", unknownCLI.Details["key"])

		afterUnknown, err := os.ReadFile(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, string(before), string(afterUnknown), "unknown-key set must leave the config file unchanged")

		// Non-integer for an int field is rejected too.
		_, _, typeErr := runConfigCmd(t, root, "set", "execution.lock_wait_timeout_seconds", "abc")
		require.Error(t, typeErr)
	})
}

func TestConfigSetRequiresKeyAndValue(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Missing value argument must be an arg-count error, not a panic.
		_, _, err := runConfigCmd(t, root, "set", "execution.lock_wait_timeout_seconds")
		require.Error(t, err)
	})
}

func TestConfigCommandRegisteredOnRoot(t *testing.T) {
	t.Parallel()
	// The live root command (the one Execute() drives) must expose `config`.
	var found bool
	for _, c := range rootCmd.Commands() {
		if c.Name() == "config" {
			found = true
			assert.NotEmpty(t, c.Short, "config command must carry a non-empty Short description")
		}
	}
	assert.True(t, found, "config command must be registered on the root command")
}

// TestConfigSetPersistsIsolatedGovernancePointer is the regression for the
// silent-loss bug: `config set governance.auto_provision_worktree` reported
// success but ToYAML dropped the lone governance key, so the value never hit
// disk. Assert it persists through the full set -> reload -> get round-trip.
func TestConfigSetPersistsIsolatedGovernancePointer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		_, errOut, err := runConfigCmd(t, root, "set", "governance.auto_provision_worktree", "false")
		require.NoError(t, err)
		assert.Empty(t, errOut)

		reloaded, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		require.NotNil(t, reloaded.Governance.AutoProvisionWorktree, "value must be persisted, not dropped")
		assert.False(t, reloaded.Governance.AutoProvisionWorktreeEnabled())

		out, _, err := runConfigCmd(t, root, "get", "governance.auto_provision_worktree")
		require.NoError(t, err)
		assert.Equal(t, "false", strings.TrimSpace(out))
	})
}

// TestConfigSetPersistsIsolatedContextLeaf is the regression for the context
// half of the same bug: context.recent_work was dropped on save when it was the
// only context leaf.
func TestConfigSetPersistsIsolatedContextLeaf(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		_, _, err := runConfigCmd(t, root, "set", "context.recent_work", "shipped PR1")
		require.NoError(t, err)

		reloaded, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, "shipped PR1", reloaded.Context.RecentWork, "value must be persisted, not dropped")

		out, _, err := runConfigCmd(t, root, "get", "context.recent_work")
		require.NoError(t, err)
		assert.Equal(t, "shipped PR1", strings.TrimSpace(out))
	})
}

// TestConfigListTextSurfacesEffectiveDefaultsAndDescriptions asserts the text
// table reports the resolved effective default (not a misleading "-"/false) and
// carries the DESCRIPTION column.
func TestConfigListTextSurfacesEffectiveDefaultsAndDescriptions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		out, _, err := runConfigCmd(t, root, "list")
		require.NoError(t, err)
		assert.Contains(t, out, "DESCRIPTION", "list text table must carry a description column")
		assert.Contains(t, out, "Whether `slipway new` provisions", "descriptions must render in the text table")
		// auto_provision_worktree resolves to its effective default (enabled), not
		// the bare zero value, on the same line as the key.
		assertConfigListLineContains(t, out, "execution.auto", "false")
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "governance.auto_provision_worktree") {
				assert.Contains(t, line, "true", "auto_provision_worktree default must render as enabled")
			}
		}
		assertConfigListLineContains(t, out, "validation.enforce_rfc2119", "false")
	})
}

func assertConfigListLineContains(t *testing.T, output, key, want string) {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, key) {
			assert.Contains(t, line, want)
			return
		}
	}
	t.Fatalf("config list output missing line for %s:\n%s", key, output)
}
