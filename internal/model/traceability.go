package model

import "fmt"

// TraceabilityStatus represents the coherence status of a traceability check.
type TraceabilityStatus string

const (
	TraceabilityStatusOK      TraceabilityStatus = "ok"
	TraceabilityStatusWarning TraceabilityStatus = "warning"
	TraceabilityStatusFail    TraceabilityStatus = "fail"
)

func (s TraceabilityStatus) String() string { return string(s) }

func (s TraceabilityStatus) IsValid() bool {
	switch s {
	case TraceabilityStatusOK, TraceabilityStatusWarning, TraceabilityStatusFail:
		return true
	default:
		return false
	}
}

// TraceabilityLink represents a single traced connection between artifact IDs.
type TraceabilityLink struct {
	FromID   string `yaml:"from_id" json:"from_id"`
	FromType string `yaml:"from_type" json:"from_type"` // intent, requirement, decision, task
	ToID     string `yaml:"to_id" json:"to_id"`
	ToType   string `yaml:"to_type" json:"to_type"`
}

// TraceabilityGap represents a missing or broken traceability link.
type TraceabilityGap struct {
	ID       string `yaml:"id" json:"id"`
	Type     string `yaml:"type" json:"type"` // intent, requirement, decision, task, assurance
	Issue    string `yaml:"issue" json:"issue"`
	Blocking bool   `yaml:"blocking" json:"blocking"`
}

// Validate checks link invariants.
func (l TraceabilityLink) Validate() error {
	if l.FromID == "" {
		return fmt.Errorf("traceability link from_id is required")
	}
	if l.ToID == "" {
		return fmt.Errorf("traceability link to_id is required")
	}
	return nil
}

// Validate checks gap invariants.
func (g TraceabilityGap) Validate() error {
	if g.ID == "" {
		return fmt.Errorf("traceability gap id is required")
	}
	if g.Issue == "" {
		return fmt.Errorf("traceability gap issue is required")
	}
	return nil
}

// TraceabilitySummary captures the traceability state across the artifact bundle.
type TraceabilitySummary struct {
	Status  TraceabilityStatus `yaml:"status" json:"status"`
	Links   []TraceabilityLink `yaml:"links,omitempty" json:"links,omitempty"`
	Gaps    []TraceabilityGap  `yaml:"gaps,omitempty" json:"gaps,omitempty"`
	Message string             `yaml:"message,omitempty" json:"message,omitempty"`
}

// FirstBlockingIntentGap returns the first blocking intent gap, if any.
func (t TraceabilitySummary) FirstBlockingIntentGap() (TraceabilityGap, bool) {
	for _, gap := range t.Gaps {
		if gap.Type == "intent" && gap.Blocking {
			return gap, true
		}
	}
	return TraceabilityGap{}, false
}

// HasBlockingIntentGap returns true if any gap is a blocking intent gap.
func (t TraceabilitySummary) HasBlockingIntentGap() bool {
	_, found := t.FirstBlockingIntentGap()
	return found
}

// Validate checks traceability summary invariants.
func (t TraceabilitySummary) Validate() error {
	if !t.Status.IsValid() {
		return fmt.Errorf("invalid traceability status: %q", t.Status)
	}
	for i, link := range t.Links {
		if err := link.Validate(); err != nil {
			return fmt.Errorf("links[%d]: %w", i, err)
		}
	}
	for i, gap := range t.Gaps {
		if err := gap.Validate(); err != nil {
			return fmt.Errorf("gaps[%d]: %w", i, err)
		}
	}
	return nil
}
