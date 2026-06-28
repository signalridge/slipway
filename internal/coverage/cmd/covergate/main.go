// Command covergate is the governed coverage gate. It parses a Go coverage
// profile and either checks per-package coverage against a committed baseline
// (failing closed on any regression) or rewrites the selected baseline. It
// mirrors the -check/-write contract of gen-surface-manifest.
//
// Usage:
//
//	go run ./internal/coverage/cmd/covergate -check -profile coverage.out
//	go run ./internal/coverage/cmd/covergate -write -profile coverage.out
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/signalridge/slipway/internal/coverage"
	"github.com/signalridge/slipway/internal/fsutil"
)

// BaselineFile is the committed kernel per-package coverage floor, at the repo root.
const BaselineFile = "coverage-baseline.json"

// PublicSurfaceBaselineFile is the committed high-risk public-surface coverage
// floor, at the repo root.
const PublicSurfaceBaselineFile = "coverage-public-surface-baseline.json"

// kernelPackages is the governance kernel the gate protects: the gate,
// governance, and readiness/progression logic (the readiness resolver lives in
// internal/engine/progression).
var kernelPackages = []string{
	"github.com/signalridge/slipway/internal/engine/gate",
	"github.com/signalridge/slipway/internal/engine/governance",
	"github.com/signalridge/slipway/internal/engine/progression",
}

// publicSurfacePackages is the tiered public-surface gate for high-risk
// lifecycle and state-authority paths.
var publicSurfacePackages = []string{
	"github.com/signalridge/slipway/cmd",
	"github.com/signalridge/slipway/internal/state",
}

type coverageTarget struct {
	ID           string
	Label        string
	BaselineFile string
	Packages     []string
	Surfaces     map[string][]coverage.Surface
}

