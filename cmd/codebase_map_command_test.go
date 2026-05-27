package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodebaseMapCommandCreatesDurableDocSet(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := makeCodebaseMapCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view codebaseMapView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "advisory", view.ExecutionMode)
		assert.Equal(t, "artifacts/codebase", view.CodebaseMapDir)
		assert.Equal(t, "populated", view.Status)
		require.Len(t, view.CodebaseMapDocs, 7)
		require.Empty(t, view.ScaffoldOnlyDocs)
		require.Len(t, view.PopulatedDocs, 7)
		require.Len(t, view.Created, 7)

		for _, path := range view.Created {
			_, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
			require.NoError(t, err)
		}
	})
}

func TestCodebaseMapCommandWritesToInvocationWorktree(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		worktreePath := filepath.Join(t.TempDir(), "codebase-map-worktree")
		runGit(t, root, "worktree", "add", worktreePath, "-b", "codebase-map-worktree")

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreePath))
		defer func() {
			require.NoError(t, os.Chdir(previousWD))
		}()

		var out bytes.Buffer
		cmd := makeCodebaseMapCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view codebaseMapView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "artifacts/codebase", view.CodebaseMapDir)
		require.FileExists(t, filepath.Join(worktreePath, "artifacts", "codebase", "STACK.md"))
		require.NoFileExists(t, filepath.Join(root, "artifacts", "codebase", "STACK.md"))
	})
}
