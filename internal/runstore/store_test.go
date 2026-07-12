package runstore

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveGitDetectsChangesToInitiallyDirtyAndUntrackedFiles(t *testing.T) {
	repository := createRepository(t)
	tracked := filepath.Join(repository, "README.md")
	untracked := filepath.Join(repository, "notes.txt")
	require.NoError(t, os.WriteFile(tracked, []byte("dirty one\n"), 0o600))
	require.NoError(t, os.WriteFile(untracked, []byte("draft one\n"), 0o600))
	initial, err := ObserveGit(repository)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"README.md", "notes.txt"}, initial.DirtyFiles)

	require.NoError(t, os.WriteFile(tracked, []byte("dirty two\n"), 0o600))
	trackedChanged, err := ObserveGit(repository)
	require.NoError(t, err)
	assert.True(t, trackedChanged.ChangedFrom(initial))

	require.NoError(t, os.WriteFile(tracked, []byte("dirty one\n"), 0o600))
	require.NoError(t, os.WriteFile(untracked, []byte("draft two\n"), 0o600))
	untrackedChanged, err := ObserveGit(repository)
	require.NoError(t, err)
	assert.True(t, untrackedChanged.ChangedFrom(initial))
}

func TestObserveGitDetectsRetargetedUntrackedSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	repository := createRepository(t)
	require.NoError(t, os.WriteFile(filepath.Join(repository, "a.txt"), []byte("a\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "b.txt"), []byte("b\n"), 0o600))
	link := filepath.Join(repository, "link.txt")
	require.NoError(t, os.Symlink("a.txt", link))
	initial, err := ObserveGit(repository)
	require.NoError(t, err)
	initialLink := requirePathObservation(t, initial, "link.txt")
	assert.Equal(t, "symlink", initialLink.Observation)
	assert.Equal(t, digestBytes([]byte("a.txt")), initialLink.ContentSHA256)

	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink("b.txt", link))
	changed, err := ObserveGit(repository)
	require.NoError(t, err)
	assert.True(t, changed.ChangedFrom(initial))
	changedLink := requirePathObservation(t, changed, "link.txt")
	assert.Equal(t, "symlink", changedLink.Observation)
	assert.Equal(t, digestBytes([]byte("b.txt")), changedLink.ContentSHA256)
}

func TestObserveGitBoundsOversizeUntrackedFileWithoutContentHash(t *testing.T) {
	repository := createRepository(t)
	path := filepath.Join(repository, "large.bin")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(t, err)
	require.NoError(t, file.Truncate(MaxObservedFileBytes+1))
	require.NoError(t, file.Close())

	observation, err := ObserveGit(repository)
	require.NoError(t, err)
	require.Len(t, observation.PathObservations, 1)
	assert.Equal(t, "oversize", observation.PathObservations[0].Observation)
	assert.Empty(t, observation.PathObservations[0].ContentSHA256)
	require.NotNil(t, observation.PathObservations[0].Size)
	assert.Equal(t, MaxObservedFileBytes+1, *observation.PathObservations[0].Size)
}

func TestStoreUsesGitCommonDirectoryAndPrivateJournalFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits and linked-worktree path assertions are Unix-specific")
	}
	mainRepository := createRepository(t)
	linked := filepath.Join(t.TempDir(), "linked")
	runGitCommand(t, mainRepository, "worktree", "add", "-q", "-b", "linked-test", linked)
	store, err := Open(linked)
	require.NoError(t, err)
	expectedCommon, err := filepath.EvalSymlinks(filepath.Join(mainRepository, ".git"))
	require.NoError(t, err)
	expectedLinked, err := filepath.EvalSymlinks(linked)
	require.NoError(t, err)
	assert.Equal(t, expectedCommon, store.CommonDir())
	assert.Equal(t, expectedLinked, store.RepositoryRoot())

	event, err := NewEvent("created", map[string]string{"id": "run-one"})
	require.NoError(t, err)
	require.NoError(t, store.Create("run-one", event, map[string]string{"state": "active"}))
	paths, err := pathsFor(store.CommonDir(), "run-one")
	require.NoError(t, err)
	for _, path := range []string{paths.JournalFile, paths.RunFile, paths.LockFile} {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), path)
	}
	info, err := os.Stat(paths.Directory)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestJournalIgnoresOnlyAnInterruptedFinalRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	first, err := NewEvent("first", map[string]int{"value": 1})
	require.NoError(t, err)
	first.Sequence = 1
	encoded, err := json.Marshal(first)
	require.NoError(t, err)
	content := append(append(encoded, '\n'), []byte(`{"sequence":2,"type":"partial"`)...)
	require.NoError(t, os.WriteFile(path, content, 0o600))

	events, err := readAllJournal(path)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "first", events[0].Type)
	repaired, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, append(append([]byte(nil), encoded...), '\n'), repaired)

	second, err := NewEvent("second", map[string]int{"value": 2})
	require.NoError(t, err)
	second.Sequence = 2
	require.NoError(t, appendAllJournal(path, []Event{second}))
	events, err = readAllJournal(path)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "second", events[1].Type)

	broken := append(append([]byte(nil), encoded...), []byte("\nnot-json\n")...)
	require.NoError(t, os.WriteFile(path, broken, 0o600))
	_, err = readAllJournal(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event 2")
}

