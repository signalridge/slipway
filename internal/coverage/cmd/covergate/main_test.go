package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	coveragepkg "github.com/signalridge/slipway/internal/coverage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithOptionsCheckModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		profile      string
		baseline     coveragepkg.Baseline
		wantErr      string
		wantStdout   string
		wantNoStdout string
	}{
		{
			name: "passes at floor",
			profile: profileWithBlocks(
				block{pkg: kernelPackages[0], covered: 8, total: 10},
				block{pkg: kernelPackages[1], covered: 9, total: 10},
				block{pkg: kernelPackages[2], covered: 10, total: 10},
			),
			baseline: baselineWithFloors(map[string]float64{
				kernelPackages[0]: 80.0,
				kernelPackages[1]: 90.0,
				kernelPackages[2]: 100.0,
			}),
			wantStdout: "coverage gate passed: all 3 kernel packages meet their baseline",
		},
		{
			name: "fails below floor",
			profile: profileWithBlocks(
				block{pkg: kernelPackages[0], covered: 7, total: 10},
				block{pkg: kernelPackages[1], covered: 9, total: 10},
				block{pkg: kernelPackages[2], covered: 10, total: 10},
			),
			baseline: baselineWithFloors(map[string]float64{
				kernelPackages[0]: 80.0,
				kernelPackages[1]: 90.0,
				kernelPackages[2]: 100.0,
			}),
			wantErr:      "coverage gate failed: 1 kernel package(s) below baseline",
			wantStdout:   kernelPackages[0] + ": 70.0% < baseline 80.0%",
			wantNoStdout: "coverage gate passed",
		},
		{
			name: "fails when baselined package missing",
			profile: profileWithBlocks(
				block{pkg: kernelPackages[0], covered: 10, total: 10},
				block{pkg: kernelPackages[2], covered: 10, total: 10},
			),
			baseline:   baselineWithFloors(nil),
			wantErr:    "coverage gate failed: 1 kernel package(s) below baseline",
			wantStdout: "package absent from coverage profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			profilePath := writeFile(t, dir, "coverage.out", tt.profile)
			baselinePath := filepath.Join(dir, BaselineFile)
			writeBaseline(t, baselinePath, tt.baseline)

			var stdout bytes.Buffer
			err := runWithOptions(options{
				check:    true,
				profile:  profilePath,
				baseline: baselinePath,
				stdout:   &stdout,
			})

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
			assert.Contains(t, stdout.String(), tt.wantStdout)
			if tt.wantNoStdout != "" {
				assert.NotContains(t, stdout.String(), tt.wantNoStdout)
			}
		})
	}
}

func TestRunWithOptionsWriteRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profilePath := writeFile(t, dir, "coverage.out", profileWithBlocks(
		block{pkg: kernelPackages[0], covered: 10, total: 10},
		block{pkg: kernelPackages[1], covered: 9, total: 10},
		block{pkg: kernelPackages[2], covered: 8, total: 10},
	))
	baselinePath := filepath.Join(dir, BaselineFile)

	var stdout bytes.Buffer
	require.NoError(t, runWithOptions(options{
		write:    true,
		profile:  profilePath,
		baseline: baselinePath,
		stdout:   &stdout,
	}))
	assert.Contains(t, stdout.String(), "wrote "+baselinePath)

	b, err := coveragepkg.LoadBaseline(baselinePath)
	require.NoError(t, err)
	assert.Equal(t, []string{
		kernelPackages[0],
		kernelPackages[1],
		kernelPackages[2],
	}, b.CoverPackages)
	assert.Equal(t, map[string]float64{
		kernelPackages[0]: 100.0,
		kernelPackages[1]: 90.0,
		kernelPackages[2]: 80.0,
	}, b.Packages)

	stdout.Reset()
	require.NoError(t, runWithOptions(options{
		check:    true,
		profile:  profilePath,
		baseline: baselinePath,
		stdout:   &stdout,
	}))
	assert.Contains(t, stdout.String(), "coverage gate passed: all 3 kernel packages meet their baseline")
}

