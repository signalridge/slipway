package fsutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const (
	transactionQuarantineItem = "snapshot"
	transactionStageItem      = "new"
)

type transactionFilesystem struct {
	rootPath string
	root     *os.Root
	hooks    FileTransactionHooks
}

type fileQuarantine struct {
	filesystem        *transactionFilesystem
	originalPath      string
	recoveryPath      string
	directoryPath     string
	originalBase      string
	itemName          string
	directoryName     string
	directoryIdentity os.FileInfo
	parent            *os.Root
	directory         *os.Root
	closed            bool
}

// ApplyFileTransactionWithin confines every mutation to root using os.Root.
func ApplyFileTransactionWithin(rootPath string, ops []FileTransactionOp) error {
	return applyFileTransactionAt(rootPath, ops, FileTransactionHooks{}, true)
}

// ApplyFileTransactionWithinWithHooks is the rooted adversarial-test variant.
func ApplyFileTransactionWithinWithHooks(rootPath string, ops []FileTransactionOp, hooks FileTransactionHooks) error {
	return applyFileTransactionAt(rootPath, ops, hooks, true)
}

func applyFileTransactionAt(rootPath string, ops []FileTransactionOp, hooks FileTransactionHooks, restricted bool) error {
	if err := validateFileTransactionOps(ops); err != nil {
		return err
	}
	if len(ops) == 0 {
		return nil
	}
	normalized := slices.Clone(ops)
	for index := range normalized {
		absolute, err := filepath.Abs(normalized[index].path)
		if err != nil {
			return fmt.Errorf("absolute transaction path %s: %w", normalized[index].path, err)
		}
		normalized[index].path = filepath.Clean(absolute)
	}

	var absoluteRoot string
	if restricted {
		absolute, err := filepath.Abs(rootPath)
		if err != nil {
			return fmt.Errorf("absolute transaction root: %w", err)
		}
		absoluteRoot = filepath.Clean(absolute)
	} else {
		absoluteRoot = filepath.Dir(normalized[0].path)
		for _, op := range normalized[1:] {
			candidate := filepath.Dir(op.path)
			for !PathWithin(absoluteRoot, candidate) {
				parent := filepath.Dir(absoluteRoot)
				if parent == absoluteRoot {
					break
				}
				absoluteRoot = parent
			}
		}
		for {
			info, statErr := os.Stat(absoluteRoot)
			if statErr == nil && info.IsDir() {
				break
			}
			parent := filepath.Dir(absoluteRoot)
			if parent == absoluteRoot {
				return fmt.Errorf("find existing transaction root for %s: %w", normalized[0].path, statErr)
			}
			absoluteRoot = parent
		}
	}
	for _, op := range normalized {
		if !PathWithin(absoluteRoot, op.path) || (!restricted && filepath.VolumeName(op.path) != filepath.VolumeName(normalized[0].path)) {
			return fmt.Errorf("transaction path %q escapes root %q", op.path, absoluteRoot)
		}
	}
	root, err := os.OpenRoot(absoluteRoot)
	if err != nil {
		return fmt.Errorf("open transaction root: %w", err)
	}
	filesystem := &transactionFilesystem{rootPath: absoluteRoot, root: root, hooks: hooks}
	defer root.Close()
	return applyFileTransactionWithFilesystem(normalized, filesystem)
}

func (filesystem *transactionFilesystem) relative(path string) (string, error) {
	if !PathWithin(filesystem.rootPath, path) {
		return "", fmt.Errorf("transaction path %q escapes root %q", path, filesystem.rootPath)
	}
	name, err := filepath.Rel(filesystem.rootPath, path)
	if err != nil {
		return "", err
	}
	name = filepath.Clean(name)
	if name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("transaction path %q is not below root %q", path, filesystem.rootPath)
	}
	return name, nil
}

func (filesystem *transactionFilesystem) snapshot(path string, kind fileTransactionOpKind) (fileSnapshot, error) {
	parent, base, err := filesystem.openStableParent(path, false)
	if errors.Is(err, fs.ErrNotExist) {
		return fileSnapshot{parentObserved: true}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	defer parent.Close()
	parentInfo, err := rootDirectoryInfo(parent)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot parent %s: %w", filepath.Dir(path), err)
	}
	snapshot, err := snapshotPathForTransactionInRoot(parent, base, path, kind)
	if err != nil {
		return fileSnapshot{}, err
	}
	snapshot.parentObserved = true
	snapshot.parentExisted = true
	snapshot.parentIdentity = parentInfo
	return snapshot, nil
}

