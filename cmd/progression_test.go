package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapProgressionError_PassesThroughUnknownError(t *testing.T) {
	t.Parallel()

	original := errors.New("task_evidence_invalid: task-a.json")
	assert.Same(t, original, mapProgressionError(original, "req-1"))
}

func TestMapProgressionError_NoNextState(t *testing.T) {
	t.Parallel()

	err := mapProgressionError(fmt.Errorf("%w: no next state from S7 at level L2", progression.ErrNoNextState), "req-2")
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "no_next_state", cliErr.ErrorCode)
	assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
}

func TestMapProgressionError_GateBlocked(t *testing.T) {
	t.Parallel()

	err := mapProgressionError(fmt.Errorf("%w: approval still pending", progression.ErrGateBlocked), "req-3")
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "gate_blocked", cliErr.ErrorCode)
	assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
	assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	assert.Equal(t, "req-3", cliErr.Slug)
}

func TestTryAdvanceRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	// tryAdvance now takes changeRef{Slug}; passing an invalid slug
	// should fail because there's no matching change state file.
	_, err := tryAdvance(t.TempDir(), changeRef{
		Slug: "nonexistent-slug",
	})
	require.Error(t, err)
}
