package tmpl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStartLauncherDelegatesToCompiledHookCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX launcher execution is not portable on Windows")
	}

	root := t.TempDir()
	logPath, binDir := installLauncherTestSlipway(t, root, `#!/bin/sh
printf '%s|%s\n' "$PWD" "$*" >> "$SLIPWAY_HOOK_LOG"
if [ "$*" = "hook session-start --tool claude" ]; then
  printf '<slipway-session-start tool="claude">ok</slipway-session-start>'
fi
`)
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID":       "claude",
		"LaunchPrefix": "slipway",
		"ProbeBin":     "slipway",
	})

	shPath, err := exec.LookPath("sh")
	require.NoError(t, err)
	cmd := exec.Command(shPath, scriptPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), `<slipway-session-start tool="claude">ok</slipway-session-start>`)
	logContent := readHookLog(t, logPath)
	assert.Contains(t, logContent, "|hook session-start --tool claude")
	assert.NotContains(t, logContent, "|root")
	assert.NotContains(t, logContent, "|next --json")
}

func TestSessionStartLauncherNoopsWhenSlipwayMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX launcher execution is not portable on Windows")
	}

	root := t.TempDir()
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID":       "claude",
		"LaunchPrefix": "slipway",
		"ProbeBin":     "slipway",
	})

	shPath, err := exec.LookPath("sh")
	require.NoError(t, err)
	cmd := exec.Command(shPath, scriptPath)
	cmd.Dir = root
	cmd.Env = []string{"PATH="}
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, out)
}

func installLauncherTestSlipway(t *testing.T, root string, script string) (string, string) {
	t.Helper()

	logPath := filepath.Join(root, "slipway-hook.log")
	binDir := filepath.Join(root, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	path := filepath.Join(binDir, "slipway")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return logPath, binDir
}

func writeRenderedHook(t *testing.T, root, templateName string, data map[string]string) string {
	t.Helper()

	content, err := Render(templateName, data)
	require.NoError(t, err)

	scriptPath := filepath.Join(root, filepath.Base(templateName))
	require.NoError(t, os.WriteFile(scriptPath, []byte(content), 0o755))
	return scriptPath
}

func readHookLog(t *testing.T, logPath string) string {
	t.Helper()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	return string(content)
}

func TestHookLauncherTemplatesRenderForNativePlatforms(t *testing.T) {
	for _, tc := range []struct {
		name     string
		template string
		want     string
	}{
		{name: "session powershell", template: "hooks/session-start.ps1.tmpl", want: `hook session-start --tool "claude"`},
		{name: "session cmd", template: "hooks/session-start.cmd.tmpl", want: `hook session-start --tool "claude"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, err := Render(tc.template, map[string]string{"ToolID": "claude", "LaunchPrefix": "slipway", "ProbeBin": "slipway"})
			require.NoError(t, err)
			assert.NotContains(t, content, "{{.")
			assert.Contains(t, content, tc.want)
			assert.NotContains(t, content, "next --json")
			assert.NotContains(t, content, "root")
		})
	}
}

func TestHookLauncherTemplateNamesStaySmall(t *testing.T) {
	for _, name := range []string{
		"hooks/session-start.sh.tmpl",
		"hooks/session-start.ps1.tmpl",
		"hooks/session-start.cmd.tmpl",
	} {
		content, err := Render(name, map[string]string{"ToolID": "claude", "LaunchPrefix": "slipway", "ProbeBin": "slipway"})
		require.NoError(t, err, fmt.Sprintf("render %s", name))
		assert.LessOrEqual(t, len([]byte(content)), 700, "%s must stay a thin launcher", name)
	}
}
