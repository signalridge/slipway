//go:build windows

package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(testingMain *testing.M) {
	if capture := os.Getenv("SLIPWAY_TEST_CAPTURE_ARGUMENTS"); capture != "" {
		encoded, err := json.Marshal(os.Args[1:])
		if err != nil {
			os.Exit(97)
		}
		if err := os.WriteFile(capture, encoded, 0o600); err != nil {
			os.Exit(98)
		}
		os.Exit(0)
	}
	os.Exit(testingMain.Run())
}

func TestInstallNextCommandExecutesThroughCommandPrompt(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "arguments.json")
	stub := filepath.Join(dir, "slipway.exe")
	executable, err := os.Executable()
	require.NoError(t, err)
	binary, err := os.ReadFile(executable)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(stub, binary, 0o700))
	t.Setenv("SLIPWAY_TEST_CAPTURE_ARGUMENTS", capture)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SLIPWAY_DELAYED", "wrong value & echo injected")

	for _, test := range []struct {
		name string
		root string
	}{
		{name: "command metacharacter", root: filepath.Join(dir, "repo & user files")},
		{name: "variable expansion markers", root: filepath.Join(dir, "repo %PATH% !SLIPWAY_DELAYED!")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.Remove(capture); err != nil {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
			command := exec.Command(os.Getenv("COMSPEC"), "/d", "/v:on", "/s", "/c", recoveryCommand("slipway", "install", "--root", test.root, "--refresh", "--tool", "claude"))
			output, err := command.CombinedOutput()
			require.NoError(t, err, string(output))
			data, err := os.ReadFile(capture)
			require.NoError(t, err)
			var arguments []string
			require.NoError(t, json.Unmarshal(data, &arguments))
			assert.Equal(t, []string{"install", "--root", test.root, "--refresh", "--tool", "claude"}, arguments)
		})
	}
}
