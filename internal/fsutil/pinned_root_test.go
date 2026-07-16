package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedRootRejectsRepositoryReplacementBeforeMutation(t *testing.T) {
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "repository")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenPinnedRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pinned.Close(); err != nil {
			t.Error(err)
		}
	}()

	originalPath := filepath.Join(parent, "repository-original")
	if err := os.Rename(rootPath, originalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}

	err = pinned.Apply([]FileTransactionOp{
		WriteFileTransactionOp(filepath.Join(rootPath, "managed.txt"), []byte("managed\n"), 0o600),
	})
	var identityErr *RootNamespaceIdentityError
	if !errors.As(err, &identityErr) {
		t.Fatalf("Apply() error = %v, want RootNamespaceIdentityError", err)
	}
	if identityErr.Committed {
		t.Fatalf("Apply() reported a commit before mutation")
	}
	for _, path := range []string{
		filepath.Join(rootPath, "managed.txt"),
		filepath.Join(originalPath, "managed.txt"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected mutation at %q: %v", path, err)
		}
	}
}