func snapshotPathForTransactionInRoot(root *os.Root, name, displayPath string, kind fileTransactionOpKind) (fileSnapshot, error) {
	if kind == fileTransactionOpRemoveAll {
		return snapshotTreeForTransactionInRoot(root, name, displayPath)
	}
	return snapshotFileForTransactionInRoot(root, name, displayPath)
}

func snapshotFileForTransactionInRoot(root *os.Root, name, displayPath string) (fileSnapshot, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := root.Readlink(name)
		if readErr != nil {
			return fileSnapshot{}, fmt.Errorf("snapshot symlink %s: %w", displayPath, readErr)
		}
		current, statErr := root.Lstat(name)
		if statErr != nil || current.Mode()&os.ModeSymlink == 0 || !os.SameFile(info, current) {
			if statErr != nil {
				return fileSnapshot{}, fmt.Errorf("snapshot symlink %s: %w", displayPath, statErr)
			}
			return fileSnapshot{}, fmt.Errorf("snapshot symlink %s: path changed while reading", displayPath)
		}
		return fileSnapshot{existed: true, isSymlink: true, linkTarget: target, perm: info.Mode().Perm(), identity: info}, nil
	}
	if !info.Mode().IsRegular() {
		if info.IsDir() {
			return fileSnapshot{}, fmt.Errorf("snapshot %s: path is a directory", displayPath)
		}
		return fileSnapshot{}, fmt.Errorf("snapshot %s: path is not a regular file or symbolic link", displayPath)
	}
	file, err := root.Open(name)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, err)
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		_ = file.Close()
		if statErr != nil {
			return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, statErr)
		}
		return fileSnapshot{}, fmt.Errorf("snapshot %s: path changed while opening", displayPath)
	}
	data, readErr := io.ReadAll(file)
	final, finalStatErr := file.Stat()
	closeErr := file.Close()
	current, lstatErr := root.Lstat(name)
	if readErr != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, readErr)
	}
	if finalStatErr != nil || lstatErr != nil || !os.SameFile(info, final) || !os.SameFile(info, current) {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: file changed while reading", displayPath)
	}
	if closeErr != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, closeErr)
	}
	return fileSnapshot{existed: true, data: data, perm: info.Mode().Perm(), identity: info}, nil
}

func snapshotTreeForTransactionInRoot(root *os.Root, name, displayPath string) (fileSnapshot, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return snapshotFileForTransactionInRoot(root, name, displayPath)
	}
	treeRoot, err := root.OpenRoot(name)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("open snapshot root %s: %w", displayPath, err)
	}
	defer treeRoot.Close()
	directory, err := treeRoot.Open(".")
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("open snapshot root %s: %w", displayPath, err)
	}
	opened, statErr := directory.Stat()
	closeErr := directory.Close()
	current, lstatErr := root.Lstat(name)
	if statErr != nil || closeErr != nil || lstatErr != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(info, opened) || !os.SameFile(opened, current) {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: directory changed while opening", displayPath)
	}
	snapshot := fileSnapshot{existed: true, isDir: true, perm: info.Mode().Perm(), identity: info}
	err = fs.WalkDir(treeRoot.FS(), ".", func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entryPath == "." {
			return nil
		}
		local := filepath.FromSlash(entryPath)
		entryInfo, err := treeRoot.Lstat(local)
		if err != nil {
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 || (!entryInfo.IsDir() && !entryInfo.Mode().IsRegular()) {
			return fmt.Errorf("snapshot %s: tree contains a symlink or special file", filepath.Join(displayPath, local))
		}
		item := fileTreeSnapshotEntry{rel: local, isDir: entryInfo.IsDir(), perm: entryInfo.Mode().Perm(), identity: entryInfo}
		if !entryInfo.IsDir() {
			fileSnapshot, err := snapshotFileForTransactionInRoot(treeRoot, local, filepath.Join(displayPath, local))
			if err != nil {
				return err
			}
			item.data = fileSnapshot.data
			item.identity = fileSnapshot.identity
		}
		snapshot.entries = append(snapshot.entries, item)
		return nil
	})
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, err)
	}
	current, err = root.Lstat(name)
	if err != nil || !current.IsDir() || !os.SameFile(info, current) {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: directory changed while reading", displayPath)
	}
	return snapshot, nil
}

func rootDirectoryInfo(root *os.Root) (os.FileInfo, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	info, statErr := directory.Stat()
	closeErr := directory.Close()
	if statErr != nil || closeErr != nil {
		return nil, errors.Join(statErr, closeErr)
	}
	if !info.IsDir() {
		return nil, errors.New("opened transaction parent is not a directory")
	}
	return info, nil
}