func TestJournalBoundsOversizedRecordsAndRepairsOversizedInterruptedTail(t *testing.T) {
	first, err := NewEvent("first", map[string]int{"value": 1})
	require.NoError(t, err)
	first.Sequence = 1
	encoded, err := json.Marshal(first)
	require.NoError(t, err)

	t.Run("interrupted final record is truncated", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "journal.jsonl")
		content := append(append(append([]byte(nil), encoded...), '\n'), bytes.Repeat([]byte{'x'}, maxJournalRecordBytes+1)...)
		require.NoError(t, os.WriteFile(path, content, 0o600))
		events, err := readAllJournal(path)
		require.NoError(t, err)
		require.Len(t, events, 1)
		repaired, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, append(append([]byte(nil), encoded...), '\n'), repaired)
	})

	t.Run("complete oversized record is rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "journal.jsonl")
		content := append(bytes.Repeat([]byte{'x'}, maxJournalRecordBytes+1), '\n')
		require.NoError(t, os.WriteFile(path, content, 0o600))
		_, err := readAllJournal(path)
		require.Error(t, err)
		assert.ErrorContains(t, err, "record exceeds")
	})
}

func TestAppendJournalRejectsOversizedEncodedRecordWithoutPartialWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	events := []Event{
		{Type: "first", At: time.Now().UTC(), Data: json.RawMessage(`{"value":1}`)},
		{Type: strings.Repeat("\n", maxJournalRecordBytes/2+1), At: time.Now().UTC(), Data: json.RawMessage(`null`)},
	}
	err := appendAllJournal(path, events)
	require.Error(t, err)
	assert.ErrorContains(t, err, "event exceeds")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestStoreVisitStreamsEventLargerThanScannerDefaults(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	payload := map[string]string{"value": strings.Repeat("x", 256<<10)}
	event, err := NewEvent("large", payload)
	require.NoError(t, err)
	require.NoError(t, store.Create("large-event", event, payload))

	visited := 0
	err = store.Visit("large-event", func(event Event) error {
		visited++
		assert.Greater(t, len(event.Data), 64<<10)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, visited)
}

func TestStoreUpdateRepairsInterruptedTailBeforeAppending(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	runID := "00000000-0000-4000-8000-000000000201"
	first, err := NewEvent("first", map[string]int{"count": 1})
	require.NoError(t, err)
	require.NoError(t, store.Create(runID, first, map[string]int{"count": 1}))
	paths, err := pathsFor(store.CommonDir(), runID)
	require.NoError(t, err)
	file, err := os.OpenFile(paths.JournalFile, os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)
	_, err = file.WriteString(`{"sequence":2,"type":"interrupted"`)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	seen := 0
	require.NoError(t, store.UpdateStream(runID, func(Event) error {
		seen++
		return nil
	}, func() ([]Event, any, error) {
		require.Equal(t, 1, seen)
		second, eventErr := NewEvent("second", map[string]int{"count": 2})
		return []Event{second}, map[string]int{"count": 2}, eventErr
	}))
	events := collectStoreEvents(t, store, runID)
	require.Len(t, events, 2)
	assert.Equal(t, "second", events[1].Type)
}

func TestStoreProjectionSerializationFailureIsPreCommit(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	first, err := NewEvent("first", map[string]int{"count": 1})
	require.NoError(t, err)
	require.NoError(t, store.Create("projection-failure", first, map[string]int{"count": 1}))

	err = store.UpdateStream("projection-failure", func(Event) error { return nil }, func() ([]Event, any, error) {
		second, eventErr := NewEvent("second", map[string]int{"count": 2})
		return []Event{second}, func() {}, eventErr
	})
	var mutationErr *MutationError
	require.ErrorAs(t, err, &mutationErr)
	assert.False(t, mutationErr.Committed)
	assert.False(t, mutationErr.ProjectionStale)
	assert.False(t, mutationErr.Ambiguous)
	assert.Equal(t, PhaseProjectionEncode, mutationErr.Phase)
	events := collectStoreEvents(t, store, "projection-failure")
	require.Len(t, events, 1)
	assert.Equal(t, "first", events[0].Type)
}

func TestStoreCreateProjectionSerializationFailureIsPreCommit(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	first, err := NewEvent("first", map[string]int{"count": 1})
	require.NoError(t, err)

	err = store.Create("create-projection-failure", first, func() {})
	var mutationErr *MutationError
	require.ErrorAs(t, err, &mutationErr)
	assert.False(t, mutationErr.Committed)
	assert.False(t, mutationErr.ProjectionStale)
	assert.Equal(t, PhaseProjectionEncode, mutationErr.Phase)
	_, statErr := os.Stat(filepath.Join(store.CommonDir(), "slipway", "runs", "create-projection-failure"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestStoreSerializesConcurrentUpdates(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	event, err := NewEvent("created", map[string]int{"count": 0})
	require.NoError(t, err)
	require.NoError(t, store.Create("concurrent", event, map[string]int{"count": 0}))

	const workers = 12
	var wait sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for index := range workers {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			count := 0
			errorsSeen <- store.UpdateStream("concurrent", func(Event) error {
				count++
				return nil
			}, func() ([]Event, any, error) {
				next, err := NewEvent("increment", map[string]int{"worker": index})
				return []Event{next}, map[string]int{"count": count}, err
			})
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		require.NoError(t, err)
	}
	events := collectStoreEvents(t, store, "concurrent")
	assert.Len(t, events, workers+1)
	for index, event := range events {
		assert.Equal(t, index+1, event.Sequence)
	}
}

func TestRemovingRunJournalOnlyRemovesRecovery(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	event, err := NewEvent("created", map[string]string{"goal": "test"})
	require.NoError(t, err)
	require.NoError(t, store.Create("remove-me", event, map[string]string{"state": "active"}))
	paths, err := pathsFor(store.CommonDir(), "remove-me")
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(paths.Directory))

	err = store.Visit("remove-me", func(Event) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	_, err = os.Stat(filepath.Join(repository, "README.md"))
	assert.NoError(t, err)
}

func TestOpenRejectsSymlinkedJournalDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	repository := createRepository(t)
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repository, ".git", "slipway"), 0o700))
	require.NoError(t, os.Symlink(target, filepath.Join(repository, ".git", "slipway", "runs")))
	_, err := Open(repository)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular directory")
}

