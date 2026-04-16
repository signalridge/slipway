package toolgen

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Support-file export tests live in this focused file, rather than the
// already-large toolgen_test.go, so the PR-0 contract stays readable without
// mixing fixture-heavy support-file cases into generated-tree assembler tests.

// TestEmitSupportFilesCopiesReferences verifies the happy path: optional
// `references/` under the template tree is copied into the destination tree.
func TestEmitSupportFilesCopiesReferences(t *testing.T) {
	t.Parallel()

	srcFS := fstest.MapFS{
		"skills/refs-only/SKILL.md":              &fstest.MapFile{Data: []byte("# refs-only\n")},
		"skills/refs-only/references/topic-a.md": &fstest.MapFile{Data: []byte("# topic a\n")},
		"skills/refs-only/references/topic-b.md": &fstest.MapFile{Data: []byte("# topic b\n")},
	}

	dst := t.TempDir()
	require.NoError(t, emitSkillSupportFilesFromFS(srcFS, "refs-only", dst, true))

	for _, name := range []string{"topic-a.md", "topic-b.md"} {
		_, err := os.Stat(filepath.Join(dst, "references", name))
		assert.NoErrorf(t, err, "missing copied reference %q", name)
	}
}

// TestEmitSupportFilesSkipsEmpty verifies the no-op path: no support files
// to copy means no error, no empty directories.
func TestEmitSupportFilesSkipsEmpty(t *testing.T) {
	t.Parallel()

	srcFS := fstest.MapFS{
		"skills/bare/SKILL.md": &fstest.MapFile{Data: []byte("# bare\n")},
	}

	dst := t.TempDir()
	require.NoError(t, emitSkillSupportFilesFromFS(srcFS, "bare", dst, true))

	entries, err := os.ReadDir(dst)
	require.NoError(t, err)
	assert.Empty(t, entries, "expected no support files in destination, got %v", entries)
}

// TestEmitSupportFilesRefreshPrunesStaleArtifacts verifies that refresh mode
// makes support payloads mirror the template tree. Legacy provenance.yaml
// files from older generated trees must also be swept so the knowledge-only
// cleanup doesn't leave stale metadata behind.
func TestEmitSupportFilesRefreshPrunesStaleArtifacts(t *testing.T) {
	t.Parallel()

	srcFS := fstest.MapFS{
		"skills/sample/SKILL.md":              &fstest.MapFile{Data: []byte("# sample\n")},
		"skills/sample/references/current.md": &fstest.MapFile{Data: []byte("# current\n")},
		"skills/sample/scripts/current.sh":    &fstest.MapFile{Data: []byte("#!/usr/bin/env bash\nexit 0\n")},
	}

	dst := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dst, "references"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dst, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dst, "provenance.yaml"), []byte("stale: true\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dst, "references", "stale.md"), []byte("# stale\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dst, "scripts", "stale.sh"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755))

	require.NoError(t, emitSkillSupportFilesFromFS(srcFS, "sample", dst, true))

	_, err := os.Stat(filepath.Join(dst, "provenance.yaml"))
	assert.True(t, os.IsNotExist(err), "refresh should sweep legacy provenance.yaml files")
	_, err = os.Stat(filepath.Join(dst, "references", "stale.md"))
	assert.True(t, os.IsNotExist(err), "refresh should prune stale reference files")
	_, err = os.Stat(filepath.Join(dst, "scripts", "stale.sh"))
	assert.True(t, os.IsNotExist(err), "refresh should prune stale script files")

	_, err = os.Stat(filepath.Join(dst, "references", "current.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dst, "scripts", "current.sh"))
	assert.NoError(t, err)
}

// TestEmitSupportFilesSkipsPythonCacheArtifacts verifies that copied support
// trees ignore transient Python cache directories/files instead of exporting
// them into generated skill trees.
func TestEmitSupportFilesSkipsPythonCacheArtifacts(t *testing.T) {
	t.Parallel()

	srcFS := fstest.MapFS{
		"skills/python-helper/SKILL.md":                                    &fstest.MapFile{Data: []byte("# python-helper\n")},
		"skills/python-helper/scripts/merge-sarif.py":                      &fstest.MapFile{Data: []byte("#!/usr/bin/env python3\nprint('ok')\n")},
		"skills/python-helper/scripts/__pycache__/merge-sarif.cpython.pyc": &fstest.MapFile{Data: []byte("compiled")},
	}

	dst := t.TempDir()
	require.NoError(t, emitSkillSupportFilesFromFS(srcFS, "python-helper", dst, true))

	_, err := os.Stat(filepath.Join(dst, "scripts", "merge-sarif.py"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dst, "scripts", "__pycache__", "merge-sarif.cpython.pyc"))
	assert.True(t, os.IsNotExist(err), "python cache artifacts must not be copied into generated trees")
}

// TestTemplateShellScriptsAreExecutable guards the checked-in template source
// tree itself. Generated trees already normalize `*.sh` to 0755, but operators
// and local tests may invoke template-side helpers directly during development.
func TestTemplateShellScriptsAreExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable-bit semantics are POSIX-only")
	}

	pattern := filepath.Join(checkedInSkillTemplatesRoot(t), "*", "scripts", "*.sh")
	matches, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, matches, "expected at least one template-side shell helper")

	for _, path := range matches {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.NotZerof(t, info.Mode().Perm()&0o111,
			"%s: expected executable bit on checked-in template shell script, got %v",
			path, info.Mode().Perm())
	}
}

