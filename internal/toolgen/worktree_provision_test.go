package toolgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeUnder writes data to root/rel, creating parent directories.
func writeUnder(t *testing.T, root, rel, data string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(data), 0o644))
}

// A real generated slipway host-skill dir, used to assert slipway-* surfaces are
// regenerated into the worktree.
const generatedSlipwaySkillDir = ".claude/skills/slipway-wave-orchestration"

func TestProvisionWorktreeHostSurfaces_CopiesThirdPartyAndRegenerates(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))

	// Third-party skills under two distinct adapters.
	writeUnder(t, repoRoot, ".claude/skills/golang-foo/SKILL.md", "third party claude")
	writeUnder(t, repoRoot, ".codex/skills/golang-foo/SKILL.md", "third party codex")
	// .serena is an MCP cache and must never be provisioned.
	writeUnder(t, repoRoot, ".serena/cache/index.bin", "serena")

	require.NoError(t, ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot))

	// Third-party content copied for every present adapter.
	assert.FileExists(t, filepath.Join(worktreeRoot, ".claude/skills/golang-foo/SKILL.md"))
	assert.FileExists(t, filepath.Join(worktreeRoot, ".codex/skills/golang-foo/SKILL.md"))
	// slipway-owned surfaces regenerated for each present adapter.
	assert.DirExists(t, filepath.Join(worktreeRoot, generatedSlipwaySkillDir))
	assert.FileExists(t, filepath.Join(worktreeRoot, ".claude/settings.json"))
	assert.FileExists(t, filepath.Join(worktreeRoot, ".claude/hooks/slipway-session-start.sh"))
	assert.DirExists(t, filepath.Join(worktreeRoot, ".codex/skills/slipway-wave-orchestration"))
	// .serena is not a toolgen adapter — it must not be provisioned.
	assert.NoDirExists(t, filepath.Join(worktreeRoot, ".serena"))
	// An adapter absent from the repo root must not be created in the worktree.
	assert.NoDirExists(t, filepath.Join(worktreeRoot, ".gemini"))
}

func TestProvisionWorktreeHostSurfaces_DoesNotMutateCodexHome(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))

	// A .codex adapter is present, so provisioning generates its worktree-local
	// skills — but Codex command prompts are host-global, and provisioning one
	// worktree must never rewrite them.
	writeUnder(t, repoRoot, ".codex/skills/golang-foo/SKILL.md", "third party codex")

	require.NoError(t, ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot))

	// Worktree-local Codex skills are generated.
	assert.DirExists(t, filepath.Join(worktreeRoot, ".codex/skills/slipway-wave-orchestration"))
	// Host-global Codex prompts under CODEX_HOME must be untouched.
	promptsDir := filepath.Join(codexHome, "prompts")
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		require.True(t, os.IsNotExist(err),
			"CODEX_HOME/prompts must not exist after provisioning, got: %v", err)
		return
	}
	assert.Empty(t, entries,
		"provisioning a worktree must not write any host-global Codex prompt")
}

func TestProvisionWorktreeHostSurfaces_DropsStaleManagedSlipwaySkill(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))

	// A managed-looking slipway-* skill that the current generator does NOT emit
	// (e.g. a skill removed in a newer Slipway). On a first-time provision the
	// worktree has no .adapter-generated sentinel, so the gated prune cannot run;
	// the copy step must therefore never carry it in.
	writeUnder(t, repoRoot, ".claude/skills/slipway-removed-legacy-skill/SKILL.md", "STALE MANAGED SKILL")
	writeUnder(t, repoRoot, ".claude/skills/golang-foo/SKILL.md", "third party")

	require.NoError(t, ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot))

	assert.NoDirExists(t, filepath.Join(worktreeRoot, ".claude/skills/slipway-removed-legacy-skill"),
		"a stale managed slipway-* skill must not be copied into a freshly provisioned worktree")
	// The third-party neighbour is still copied, and real generated skills exist.
	assert.FileExists(t, filepath.Join(worktreeRoot, ".claude/skills/golang-foo/SKILL.md"))
	assert.DirExists(t, filepath.Join(worktreeRoot, generatedSlipwaySkillDir))
}

