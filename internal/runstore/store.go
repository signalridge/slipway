package runstore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Store struct {
	repositoryRoot    string
	commonDir         string
	commonRoot        *os.Root
	namespaceRoot     *os.Root
	runsRoot          *os.Root
	commonIdentity    os.FileInfo
	namespaceIdentity os.FileInfo
	runsIdentity      os.FileInfo
	hooks             storeHooks
	closeOnce         sync.Once
	closeErr          error
}

type runHandle struct {
	store    *Store
	id       string
	root     *os.Root
	identity os.FileInfo
}

func Open(start string) (*Store, error) {
	return openWithHooks(start, storeHooks{})
}

func openWithHooks(start string, hooks storeHooks) (*Store, error) {
	repository, err := discover(start)
	if err != nil {
		return nil, err
	}
	commonRoot, commonIdentity, err := openAbsoluteDirectoryRoot(repository.CommonDir)
	if err != nil {
		return nil, fmt.Errorf("open Git common directory: %w", err)
	}

	if err := faultBeforeMissingDirectoryCreate(commonRoot, "slipway", hooks, faultCreateSlipway); err != nil {
		_ = commonRoot.Close()
		return nil, fmt.Errorf("create journal namespace: %w", err)
	}
	namespaceRoot, namespaceIdentity, _, err := openPrivateChild(commonRoot, "slipway", true)
	if err != nil {
		_ = commonRoot.Close()
		return nil, fmt.Errorf("open journal namespace: %w", err)
	}
	if err := syncNewDirectory(namespaceRoot, namespaceIdentity, commonRoot, commonIdentity, "slipway", hooks, faultSyncSlipwayDirectory, faultSyncCommonDirectory); err != nil {
		_ = namespaceRoot.Close()
		_ = commonRoot.Close()
		return nil, fmt.Errorf("sync journal namespace: %w", err)
	}

	if err := faultBeforeMissingDirectoryCreate(namespaceRoot, "runs", hooks, faultCreateRuns); err != nil {
		_ = namespaceRoot.Close()
		_ = commonRoot.Close()
		return nil, fmt.Errorf("create journal root: %w", err)
	}
	runsRoot, runsIdentity, _, err := openPrivateChild(namespaceRoot, "runs", true)
	if err != nil {
		_ = namespaceRoot.Close()
		_ = commonRoot.Close()
		return nil, fmt.Errorf("open journal root: %w", err)
	}
	if err := syncNewDirectory(runsRoot, runsIdentity, namespaceRoot, namespaceIdentity, "runs", hooks, faultSyncRunsDirectory, faultSyncSlipwayDirectory); err != nil {
		_ = runsRoot.Close()
		_ = namespaceRoot.Close()
		_ = commonRoot.Close()
		return nil, fmt.Errorf("sync journal root: %w", err)
	}

	store := &Store{
		repositoryRoot:    repository.WorktreeRoot,
		commonDir:         repository.CommonDir,
		commonRoot:        commonRoot,
		namespaceRoot:     namespaceRoot,
		runsRoot:          runsRoot,
		commonIdentity:    commonIdentity,
		namespaceIdentity: namespaceIdentity,
		runsIdentity:      runsIdentity,
		hooks:             hooks,
	}
	if err := store.validateAnchors(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (store *Store) RepositoryRoot() string { return store.repositoryRoot }
func (store *Store) CommonDir() string      { return store.commonDir }

func (store *Store) Close() error {
	store.closeOnce.Do(func() {
		store.closeErr = errors.Join(store.runsRoot.Close(), store.namespaceRoot.Close(), store.commonRoot.Close())
	})
	return store.closeErr
}

// Create initializes a Run without content-addressed materials.
func (store *Store) Create(runID string, event Event, projection any) error {
	return store.create(runID, event, projection, nil)
}

// CreateWithMaterials makes every material durable before the first journal
// event that references it.
func (store *Store) CreateWithMaterials(
	runID string,
	event Event,
	projection any,
	materials []Material,
) error {
	return store.create(runID, event, projection, materials)
}

func (store *Store) create(runID string, event Event, projection any, materials []Material) error {
	tracker := newMutationTracker()
	if err := validateRunID(runID); err != nil {
		return tracker.fail(PhaseValidation, false, err)
	}
	if event.At.IsZero() {
		return tracker.fail(PhaseValidation, false, errors.New("event timestamp is required"))
	}
	event.Sequence = 1
	journalContent, err := encodeJournalEvents([]Event{event})
	if err != nil {
		return tracker.fail(PhaseJournalWrite, false, err)
	}
	projectionContent, err := encodeSnapshot(1, projection)
	if err != nil {
		return tracker.fail(PhaseProjectionEncode, false, err)
	}
	if err := store.validateAnchors(); err != nil {
		return tracker.fail(PhaseNamespaceVerify, false, err)
	}
	if err := store.hooks.at(faultCreateRun); err != nil {
		return tracker.fail(PhaseDirectoryCreate, false, err)
	}
	if err := store.runsRoot.Mkdir(runID, 0o700); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return tracker.fail(PhaseDirectoryCreate, false, fmt.Errorf("run %q already exists", runID))
		}
		return tracker.fail(PhaseDirectoryCreate, false, fmt.Errorf("create run directory: %w", err))
	}
	runRoot, runIdentity, _, err := openPrivateChild(store.runsRoot, runID, false)
	if err != nil {
		return tracker.fail(PhaseDirectoryCreate, false, fmt.Errorf("open run directory: %w", err))
	}
	run := &runHandle{store: store, id: runID, root: runRoot, identity: runIdentity}
	defer run.Close()
	if err := run.validate(); err != nil {
		return tracker.fail(PhaseNamespaceVerify, false, err)
	}
	if err := syncNewDirectory(run.root, run.identity, store.runsRoot, store.runsIdentity, runID, store.hooks, faultSyncRunDirectory, faultSyncRunsParent); err != nil {
		return tracker.fail(PhaseDirectorySync, false, err)
	}
	if err := store.putMaterials(run, materials); err != nil {
		return tracker.fail(PhaseDirectorySync, false, fmt.Errorf("persist run materials: %w", err))
	}

	err = withRunLock(run, tracker, func(transaction *runTransaction) error {
		prepared, err := prepareSnapshot(transaction, projectionFileName, projectionContent)
		if err != nil {
			return err
		}
		defer prepared.discard()
		if err := appendEncodedJournal(transaction, journalFileName, journalContent); err != nil {
			return err
		}
		if err := syncAnchoredDirectory(run.root, run.identity, store.hooks, faultSyncRunDirectory); err != nil {
			return tracker.fail(PhaseDirectorySync, false, fmt.Errorf("sync new journal entry: %w", err))
		}
		if err := transaction.validate(PhaseNamespaceVerify, faultValidateRun); err != nil {
			return err
		}
		if err := commitSnapshot(transaction, prepared); err != nil {
			return err
		}
		if err := transaction.validate(PhaseDirectorySync, faultValidateRun); err != nil {
			return err
		}
		if err := syncAnchoredDirectory(store.runsRoot, store.runsIdentity, store.hooks, faultSyncRunsParent); err != nil {
			return tracker.fail(PhaseDirectorySync, false, fmt.Errorf("sync runs parent: %w", err))
		}
		return transaction.validate(PhaseNamespaceVerify, faultValidateRun)
	})
	return err
}

