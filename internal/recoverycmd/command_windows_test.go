//go:build windows

package recoverycmd

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
	if capture := os.Getenv("SLIPWAY_RECOVERY_CAPTURE_ARGUMENTS"); capture != "" {
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

func TestCommandPreservesArgvThroughCommandPromptAndPowerShell(t *testing.T) {
	directory := t.TempDir()
	capture := filepath.Join(directory, "arguments.json")
	stub := filepath.Join(directory, "slipway.exe")
	executable, err := os.Executable()
	require.NoError(t, err)
	binary, err := os.ReadFile(executable)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(stub, binary, 0o700))
	t.Setenv("SLIPWAY_RECOVERY_CAPTURE_ARGUMENTS", capture)
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))

	tests := []struct {
		name   string
		values []string
	}{
		{
			name: "command prompt metacharacters",
			values: []string{
				"run", "answer", "--run", "run with spaces", "--action", `quoted'"界`,
				"--root", filepath.Join(directory, "root & caret^ (data)"), "--text", "line\r\n&^", "--empty", "",
			},
		},
		{
			name: "powershell encoded expansion markers",
			values: []string{
				"run", "answer", "--run", "run%!", "--action", `quoted'"界`,
				"--root", filepath.Join(directory, "root %PATH% !value! & caret^"), "--text", "line\r\n%!&^", "--empty", "",
			},
		},
		{
			name:   "leading argument needs quoting",
			values: []string{`leading '" argument`, "", `trailing\`},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_ = os.Remove(capture)
			rendered := Command(append([]string{"slipway"}, test.values...)...)
			command := exec.Command(os.Getenv("COMSPEC"), "/d", "/v:on", "/s", "/c", rendered)
			output, err := command.CombinedOutput()
			require.NoError(t, err, string(output))
			data, err := os.ReadFile(capture)
			require.NoError(t, err)
			var actual []string
			require.NoError(t, json.Unmarshal(data, &actual))
			assert.Equal(t, test.values, actual)
		})
	}
}

func TestQuoteWindowsCommandArgumentDoublesTrailingBackslashes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, `"C:\repo\\"`, quoteWindowsCommandArgument(`C:\repo\`))
}
