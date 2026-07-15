// Command covergate checks or writes the reviewed per-package coverage baseline.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/signalridge/slipway/internal/covergate"
)

const baselineFile = "coverage-baseline.json"

var requiredPackages = []string{
	"github.com/signalridge/slipway/cmd",
	"github.com/signalridge/slipway/internal/adapter",
	"github.com/signalridge/slipway/internal/autopilot",
	"github.com/signalridge/slipway/internal/fsutil",
	"github.com/signalridge/slipway/internal/jsonstrict",
	"github.com/signalridge/slipway/internal/recoverycmd",
	"github.com/signalridge/slipway/internal/runstore",
	"github.com/signalridge/slipway/internal/testlint",
	"github.com/signalridge/slipway/internal/tmpl",
}

type options struct {
	check    bool
	write    bool
	profile  string
	baseline string
	stdout   io.Writer
	goos     string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var opts options
	flag.BoolVar(&opts.check, "check", false, "fail if a required package is below its committed baseline")
	flag.BoolVar(&opts.write, "write", false, "rewrite the committed baseline from the supplied profile")
	flag.StringVar(&opts.profile, "profile", "coverage.out", "Go coverage profile")
	flag.StringVar(&opts.baseline, "baseline", "", "baseline path (defaults to coverage-baseline.json at the repository root)")
	flag.Parse()
	opts.stdout = os.Stdout

	if opts.baseline == "" {
		root, err := repositoryRoot()
		if err != nil {
			return err
		}
		opts.baseline = filepath.Join(root, baselineFile)
	}
	return runWithOptions(opts)
}

func runWithOptions(opts options) error {
	if opts.check && opts.write {
		return errors.New("choose only one of -check or -write")
	}
	if !opts.check && !opts.write {
		return errors.New("choose one of -check or -write")
	}
	if opts.goos == "" {
		opts.goos = runtime.GOOS
	}
	if opts.write && opts.goos != "linux" {
		return fmt.Errorf("coverage baselines must be written on linux, not %s", opts.goos)
	}
	if opts.stdout == nil {
		opts.stdout = io.Discard
	}

	profile, err := os.Open(opts.profile) // #nosec G304 -- the caller supplies the generated coverage profile.
	if err != nil {
		return fmt.Errorf("open coverage profile %s: %w", opts.profile, err)
	}
	defer func() { _ = profile.Close() }()
	current, err := covergate.ParseProfile(profile)
	if err != nil {
		return fmt.Errorf("parse coverage profile: %w", err)
	}

	if opts.write {
		baseline, err := covergate.BuildBaseline(current, requiredPackages)
		if err != nil {
			return fmt.Errorf("cannot write incomplete coverage baseline: %w", err)
		}
		data, err := baseline.MarshalIndented()
		if err != nil {
			return fmt.Errorf("encode baseline: %w", err)
		}
		if err := os.WriteFile(opts.baseline, data, 0o644); err != nil { // #nosec G306 -- committed project artifact.
			return fmt.Errorf("write baseline %s: %w", opts.baseline, err)
		}
		printCoverage(opts.stdout, "wrote "+opts.baseline, baseline.Packages, current)
		return nil
	}

	baseline, err := covergate.LoadBaseline(opts.baseline)
	if err != nil {
		return err
	}
	if err := baseline.Validate(requiredPackages); err != nil {
		return fmt.Errorf("invalid coverage baseline: %w", err)
	}
	regressions := baseline.Check(current)
	if len(regressions) > 0 {
		for _, regression := range regressions {
			fmt.Fprintf(opts.stdout, "  %s\n", regression.String())
		}
		return fmt.Errorf("coverage gate failed: %d package(s) below baseline", len(regressions))
	}
	printCoverage(opts.stdout, "coverage gate passed", baseline.Packages, current)
	return nil
}

func printCoverage(w io.Writer, heading string, floors map[string]float64, current map[string]covergate.Statements) {
	fmt.Fprintln(w, heading)
	packages := make([]string, 0, len(floors))
	for pkg := range floors {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	for _, pkg := range packages {
		fmt.Fprintf(w, "  %-62s %.1f%% (floor %.1f%%)\n", pkg, current[pkg].Percent(), floors[pkg])
	}
}

func repositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("could not find repository root")
		}
		directory = parent
	}
}
