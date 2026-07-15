package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/covergate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithOptionsChecksCoverage(t *testing.T) {
	t.Parallel()
	profile := profileForRequiredPackages(10, 10)
	baseline, err := covergate.BuildBaseline(mustParseProfile(t, profile), requiredPackages)
	require.NoError(t, err)

	directory := t.TempDir()
	profilePath := writeTestFile(t, directory, "coverage.out", profile)
	baselinePath := writeBaseline(t, directory, baseline)
	var stdout bytes.Buffer
	require.NoError(t, runWithOptions(options{check: true, profile: profilePath, baseline: baselinePath, stdout: &stdout}))
	assert.Contains(t, stdout.String(), "coverage gate passed")
}

func TestRunWithOptionsFailsOnRegression(t *testing.T) {
	t.Parallel()
	baselineProfile := profileForRequiredPackages(10, 10)
	baseline, err := covergate.BuildBaseline(mustParseProfile(t, baselineProfile), requiredPackages)
	require.NoError(t, err)
	currentProfile := profileForRequiredPackages(9, 10)

	directory := t.TempDir()
	profilePath := writeTestFile(t, directory, "coverage.out", currentProfile)
	baselinePath := writeBaseline(t, directory, baseline)
	var stdout bytes.Buffer
	err = runWithOptions(options{check: true, profile: profilePath, baseline: baselinePath, stdout: &stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "coverage gate failed")
	assert.Contains(t, stdout.String(), "90.0% < baseline 100.0%")
}

func TestRunWithOptionsWritesCompleteBaseline(t *testing.T) {
	t.Parallel()
	profile := profileForRequiredPackages(8, 10)
	directory := t.TempDir()
	profilePath := writeTestFile(t, directory, "coverage.out", profile)
	baselinePath := filepath.Join(directory, baselineFile)

	require.NoError(t, runWithOptions(options{write: true, profile: profilePath, baseline: baselinePath, goos: "linux"}))
	baseline, err := covergate.LoadBaseline(baselinePath)
	require.NoError(t, err)
	require.NoError(t, baseline.Validate(requiredPackages))
	assert.Len(t, baseline.Packages, len(requiredPackages))
}

func TestRunWithOptionsRejectsBaselineWritesOffLinux(t *testing.T) {
	t.Parallel()
	err := runWithOptions(options{write: true, goos: "darwin"})
	require.ErrorContains(t, err, "coverage baselines must be written on linux")
}

func TestRunWithOptionsRejectsInvalidMode(t *testing.T) {
	t.Parallel()
	err := runWithOptions(options{profile: filepath.Join(t.TempDir(), "missing")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "choose one of -check or -write")
}

func profileForRequiredPackages(covered, total int) string {
	lines := []string{"mode: set"}
	for index, pkg := range requiredPackages {
		lines = append(lines, coverageLine(pkg, index*2, covered, 1))
		if uncovered := total - covered; uncovered > 0 {
			lines = append(lines, coverageLine(pkg, index*2+1, uncovered, 0))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func coverageLine(pkg string, index, statements, count int) string {
	return pkg + "/file.go:" + strconv.Itoa(index+1) + ".1," + strconv.Itoa(index+2) + ".1 " + strconv.Itoa(statements) + " " + strconv.Itoa(count)
}

func mustParseProfile(t *testing.T, profile string) map[string]covergate.Statements {
	t.Helper()
	parsed, err := covergate.ParseProfile(strings.NewReader(profile))
	require.NoError(t, err)
	return parsed
}

func writeTestFile(t *testing.T, directory, name, content string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func writeBaseline(t *testing.T, directory string, baseline covergate.Baseline) string {
	t.Helper()
	data, err := baseline.MarshalIndented()
	require.NoError(t, err)
	return writeTestFile(t, directory, baselineFile, string(data))
}
