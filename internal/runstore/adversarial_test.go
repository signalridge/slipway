package runstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errInjectedRunstoreFault = errors.New("injected runstore fault")

func TestStoreUsesOnlyAuthoritativeJournalFilename(t *testing.T) {
	store, paths := createAdversarialRun(t, "00000000-0000-4000-8000-000000000301")
	_, err := os.Stat(paths.JournalFile)
	require.NoError(t, err)
	legacy := filepath.Join(paths.Directory, "events.jsonl")
	_, err = os.Stat(legacy)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, os.WriteFile(legacy, []byte("owned by a legacy implementation\n"), 0o600))

	events := collectStoreEvents(t, store, "00000000-0000-4000-8000-000000000301")
	require.Len(t, events, 1)
	assert.Equal(t, "created", events[0].Type)
	content, err := os.ReadFile(legacy)
	require.NoError(t, err)
	assert.Equal(t, "owned by a legacy implementation\n", string(content))
}

func TestStoreRejectsNamespaceLayerReplacementBeforeMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming opened directory handles is not portable on Windows")
	}
	tests := []struct {
		name   string
		point  faultPoint
		target func(*Store, Paths) string
	}{
		{name: "common directory", point: faultValidateCommon, target: func(store *Store, _ Paths) string { return store.CommonDir() }},
		{name: "slipway directory", point: faultValidateSlipway, target: func(store *Store, _ Paths) string { return filepath.Join(store.CommonDir(), "slipway") }},
		{name: "runs directory", point: faultValidateRuns, target: func(store *Store, _ Paths) string { return filepath.Join(store.CommonDir(), "slipway", "runs") }},
		{name: "run directory", point: faultValidateRun, target: func(_ *Store, paths Paths) string { return paths.Directory }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, paths := createAdversarialRun(t, "00000000-0000-4000-8000-000000000302")
			target := test.target(store, paths)
			detached := target + ".detached"
			installOneShotHook(store, test.point, func() error {
				require.NoError(t, os.Rename(target, detached))
				require.NoError(t, os.Mkdir(target, 0o700))
				return nil
			})

			callbackCalled := false
			err := updateAdversarialRun(store, "00000000-0000-4000-8000-000000000302", &callbackCalled)
			mutationErr := requireMutationError(t, err)
			assert.False(t, mutationErr.Committed)
			assert.False(t, mutationErr.Ambiguous)
			assert.False(t, callbackCalled)
			assert.Empty(t, mustReadDirNames(t, target))
			assert.NotEmpty(t, mustReadDirNames(t, detached))
		})
	}
}

func TestStoreReportsNamespaceDetachmentAfterJournalWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming opened directory handles is not portable on Windows")
	}
	tests := []struct {
		name   string
		target func(*Store, Paths) string
	}{
		{name: "common directory", target: func(store *Store, _ Paths) string { return store.CommonDir() }},
		{name: "runs directory", target: func(store *Store, _ Paths) string { return filepath.Join(store.CommonDir(), "slipway", "runs") }},
		{name: "run directory", target: func(_ *Store, paths Paths) string { return paths.Directory }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runID := "00000000-0000-4000-8000-000000000309"
			store, paths := createAdversarialRun(t, runID)
			target := test.target(store, paths)
			installOneShotHook(store, faultJournalAfterWrite, func() error {
				require.NoError(t, os.Rename(target, target+".detached"))
				require.NoError(t, os.Mkdir(target, 0o700))
				return nil
			})

			err := updateAdversarialRun(store, runID, nil)
			mutationErr := requireMutationError(t, err)
			assert.False(t, mutationErr.Committed)
			assert.True(t, mutationErr.Ambiguous)
			assert.True(t, mutationErr.NamespaceDetached)
			assert.Contains(t, mutationErr.Error(), "namespace detachment")
		})
	}
}

func TestStoreRejectsRollbackToOlderRunsDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming opened directory handles is not portable on Windows")
	}
	repository := createRepository(t)
	slipway := filepath.Join(repository, ".git", "slipway")
	older := filepath.Join(slipway, "runs-older")
	require.NoError(t, os.MkdirAll(older, 0o700))
	store, err := Open(repository)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	event, err := NewEvent("created", map[string]int{"count": 1})
	require.NoError(t, err)
	runID := "00000000-0000-4000-8000-000000000303"
	require.NoError(t, store.Create(runID, event, map[string]int{"count": 1}))
	current := filepath.Join(slipway, "runs")
	installOneShotHook(store, faultValidateRuns, func() error {
		require.NoError(t, os.Rename(current, filepath.Join(slipway, "runs-current")))
		require.NoError(t, os.Rename(older, current))
		return nil
	})

	err = updateAdversarialRun(store, runID, nil)
	mutationErr := requireMutationError(t, err)
	assert.False(t, mutationErr.Committed)
	assert.ErrorContains(t, err, "journal root changed")
	assert.Empty(t, mustReadDirNames(t, current))
}

func TestJournalTailRepairRejectsReplacementAndKeepsReplacementUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an opened journal is not portable on Windows")
	}
	runID := "00000000-0000-4000-8000-000000000304"
	store, paths := createAdversarialRun(t, runID)
	file, err := os.OpenFile(paths.JournalFile, os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)
	_, err = file.WriteString(`{"sequence":2,"type":"interrupted"`)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	original, err := os.ReadFile(paths.JournalFile)
	require.NoError(t, err)
	replacement := []byte("replacement must remain byte-for-byte unchanged")
	detached := paths.JournalFile + ".scanned"
	installOneShotHook(store, faultTailScanBeforeRepair, func() error {
		require.NoError(t, os.Rename(paths.JournalFile, detached))
		require.NoError(t, os.WriteFile(paths.JournalFile, replacement, 0o600))
		return nil
	})

	err = store.Visit(runID, func(Event) error { return nil })
	require.Error(t, err)
	assert.ErrorContains(t, err, "journal leaf")
	actualReplacement, readErr := os.ReadFile(paths.JournalFile)
	require.NoError(t, readErr)
	assert.Equal(t, replacement, actualReplacement)
	actualDetached, readErr := os.ReadFile(detached)
	require.NoError(t, readErr)
	assert.Equal(t, original, actualDetached)
}

func TestJournalReplacementAfterWriteIsAmbiguousAndNotCommitted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an opened journal is not portable on Windows")
	}
	runID := "00000000-0000-4000-8000-000000000305"
	store, paths := createAdversarialRun(t, runID)
	initial, err := os.ReadFile(paths.JournalFile)
	require.NoError(t, err)
	detached := paths.JournalFile + ".detached"
	installOneShotHook(store, faultJournalAfterWrite, func() error {
		require.NoError(t, os.Rename(paths.JournalFile, detached))
		require.NoError(t, os.WriteFile(paths.JournalFile, initial, 0o600))
		return nil
	})

	err = updateAdversarialRun(store, runID, nil)
	mutationErr := requireMutationError(t, err)
	assert.False(t, mutationErr.Committed)
	assert.True(t, mutationErr.Ambiguous)
	assert.True(t, mutationErr.NamespaceDetached)
	assert.Contains(t, mutationErr.Error(), "do not retry blindly")
	events := collectStoreEvents(t, store, runID)
	require.Len(t, events, 1)
	assert.Equal(t, "created", events[0].Type)
}

