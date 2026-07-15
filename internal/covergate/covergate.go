// Package covergate implements the repository's per-package coverage regression gate.
package covergate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Statements is a union-deduplicated statement tally for one package.
type Statements struct {
	Covered int
	Total   int
}

// Percent returns coverage rounded to the one-decimal precision used by baselines.
func (s Statements) Percent() float64 {
	if s.Total == 0 {
		return 100
	}
	return math.Round((100*float64(s.Covered)/float64(s.Total))*10) / 10
}

// ParseProfile parses a Go coverage profile using union semantics. Profiles
// produced with -coverpkg repeat instrumented blocks once per test binary; a
// repeated block is covered when any occurrence executed it.
func ParseProfile(r io.Reader) (map[string]Statements, error) {
	type block struct {
		statements int
		covered    bool
		pkg        string
	}

	blocks := make(map[string]block)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed coverage line: %q", line)
		}
		statements, err := strconv.Atoi(fields[1])
		if err != nil || statements < 0 {
			return nil, fmt.Errorf("malformed statement count in %q", line)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil || count < 0 {
			return nil, fmt.Errorf("malformed hit count in %q", line)
		}
		locator := fields[0]
		colon := strings.LastIndex(locator, ":")
		if colon < 0 {
			return nil, fmt.Errorf("malformed coverage block locator: %q", locator)
		}
		pkg := path.Dir(locator[:colon])
		current, exists := blocks[locator]
		if exists && current.statements != statements {
			return nil, fmt.Errorf("coverage block %q has inconsistent statement counts", locator)
		}
		current.statements = statements
		current.pkg = pkg
		current.covered = current.covered || count > 0
		blocks[locator] = current
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading coverage profile: %w", err)
	}

	result := make(map[string]Statements)
	for _, current := range blocks {
		tally := result[current.pkg]
		tally.Total += current.statements
		if current.covered {
			tally.Covered += current.statements
		}
		result[current.pkg] = tally
	}
	return result, nil
}

// Baseline is the reviewed no-regression floor for every required package.
type Baseline struct {
	Tool          string             `json:"tool"`
	CoverPackages []string           `json:"cover_packages"`
	Packages      map[string]float64 `json:"packages"`
}

// BuildBaseline creates an exact baseline for required packages and fails if
// the profile did not measure any of them.
func BuildBaseline(all map[string]Statements, required []string) (Baseline, error) {
	packages := make(map[string]float64, len(required))
	for _, pkg := range required {
		statements, ok := all[pkg]
		if !ok {
			return Baseline{}, fmt.Errorf("required package %s is absent from the coverage profile", pkg)
		}
		packages[pkg] = statements.Percent()
	}
	coverPackages := append([]string(nil), required...)
	sort.Strings(coverPackages)
	return Baseline{Tool: "covergate", CoverPackages: coverPackages, Packages: packages}, nil
}

// Validate requires the baseline package declaration and floor map to match the
// current gate exactly, preventing a baseline edit from silently narrowing it.
func (b Baseline) Validate(required []string) error {
	if b.Tool != "covergate" {
		return fmt.Errorf("unexpected baseline tool %q", b.Tool)
	}
	want := append([]string(nil), required...)
	sort.Strings(want)
	got := append([]string(nil), b.CoverPackages...)
	sort.Strings(got)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		return fmt.Errorf("cover_packages must exactly match required packages: got %v, want %v", got, want)
	}
	if len(b.Packages) != len(want) {
		return fmt.Errorf("baseline floors must exactly match required packages")
	}
	for _, pkg := range want {
		floor, ok := b.Packages[pkg]
		if !ok {
			return fmt.Errorf("baseline is missing required package floor %s", pkg)
		}
		if math.IsNaN(floor) || math.IsInf(floor, 0) || floor < 0 || floor > 100 {
			return fmt.Errorf("baseline floor for %s is outside 0..100", pkg)
		}
	}
	return nil
}

// Regression describes a missing package or a package below its reviewed floor.
type Regression struct {
	Package  string
	Baseline float64
	Current  float64
	Missing  bool
}

func (r Regression) String() string {
	if r.Missing {
		return fmt.Sprintf("%s: baseline %.1f%% but package is absent from the coverage profile", r.Package, r.Baseline)
	}
	return fmt.Sprintf("%s: %.1f%% < baseline %.1f%%", r.Package, r.Current, r.Baseline)
}

// Check returns every regression in deterministic package order.
func (b Baseline) Check(current map[string]Statements) []Regression {
	packages := make([]string, 0, len(b.Packages))
	for pkg := range b.Packages {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	var regressions []Regression
	for _, pkg := range packages {
		floor := b.Packages[pkg]
		statements, ok := current[pkg]
		if !ok {
			regressions = append(regressions, Regression{Package: pkg, Baseline: floor, Missing: true})
			continue
		}
		if statements.Percent() < floor {
			regressions = append(regressions, Regression{Package: pkg, Baseline: floor, Current: statements.Percent()})
		}
	}
	return regressions
}

// MarshalIndented returns stable, reviewable baseline JSON.
func (b Baseline) MarshalIndented() ([]byte, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// LoadBaseline reads a committed baseline file.
func LoadBaseline(file string) (Baseline, error) {
	var baseline Baseline
	data, err := os.ReadFile(file) // #nosec G304 -- the caller selects the committed baseline path.
	if err != nil {
		return baseline, fmt.Errorf("reading baseline %s: %w", file, err)
	}
	if err := json.Unmarshal(data, &baseline); err != nil {
		return baseline, fmt.Errorf("parsing baseline %s: %w", file, err)
	}
	return baseline, nil
}
