package runstore

import (
	"errors"
	"fmt"
)

// MutationPhase identifies the durable boundary at which a run mutation failed.
// Values are stable because callers expose them in machine-readable errors.
type MutationPhase string

const (
	PhaseValidation              MutationPhase = "validation"
	PhaseDirectoryCreate         MutationPhase = "directory_create"
	PhaseDirectorySync           MutationPhase = "directory_sync"
	PhaseLockOpen                MutationPhase = "lock_open"
	PhaseLockSync                MutationPhase = "lock_sync"
	PhaseLockVerify              MutationPhase = "lock_verify"
	PhaseReplay                  MutationPhase = "replay"
	PhaseCallback                MutationPhase = "callback"
	PhaseProjectionEncode        MutationPhase = "projection_encode"
	PhaseProjectionPrepare       MutationPhase = "projection_prepare"
	PhaseProjectionTemp          MutationPhase = "projection_temp"
	PhaseProjectionWrite         MutationPhase = "projection_write"
	PhaseProjectionFsync         MutationPhase = "projection_fsync"
	PhaseProjectionDirectorySync MutationPhase = "projection_directory_sync"
	PhaseJournalOpen             MutationPhase = "journal_open"
	PhaseJournalWrite            MutationPhase = "journal_write"
	PhaseJournalSync             MutationPhase = "journal_sync"
	PhaseJournalVerify           MutationPhase = "journal_verify"
	PhaseProjectionRename        MutationPhase = "projection_rename"
	PhaseProjectionSync          MutationPhase = "projection_sync"
	PhaseNamespaceVerify         MutationPhase = "namespace_verify"
)

// MutationError reports what is known about a failed mutation. Committed means
// journal bytes were written and fsynced. ProjectionStale means journal replay is
// authoritative and run.json may not describe the committed revision.
// NamespaceDetached means a write reached an opened inode whose path membership
// could no longer be proved. Ambiguous means an inode was written but durable
// commit could not be established. Callers must inspect/recover and must not
// retry either state blindly.
type MutationError struct {
	Phase             MutationPhase
	Committed         bool
	ProjectionStale   bool
	NamespaceDetached bool
	Ambiguous         bool
	Err               error
}

func (err *MutationError) Error() string {
	if err == nil {
		return ""
	}
	cause := "unknown mutation failure"
	if err.Err != nil {
		cause = err.Err.Error()
	}
	switch {
	case err.NamespaceDetached:
		return fmt.Sprintf("mutation outcome is ambiguous after namespace detachment during %s; inspect the authoritative journal before recovery and do not retry blindly: %s", err.Phase, cause)
	case err.Ambiguous:
		return fmt.Sprintf("mutation outcome is ambiguous after an inode write during %s; inspect journal.jsonl before recovery and do not retry blindly: %s", err.Phase, cause)
	case err.Committed && err.ProjectionStale:
		return fmt.Sprintf("mutation committed, projection stale during %s; replay journal.jsonl before recovery and do not retry blindly: %s", err.Phase, cause)
	case err.Committed:
		return fmt.Sprintf("mutation committed but post-commit verification failed during %s; inspect journal.jsonl before recovery and do not retry blindly: %s", err.Phase, cause)
	default:
		return fmt.Sprintf("mutation not committed during %s: %s", err.Phase, cause)
	}
}

func (err *MutationError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func (err *MutationError) StorageMutationPhase() string {
	if err == nil {
		return ""
	}
	return string(err.Phase)
}

func (err *MutationError) StorageMutationCommitted() bool {
	return err != nil && err.Committed
}

func (err *MutationError) StorageMutationProjectionStale() bool {
	return err != nil && err.ProjectionStale
}

func (err *MutationError) StorageMutationNamespaceDetached() bool {
	return err != nil && err.NamespaceDetached
}

func (err *MutationError) StorageMutationAmbiguous() bool {
	return err != nil && err.Ambiguous
}

func mutationFailure(phase MutationPhase, committed, projectionStale, detached, ambiguous bool, err error) error {
	if err == nil {
		return nil
	}
	var mutationErr *MutationError
	if errors.As(err, &mutationErr) {
		return err
	}
	return &MutationError{
		Phase:             phase,
		Committed:         committed,
		ProjectionStale:   projectionStale,
		NamespaceDetached: detached,
		Ambiguous:         ambiguous,
		Err:               err,
	}
}
