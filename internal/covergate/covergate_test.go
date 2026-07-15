package covergate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPackage = "github.com/signalridge/slipway/internal/autopilot"

func TestParseProfileUsesUnionSemantics(t *testing.T) {
	t.Parallel()
	profile := strings.Join([]string{
		"mode: set",
		testPackage + "/a.go:1.1,2.1 2 1",
		testPackage + "/a.go:1.1,2.1 2 0",
		testPackage + "/a.go:3.1,4.1 3 0",
	}, "\n")

	got, err := ParseProfile(strings.NewReader(profile))
	require.NoError(t, err)
	assert.Equal(t, Statements{Covered: 2, Total: 5}, got[testPackage])
	assert.Equal(t, 40.0, got[testPackage].Percent())
}

func TestParseProfileRejectsInconsistentDuplicate(t *testing.T) {
	t.Parallel()
	profile := strings.Join([]string{
		"mode: set",
		testPackage + "/a.go:1.1,2.1 2 1",
		testPackage + "/a.go:1.1,2.1 3 1",
	}, "\n")

	_, err := ParseProfile(strings.NewReader(profile))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inconsistent statement counts")
}

func TestBaselineValidationFailsClosed(t *testing.T) {
	t.Parallel()
	required := []string{testPackage, "github.com/signalridge/slipway/cmd"}

	tests := []struct {
		name     string
		baseline Baseline
		want     string
	}{
		{
			name:     "missing package declaration",
			baseline: Baseline{Tool: "covergate", CoverPackages: []string{testPackage}, Packages: map[string]float64{testPackage: 80}},
			want:     "cover_packages must exactly match",
		},
		{
			name: "missing floor",
			baseline: Baseline{
				Tool:          "covergate",
				CoverPackages: required,
				Packages:      map[string]float64{testPackage: 80},
			},
			want: "floors must exactly match",
		},
		{
			name: "invalid floor",
			baseline: Baseline{
				Tool:          "covergate",
				CoverPackages: required,
				Packages:      map[string]float64{testPackage: 101, required[1]: 80},
			},
			want: "outside 0..100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.baseline.Validate(required)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestBaselineCheckReportsDropAndMissingPackage(t *testing.T) {
	t.Parallel()
	missing := "github.com/signalridge/slipway/internal/runstore"
	baseline := Baseline{Packages: map[string]float64{testPackage: 80, missing: 70}}
	regressions := baseline.Check(map[string]Statements{testPackage: {Covered: 79, Total: 100}})
	require.Len(t, regressions, 2)
	assert.Contains(t, regressions[0].String()+regressions[1].String(), "79.0% < baseline 80.0%")
	assert.Contains(t, regressions[0].String()+regressions[1].String(), "absent from the coverage profile")
}

func TestLoadBaselineMissingFileFails(t *testing.T) {
	t.Parallel()
	_, err := LoadBaseline(filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)
}