func TestRunWithOptionsWriteRejectsPartialKernelProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profilePath := writeFile(t, dir, "coverage.out", profileWithBlocks(
		block{pkg: kernelPackages[0], covered: 10, total: 10},
	))
	baselinePath := filepath.Join(dir, BaselineFile)

	err := runWithOptions(options{
		write:    true,
		profile:  profilePath,
		baseline: baselinePath,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot write incomplete kernel coverage baseline")
	assert.Contains(t, err.Error(), "missing required kernel package floor(s)")
	assert.NoFileExists(t, baselinePath)
}

func TestRunWithOptionsWriteRejectsRequiredKernelExclusion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profilePath := writeFile(t, dir, "coverage.out", profileWithBlocks(
		block{pkg: kernelPackages[0], covered: 10, total: 10},
		block{pkg: kernelPackages[1], covered: 10, total: 10},
		block{pkg: kernelPackages[2], covered: 10, total: 10},
	))
	baselinePath := filepath.Join(dir, BaselineFile)

	err := runWithOptions(options{
		write:    true,
		profile:  profilePath,
		baseline: baselinePath,
		exclude:  []string{kernelPackages[1]},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "excludes required kernel package(s)")
	assert.NoFileExists(t, baselinePath)
}

func TestRunWithOptionsCheckRejectsInvalidKernelBaseline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseline coveragepkg.Baseline
		wantErr  string
	}{
		{
			name: "missing required floors",
			baseline: coveragepkg.Baseline{
				Tool:          "covergate",
				CoverPackages: append([]string(nil), kernelPackages...),
				Packages: map[string]float64{
					kernelPackages[0]: 100.0,
				},
			},
			wantErr: "missing required kernel package floor(s)",
		},
		{
			name: "missing cover packages declaration",
			baseline: coveragepkg.Baseline{
				Tool: "covergate",
				Packages: map[string]float64{
					kernelPackages[0]: 100.0,
					kernelPackages[1]: 100.0,
					kernelPackages[2]: 100.0,
				},
			},
			wantErr: "missing required kernel cover package(s)",
		},
		{
			name: "excluded required package",
			baseline: coveragepkg.Baseline{
				Tool:          "covergate",
				CoverPackages: append([]string(nil), kernelPackages...),
				Exclude:       []string{kernelPackages[2]},
				Packages: map[string]float64{
					kernelPackages[0]: 100.0,
					kernelPackages[1]: 100.0,
					kernelPackages[2]: 100.0,
				},
			},
			wantErr: "excludes required kernel package(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			profilePath := writeFile(t, dir, "coverage.out", profileWithBlocks(
				block{pkg: kernelPackages[0], covered: 10, total: 10},
				block{pkg: kernelPackages[1], covered: 10, total: 10},
				block{pkg: kernelPackages[2], covered: 10, total: 10},
			))
			baselinePath := filepath.Join(dir, BaselineFile)
			writeBaseline(t, baselinePath, tt.baseline)

			err := runWithOptions(options{
				check:    true,
				profile:  profilePath,
				baseline: baselinePath,
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid coverage baseline")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRunWithOptionsRejectsInvalidModesBeforeReadingProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    options
		wantErr string
	}{
		{
			name:    "missing mode",
			opts:    options{profile: filepath.Join(t.TempDir(), "missing.out")},
			wantErr: "choose one of -check or -write",
		},
		{
			name: "check and write together",
			opts: options{
				check:   true,
				write:   true,
				profile: filepath.Join(t.TempDir(), "missing.out"),
			},
			wantErr: "choose only one of -check or -write",
		},
		{
			name: "check with exclude",
			opts: options{
				check:   true,
				exclude: []string{kernelPackages[0]},
				profile: filepath.Join(t.TempDir(), "missing.out"),
			},
			wantErr: "-exclude is only valid with -write",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := runWithOptions(tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.NotContains(t, err.Error(), "open coverage profile")
		})
	}
}

type block struct {
	pkg     string
	covered int
	total   int
}

func profileWithBlocks(blocks ...block) string {
	lines := []string{"mode: set"}
	for i, b := range blocks {
		if b.covered > 0 {
			lines = append(lines, coverageLine(b.pkg, i, b.covered, 1))
		}
		if uncovered := b.total - b.covered; uncovered > 0 {
			lines = append(lines, coverageLine(b.pkg, i+len(blocks), uncovered, 0))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func coverageLine(pkg string, id int, stmtCount int, hitCount int) string {
	return pkg + "/file.go:" + strconv.Itoa(id+1) + ".1," + strconv.Itoa(id+2) + ".1 " +
		strconv.Itoa(stmtCount) + " " + strconv.Itoa(hitCount)
}

func baselineWithFloors(floors map[string]float64) coveragepkg.Baseline {
	packages := map[string]float64{}
	for _, pkg := range kernelPackages {
		packages[pkg] = 100.0
	}
	for pkg, floor := range floors {
		packages[pkg] = floor
	}
	return coveragepkg.Baseline{
		Tool:          "covergate",
		CoverPackages: append([]string(nil), kernelPackages...),
		Packages:      packages,
	}
}

func writeFile(t *testing.T, dir string, name string, contents string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func writeBaseline(t *testing.T, path string, b coveragepkg.Baseline) {
	t.Helper()

	data, err := b.MarshalIndented()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
}
