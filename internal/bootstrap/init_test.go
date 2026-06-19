package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(gitignore), state.LocalStateGitIgnoreBlock())
}

func TestInitWorkspacePreservesExistingGitIgnoreEntries(t *testing.T) {
	t.Parallel()
	root := initGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644))

	require.NoError(t, InitWorkspace(root, nil, false))
	require.NoError(t, InitWorkspace(root, nil, false))

	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(gitignore), "node_modules/\n")
	assert.Contains(t, string(gitignore), state.LocalStateGitIgnoreBlock())
	assert.Equal(t, 1, strings.Count(string(gitignore), "# Slipway local state (managed)"))
}

func TestInitWorkspaceRefreshWithoutToolsAutoDetects(t *testing.T) {
	root := initGitRepo(t)

	// Initial generation with explicit tools.
	require.NoError(t, InitWorkspace(root, []string{"claude"}, false))
	sentinelPath := filepath.Join(root, ".claude", "slipway", ".adapter-generated")
	_, err := os.Stat(sentinelPath)
	require.NoError(t, err, "initial generation should create sentinel")

	commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
	pristine, err := os.ReadFile(commandPath)
	require.NoError(t, err)

	// Refresh without --tools: should auto-detect .claude/ and refresh pristine content.
	require.NoError(t, InitWorkspace(root, nil, true))
	refreshed, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, string(pristine), string(refreshed), "refresh should regenerate pristine managed content")

	// Tampered marker-only files have no ownership-manifest proof, so refresh
	// must refuse to overwrite them.
	require.NoError(t, os.WriteFile(commandPath, []byte("tampered"), 0o644))

	err = InitWorkspace(root, nil, true)
	require.Error(t, err)
	content, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, "tampered", string(content), "refresh must preserve unknown modified content")
}

func TestInitWorkspaceRefreshWithoutToolsCleanWorkspace(t *testing.T) {
	t.Parallel()
	root := initGitRepo(t)

	// Refresh without --tools in a clean workspace: fail-closed (no sentinel).
	err := InitWorkspace(root, nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sentinelized adapters detected")
}

func TestInitWorkspaceExplicitRefreshWithoutSentinelPreservesUnknownSurface(t *testing.T) {
	root := initGitRepo(t)

	commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
	require.NoError(t, os.WriteFile(commandPath, []byte("user command"), 0o644))

	err := InitWorkspace(root, []string{"claude"}, true, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to overwrite unknown file")

	got, readErr := os.ReadFile(commandPath)
	require.NoError(t, readErr)
	assert.Equal(t, "user command", string(got))
}

func TestInitWorkspaceWithToolsPreservesExistingConfig(t *testing.T) {
	root := initGitRepo(t)

	cfg := model.DefaultConfig()
	cfg.Context.TechStack = "Go"
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

	require.NoError(t, InitWorkspace(root, []string{"claude", "codex"}, false))

	loaded, err := model.LoadConfig(state.ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, "Go", loaded.Context.TechStack)
	_, err = os.Stat(filepath.Join(root, ".claude", "slipway", ".adapter-generated"))
	require.NoError(t, err)

	require.NoError(t, InitWorkspace(root, []string{"claude", "codex"}, true))

	loaded, err = model.LoadConfig(state.ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, "Go", loaded.Context.TechStack)
	_, err = os.Stat(filepath.Join(root, ".codex", "skills", "slipway-wave-orchestration", "SKILL.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, ".codex", "agents"))
	assert.True(t, os.IsNotExist(err), "codex should not recreate exported agents during refresh")
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
	scopeCfg.Context.TechStack = "scope-go"
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), scopeCfg))

	worktreeCfg, err := model.LoadConfig(state.ConfigPath(worktreeRoot))
	require.NoError(t, err)
	worktreeCfg.Context.TechStack = "worktree-go"
	require.NoError(t, model.SaveConfig(state.ConfigPath(worktreeRoot), worktreeCfg))

	require.NoError(t, InitWorkspace(worktreeRoot, nil, false))

	refreshed, err := model.LoadConfig(state.ConfigPath(worktreeRoot))
	require.NoError(t, err)
	assert.Equal(t, "scope-go", refreshed.Context.TechStack)
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