func TestProvisionWorktreeHostSurfaces_ExcludesWorktreesLocksAndSourceSentinel(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))

	writeUnder(t, repoRoot, ".claude/skills/golang-foo/SKILL.md", "keep me")
	writeUnder(t, repoRoot, ".claude/worktrees/nested/inner.txt", "nested worktree content")
	writeUnder(t, repoRoot, ".claude/scheduled_tasks.lock", "lock state")
	writeUnder(t, repoRoot, ".claude/slipway/.adapter-generated", "SOURCE-SENTINEL")

	require.NoError(t, ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot))

	assert.FileExists(t, filepath.Join(worktreeRoot, ".claude/skills/golang-foo/SKILL.md"))
	// Excluded paths must not be carried into the worktree.
	assert.NoDirExists(t, filepath.Join(worktreeRoot, ".claude/worktrees"))
	assert.NoFileExists(t, filepath.Join(worktreeRoot, ".claude/scheduled_tasks.lock"))
	// Generate writes a fresh sentinel; the source's verbatim sentinel is not carried.
	got, err := os.ReadFile(filepath.Join(worktreeRoot, ".claude/slipway/.adapter-generated"))
	require.NoError(t, err)
	assert.NotEqual(t, "SOURCE-SENTINEL", string(got),
		"the .adapter-generated sentinel must be regenerated, not copied verbatim")
}

func TestProvisionWorktreeHostSurfaces_RegenerationOverwritesStaleSlipwaySkill(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))

	const staleMarker = "STALE-SLIPWAY-CONTENT-DO-NOT-PROPAGATE"
	staleRel := generatedSlipwaySkillDir + "/SKILL.md"
	writeUnder(t, repoRoot, staleRel, staleMarker)
	writeUnder(t, repoRoot, ".claude/skills/golang-foo/SKILL.md", "third party")

	require.NoError(t, ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot))

	got, err := os.ReadFile(filepath.Join(worktreeRoot, staleRel))
	require.NoError(t, err)
	assert.NotContains(t, string(got), staleMarker,
		"regeneration must overwrite the stale slipway skill rather than copy it verbatim")
	assert.NotEmpty(t, string(got))
	// The third-party neighbour is preserved verbatim.
	tp, err := os.ReadFile(filepath.Join(worktreeRoot, ".claude/skills/golang-foo/SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "third party", string(tp))
}

func TestProvisionWorktreeHostSurfaces_PreservesWorktreeLocalThirdPartyEdit(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))

	writeUnder(t, repoRoot, ".claude/skills/golang-foo/SKILL.md", "repo version")
	// The worktree already holds a locally-edited copy.
	writeUnder(t, worktreeRoot, ".claude/skills/golang-foo/SKILL.md", "WORKTREE LOCAL EDIT")

	require.NoError(t, ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot))

	got, err := os.ReadFile(filepath.Join(worktreeRoot, ".claude/skills/golang-foo/SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "WORKTREE LOCAL EDIT", string(got),
		"skip-if-exists must never clobber a worktree-local third-party edit")
}

func TestProvisionWorktreeHostSurfaces_FailClosedOnCopyError(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))
	writeUnder(t, repoRoot, ".claude/skills/golang-foo/SKILL.md", "x")
	// Destination .claude is a regular file, so creating the tool root dir fails.
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".claude"), []byte("not a dir"), 0o644))

	err := ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot)
	require.Error(t, err, "provisioning must fail closed when a copy step errors")
}

func TestProvisionWorktreeHostSurfaces_FailClosedOnRegenerationError(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "wt")
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o755))
	// Source carries only third-party content, so the copy step succeeds without
	// ever touching .claude/slipway.
	writeUnder(t, repoRoot, ".claude/skills/golang-foo/SKILL.md", "third party")
	// The worktree already holds .claude/slipway as a regular FILE. The copy step
	// still completes, but Generate cannot create the slipway surface directory,
	// so provisioning must fail closed on the regeneration step.
	require.NoError(t, os.MkdirAll(filepath.Join(worktreeRoot, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".claude/slipway"), []byte("not a dir"), 0o644))

	err := ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot)
	require.Error(t, err, "provisioning must fail closed when regeneration errors")
	assert.ErrorContains(t, err, "regenerate",
		"a regeneration failure must be reported distinctly from a copy failure")
	// The third-party copy completed before the regeneration step failed.
	assert.FileExists(t, filepath.Join(worktreeRoot, ".claude/skills/golang-foo/SKILL.md"))
}
