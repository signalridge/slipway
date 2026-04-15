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

// TestEmitSupportFilesForNonCatalogSkills verifies the post-PR-0 contract:
// any skill (not just catalog) that has a template-side provenance.yaml gets
// it copied into the destination tree.
func TestEmitSupportFilesForNonCatalogSkills(t *testing.T) {
	t.Parallel()

	srcFS := fstest.MapFS{
		"skills/sample-technique/SKILL.md":        &fstest.MapFile{Data: []byte("# sample\n")},
		"skills/sample-technique/provenance.yaml": &fstest.MapFile{Data: []byte("name: sample-technique\nupstream: example\n")},
	}

	dst := t.TempDir()
	require.NoError(t, emitSkillSupportFilesFromFS(srcFS, "sample-technique", dst, true))

	provBytes, err := os.ReadFile(filepath.Join(dst, "provenance.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(provBytes), "name: sample-technique")
}

// TestCatalogSkillsRetainProvenanceOnPresenceCheckMigration verifies that
// after switching from the explicit `includeProvenance` gate to a template-
// side presence check, every catalog skill that previously shipped
// provenance.yaml continues to do so.
func TestCatalogSkillsRetainProvenanceOnPresenceCheckMigration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))

	cfg := toolRegistry["codex"]
	for _, id := range catalogSkillsWithProvenanceSnapshot {
		provPath := filepath.Join(root, cfg.SkillsDir, "slipway", id, "provenance.yaml")
		_, err := os.Stat(provPath)
		assert.NoErrorf(t, err, "catalog skill %q lost provenance.yaml after migration", id)
	}
}

// catalogSkillsWithProvenanceSnapshot freezes the PR-0 migration contract to
// the historical set that shipped provenance at the time the template-side
// presence check landed. Future catalog additions can make an explicit support-
// file decision without silently broadening this regression test.
var catalogSkillsWithProvenanceSnapshot = []string{
	"ci-triage",
	"context-assembly",
	"coverage-analysis",
	"differential-review",
	"fresh-verification-evidence",
	"gha-security-review",
	"git-recovery",
	"incident-response",
	"independent-review",
	"multi-reviewer-calibration",
	"mutation-testing",
	"parallel-executor-contract",
	"performance-profiling",
	"plan-authoring",
	"property-testing",
	"review-comment-triage",
	"root-cause-tracing",
	"sast-orchestration",
	"scope-clarification",
	"security-review",
	"spec-trace",
	"supply-chain-audit",
	"tdd-proof",
	"threat-modeling",
	"variant-analysis",
}

// TestEmitSupportFilesWithoutProvenanceStillCopiesReferences verifies that
// the helper copies optional references/ even when no provenance.yaml exists
// template-side.
func TestEmitSupportFilesWithoutProvenanceStillCopiesReferences(t *testing.T) {
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
	_, err := os.Stat(filepath.Join(dst, "provenance.yaml"))
	assert.True(t, os.IsNotExist(err), "provenance.yaml should not be created when absent template-side")
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
// makes support payloads mirror the template tree instead of accumulating
// stale references/scripts/provenance from a previous render.
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
	assert.True(t, os.IsNotExist(err), "refresh should remove stale provenance when template-side provenance.yaml is absent")
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

// TestGeneratedSkillTreeInventoryManifest catches accidental structural drift
// in the generated `.codex/skills/slipway/` tree. The golden manifest tracks
// (path, file_kind, executable) per file. Semantic content drift stays with
// rendered-tree review and feature-specific fixture tests; this gate only
// catches missing files, unexpected extras, and executable-bit flips.
//
// Update the golden by running with -update.
func TestGeneratedSkillTreeInventoryManifest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))

	cfg := toolRegistry["codex"]
	skillsRoot := filepath.Join(root, cfg.SkillsDir, "slipway")
	manifest := buildSkillTreeInventory(t, skillsRoot)

	goldenPath := filepath.Join("testdata", "skill_tree_inventory.codex.golden")
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
	// Append a stable summary so deletions show up obviously in diffs.
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
	case strings.HasSuffix(rel, "provenance.yaml"):
		return "provenance"
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

// executableSentinel returns a platform-stable token for the file's exec bit.
// On POSIX, it asserts whether any +x bit is set. Windows normalizes to a
// fixed sentinel so the manifest stays portable.
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
