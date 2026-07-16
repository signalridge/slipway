package runstore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorePersistsAndRevalidatesContentAddressedMaterials(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	defer store.Close()

	content := []byte("# Requirements\n\nPreserve exact bytes.\n")
	digest := materialDigest(content)
	event, err := NewEvent("created", map[string]int{"count": 1})
	require.NoError(t, err)
	err = store.CreateWithMaterials(
		"run-material",
		event,
		map[string]int{"count": 1},
		[]Material{{Digest: digest, Data: content}},
	)
	require.NoError(t, err)

	read, err := store.ReadMaterial("run-material", digest)
	require.NoError(t, err)
	assert.Equal(t, content, read)
	require.NoError(t, store.PutMaterials("run-material", []Material{{Digest: digest, Data: content}}))

	filename, err := materialFilename(digest)
	require.NoError(t, err)
	materialDirectory := filepath.Join(store.CommonDir(), "slipway", "runs", "run-material", materialsDirectoryName)
	materialPath := filepath.Join(materialDirectory, filename)
	info, err := os.Stat(materialPath)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		directoryInfo, statErr := os.Stat(materialDirectory)
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o700), directoryInfo.Mode().Perm())
	}
	journal, err := os.ReadFile(filepath.Join(filepath.Dir(materialDirectory), journalFileName))
	require.NoError(t, err)
	assert.NotContains(t, string(journal), "Preserve exact bytes")

	require.NoError(t, os.WriteFile(materialPath, []byte(strings.Repeat("x", len(content))), 0o600))
	_, err = store.ReadMaterial("run-material", digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupt")
}

func TestReadOnlyVisitWithMaterialReaderUsesCommitBoundaryWithoutMutation(t *testing.T) {
	repository := createRepository(t)
	writable, err := Open(repository)
	require.NoError(t, err)
	content := []byte("# Read-only material\n")
	digest := materialDigest(content)
	event, err := NewEvent("created", map[string]int{"count": 1})
	require.NoError(t, err)
	require.NoError(t, writable.CreateWithMaterials(
		"run-read-only-material",
		event,
		map[string]int{"count": 1},
		[]Material{{Digest: digest, Data: content}},
	))
	require.NoError(t, writable.Close())

	readOnly, err := OpenReadOnly(repository)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, readOnly.Close()) })
	var events int
	err = readOnly.VisitWithMaterialReader(
		"run-read-only-material",
		func(Event) error {
			events++
			return nil
		},
		func(readMaterial MaterialReader) error {
			actual, readErr := readMaterial(digest)
			if readErr != nil {
				return readErr
			}
			assert.Equal(t, content, actual)
			return nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, events)
}

func TestVisitWithMaterialReaderHoldsRunLockThroughRead(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	defer store.Close()

	content := []byte("# Locked material\n")
	digest := materialDigest(content)
	event, err := NewEvent("created", map[string]int{"count": 1})
	require.NoError(t, err)
	require.NoError(t, store.CreateWithMaterials(
		"run-material-lock",
		event,
		map[string]int{"count": 1},
		[]Material{{Digest: digest, Data: content}},
	))

	readerEntered := make(chan struct{})
	releaseReader := make(chan struct{})
	readerDone := make(chan error, 1)
	go func() {
		readerDone <- store.VisitWithMaterialReader(
			"run-material-lock",
			func(Event) error { return nil },
			func(readMaterial MaterialReader) error {
				close(readerEntered)
				<-releaseReader
				read, readErr := readMaterial(digest)
				if readErr != nil {
					return readErr
				}
				if string(read) != string(content) {
					return errors.New("material content changed")
				}
				return nil
			},
		)
	}()
	<-readerEntered

	mutationStarted := make(chan struct{})
	mutationEntered := make(chan struct{})
	mutationDone := make(chan error, 1)
	go func() {
		close(mutationStarted)
		mutationDone <- store.UpdateStream(
			"run-material-lock",
			func(Event) error { return nil },
			func() ([]Event, any, error) {
				close(mutationEntered)
				return nil, nil, nil
			},
		)
	}()
	<-mutationStarted
	select {
	case <-mutationEntered:
		t.Fatal("mutation entered while the authorized material read held the Run lock")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseReader)
	require.NoError(t, <-readerDone)
	select {
	case <-mutationEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("mutation did not enter after the material read released the Run lock")
	}
	require.NoError(t, <-mutationDone)
}