func validateGuardedParent(parent *os.Root, path string, guard fileSnapshot) (os.FileInfo, error) {
	info, err := rootDirectoryInfo(parent)
	if err != nil {
		return nil, err
	}
	if guard.parentObserved && guard.parentExisted && guard.parentIdentity != nil && !os.SameFile(info, guard.parentIdentity) {
		return nil, &FileTransactionRecoveryError{OriginalPath: path, Cause: errors.New("destination parent changed after namespace validation")}
	}
	return info, nil
}

func (filesystem *transactionFilesystem) openStableParent(path string, create bool) (*os.Root, string, error) {
	name, err := filesystem.relative(path)
	if err != nil {
		return nil, "", err
	}
	parent, err := filesystem.openStableDirectory(filepath.Dir(name), create)
	if err != nil {
		return nil, "", fmt.Errorf("open transaction parent %s: %w", filepath.Dir(path), err)
	}
	return parent, filepath.Base(name), nil
}

func (filesystem *transactionFilesystem) openStableDirectory(name string, create bool) (*os.Root, error) {
	current, err := filesystem.root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	if name == "." {
		return current, nil
	}
	for _, part := range strings.Split(filepath.Clean(name), string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		info, err := current.Lstat(part)
		created := false
		if errors.Is(err, fs.ErrNotExist) && create {
			if err := current.Mkdir(part, 0o755); err != nil {
				_ = current.Close()
				if errors.Is(err, fs.ErrExist) {
					return nil, fmt.Errorf("%w: parent component %s appeared concurrently", ErrFileTransactionConcurrentEdit, part)
				}
				return nil, err
			}
			created = true
			info, err = current.Lstat(part)
		}
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("transaction parent component %q is not a real directory", part)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		directory, err := next.Open(".")
		if err != nil {
			_ = next.Close()
			_ = current.Close()
			return nil, err
		}
		opened, statErr := directory.Stat()
		latest, lstatErr := current.Lstat(part)
		if created && statErr == nil && lstatErr == nil && os.SameFile(info, opened) && os.SameFile(opened, latest) {
			statErr = directory.Chmod(0o755)
		}
		closeErr := directory.Close()
		if statErr != nil || lstatErr != nil || closeErr != nil || !os.SameFile(info, opened) || !os.SameFile(opened, latest) || latest.Mode()&os.ModeSymlink != 0 || !latest.IsDir() {
			_ = next.Close()
			_ = current.Close()
			return nil, fmt.Errorf("transaction parent component %q changed while opening", part)
		}
		if err := current.Close(); err != nil {
			_ = next.Close()
			return nil, err
		}
		current = next
	}
	return current, nil
}

func (filesystem *transactionFilesystem) quarantineExpected(path string, expected fileSnapshot, rollback bool) (*fileQuarantine, bool, error) {
	if err := callFileTransactionHook(filesystem.hooks.AfterGuardBeforeQuarantine, path, ""); err != nil {
		return nil, false, &FileTransactionRecoveryError{OriginalPath: path, Rollback: rollback, Cause: fmt.Errorf("after guard snapshot: %w", err)}
	}
	parent, base, err := filesystem.openStableParent(path, false)
	if expected.parentObserved && !expected.parentExisted {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		if err == nil {
			_ = parent.Close()
			return nil, false, &FileTransactionRecoveryError{OriginalPath: path, Rollback: rollback, Cause: errors.New("destination parent appeared after the guard snapshot")}
		}
	}
	if err != nil {
		return nil, false, &FileTransactionRecoveryError{OriginalPath: path, Rollback: rollback, Cause: fmt.Errorf("open guarded parent: %w", err)}
	}
	parentInfo, err := rootDirectoryInfo(parent)
	if err != nil {
		_ = parent.Close()
		return nil, false, &FileTransactionRecoveryError{OriginalPath: path, Rollback: rollback, Cause: fmt.Errorf("inspect guarded parent: %w", err)}
	}
	if expected.parentObserved && expected.parentExisted && expected.parentIdentity != nil && !os.SameFile(parentInfo, expected.parentIdentity) {
		_ = parent.Close()
		return nil, false, &FileTransactionRecoveryError{OriginalPath: path, Rollback: rollback, Cause: errors.New("destination parent changed after the guard snapshot")}
	}
	if !expected.existed {
		_ = parent.Close()
		return nil, false, nil
	}
	quarantine, err := filesystem.allocateQuarantine(parent, base, path, transactionQuarantineItem)
	if err != nil {
		_ = parent.Close()
		if rollback {
			var recoveryErr *FileTransactionRecoveryError
			recovery := ""
			if errors.As(err, &recoveryErr) {
				recovery = recoveryErr.RecoveryPath
			}
			return nil, false, &FileTransactionRecoveryError{OriginalPath: path, RecoveryPath: recovery, Rollback: true, Cause: err}
		}
		return nil, false, err
	}
	if err := renameNoReplaceRoots(parent, quarantine.directory, base, transactionQuarantineItem); err != nil {
		cleanupErr := quarantine.removeEmptyDirectory()
		if errors.Is(err, ErrFileTransactionNoReplaceUnsupported) {
			return nil, false, errors.Join(fmt.Errorf("quarantine %s: %w", path, err), cleanupErr)
		}
		recovery := ""
		if cleanupErr != nil {
			recovery = quarantine.directoryPath
		}
		return nil, false, &FileTransactionRecoveryError{
			OriginalPath: path,
			RecoveryPath: recovery,
			Rollback:     rollback,
			Cause:        errors.Join(fmt.Errorf("guarded path changed before quarantine: %w", err), cleanupErr),
		}
	}
	if err := callFileTransactionHook(filesystem.hooks.AfterQuarantineBeforeValidation, path, quarantine.recoveryPath); err != nil {
		return nil, true, filesystem.recoverMismatchedQuarantine(quarantine, rollback, fmt.Errorf("after quarantine: %w", err))
	}
	actual, err := snapshotPathForTransactionInRoot(quarantine.directory, transactionQuarantineItem, quarantine.recoveryPath, snapshotKind(expected))
	if err != nil || !transactionSnapshotsMatchIgnoringParent(actual, expected, true) {
		if err == nil {
			err = errors.New("quarantined path does not match the guarded inode and contents")
		}
		return nil, true, filesystem.recoverMismatchedQuarantine(quarantine, rollback, err)
	}
	return quarantine, true, nil
}

