package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
)

type fileTransactionOpKind string

const (
	fileTransactionOpWrite  fileTransactionOpKind = "write"
	fileTransactionOpRemove fileTransactionOpKind = "remove"
)

type fileTransactionPrecondition string

const (
	fileTransactionExpectNone    fileTransactionPrecondition = ""
	fileTransactionExpectMissing fileTransactionPrecondition = "missing"
	fileTransactionExpectSHA256  fileTransactionPrecondition = "sha256"
)

// ErrFileTransactionPrecondition identifies a path that changed after a
// transaction was planned but before its mutation could be applied.
var ErrFileTransactionPrecondition = errors.New("file transaction precondition failed")

// ErrFileTransactionRollbackPrecondition identifies an applied path that no
// longer has the transaction's exact post-state and was therefore preserved.
var ErrFileTransactionRollbackPrecondition = errors.New("file transaction rollback precondition failed")

// ErrFileTransactionConcurrentEdit identifies a path whose namespace entry or
// contents changed while a transaction or rollback was in progress.
var ErrFileTransactionConcurrentEdit = errors.New("file transaction concurrent edit")

// ErrFileTransactionNoReplaceUnsupported identifies a platform or filesystem
// that cannot provide an atomic no-replace rename. Mutations fail closed when
// this guarantee is unavailable.
var ErrFileTransactionNoReplaceUnsupported = errors.New("atomic no-replace rename unsupported")

// ErrFileTransactionSymlinkUnsupported identifies a symbolic-link kind that
// cannot be restored exactly on the current platform. The transaction fails
// before mutation rather than guessing a different link kind.
var ErrFileTransactionSymlinkUnsupported = errors.New("exact symbolic-link transaction unsupported")

// FileTransactionSnapshotLimitError reports a managed file that cannot be
// snapshotted within the transaction's bounded rollback buffer.
type FileTransactionSnapshotLimitError struct {
	Path        string
	Observation string
	Size        int64
	Limit       int64
}

func (err *FileTransactionSnapshotLimitError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("managed file exceeds %d-byte limit (observed %d bytes)", err.Limit, err.Size)
}

// FileTransactionOp describes one ordered file mutation in a file transaction.
type FileTransactionOp struct {
	kind           fileTransactionOpKind
	path           string
	data           []byte
	perm           os.FileMode
	precondition   fileTransactionPrecondition
	expectedSHA256 string
}

// WriteFileTransactionOp returns a transaction operation that atomically writes
// one file when the transaction is applied.
func WriteFileTransactionOp(path string, data []byte, perm os.FileMode) FileTransactionOp {
	return FileTransactionOp{kind: fileTransactionOpWrite, path: path, data: slices.Clone(data), perm: perm}
}

// RemoveFileTransactionOp returns a transaction operation that removes one file
// or an exactly restorable symbolic link when it exists.
func RemoveFileTransactionOp(path string) FileTransactionOp {
	return FileTransactionOp{kind: fileTransactionOpRemove, path: path}
}

// WithExpectedMissing requires the path to still be absent when the operation
// is applied.
func (op FileTransactionOp) WithExpectedMissing() FileTransactionOp {
	op.precondition = fileTransactionExpectMissing
	op.expectedSHA256 = ""
	return op
}

// WithExpectedSHA256 requires the regular file to still contain the bytes
// identified by expected when the operation is applied.
func (op FileTransactionOp) WithExpectedSHA256(expected string) FileTransactionOp {
	op.precondition = fileTransactionExpectSHA256
	op.expectedSHA256 = strings.ToLower(strings.TrimSpace(expected))
	return op
}

// fileTransactionHook is a deterministic, per-transaction test hook. The
// recovery path is empty until a path has been moved into private quarantine.
type fileTransactionHook func(originalPath, recoveryPath string) error

// fileTransactionHooks exposes exact transaction windows for adversarial tests.
// Hooks belong to one transaction; no package-global mutation is involved.
type fileTransactionHooks struct {
	BeforeMutation                            fileTransactionHook
	AfterMutation                             fileTransactionHook
	AfterGuardBeforeQuarantine                fileTransactionHook
	AfterQuarantineBeforeValidation           fileTransactionHook
	AfterValidationBeforeRestore              fileTransactionHook
	DuringExclusiveRestore                    fileTransactionHook
	AfterRestoreBeforePostValidation          fileTransactionHook
	DuringQuarantineCleanup                   fileTransactionHook
	AfterQuarantineValidationBeforeRelocation fileTransactionHook
	AfterQuarantineMkdir                      fileTransactionHook
	AfterQuarantineOpen                       fileTransactionHook
}