// Visit streams authoritative journal events while holding the per-run lock.
// The caller retains only the projection state it needs.
func (store *Store) Visit(runID string, consume func(Event) error) error {
	run, err := store.openRunRoot(runID)
	if err != nil {
		return err
	}
	defer run.Close()
	return withRunLock(run, nil, func(transaction *runTransaction) error {
		_, err := visitJournal(transaction.journalContext(), journalFileName, consume)
		return err
	})
}

// UpdateStream serializes a streaming replay/mutate/append transaction. Events
// are durable before the replaceable projection is committed.
// UpdateResult is the complete output of one locked Run mutation callback.
type UpdateResult struct {
	Events     []Event
	Projection any
	Materials  []Material
}

// UpdateStreamWithMaterials makes callback-selected materials durable after
// business validation succeeds and before any returned event is appended.
func (store *Store) UpdateStreamWithMaterials(
	runID string,
	consume func(Event) error,
	callback func() (UpdateResult, error),
) error {
	tracker := newMutationTracker()
	run, err := store.openRunRoot(runID)
	if err != nil {
		return tracker.fail(PhaseNamespaceVerify, false, err)
	}
	defer run.Close()
	return withRunLock(run, tracker, func(transaction *runTransaction) error {
		count, err := visitJournal(transaction.journalContext(), journalFileName, consume)
		if err != nil {
			return tracker.fail(PhaseReplay, false, err)
		}
		if err := transaction.validate(PhaseLockVerify, faultLockBeforeCallback); err != nil {
			return err
		}
		result, err := callback()
		if err != nil {
			return tracker.fail(PhaseCallback, false, err)
		}
		if err := transaction.validate(PhaseLockVerify, faultLockAfterCallback); err != nil {
			return err
		}
		if len(result.Events) == 0 {
			return nil
		}
		for index := range result.Events {
			result.Events[index].Sequence = count + index + 1
			if result.Events[index].At.IsZero() {
				return tracker.fail(PhaseValidation, false, fmt.Errorf("event %d timestamp is required", index))
			}
		}
		journalContent, err := encodeJournalEvents(result.Events)
		if err != nil {
			return tracker.fail(PhaseJournalWrite, false, err)
		}
		projectionContent, err := encodeSnapshot(count+len(result.Events), result.Projection)
		if err != nil {
			return tracker.fail(PhaseProjectionEncode, false, err)
		}
		prepared, err := prepareSnapshot(transaction, projectionFileName, projectionContent)
		if err != nil {
			return err
		}
		defer prepared.discard()
		if err := store.putMaterials(run, result.Materials); err != nil {
			return tracker.fail(PhaseDirectorySync, false, fmt.Errorf("persist run materials: %w", err))
		}
		if err := appendEncodedJournal(transaction, journalFileName, journalContent); err != nil {
			return err
		}
		return commitSnapshot(transaction, prepared)
	})
}

