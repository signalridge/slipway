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
	maxManagedSnapshotBytes   = 16 << 20
)

type transactionFilesystem struct {
	rootPath      string
	root          *os.Root
	hooks         fileTransactionHooks
	identityLease *transactionIdentityLease
	// wroteFilesystem is set only after a transaction syscall has successfully
	// changed the pinned namespace. Preparation and failed mutation attempts do
	// not make a later root-identity error look committed.
	wroteFilesystem bool
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
	reattached        bool
}

// ApplyFileTransactionWithin confines every mutation to root using os.Root.
func ApplyFileTransactionWithin(rootPath string, ops []FileTransactionOp) error {
	return applyFileTransactionAt(rootPath, ops, fileTransactionHooks{}, true)
}

func applyFileTransactionAt(rootPath string, ops []FileTransactionOp, hooks fileTransactionHooks, restricted bool) error {
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
	snapshot, err := snapshotPathForTransactionInRoot(parent, base, path, kind, nil)
	if err != nil {
		return fileSnapshot{}, err
	}
	snapshot.parentObserved = true
	snapshot.parentExisted = true
	snapshot.parentIdentity = parentInfo
	return snapshot, nil
}

func (filesystem *transactionFilesystem) snapshotGuard(path string, kind fileTransactionOpKind, lease *transactionIdentityLease) (fileSnapshot, error) {
	parent, base, err := filesystem.openStableParent(path, false)
	if errors.Is(err, fs.ErrNotExist) {
		return fileSnapshot{parentObserved: true}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	lease.add(parent)
	parentInfo, err := rootDirectoryInfo(parent)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot parent %s: %w", filepath.Dir(path), err)
	}
	snapshot, err := snapshotPathForTransactionInRoot(parent, base, path, kind, lease)
	if err != nil {
		return fileSnapshot{}, err
	}
	snapshot.parentObserved = true
	snapshot.parentExisted = true
	snapshot.parentIdentity = parentInfo
	return snapshot, nil
}

func snapshotPathForTransactionInRoot(root *os.Root, name, displayPath string, _ fileTransactionOpKind, lease *transactionIdentityLease) (snapshot fileSnapshot, resultErr error) {
	// A caller-owned lease pins transaction guards for the full transaction.
	// Read-only validation snapshots get a local lease so every identity remains
	// pinned until that whole snapshot has been assembled and checked.
	if lease == nil {
		lease = &transactionIdentityLease{}
		defer func() {
			resultErr = errors.Join(resultErr, lease.close())
		}()
	}
	return snapshotFileForTransactionInRoot(root, name, displayPath, lease)
}

