package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomicReplacesContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "state.yaml")

	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))
	require.NoError(t, WriteFileAtomic(target, []byte("new"), 0o644))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}

func TestWriteFileAtomicCrashSafetyOldOrNewVisibilityAndRepairableTemps(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "state.yaml")
	oldData := bytes.Repeat([]byte("A"), 1024)
	newData := bytes.Repeat([]byte("B"), 2048)

	require.NoError(t, os.WriteFile(target, oldData, 0o644))

	writerErr := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			payload := oldData
			if i%2 == 1 {
				payload = newData
			}
			if err := WriteFileAtomic(target, payload, 0o644); err != nil {
				writerErr <- err
				return
			}
		}
	}()

readLoop:
	for {
		select {
		case <-done:
			break readLoop
		default:
			got, err := os.ReadFile(target)
			require.NoError(t, err)
			if !bytes.Equal(got, oldData) && !bytes.Equal(got, newData) {
				t.Fatalf("atomic visibility violated: observed non old/new content length=%d", len(got))
			}
		}
	}

	select {
	case err := <-writerErr:
		require.NoError(t, err)
	default:
	}

	staleTemp := filepath.Join(dir, ".tmp-state.yaml-stale")
	require.NoError(t, os.WriteFile(staleTemp, []byte("stale"), 0o644))

	deleted, err := CleanupAtomicTempArtifacts(dir)
	require.NoError(t, err)
	assert.Contains(t, deleted, staleTemp)

	final, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(final, oldData) || bytes.Equal(final, newData))
}
