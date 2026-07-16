package cmd

import (
	"errors"
	"testing"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/signalridge/slipway/internal/runstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsCLIErrorReportsMutationCommitStateWithoutRetry(t *testing.T) {
	tests := []struct {
		name string
		err  *runstore.MutationError
		code string
	}{
		{
			name: "projection stale",
			err: &runstore.MutationError{
				Phase:           runstore.PhaseProjectionRename,
				Committed:       true,
				ProjectionStale: true,
				Err:             errors.New("rename fault"),
			},
			code: "mutation_committed_projection_stale",
		},
		{
			name: "inode write ambiguous",
			err: &runstore.MutationError{
				Phase:     runstore.PhaseJournalSync,
				Ambiguous: true,
				Err:       errors.New("sync fault"),
			},
			code: "mutation_outcome_ambiguous",
		},
		{
			name: "precommit failure",
			err: &runstore.MutationError{
				Phase: runstore.PhaseProjectionEncode,
				Err:   errors.New("encode fault"),
			},
			code: "mutation_not_committed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := asCLIError(test.err)
			require.NotNil(t, actual)
			assert.Equal(t, test.code, actual.Code)
			if test.err.Committed || test.err.Ambiguous {
				assert.Equal(t, autopilot.NextOperationCommand, actual.Next.Operation)
				require.Len(t, actual.Next.Variants, 1)
				assert.Equal(t, "inspect-runs", actual.Next.Variants[0].ID)
				assert.Contains(t, actual.Next.Variants[0].BaseArgv, "status")
			} else {
				assert.Equal(t, autopilot.NextOperationNone, actual.Next.Operation)
				assert.Empty(t, actual.Next.Variants)
			}
			assert.Equal(t, test.err.Committed, actual.Details["committed"])
			assert.Equal(t, test.err.ProjectionStale, actual.Details["projection_stale"])
			if test.err.Committed || test.err.Ambiguous {
				assert.Contains(t, actual.Message, "do not retry blindly")
			}
		})
	}
}

// TestAsCLIErrorJournalRecordLimitOffersRecoverableNext covers issue #434 §1.3:
// a journal-record-limit failure on Submit does not kill the persistent Run,
// so the error must offer a recoverable read-only inspection command rather
// than a terminal `none`.
func TestAsCLIErrorJournalRecordLimitOffersRecoverableNext(t *testing.T) {
	for _, message := range []string{
		"encode event data: payload exceeds journal record limit",
		"append journal: event exceeds 4194304 bytes",
	} {
		actual := asCLIError(errors.New(message))
		require.NotNil(t, actual)
		assert.Equal(t, "journal_record_too_large", actual.Code)
		assert.Equal(t, autopilot.NextOperationCommand, actual.Next.Operation, "limit error must not surface as terminal none")
		require.NotEmpty(t, actual.Next.Variants)
		assert.Equal(t, "inspect-runs", actual.Next.Variants[0].ID)
		assert.Contains(t, actual.Next.Variants[0].BaseArgv, "status")
	}
}
