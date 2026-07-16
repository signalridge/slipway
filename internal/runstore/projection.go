package runstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/signalridge/slipway/internal/fsutil"
)

type Snapshot struct {
	Revision int             `json:"revision"`
	Data     json.RawMessage `json:"data"`
}

func encodeSnapshot(revision int, value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode run projection: %w", err)
	}
	snapshot, err := json.MarshalIndent(Snapshot{Revision: revision, Data: data}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode projection envelope: %w", err)
	}
	return append(snapshot, '\n'), nil
}

type preparedSnapshot struct {
	transaction  *runTransaction
	destination  string
	previous     leafIdentity
	previousFile *os.File
	temporary    string
	temporaryID  leafIdentity
	quarantine   string
	quarantineID leafIdentity
	renamed      bool
}

func (prepared *preparedSnapshot) discard() {
	if prepared == nil {
		return
	}
	if prepared.previousFile != nil {
		_ = prepared.previousFile.Close()
		prepared.previousFile = nil
	}
	if prepared.renamed || prepared.temporary == "" {
		return
	}
	root := prepared.transaction.run.root
	if err := verifyLeafIdentity(root, prepared.temporary, prepared.temporaryID); err != nil {
		return
	}
	_ = root.Remove(prepared.temporary)
}

func prepareSnapshot(transaction *runTransaction, name string, content []byte) (*preparedSnapshot, error) {
	if len(content) == 0 {
		return nil, transaction.tracker.fail(PhaseProjectionPrepare, false, errors.New("projection content is empty"))
	}
	if err := transaction.validate(PhaseProjectionPrepare, faultProjectionInspect); err != nil {
		return nil, err
	}
	previous, previousFile, err := pinRegularFileOrMissingInRoot(transaction.run.root, name)
	if err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionPrepare, false, fmt.Errorf("inspect and pin run projection: %w", err))
	}
	retainPrevious := false
	defer func() {
		if !retainPrevious && previousFile != nil {
			_ = previousFile.Close()
		}
	}()
	if err := transaction.validate(PhaseProjectionTemp, faultProjectionTemp); err != nil {
		return nil, err
	}
	temporary, file, err := createTemporaryFileInRoot(transaction.run.root, name, 0o600)
	if err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionTemp, false, fmt.Errorf("create run projection temp: %w", err))
	}
	prepared := &preparedSnapshot{
		transaction:  transaction,
		destination:  name,
		previous:     previous,
		previousFile: previousFile,
		temporary:    temporary,
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		_ = file.Close()
		if prepared.temporaryID.info == nil || verifyLeafIdentity(transaction.run.root, temporary, prepared.temporaryID) == nil {
			_ = transaction.run.root.Remove(temporary)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionTemp, false, fmt.Errorf("stat run projection temp: %w", err))
	}
	prepared.temporaryID = leafIdentity{exists: true, info: info}
	if err := verifyOpenedRegularFileInRoot(transaction.run.root, temporary, file); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionTemp, false, fmt.Errorf("verify run projection temp: %w", err))
	}
	written, err := file.Write(content)
	if err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionWrite, false, fmt.Errorf("write run projection temp: %w", err))
	}
	if written != len(content) {
		return nil, transaction.tracker.fail(PhaseProjectionWrite, false, io.ErrShortWrite)
	}
	if err := transaction.run.store.hooks.at(faultProjectionAfterWrite); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionWrite, false, err)
	}
	if err := file.Chmod(0o600); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionWrite, false, fmt.Errorf("secure run projection temp: %w", err))
	}
	if err := verifyOpenedRegularFileInRoot(transaction.run.root, temporary, file); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionWrite, false, fmt.Errorf("verify written run projection temp: %w", err))
	}
	if err := transaction.validate(PhaseProjectionFsync, faultProjectionBeforeSync); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionFsync, false, fmt.Errorf("sync run projection temp: %w", err))
	}
	if err := transaction.run.store.hooks.at(faultProjectionAfterSync); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionFsync, false, err)
	}
	if err := verifyOpenedRegularFileInRoot(transaction.run.root, temporary, file); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionFsync, false, fmt.Errorf("verify synced run projection temp: %w", err))
	}
	if err := transaction.validate(PhaseProjectionFsync, faultValidateRun); err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionFsync, false, fmt.Errorf("close run projection temp: %w", err))
	}
	closed = true
	retainPrevious = true
	return prepared, nil
}

func relocatePreviousProjection(root *os.Root, prepared *preparedSnapshot) error {
	if !prepared.previous.exists {
		return nil
	}
	for attempt := 0; attempt < rootRenameAttempts; attempt++ {
		quarantine, err := randomRunLeaf(".quarantine-", prepared.destination)
		if err != nil {
			return err
		}
		if err := fsutil.RenameNoReplace(root, prepared.destination, quarantine); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return err
		}
		relocated, inspectErr := inspectRegularFileOrMissingInRoot(root, quarantine)
		if inspectErr != nil {
			return fmt.Errorf("relocated projection could not be validated and was preserved at %q: %w", quarantine, inspectErr)
		}
		if identityErr := verifyPinnedLeafIdentity(root, quarantine, prepared.previous, prepared.previousFile); identityErr != nil {
			restoreErr := restoreRelocatedProjection(root, quarantine, prepared.destination, relocated)
			if restoreErr != nil {
				return errors.Join(
					fmt.Errorf("run projection changed during no-replace relocation; competing entry was preserved at %q: %w", quarantine, identityErr),
					restoreErr,
				)
			}
			return fmt.Errorf("run projection changed during no-replace relocation; competing entry was restored: %w", identityErr)
		}
		prepared.quarantine = quarantine
		prepared.quarantineID = relocated
		return nil
	}
	return errors.New("could not allocate projection quarantine name")
}

