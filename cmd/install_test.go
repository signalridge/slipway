package cmd

import (
	"errors"
	"testing"

	"github.com/signalridge/slipway/internal/adapter"
	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapterMutationErrorPreservesReportAndDisablesBlindRetry(t *testing.T) {
	t.Parallel()
	report := adapter.ChangeReport{
		Hosts:     []string{"claude"},
		Written:   []string{".claude/skills/slipway-run/SKILL.md"},
		Preserved: []string{".claude/slipway/recovery-file"},
		Warnings:  []string{"rollback preserved a concurrent edit"},
	}

	cliErr := adapterMutationError("install_failed", errors.New("transaction failed"), "/workspace", report)
	assert.Equal(t, autopilot.NextOperationNone, cliErr.Next.Operation)
	assert.Empty(t, cliErr.Next.Variants)
	require.NotNil(t, cliErr.Details)
	encodedReport, ok := cliErr.Details["report"].(changeReportOutput)
	require.True(t, ok)
	assert.Equal(t, makeChangeReportOutput(report), encodedReport)
}
