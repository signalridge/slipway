package fsutil

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunGitHonorsCanceledDiscoveryContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runGit(ctx, t.TempDir(), "rev-parse", "--show-toplevel")
	if err == nil {
		t.Fatal("runGit() unexpectedly succeeded with a canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runGit() error = %v, want context.Canceled", err)
	}
}

func TestDiscoverGitPreservesTrailingWhitespaceInRepositoryPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows normalizes trailing spaces in path components")
	}

	base := t.TempDir()
	repository := filepath.Join(base, "repo ")
	sibling := filepath.Join(base, "repo")
	for _, path := range []string{repository, sibling} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("git", "-C", path, "init", "--quiet")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git init %q: %v: %s", path, err, output)
		}
	}

	discovered, err := DiscoverGit(repository)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	if discovered.WorktreeRoot != resolved {
		t.Fatalf("worktree root = %q, want %q", discovered.WorktreeRoot, resolved)
	}
	if discovered.WorktreeRoot == sibling {
		t.Fatalf("trailing-space repository redirected to sibling %q", sibling)
	}
}

func TestDiscoverGitIgnoresRepositoryRedirectingEnvironment(t *testing.T) {
	base := t.TempDir()
	repository := filepath.Join(base, "repository")
	foreign := filepath.Join(base, "foreign")
	for _, path := range []string{repository, foreign} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("git", "-C", path, "init", "--quiet")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git init %q: %v: %s", path, err, output)
		}
	}

	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("GIT_COMMON_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(foreign, ".git", "index"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(foreign, ".git", "objects"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.bare")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")

	discovered, err := DiscoverGit(repository)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(resolved)
	wantGitDir, err := filepath.EvalSymlinks(filepath.Join(repository, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	wantGitDir = filepath.Clean(wantGitDir)
	if discovered.WorktreeRoot != want {
		t.Fatalf("worktree root = %q, want %q", discovered.WorktreeRoot, want)
	}
	if discovered.GitDir != wantGitDir {
		t.Fatalf("git directory = %q, want %q", discovered.GitDir, wantGitDir)
	}
	if discovered.CommonDir != wantGitDir {
		t.Fatalf("common directory = %q, want %q", discovered.CommonDir, wantGitDir)
	}
}

func TestTrimGitOutputTerminatorPreservesPathBytes(t *testing.T) {
	tests := map[string]string{
		"line feed":          "path ",
		"carriage line feed": "path ",
		"embedded line feed": "path\n",
		"no terminator":      "path ",
	}
	inputs := map[string]string{
		"line feed":          "path \n",
		"carriage line feed": "path \r\n",
		"embedded line feed": "path\n\n",
		"no terminator":      "path ",
	}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			if got := trimGitOutputTerminator(inputs[name]); got != want {
				t.Fatalf("trimGitOutputTerminator(%q) = %q, want %q", inputs[name], got, want)
			}
		})
	}
}
