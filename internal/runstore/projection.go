package runstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	transaction *runTransaction
	destination string
	previous    leafIdentity
	temporary   string
	temporaryID leafIdentity
	renamed     bool
}

func (prepared *preparedSnapshot) discard() {
	if prepared == nil || prepared.renamed || prepared.temporary == "" {
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
	previous, err := inspectRegularFileOrMissingInRoot(transaction.run.root, name)
	if err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionPrepare, false, fmt.Errorf("inspect run projection: %w", err))
	}
	if err := transaction.validate(PhaseProjectionTemp, faultProjectionTemp); err != nil {
		return nil, err
	}
	temporary, file, err := createTemporaryFileInRoot(transaction.run.root, name, 0o600)
	if err != nil {
		return nil, transaction.tracker.fail(PhaseProjectionTemp, false, fmt.Errorf("create run projection temp: %w", err))
	}
	prepared := &preparedSnapshot{
		transaction: transaction,
		destination: name,
		previous:    previous,
		temporary:   temporary,
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
	return prepared, nil
}

func commitSnapshot(transaction *runTransaction, prepared *preparedSnapshot) error {
	if prepared == nil || prepared.transaction != transaction {
		return transaction.tracker.fail(PhaseProjectionRename, false, errors.New("invalid prepared projection"))
	}
	root := transaction.run.root
	if err := transaction.validate(PhaseProjectionRename, faultProjectionPreRename); err != nil {
		return err
	}
	if err := verifyLeafIdentity(root, prepared.destination, prepared.previous); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, false, fmt.Errorf("run projection changed before rename: %w", err))
	}
	if err := verifyLeafIdentity(root, prepared.temporary, prepared.temporaryID); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, false, fmt.Errorf("run projection temp changed before rename: %w", err))
	}
	if err := renameInRootWithRetry(root, prepared.temporary, prepared.destination); err != nil {
		return transaction.tracker.fail(PhaseProjectionRename, false, fmt.Errorf("rename run projection: %w", err))
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
	return nil
}