func (store *Store) UpdateStream(
	runID string,
	consume func(Event) error,
	callback func() ([]Event, any, error),
) error {
	return store.UpdateStreamWithMaterials(runID, consume, func() (UpdateResult, error) {
		events, projection, err := callback()
		return UpdateResult{Events: events, Projection: projection}, err
	})
}

func (store *Store) ListIDs() ([]string, error) {
	if err := store.validateAnchors(); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(store.runsRoot.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	if err := store.validateAnchors(); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || validateRunID(entry.Name()) != nil {
			continue
		}
		ids = append(ids, entry.Name())
	}
	sort.Strings(ids)
	return ids, nil
}

func (store *Store) openRunRoot(runID string) (*runHandle, error) {
	if err := validateRunID(runID); err != nil {
		return nil, err
	}
	if err := store.validateAnchors(); err != nil {
		return nil, err
	}
	runRoot, runIdentity, _, err := openPrivateChild(store.runsRoot, runID, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("run %q not found", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("inspect run: %w", err)
	}
	run := &runHandle{store: store, id: runID, root: runRoot, identity: runIdentity}
	if err := run.validate(); err != nil {
		_ = run.Close()
		return nil, err
	}
	return run, nil
}

func (run *runHandle) Close() error {
	if run == nil || run.root == nil {
		return nil
	}
	return run.root.Close()
}

func (run *runHandle) validate() error {
	if err := run.store.validateAnchors(); err != nil {
		return err
	}
	if err := run.store.hooks.at(faultValidateRun); err != nil {
		return fmt.Errorf("validate run directory: %w", err)
	}
	if err := validateDirectoryIdentity(run.store.runsRoot, run.id, run.identity); err != nil {
		return fmt.Errorf("run directory changed after opening: %w", err)
	}
	if err := validateOpenedDirectoryRoot(run.root, run.identity); err != nil {
		return fmt.Errorf("opened run directory changed: %w", err)
	}
	return nil
}

func (store *Store) validateAnchors() error {
	if err := store.hooks.at(faultValidateCommon); err != nil {
		return fmt.Errorf("validate Git common directory: %w", err)
	}
	if err := validateAbsoluteDirectoryIdentity(store.commonDir, store.commonRoot, store.commonIdentity); err != nil {
		return fmt.Errorf("git common directory changed after opening: %w", err)
	}
	if err := store.hooks.at(faultValidateSlipway); err != nil {
		return fmt.Errorf("validate journal namespace: %w", err)
	}
	if err := validateDirectoryIdentity(store.commonRoot, "slipway", store.namespaceIdentity); err != nil {
		return fmt.Errorf("journal namespace changed after opening: %w", err)
	}
	if err := validateOpenedDirectoryRoot(store.namespaceRoot, store.namespaceIdentity); err != nil {
		return fmt.Errorf("opened journal namespace changed: %w", err)
	}
	if err := store.hooks.at(faultValidateRuns); err != nil {
		return fmt.Errorf("validate journal root: %w", err)
	}
	if err := validateDirectoryIdentity(store.namespaceRoot, "runs", store.runsIdentity); err != nil {
		return fmt.Errorf("journal root changed after opening: %w", err)
	}
	if err := validateOpenedDirectoryRoot(store.runsRoot, store.runsIdentity); err != nil {
		return fmt.Errorf("opened journal root changed: %w", err)
	}
	return nil
}

func openAbsoluteDirectoryRoot(path string) (*os.Root, os.FileInfo, error) {
	if !filepath.IsAbs(path) {
		return nil, nil, fmt.Errorf("directory path %q is not absolute", path)
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, nil, fmt.Errorf("path %q is not a regular directory", path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, nil, err
	}
	if err := validateAbsoluteDirectoryIdentity(path, root, before); err != nil {
		_ = root.Close()
		return nil, nil, fmt.Errorf("directory path %q changed while opening: %w", path, err)
	}
	return root, before, nil
}

func openPrivateChild(parent *os.Root, name string, create bool) (*os.Root, os.FileInfo, bool, error) {
	if name == "" || filepath.Base(name) != name {
		return nil, nil, false, fmt.Errorf("invalid child directory %q", name)
	}
	created := false
	for attempt := 0; attempt < 3; attempt++ {
		before, err := parent.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) && create {
			if err := parent.Mkdir(name, 0o700); errors.Is(err, fs.ErrExist) {
				continue
			} else if err != nil {
				return nil, nil, false, fmt.Errorf("create private directory: %w", err)
			}
			created = true
			before, err = parent.Lstat(name)
		}
		if err != nil {
			return nil, nil, false, err
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			return nil, nil, false, fmt.Errorf("run path %q is not a regular directory", name)
		}
		child, err := parent.OpenRoot(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, false, err
		}
		current, err := parent.Lstat(name)
		if err != nil {
			_ = child.Close()
			return nil, nil, false, err
		}
		if current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(before, current) {
			_ = child.Close()
			continue
		}
		if err := validateOpenedDirectoryRoot(child, current); err != nil {
			_ = child.Close()
			continue
		}
		directory, err := child.Open(".")
		if err != nil {
			_ = child.Close()
			return nil, nil, false, err
		}
		if err := directory.Chmod(0o700); err != nil {
			_ = directory.Close()
			_ = child.Close()
			return nil, nil, false, fmt.Errorf("secure private directory: %w", err)
		}
		if err := directory.Close(); err != nil {
			_ = child.Close()
			return nil, nil, false, fmt.Errorf("close private directory: %w", err)
		}
		return child, current, created, nil
	}
	return nil, nil, false, fmt.Errorf("run path %q changed while opening", name)
}

func validateAbsoluteDirectoryIdentity(path string, root *os.Root, expected os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(current, expected) {
		return fmt.Errorf("path %q is not the opened regular directory", path)
	}
	return validateOpenedDirectoryRoot(root, expected)
}

func validateDirectoryIdentity(root *os.Root, name string, expected os.FileInfo) error {
	current, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(current, expected) {
		return fmt.Errorf("path %q is not the opened regular directory", name)
	}
	return nil
}

func validateOpenedDirectoryRoot(root *os.Root, expected os.FileInfo) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	opened, err := directory.Stat()
	if err != nil {
		return err
	}
	if !opened.IsDir() || !os.SameFile(opened, expected) {
		return errors.New("opened directory identity changed")
	}
	return nil
}