func restoreRelocatedProjection(root *os.Root, quarantine, destination string, expected leafIdentity) error {
	if !expected.exists {
		return fmt.Errorf("relocated projection %q has no validated identity", quarantine)
	}
	if err := verifyLeafIdentity(root, quarantine, expected); err != nil {
		return fmt.Errorf("relocated projection %q changed before restore: %w", quarantine, err)
	}
	current, err := inspectRegularFileOrMissingInRoot(root, destination)
	if err != nil {
		return err
	}
	if current.exists {
		return fmt.Errorf("projection destination is occupied; relocated entry remains at %q", quarantine)
	}
	if err := fsutil.RenameNoReplace(root, quarantine, destination); err != nil {
		return fmt.Errorf("restore relocated projection without replacement: %w", err)
	}
	if err := verifyLeafIdentity(root, destination, expected); err != nil {
		return fmt.Errorf("verify restored projection: %w", err)
	}
	return nil
}

func restorePreviousProjection(root *os.Root, prepared *preparedSnapshot) error {
	if prepared.quarantine == "" {
		return nil
	}
	if err := restoreRelocatedProjection(root, prepared.quarantine, prepared.destination, prepared.quarantineID); err != nil {
		return err
	}
	prepared.quarantine = ""
	prepared.quarantineID = leafIdentity{}
	return nil
}

func cleanupPreviousProjection(root *os.Root, prepared *preparedSnapshot) error {
	if prepared.quarantine == "" {
		return nil
	}
	if err := verifyPinnedLeafIdentity(root, prepared.quarantine, prepared.quarantineID, prepared.previousFile); err != nil {
		return fmt.Errorf("old projection was preserved at %q after its identity changed: %w", prepared.quarantine, err)
	}
	if err := root.Remove(prepared.quarantine); err != nil {
		return fmt.Errorf("remove old projection %q: %w", prepared.quarantine, err)
	}
	current, err := inspectRegularFileOrMissingInRoot(root, prepared.quarantine)
	if err != nil {
		return fmt.Errorf("inspect removed old projection %q: %w", prepared.quarantine, err)
	}
	if current.exists {
		return fmt.Errorf("old projection path %q reappeared during cleanup", prepared.quarantine)
	}
	prepared.quarantine = ""
	prepared.quarantineID = leafIdentity{}
	return nil
}

func commitSnapshot(transaction *runTransaction, prepared *preparedSnapshot) error {
	if prepared == nil || prepared.transaction != transaction {
		return transaction.tracker.fail(PhaseProjectionRename, false, errors.New("invalid prepared projection"))
	}
	root := transaction.run.root
	if err := verifyPinnedLeafIdentity(root, prepared.destination, prepared.previous, prepared.previousFile); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, false, fmt.Errorf("run projection changed before rename: %w", err))
	}
	if err := verifyLeafIdentity(root, prepared.temporary, prepared.temporaryID); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, false, fmt.Errorf("run projection temp changed before rename: %w", err))
	}
	if err := transaction.validate(PhaseProjectionRename, faultProjectionPreRename); err != nil {
		return err
	}
	if err := relocatePreviousProjection(root, prepared); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, false, fmt.Errorf("relocate old run projection without replacement: %w", err))
	}
	if err := transaction.run.store.hooks.at(faultProjectionRelocated); err != nil {
		restoreErr := restorePreviousProjection(root, prepared)
		return transaction.tracker.fail(
			PhaseProjectionRename,
			false,
			errors.Join(fmt.Errorf("install run projection after relocation: %w", err), restoreErr),
		)
	}
	if err := fsutil.RenameNoReplace(root, prepared.temporary, prepared.destination); err != nil {
		restoreErr := restorePreviousProjection(root, prepared)
		return transaction.tracker.fail(
			PhaseProjectionRename,
			false,
			errors.Join(fmt.Errorf("install run projection without replacement: %w", err), restoreErr),
		)
	}
	prepared.renamed = true
	if err := transaction.run.store.hooks.at(faultProjectionPostRename); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, false, err)
	}
	if err := verifyLeafIdentity(root, prepared.destination, prepared.temporaryID); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, true, fmt.Errorf("run projection changed after rename: %w", err))
	}
	if err := transaction.validate(PhaseProjectionRename, faultValidateRun); err != nil {
		return err
	}
	if err := syncAnchoredDirectory(root, transaction.run.identity, transaction.run.store.hooks, faultProjectionDirSync); err != nil {
		return transaction.tracker.fail(PhaseProjectionDirectorySync, false, fmt.Errorf("sync run projection directory: %w", err))
	}
	if err := transaction.validate(PhaseProjectionDirectorySync, faultValidateRun); err != nil {
		return err
	}
	transaction.tracker.markProjectionCurrent()
	if prepared.quarantine != "" {
		if err := cleanupPreviousProjection(root, prepared); err != nil {
			return transaction.tracker.fail(PhaseProjectionRename, false, err)
		}
		if err := syncAnchoredDirectory(root, transaction.run.identity, transaction.run.store.hooks, faultProjectionDirSync); err != nil {
			return transaction.tracker.fail(PhaseProjectionDirectorySync, false, fmt.Errorf("sync old projection cleanup: %w", err))
		}
		if err := transaction.validate(PhaseProjectionDirectorySync, faultValidateRun); err != nil {
			return err
		}
	}
	return nil
}
