// Command covergate is the governance-kernel coverage gate. It parses a Go
// coverage profile and either checks per-package coverage against a committed
// baseline (failing closed on any regression) or rewrites the baseline. It
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
)

// BaselineFile is the committed per-package coverage floor, at the repo root.
const BaselineFile = "coverage-baseline.json"

// kernelPackages is the governance kernel the gate protects: the gate,
// governance, and readiness/progression logic (the readiness resolver lives in
// internal/engine/progression).
var kernelPackages = []string{
	"github.com/signalridge/slipway/internal/engine/gate",
	"github.com/signalridge/slipway/internal/engine/governance",
	"github.com/signalridge/slipway/internal/engine/progression",
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
	profile  string
	baseline string
	exclude  []string
	stdout   io.Writer
}

func run() error {
	var (
		check    bool
		write    bool
		profile  string
		baseline string
		exclude  string
	)
	flag.BoolVar(&check, "check", false, "fail if any kernel package is below its committed baseline")
	flag.BoolVar(&write, "write", false, "rewrite the committed coverage baseline from the profile")
	flag.StringVar(&profile, "profile", "coverage.out", "path to the Go coverage profile")
	flag.StringVar(&baseline, "baseline", "", "path to the baseline file (default <repo-root>/"+BaselineFile+")")
	flag.StringVar(&exclude, "exclude", "", "comma-separated package import-path prefixes to exclude from gating")
	flag.Parse()

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	if baseline == "" {
		baseline = filepath.Join(repoRoot, BaselineFile)
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
		b := coverage.BuildBaseline("covergate", all, kernelPackages, opts.exclude)
		if err := validateKernelBaseline(b); err != nil {
			return fmt.Errorf("cannot write incomplete kernel coverage baseline from %s: %w", opts.profile, err)
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
			return fmt.Errorf("%w\nrun `go run ./internal/coverage/cmd/covergate -write -profile %s` to create it", err, opts.profile)
		}
		if err := validateKernelBaseline(b); err != nil {
			return fmt.Errorf("invalid coverage baseline %s: %w", opts.baseline, err)
		}
		regs := b.Check(all)
		if len(regs) == 0 {
			fmt.Fprintf(opts.stdout, "coverage gate passed: all %d kernel packages meet their baseline\n", len(b.Packages))
			for _, pkg := range sortedKeys(b.Packages) {
				cur := all[pkg]
				fmt.Fprintf(opts.stdout, "  %-60s %.1f%% (floor %.1f%%)\n", pkg, cur.Percent(), b.Packages[pkg])
			}
			return nil
		}
		for _, r := range regs {
			fmt.Fprintf(opts.stdout, "  %s\n", r.String())
		}
		fmt.Fprintf(opts.stdout, "\nAdd tests to restore coverage, or regenerate the baseline with `go run ./internal/coverage/cmd/covergate -write -profile %s` so the reviewed change shows in the diff (there is no skip path)\n", opts.profile)
		return fmt.Errorf("coverage gate failed: %d kernel package(s) below baseline", len(regs))

	default:
		panic("unreachable covergate mode")
	}
}

func validateKernelBaseline(b coverage.Baseline) error {
	if missing := b.MissingCoverPackages(kernelPackages); len(missing) > 0 {
		return fmt.Errorf("missing required kernel cover package(s): %s", strings.Join(missing, ", "))
	}
	if excluded := b.ExcludedRequiredPackages(kernelPackages); len(excluded) > 0 {
		return fmt.Errorf("excludes required kernel package(s): %s", strings.Join(excluded, ", "))
	}
	if missing := b.MissingFloors(kernelPackages); len(missing) > 0 {
		return fmt.Errorf("missing required kernel package floor(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find repository root containing go.mod")
		}
		dir = parent
	}
}
