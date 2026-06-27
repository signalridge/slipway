package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/perfbaseline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareFixtureDoesNotCreateOrphanChangeBundles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	binary := filepath.Join(root, binaryName())
	require.NoError(t, os.WriteFile(binary, []byte("test binary"), 0o755))

	opts := options{
		worktrees:     2,
		changes:       3,
		verifications: 8,
	}
	require.NoError(t, prepareFixture(root, binary, opts))

	changeDirs, err := os.ReadDir(filepath.Join(root, "artifacts", "changes"))
	require.NoError(t, err)
	for _, entry := range changeDirs {
		if !entry.IsDir() || entry.Name() == "archived" {
			continue
		}
		assert.FileExists(t, filepath.Join(root, "artifacts", "changes", entry.Name(), "change.yaml"))
	}
	assert.FileExists(t, filepath.Join(
		root,
		"artifacts",
		"changes",
		"perf-explicit-change",
		"verification",
		"skill-001.yaml",
	))
	assert.FileExists(t, filepath.Join(
		root,
		".worktrees",
		"perf-bound-change",
		"artifacts",
		"changes",
		"perf-bound-change",
		"verification",
		"skill-000.yaml",
	))
	assert.NoDirExists(t, filepath.Join(root, "artifacts", "changes", "perf-bound-change"))
	assert.NoDirExists(t, filepath.Join(root, "artifacts", "changes", "perf-change-001"))
}

func TestEnsureVerificationRecordsRequiresGeneratedSlugs(t *testing.T) {
	t.Parallel()

	err := ensureVerificationRecords(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one generated change slug")
}

func TestParseOptionsRequiresExplicitChangeCapacity(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{"-changes", "1"}, io.Discard, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2")
}

func TestParseOptionsRequiresPositiveCheckAttempts(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{"-check-attempts", "0"}, io.Discard, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check-attempts")
}

func TestParseOptionsRequiresCheckOutput(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{"-mode", "check"}, io.Discard, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-out is required in check mode")
}

func TestRunMeasuredRecordsFastestSample(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	countFile := filepath.Join(root, "count")
	measured, err := runMeasured(os.Args[0], commandMeasureForTest(root, countFile), 0, 3)
	require.NoError(t, err)
	assert.Greater(t, measured.RealMS, 0.0)

	raw, err := os.ReadFile(countFile)
	require.NoError(t, err)
	assert.Equal(t, "3", string(raw))
}

func TestFastestMeasurementSelectsLowestRealTime(t *testing.T) {
	t.Parallel()

	fastest := fastestMeasurement([]perfbaseline.CommandMeasure{
		{ID: "slow", RealMS: 150},
		{ID: "fast", RealMS: 10},
		{ID: "middle", RealMS: 50},
	})

	assert.Equal(t, "fast", fastest.ID)
}

func commandMeasureForTest(root, countFile string) perfbaseline.CommandMeasure {
	return perfbaseline.CommandMeasure{
		ID:   "test-measure",
		CWD:  root,
		Args: []string{"-test.run=TestMeasureHelperProcess", "--", countFile},
	}
}

func TestMeasureHelperProcess(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator == -1 || separator+1 >= len(os.Args) {
		return
	}

	countFile := os.Args[separator+1]
	count := 0
	if raw, err := os.ReadFile(countFile); err == nil {
		parsed, err := strconv.Atoi(string(raw))
		if err == nil {
			count = parsed
		}
	}
	count++
	if err := os.WriteFile(countFile, []byte(strconv.Itoa(count)), 0o600); err != nil {
		os.Exit(1)
	}
	if count == 1 {
		time.Sleep(150 * time.Millisecond)
	} else {
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(0)
}