var coverageTargets = map[string]coverageTarget{
	"kernel": {
		ID:           "kernel",
		Label:        "kernel",
		BaselineFile: BaselineFile,
		Packages:     kernelPackages,
	},
	"public-surface": {
		ID:           "public-surface",
		Label:        "public-surface",
		BaselineFile: PublicSurfaceBaselineFile,
		Packages:     publicSurfacePackages,
		Surfaces: map[string][]coverage.Surface{
			"github.com/signalridge/slipway/cmd": {
				{
					Name: "public lifecycle commands",
					Files: []string{
						"cmd/status.go",
						"cmd/next.go",
						"cmd/validate.go",
						"cmd/done.go",
						"cmd/evidence.go",
					},
				},
				{
					Name: "release, security, and workflow-adjacent helpers",
					Files: []string{
						"cmd/tool_actions.go",
						"cmd/tool_github.go",
						"cmd/tool_sarif.go",
						"cmd/release_workflow_contract_test.go",
					},
				},
			},
			"github.com/signalridge/slipway/internal/state": {
				{
					Name: "state verification authority",
					Files: []string{
						"internal/state/verification.go",
						"internal/state/evidence_digests.go",
					},
				},
				{
					Name: "worktree and runtime state authority",
					Files: []string{
						"internal/state/worktree.go",
						"internal/state/store.go",
						"internal/state/local_runtime_paths.go",
					},
				},
			},
		},
	},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type options struct {
	check    bool
	write    bool
	target   string
	profile  string
	baseline string
	exclude  []string
	stdout   io.Writer
}

func run() error {
	var (
		check    bool
		write    bool
		targetID string
		profile  string
		baseline string
		exclude  string
	)
	flag.BoolVar(&check, "check", false, "fail if any gated package is below its committed baseline")
	flag.BoolVar(&write, "write", false, "rewrite the selected committed coverage baseline from the profile")
	flag.StringVar(&targetID, "target", "kernel", "coverage target: kernel or public-surface")
	flag.StringVar(&profile, "profile", "coverage.out", "path to the Go coverage profile")
	flag.StringVar(&baseline, "baseline", "", "path to the baseline file (default depends on -target)")
	flag.StringVar(&exclude, "exclude", "", "comma-separated package import-path prefixes to exclude from gating")
	flag.Parse()

	target, err := resolveTarget(targetID)
	if err != nil {
		return err
	}
	repoRoot, err := fsutil.FindRepoRoot("")
	if err != nil {
		return err
	}
	if baseline == "" {
		baseline = filepath.Join(repoRoot, target.BaselineFile)
	}
	var excludes []string
	for _, e := range strings.Split(exclude, ",") {
		if e = strings.TrimSpace(e); e != "" {
			excludes = append(excludes, e)
		}
	}

	return runWithOptions(options{
		check:    check,
		write:    write,
		target:   target.ID,
		profile:  profile,
		baseline: baseline,
		exclude:  excludes,
		stdout:   os.Stdout,
	})
}

func runWithOptions(opts options) error {
	if opts.check && opts.write {
		return errors.New("choose only one of -check or -write")
	}
	if !opts.check && !opts.write {
		return errors.New("choose one of -check or -write")
	}
	if opts.check && len(opts.exclude) > 0 {
		return errors.New("-exclude is only valid with -write; -check uses the committed baseline")
	}
	if opts.stdout == nil {
		opts.stdout = io.Discard
	}
	target, err := resolveTarget(opts.target)
	if err != nil {
		return err
	}

	f, err := os.Open(opts.profile) // #nosec G304 -- profile path is supplied by the gate invocation.
	if err != nil {
		return fmt.Errorf("open coverage profile %s: %w (run the coverage job's `go test ... -coverprofile` first)", opts.profile, err)
	}
	defer func() { _ = f.Close() }()

	all, err := coverage.ParseProfile(f)
	if err != nil {
		return fmt.Errorf("parse coverage profile %s: %w", opts.profile, err)
	}

	switch {
	case opts.write:
		b := coverage.BuildBaseline("covergate", all, target.Packages, opts.exclude)
		applyTargetMetadata(&b, target)
		if err := validateTargetBaseline(target, b); err != nil {
			return fmt.Errorf("cannot write incomplete %s coverage baseline from %s: %w", target.Label, opts.profile, err)
		}
		data, err := b.MarshalIndented()
		if err != nil {
			return fmt.Errorf("encode baseline: %w", err)
		}
		if err := os.WriteFile(opts.baseline, data, 0o644); err != nil { // #nosec G306 -- committed project artifact.
			return fmt.Errorf("write baseline %s: %w", opts.baseline, err)
		}
		fmt.Fprintf(opts.stdout, "wrote %s\n", opts.baseline)
		for _, pkg := range sortedKeys(b.Packages) {
			fmt.Fprintf(opts.stdout, "  %-60s %.1f%%\n", pkg, b.Packages[pkg])
		}
		return nil

	case opts.check:
		b, err := coverage.LoadBaseline(opts.baseline)
		if err != nil {
			return fmt.Errorf("%w\nrun `go run ./internal/coverage/cmd/covergate -target %s -write -profile %s` to create it", err, target.ID, opts.profile)
		}
		if err := validateTargetBaseline(target, b); err != nil {
			return fmt.Errorf("invalid coverage baseline %s: %w", opts.baseline, err)
		}
		regs := b.Check(all)
		if len(regs) == 0 {
			fmt.Fprintf(opts.stdout, "coverage gate passed: all %d %s packages meet their baseline\n", len(b.Packages), target.Label)
			for _, pkg := range sortedKeys(b.Packages) {
				cur := all[pkg]
				fmt.Fprintf(opts.stdout, "  %-60s %.1f%% (floor %.1f%%)\n", pkg, cur.Percent(), b.Packages[pkg])
			}
			return nil
		}
		for _, r := range regs {
			fmt.Fprintf(opts.stdout, "  %s\n", r.String())
		}
		fmt.Fprintf(opts.stdout, "\nAdd tests to restore coverage, or regenerate the baseline with `go run ./internal/coverage/cmd/covergate -target %s -write -profile %s` so the reviewed change shows in the diff (there is no skip path)\n", target.ID, opts.profile)
		return fmt.Errorf("coverage gate failed: %d %s package(s) below baseline", len(regs), target.Label)

	default:
		panic("unreachable covergate mode")
	}
}

func resolveTarget(id string) (coverageTarget, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = "kernel"
	}
	target, ok := coverageTargets[id]
	if !ok {
		return coverageTarget{}, fmt.Errorf("unknown coverage target %q (expected one of: %s)", id, strings.Join(sortedTargetIDs(), ", "))
	}
	return target, nil
}

