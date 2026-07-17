//go:build unix

package runstore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadOnlyCommitBoundariesShareUnixDirectoryLock(t *testing.T) {
	repository := createRepository(t)
	writable, err := Open(repository)
	require.NoError(t, err)
	event, err := NewEvent("created", map[string]int{"count": 1})
	require.NoError(t, err)
	require.NoError(t, writable.Create("shared-readers", event, map[string]int{"count": 1}))
	require.NoError(t, writable.Close())

	readOnly, err := OpenReadOnly(repository)
	require.NoError(t, err)
	defer readOnly.Close()
	first, err := readOnly.openRunRoot("shared-readers")
	require.NoError(t, err)
	defer first.Close()
	second, err := readOnly.openRunRoot("shared-readers")
	require.NoError(t, err)
	defer second.Close()

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- withRunCommitBoundary(first, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	secondEntered := make(chan struct{})
	secondResult := make(chan error, 1)
	go func() {
		secondResult <- withRunCommitBoundary(second, func() error {
			close(secondEntered)
			return nil
		})
	}()
	select {
	case <-secondEntered:
	case <-time.After(2 * time.Second):
		close(releaseFirst)
		require.NoError(t, <-firstResult)
		t.Fatal("second read-only commit boundary was serialized behind the first")
	}
	require.NoError(t, <-secondResult)
	close(releaseFirst)
	require.NoError(t, <-firstResult)
}