func (filesystem *transactionFilesystem) allocateQuarantine(parent *os.Root, originalBase, originalPath, itemName string) (*fileQuarantine, error) {
	for range 32 {
		name, err := randomTransactionName(".slipway-recovery-")
		if err != nil {
			return nil, err
		}
		if err := parent.Mkdir(name, 0o700); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		directoryPath := filepath.Join(filepath.Dir(originalPath), name)
		recoveryPath := filepath.Join(directoryPath, itemName)
		allocationError := func(cause error) (*fileQuarantine, error) {
			return nil, &FileTransactionRecoveryError{
				OriginalPath: originalPath,
				RecoveryPath: recoveryPath,
				Cause:        fmt.Errorf("private transaction quarantine could not be safely initialized: %w", cause),
			}
		}
		info, err := parent.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return allocationError(errors.Join(errors.New("created quarantine path is not a stable directory"), err))
		}
		directory, err := parent.OpenRoot(name)
		if err != nil {
			return allocationError(err)
		}
		dirFile, err := directory.Open(".")
		if err != nil {
			_ = directory.Close()
			return allocationError(err)
		}
		opened, statErr := dirFile.Stat()
		chmodErr := dirFile.Chmod(0o700)
		closeErr := dirFile.Close()
		current, lstatErr := parent.Lstat(name)
		if statErr != nil || chmodErr != nil || closeErr != nil || lstatErr != nil || !os.SameFile(info, opened) || !os.SameFile(opened, current) || current.Mode().Perm()&0o077 != 0 {
			_ = directory.Close()
			return allocationError(errors.Join(errors.New("transaction quarantine is not private and stable"), statErr, chmodErr, closeErr, lstatErr))
		}
		return &fileQuarantine{
			filesystem:        filesystem,
			originalPath:      originalPath,
			recoveryPath:      recoveryPath,
			directoryPath:     directoryPath,
			originalBase:      originalBase,
			itemName:          itemName,
			directoryName:     name,
			directoryIdentity: current,
			parent:            parent,
			directory:         directory,
		}, nil
	}
	return nil, errors.New("allocate private transaction quarantine: name attempts exhausted")
}

func randomTransactionName(prefix string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random[:]), nil
}

func (filesystem *transactionFilesystem) recoverMismatchedQuarantine(quarantine *fileQuarantine, rollback bool, cause error) error {
	err := renameNoReplaceRoots(quarantine.directory, quarantine.parent, transactionQuarantineItem, quarantine.originalBase)
	if err == nil {
		removeErr := quarantine.removeEmptyDirectory()
		if removeErr != nil {
			return &FileTransactionRecoveryError{OriginalPath: quarantine.originalPath, RecoveryPath: quarantine.directoryPath, Reattached: true, Rollback: rollback, Cause: errors.Join(cause, removeErr)}
		}
		return &FileTransactionRecoveryError{OriginalPath: quarantine.originalPath, RecoveryPath: quarantine.recoveryPath, Reattached: true, Rollback: rollback, Cause: cause}
	}
	quarantine.preserve()
	return &FileTransactionRecoveryError{OriginalPath: quarantine.originalPath, RecoveryPath: quarantine.recoveryPath, Rollback: rollback, Cause: errors.Join(cause, fmt.Errorf("reattach without replacement: %w", err))}
}

