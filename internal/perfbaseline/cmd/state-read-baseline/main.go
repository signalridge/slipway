package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/perfbaseline"
	"github.com/signalridge/slipway/internal/state"
)

const (
	defaultWorktrees     = 25
	defaultChanges       = 300
	defaultVerifications = 100
)

type options struct {
	mode          string
	out           string
	baseline      string
	binary        string
	fixtureRoot   string
	keepFixture   bool
	threshold     float64
	worktrees     int
	changes       int
	verifications int
	warmups       int
	samples       int
	checkAttempts int
	stdout        io.Writer
	stderr        io.Writer
}

type fixtureInfo struct {
	root               string
	boundWorktree      string
	boundChangeSlug    string
	explicitChangeSlug string
}

type generatedChange struct {
	slug      string
	bundleDir string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	opts, err := parseOptions(args, stdout, stderr)
	if err != nil {
		return err
	}
	switch opts.mode {
	case "refresh":
		return refresh(opts)
	case "check":
		return check(opts)
	default:
		return fmt.Errorf("unsupported -mode %q (expected refresh or check)", opts.mode)
	}
}

func parseOptions(args []string, stdout, stderr io.Writer) (options, error) {
	opts := options{
		mode:          "refresh",
		out:           "state-read-performance-baseline.json",
		baseline:      "state-read-performance-baseline.json",
		threshold:     perfbaseline.DefaultRegressionBudget,
		worktrees:     defaultWorktrees,
		changes:       defaultChanges,
		verifications: defaultVerifications,
		warmups:       1,
		samples:       7,
		checkAttempts: 3,
		stdout:        stdout,
		stderr:        stderr,
	}
	fs := flag.NewFlagSet("state-read-baseline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.mode, "mode", opts.mode, "mode: refresh or check")
	fs.StringVar(&opts.out, "out", opts.out, "output path for refresh or current measurement path for check")
	fs.StringVar(&opts.baseline, "baseline", opts.baseline, "baseline path used by check")
	fs.StringVar(&opts.binary, "binary", "", "existing slipway binary to measure; defaults to building a temporary binary")
	fs.StringVar(&opts.fixtureRoot, "fixture-root", "", "existing or new fixture root; defaults to a temporary directory")
	fs.BoolVar(&opts.keepFixture, "keep-fixture", false, "keep generated temporary fixture after the command exits")
	fs.Float64Var(&opts.threshold, "threshold", opts.threshold, "allowed real-time regression ratio")
	fs.IntVar(&opts.worktrees, "worktrees", opts.worktrees, "minimum fixture worktree count")
	fs.IntVar(&opts.changes, "changes", opts.changes, "minimum fixture change.yaml count")
	fs.IntVar(&opts.verifications, "verifications", opts.verifications, "minimum fixture verification record count")
	fs.IntVar(&opts.warmups, "warmups", opts.warmups, "warmup executions per measured command")
	fs.IntVar(&opts.samples, "samples", opts.samples, "timed sample executions per measured command; the fastest sample is recorded")
	fs.IntVar(&opts.checkAttempts, "check-attempts", opts.checkAttempts, "independent measurement attempts for check mode; any passing attempt satisfies the threshold")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	outProvided := flagProvided(fs, "out")
	if opts.worktrees < 1 {
		return options{}, errors.New("-worktrees must be positive")
	}
	if opts.changes < 2 {
		return options{}, errors.New("-changes must be at least 2 to cover bound and explicit change scenarios")
	}
	if opts.verifications < 1 {
		return options{}, errors.New("-verifications must be positive")
	}
	if opts.warmups < 0 {
		return options{}, errors.New("-warmups must be non-negative")
	}
	if opts.samples < 1 {
		return options{}, errors.New("-samples must be positive")
	}
	if opts.checkAttempts < 1 {
		return options{}, errors.New("-check-attempts must be positive")
	}
	if opts.mode == "check" && !outProvided {
		return options{}, errors.New("-out is required in check mode to avoid overwriting the committed baseline")
	}
	return opts, nil
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

func refresh(opts options) error {
	b, cleanup, err := measure(opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := perfbaseline.Write(&buf, b); err != nil {
		return err
	}
	if err := os.WriteFile(opts.out, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write baseline %s: %w", opts.out, err)
	}
	fmt.Fprintf(opts.stdout, "wrote %s\n", opts.out)
	return nil
}

func check(opts options) error {
	baselineFile, err := os.Open(opts.baseline)
	if err != nil {
		return fmt.Errorf("open baseline %s: %w", opts.baseline, err)
	}
	defer func() { _ = baselineFile.Close() }()

	baseline, err := perfbaseline.Read(baselineFile)
	if err != nil {
		return err
	}

	var lastRegressions []perfbaseline.Regression
	for attempt := 1; attempt <= opts.checkAttempts; attempt++ {
		current, cleanup, err := measure(opts)
		if cleanup != nil {
			cleanup()
		}
		if err != nil {
			return err
		}
		regressions, err := perfbaseline.Compare(baseline, current, opts.threshold)
		if err != nil {
			return err
		}
		if len(regressions) == 0 {
			return writeCheckResult(opts, current, attempt)
		}
		lastRegressions = regressions
	}

	for _, r := range lastRegressions {
		fmt.Fprintf(
			opts.stderr,
			"%s regressed: baseline %.2fms, current %.2fms, allowed %.2fms (%.0f%%)\n",
			r.CommandID,
			r.BaselineMS,
			r.CurrentMS,
			r.AllowedMS,
			r.ThresholdPct,
		)
	}
	return fmt.Errorf("%d state-read command(s) exceeded regression threshold after %d attempt(s)", len(lastRegressions), opts.checkAttempts)
}

func writeCheckResult(opts options, current perfbaseline.Baseline, attempt int) error {
	var buf bytes.Buffer
	if err := perfbaseline.Write(&buf, current); err != nil {
		return err
	}
	if err := os.WriteFile(opts.out, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write current measurement %s: %w", opts.out, err)
	}
	fmt.Fprintf(opts.stdout, "state-read baseline check passed on attempt %d; wrote %s\n", attempt, opts.out)
	return nil
}

func measure(opts options) (perfbaseline.Baseline, func(), error) {
	binary, binaryCleanup, err := ensureBinary(opts)
	if err != nil {
		return perfbaseline.Baseline{}, nil, err
	}

	fixture, fixtureCleanup, err := ensureFixture(opts, binary)
	if err != nil {
		if binaryCleanup != nil {
			binaryCleanup()
		}
		return perfbaseline.Baseline{}, nil, err
	}
	cleanup := func() {
		if fixtureCleanup != nil {
			fixtureCleanup()
		}
		if binaryCleanup != nil {
			binaryCleanup()
		}
	}

	commands := []perfbaseline.CommandMeasure{
		{
			ID:          "root-status-json",
			Description: "root worktree: slipway status --json",
			CWD:         fixture.root,
			Args:        []string{"status", "--json"},
		},
		{
			ID:          "bound-status-json",
			Description: "bound worktree: slipway status --json",
			CWD:         fixture.boundWorktree,
			Args:        []string{"status", "--json"},
		},
		{
			ID:          "bound-next-json-diagnostics",
			Description: "bound worktree: slipway next --json --diagnostics",
			CWD:         fixture.boundWorktree,
			Args:        []string{"next", "--json", "--diagnostics"},
		},
		{
			ID:          "bound-validate-json",
			Description: "bound worktree: slipway validate",
			CWD:         fixture.boundWorktree,
			Args:        []string{"validate"},
		},
		{
			ID:          "explicit-change-status-json",
			Description: "root worktree: slipway status --json --change <slug>",
			CWD:         fixture.root,
			Args:        []string{"status", "--json", "--change", fixture.explicitChangeSlug},
		},
	}

	for i := range commands {
		measured, err := runMeasured(binary, commands[i], opts.warmups, opts.samples)
		if err != nil {
			cleanup()
			return perfbaseline.Baseline{}, nil, err
		}
		commands[i] = measured
	}
	perfbaseline.SortCommands(commands)

	b := perfbaseline.NewBaseline()
	b.GitCommit = gitCommit()
	b.GoVersion = runtime.Version()
	b.SlipwayBinary = binary
	b.RegressionBudget = opts.threshold
	b.WarmupCount = opts.warmups
	b.SampleCount = opts.samples
	b.SampleStatistic = "fastest"
	b.CheckAttempts = opts.checkAttempts
	b.Fixture = perfbaseline.Fixture{
		Root:                 fixture.root,
		WorktreeCount:        opts.worktrees,
		ChangeYAMLCount:      opts.changes,
		VerificationCount:    opts.verifications,
		BoundChangeSlug:      fixture.boundChangeSlug,
		ExplicitChangeSlug:   fixture.explicitChangeSlug,
		FixtureGenerationRef: "internal/perfbaseline/cmd/state-read-baseline",
	}
	b.Commands = commands
	b.RefreshCommand = "go run ./internal/perfbaseline/cmd/state-read-baseline -mode refresh -out state-read-performance-baseline.json"
	b.CheckCommand = "go run ./internal/perfbaseline/cmd/state-read-baseline -mode check -baseline state-read-performance-baseline.json -out /tmp/slipway-state-read-current.json"
	if missing := perfbaseline.MissingRequiredCommands(b); len(missing) > 0 {
		cleanup()
		return perfbaseline.Baseline{}, nil, fmt.Errorf("missing required command measurements: %s", strings.Join(missing, ", "))
	}
	return b, cleanup, nil
}

func ensureBinary(opts options) (string, func(), error) {
	if strings.TrimSpace(opts.binary) != "" {
		abs, err := filepath.Abs(opts.binary)
		if err != nil {
			return "", nil, fmt.Errorf("resolve binary: %w", err)
		}
		return abs, nil, nil
	}
	dir, err := os.MkdirTemp("", "slipway-state-read-bin-*")
	if err != nil {
		return "", nil, fmt.Errorf("create binary tempdir: %w", err)
	}
	binary := filepath.Join(dir, binaryName())
	cmd := exec.Command("go", "build", "-o", binary, ".") // #nosec G204 -- fixed go build invocation for the current trusted repository.
	cmd.Stdout = opts.stdout
	cmd.Stderr = opts.stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("build slipway binary: %w", err)
	}
	return binary, func() { _ = os.RemoveAll(dir) }, nil
}

func ensureFixture(opts options, binary string) (fixtureInfo, func(), error) {
	root := strings.TrimSpace(opts.fixtureRoot)
	var cleanup func()
	if root == "" {
		dir, err := os.MkdirTemp("", "slipway-state-read-fixture-*")
		if err != nil {
			return fixtureInfo{}, nil, fmt.Errorf("create fixture tempdir: %w", err)
		}
		root = dir
		if !opts.keepFixture {
			cleanup = func() { _ = os.RemoveAll(dir) }
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return fixtureInfo{}, nil, fmt.Errorf("resolve fixture root: %w", err)
	}
	if err := prepareFixture(abs, binary, opts); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return fixtureInfo{}, nil, err
	}
	return fixtureInfo{
		root:               abs,
		boundWorktree:      filepath.Join(abs, ".worktrees", "perf-bound-change"),
		boundChangeSlug:    "perf-bound-change",
		explicitChangeSlug: "perf-explicit-change",
	}, cleanup, nil
}

func prepareFixture(root, binary string, opts options) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create fixture root: %w", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat fixture git dir: %w", err)
		}
		if err := runGitCommand(root, "init"); err != nil {
			return err
		}
		if err := runGitCommand(root, "config", "user.email", "fixture@example.invalid"); err != nil {
			return err
		}
		if err := runGitCommand(root, "config", "user.name", "Slipway Fixture"); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("state-read fixture\n"), 0o600); err != nil {
			return fmt.Errorf("write fixture README: %w", err)
		}
		if err := runGitCommand(root, "add", "README.md"); err != nil {
			return err
		}
		if err := runGitCommand(root, "commit", "-m", "fixture baseline"); err != nil {
			return err
		}
	}
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		return fmt.Errorf("write fixture config: %w", err)
	}
	if err := ensureWorktrees(root, opts.worktrees); err != nil {
		return err
	}
	changes, err := ensureChanges(root, opts)
	if err != nil {
		return err
	}
	if err := ensureVerificationRecords(changes, opts.verifications); err != nil {
		return err
	}
	if _, err := os.Stat(binary); err != nil { // #nosec G703 -- binary is a local path explicitly selected by the operator or built in a private tempdir.
		return fmt.Errorf("stat slipway binary %s: %w", binary, err)
	}
	return nil
}