type fileSnapshot struct {
	existed        bool
	isSymlink      bool
	data           []byte
	linkTarget     string
	perm           os.FileMode
	identity       os.FileInfo
	parentObserved bool
	parentExisted  bool
	parentIdentity os.FileInfo
}

type appliedFileTransactionOp struct {
	op               FileTransactionOp
	before           fileSnapshot
	after            fileSnapshot
	beforeQuarantine *fileQuarantine
	mutationStarted  bool
	mutationApplied  bool
	rollbackHandled  bool
}

// FileTransactionRecoveryError reports a concurrent path preserved during a
// failed mutation or rollback.
type FileTransactionRecoveryError struct {
	OriginalPath string
	RecoveryPath string
	Reattached   bool
	Rollback     bool
	Cause        error
}

func (err *FileTransactionRecoveryError) Error() string {
	if err == nil {
		return ""
	}
	context := "transaction incomplete"
	if err.Rollback {
		context = "rollback incomplete"
	}
	if err.Reattached {
		return fmt.Sprintf("%s for %s: concurrent edit from recovery path %s was safely reattached: %v", context, err.OriginalPath, err.RecoveryPath, err.Cause)
	}
	if err.RecoveryPath != "" {
		return fmt.Sprintf("%s for %s: concurrent edit preserved at recovery path %s: %v", context, err.OriginalPath, err.RecoveryPath, err.Cause)
	}
	return fmt.Sprintf("%s for %s: concurrent destination preserved: %v", context, err.OriginalPath, err.Cause)
}

func (err *FileTransactionRecoveryError) Unwrap() []error {
	if err == nil {
		return nil
	}
	joined := []error{ErrFileTransactionConcurrentEdit}
	if err.Rollback {
		joined = append(joined, ErrFileTransactionRollbackPrecondition)
	}
	if err.Cause != nil {
		joined = append(joined, err.Cause)
	}
	return joined
}

type rollbackPathError struct {
	path string
	err  error
}

func (err rollbackPathError) Error() string { return fmt.Sprintf("%s: %v", err.path, err.err) }
func (err rollbackPathError) Unwrap() error { return err.err }

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
	paths = slices.Compact(paths)
	return fmt.Sprintf("apply file transaction: %v; rollback incomplete for %s: %v", err.OperationErr, strings.Join(paths, ", "), errors.Join(err.RollbackErrs...))
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

// FileTransactionCleanupError means all requested mutations committed, but
// post-commit cleanup such as quarantine removal or identity-handle release
// did not complete.
type FileTransactionCleanupError struct {
	Errors []error
}

func (err *FileTransactionCleanupError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("file transaction committed but post-commit cleanup is incomplete: %v", errors.Join(err.Errors...))
}

func (err *FileTransactionCleanupError) Unwrap() error {
	if err == nil {
		return nil
	}
	return errors.Join(err.Errors...)
}

func preflightFileTransaction(ops []FileTransactionOp, filesystem *transactionFilesystem) error {
	for _, op := range ops {
		guardLease := &transactionIdentityLease{}
		before, err := filesystem.snapshotGuard(op.path, op.kind, guardLease)
		if err == nil {
			err = checkFileTransactionPrecondition(op, before)
		}
		err = errors.Join(err, guardLease.close())
		if err != nil {
			return err
		}
	}
	return nil
}