func syncNewDirectory(child *os.Root, childIdentity os.FileInfo, parent *os.Root, parentIdentity os.FileInfo, name string, hooks storeHooks, childPoint, parentPoint faultPoint) error {
	if err := validateDirectoryIdentity(parent, name, childIdentity); err != nil {
		return fmt.Errorf("validate new directory before sync: %w", err)
	}
	if err := syncAnchoredDirectory(child, childIdentity, hooks, childPoint); err != nil {
		return err
	}
	if err := validateDirectoryIdentity(parent, name, childIdentity); err != nil {
		return fmt.Errorf("validate new directory before parent sync: %w", err)
	}
	if err := syncAnchoredDirectory(parent, parentIdentity, hooks, parentPoint); err != nil {
		return err
	}
	if err := validateDirectoryIdentity(parent, name, childIdentity); err != nil {
		return fmt.Errorf("validate new directory after parent sync: %w", err)
	}
	return nil
}

func syncAnchoredDirectory(root *os.Root, expected os.FileInfo, hooks storeHooks, point faultPoint) error {
	if err := validateOpenedDirectoryRoot(root, expected); err != nil {
		return err
	}
	if err := hooks.at(point); err != nil {
		return err
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	opened, err := directory.Stat()
	if err != nil {
		return err
	}
	if !opened.IsDir() || !os.SameFile(opened, expected) {
		return errors.New("directory identity changed before sync")
	}
	if err := syncDirectoryFile(directory); err != nil {
		return err
	}
	after, err := directory.Stat()
	if err != nil {
		return err
	}
	if !after.IsDir() || !os.SameFile(after, expected) {
		return errors.New("directory identity changed after sync")
	}
	return validateOpenedDirectoryRoot(root, expected)
}

func faultBeforeMissingDirectoryCreate(parent *os.Root, name string, hooks storeHooks, point faultPoint) error {
	_, err := parent.Lstat(name)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return hooks.at(point)
}