func TestJournalReplacementAfterFsyncReportsCommittedDetached(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an opened journal is not portable on Windows")
	}
	runID := "00000000-0000-4000-8000-000000000344"
	store, paths := createAdversarialRun(t, runID)
	initial, err := os.ReadFile(paths.JournalFile)
	require.NoError(t, err)
	installOneShotHook(store, faultJournalAfterSync, func() error {
		require.NoError(t, os.Rename(paths.JournalFile, paths.JournalFile+".synced"))
		require.NoError(t, os.WriteFile(paths.JournalFile, initial, 0o600))
		return nil
	})

	err = updateAdversarialRun(store, runID, nil)
	mutationErr := requireMutationError(t, err)
	assert.True(t, mutationErr.Committed)
	assert.True(t, mutationErr.ProjectionStale)
	assert.True(t, mutationErr.NamespaceDetached)
	assert.Contains(t, mutationErr.Error(), "namespace detachment")
	events := collectStoreEvents(t, store, runID)
	require.Len(t, events, 1)
}

func TestLockReplacementBeforeCallbackPreventsMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an opened lock file is not portable on Windows")
	}
	runID := "00000000-0000-4000-8000-000000000306"
	store, paths := createAdversarialRun(t, runID)
	installOneShotHook(store, faultLockBeforeCallback, func() error {
		replaceLeaf(t, paths.LockFile, []byte("replacement lock"))
		return nil
	})
	called := false
	err := updateAdversarialRun(store, runID, &called)
	mutationErr := requireMutationError(t, err)
	assert.False(t, mutationErr.Committed)
	assert.False(t, mutationErr.Ambiguous)
	assert.False(t, called)
	assert.ErrorContains(t, err, "run lock changed")
}

func TestRunWriterGuardSurvivesRunLockReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("run.lock replacement while opened is not portable on Windows")
	}
	runID := "00000000-0000-4000-8000-000000000307"
	storeA, paths := createAdversarialRun(t, runID)
	storeB, err := Open(storeA.RepositoryRoot())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storeB.Close()) })
	runA, err := storeA.openRunRoot(runID)
	require.NoError(t, err)
	defer runA.Close()
	runB, err := storeB.openRunRoot(runID)
	require.NoError(t, err)
	defer runB.Close()

	aEntered := make(chan struct{})
	releaseA := make(chan struct{})
	aResult := make(chan error, 1)
	go func() {
		aResult <- withRunLock(runA, nil, func(*runTransaction) error {
			replaceLeaf(t, paths.LockFile, []byte("replacement lock"))
			close(aEntered)
			<-releaseA
			return nil
		})
	}()
	<-aEntered

	bWaiting := make(chan struct{})
	var waitOnce sync.Once
	runB.writerWait = func() { waitOnce.Do(func() { close(bWaiting) }) }
	bEntered := make(chan struct{})
	bResult := make(chan error, 1)
	go func() {
		bResult <- withRunLock(runB, nil, func(*runTransaction) error {
			close(bEntered)
			return nil
		})
	}()
	<-bWaiting
	select {
	case <-bEntered:
		t.Fatal("replacement run.lock bypassed the run writer guard")
	default:
	}
	close(releaseA)
	require.Error(t, <-aResult)
	require.NoError(t, <-bResult)
	select {
	case <-bEntered:
	default:
		t.Fatal("second writer never entered after the first released the writer guard")
	}
}

func TestLockReplacementAfterJournalSyncReportsCommittedStale(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an opened lock file is not portable on Windows")
	}
	runID := "00000000-0000-4000-8000-000000000307"
	store, paths := createAdversarialRun(t, runID)
	installOneShotHook(store, faultJournalAfterSync, func() error {
		replaceLeaf(t, paths.LockFile, []byte("replacement lock"))
		return nil
	})

	err := updateAdversarialRun(store, runID, nil)
	mutationErr := requireMutationError(t, err)
	assert.True(t, mutationErr.Committed)
	assert.True(t, mutationErr.ProjectionStale)
	assert.False(t, mutationErr.NamespaceDetached, "a replaceable lock is not namespace identity")
	assert.False(t, mutationErr.Ambiguous, "a durably committed journal write stays determinate")
	events := collectStoreEvents(t, store, runID)
	require.Len(t, events, 2)
	assert.Equal(t, "updated", events[1].Type)
}

