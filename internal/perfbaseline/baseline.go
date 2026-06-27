package perfbaseline

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	SchemaVersion           = 1
	DefaultRegressionBudget = 0.30
)

type Baseline struct {
	SchemaVersion    int              `json:"schema_version"`
	GeneratedAt      time.Time        `json:"generated_at"`
	GitCommit        string           `json:"git_commit"`
	GoVersion        string           `json:"go_version"`
	SlipwayBinary    string           `json:"slipway_binary"`
	RegressionBudget float64          `json:"regression_budget"`
	WarmupCount      int              `json:"warmup_count"`
	SampleCount      int              `json:"sample_count"`
	SampleStatistic  string           `json:"sample_statistic"`
	CheckAttempts    int              `json:"check_attempts"`
	Fixture          Fixture          `json:"fixture"`
	Commands         []CommandMeasure `json:"commands"`
	RefreshCommand   string           `json:"refresh_command"`
	CheckCommand     string           `json:"check_command"`
}

type Fixture struct {
	Root                 string `json:"root"`
	WorktreeCount        int    `json:"worktree_count"`
	ChangeYAMLCount      int    `json:"change_yaml_count"`
	VerificationCount    int    `json:"verification_count"`
	BoundChangeSlug      string `json:"bound_change_slug"`
	ExplicitChangeSlug   string `json:"explicit_change_slug"`
	FixtureGenerationRef string `json:"fixture_generation_ref,omitempty"`
}

type CommandMeasure struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	CWD         string   `json:"cwd"`
	Args        []string `json:"args"`
	RealMS      float64  `json:"real_ms"`
	UserMS      float64  `json:"user_ms"`
	SystemMS    float64  `json:"system_ms"`
	ExitCode    int      `json:"exit_code"`
}

type Regression struct {
	CommandID    string
	BaselineMS   float64
	CurrentMS    float64
	AllowedMS    float64
	ThresholdPct float64
}

func NewBaseline() Baseline {
	return Baseline{
		SchemaVersion:    SchemaVersion,
		GeneratedAt:      time.Now().UTC(),
		RegressionBudget: DefaultRegressionBudget,
		Commands:         []CommandMeasure{},
	}
}

func Read(r io.Reader) (Baseline, error) {
	var b Baseline
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return Baseline{}, fmt.Errorf("decode state-read baseline: %w", err)
	}
	if err := b.Validate(); err != nil {
		return Baseline{}, err
	}
	return b, nil
}

func Write(w io.Writer, b Baseline) error {
	if err := b.Validate(); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(b); err != nil {
		return fmt.Errorf("encode state-read baseline: %w", err)
	}
	return nil
}

func (b Baseline) Validate() error {
	if b.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported state-read baseline schema_version %d", b.SchemaVersion)
	}
	if b.RegressionBudget <= 0 {
		return fmt.Errorf("regression_budget must be positive")
	}
	if b.WarmupCount < 0 {
		return fmt.Errorf("warmup_count must be non-negative")
	}
	if b.SampleCount <= 0 {
		return fmt.Errorf("sample_count must be positive")
	}
	if strings.TrimSpace(b.SampleStatistic) == "" {
		return fmt.Errorf("sample_statistic is required")
	}
	if b.CheckAttempts <= 0 {
		return fmt.Errorf("check_attempts must be positive")
	}
	if b.Fixture.WorktreeCount <= 0 {
		return fmt.Errorf("fixture.worktree_count must be positive")
	}
	if b.Fixture.ChangeYAMLCount <= 0 {
		return fmt.Errorf("fixture.change_yaml_count must be positive")
	}
	if b.Fixture.VerificationCount <= 0 {
		return fmt.Errorf("fixture.verification_count must be positive")
	}
	if len(b.Commands) == 0 {
		return fmt.Errorf("commands must not be empty")
	}
	seen := map[string]struct{}{}
	for i, cmd := range b.Commands {
		if err := cmd.Validate(); err != nil {
			return fmt.Errorf("commands[%d]: %w", i, err)
		}
		if _, ok := seen[cmd.ID]; ok {
			return fmt.Errorf("duplicate command id %q", cmd.ID)
		}
		seen[cmd.ID] = struct{}{}
	}
	return nil
}

func (m CommandMeasure) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if len(m.Args) == 0 {
		return fmt.Errorf("args must not be empty")
	}
	if math.IsNaN(m.RealMS) || m.RealMS <= 0 {
		return fmt.Errorf("real_ms must be positive")
	}
	if math.IsNaN(m.UserMS) || m.UserMS < 0 {
		return fmt.Errorf("user_ms must be non-negative")
	}
	if math.IsNaN(m.SystemMS) || m.SystemMS < 0 {
		return fmt.Errorf("system_ms must be non-negative")
	}
	return nil
}

func Compare(baseline, current Baseline, threshold float64) ([]Regression, error) {
	if threshold <= 0 {
		threshold = baseline.RegressionBudget
	}
	if threshold <= 0 {
		threshold = DefaultRegressionBudget
	}
	if err := baseline.Validate(); err != nil {
		return nil, fmt.Errorf("baseline invalid: %w", err)
	}
	if err := current.Validate(); err != nil {
		return nil, fmt.Errorf("current measurement invalid: %w", err)
	}

	currentByID := make(map[string]CommandMeasure, len(current.Commands))
	for _, cmd := range current.Commands {
		currentByID[cmd.ID] = cmd
	}

	var regressions []Regression
	for _, base := range baseline.Commands {
		cur, ok := currentByID[base.ID]
		if !ok {
			return nil, fmt.Errorf("current measurement missing command %q", base.ID)
		}
		allowed := base.RealMS * (1 + threshold)
		if cur.RealMS > allowed {
			regressions = append(regressions, Regression{
				CommandID:    base.ID,
				BaselineMS:   base.RealMS,
				CurrentMS:    cur.RealMS,
				AllowedMS:    allowed,
				ThresholdPct: threshold * 100,
			})
		}
	}
	sort.Slice(regressions, func(i, j int) bool {
		return regressions[i].CommandID < regressions[j].CommandID
	})
	return regressions, nil
}

func RequiredCommandIDs() []string {
	return []string{
		"root-status-json",
		"bound-status-json",
		"bound-next-json-diagnostics",
		"bound-validate-json",
		"explicit-change-status-json",
	}
}

func MissingRequiredCommands(b Baseline) []string {
	present := map[string]struct{}{}
	for _, cmd := range b.Commands {
		present[cmd.ID] = struct{}{}
	}
	var missing []string
	for _, id := range RequiredCommandIDs() {
		if _, ok := present[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

func SortCommands(commands []CommandMeasure) {
	order := RequiredCommandIDs()
	slices.SortFunc(commands, func(a, b CommandMeasure) int {
		ai := slices.Index(order, a.ID)
		bi := slices.Index(order, b.ID)
		if ai == -1 {
			ai = len(order)
		}
		if bi == -1 {
			bi = len(order)
		}
		if ai != bi {
			return ai - bi
		}
		return strings.Compare(a.ID, b.ID)
	})
}
