package capability

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// AbsorbedAs names how a single source skill contributes to a catalog skill.
type AbsorbedAs string

const (
	AbsorbedStandalone  AbsorbedAs = "standalone"
	AbsorbedPostureOnly AbsorbedAs = "posture-only"
	AbsorbedPartialOnly AbsorbedAs = "partial-only"
)

// Provenance captures the per-skill source decisions that back the
// provenance-coverage-scan gate.
type Provenance struct {
	Sources []ProvenanceSource `yaml:"sources"`
}

// ProvenanceSource is a single source attribution record.
type ProvenanceSource struct {
	Source        string     `yaml:"source"`
	AbsorbedAs    AbsorbedAs `yaml:"absorbed_as"`
	Extracted     []string   `yaml:"extracted,omitempty"`
	Dropped       []string   `yaml:"dropped,omitempty"`
	ConflictsWith []Conflict `yaml:"conflicts_with,omitempty"`
}

// Conflict records a conservative-merge decision.
type Conflict struct {
	With   string `yaml:"with"`
	Reason string `yaml:"reason"`
}

// LoadProvenance parses a provenance.yaml file on disk.
func LoadProvenance(path string) (*Provenance, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("provenance: read %s: %w", path, err)
	}
	var p Provenance
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("provenance: parse %s: %w", path, err)
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("provenance: validate %s: %w", path, err)
	}
	return &p, nil
}

// Validate enforces the provenance authoring contract.
func (p *Provenance) Validate() error {
	seen := map[string]bool{}
	for i, src := range p.Sources {
		if strings.TrimSpace(src.Source) == "" {
			return fmt.Errorf("sources[%d]: empty source", i)
		}
		if seen[src.Source] {
			return fmt.Errorf("sources[%d]: duplicate source %q", i, src.Source)
		}
		seen[src.Source] = true
		switch src.AbsorbedAs {
		case AbsorbedStandalone, AbsorbedPostureOnly, AbsorbedPartialOnly:
		default:
			return fmt.Errorf("sources[%d]: invalid absorbed_as %q", i, src.AbsorbedAs)
		}
		if len(src.Extracted) == 0 && len(src.Dropped) == 0 && len(src.ConflictsWith) == 0 {
			return fmt.Errorf("sources[%d]: %s must list at least one of extracted/dropped/conflicts_with", i, src.Source)
		}
	}
	return nil
}

// Coverage returns the sorted set of unique source identifiers covered by
// this provenance record.
func (p *Provenance) Coverage() []string {
	if p == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, src := range p.Sources {
		seen[src.Source] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// UnionCoverage unions Coverage across multiple provenance records. The
// caller typically passes every catalog skill's provenance and then diffs
// the result against the authoritative source corpus list.
func UnionCoverage(records ...*Provenance) []string {
	seen := map[string]struct{}{}
	for _, p := range records {
		for _, s := range p.Coverage() {
			seen[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}