func TestStoreRejectsInvalidMaterialInputsAndUnsafeLeaves(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, createOneRun(store, "run-material-invalid"))

	content := []byte("bounded")
	digest := materialDigest(content)
	tests := []struct {
		name     string
		material Material
		want     string
	}{
		{name: "malformed digest", material: Material{Digest: "sha256:ABC", Data: content}, want: "lowercase"},
		{name: "digest mismatch", material: Material{Digest: digest, Data: []byte("different")}, want: "does not match"},
		{name: "empty", material: Material{Digest: materialDigest(nil), Data: nil}, want: "1.."},
		{name: "too large", material: Material{Digest: materialDigest(make([]byte, maxMaterialBytes+1)), Data: make([]byte, maxMaterialBytes+1)}, want: "1.."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := store.PutMaterials("run-material-invalid", []Material{test.material})
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}

	if runtime.GOOS == "windows" {
		return
	}
	require.NoError(t, store.PutMaterials("run-material-invalid", []Material{{Digest: digest, Data: content}}))
	filename, err := materialFilename(digest)
	require.NoError(t, err)
	materialPath := filepath.Join(store.CommonDir(), "slipway", "runs", "run-material-invalid", materialsDirectoryName, filename)
	require.NoError(t, os.Remove(materialPath))
	require.NoError(t, os.Symlink(filepath.Join(repository, "README.md"), materialPath))
	_, err = store.ReadMaterial("run-material-invalid", digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regular file")
}

func TestMaterialCommitNeverOverwritesConflictingDestination(t *testing.T) {
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, createOneRun(store, "run-material-conflict"))

	content := []byte("expected immutable bytes")
	digest := materialDigest(content)
	filename, err := materialFilename(digest)
	require.NoError(t, err)
	materialDirectory := filepath.Join(store.CommonDir(), "slipway", "runs", "run-material-conflict", materialsDirectoryName)
	require.NoError(t, os.Mkdir(materialDirectory, 0o700))
	path := filepath.Join(materialDirectory, filename)
	conflict := []byte("attacker-controlled conflict")
	require.NoError(t, os.WriteFile(path, conflict, 0o600))
	before, err := os.Stat(path)
	require.NoError(t, err)

	err = store.PutMaterials("run-material-conflict", []Material{{Digest: digest, Data: content}})
	require.Error(t, err)
	after, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.True(t, os.SameFile(before, after))
	bytes, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, conflict, bytes)
}

func TestMaterialsReplacementBeforeJournalAppendKeepsJournalUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement while opened is not portable on Windows")
	}
	repository := createRepository(t)
	store, err := Open(repository)
	require.NoError(t, err)
	defer store.Close()
	runID := "run-material-detached"
	require.NoError(t, createOneRun(store, runID))

	seed := []byte("seed")
	require.NoError(t, store.PutMaterials(runID, []Material{{Digest: materialDigest(seed), Data: seed}}))
	runDirectory := filepath.Join(store.CommonDir(), "slipway", "runs", runID)
	materialDirectory := filepath.Join(runDirectory, materialsDirectoryName)
	journalPath := filepath.Join(runDirectory, journalFileName)
	journalBefore, err := os.ReadFile(journalPath)
	require.NoError(t, err)

	detached := materialDirectory + ".detached"
	installOneShotHook(store, faultSyncRunDirectory, func() error {
		require.NoError(t, os.Rename(materialDirectory, detached))
		return os.Mkdir(materialDirectory, 0o700)
	})
	content := []byte("new detached material")
	next, err := NewEvent("updated", map[string]bool{"material": true})
	require.NoError(t, err)
	err = store.UpdateStreamWithMaterials(runID, func(Event) error { return nil }, func() (UpdateResult, error) {
		return UpdateResult{
			Events:     []Event{next},
			Projection: map[string]bool{"material": true},
			Materials:  []Material{{Digest: materialDigest(content), Data: content}},
		}, nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "materials")
	journalAfter, readErr := os.ReadFile(journalPath)
	require.NoError(t, readErr)
	assert.Equal(t, journalBefore, journalAfter)
	filename, err := materialFilename(materialDigest(content))
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(detached, filename))
}
