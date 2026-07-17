package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		// Windows may deny renaming an opened directory. That is a stronger
		// namespace pin than the replacement scenario below, and it must be
		// asserted rather than skipped so native CI still exercises the policy.
		if validateErr := pinned.ValidateNamespace(); validateErr != nil {
			t.Fatalf("ValidateNamespace() after denied replacement = %v", validateErr)
		}
		if _, statErr := os.Stat(originalPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("denied replacement unexpectedly created %q: %v", originalPath, statErr)
		}
		return
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

func TestPinnedRootPreflightFailureDoesNotClaimMutationAfterConcurrentRootReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows pins an opened directory strongly enough to deny this replacement fixture")
	}
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "repository")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(rootPath, "first.txt")
	later := filepath.Join(rootPath, "later.txt")
	if err := os.WriteFile(first, []byte("first-before"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(later, []byte("later-before"), 0o600); err != nil {
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
	replaced := false
	err = pinned.apply([]FileTransactionOp{
		WriteFileTransactionOp(first, []byte("first-after"), 0o600),
		WriteFileTransactionOp(later, []byte("later-after"), 0o600).
			WithExpectedSHA256(testSHA256([]byte("different-content"))),
	}, fileTransactionHooks{AfterPreflight: func(_, _ string) error {
		if replaced {
			return nil
		}
		replaced = true
		if renameErr := os.Rename(rootPath, originalPath); renameErr != nil {
			return renameErr
		}
		return os.Mkdir(rootPath, 0o700)
	}})
	if !replaced {
		t.Fatal("preflight replacement hook did not run")
	}
	if !errors.Is(err, ErrFileTransactionPrecondition) {
		t.Fatalf("Apply() error = %v, want ErrFileTransactionPrecondition", err)
	}
	var identityErr *RootNamespaceIdentityError
	if !errors.As(err, &identityErr) {
		t.Fatalf("Apply() error = %v, want RootNamespaceIdentityError", err)
	}
	if identityErr.Committed {
		t.Fatalf("Apply() reported a commit even though preflight rejected every mutation")
	}
	for _, path := range []string{
		filepath.Join(originalPath, "first.txt"),
		filepath.Join(originalPath, "later.txt"),
	} {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(content) != map[string]string{
			filepath.Join(originalPath, "first.txt"): "first-before",
			filepath.Join(originalPath, "later.txt"): "later-before",
		}[path] {
			t.Fatalf("preflight mutated %q: %q", path, content)
		}
	}
}

func TestPinnedRootGuardFailureBeforeMutationDoesNotClaimCommitAfterRootReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows pins an opened directory strongly enough to deny this replacement fixture")
	}
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "repository")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(rootPath, "managed.txt")
	if err := os.WriteFile(managed, []byte("before"), 0o600); err != nil {
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
	injected := errors.New("guard failed before quarantine")
	replaced := false
	err = pinned.apply([]FileTransactionOp{
		WriteFileTransactionOp(managed, []byte("after"), 0o600),
	}, fileTransactionHooks{AfterGuardBeforeQuarantine: func(_, _ string) error {
		if replaced {
			return injected
		}
		replaced = true
		if renameErr := os.Rename(rootPath, originalPath); renameErr != nil {
			return renameErr
		}
		if mkdirErr := os.Mkdir(rootPath, 0o700); mkdirErr != nil {
			return mkdirErr
		}
		return injected
	}})
	if !replaced {
		t.Fatal("guard replacement hook did not run")
	}
	if !errors.Is(err, injected) {
		t.Fatalf("Apply() error = %v, want injected guard failure", err)
	}
	var identityErr *RootNamespaceIdentityError
	if !errors.As(err, &identityErr) {
		t.Fatalf("Apply() error = %v, want RootNamespaceIdentityError", err)
	}
	if identityErr.Committed {
		t.Fatalf("Apply() reported a commit before the first filesystem mutation")
	}
	content, readErr := os.ReadFile(filepath.Join(originalPath, "managed.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "before" {
		t.Fatalf("guard failure mutated original content: %q", content)
	}
	if _, statErr := os.Stat(filepath.Join(rootPath, "managed.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("guard failure mutated replacement root: %v", statErr)
	}
}

func TestPinnedRootReportsCommitAfterMutationWhenRootIsReplaced(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows pins an opened directory strongly enough to deny this replacement fixture")
	}
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "repository")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(rootPath, "managed.txt")
	if err := os.WriteFile(managed, []byte("before"), 0o600); err != nil {
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
	replaced := false
	err = pinned.apply([]FileTransactionOp{
		WriteFileTransactionOp(managed, []byte("after"), 0o600),
	}, fileTransactionHooks{AfterMutation: func(_, _ string) error {
		if replaced {
			return nil
		}
		replaced = true
		if renameErr := os.Rename(rootPath, originalPath); renameErr != nil {
			return renameErr
		}
		return os.Mkdir(rootPath, 0o700)
	}})
	if !replaced {
		t.Fatal("post-mutation replacement hook did not run")
	}
	var identityErr *RootNamespaceIdentityError
	if !errors.As(err, &identityErr) {
		t.Fatalf("Apply() error = %v, want RootNamespaceIdentityError", err)
	}
	if !identityErr.Committed {
		t.Fatal("Apply() did not report the completed filesystem mutation")
	}
	content, readErr := os.ReadFile(filepath.Join(originalPath, "managed.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "after" {
		t.Fatalf("committed content = %q, want after", content)
	}
	if _, statErr := os.Stat(filepath.Join(rootPath, "managed.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("transaction mutated replacement root: %v", statErr)
	}
}