func TestProjectionReplacementAfterJournalCommitReportsStale(t *testing.T) {
	runID := "00000000-0000-4000-8000-000000000308"
	store, paths := createAdversarialRun(t, runID)
	installOneShotHook(store, faultProjectionPreRename, func() error {
		replaceLeaf(t, paths.RunFile, []byte("replacement projection"))
		return nil
	})

	err := updateAdversarialRun(store, runID, nil)
	mutationErr := requireMutationError(t, err)
	assert.True(t, mutationErr.Committed)
	assert.True(t, mutationErr.ProjectionStale)
	assert.False(t, mutationErr.NamespaceDetached)
	assert.Equal(t, PhaseProjectionRename, mutationErr.Phase)
	replacement, readErr := os.ReadFile(paths.RunFile)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("replacement projection"), replacement, "the competing projection must survive the failed commit")
	assertReplayContainsOneUpdate(t, store, runID)
}

func TestProjectionReplacementAfterInstallIsDetectedAndPreserved(t *testing.T) {
	runID := "00000000-0000-4000-8000-000000000345"
	store, paths := createAdversarialRun(t, runID)
	competitor := []byte("competing projection after install")
	installed := paths.RunFile + ".installed"
	installOneShotHook(store, faultProjectionPostRename, func() error {
		require.NoError(t, os.Rename(paths.RunFile, installed))
		require.NoError(t, os.WriteFile(paths.RunFile, competitor, 0o600))
		return nil
	})

	err := updateAdversarialRun(store, runID, nil)
	mutationErr := requireMutationError(t, err)
	assert.True(t, mutationErr.Committed)
	assert.True(t, mutationErr.ProjectionStale)
	assert.False(t, mutationErr.Ambiguous)
	assert.Equal(t, PhaseProjectionRename, mutationErr.Phase)
	actual, readErr := os.ReadFile(paths.RunFile)
	require.NoError(t, readErr)
	assert.Equal(t, competitor, actual, "cleanup must not unlink the competing projection")
	_, statErr := os.Stat(installed)
	require.NoError(t, statErr, "the displaced installed projection must be preserved for diagnosis")
	assertReplayContainsOneUpdate(t, store, runID)
}

func TestProjectionInstallFaultAfterRelocationRestoresPreviousProjection(t *testing.T) {
	runID := "00000000-0000-4000-8000-000000000346"
	store, paths := createAdversarialRun(t, runID)
	before, err := os.Stat(paths.RunFile)
	require.NoError(t, err)
	beforeContent, err := os.ReadFile(paths.RunFile)
	require.NoError(t, err)
	installOneShotHook(store, faultProjectionRelocated, func() error { return errInjectedRunstoreFault })

	err = updateAdversarialRun(store, runID, nil)
	mutationErr := requireMutationError(t, err)
	assert.True(t, mutationErr.Committed)
	assert.True(t, mutationErr.ProjectionStale)
	assert.Equal(t, PhaseProjectionRename, mutationErr.Phase)
	after, statErr := os.Stat(paths.RunFile)
	require.NoError(t, statErr)
	assert.True(t, os.SameFile(before, after), "the exact previous projection inode must be restored")
	afterContent, readErr := os.ReadFile(paths.RunFile)
	require.NoError(t, readErr)
	assert.Equal(t, beforeContent, afterContent)
	for _, name := range mustReadDirNames(t, paths.Directory) {
		assert.False(t, strings.HasPrefix(name, ".quarantine-run.json-"), name)
	}
	assertReplayContainsOneUpdate(t, store, runID)
}

