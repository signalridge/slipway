package runstore

// faultPoint names deterministic storage boundaries used by package tests.
// Production stores have a nil hook, so every point is a no-op.
type faultPoint string

const (
	faultCreateSlipway        faultPoint = "create_slipway"
	faultCreateRuns           faultPoint = "create_runs"
	faultCreateRun            faultPoint = "create_run"
	faultCreateLock           faultPoint = "create_lock"
	faultCreateJournal        faultPoint = "create_journal"
	faultValidateCommon       faultPoint = "validate_common"
	faultValidateSlipway      faultPoint = "validate_slipway"
	faultValidateRuns         faultPoint = "validate_runs"
	faultValidateRun          faultPoint = "validate_run"
	faultSyncSlipwayDirectory faultPoint = "sync_slipway_directory" // #nosec G101 -- deterministic fault-injection label, not credential material.
	faultSyncCommonDirectory  faultPoint = "sync_common_directory"
	faultSyncRunsDirectory    faultPoint = "sync_runs_directory"
	faultSyncRunDirectory     faultPoint = "sync_run_directory"
	faultSyncRunsParent       faultPoint = "sync_runs_parent"
	faultLockOpened           faultPoint = "lock_opened"
	faultLockBeforeSync       faultPoint = "lock_before_sync"
	faultLockAcquired         faultPoint = "lock_acquired"
	faultLockBeforeCallback   faultPoint = "lock_before_callback"
	faultLockAfterCallback    faultPoint = "lock_after_callback"
	faultJournalOpened        faultPoint = "journal_opened"
	faultTailScanBeforeRepair faultPoint = "tail_scan_before_repair"
	faultTailAfterRepair      faultPoint = "tail_after_repair"
	faultJournalAfterWrite    faultPoint = "journal_after_write"
	faultJournalBeforeSync    faultPoint = "journal_before_sync"
	faultJournalAfterSync     faultPoint = "journal_after_sync"
	faultProjectionInspect    faultPoint = "projection_inspect"
	faultProjectionTemp       faultPoint = "projection_temp"
	faultProjectionAfterWrite faultPoint = "projection_after_write"
	faultProjectionBeforeSync faultPoint = "projection_before_sync"
	faultProjectionAfterSync  faultPoint = "projection_after_sync"
	faultProjectionPreRename  faultPoint = "projection_pre_rename"
	faultProjectionPostRename faultPoint = "projection_post_rename"
	faultProjectionDirSync    faultPoint = "projection_dir_sync"
)

type storeHooks struct {
	fault func(faultPoint) error
}

func (hooks storeHooks) at(point faultPoint) error {
	if hooks.fault == nil {
		return nil
	}
	return hooks.fault(point)
}
