package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "--initial-branch=main", root)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git init: %s", out)
	return root
}

func TestInitWorkspaceRejectsNonGitDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := InitWorkspace(root, nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slipway requires a git repository")

	// .slipway.yaml must NOT have been created.
	_, statErr := os.Stat(filepath.Join(root, ".slipway.yaml"))
	assert.True(t, os.IsNotExist(statErr), ".slipway.yaml must not be created in non-git workspace")
}

func TestInitWorkspaceCreatesRuntimeMinimalLayout(t *testing.T) {
	t.Parallel()
	root := initGitRepo(t)
	require.NoError(t, InitWorkspace(root, nil, false))

	required := []string{
		".slipway.yaml",
	}
	for _, rel := range required {
		_, err := os.Stat(filepath.Join(root, rel))
		require.NoErrorf(t, err, "missing %s", rel)
	}

	_, err := os.Stat(filepath.Join(root, ".slipway"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(root, "artifacts"))
	assert.True(t, os.IsNotExist(err))
}

func TestInitWorkspaceRefreshWithoutToolsAutoDetects(t *testing.T) {
	root := initGitRepo(t)
	t.Setenv("CODEX_HOME", t.TempDir())

	// Initial generation with explicit tools.
	require.NoError(t, InitWorkspace(root, []string{"claude"}, false))
	sentinelPath := filepath.Join(root, ".claude", "slipway", ".adapter-generated")
	_, err := os.Stat(sentinelPath)
	require.NoError(t, err, "initial generation should create sentinel")

	// Tamper with a command entry to verify refresh overwrites it.
	commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
	require.NoError(t, os.WriteFile(commandPath, []byte("tampered"), 0o644))

	// Refresh without --tools: should auto-detect .claude/ and refresh.
	require.NoError(t, InitWorkspace(root, nil, true))
	content, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.NotEqual(t, "tampered", string(content), "refresh should have regenerated the file")
}

func TestInitWorkspaceRefreshWithoutToolsCleanWorkspace(t *testing.T) {
	t.Parallel()
	root := initGitRepo(t)

	// Refresh without --tools in a clean workspace: fail-closed (no sentinel).
	err := InitWorkspace(root, nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sentinelized adapters detected")
}

func TestInitWorkspaceWithToolsPreservesExistingAgentMappings(t *testing.T) {
	root := initGitRepo(t)
	t.Setenv("CODEX_HOME", t.TempDir())

	cfg := model.DefaultConfig()
	cfg.Agents.Mappings = map[string]string{
		"wave-orchestration": "slipway-executor",
	}
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

	require.NoError(t, InitWorkspace(root, []string{"claude", "codex"}, false))

	loaded, err := model.LoadConfig(state.ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, "slipway-executor", loaded.Agents.Mappings["wave-orchestration"])
	_, err = os.Stat(filepath.Join(root, ".claude", "slipway", ".adapter-generated"))
	require.NoError(t, err)

	require.NoError(t, InitWorkspace(root, []string{"claude", "codex"}, true))

	loaded, err = model.LoadConfig(state.ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, "slipway-executor", loaded.Agents.Mappings["wave-orchestration"])
	_, err = os.Stat(filepath.Join(root, ".codex", "agents", "slipway-executor.toml"))
	require.NoError(t, err)
}

func TestInitWorkspaceCreatesScopedRuntimeMarkerForNestedScope(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	nested := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	require.NoError(t, InitWorkspace(nested, nil, false))

	_, err := os.Stat(filepath.Join(nested, ".slipway.yaml"))
	require.NoError(t, err)
	_, err = os.Stat(state.GitRuntimeDir(nested))
	require.NoError(t, err)
}

func TestInitWorkspaceFromLinkedWorktreeSeedsCanonicalScope(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	cmd := exec.Command("git", "-C", root, "config", "user.email", "test@example.com")
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git config user.email: %s", out)
	cmd = exec.Command("git", "-C", root, "config", "user.name", "Test")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git config user.name: %s", out)
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644))
	cmd = exec.Command("git", "-C", root, "add", "README.md")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git add: %s", out)
	cmd = exec.Command("git", "-C", root, "commit", "-m", "init")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git commit: %s", out)

	worktreeRoot := filepath.Join(t.TempDir(), "linked-worktree")
	cmd = exec.Command("git", "-C", root, "worktree", "add", worktreeRoot, "-b", "feat/init-linked-worktree", "HEAD")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git worktree add: %s", out)

	require.NoError(t, InitWorkspace(worktreeRoot, nil, false))

	resolvedRoot, err := fsutil.FindProjectRoot(worktreeRoot)
	require.NoError(t, err)
	expectedRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	assert.Equal(t, expectedRoot, resolvedRoot)

	_, err = os.Stat(state.ConfigPath(expectedRoot))
	require.NoError(t, err, "canonical scope config should be created at main repo root")
	_, err = os.Stat(state.ConfigPath(worktreeRoot))
	require.NoError(t, err, "linked worktree should keep a local scope config mirror")
	_, err = os.Stat(state.WorkspaceScopeMarkerPath(worktreeRoot))
	require.NoError(t, err, "linked worktree should keep a local scope marker")
}

func TestInitWorkspaceFromLinkedWorktreeRefreshesLocalScopeConfigMirror(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	cmd := exec.Command("git", "-C", root, "config", "user.email", "test@example.com")
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git config user.email: %s", out)
	cmd = exec.Command("git", "-C", root, "config", "user.name", "Test")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git config user.name: %s", out)
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644))
	cmd = exec.Command("git", "-C", root, "add", "README.md")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git add: %s", out)
	cmd = exec.Command("git", "-C", root, "commit", "-m", "init")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git commit: %s", out)

	worktreeRoot := filepath.Join(t.TempDir(), "linked-worktree")
	cmd = exec.Command("git", "-C", root, "worktree", "add", worktreeRoot, "-b", "feat/init-linked-worktree-mirror", "HEAD")
	out, err = cmd.CombinedOutput()
	require.NoErrorf(t, err, "git worktree add: %s", out)

	require.NoError(t, InitWorkspace(worktreeRoot, nil, false))

	scopeCfg, err := model.LoadConfig(state.ConfigPath(root))
	require.NoError(t, err)
	scopeCfg.Agents.Mappings = map[string]string{
		"wave-orchestration": "slipway-executor",
	}
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), scopeCfg))

	worktreeCfg, err := model.LoadConfig(state.ConfigPath(worktreeRoot))
	require.NoError(t, err)
	worktreeCfg.Agents.Mappings = map[string]string{
		"wave-orchestration": "slipway-orchestrator",
	}
	require.NoError(t, model.SaveConfig(state.ConfigPath(worktreeRoot), worktreeCfg))

	require.NoError(t, InitWorkspace(worktreeRoot, nil, false))

	refreshed, err := model.LoadConfig(state.ConfigPath(worktreeRoot))
	require.NoError(t, err)
	assert.Equal(t, "slipway-executor", refreshed.Agents.Mappings["wave-orchestration"])
}

func TestInitWorkspaceToolsAllAndNone(t *testing.T) {
	t.Parallel()
	rootAll := initGitRepo(t)
	require.NoError(t, InitWorkspace(rootAll, []string{"claude", "cursor", "codex", "opencode"}, false))
	_, err := os.Stat(filepath.Join(rootAll, ".claude", "slipway", ".adapter-generated"))
	require.NoError(t, err)

	rootNone := initGitRepo(t)
	require.NoError(t, InitWorkspace(rootNone, nil, false))
	_, err = os.Stat(filepath.Join(rootNone, ".claude"))
	assert.True(t, os.IsNotExist(err))
}