func TestProjectionFailureAfterJournalCommitReplaysDeterministically(t *testing.T) {
	tests := []struct {
		name  string
		point faultPoint
		phase MutationPhase
	}{
		{name: "rename", point: faultProjectionPreRename, phase: PhaseProjectionRename},
		{name: "post rename verification", point: faultProjectionPostRename, phase: PhaseProjectionRename},
		{name: "directory sync", point: faultProjectionDirSync, phase: PhaseProjectionDirectorySync},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runID := "00000000-0000-4000-8000-00000000031" + string(rune('0'+index))
			store, _ := createAdversarialRun(t, runID)
			installOneShotHook(store, test.point, func() error { return errInjectedRunstoreFault })

			err := updateAdversarialRun(store, runID, nil)
			mutationErr := requireMutationError(t, err)
			assert.True(t, mutationErr.Committed)
			assert.True(t, mutationErr.ProjectionStale)
			assert.False(t, mutationErr.Ambiguous)
			assert.Equal(t, test.phase, mutationErr.Phase)
			assert.Contains(t, mutationErr.Error(), "mutation committed, projection stale")
			firstReplay := collectStoreEvents(t, store, runID)
			require.Len(t, firstReplay, 2)
			assert.Equal(t, "updated", firstReplay[1].Type)
			secondReplay := collectStoreEvents(t, store, runID)
			assert.Equal(t, firstReplay, secondReplay, "repeated recovery replay must be deterministic")
		})
	}
}

func TestProjectionPreparationFaultsArePreCommit(t *testing.T) {
	tests := []struct {
		name  string
		point faultPoint
		phase MutationPhase
	}{
		{name: "temporary create", point: faultProjectionTemp, phase: PhaseProjectionTemp},
		{name: "temporary write", point: faultProjectionAfterWrite, phase: PhaseProjectionWrite},
		{name: "temporary fsync", point: faultProjectionBeforeSync, phase: PhaseProjectionFsync},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runID := "00000000-0000-4000-8000-00000000032" + string(rune('0'+index))
			store, paths := createAdversarialRun(t, runID)
			installOneShotHook(store, test.point, func() error { return errInjectedRunstoreFault })

			err := updateAdversarialRun(store, runID, nil)
			mutationErr := requireMutationError(t, err)
			assert.False(t, mutationErr.Committed)
			assert.False(t, mutationErr.ProjectionStale)
			assert.False(t, mutationErr.Ambiguous)
			assert.Equal(t, test.phase, mutationErr.Phase)
			events := collectStoreEvents(t, store, runID)
			require.Len(t, events, 1)
			for _, name := range mustReadDirNames(t, paths.Directory) {
				assert.False(t, strings.HasPrefix(name, ".tmp-run.json-"), name)
			}
		})
	}
}

func TestJournalWriteAndSyncFaultsAreAmbiguousWithoutCommittedClaim(t *testing.T) {
	tests := []struct {
		name  string
		point faultPoint
		phase MutationPhase
	}{
		{name: "after append", point: faultJournalAfterWrite, phase: PhaseJournalWrite},
		{name: "before fsync", point: faultJournalBeforeSync, phase: PhaseJournalSync},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runID := "00000000-0000-4000-8000-00000000033" + string(rune('0'+index))
			store, _ := createAdversarialRun(t, runID)
			installOneShotHook(store, test.point, func() error { return errInjectedRunstoreFault })

			err := updateAdversarialRun(store, runID, nil)
			mutationErr := requireMutationError(t, err)
			assert.False(t, mutationErr.Committed)
			assert.True(t, mutationErr.Ambiguous)
			assert.False(t, mutationErr.ProjectionStale)
			assert.Equal(t, test.phase, mutationErr.Phase)
			assert.Contains(t, mutationErr.Error(), "do not retry blindly")
			assertReplayContainsOneUpdate(t, store, runID)
		})
	}
}

