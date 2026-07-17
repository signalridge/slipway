//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallNextCommandExecutesThroughPOSIXShell(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "arguments.txt")
	stub := filepath.Join(dir, "slipway")
	script := "#!/bin/sh\n: > \"$CAPTURE\"\nfor argument do\n  printf '%s\\n' \"$argument\" >> \"$CAPTURE\"\ndone\n"
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o700))
	t.Setenv("CAPTURE", capture)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := filepath.Join(dir, "repo with user's files")
	command := exec.Command("sh", "-c", recoveryCommand("slipway", "install", "--root", root, "--refresh", "--tool", "claude"))
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(capture)
	require.NoError(t, err)
	arguments := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	assert.Equal(t, []string{"install", "--root", root, "--refresh", "--tool", "claude"}, arguments)
}