func applyFileTransactionWithFilesystem(ops []FileTransactionOp, filesystem *transactionFilesystem) (resultErr error) {
	identityLease := &transactionIdentityLease{}
	filesystem.identityLease = identityLease
	defer func() {
		filesystem.identityLease = nil
		closeErr := identityLease.close()
		if closeErr == nil {
			return
		}
		if resultErr == nil {
			resultErr = &FileTransactionCleanupError{Errors: []error{closeErr}}
			return
		}
		var cleanupErr *FileTransactionCleanupError
		if errors.As(resultErr, &cleanupErr) {
			cleanupErr.Errors = append(cleanupErr.Errors, closeErr)
			return
		}
		resultErr = errors.Join(resultErr, closeErr)
	}()
	if err := validateFileTransactionOps(ops); err != nil {
		return err
	}
	if err := preflightFileTransaction(ops, filesystem); err != nil {
		return err
	}
	applied := make([]*appliedFileTransactionOp, 0, len(ops))
	for _, op := range ops {
		guardLease := &transactionIdentityLease{}
		before, err := filesystem.snapshotGuard(op.path, op.kind, guardLease)
		if err != nil {
			return transactionFailure(errors.Join(err, guardLease.close()), applied, filesystem)
		}
		if err := checkFileTransactionPrecondition(op, before); err != nil {
			return transactionFailure(errors.Join(err, guardLease.close()), applied, filesystem)
		}
		identityLease.absorb(guardLease)
		item := &appliedFileTransactionOp{op: op, before: before, after: expectedPostTransactionSnapshot(op)}
		applied = append(applied, item)
		if err := callFileTransactionHook(filesystem.hooks.BeforeMutation, op.path, ""); err != nil {
			return transactionFailure(fmt.Errorf("before mutation %s: %w", op.path, err), applied, filesystem)
		}
		item.mutationStarted = true
		quarantine, moved, err := filesystem.quarantineExpected(op.path, before, false)
		item.beforeQuarantine = quarantine
		if moved {
			item.after = missingSnapshotWithParent(before)
		}
		if err != nil {
			item.rollbackHandled = true
			return transactionFailure(fmt.Errorf("guard mutation %s: %w", op.path, err), applied, filesystem)
		}

		switch op.kind {
		case fileTransactionOpWrite:
			installed, appliedWrite, writeErr := filesystem.writeFileAtomicNoReplace(op.path, op.data, op.perm, before, identityLease)
			item.mutationApplied = appliedWrite
			if appliedWrite {
				item.after = installed
			}
			if writeErr != nil {
				return transactionFailure(fmt.Errorf("write %s: %w", op.path, writeErr), applied, filesystem)
			}
		case fileTransactionOpRemove:
			item.mutationApplied = moved
		}
		if err := callFileTransactionHook(filesystem.hooks.AfterMutation, op.path, recoveryPath(quarantine)); err != nil {
			return transactionFailure(fmt.Errorf("after mutation %s: %w", op.path, err), applied, filesystem)
		}
		actual, err := filesystem.snapshot(op.path, op.kind)
		if err != nil {
			return transactionFailure(fmt.Errorf("validate mutation %s: %w", op.path, err), applied, filesystem)
		}
		if !transactionSnapshotsMatch(actual, item.after, true) {
			concurrent := &FileTransactionRecoveryError{OriginalPath: op.path, RecoveryPath: recoveryPath(quarantine), Cause: errors.New("post-state changed before validation")}
			return transactionFailure(concurrent, applied, filesystem)
		}
		item.after = actual
	}

	cleanupErrs := cleanupAppliedQuarantines(applied, filesystem)
	if len(cleanupErrs) > 0 {
		return &FileTransactionCleanupError{Errors: cleanupErrs}
	}
	return nil
}

func transactionFailure(operationErr error, applied []*appliedFileTransactionOp, filesystem *transactionFilesystem) error {
	if len(applied) == 0 {
		return operationErr
	}
	return &FileTransactionError{OperationErr: operationErr, RollbackErrs: rollbackFileTransaction(applied, filesystem)}
}

func rollbackFileTransaction(applied []*appliedFileTransactionOp, filesystem *transactionFilesystem) []error {
	rollbackErrs := make([]error, 0)
	for index := len(applied) - 1; index >= 0; index-- {
		item := applied[index]
		if item.rollbackHandled {
			continue
		}
		if err := rollbackFileTransactionItem(item, filesystem); err != nil {
			rollbackErrs = append(rollbackErrs, rollbackPathError{path: item.op.path, err: err})
		}
	}
	return rollbackErrs
}