func ensureWorktrees(root string, count int) error {
	if count <= 1 {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(root, ".worktrees"), 0o700); err != nil {
		return fmt.Errorf("create worktrees dir: %w", err)
	}
	for i := 1; i < count; i++ {
		name := fmt.Sprintf("perf-wt-%02d", i)
		path := filepath.Join(root, ".worktrees", name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		branch := "perf/" + name
		if err := runGitCommand(root, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
			return err
		}
	}
	return nil
}

func ensureChanges(root string, opts options) ([]generatedChange, error) {
	changes := make([]generatedChange, 0, opts.changes)
	for i := 0; i < opts.changes; i++ {
		slug := fmt.Sprintf("perf-change-%03d", i)
		switch i {
		case 0:
			slug = "perf-bound-change"
		case 1:
			slug = "perf-explicit-change"
		}
		change := model.NewChange(slug)
		change.Description = "state read fixture " + strconv.Itoa(i)
		change.ArtifactSchema = model.ArtifactSchemaCore
		change.WorkflowProfile = model.WorkflowProfileCode
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.QualityMode = model.QualityModeStandard
		change.CurrentState = model.StateS2Implement
		if slug == "perf-bound-change" {
			change.WorktreePath = filepath.Join(root, ".worktrees", slug)
			if _, err := os.Stat(change.WorktreePath); os.IsNotExist(err) {
				if err := runGitCommand(root, "worktree", "add", "-b", "feat/"+slug, change.WorktreePath, "HEAD"); err != nil {
					return nil, err
				}
			}
		}
		if err := state.SaveChange(root, change); err != nil {
			return nil, fmt.Errorf("save fixture change %s: %w", slug, err)
		}
		bundleDir, err := state.GovernedBundleDir(root, change)
		if err != nil {
			return nil, fmt.Errorf("resolve fixture bundle %s: %w", slug, err)
		}
		changes = append(changes, generatedChange{
			slug:      slug,
			bundleDir: bundleDir,
		})
	}
	return changes, nil
}

func ensureVerificationRecords(changes []generatedChange, count int) error {
	if len(changes) == 0 {
		return errors.New("verification records require at least one generated change slug")
	}
	for i := 0; i < count; i++ {
		change := changes[i%len(changes)]
		path := filepath.Join(change.bundleDir, "verification", fmt.Sprintf("skill-%03d.yaml", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create verification dir: %w", err)
		}
		raw := []byte(strings.Join([]string{
			"verdict: pass",
			"blockers: []",
			"timestamp: 2026-01-01T00:00:00Z",
			"run_version: 1",
			"references:",
			"  - fixture:state-read-baseline",
			"notes: fixture verification record",
			"",
		}, "\n"))
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			return fmt.Errorf("write verification %s: %w", path, err)
		}
	}
	return nil
}

func runMeasured(binary string, measure perfbaseline.CommandMeasure, warmups, samples int) (perfbaseline.CommandMeasure, error) {
	for i := 0; i < warmups; i++ {
		if _, err := runSingleMeasurement(binary, measure); err != nil {
			return measure, err
		}
	}
	measurements := make([]perfbaseline.CommandMeasure, 0, samples)
	for i := 0; i < samples; i++ {
		measured, err := runSingleMeasurement(binary, measure)
		if err != nil {
			return measure, err
		}
		measurements = append(measurements, measured)
	}
	return fastestMeasurement(measurements), nil
}

func fastestMeasurement(measurements []perfbaseline.CommandMeasure) perfbaseline.CommandMeasure {
	sort.Slice(measurements, func(i, j int) bool {
		return measurements[i].RealMS < measurements[j].RealMS
	})
	return measurements[0]
}

func runSingleMeasurement(binary string, measure perfbaseline.CommandMeasure) (perfbaseline.CommandMeasure, error) {
	cmd := exec.Command(binary, measure.Args...) // #nosec G204 G702 -- benchmark intentionally executes the selected local slipway binary with fixed scenario args.
	cmd.Dir = measure.CWD
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	measure.RealMS = float64(elapsed.Microseconds()) / 1000
	if state := cmd.ProcessState; state != nil {
		measure.ExitCode = state.ExitCode()
		if userMS, systemMS, ok := processTimesMS(state); ok {
			measure.UserMS = userMS
			measure.SystemMS = systemMS
		}
	}
	if err != nil {
		return measure, fmt.Errorf("%s failed: %w: %s", measure.ID, err, strings.TrimSpace(stderr.String()))
	}
	return measure, nil
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...) // #nosec G204 -- caller supplies fixed git subcommands from this file's fixture builder.
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "slipway.exe"
	}
	return "slipway"
}
