package perfbaseline

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaselineRoundTrip(t *testing.T) {
	t.Parallel()

	b := sampleBaseline()
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, b))

	got, err := Read(&buf)
	require.NoError(t, err)
	assert.Equal(t, b.SchemaVersion, got.SchemaVersion)
	assert.Equal(t, b.Fixture.WorktreeCount, got.Fixture.WorktreeCount)
	assert.Equal(t, RequiredCommandIDs(), commandIDs(got.Commands))
}

func TestComparePassesWithinThreshold(t *testing.T) {
	t.Parallel()

	baseline := sampleBaseline()
	current := sampleBaseline()
	current.Commands[0].RealMS = 129

	regressions, err := Compare(baseline, current, 0.30)
	require.NoError(t, err)
	assert.Empty(t, regressions)
}

func TestCompareReportsCommandSpecificRegression(t *testing.T) {
	t.Parallel()

	baseline := sampleBaseline()
	current := sampleBaseline()
	current.Commands[0].RealMS = 131
	current.Commands[2].RealMS = 92

	regressions, err := Compare(baseline, current, 0.30)
	require.NoError(t, err)
	require.Len(t, regressions, 2)
	assert.Equal(t, "bound-next-json-diagnostics", regressions[0].CommandID)
	assert.Equal(t, "root-status-json", regressions[1].CommandID)
	assert.InDelta(t, 130, regressions[1].AllowedMS, 0.001)
}

func TestCompareRejectsMissingCurrentCommand(t *testing.T) {
	t.Parallel()

	baseline := sampleBaseline()
	current := sampleBaseline()
	current.Commands = current.Commands[:len(current.Commands)-1]

	_, err := Compare(baseline, current, 0.30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current measurement missing command")
}

func TestReadRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := Read(strings.NewReader(`{
		"schema_version": 1,
		"generated_at": "2026-06-27T00:00:00Z",
		"git_commit": "abc",
		"go_version": "go1.26",
		"slipway_binary": "./slipway",
		"regression_budget": 0.3,
		"fixture": {
			"root": "/tmp/fixture",
			"worktree_count": 25,
			"change_yaml_count": 300,
			"verification_count": 100,
			"bound_change_slug": "bound",
			"explicit_change_slug": "explicit"
		},
		"commands": [
			{
				"id": "root-status-json",
				"description": "root status",
				"cwd": "/tmp/fixture",
				"args": ["status", "--json"],
				"real_ms": 100,
				"user_ms": 20,
				"system_ms": 10,
				"exit_code": 0
			}
		],
		"unexpected": true
	}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestValidateRequiresSampleMetadata(t *testing.T) {
	t.Parallel()

	b := sampleBaseline()
	b.WarmupCount = -1
	assert.ErrorContains(t, b.Validate(), "warmup_count must be non-negative")

	b = sampleBaseline()
	b.SampleCount = 0
	assert.ErrorContains(t, b.Validate(), "sample_count must be positive")

	b = sampleBaseline()
	b.SampleStatistic = ""
	assert.ErrorContains(t, b.Validate(), "sample_statistic is required")

	b = sampleBaseline()
	b.CheckAttempts = 0
	assert.ErrorContains(t, b.Validate(), "check_attempts must be positive")
}

func TestMissingRequiredCommands(t *testing.T) {
	t.Parallel()

	b := sampleBaseline()
	b.Commands = b.Commands[:2]
	assert.Equal(t, RequiredCommandIDs()[2:], MissingRequiredCommands(b))
}

func sampleBaseline() Baseline {
	b := NewBaseline()
	b.GeneratedAt = time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	b.GitCommit = "abc123"
	b.GoVersion = "go1.26.4"
	b.SlipwayBinary = "/tmp/slipway"
	b.WarmupCount = 1
	b.SampleCount = 7
	b.SampleStatistic = "fastest"
	b.CheckAttempts = 3
	b.Fixture = Fixture{
		Root:               "/tmp/fixture",
		WorktreeCount:      25,
		ChangeYAMLCount:    300,
		VerificationCount:  100,
		BoundChangeSlug:    "bound-change",
		ExplicitChangeSlug: "explicit-change",
	}
	b.Commands = []CommandMeasure{
		{
			ID:          "root-status-json",
			Description: "root status",
			CWD:         "/tmp/fixture",
			Args:        []string{"status", "--json"},
			RealMS:      100,
			UserMS:      20,
			SystemMS:    10,
		},
		{
			ID:          "bound-status-json",
			Description: "bound status",
			CWD:         "/tmp/fixture/.worktrees/bound-change",
			Args:        []string{"status", "--json"},
			RealMS:      80,
			UserMS:      18,
			SystemMS:    9,
		},
		{
			ID:          "bound-next-json-diagnostics",
			Description: "bound next",
			CWD:         "/tmp/fixture/.worktrees/bound-change",
			Args:        []string{"next", "--json", "--diagnostics"},
			RealMS:      70,
			UserMS:      17,
			SystemMS:    8,
		},
		{
			ID:          "bound-validate-json",
			Description: "bound validate",
			CWD:         "/tmp/fixture/.worktrees/bound-change",
			Args:        []string{"validate"},
			RealMS:      60,
			UserMS:      16,
			SystemMS:    7,
		},
		{
			ID:          "explicit-change-status-json",
			Description: "explicit status",
			CWD:         "/tmp/fixture",
			Args:        []string{"status", "--json", "--change", "explicit-change"},
			RealMS:      50,
			UserMS:      15,
			SystemMS:    6,
		},
	}
	return b
}

func commandIDs(commands []CommandMeasure) []string {
	ids := make([]string, 0, len(commands))
	for _, cmd := range commands {
		ids = append(ids, cmd.ID)
	}
	return ids
}
