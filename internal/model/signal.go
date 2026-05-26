package model

import "fmt"

// SignalLevel represents the severity level of a governance signal.
type SignalLevel string

const (
	SignalLevelLow    SignalLevel = "low"
	SignalLevelMedium SignalLevel = "medium"
	SignalLevelHigh   SignalLevel = "high"
)

func (l SignalLevel) String() string { return string(l) }

func (l SignalLevel) IsValid() bool {
	switch l {
	case SignalLevelLow, SignalLevelMedium, SignalLevelHigh:
		return true
	default:
		return false
	}
}

// SignalLevelOrder returns numeric ordering for comparison (low=0, medium=1, high=2).
func (l SignalLevel) Order() int {
	switch l {
	case SignalLevelLow:
		return 0
	case SignalLevelMedium:
		return 1
	case SignalLevelHigh:
		return 2
	default:
		return -1
	}
}

// SignalName identifies a governance signal type.
type SignalName string

const (
	SignalDomain      SignalName = "domain"
	SignalBlastRadius SignalName = "blast_radius"
)

func (n SignalName) String() string { return string(n) }

func (n SignalName) IsValid() bool {
	switch n {
	case SignalDomain, SignalBlastRadius:
		return true
	default:
		return false
	}
}

// SignalSummary is the compact governance surface rendered in status/next.
// Reduced to domain + blast-radius provenance only.
type SignalSummary struct {
	Domains     []string    `yaml:"domains,omitempty" json:"domains,omitempty"`
	BlastRadius SignalLevel `yaml:"blast_radius" json:"blast_radius"`
}

// Validate checks that all signal levels are valid.
func (s SignalSummary) Validate() error {
	if !s.BlastRadius.IsValid() {
		return fmt.Errorf("invalid blast_radius level: %q", s.BlastRadius)
	}
	return nil
}

// SignalObservation is a single derived governance signal with provenance.
type SignalObservation struct {
	ID           string      `yaml:"id" json:"id"`
	Signal       SignalName  `yaml:"signal" json:"signal"`
	Level        SignalLevel `yaml:"level" json:"level"`
	Source       string      `yaml:"source" json:"source"`
	Reason       string      `yaml:"reason" json:"reason"`
	EvidenceRefs []string    `yaml:"evidence_refs,omitempty" json:"evidence_refs,omitempty"`
}

// Validate checks observation invariants.
func (o SignalObservation) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("observation id is required")
	}
	if !o.Signal.IsValid() {
		return fmt.Errorf("invalid signal name: %q", o.Signal)
	}
	if !o.Level.IsValid() {
		return fmt.Errorf("invalid signal level: %q", o.Level)
	}
	if o.Source == "" {
		return fmt.Errorf("observation source is required")
	}
	if o.Reason == "" {
		return fmt.Errorf("observation reason is required")
	}
	return nil
}