func rollbackFileTransactionItem(item *appliedFileTransactionOp, filesystem *transactionFilesystem) error {
	if !item.mutationStarted || (!item.mutationApplied && item.beforeQuarantine == nil) {
		actual, err := filesystem.snapshot(item.op.path, item.op.kind)
		if err != nil {
			return err
		}
		if !transactionSnapshotsMatch(actual, item.before, true) {
			return rollbackRecoveryError(item, nil, errors.New("path changed before mutation began"))
		}
		return nil
	}

	var currentQuarantine *fileQuarantine
	if item.mutationApplied {
		quarantine, _, err := filesystem.quarantineExpected(item.op.path, item.after, true)
		if err != nil {
			preserveQuarantine(item.beforeQuarantine)
			return errors.Join(err, retainedRollbackQuarantineError(item, item.beforeQuarantine, errors.New("pre-transaction state retained because rollback validation failed")))
		}
		currentQuarantine = quarantine
	}
	if err := callFileTransactionHook(filesystem.hooks.AfterValidationBeforeRestore, item.op.path, firstRecoveryPath(currentQuarantine, item.beforeQuarantine)); err != nil {
		preserveQuarantine(currentQuarantine)
		preserveQuarantine(item.beforeQuarantine)
		return rollbackRecoveryError(item, currentQuarantine, fmt.Errorf("after quarantine validation: %w", err))
	}
	restored, reattached, err := filesystem.reattachQuarantinedSnapshot(
		item.op.path,
		item.before,
		item.after,
		item.beforeQuarantine,
		filesystem.identityLease,
	)
	if err == nil && !reattached {
		restored, err = filesystem.restoreSnapshotExclusive(item.op.path, item.before, item.after, filesystem.identityLease)
	}
	if err != nil {
		preserveQuarantine(currentQuarantine)
		preserveQuarantine(item.beforeQuarantine)
		return rollbackRecoveryError(item, currentQuarantine, fmt.Errorf("restore pre-state: %w", err))
	}
	if err := callFileTransactionHook(filesystem.hooks.AfterRestoreBeforePostValidation, item.op.path, firstRecoveryPath(currentQuarantine, item.beforeQuarantine)); err != nil {
		preserveQuarantine(currentQuarantine)
		preserveQuarantine(item.beforeQuarantine)
		return rollbackRecoveryError(item, currentQuarantine, fmt.Errorf("after restore: %w", err))
	}
	actual, err := filesystem.snapshot(item.op.path, snapshotKind(item.before))
	if err != nil {
		preserveQuarantine(currentQuarantine)
		preserveQuarantine(item.beforeQuarantine)
		return rollbackRecoveryError(item, currentQuarantine, fmt.Errorf("validate restored pre-state: %w", err))
	}
	if !transactionSnapshotsMatch(actual, restored, true) {
		preserveQuarantine(currentQuarantine)
		preserveQuarantine(item.beforeQuarantine)
		return rollbackRecoveryError(item, currentQuarantine, errors.New("restored path changed before post-validation"))
	}

	var cleanupErrs []error
	if currentQuarantine != nil {
		if err := filesystem.cleanupQuarantine(currentQuarantine, item.after, &restored, true); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}
	if item.beforeQuarantine != nil {
		if err := filesystem.cleanupQuarantine(item.beforeQuarantine, item.before, &restored, true); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}
	postCleanup, postCleanupErr := filesystem.snapshot(item.op.path, snapshotKind(item.before))
	if postCleanupErr != nil || !transactionSnapshotsMatch(postCleanup, restored, true) {
		cleanupErrs = append(cleanupErrs, &FileTransactionRecoveryError{
			OriginalPath: item.op.path,
			Rollback:     true,
			Cause:        errors.Join(errors.New("restored path changed during quarantine cleanup"), postCleanupErr),
		})
	}
	return errors.Join(cleanupErrs...)
}

func rollbackRecoveryError(item *appliedFileTransactionOp, current *fileQuarantine, cause error) error {
	var recoveryErrs []error
	if current != nil {
		recoveryErrs = append(recoveryErrs, retainedRollbackQuarantineError(item, current, cause))
	}
	if item.beforeQuarantine != nil && item.beforeQuarantine != current {
		recoveryErrs = append(recoveryErrs, retainedRollbackQuarantineError(item, item.beforeQuarantine, fmt.Errorf("pre-transaction state retained: %w", cause)))
	}
	if len(recoveryErrs) == 0 {
		return &FileTransactionRecoveryError{OriginalPath: item.op.path, Rollback: true, Cause: cause}
	}
	return errors.Join(recoveryErrs...)
}

func retainedRollbackQuarantineError(item *appliedFileTransactionOp, quarantine *fileQuarantine, cause error) error {
	if quarantine == nil {
		return nil
	}
	return &FileTransactionRecoveryError{
		OriginalPath: item.op.path,
		RecoveryPath: quarantine.recoveryPath,
		Rollback:     true,
		Cause:        cause,
	}
}

func cleanupAppliedQuarantines(applied []*appliedFileTransactionOp, filesystem *transactionFilesystem) []error {
	var cleanupErrs []error
	for _, item := range applied {
		if item.beforeQuarantine == nil {
			continue
		}
		if err := filesystem.cleanupQuarantine(item.beforeQuarantine, item.before, &item.after, false); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
		actual, err := filesystem.snapshot(item.op.path, item.op.kind)
		if err != nil || !transactionSnapshotsMatch(actual, item.after, true) {
			cleanupErrs = append(cleanupErrs, &FileTransactionRecoveryError{
				OriginalPath: item.op.path,
				Cause:        errors.Join(errors.New("committed path changed during quarantine cleanup"), err),
			})
		}
	}
	return cleanupErrs
}