func TestFirstCreationSyncFaultsHaveTruthfulCommitClassification(t *testing.T) {
	for _, test := range []struct {
		name  string
		point faultPoint
	}{
		{name: "run directory create", point: faultCreateRun},
		{name: "lock file create", point: faultCreateLock},
		{name: "journal file create", point: faultCreateJournal},
		{name: "projection temp create", point: faultProjectionTemp},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := openAdversarialStore(t)
			installOneShotHook(store, test.point, func() error { return errInjectedRunstoreFault })
			err := createOneRun(store, "00000000-0000-4000-8000-000000000349")
			mutationErr := requireMutationError(t, err)
			assert.False(t, mutationErr.Committed)
			assert.False(t, mutationErr.Ambiguous)
		})
	}
	t.Run("lock file fsync", func(t *testing.T) {
		store := openAdversarialStore(t)
		installOneShotHook(store, faultLockBeforeSync, func() error { return errInjectedRunstoreFault })
		err := createOneRun(store, "00000000-0000-4000-8000-000000000340")
		mutationErr := requireMutationError(t, err)
		assert.False(t, mutationErr.Committed)
		assert.False(t, mutationErr.Ambiguous)
	})
	t.Run("journal file fsync", func(t *testing.T) {
		store := openAdversarialStore(t)
		installOneShotHook(store, faultJournalBeforeSync, func() error { return errInjectedRunstoreFault })
		err := createOneRun(store, "00000000-0000-4000-8000-000000000341")
		mutationErr := requireMutationError(t, err)
		assert.False(t, mutationErr.Committed)
		assert.True(t, mutationErr.Ambiguous)
	})
	t.Run("run directory after journal", func(t *testing.T) {
		store := openAdversarialStore(t)
		var count atomic.Int32
		store.hooks = storeHooks{fault: func(point faultPoint) error {
			if point == faultSyncRunDirectory && count.Add(1) == 3 {
				return errInjectedRunstoreFault
			}
			return nil
		}}
		err := createOneRun(store, "00000000-0000-4000-8000-000000000342")
		mutationErr := requireMutationError(t, err)
		assert.False(t, mutationErr.Committed)
		assert.True(t, mutationErr.Ambiguous)
		assert.False(t, mutationErr.ProjectionStale)
	})
	t.Run("runs parent after projection", func(t *testing.T) {
		store := openAdversarialStore(t)
		var count atomic.Int32
		store.hooks = storeHooks{fault: func(point faultPoint) error {
			if point == faultSyncRunsParent && count.Add(1) == 2 {
				return errInjectedRunstoreFault
			}
			return nil
		}}
		err := createOneRun(store, "00000000-0000-4000-8000-000000000343")
		mutationErr := requireMutationError(t, err)
		if PlatformDurability().DirectorySync {
			assert.True(t, mutationErr.Committed)
			assert.False(t, mutationErr.Ambiguous)
		} else {
			assert.False(t, mutationErr.Committed)
			assert.True(t, mutationErr.Ambiguous)
		}
		assert.False(t, mutationErr.ProjectionStale)
	})
}

func TestOpenReportsFirstDirectorySyncFaults(t *testing.T) {
	tests := []struct {
		name  string
		point faultPoint
	}{
		{name: "slipway create", point: faultCreateSlipway},
		{name: "runs create", point: faultCreateRuns},
		{name: "slipway directory", point: faultSyncSlipwayDirectory},
		{name: "common parent", point: faultSyncCommonDirectory},
		{name: "runs directory", point: faultSyncRunsDirectory},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := createRepository(t)
			store, err := openWithHooks(repository, storeHooks{fault: func(point faultPoint) error {
				if point == test.point {
					return errInjectedRunstoreFault
				}
				return nil
			}})
			if store != nil {
				require.NoError(t, store.Close())
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, errInjectedRunstoreFault)
		})
	}
}

func TestOpenResyncsExistingNamespaceDurability(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	require.NoError(t, store.Close())

	store, err = openWithHooks(repository, storeHooks{fault: func(point faultPoint) error {
		if point == faultSyncCommonDirectory {
			return errInjectedRunstoreFault
		}
		return nil
	}})
	if store != nil {
		require.NoError(t, store.Close())
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, errInjectedRunstoreFault)
}

