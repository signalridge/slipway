// Package coverage implements the governance-kernel coverage gate: it parses a
// Go coverage profile with union semantics, aggregates per-package statement
// coverage, and compares it against a committed baseline so CI fails closed on
// any regression. It mirrors the -check/-write contract of
// internal/toolgen/cmd/gen-surface-manifest.
package coverage

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

// Stmts is a union-deduplicated statement tally for a single package.
type Stmts struct {
	Covered int
	Total   int
}

// Percent returns the covered fraction as a percentage rounded to one decimal,
// the precision the baseline is stored and compared at. An empty package (no
// statements) is reported as 100% so it can never trip the gate.
func (s Stmts) Percent() float64 {
	if s.Total == 0 {
		return 100.0
	}
	return round1(100 * float64(s.Covered) / float64(s.Total))
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// ParseProfile reads a Go coverage profile and returns union-deduplicated
// statement tallies keyed by package import path.
//
// A profile produced with -coverpkg over a multi-package `go test` run contains
// the same code block once per test binary; each block is therefore counted
// once and treated as covered if ANY occurrence executed. Summing instead of
// unioning would inflate denominators and badly under-report coverage.
func ParseProfile(r io.Reader) (map[string]Stmts, error) {
	type block struct {
		numStmt int
		covered bool
	}
	blocks := map[string]*block{}
	pkgOf := map[string]string{}

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
		// Format: <import/path/file.go>:<startLine.col>,<endLine.col> <numStmt> <count>
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed coverage line: %q", line)
		}
		key := fields[0]
		numStmt, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("malformed statement count in %q: %w", line, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("malformed hit count in %q: %w", line, err)
		}
		colon := strings.LastIndex(key, ":")
		if colon < 0 {
			return nil, fmt.Errorf("malformed coverage block locator: %q", key)
		}
		pkgOf[key] = path.Dir(key[:colon])

		b := blocks[key]
		if b == nil {
			b = &block{numStmt: numStmt}
			blocks[key] = b
		}
		if count > 0 {
			b.covered = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading coverage profile: %w", err)
	}

	out := map[string]Stmts{}
	for key, b := range blocks {
		pkg := pkgOf[key]
		s := out[pkg]
		s.Total += b.numStmt
		if b.covered {
			s.Covered += b.numStmt
		}
		out[pkg] = s
	}
	return out, nil
}

// SelectPackages filters per-package tallies to those matched by an include
// prefix and not matched by an exclude prefix (exclude wins). A package matches
// a prefix when it equals the prefix or sits beneath it as a subpackage.
func SelectPackages(all map[string]Stmts, include, exclude []string) map[string]Stmts {
	out := map[string]Stmts{}
	for pkg, s := range all {
		if !matchesAny(pkg, include) {
			continue
		}
		if matchesAny(pkg, exclude) {
			continue
		}
		out[pkg] = s
	}
	return out
}

func matchesAny(pkg string, prefixes []string) bool {
	for _, p := range prefixes {
		if pkg == p || strings.HasPrefix(pkg, p+"/") {
			return true
		}
	}
	return false
}

func missingStrings(have []string, required []string) []string {
	seen := map[string]bool{}
	for _, item := range have {
		seen[item] = true
	}
	var missing []string
	for _, item := range required {
		if !seen[item] {
			missing = append(missing, item)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingFloatKeys(have map[string]float64, required []string) []string {
	var missing []string
	for _, item := range required {
		if _, ok := have[item]; !ok {
			missing = append(missing, item)
		}
	}
	sort.Strings(missing)
	return missing
}

// Baseline is the committed no-regression floor. Packages maps an import path to
// its minimum acceptable coverage percentage (one decimal). CoverPackages and
// Exclude record the gated set for transparency and for regenerating the
// baseline via -write.
type Baseline struct {
	Tool          string             `json:"tool"`
	CoverPackages []string           `json:"cover_packages"`
	Exclude       []string           `json:"exclude,omitempty"`
	Packages      map[string]float64 `json:"packages"`
}

// MissingFloors returns required packages that have no committed floor in the
// baseline. Callers use this to keep a declared gate from silently narrowing.
func (b Baseline) MissingFloors(required []string) []string {
	return missingFloatKeys(b.Packages, required)
}

// MissingCoverPackages returns required packages absent from the declared
// cover-package set.
func (b Baseline) MissingCoverPackages(required []string) []string {
	return missingStrings(b.CoverPackages, required)
}

// ExcludedRequiredPackages returns required packages hidden by the baseline's
// exclusion list.
func (b Baseline) ExcludedRequiredPackages(required []string) []string {
	var out []string
	for _, pkg := range required {
		if matchesAny(pkg, b.Exclude) {
			out = append(out, pkg)
		}
	}
	sort.Strings(out)
	return out
}

// BuildBaseline computes a baseline from a parsed profile using the include and
// exclude prefixes. Floors are the measured percentages (one decimal).
func BuildBaseline(tool string, all map[string]Stmts, include, exclude []string) Baseline {
	selected := SelectPackages(all, include, exclude)
	pkgs := map[string]float64{}
	for pkg, s := range selected {
		pkgs[pkg] = s.Percent()
	}
	inc := append([]string(nil), include...)
	exc := append([]string(nil), exclude...)
	sort.Strings(inc)
	sort.Strings(exc)
	return Baseline{Tool: tool, CoverPackages: inc, Exclude: exc, Packages: pkgs}
}

// Regression describes one package whose current coverage is below its floor, or
// whose coverage could not be measured at all (treated as a regression so the
// gate fails closed).
type Regression struct {
	Package  string
	Baseline float64
	Current  float64
	Missing  bool
}

func (r Regression) String() string {
	if r.Missing {
		return fmt.Sprintf("%s: baseline %.1f%% but package absent from coverage profile", r.Package, r.Baseline)
	}
	return fmt.Sprintf("%s: %.1f%% < baseline %.1f%%", r.Package, r.Current, r.Baseline)
}

// Check compares current per-package coverage against the baseline floors and
// returns every regression. A baselined package missing from the current
// measurement is a regression (fail closed): a gate must never silently pass
// because coverage data disappeared. Results are sorted by package for
// deterministic output.
func (b Baseline) Check(current map[string]Stmts) []Regression {
	var regs []Regression
	pkgs := make([]string, 0, len(b.Packages))
	for pkg := range b.Packages {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		floor := b.Packages[pkg]
		cur, ok := current[pkg]
		if !ok {
			regs = append(regs, Regression{Package: pkg, Baseline: floor, Missing: true})
			continue
		}
		if cur.Percent() < floor {
			regs = append(regs, Regression{Package: pkg, Baseline: floor, Current: cur.Percent()})
		}
	}
	return regs
}

// MarshalIndented emits the baseline as stable, human-reviewable JSON (sorted
// keys, two-space indent, trailing newline) so a baseline change is a clean diff.
func (b Baseline) MarshalIndented() ([]byte, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// LoadBaseline reads and parses a committed baseline file.
func LoadBaseline(path string) (Baseline, error) {
	var b Baseline
	data, err := os.ReadFile(path) // #nosec G304 -- path is the committed baseline file supplied by the gate.
	if err != nil {
		return b, fmt.Errorf("reading baseline %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return b, fmt.Errorf("parsing baseline %s: %w", path, err)
	}
	if len(b.Packages) == 0 {
		return b, fmt.Errorf("baseline %s declares no packages", path)
	}
	return b, nil
}