func sortedTargetIDs() []string {
	ids := make([]string, 0, len(coverageTargets))
	for id := range coverageTargets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func applyTargetMetadata(b *coverage.Baseline, target coverageTarget) {
	if target.ID != "kernel" {
		b.Target = target.ID
	}
	if len(target.Surfaces) > 0 {
		b.Surfaces = cloneSurfaces(target.Surfaces)
	}
}

func cloneSurfaces(in map[string][]coverage.Surface) map[string][]coverage.Surface {
	out := make(map[string][]coverage.Surface, len(in))
	for pkg, surfaces := range in {
		out[pkg] = append([]coverage.Surface(nil), surfaces...)
	}
	return out
}

func validateTargetBaseline(target coverageTarget, b coverage.Baseline) error {
	if missing := b.MissingCoverPackages(target.Packages); len(missing) > 0 {
		return fmt.Errorf(
			"missing required %s cover package(s): %s",
			target.Label,
			formatPackagesWithSurfaces(missing, target.Surfaces),
		)
	}
	if excluded := b.ExcludedRequiredPackages(target.Packages); len(excluded) > 0 {
		return fmt.Errorf(
			"excludes required %s package(s): %s",
			target.Label,
			formatPackagesWithSurfaces(excluded, target.Surfaces),
		)
	}
	if missing := b.MissingFloors(target.Packages); len(missing) > 0 {
		return fmt.Errorf(
			"missing required %s package floor(s): %s",
			target.Label,
			formatPackagesWithSurfaces(missing, target.Surfaces),
		)
	}
	if len(target.Surfaces) > 0 {
		if missing := missingRequiredSurfaces(target.Surfaces, b.Surfaces); len(missing) > 0 {
			return fmt.Errorf("missing required %s surface metadata: %s", target.Label, strings.Join(missing, ", "))
		}
	}
	return nil
}

func formatPackagesWithSurfaces(packages []string, surfacesByPackage map[string][]coverage.Surface) string {
	packages = append([]string(nil), packages...)
	sort.Strings(packages)

	parts := make([]string, 0, len(packages))
	for _, pkg := range packages {
		surfaces := surfacesByPackage[pkg]
		if len(surfaces) == 0 {
			parts = append(parts, pkg)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s [surfaces: %s]", pkg, formatSurfaceDiagnostics(surfaces)))
	}
	return strings.Join(parts, ", ")
}

func formatSurfaceDiagnostics(surfaces []coverage.Surface) string {
	parts := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		parts = append(parts, surface.String())
	}
	return strings.Join(parts, "; ")
}

func missingRequiredSurfaces(required, actual map[string][]coverage.Surface) []string {
	packages := make([]string, 0, len(required))
	for pkg := range required {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	var missing []string
	for _, pkg := range packages {
		for _, requiredSurface := range required[pkg] {
			if !hasSurface(actual[pkg], requiredSurface) {
				missing = append(missing, fmt.Sprintf("%s: %s", pkg, requiredSurface.String()))
			}
		}
	}
	return missing
}

func hasSurface(actual []coverage.Surface, required coverage.Surface) bool {
	for _, surface := range actual {
		if sameSurface(surface, required) {
			return true
		}
	}
	return false
}

func sameSurface(a, b coverage.Surface) bool {
	if a.Name != b.Name || len(a.Files) != len(b.Files) {
		return false
	}
	for i := range a.Files {
		if a.Files[i] != b.Files[i] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