func snapshotFileForTransactionInRoot(root *os.Root, name, displayPath string, lease *transactionIdentityLease) (fileSnapshot, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot %s: %w", displayPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		pinned, err := openSymlinkIdentity(root, name)
		if err != nil {
			return fileSnapshot{}, fmt.Errorf("pin snapshot symlink %s: %w", displayPath, err)
		}
		pinnedInfo, statErr := pinned.Stat()
		if statErr != nil || pinnedInfo.Mode()&os.ModeSymlink == 0 || !os.SameFile(info, pinnedInfo) {
			_ = pinned.Close()
			return fileSnapshot{}, errors.Join(fmt.Errorf("snapshot symlink %s changed while pinning", displayPath), statErr)
		}
		if err := validateSymlinkTransactionIdentity(pinnedInfo); err != nil {
			_ = pinned.Close()
			return fileSnapshot{}, fmt.Errorf("snapshot symlink %s: %w", displayPath, err)
		}
		lease.add(pinned)
		target, readErr := readSymlinkIdentity(root, name, pinned)
		if readErr != nil {
			return fileSnapshot{}, fmt.Errorf("snapshot symlink %s: %w", displayPath, readErr)
		}
		current, statErr := root.Lstat(name)
		if statErr != nil || current.Mode()&os.ModeSymlink == 0 || !os.SameFile(pinnedInfo, current) {
			if statErr != nil {
				return fileSnapshot{}, fmt.Errorf("snapshot symlink %s: %w", displayPath, statErr)
			}
			return fileSnapshot{}, fmt.Errorf("snapshot symlink %s: path changed while reading", displayPath)
		}
		return fileSnapshot{existed: true, isSymlink: true, linkTarget: target, perm: info.Mode().Perm(), identity: pinnedInfo}, nil
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
	if opened.Size() > maxManagedSnapshotBytes {
		limitErr := &FileTransactionSnapshotLimitError{Path: displayPath, Observation: "oversize", Size: opened.Size(), Limit: maxManagedSnapshotBytes}
		return fileSnapshot{}, errors.Join(fmt.Errorf("snapshot %s: %w", displayPath, limitErr), file.Close())
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxManagedSnapshotBytes+1))
	if readErr != nil {
		return fileSnapshot{}, errors.Join(fmt.Errorf("snapshot %s: %w", displayPath, readErr), file.Close())
	}
	if int64(len(data)) > maxManagedSnapshotBytes {
		limitErr := &FileTransactionSnapshotLimitError{Path: displayPath, Observation: "oversize", Size: int64(len(data)), Limit: maxManagedSnapshotBytes}
		return fileSnapshot{}, errors.Join(fmt.Errorf("snapshot %s: %w", displayPath, limitErr), file.Close())
	}
	final, finalStatErr := file.Stat()
	current, lstatErr := root.Lstat(name)
	if finalStatErr != nil || lstatErr != nil || !os.SameFile(info, final) || !os.SameFile(info, current) {
		_ = file.Close()
		return fileSnapshot{}, fmt.Errorf("snapshot %s: file changed while reading", displayPath)
	}
	lease.add(file)
	return fileSnapshot{existed: true, data: data, perm: info.Mode().Perm(), identity: info}, nil
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
			filesystem.wroteFilesystem = true
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
	actual, err := snapshotPathForTransactionInRoot(quarantine.directory, transactionQuarantineItem, quarantine.recoveryPath, snapshotKind(expected), nil)
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
		filesystem.wroteFilesystem = true
		directoryPath := filepath.Join(filepath.Dir(originalPath), name)
		recoveryPath := filepath.Join(directoryPath, itemName)
		info, err := parent.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, &FileTransactionRecoveryError{
				OriginalPath: originalPath,
				RecoveryPath: directoryPath,
				Cause:        errors.Join(errors.New("created quarantine path is not a stable directory"), err),
			}
		}
		allocationError := func(cause error) (*fileQuarantine, error) {
			retained, cleanupErr := cleanupFailedQuarantineAllocation(parent, name, info)
			artifact := ""
			if retained {
				artifact = recoveryPath
			}
			return nil, &FileTransactionRecoveryError{
				OriginalPath: originalPath,
				RecoveryPath: artifact,
				Cause:        errors.Join(fmt.Errorf("private transaction quarantine could not be safely initialized: %w", cause), cleanupErr),
			}
		}

		// Harden before any fault or concurrent-observation hook can force this
		// directory to be retained as a recovery artifact. Validate it again
		// afterward so a hook cannot replace or relax the opened namespace.
		directory, _, err := secureTransactionQuarantineDirectory(parent, name, info)
		if err != nil {
			return allocationError(err)
		}
		if err := directory.Close(); err != nil {
			return allocationError(err)
		}
		if err := callFileTransactionHook(filesystem.hooks.AfterQuarantineMkdir, originalPath, directoryPath); err != nil {
			return allocationError(fmt.Errorf("after quarantine mkdir: %w", err))
		}
		directory, current, err := secureTransactionQuarantineDirectory(parent, name, info)
		if err != nil {
			return allocationError(err)
		}
		if err := callFileTransactionHook(filesystem.hooks.AfterQuarantineOpen, originalPath, directoryPath); err != nil {
			_ = directory.Close()
			return allocationError(fmt.Errorf("after quarantine open: %w", err))
		}
		pinnedDirectory, err := directory.OpenRoot(".")
		if err != nil {
			_ = directory.Close()
			return allocationError(fmt.Errorf("pin transaction quarantine directory: %w", err))
		}
		pinnedInfo, pinErr := rootDirectoryInfo(pinnedDirectory)
		if pinErr != nil || !os.SameFile(current, pinnedInfo) {
			_ = pinnedDirectory.Close()
			_ = directory.Close()
			return allocationError(errors.Join(errors.New("transaction quarantine changed while pinning"), pinErr))
		}
		if filesystem.identityLease != nil {
			filesystem.identityLease.add(pinnedDirectory)
		} else if closeErr := pinnedDirectory.Close(); closeErr != nil {
			_ = directory.Close()
			return allocationError(closeErr)
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

func secureTransactionQuarantineDirectory(parent *os.Root, name string, expected os.FileInfo) (*os.Root, os.FileInfo, error) {
	directory, err := parent.OpenRoot(name)
	if err != nil {
		return nil, nil, err
	}
	dirFile, err := directory.Open(".")
	if err != nil {
		return nil, nil, errors.Join(err, directory.Close())
	}
	opened, statErr := dirFile.Stat()
	chmodErr := dirFile.Chmod(0o700)
	aclErr := restrictToOwner(dirFile)
	current, lstatErr := parent.Lstat(name)
	private := lstatErr == nil && transactionQuarantineModeIsPrivate(directory, current.Mode())
	closeErr := dirFile.Close()
	if statErr != nil || chmodErr != nil || aclErr != nil || closeErr != nil || lstatErr != nil ||
		!os.SameFile(expected, opened) || !os.SameFile(opened, current) || !private {
		return nil, nil, errors.Join(
			errors.New("transaction quarantine is not private and stable"),
			statErr,
			chmodErr,
			aclErr,
			closeErr,
			lstatErr,
			directory.Close(),
		)
	}
	return directory, current, nil
}

func cleanupFailedQuarantineAllocation(parent *os.Root, name string, identity os.FileInfo) (bool, error) {
	current, err := parent.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil || identity == nil || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(current, identity) {
		return true, errors.Join(errors.New("failed quarantine allocation changed identity; recovery artifact retained"), err)
	}
	directory, err := parent.OpenRoot(name)
	if err != nil {
		return true, fmt.Errorf("open failed quarantine allocation for cleanup: %w", err)
	}
	entries, readErr := fs.ReadDir(directory.FS(), ".")
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil || len(entries) != 0 {
		return true, errors.Join(errors.New("failed quarantine allocation is not verifiably empty; recovery artifact retained"), readErr, closeErr)
	}
	current, err = parent.Lstat(name)
	if err != nil || !os.SameFile(current, identity) {
		return true, errors.Join(errors.New("failed quarantine allocation changed before cleanup; recovery artifact retained"), err)
	}
	if err := parent.Remove(name); err != nil {
		return true, fmt.Errorf("remove failed quarantine allocation: %w", err)
	}
	syncErr := syncRootDirectory(parent)
	_, verifyErr := parent.Lstat(name)
	if errors.Is(verifyErr, fs.ErrNotExist) {
		verifyErr = nil
	} else if verifyErr == nil {
		verifyErr = errors.New("failed quarantine allocation still exists after cleanup")
	}
	if err := errors.Join(syncErr, verifyErr); err != nil {
		return false, err
	}
	return false, nil
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

func (filesystem *transactionFilesystem) writeFileAtomicNoReplace(path string, data []byte, perm os.FileMode, guard fileSnapshot, lease *transactionIdentityLease) (fileSnapshot, bool, error) {
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
	pinnedParent, err := parent.OpenRoot(".")
	if err != nil {
		_ = parent.Close()
		return fileSnapshot{}, false, fmt.Errorf("pin transaction destination parent: %w", err)
	}
	lease.add(pinnedParent)
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
	if perm.Perm()&0o077 == 0 {
		if err := restrictToOwner(file); err != nil {
			_ = file.Close()
			stage.preserve()
			return fileSnapshot{}, false, fmt.Errorf("secure staged private file: %w", err)
		}
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
	if statErr != nil {
		closeErr := file.Close()
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
	lease.add(file)
	if err := renameNoReplaceRoots(stage.directory, parent, transactionStageItem, base); err != nil {
		cleanupErr := filesystem.cleanupQuarantine(stage, expected, nil, false)
		return fileSnapshot{}, false, errors.Join(fmt.Errorf("install without replacement: %w", err), cleanupErr)
	}
	if err := stage.removeEmptyDirectory(); err != nil {
		return expected, true, fmt.Errorf("remove empty staging quarantine: %w", err)
	}
	return expected, true, nil
}

func (filesystem *transactionFilesystem) reattachQuarantinedSnapshot(
	path string,
	snapshot, namespaceGuard fileSnapshot,
	quarantine *fileQuarantine,
	lease *transactionIdentityLease,
) (fileSnapshot, bool, error) {
	if quarantine == nil || quarantine.closed || !snapshot.existed {
		return fileSnapshot{}, false, nil
	}
	private, err := quarantine.parent.Lstat(quarantine.directoryName)
	if err != nil || !private.IsDir() || private.Mode()&os.ModeSymlink != 0 ||
		quarantine.directoryIdentity == nil || !os.SameFile(private, quarantine.directoryIdentity) ||
		!transactionQuarantineModeIsPrivate(quarantine.directory, private.Mode()) {
		return fileSnapshot{}, false, errors.Join(errors.New("pre-transaction quarantine changed before reattachment"), err)
	}
	actual, err := snapshotPathForTransactionInRoot(
		quarantine.directory,
		quarantine.itemName,
		quarantine.recoveryPath,
		snapshotKind(snapshot),
		lease,
	)
	if err != nil || !transactionSnapshotsMatchIgnoringParent(actual, snapshot, true) {
		return fileSnapshot{}, false, errors.Join(errors.New("pre-transaction object changed before reattachment"), err)
	}

	parent, base, err := filesystem.openStableParent(path, true)
	if err != nil {
		return fileSnapshot{}, false, err
	}
	defer parent.Close()
	parentInfo, err := validateGuardedParent(parent, path, namespaceGuard)
	if err != nil {
		return fileSnapshot{}, false, err
	}
	if _, err := parent.Lstat(base); !errors.Is(err, fs.ErrNotExist) {
		if err == nil {
			err = ErrFileTransactionConcurrentEdit
		}
		return fileSnapshot{}, false, fmt.Errorf("destination is not empty before reattaching pre-transaction object: %w", err)
	}
	if err := renameNoReplaceRoots(quarantine.directory, parent, quarantine.itemName, base); err != nil {
		return fileSnapshot{}, false, fmt.Errorf("reattach pre-transaction object without replacement: %w", err)
	}
	quarantine.reattached = true
	quarantine.recoveryPath = path

	restored, err := snapshotPathForTransactionInRoot(parent, base, path, snapshotKind(snapshot), lease)
	if err != nil {
		return fileSnapshot{}, true, err
	}
	restored.parentObserved = true
	restored.parentExisted = true
	restored.parentIdentity = parentInfo
	if !transactionSnapshotsMatch(restored, func() fileSnapshot {
		expected := snapshot
		expected.parentObserved = true
		expected.parentExisted = true
		expected.parentIdentity = parentInfo
		return expected
	}(), true) {
		return fileSnapshot{}, true, errors.New("reattached pre-transaction object changed during installation")
	}
	if err := callFileTransactionHook(filesystem.hooks.DuringExclusiveRestore, path, ""); err != nil {
		return fileSnapshot{}, true, err
	}
	return restored, true, nil
}

func (filesystem *transactionFilesystem) restoreSnapshotExclusive(path string, snapshot, namespaceGuard fileSnapshot, lease *transactionIdentityLease) (fileSnapshot, error) {
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
	if snapshot.isSymlink {
		return filesystem.restoreSymlinkExclusive(path, snapshot, namespaceGuard, lease)
	}
	return filesystem.restoreFileExclusive(path, snapshot, namespaceGuard, lease)
}

func (filesystem *transactionFilesystem) restoreFileExclusive(path string, snapshot, namespaceGuard fileSnapshot, lease *transactionIdentityLease) (fileSnapshot, error) {
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
	if snapshot.perm.Perm()&0o077 == 0 {
		if err := restrictToOwner(file); err != nil {
			_ = file.Close()
			return fileSnapshot{}, fmt.Errorf("secure restored private file: %w", err)
		}
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
	if statErr != nil {
		closeErr := file.Close()
		return fileSnapshot{}, errors.Join(statErr, closeErr)
	}
	lease.add(file)
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

func (filesystem *transactionFilesystem) restoreSymlinkExclusive(path string, snapshot, namespaceGuard fileSnapshot, lease *transactionIdentityLease) (fileSnapshot, error) {
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
	pinned, err := openSymlinkIdentity(parent, base)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("pin restored symlink %s: %w", path, err)
	}
	info, statErr := pinned.Stat()
	target, readErr := readSymlinkIdentity(parent, base, pinned)
	current, lstatErr := parent.Lstat(base)
	var kindErr error
	if statErr == nil {
		kindErr = validateSymlinkTransactionIdentity(info)
	}
	var targetErr error
	if readErr == nil && target != snapshot.linkTarget {
		targetErr = errors.New("restored symbolic-link target changed after creation")
	}
	if statErr != nil || readErr != nil || kindErr != nil || targetErr != nil || lstatErr != nil || info.Mode()&os.ModeSymlink == 0 || current.Mode()&os.ModeSymlink == 0 || !os.SameFile(info, current) {
		_ = pinned.Close()
		return fileSnapshot{}, errors.Join(fmt.Errorf("validate restored symlink %s", path), statErr, readErr, kindErr, targetErr, lstatErr)
	}
	lease.add(pinned)
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

func (filesystem *transactionFilesystem) cleanupQuarantine(quarantine *fileQuarantine, expected fileSnapshot, destinationExpected *fileSnapshot, rollback bool) error {
	if quarantine == nil || quarantine.closed {
		return nil
	}
	if quarantine.reattached {
		return quarantine.removeEmptyDirectory()
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
	if err != nil || !private.IsDir() || private.Mode()&os.ModeSymlink != 0 || !transactionQuarantineModeIsPrivate(quarantine.directory, private.Mode()) ||
		quarantine.directoryIdentity == nil || !os.SameFile(private, quarantine.directoryIdentity) {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, errors.Join(errors.New("quarantine directory is no longer private and transaction-owned"), err))
	}
	actual, err := snapshotPathForTransactionInRoot(quarantine.directory, quarantine.itemName, quarantine.recoveryPath, snapshotKind(expected), nil)
	if err != nil || !transactionSnapshotsMatchIgnoringParent(actual, expected, true) {
		if err == nil {
			err = errors.New("quarantined entry changed before cleanup")
		}
		return preserveQuarantineFailure(quarantine, rollback, observedPath, err)
	}
	if err := callFileTransactionHook(filesystem.hooks.AfterQuarantineValidationBeforeRelocation, quarantine.originalPath, observedPath); err != nil {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, fmt.Errorf("after quarantine validation: %w", err))
	}
	if err := relocateQuarantineItemForCleanup(quarantine); err != nil {
		return preserveQuarantineFailure(quarantine, rollback, quarantine.recoveryPath, fmt.Errorf("isolate validated quarantine before cleanup: %w", err))
	}
	observedPath = quarantine.recoveryPath
	deletionLease := &transactionIdentityLease{}
	actual, err = snapshotPathForTransactionInRoot(quarantine.directory, quarantine.itemName, quarantine.recoveryPath, snapshotKind(expected), deletionLease)
	if err != nil || !transactionSnapshotsMatchIgnoringParent(actual, expected, true) {
		closeErr := deletionLease.close()
		if err == nil {
			err = errors.New("relocated quarantine entry does not match the validated transaction identity")
		}
		return preserveQuarantineFailure(quarantine, rollback, observedPath, errors.Join(err, closeErr))
	}
	err = quarantine.directory.Remove(quarantine.itemName)
	closeErr := deletionLease.close()
	if err != nil {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, errors.Join(fmt.Errorf("remove isolated transaction quarantine: %w", err), closeErr))
	}
	missing, verifyErr := snapshotPathForTransactionInRoot(quarantine.directory, quarantine.itemName, quarantine.recoveryPath, snapshotKind(expected), nil)
	if verifyErr != nil || missing.existed {
		return preserveQuarantineFailure(quarantine, rollback, observedPath, errors.Join(errors.New("quarantine cleanup post-validation failed"), verifyErr, closeErr))
	}
	if err := quarantine.removeEmptyDirectory(); err != nil {
		return errors.Join(&FileTransactionRecoveryError{OriginalPath: quarantine.originalPath, RecoveryPath: quarantine.directoryPath, Rollback: rollback, Cause: err}, closeErr)
	}
	return closeErr
}

func relocateQuarantineItemForCleanup(quarantine *fileQuarantine) (resultErr error) {
	directory, err := quarantine.directory.Open(".")
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, directory.Close())
	}()
	for range 32 {
		name, err := randomTransactionName(".slipway-delete-")
		if err != nil {
			return err
		}
		if err := renameNoReplaceAt(directory, quarantine.itemName, directory, name); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return err
		}
		quarantine.itemName = name
		quarantine.recoveryPath = filepath.Join(quarantine.directoryPath, name)
		return syncFileDirectory(directory)
	}
	return errors.New("isolate quarantine for cleanup: name attempts exhausted")
}

func transactionQuarantineModeIsPrivate(root *os.Root, mode os.FileMode) bool {
	file, err := root.Open(".")
	if err != nil {
		return false
	}
	defer file.Close()
	return ownerProtectionIsPrivate(file, mode)
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
	if err := relocateQuarantineDirectoryForCleanup(quarantine); err != nil {
		observedPath := quarantine.directoryPath
		_, retainedPath := quarantine.refreshRecoveryPath()
		quarantine.preserve()
		return &FileTransactionRecoveryError{
			OriginalPath: quarantine.originalPath,
			RecoveryPath: filepath.Dir(retainedPath),
			Cause:        fmt.Errorf("isolate empty transaction quarantine before removal (observed at %s): %w", observedPath, err),
		}
	}
	current, identityErr = quarantine.parent.Lstat(quarantine.directoryName)
	if identityErr != nil || quarantine.directoryIdentity == nil || !os.SameFile(current, quarantine.directoryIdentity) {
		observedPath := quarantine.directoryPath
		_, retainedPath := quarantine.refreshRecoveryPath()
		quarantine.preserve()
		return &FileTransactionRecoveryError{
			OriginalPath: quarantine.originalPath,
			RecoveryPath: filepath.Dir(retainedPath),
			Cause:        errors.Join(fmt.Errorf("relocated transaction quarantine changed before removal (observed at %s)", observedPath), identityErr),
		}
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

func relocateQuarantineDirectoryForCleanup(quarantine *fileQuarantine) (resultErr error) {
	parent, err := quarantine.parent.Open(".")
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, parent.Close())
	}()
	for range 32 {
		name, err := randomTransactionName(".slipway-delete-dir-")
		if err != nil {
			return err
		}
		if err := renameNoReplaceAt(parent, quarantine.directoryName, parent, name); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return err
		}
		quarantine.directoryName = name
		quarantine.directoryPath = filepath.Join(filepath.Dir(quarantine.directoryPath), name)
		quarantine.recoveryPath = filepath.Join(quarantine.directoryPath, quarantine.itemName)
		return syncFileDirectory(parent)
	}
	return errors.New("isolate quarantine directory for cleanup: name attempts exhausted")
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