// TestGeneratedSkillTreeInventoryManifest catches accidental structural drift
// in the generated `.codex/skills/slipway/` tree. The golden manifest tracks
// (path, file_kind, executable) per file. Semantic content drift stays with
// rendered-tree review and feature-specific fixture tests; this gate only
// catches missing files, unexpected extras, and executable-bit flips.
//
// Update the golden by running with UPDATE_GOLDEN=1.
func TestGeneratedSkillTreeInventoryManifest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))

	cfg := toolRegistry["codex"]
	skillsRoot := filepath.Join(root, cfg.SkillsDir, "slipway")
	manifest := buildSkillTreeInventory(t, skillsRoot)

	goldenPath := toolgenTestdataPath(t, "skill_tree_inventory.codex.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(manifest), 0o644))
		t.Skip("golden updated; rerun without UPDATE_GOLDEN=1 to assert")
	}

	want, err := os.ReadFile(goldenPath)
	require.NoErrorf(t, err, "missing golden manifest; regenerate with UPDATE_GOLDEN=1")
	if string(want) != manifest {
		t.Errorf("skill tree inventory drift; rerun with UPDATE_GOLDEN=1 to refresh after intentional changes.\n--- diff sample ---\n%s",
			firstNDiffLines(string(want), manifest, 20))
	}
}

func buildSkillTreeInventory(t *testing.T, root string) string {
	t.Helper()

	type entry struct {
		path string
		kind string
		exec string
	}
	var entries []entry

	rootInfo, err := os.Stat(root)
	require.NoError(t, err)
	require.True(t, rootInfo.IsDir(), "expected skills root %q to be a directory", root)

	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		kind := classifyByPath(rel)
		exec := executableSentinel(p)
		entries = append(entries, entry{path: rel, kind: kind, exec: exec})
		return nil
	}))

	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.path)
		b.WriteByte('\t')
		b.WriteString(e.kind)
		b.WriteByte('\t')
		b.WriteString(e.exec)
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	b.WriteString("# inventory_sha256:")
	b.WriteString(hex.EncodeToString(sum[:]))
	b.WriteByte('\n')
	return b.String()
}

func classifyByPath(rel string) string {
	switch {
	case strings.HasSuffix(rel, "SKILL.md"):
		return "skill_md"
	case strings.Contains(rel, "/references/"):
		return "reference"
	case strings.Contains(rel, "/scripts/") && strings.HasSuffix(rel, ".sh"):
		return "script_sh"
	case strings.Contains(rel, "/scripts/") && strings.HasSuffix(rel, ".py"):
		return "script_py"
	case strings.Contains(rel, "/scripts/"):
		return "script_other"
	case strings.HasSuffix(rel, ".md"):
		return "manifest_md"
	default:
		return "other"
	}
}

func executableSentinel(p string) string {
	if runtime.GOOS == "windows" {
		return "platform-windows"
	}
	st, err := os.Stat(p)
	if err != nil {
		return "stat-error"
	}
	if st.Mode().Perm()&0o111 != 0 {
		return "exec"
	}
	return "non-exec"
}

func firstNDiffLines(want, got string, n int) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	var b strings.Builder
	count := 0
	max := len(wl)
	if len(gl) > max {
		max = len(gl)
	}
	for i := 0; i < max && count < n; i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			b.WriteString("- ")
			b.WriteString(w)
			b.WriteByte('\n')
			b.WriteString("+ ")
			b.WriteString(g)
			b.WriteByte('\n')
			count++
		}
	}
	return b.String()
}

func toolgenPackageDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Dir(filename)
}

func toolgenRepoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(toolgenPackageDir(t), "..", ".."))
}

func checkedInSkillTemplatesRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(toolgenRepoRoot(t), "internal", "tmpl", "templates", "skills")
}

func toolgenTestdataPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(toolgenPackageDir(t), "testdata", name)
}