func TestOpenRejectsSymlinkedSlipwayNamespace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	repository := createRepository(t)
	target := t.TempDir()
	require.NoError(t, os.Symlink(target, filepath.Join(repository, ".git", "slipway")))
	_, err := Open(repository)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular directory")
}

func TestStoreRejectsJournalParentSwapAfterOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	for _, component := range []string{"slipway", filepath.Join("slipway", "runs")} {
		component := component
		t.Run(component, func(t *testing.T) {
			repository := createRepository(t)
			store, err := Open(repository)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			original := filepath.Join(store.CommonDir(), component)
			moved := original + "-original"
			require.NoError(t, os.Rename(original, moved))
			outside := t.TempDir()
			require.NoError(t, os.Symlink(outside, original))

			event, eventErr := NewEvent("created", map[string]int{"count": 0})
			require.NoError(t, eventErr)
			err = store.Create("parent-swap", event, map[string]int{"count": 0})
			require.Error(t, err)
			assert.Empty(t, mustReadDirNames(t, outside))
		})
	}
}

func TestStoreRejectsSymlinkedRunAndLeafPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)

	newRun := func(t *testing.T, runID string) Paths {
		t.Helper()
		event, err := NewEvent("created", map[string]int{"count": 0})
		require.NoError(t, err)
		require.NoError(t, store.Create(runID, event, map[string]int{"count": 0}))
		paths, err := pathsFor(store.CommonDir(), runID)
		require.NoError(t, err)
		return paths
	}

	t.Run("run directory", func(t *testing.T) {
		paths := newRun(t, "00000000-0000-4000-8000-000000000101")
		require.NoError(t, os.RemoveAll(paths.Directory))
		outside := t.TempDir()
		require.NoError(t, os.Symlink(outside, paths.Directory))
		err := store.Visit("00000000-0000-4000-8000-000000000101", func(Event) error { return nil })
		require.Error(t, err)
		assert.Empty(t, mustReadDirNames(t, outside))
	})

	for _, test := range []struct {
		name  string
		runID string
		leaf  func(Paths) string
		load  bool
	}{
		{name: "journal", runID: "00000000-0000-4000-8000-000000000102", leaf: func(paths Paths) string { return paths.JournalFile }, load: true},
		{name: "lock", runID: "00000000-0000-4000-8000-000000000103", leaf: func(paths Paths) string { return paths.LockFile }, load: true},
		{name: "projection", runID: "00000000-0000-4000-8000-000000000104", leaf: func(paths Paths) string { return paths.RunFile }},
	} {
		t.Run(test.name, func(t *testing.T) {
			paths := newRun(t, test.runID)
			leaf := test.leaf(paths)
			require.NoError(t, os.Remove(leaf))
			target := filepath.Join(t.TempDir(), "outside")
			require.NoError(t, os.WriteFile(target, []byte("unchanged"), 0o600))
			require.NoError(t, os.Symlink(target, leaf))
			if test.load {
				err = store.Visit(test.runID, func(Event) error { return nil })
			} else {
				count := 0
				err = store.UpdateStream(test.runID, func(Event) error {
					count++
					return nil
				}, func() ([]Event, any, error) {
					next, eventErr := NewEvent("updated", map[string]int{"count": count})
					return []Event{next}, map[string]int{"count": count}, eventErr
				})
			}
			require.Error(t, err)
			content, readErr := os.ReadFile(target)
			require.NoError(t, readErr)
			assert.Equal(t, "unchanged", string(content))
		})
	}
}

func readAllJournal(path string) ([]Event, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	context, err := newStandaloneJournalContext(root)
	if err != nil {
		return nil, err
	}
	var events []Event
	_, err = visitJournal(context, filepath.Base(path), func(event Event) error {
		events = append(events, event)
		return nil
	})
	return events, err
}

func appendAllJournal(path string, events []Event) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	context, err := newStandaloneJournalContext(root)
	if err != nil {
		return err
	}
	return appendJournal(context, filepath.Base(path), events)
}

func collectStoreEvents(t *testing.T, store *Store, runID string) []Event {
	t.Helper()
	var events []Event
	require.NoError(t, store.Visit(runID, func(event Event) error {
		events = append(events, event)
		return nil
	}))
	return events
}

func mustReadDirNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func createRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitCommand(t, root, "init", "-q")
	runGitCommand(t, root, "config", "user.name", "Slipway Test")
	runGitCommand(t, root, "config", "user.email", "test@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("initial\n"), 0o600))
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-qm", "initial")
	return root
}

func runGitCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
}
