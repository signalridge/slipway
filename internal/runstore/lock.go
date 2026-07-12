package runstore

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const lockTimeout = 5 * time.Second

type mutationTracker struct {
	wroteJournal      bool
	committed         bool
	projectionPending bool
}

func newMutationTracker() *mutationTracker {
	return &mutationTracker{}
}

func (tracker *mutationTracker) markJournalWrite() {
	if tracker != nil {
		tracker.wroteJournal = true
	}
}

func (tracker *mutationTracker) markJournalCommitted() {
	if tracker != nil {
		tracker.committed = true
		tracker.projectionPending = true
	}
}

func (tracker *mutationTracker) markProjectionCurrent() {
	if tracker != nil {
		tracker.projectionPending = false
	}
}

func (tracker *mutationTracker) fail(phase MutationPhase, detached bool, err error) error {
	if err == nil {
		return nil
	}
	if tracker == nil {
		return err
	}
	return mutationFailure(
		phase,
		tracker.committed,
		tracker.committed && tracker.projectionPending,
		detached,
		tracker.wroteJournal && !tracker.committed,
		err,
	)
}

type runTransaction struct {
	run     *runHandle
	lock    *os.File
	tracker *mutationTracker
}

func (transaction *runTransaction) validate(phase MutationPhase, point faultPoint) error {
	if point != faultValidateRun {
		if err := transaction.run.store.hooks.at(point); err != nil {
			return transaction.tracker.fail(phase, false, err)
		}
	}
	if err := transaction.run.validate(); err != nil {
		return transaction.tracker.fail(phase, transaction.tracker != nil && transaction.tracker.wroteJournal, err)
	}
	if err := verifyOpenedRegularFileInRoot(transaction.run.root, lockFileName, transaction.lock); err != nil {
		return transaction.tracker.fail(phase, transaction.tracker != nil && transaction.tracker.wroteJournal, fmt.Errorf("run lock changed after acquisition: %w", err))
	}
	return nil
}

func withRunLock(run *runHandle, tracker *mutationTracker, callback func(*runTransaction) error) error {
	if err := run.validate(); err != nil {
		return tracker.fail(PhaseNamespaceVerify, false, err)
	}
	lockIdentity, err := inspectRegularFileOrMissingInRoot(run.root, lockFileName)
	if err != nil {
		return tracker.fail(PhaseLockOpen, false, fmt.Errorf("inspect run lock: %w", err))
	}
	if !lockIdentity.exists {
		if err := run.store.hooks.at(faultCreateLock); err != nil {
			return tracker.fail(PhaseLockOpen, false, err)
		}
	}
	anchor, created, err := openRegularFileInRoot(run.root, lockFileName, os.O_RDWR, 0o600, true)
	if err != nil {
		return tracker.fail(PhaseLockOpen, false, fmt.Errorf("open run lock: %w", err))
	}
	defer anchor.Close()
	if err := run.store.hooks.at(faultLockOpened); err != nil {
		return tracker.fail(PhaseLockOpen, false, err)
	}
	if err := anchor.Chmod(0o600); err != nil {
		return tracker.fail(PhaseLockOpen, false, fmt.Errorf("secure run lock: %w", err))
	}
	if created {
		if err := run.store.hooks.at(faultLockBeforeSync); err != nil {
			return tracker.fail(PhaseLockSync, false, err)
		}
		if err := anchor.Sync(); err != nil {
			return tracker.fail(PhaseLockSync, false, fmt.Errorf("sync new run lock: %w", err))
		}
		if err := verifyOpenedRegularFileInRoot(run.root, lockFileName, anchor); err != nil {
			return tracker.fail(PhaseLockVerify, false, fmt.Errorf("verify new run lock: %w", err))
		}
		if err := run.validate(); err != nil {
			return tracker.fail(PhaseNamespaceVerify, false, err)
		}
		if err := syncAnchoredDirectory(run.root, run.identity, run.store.hooks, faultSyncRunDirectory); err != nil {
			return tracker.fail(PhaseDirectorySync, false, fmt.Errorf("sync new run lock entry: %w", err))
		}
	}

	deadline := time.Now().Add(lockTimeout)
	for {
		locked, lockErr := tryLockFile(anchor)
		if lockErr != nil {
			return tracker.fail(PhaseLockOpen, false, fmt.Errorf("acquire run lock: %w", lockErr))
		}
		if locked {
			break
		}
		if !time.Now().Before(deadline) {
			return tracker.fail(PhaseLockOpen, false, fmt.Errorf("acquire run lock: timed out after %s", lockTimeout))
		}
		time.Sleep(25 * time.Millisecond)
	}

	transaction := &runTransaction{run: run, lock: anchor, tracker: tracker}
	if err := run.store.hooks.at(faultLockAcquired); err != nil {
		_ = unlockFile(anchor)
		return tracker.fail(PhaseLockVerify, false, err)
	}
	if err := transaction.validate(PhaseLockVerify, faultLockBeforeCallback); err != nil {
		_ = unlockFile(anchor)
		return err
	}

	callbackErr := callback(transaction)
	validationErr := transaction.validate(PhaseLockVerify, faultLockAfterCallback)
	unlockErr := unlockFile(anchor)
	if callbackErr != nil {
		if validationErr != nil || unlockErr != nil {
			return tracker.fail(PhaseLockVerify, tracker != nil && tracker.wroteJournal, errors.Join(callbackErr, validationErr, unlockErr))
		}
		return callbackErr
	}
	if validationErr != nil {
		return validationErr
	}
	if unlockErr != nil {
		return tracker.fail(PhaseLockVerify, tracker != nil && tracker.wroteJournal, fmt.Errorf("release run lock: %w", unlockErr))
	}
	return nil
}