func (filesystem *transactionFilesystem) writeFileAtomicNoReplace(path string, data []byte, perm os.FileMode, guard fileSnapshot) (fileSnapshot, bool, error) {
	parent, base, err := filesystem.openStableParent(path, true)
	if err != nil {
		return fileSnapshot{}, false, err
	}
	parentInfo, err := rootDirectoryInfo(parent)
	if err != nil {
		_ = parent.Close()
		return fileSnapshot{}, false, err
	}
	if guard.parentObserved && guard.parentExisted && guard.parentIdentity != nil && !os.SameFile(parentInfo, guard.parentIdentity) {
		_ = parent.Close()
		return fileSnapshot{}, false, &FileTransactionRecoveryError{OriginalPath: path, Cause: errors.New("destination parent changed before atomic installation")}
	}
	stage, err := filesystem.allocateQuarantine(parent, base, path, transactionStageItem)
	if err != nil {
		_ = parent.Close()
		return fileSnapshot{}, false, err
	}
	file, err := stage.directory.OpenFile(transactionStageItem, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = stage.removeEmptyDirectory()
		return fileSnapshot{}, false, err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		stage.preserve()
		return fileSnapshot{}, false, err
	}
	if err := file.Chmod(perm.Perm()); err != nil {
		_ = file.Close()
		stage.preserve()
		return fileSnapshot{}, false, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		stage.preserve()
		return fileSnapshot{}, false, err
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || closeErr != nil {
		stage.preserve()
		return fileSnapshot{}, false, errors.Join(statErr, closeErr)
	}
	expected := fileSnapshot{
		existed:        true,
		data:           slices.Clone(data),
		perm:           perm.Perm(),
		identity:       info,
		parentObserved: true,
		parentExisted:  true,
		parentIdentity: parentInfo,
	}
	if err := renameNoReplaceRoots(stage.directory, parent, transactionStageItem, base); err != nil {
		cleanupErr := filesystem.cleanupQuarantine(stage, expected, nil, false)
		return fileSnapshot{}, false, errors.Join(fmt.Errorf("install without replacement: %w", err), cleanupErr)
	}
	if err := stage.removeEmptyDirectory(); err != nil {
		return expected, true, fmt.Errorf("remove empty staging quarantine: %w", err)
	}
	return expected, true, nil
}

func (filesystem *transactionFilesystem) restoreSnapshotExclusive(path string, snapshot, namespaceGuard fileSnapshot) (fileSnapshot, error) {
	if !snapshot.existed {
		current, err := filesystem.snapshot(path, snapshotKind(snapshot))
		if err != nil {
			return fileSnapshot{}, err
		}
		if current.existed {
			return fileSnapshot{}, fmt.Errorf("%w: destination appeared before restoring missing pre-state", ErrFileTransactionConcurrentEdit)
		}
		return current, nil
	}
	if snapshot.isDir {
		return filesystem.restoreTreeExclusive(path, snapshot, namespaceGuard)
	}
	if snapshot.isSymlink {
		return filesystem.restoreSymlinkExclusive(path, snapshot, namespaceGuard)
	}
	return filesystem.restoreFileExclusive(path, snapshot, namespaceGuard)
}

func (filesystem *transactionFilesystem) restoreFileExclusive(path string, snapshot, namespaceGuard fileSnapshot) (fileSnapshot, error) {
	parent, base, err := filesystem.openStableParent(path, true)
	if err != nil {
		return fileSnapshot{}, err
	}
	defer parent.Close()
	parentInfo, err := validateGuardedParent(parent, path, namespaceGuard)
	if err != nil {
		return fileSnapshot{}, err
	}
	file, err := parent.OpenFile(base, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("exclusive create %s: %w", path, err)
	}
	if err := callFileTransactionHook(filesystem.hooks.DuringExclusiveRestore, path, ""); err != nil {
		_ = file.Close()
		return fileSnapshot{}, err
	}
	if _, err := file.Write(snapshot.data); err != nil {
		_ = file.Close()
		return fileSnapshot{}, err
	}
	if err := file.Chmod(snapshot.perm); err != nil {
		_ = file.Close()
		return fileSnapshot{}, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fileSnapshot{}, err
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || closeErr != nil {
		return fileSnapshot{}, errors.Join(statErr, closeErr)
	}
	if err := syncRootDirectory(parent); err != nil {
		return fileSnapshot{}, err
	}
	restored := snapshot
	restored.identity = info
	restored.parentObserved = true
	restored.parentExisted = true
	restored.parentIdentity = parentInfo
	return restored, nil
}

func (filesystem *transactionFilesystem) restoreSymlinkExclusive(path string, snapshot, namespaceGuard fileSnapshot) (fileSnapshot, error) {
	parent, base, err := filesystem.openStableParent(path, true)
	if err != nil {
		return fileSnapshot{}, err
	}
	defer parent.Close()
	parentInfo, err := validateGuardedParent(parent, path, namespaceGuard)
	if err != nil {
		return fileSnapshot{}, err
	}
	if err := callFileTransactionHook(filesystem.hooks.DuringExclusiveRestore, path, ""); err != nil {
		return fileSnapshot{}, err
	}
	if err := parent.Symlink(snapshot.linkTarget, base); err != nil {
		return fileSnapshot{}, fmt.Errorf("exclusive symlink create %s: %w", path, err)
	}
	info, err := parent.Lstat(base)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return fileSnapshot{}, fmt.Errorf("validate restored symlink %s: %w", path, err)
	}
	if err := syncRootDirectory(parent); err != nil {
		return fileSnapshot{}, err
	}
	restored := snapshot
	restored.identity = info
	restored.parentObserved = true
	restored.parentExisted = true
	restored.parentIdentity = parentInfo
	return restored, nil
}

func (filesystem *transactionFilesystem) restoreTreeExclusive(path string, snapshot, namespaceGuard fileSnapshot) (fileSnapshot, error) {
	parent, base, err := filesystem.openStableParent(path, true)
	if err != nil {
		return fileSnapshot{}, err
	}
	defer parent.Close()
	parentInfo, err := validateGuardedParent(parent, path, namespaceGuard)
	if err != nil {
		return fileSnapshot{}, err
	}
	if err := parent.Mkdir(base, 0o700); err != nil {
		return fileSnapshot{}, fmt.Errorf("exclusive directory create %s: %w", path, err)
	}
	created, err := parent.Lstat(base)
	if err != nil || !created.IsDir() || created.Mode()&os.ModeSymlink != 0 {
		return fileSnapshot{}, fmt.Errorf("validate restored directory %s: %w", path, err)
	}
	treeRoot, err := parent.OpenRoot(base)
	if err != nil {
		return fileSnapshot{}, err
	}
	defer treeRoot.Close()
	rootFile, err := treeRoot.Open(".")
	if err != nil {
		return fileSnapshot{}, err
	}
	defer rootFile.Close()
	opened, err := rootFile.Stat()
	if err != nil || !os.SameFile(created, opened) {
		return fileSnapshot{}, fmt.Errorf("restored directory %s changed while opening", path)
	}
	if err := callFileTransactionHook(filesystem.hooks.DuringExclusiveRestore, path, ""); err != nil {
		return fileSnapshot{}, err
	}

	restored := snapshot
	restored.identity = opened
	restored.parentObserved = true
	restored.parentExisted = true
	restored.parentIdentity = parentInfo
	restored.entries = slices.Clone(snapshot.entries)
	roots := map[string]*os.Root{".": treeRoot}
	directoryFiles := map[string]*os.File{".": rootFile}
	var ownedRoots []*os.Root
	var ownedFiles []*os.File
	defer func() {
		for _, file := range ownedFiles {
			_ = file.Close()
		}
		for _, root := range ownedRoots {
			_ = root.Close()
		}
	}()

	for index, item := range snapshot.entries {
		clean, err := cleanTreeSnapshotPath(item.rel)
		if err != nil {
			return fileSnapshot{}, err
		}
		parentRel := filepath.Dir(clean)
		parentRoot, ok := roots[parentRel]
		if !ok {
			return fileSnapshot{}, fmt.Errorf("restore tree %s: parent %s is missing from snapshot", path, parentRel)
		}
		name := filepath.Base(clean)
		if item.isDir {
			if err := parentRoot.Mkdir(name, 0o700); err != nil {
				return fileSnapshot{}, fmt.Errorf("exclusive directory create %s: %w", filepath.Join(path, clean), err)
			}
			info, err := parentRoot.Lstat(name)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fileSnapshot{}, fmt.Errorf("validate restored directory %s: %w", filepath.Join(path, clean), err)
			}
			childRoot, err := parentRoot.OpenRoot(name)
			if err != nil {
				return fileSnapshot{}, err
			}
			dirFile, err := childRoot.Open(".")
			if err != nil {
				_ = childRoot.Close()
				return fileSnapshot{}, err
			}
			opened, err := dirFile.Stat()
			if err != nil || !os.SameFile(info, opened) {
				_ = dirFile.Close()
				_ = childRoot.Close()
				return fileSnapshot{}, fmt.Errorf("restored directory %s changed while opening", filepath.Join(path, clean))
			}
			roots[clean] = childRoot
			directoryFiles[clean] = dirFile
			ownedRoots = append(ownedRoots, childRoot)
			ownedFiles = append(ownedFiles, dirFile)
			restored.entries[index].identity = opened
			continue
		}
		file, err := parentRoot.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return fileSnapshot{}, fmt.Errorf("exclusive file create %s: %w", filepath.Join(path, clean), err)
		}
		if _, err := file.Write(item.data); err != nil {
			_ = file.Close()
			return fileSnapshot{}, err
		}
		if err := file.Chmod(item.perm); err != nil {
			_ = file.Close()
			return fileSnapshot{}, err
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fileSnapshot{}, err
		}
		info, statErr := file.Stat()
		closeErr := file.Close()
		if statErr != nil || closeErr != nil {
			return fileSnapshot{}, errors.Join(statErr, closeErr)
		}
		restored.entries[index].identity = info
	}
	for index := len(snapshot.entries) - 1; index >= 0; index-- {
		item := snapshot.entries[index]
		if !item.isDir {
			continue
		}
		if err := directoryFiles[item.rel].Chmod(item.perm); err != nil {
			return fileSnapshot{}, err
		}
	}
	if err := rootFile.Chmod(snapshot.perm); err != nil {
		return fileSnapshot{}, err
	}
	if err := syncRootDirectory(treeRoot); err != nil {
		return fileSnapshot{}, err
	}
	if err := syncRootDirectory(parent); err != nil {
		return fileSnapshot{}, err
	}
	return restored, nil
}

