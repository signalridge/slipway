package coverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gatePkg = "github.com/signalridge/slipway/internal/engine/gate"

// TestParseProfileUnionDedup is the central correctness test: a block that
// appears once per test binary (as -coverpkg produces) must be counted once and
// covered if any occurrence hit. Naive summing would report Total=7 here.
func TestParseProfileUnionDedup(t *testing.T) {
	profile := strings.Join([]string{
		"mode: set",
		gatePkg + "/a.go:1.1,2.1 2 1", // covered in binary A
		gatePkg + "/a.go:1.1,2.1 2 0", // same block, binary B did not hit it
		gatePkg + "/a.go:3.1,4.1 3 0", // never covered
	}, "\n")

	got, err := ParseProfile(strings.NewReader(profile))
	require.NoError(t, err)

	s := got[gatePkg]
	assert.Equal(t, 5, s.Total, "duplicate block must be counted once (2+3, not 2+2+3)")
	assert.Equal(t, 2, s.Covered, "block is covered because one occurrence hit it")
	assert.InDelta(t, 40.0, s.Percent(), 0.001)
}

func TestParseProfileAggregatesPerPackage(t *testing.T) {
	govPkg := "github.com/signalridge/slipway/internal/engine/governance"
	profile := strings.Join([]string{
		"mode: set",
		gatePkg + "/a.go:1.1,2.1 4 1",
		gatePkg + "/b.go:1.1,2.1 1 0",
		govPkg + "/c.go:1.1,2.1 10 1",
	}, "\n")

	got, err := ParseProfile(strings.NewReader(profile))
	require.NoError(t, err)

	assert.Equal(t, Stmts{Covered: 4, Total: 5}, got[gatePkg])
	assert.Equal(t, Stmts{Covered: 10, Total: 10}, got[govPkg])
}

func TestParseProfileMalformed(t *testing.T) {
	_, err := ParseProfile(strings.NewReader("mode: set\n" + gatePkg + "/a.go:1.1,2.1 notanumber 1\n"))
	require.Error(t, err)
}

func TestStmtsPercentEmptyIs100(t *testing.T) {
	assert.Equal(t, 100.0, Stmts{}.Percent(), "an empty package can never trip the gate")
	assert.Equal(t, 66.7, Stmts{Covered: 2, Total: 3}.Percent(), "rounds to one decimal")
}

func TestSelectPackagesExcludeWinsAndMatchesSubpackages(t *testing.T) {
	all := map[string]Stmts{
		gatePkg:               {Covered: 1, Total: 2},
		gatePkg + "/internal": {Covered: 1, Total: 1}, // subpackage matches the prefix
		"github.com/x/other":  {Covered: 1, Total: 1}, // not included
	}
	include := []string{gatePkg}
	exclude := []string{gatePkg + "/internal"}

	got := SelectPackages(all, include, exclude)
	_, hasGate := got[gatePkg]
	_, hasSub := got[gatePkg+"/internal"]
	_, hasOther := got["github.com/x/other"]
	assert.True(t, hasGate, "included package is gated")
	assert.False(t, hasSub, "excluded subpackage is not gated even though it matches include")
	assert.False(t, hasOther, "package outside the include set is not gated")
}

func TestCheckPassesAtOrAboveFloor(t *testing.T) {
	b := Baseline{Packages: map[string]float64{gatePkg: 80.0}}
	// Exactly at floor and above floor both pass.
	assert.Empty(t, b.Check(map[string]Stmts{gatePkg: {Covered: 80, Total: 100}}))
	assert.Empty(t, b.Check(map[string]Stmts{gatePkg: {Covered: 90, Total: 100}}))
}

// TestCheckFailsClosedOnDrop is the fail-closed (RED) test: a coverage drop
// below the committed floor must surface a regression.
func TestCheckFailsClosedOnDrop(t *testing.T) {
	b := Baseline{Packages: map[string]float64{gatePkg: 80.0}}
	regs := b.Check(map[string]Stmts{gatePkg: {Covered: 79, Total: 100}})
	require.Len(t, regs, 1)
	assert.Equal(t, gatePkg, regs[0].Package)
	assert.False(t, regs[0].Missing)
	assert.InDelta(t, 79.0, regs[0].Current, 0.001)
}

// TestCheckFailsClosedOnMissingPackage ensures the gate never silently passes
// when a baselined package is absent from the measured profile.
func TestCheckFailsClosedOnMissingPackage(t *testing.T) {
	b := Baseline{Packages: map[string]float64{gatePkg: 80.0}}
	regs := b.Check(map[string]Stmts{}) // package disappeared from the profile
	require.Len(t, regs, 1)
	assert.True(t, regs[0].Missing)
}

func TestBuildBaselineComputesFloorsForSelected(t *testing.T) {
	all := map[string]Stmts{
		gatePkg:              {Covered: 3, Total: 4}, // 75.0
		"github.com/x/other": {Covered: 1, Total: 1},
	}
	b := BuildBaseline("covergate", all, []string{gatePkg}, nil)
	assert.Equal(t, []string{gatePkg}, b.CoverPackages)
	require.Contains(t, b.Packages, gatePkg)
	assert.InDelta(t, 75.0, b.Packages[gatePkg], 0.001)
	assert.NotContains(t, b.Packages, "github.com/x/other")
}

func TestBaselineIntegrityHelpers(t *testing.T) {
	govPkg := "github.com/signalridge/slipway/internal/engine/governance"
	progPkg := "github.com/signalridge/slipway/internal/engine/progression"
	required := []string{gatePkg, govPkg, progPkg}
	b := Baseline{
		CoverPackages: []string{gatePkg, govPkg},
		Exclude:       []string{govPkg},
		Packages: map[string]float64{
			gatePkg: 80.0,
		},
	}

	assert.Equal(t, []string{progPkg}, b.MissingCoverPackages(required))
	assert.Equal(t, []string{govPkg, progPkg}, b.MissingFloors(required))
	assert.Equal(t, []string{govPkg}, b.ExcludedRequiredPackages(required))
}

func TestBaselineRoundTrip(t *testing.T) {
	b := BuildBaseline("covergate", map[string]Stmts{gatePkg: {Covered: 1, Total: 2}}, []string{gatePkg}, nil)
	data, err := b.MarshalIndented()
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(data), "\n"))

	dir := t.TempDir()
	path := filepath.Join(dir, "coverage-baseline.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	loaded, err := LoadBaseline(path)
	require.NoError(t, err)
	assert.Equal(t, b.Packages, loaded.Packages)
}

func TestLoadBaselineMissingFileFailsClosed(t *testing.T) {
	_, err := LoadBaseline(filepath.Join(t.TempDir(), "does-not-exist.json"))
	require.Error(t, err, "a missing baseline must error, never pass silently")
}

func TestLoadBaselineEmptyPackagesRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coverage-baseline.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"tool":"covergate","packages":{}}`), 0o600))
	_, err := LoadBaseline(path)
	require.Error(t, err)
}