func TestMutationErrorSupportsErrorsIsAndAs(t *testing.T) {
	err := mutationFailure(PhaseProjectionSync, true, true, false, false, errInjectedRunstoreFault)
	assert.ErrorIs(t, err, errInjectedRunstoreFault)
	var mutationErr *MutationError
	require.ErrorAs(t, err, &mutationErr)
	assert.True(t, mutationErr.Committed)
	assert.True(t, mutationErr.ProjectionStale)
	assert.Equal(t, PhaseProjectionSync, mutationErr.Phase)
}

func TestPlatformDurabilityCapabilityIsStable(t *testing.T) {
	capability := PlatformDurability()
	assert.True(t, capability.FileSync)
	if runtime.GOOS == "windows" {
		assert.Equal(t, DurabilityLevelFileOnly, capability.Level)
		assert.False(t, capability.DirectorySync)
		assert.Equal(t, DurabilityLimitDirectorySync, capability.Limitation)
		return
	}
	assert.Equal(t, DurabilityLevelFileAndDirectory, capability.Level)
	assert.True(t, capability.DirectorySync)
	assert.Empty(t, capability.Limitation)
}

func TestOpenedDirectoryIdentityRejectsAnotherDirectory(t *testing.T) {
	firstPath := t.TempDir()
	secondPath := t.TempDir()
	first, firstIdentity, err := openAbsoluteDirectoryRoot(firstPath)
	require.NoError(t, err)
	defer first.Close()
	second, secondIdentity, err := openAbsoluteDirectoryRoot(secondPath)
	require.NoError(t, err)
	defer second.Close()
	assert.NoError(t, validateOpenedDirectoryRoot(first, firstIdentity))
	assert.Error(t, validateOpenedDirectoryRoot(first, secondIdentity))
}

func createAdversarialRun(t *testing.T, runID string) (*Store, Paths) {
	t.Helper()
	store := openAdversarialStore(t)
	require.NoError(t, createOneRun(store, runID))
	paths, err := pathsFor(store.CommonDir(), runID)
	require.NoError(t, err)
	return store, paths
}

func openAdversarialStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(createRepository(t))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

func createOneRun(store *Store, runID string) error {
	event, err := NewEvent("created", map[string]int{"count": 1})
	if err != nil {
		return err
	}
	return store.Create(runID, event, map[string]int{"count": 1})
}

func updateAdversarialRun(store *Store, runID string, callbackCalled *bool) error {
	count := 0
	return store.UpdateStream(runID, func(Event) error {
		count++
		return nil
	}, func() ([]Event, any, error) {
		if callbackCalled != nil {
			*callbackCalled = true
		}
		event, err := NewEvent("updated", map[string]int{"count": count + 1})
		return []Event{event}, map[string]int{"count": count + 1}, err
	})
}

func installOneShotHook(store *Store, selected faultPoint, callback func() error) {
	var lock sync.Mutex
	fired := false
	store.hooks = storeHooks{fault: func(point faultPoint) error {
		if point != selected {
			return nil
		}
		lock.Lock()
		defer lock.Unlock()
		if fired {
			return nil
		}
		fired = true
		return callback()
	}}
}

func requireMutationError(t *testing.T, err error) *MutationError {
	t.Helper()
	var mutationErr *MutationError
	require.ErrorAs(t, err, &mutationErr)
	return mutationErr
}

func replaceLeaf(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.Rename(path, path+".detached"))
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

func assertReplayContainsOneUpdate(t *testing.T, store *Store, runID string) {
	t.Helper()
	events := collectStoreEvents(t, store, runID)
	require.Len(t, events, 2)
	assert.Equal(t, "created", events[0].Type)
	assert.Equal(t, "updated", events[1].Type)
	encoded, err := json.Marshal(events[1])
	require.NoError(t, err)
	assert.LessOrEqual(t, len(encoded)+1, maxJournalRecordBytes)
}