func cleanTreeSnapshotPath(path string) (string, error) {
	clean := filepath.Clean(path)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe tree snapshot path %q", path)
	}
	return clean, nil
}

func (filesystem *transactionFilesystem) cleanupQuarantine(quarantine *fileQuarantine, expected fileSnapshot, destinationExpected *fileSnapshot, rollback bool) error {
	if quarantine == nil || quarantine.closed {
		return nil
	}
	observedPath := quarantine.recoveryPath
	if err := callFileTransactionHook(filesystem.hooks.DuringQuarantineCleanup, quarantine.originalPath, observedPath); err != nil {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, fmt.Errorf("during quarantine cleanup: %w", err))
	}
	if destinationExpected != nil {
		destination, err := filesystem.snapshot(quarantine.originalPath, snapshotKind(*destinationExpected))
		if err != nil || !transactionSnapshotsMatch(destination, *destinationExpected, true) {
			return preserveQuarantineFailure(
				quarantine,
				rollback,
				observedPath,
				errors.Join(errors.New("destination changed during quarantine cleanup"), err),
			)
		}
	}
	private, err := quarantine.parent.Lstat(quarantine.directoryName)
	if err != nil || !private.IsDir() || private.Mode()&os.ModeSymlink != 0 || private.Mode().Perm()&0o077 != 0 ||
		quarantine.directoryIdentity == nil || !os.SameFile(private, quarantine.directoryIdentity) {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, errors.Join(errors.New("quarantine directory is no longer private and transaction-owned"), err))
	}
	actual, err := snapshotPathForTransactionInRoot(quarantine.directory, quarantine.itemName, quarantine.recoveryPath, snapshotKind(expected))
	if err != nil || !transactionSnapshotsMatchIgnoringParent(actual, expected, true) {
		if err == nil {
			err = errors.New("quarantined entry changed before cleanup")
		}
		return preserveQuarantineFailure(quarantine, rollback, observedPath, err)
	}
	if expected.isDir {
		err = quarantine.directory.RemoveAll(quarantine.itemName)
	} else {
		err = quarantine.directory.Remove(quarantine.itemName)
	}
	if err != nil {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, fmt.Errorf("remove proven transaction quarantine: %w", err))
	}
	missing, verifyErr := snapshotPathForTransactionInRoot(quarantine.directory, quarantine.itemName, quarantine.recoveryPath, snapshotKind(expected))
	if verifyErr != nil || missing.existed {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, errors.Join(errors.New("quarantine cleanup post-validation failed"), verifyErr))
	}
	if err := quarantine.removeEmptyDirectory(); err != nil {
		return &FileTransactionRecoveryError{OriginalPath: quarantine.originalPath, RecoveryPath: quarantine.directoryPath, Rollback: rollback, Cause: err}
	}
	return nil
}

