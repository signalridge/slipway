package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

func TestWriteStatusTextPropagatesWriterError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	view := statusView{
		Slug:            "req-123",
		ExecutionMode:   "direct",
		LifecycleStatus: "active",
		CurrentState:    "S1_PLAN",
	}

	err := writeStatusText(failingWriter{err: writeErr}, view)
	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)
}

func TestWriteContextGuardHookMessagesPropagatesWriterError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	view := nextView{
		ContextBudget: &contextBudget{
			GuardAction:      "warn",
			RemainingPercent: 42.0,
		},
	}

	err := writeContextGuardHookMessages(failingWriter{err: writeErr}, view)
	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)
}
