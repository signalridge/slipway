package fsutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

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