func preserveQuarantineFailure(quarantine *fileQuarantine, rollback bool, observedPath string, cause error) error {
	_, retainedPath := quarantine.refreshRecoveryPath()
	quarantine.preserve()
	errs := []error{&FileTransactionRecoveryError{
		OriginalPath: quarantine.originalPath,
		RecoveryPath: retainedPath,
		Rollback:     rollback,
		Cause:        cause,
	}}
	if observedPath != retainedPath {
		errs = append(errs, &FileTransactionRecoveryError{
			OriginalPath: quarantine.originalPath,
			RecoveryPath: observedPath,
			Rollback:     rollback,
			Cause:        errors.New("concurrent quarantine namespace was preserved at the original recovery path"),
		})
	}
	return errors.Join(errs...)
}

func (quarantine *fileQuarantine) removeEmptyDirectory() error {
	if quarantine == nil || quarantine.closed {
		return nil
	}
	current, identityErr := quarantine.parent.Lstat(quarantine.directoryName)
	if identityErr != nil || quarantine.directoryIdentity == nil || !os.SameFile(current, quarantine.directoryIdentity) {
		oldPath, retainedPath := quarantine.refreshRecoveryPath()
		if quarantine.directory != nil {
			_ = quarantine.directory.Close()
			quarantine.directory = nil
		}
		if quarantine.parent != nil {
			_ = quarantine.parent.Close()
			quarantine.parent = nil
		}
		quarantine.closed = true
		namespaceErrs := []error{&FileTransactionRecoveryError{
			OriginalPath: quarantine.originalPath,
			RecoveryPath: retainedPath,
			Cause:        errors.Join(errors.New("transaction quarantine directory changed before removal"), identityErr),
		}}
		if oldPath != retainedPath {
			namespaceErrs = append(namespaceErrs, &FileTransactionRecoveryError{
				OriginalPath: quarantine.originalPath,
				RecoveryPath: oldPath,
				Cause:        errors.New("concurrent quarantine namespace preserved at the original recovery path"),
			})
		}
		return errors.Join(namespaceErrs...)
	}
	closeDirectoryErr := quarantine.directory.Close()
	quarantine.directory = nil
	removeErr := quarantine.parent.Remove(quarantine.directoryName)
	syncErr := syncRootDirectory(quarantine.parent)
	_, verifyErr := quarantine.parent.Lstat(quarantine.directoryName)
	if errors.Is(verifyErr, fs.ErrNotExist) {
		verifyErr = nil
	} else if verifyErr == nil {
		verifyErr = errors.New("quarantine directory still exists after cleanup")
	}
	closeParentErr := quarantine.parent.Close()
	quarantine.parent = nil
	quarantine.closed = true
	return errors.Join(closeDirectoryErr, removeErr, syncErr, verifyErr, closeParentErr)
}

