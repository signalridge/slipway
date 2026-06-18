package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type fileTransactionOpKind string

const (
	fileTransactionOpWrite     fileTransactionOpKind = "write"
	fileTransactionOpRemove    fileTransactionOpKind = "remove"
	fileTransactionOpRemoveAll fileTransactionOpKind = "remove_all"
)

// FileTransactionOp describes one ordered file mutation in a file transaction.
type FileTransactionOp struct {
	kind fileTransactionOpKind
	path string
	data []byte
	perm os.FileMode
}

// WriteFileTransactionOp returns a transaction operation that atomically writes
// one file when the transaction is applied.
func WriteFileTransactionOp(path string, data []byte, perm os.FileMode) FileTransactionOp {
	return FileTransactionOp{
		kind: fileTransactionOpWrite,
		path: path,
		data: slices.Clone(data),
		perm: perm,
	}
}

// RemoveFileTransactionOp returns a transaction operation that removes one file
// when it exists.
func RemoveFileTransactionOp(path string) FileTransactionOp {
	return FileTransactionOp{
		kind: fileTransactionOpRemove,
		path: path,
	}
}

// RemoveAllTransactionOp returns a transaction operation that removes a file or
// directory tree when it exists.
func RemoveAllTransactionOp(path string) FileTransactionOp {
	return FileTransactionOp{
		kind: fileTransactionOpRemoveAll,
		path: path,
	}
}

// ApplyFileTransaction applies ordered file writes/removes. If an operation
// fails after earlier operations succeeded, earlier files are restored or
// removed so callers do not observe a partial file set.
func ApplyFileTransaction(ops []FileTransactionOp) error {
	return applyFileTransaction(ops, fileTransactionHooks{})
}

// FileTransactionHooks overrides file IO for deterministic tests.
type FileTransactionHooks struct {
	WriteFile  func(path string, data []byte, perm os.FileMode) error
	RemoveFile func(path string) error
	RemoveAll  func(path string) error
}

// ApplyFileTransactionWithHooks applies a file transaction with caller-supplied
// file IO hooks. Production callers should normally use ApplyFileTransaction.
func ApplyFileTransactionWithHooks(ops []FileTransactionOp, hooks FileTransactionHooks) error {
	return applyFileTransaction(ops, fileTransactionHooks{
		writeFile:  hooks.WriteFile,
		removeFile: hooks.RemoveFile,
		removeAll:  hooks.RemoveAll,
	})
}

type fileTransactionHooks struct {
	writeFile  func(path string, data []byte, perm os.FileMode) error
	removeFile func(path string) error
	removeAll  func(path string) error
}

func (hooks fileTransactionHooks) withDefaults() fileTransactionHooks {
	if hooks.writeFile == nil {
		hooks.writeFile = WriteFileAtomic
	}
	if hooks.removeFile == nil {
		hooks.removeFile = removeTransactionFileIfExists
	}
	if hooks.removeAll == nil {
		hooks.removeAll = removeTransactionPathIfExists
	}
	return hooks
}

type fileSnapshot struct {
	existed bool
	isDir   bool
	data    []byte
	perm    os.FileMode
	entries []fileTreeSnapshotEntry
}

type fileTreeSnapshotEntry struct {
	rel   string
	isDir bool
	data  []byte
	perm  os.FileMode
}

type appliedFileTransactionOp struct {
	kind   fileTransactionOpKind
	path   string
	before fileSnapshot
}

type rollbackPathError struct {
	path string
	err  error
}

func (err rollbackPathError) Error() string {
	return fmt.Sprintf("%s: %v", err.path, err.err)
}

func (err rollbackPathError) Unwrap() error {
	return err.err
}

// FileTransactionError reports the operation failure and any rollback failures.
type FileTransactionError struct {
	OperationErr error
	RollbackErrs []error
}

func (err *FileTransactionError) Error() string {
	if err == nil {
		return ""
	}
	if len(err.RollbackErrs) == 0 {
		return fmt.Sprintf("apply file transaction: %v", err.OperationErr)
	}
	paths := make([]string, 0, len(err.RollbackErrs))
	for _, rollbackErr := range err.RollbackErrs {
		var pathErr rollbackPathError
		if errors.As(rollbackErr, &pathErr) {
			paths = append(paths, pathErr.path)
		}
	}
	slices.Sort(paths)
	return fmt.Sprintf(
		"apply file transaction: %v; rollback incomplete for %s: %v",
		err.OperationErr,
		strings.Join(paths, ", "),
		errors.Join(err.RollbackErrs...),
	)
}

func (err *FileTransactionError) Unwrap() error {
	if err == nil {
		return nil
	}
	joined := make([]error, 0, 1+len(err.RollbackErrs))
	if err.OperationErr != nil {
		joined = append(joined, err.OperationErr)
	}
	joined = append(joined, err.RollbackErrs...)
	return errors.Join(joined...)
}

func applyFileTransaction(ops []FileTransactionOp, hooks fileTransactionHooks) error {
	hooks = hooks.withDefaults()
	if err := validateFileTransactionOps(ops); err != nil {
		return err
	}

	applied := make([]appliedFileTransactionOp, 0, len(ops))
	for _, op := range ops {
		before, err := snapshotPathForTransaction(op)
		if err != nil {
			if len(applied) > 0 {
				return newFileTransactionError(err, rollbackFileTransaction(applied, hooks))
			}
			return err
		}
		if err := applyFileTransactionOp(op, hooks); err != nil {
			return newFileTransactionError(err, rollbackFileTransaction(applied, hooks))
		}
		applied = append(applied, appliedFileTransactionOp{
			kind:   op.kind,
			path:   op.path,
			before: before,
		})
	}
	return nil
}

