package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// RootNamespaceIdentityError reports that a transaction root pathname no longer
// names the directory handle pinned before planning.
type RootNamespaceIdentityError struct {
	Root      string
	Committed bool
}

func (err *RootNamespaceIdentityError) Error() string {
	if err.Committed {
		return fmt.Sprintf("transaction root %q detached after mutation; outcome requires inspection", err.Root)
	}
	return fmt.Sprintf("transaction root %q changed after planning; no mutation was attempted", err.Root)
}

// PinnedRoot binds planning and mutation to one opened repository root.
type PinnedRoot struct {
	path     string
	root     *os.Root
	identity os.FileInfo
}

// OpenPinnedRoot opens rootPath once and records the directory identity that its
// namespace entry must continue to reference.
func OpenPinnedRoot(rootPath string) (*PinnedRoot, error) {
	absolute, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("absolute transaction root: %w", err)
	}
	absolute = filepath.Clean(absolute)
	root, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, fmt.Errorf("open transaction root: %w", err)
	}
	identity, err := rootDirectoryInfo(root)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("inspect opened transaction root: %w", err)
	}
	current, err := os.Stat(absolute)
	if err != nil || !current.IsDir() || !os.SameFile(identity, current) {
		_ = root.Close()
		if err != nil {
			return nil, fmt.Errorf("inspect transaction root namespace: %w", err)
		}
		return nil, fmt.Errorf("transaction root namespace does not match opened directory")
	}
	return &PinnedRoot{path: absolute, root: root, identity: identity}, nil
}

// Close releases the pinned root handle.
func (root *PinnedRoot) Close() error {
	if root == nil || root.root == nil {
		return nil
	}
	err := root.root.Close()
	root.root = nil
	return err
}

// ValidateNamespace checks that root.path still names the opened directory.
func (root *PinnedRoot) ValidateNamespace() error {
	if root == nil || root.root == nil {
		return fmt.Errorf("pinned transaction root is closed")
	}
	opened, err := rootDirectoryInfo(root.root)
	if err != nil {
		return err
	}
	current, err := os.Stat(root.path)
	if err != nil {
		return err
	}
	if !current.IsDir() || !os.SameFile(root.identity, opened) || !os.SameFile(opened, current) {
		return fmt.Errorf("root namespace no longer names the pinned directory")
	}
	return nil
}

// Lstat inspects path through the repository handle pinned before planning.
// Parent components must be real directories, not symbolic links.
func (root *PinnedRoot) Lstat(path string) (os.FileInfo, error) {
	if root == nil || root.root == nil {
		return nil, fmt.Errorf("pinned transaction root is closed")
	}
	if err := root.ValidateNamespace(); err != nil {
		return nil, &RootNamespaceIdentityError{Root: root.path}
	}
	filesystem := &transactionFilesystem{rootPath: root.path, root: root.root}
	parent, name, err := filesystem.openStableParent(path, false)
	if err != nil {
		if namespaceErr := root.ValidateNamespace(); namespaceErr != nil {
			return nil, &RootNamespaceIdentityError{Root: root.path}
		}
		return nil, err
	}
	info, statErr := parent.Lstat(name)
	closeErr := parent.Close()
	if namespaceErr := root.ValidateNamespace(); namespaceErr != nil {
		return nil, &RootNamespaceIdentityError{Root: root.path}
	}
	return info, errors.Join(statErr, closeErr)
}

// ReadFileNoSymlink reads a bounded regular file through the repository handle
// pinned before planning and verifies that the root namespace remains attached.
func (root *PinnedRoot) ReadFileNoSymlink(path string, maxBytes int64) ([]byte, error) {
	if root == nil || root.root == nil {
		return nil, fmt.Errorf("pinned transaction root is closed")
	}
	if err := root.ValidateNamespace(); err != nil {
		return nil, &RootNamespaceIdentityError{Root: root.path}
	}
	filesystem := &transactionFilesystem{rootPath: root.path, root: root.root}
	parent, name, err := filesystem.openStableParent(path, false)
	if err != nil {
		if namespaceErr := root.ValidateNamespace(); namespaceErr != nil {
			return nil, &RootNamespaceIdentityError{Root: root.path}
		}
		return nil, err
	}
	data, readErr := readFileNoSymlinkInRoot(parent, name, path, maxBytes)
	closeErr := parent.Close()
	if namespaceErr := root.ValidateNamespace(); namespaceErr != nil {
		return nil, &RootNamespaceIdentityError{Root: root.path}
	}
	return data, errors.Join(readErr, closeErr)
}

// Apply validates the namespace, applies ops through the same os.Root used to
// pin the repository, and checks that the namespace stayed attached afterward.
func (root *PinnedRoot) Apply(ops []FileTransactionOp) error {
	if root == nil || root.root == nil {
		return fmt.Errorf("pinned transaction root is closed")
	}
	if err := validateFileTransactionOps(ops); err != nil {
		return err
	}
	normalized := slices.Clone(ops)
	for index := range normalized {
		absolute, err := filepath.Abs(normalized[index].path)
		if err != nil {
			return fmt.Errorf("absolute transaction path %s: %w", normalized[index].path, err)
		}
		normalized[index].path = filepath.Clean(absolute)
		if !PathWithin(root.path, normalized[index].path) {
			return fmt.Errorf("transaction path %q escapes root %q", normalized[index].path, root.path)
		}
	}
	if err := root.ValidateNamespace(); err != nil {
		return &RootNamespaceIdentityError{Root: root.path}
	}
	filesystem := &transactionFilesystem{rootPath: root.path, root: root.root}
	if err := applyFileTransactionWithFilesystem(normalized, filesystem); err != nil {
		return err
	}
	if err := root.ValidateNamespace(); err != nil {
		return &RootNamespaceIdentityError{Root: root.path, Committed: len(normalized) > 0}
	}
	return nil
}
