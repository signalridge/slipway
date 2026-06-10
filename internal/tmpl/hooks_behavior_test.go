package tmpl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStartHookFindsNearestParentScope(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, initHookGitRepo(root))
	writeHookProjectConfig(t, root)
	writeHookSharedScopeMarker(t, root, "")
	nested := filepath.Join(root, "pkg", "feature")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	logPath, binDir := installHookTestSlipway(t, root)
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = nested
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), "<slipway-session-start tool=\"claude\">")
	assert.Contains(t, string(out), "session_handoff_present: false")
	logContent := readHookLog(t, logPath)
	assert.Contains(t, logContent, hookInitialPath(t, nested)+"|root")
	assert.Contains(t, logContent, hookObservedPath(t, nested)+"|next --json")
	assert.NotContains(t, logContent, "status --json")
	assert.NotContains(t, logContent, "--hook-lite")
	assert.NotContains(t, logContent, "--preview")
}

func TestSessionStartHookReadsScopeScopedHandoff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, initHookGitRepo(root))
	writeHookProjectConfig(t, root)
	writeHookSharedScopeMarker(t, root, "")

	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	writeHookProjectConfig(t, scopeRoot)
	writeHookSharedScopeMarker(t, root, filepath.Join("services", "billing"))
	nested := filepath.Join(scopeRoot, "pkg", "feature")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	globalHandoff := filepath.Join(root, ".git", "slipway", "runtime", "handoff.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(globalHandoff), 0o755))
	require.NoError(t, os.WriteFile(globalHandoff, []byte("GLOBAL HANDOFF"), 0o644))

	scopedHandoff := filepath.Join(root, ".git", "slipway", "scopes", "services", "billing", "runtime", "handoff.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(scopedHandoff), 0o755))
	require.NoError(t, os.WriteFile(scopedHandoff, []byte("SCOPED HANDOFF"), 0o644))
	canonicalScopedHandoff, err := filepath.EvalSymlinks(scopedHandoff)
	require.NoError(t, err)

	logPath, binDir := installHookTestSlipway(t, scopeRoot)
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = nested
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), "session_handoff_path: "+hookObservedPath(t, canonicalScopedHandoff))
	assert.Contains(t, string(out), "session_handoff_present: true")
	assert.NotContains(t, string(out), "SCOPED HANDOFF")
	assert.NotContains(t, string(out), "GLOBAL HANDOFF")
}

func TestSessionStartHookUsesCanonicalScopeRootForMarkerOnlyBoundWorktree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	root = resolvedRoot
	require.NoError(t, initHookGitRepo(root))
	writeHookProjectConfig(t, root)
	writeHookSharedScopeMarker(t, root, "")

	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	writeHookProjectConfig(t, scopeRoot)
	writeHookSharedScopeMarker(t, root, filepath.Join("services", "billing"))
	commitHookRepoState(t, root)

	worktreeRoot := filepath.Join(t.TempDir(), "bound-worktree")
	addHookGitWorktree(t, root, worktreeRoot, "feat/hook-bound")

	worktreeScopeRoot := filepath.Join(worktreeRoot, "services", "billing")
	require.NoError(t, os.Remove(filepath.Join(worktreeScopeRoot, ".slipway.yaml")))
	nested := filepath.Join(worktreeScopeRoot, "pkg", "feature")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	logPath, binDir := installHookTestSlipway(t, scopeRoot)
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = nested
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), "<slipway-session-start tool=\"claude\">")
	logContent := readHookLog(t, logPath)
	assert.Contains(t, logContent, hookObservedPath(t, nested)+"|next --json")
	assert.NotContains(t, logContent, hookObservedPath(t, scopeRoot)+"|next --json")
	assert.NotContains(t, logContent, hookObservedPath(t, root)+"|next --json")
}

func TestSessionStartHookIgnoresStaleNestedScopeMarkerWithoutConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, initHookGitRepo(root))
	writeHookProjectConfig(t, root)
	writeHookSharedScopeMarker(t, root, "")

	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	writeHookSharedScopeMarker(t, root, filepath.Join("services", "billing"))
	nested := filepath.Join(scopeRoot, "pkg", "feature")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	logPath, binDir := installHookTestSlipway(t, root)
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = nested
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), "<slipway-session-start tool=\"claude\">")
	logContent := readHookLog(t, logPath)
	assert.Contains(t, logContent, hookObservedPath(t, nested)+"|next --json")
	assert.NotContains(t, logContent, hookObservedPath(t, root)+"|next --json")
	assert.NotContains(t, logContent, hookObservedPath(t, scopeRoot)+"|next --json")
}