func validateFileTransactionOps(ops []FileTransactionOp) error {
	for _, op := range ops {
		if strings.TrimSpace(op.path) == "" {
			return errors.New("file transaction path is required")
		}
		switch op.kind {
		case fileTransactionOpWrite, fileTransactionOpRemove, fileTransactionOpRemoveAll:
		default:
			return fmt.Errorf("unknown file transaction operation %q for %s", op.kind, op.path)
		}
	}
	return nil
}

func snapshotPathForTransaction(op FileTransactionOp) (fileSnapshot, error) {
	if op.kind == fileTransactionOpRemoveAll {
		return snapshotTreeForTransaction(op.path)
	}
	return snapshotFileForTransaction(op.path)
}

func snapshotFileForTransaction(path string) (fileSnapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fileSnapshot{}, nil
		}
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", path, err)
	}
	if info.IsDir() {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: path is a directory", path)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- transaction paths are supplied by governed artifact/state code or tests.
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", path, err)
	}
	return fileSnapshot{
		existed: true,
		data:    data,
		perm:    info.Mode().Perm(),
	}, nil
}

func snapshotTreeForTransaction(path string) (fileSnapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fileSnapshot{}, nil
		}
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", path, err)
	}
	if !info.IsDir() {
		return snapshotFileForTransaction(path)
	}

	snapshot := fileSnapshot{
		existed: true,
		isDir:   true,
		perm:    info.Mode().Perm(),
	}
	err = filepath.WalkDir(path, func(entryPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entryPath == path {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, entryPath)
		if err != nil {
			return err
		}
		item := fileTreeSnapshotEntry{
			rel:   rel,
			isDir: info.IsDir(),
			perm:  info.Mode().Perm(),
		}
		if !info.IsDir() {
			data, err := os.ReadFile(entryPath) // #nosec G304 -- transaction paths are supplied by governed artifact/state code or tests.
			if err != nil {
				return err
			}
			item.data = data
		}
		snapshot.entries = append(snapshot.entries, item)
		return nil
	})
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

func applyFileTransactionOp(op FileTransactionOp, hooks fileTransactionHooks) error {
	switch op.kind {
	case fileTransactionOpWrite:
		if err := hooks.writeFile(op.path, op.data, op.perm); err != nil {
			return fmt.Errorf("write %s: %w", op.path, err)
		}
	case fileTransactionOpRemove:
		if err := hooks.removeFile(op.path); err != nil {
			return fmt.Errorf("remove %s: %w", op.path, err)
		}
	case fileTransactionOpRemoveAll:
		if err := hooks.removeAll(op.path); err != nil {
			return fmt.Errorf("remove %s: %w", op.path, err)
		}
	}
	return nil
}

func rollbackFileTransaction(applied []appliedFileTransactionOp, hooks fileTransactionHooks) []error {
	rollbackErrs := make([]error, 0)
	for i := len(applied) - 1; i >= 0; i-- {
		item := applied[i]
		var err error
		if item.before.existed && item.before.isDir {
			err = restoreFileTreeSnapshot(item.path, item.before, hooks)
		} else if item.before.existed {
			err = hooks.writeFile(item.path, item.before.data, item.before.perm)
		} else if item.kind == fileTransactionOpRemoveAll {
			err = hooks.removeAll(item.path)
		} else {
			err = hooks.removeFile(item.path)
		}
		if err != nil {
			rollbackErrs = append(rollbackErrs, rollbackPathError{
				path: item.path,
				err:  err,
			})
		}
	}
	return rollbackErrs
}

func restoreFileTreeSnapshot(path string, snapshot fileSnapshot, hooks fileTransactionHooks) error {
	if err := hooks.removeAll(path); err != nil {
		return err
	}
	if err := os.MkdirAll(path, snapshot.perm); err != nil { // #nosec G301 -- transaction restores a previously snapshotted mode.
		return err
	}
	for _, item := range snapshot.entries {
		target := filepath.Join(path, item.rel)
		if item.isDir {
			if err := os.MkdirAll(target, item.perm); err != nil { // #nosec G301 -- transaction restores a previously snapshotted mode.
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { // #nosec G301 -- transaction restores caller-owned artifact directories.
			return err
		}
		if err := hooks.writeFile(target, item.data, item.perm); err != nil {
			return err
		}
	}
	return nil
}

func newFileTransactionError(operationErr error, rollbackErrs []error) error {
	return &FileTransactionError{
		OperationErr: operationErr,
		RollbackErrs: rollbackErrs,
	}
}

func removeTransactionFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) { // #nosec G122 -- transaction paths are supplied by governed artifact/state code or tests.
		return err
	}
	return nil
}

func removeTransactionPathIfExists(path string) error {
	if err := os.RemoveAll(path); err != nil && !errors.Is(err, fs.ErrNotExist) { // #nosec G122 -- transaction paths are supplied by governed artifact/state code or tests.
		return err
	}
	return nil
}