func expectedPostTransactionSnapshot(op FileTransactionOp) fileSnapshot {
	if op.kind != fileTransactionOpWrite {
		return fileSnapshot{}
	}
	return fileSnapshot{existed: true, data: slices.Clone(op.data), perm: op.perm.Perm()}
}

func missingSnapshotWithParent(snapshot fileSnapshot) fileSnapshot {
	return fileSnapshot{
		parentObserved: snapshot.parentObserved,
		parentExisted:  snapshot.parentExisted,
		parentIdentity: snapshot.parentIdentity,
	}
}

func snapshotKind(fileSnapshot) fileTransactionOpKind {
	return fileTransactionOpRemove
}

func transactionSnapshotsMatch(actual, expected fileSnapshot, exactIdentity bool) bool {
	if exactIdentity && expected.parentObserved {
		if !actual.parentObserved || actual.parentExisted != expected.parentExisted {
			return false
		}
		if expected.parentExisted && expected.parentIdentity != nil && (actual.parentIdentity == nil || !os.SameFile(actual.parentIdentity, expected.parentIdentity)) {
			return false
		}
	}
	if actual.existed != expected.existed {
		return false
	}
	if !expected.existed {
		return true
	}
	if actual.isSymlink != expected.isSymlink ||
		!transactionModesMatch(actual.perm, expected.perm) || !slices.Equal(actual.data, expected.data) ||
		actual.linkTarget != expected.linkTarget {
		return false
	}
	if exactIdentity && expected.identity != nil && (actual.identity == nil || !os.SameFile(actual.identity, expected.identity)) {
		return false
	}
	return true
}

func transactionSnapshotsMatchIgnoringParent(actual, expected fileSnapshot, exactIdentity bool) bool {
	expected.parentObserved = false
	expected.parentExisted = false
	expected.parentIdentity = nil
	return transactionSnapshotsMatch(actual, expected, exactIdentity)
}

func transactionModesMatch(actual, expected os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return actual.Perm()&0o200 == expected.Perm()&0o200
	}
	return actual.Perm() == expected.Perm()
}

func validateFileTransactionOps(ops []FileTransactionOp) error {
	for _, op := range ops {
		if strings.TrimSpace(op.path) == "" {
			return errors.New("file transaction path is required")
		}
		switch op.kind {
		case fileTransactionOpWrite, fileTransactionOpRemove:
		default:
			return fmt.Errorf("unknown file transaction operation %q for %s", op.kind, op.path)
		}
		switch op.precondition {
		case fileTransactionExpectNone, fileTransactionExpectMissing:
		case fileTransactionExpectSHA256:
			decoded, err := hex.DecodeString(op.expectedSHA256)
			if err != nil || len(decoded) != sha256.Size {
				return fmt.Errorf("invalid expected sha256 for %s", op.path)
			}
		default:
			return fmt.Errorf("unknown file transaction precondition %q for %s", op.precondition, op.path)
		}
	}
	return nil
}

func checkFileTransactionPrecondition(op FileTransactionOp, before fileSnapshot) error {
	switch op.precondition {
	case fileTransactionExpectNone:
		return nil
	case fileTransactionExpectMissing:
		if !before.existed {
			return nil
		}
		return fmt.Errorf("%w: expected %s to be missing", ErrFileTransactionPrecondition, op.path)
	case fileTransactionExpectSHA256:
		if !before.existed || before.isSymlink {
			return fmt.Errorf("%w: expected regular file %s", ErrFileTransactionPrecondition, op.path)
		}
		actual := sha256.Sum256(before.data)
		if hex.EncodeToString(actual[:]) != op.expectedSHA256 {
			return fmt.Errorf("%w: %s changed after planning", ErrFileTransactionPrecondition, op.path)
		}
		return nil
	default:
		return fmt.Errorf("unknown file transaction precondition %q for %s", op.precondition, op.path)
	}
}

func callFileTransactionHook(hook fileTransactionHook, originalPath, recoveryPath string) error {
	if hook == nil {
		return nil
	}
	return hook(originalPath, recoveryPath)
}

func recoveryPath(quarantine *fileQuarantine) string {
	if quarantine == nil {
		return ""
	}
	return quarantine.recoveryPath
}

func firstRecoveryPath(quarantines ...*fileQuarantine) string {
	for _, quarantine := range quarantines {
		if quarantine != nil {
			return quarantine.recoveryPath
		}
	}
	return ""
}

func preserveQuarantine(quarantine *fileQuarantine) {
	if quarantine != nil {
		quarantine.preserve()
	}
}