func (quarantine *fileQuarantine) refreshRecoveryPath() (string, string) {
	oldPath := quarantine.recoveryPath
	if quarantine.filesystem == nil || quarantine.filesystem.root == nil || quarantine.directoryIdentity == nil {
		return oldPath, oldPath
	}
	found := errors.New("transaction quarantine found")
	_ = fs.WalkDir(quarantine.filesystem.root.FS(), ".", func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !entry.IsDir() || entryPath == "." {
			return nil
		}
		local := filepath.FromSlash(entryPath)
		info, err := quarantine.filesystem.root.Lstat(local)
		if err != nil || !os.SameFile(info, quarantine.directoryIdentity) {
			return nil
		}
		quarantine.directoryPath = filepath.Join(quarantine.filesystem.rootPath, local)
		quarantine.recoveryPath = filepath.Join(quarantine.directoryPath, quarantine.itemName)
		return found
	})
	return oldPath, quarantine.recoveryPath
}

func (quarantine *fileQuarantine) preserve() {
	if quarantine == nil || quarantine.closed {
		return
	}
	quarantine.refreshRecoveryPath()
	if quarantine.directory != nil {
		_ = quarantine.directory.Close()
		quarantine.directory = nil
	}
	if quarantine.parent != nil {
		_ = quarantine.parent.Close()
		quarantine.parent = nil
	}
	quarantine.closed = true
}

func renameNoReplaceRoots(oldRoot, newRoot *os.Root, oldName, newName string) error {
	oldDirectory, err := oldRoot.Open(".")
	if err != nil {
		return err
	}
	defer oldDirectory.Close()
	newDirectory, err := newRoot.Open(".")
	if err != nil {
		return err
	}
	defer newDirectory.Close()
	if err := renameNoReplaceAt(oldDirectory, oldName, newDirectory, newName); err != nil {
		return err
	}
	return errors.Join(syncFileDirectory(oldDirectory), syncFileDirectory(newDirectory))
}

func syncRootDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return syncFileDirectory(directory)
}

func syncFileDirectory(directory *os.File) error {
	if err := directory.Sync(); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}