func TestSessionStartHookSurfacesNextFailureDiagnostic(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, initHookGitRepo(root))
	writeHookProjectConfig(t, root)
	writeHookSharedScopeMarker(t, root, "")

	logPath, binDir := installHookTestSlipwayScript(t, root, fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
printf '%%s|%%s\n' "$PWD" "$*" >> "${SLIPWAY_HOOK_LOG}"
	case "$*" in
	  "root")
	    printf '%%s\n' %q
	    ;;
	  "next --json")
    echo 'next contract broke' >&2
    exit 1
    ;;
	esac
	`, root))
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), "hook_diagnostic: slipway next --json failed: next contract broke")
}

func TestSessionStartHookSetsToolEnvForReadOnlyCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, initHookGitRepo(root))
	writeHookProjectConfig(t, root)
	writeHookSharedScopeMarker(t, root, "")

	logPath, binDir := installHookTestSlipwayScript(t, root, fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
printf '%%s|%%s\n' "$PWD" "$*" >> "${SLIPWAY_HOOK_LOG}"
case "$*" in
  "root")
    printf '%%s\n' %q
    ;;
  "next --json")
    printf '{"next_skill":{"name":"plan-audit"}}'
    ;;
esac
`, root))
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.LessOrEqual(t, len(out), 768, "session-start payload must stay compact")
	assert.NotContains(t, string(out), "hook_diagnostic:")
	assert.Contains(t, string(out), `{"next_skill":{"name":"plan-audit"}}`)

	logContent := readHookLog(t, logPath)
	assert.Contains(t, logContent, hookObservedPath(t, root)+"|next --json")
	assert.NotContains(t, logContent, "status --json")
	assert.NotContains(t, logContent, "--hook-lite")
}

func TestSessionStartHookSurfacesRootFailureDiagnostic(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	logPath, binDir := installHookTestSlipwayScript(t, root, `#!/usr/bin/env bash
set -euo pipefail
printf '%s|%s\n' "$PWD" "$*" >> "${SLIPWAY_HOOK_LOG}"
case "$*" in
  "root")
    echo 'root resolution failed' >&2
    exit 1
    ;;
esac
`)
	scriptPath := writeRenderedHook(t, root, "hooks/session-start.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), "hook_diagnostic: slipway root failed: root resolution failed")
}

func TestContextPressurePostToolUseHookDelegatesToSlipwayHookCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	logPath, binDir := installHookTestSlipwayScript(t, root, `#!/usr/bin/env bash
set -euo pipefail
printf '%s|%s\n' "$PWD" "$*" >> "${SLIPWAY_HOOK_LOG}"
case "$*" in
  "hook context-pressure")
    cat >/dev/null
    printf '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"CONTEXT CRITICAL: run slipway checkpoint"}}'
    ;;
esac
`)
	scriptPath := writeRenderedHook(t, root, "hooks/context-pressure-post-tool-use.sh.tmpl", map[string]string{
		"ToolID": "claude",
	})

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"PostToolUse"}`)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SLIPWAY_HOOK_LOG="+logPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Contains(t, string(out), "hookSpecificOutput")
	assert.Contains(t, string(out), "CONTEXT CRITICAL")
	logContent := readHookLog(t, logPath)
	assert.Contains(t, logContent, hookObservedPath(t, root)+"|hook context-pressure")
}

func installHookTestSlipway(t *testing.T, canonicalRoot string) (string, string) {
	t.Helper()

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
printf '%%s|%%s\n' "$PWD" "$*" >> "${SLIPWAY_HOOK_LOG}"
	case "$*" in
	  "root")
	    printf '%%s\n' %q
	    ;;
		  "next --json")
		    printf '{"next_skill":null}'
		    ;;
	esac
	`, canonicalRoot)
	return installHookTestSlipwayScript(t, canonicalRoot, script)
}

func installHookTestSlipwayScript(t *testing.T, canonicalRoot string, script string) (string, string) {
	t.Helper()

	logPath := filepath.Join(canonicalRoot, "slipway-hook.log")
	binDir := filepath.Join(canonicalRoot, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	path := filepath.Join(binDir, "slipway")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return logPath, binDir
}

func initHookGitRepo(root string) error {
	cmd := exec.Command("git", "init", "--initial-branch=main", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	for _, args := range [][]string{
		{"-C", root, "config", "user.email", "test@example.com"},
		{"-C", root, "config", "user.name", "Test User"},
	} {
		cmd = exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func writeHookSharedScopeMarker(t *testing.T, root, scopeRel string) {
	t.Helper()

	base := filepath.Join(root, ".git", "slipway")
	if scopeRel != "" {
		base = filepath.Join(base, "scopes", scopeRel)
	}
	require.NoError(t, os.MkdirAll(base, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "scope-root"), []byte("scope\n"), 0o644))
}

func writeHookProjectConfig(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))
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

func hookObservedPath(t *testing.T, p string) string {
	t.Helper()

	if runtime.GOOS != "windows" {
		return p
	}

	info, err := os.Stat(p)
	require.NoError(t, err)

	dir := p
	base := ""
	if !info.IsDir() {
		dir = filepath.Dir(p)
		base = filepath.Base(p)
	}

	cmd := exec.Command("bash", "-lc", "pwd -P")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)

	observed := strings.TrimSpace(string(out))
	if base != "" {
		observed += "/" + base
	}
	return observed
}

func hookInitialPath(t *testing.T, p string) string {
	t.Helper()

	if runtime.GOOS != "windows" {
		return p
	}

	cmd := exec.Command("bash", "-lc", "printf '%s' \"$PWD\"")
	cmd.Dir = p
	out, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func commitHookRepoState(t *testing.T, root string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644))
	runHookGit(t, root, "add", ".")
	runHookGit(t, root, "commit", "-m", "init")
}

func addHookGitWorktree(t *testing.T, root, worktreePath, branch string) {
	t.Helper()

	runHookGit(t, root, "worktree", "add", worktreePath, "-b", branch)
}

func runHookGit(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
}
