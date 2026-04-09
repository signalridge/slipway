package model

import (
	"fmt"
	"reflect"
	"time"
)

// GovernanceSnapshotVersion is the current schema version for governance snapshots.
// Bumped to 2 for the reduced summary/observation schema (domain + blast-radius only).
const GovernanceSnapshotVersion = 2

// GovernanceSnapshot is the runtime governance sidecar persisted under
// <git-common-dir>/slipway/cache/changes/<slug>/governance_snapshot.yaml.
// It contains derived governance state and must not be written into change.yaml.
type GovernanceSnapshot struct {
	Version        int                 `yaml:"version" json:"version"`
	Summary        SignalSummary       `yaml:"summary" json:"summary"`
	Observations   []SignalObservation `yaml:"observations,omitempty" json:"observations,omitempty"`
	Traceability   TraceabilitySummary `yaml:"traceability" json:"traceability"`
	ActiveControls []ControlActivation `yaml:"active_controls,omitempty" json:"active_controls,omitempty"`
	ComputedAt     time.Time           `yaml:"computed_at" json:"computed_at"`
}

// Validate checks governance snapshot invariants.
func (g GovernanceSnapshot) Validate() error {
	if g.Version != GovernanceSnapshotVersion {
		return fmt.Errorf("version must equal %d: %d", GovernanceSnapshotVersion, g.Version)
	}
	if err := g.Summary.Validate(); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	for i, obs := range g.Observations {
		if err := obs.Validate(); err != nil {
			return fmt.Errorf("observations[%d]: %w", i, err)
		}
	}
	if err := g.Traceability.Validate(); err != nil {
		return fmt.Errorf("traceability: %w", err)
	}
	for i, ctrl := range g.ActiveControls {
		if err := ctrl.Validate(); err != nil {
			return fmt.Errorf("active_controls[%d]: %w", i, err)
		}
		if !ctrl.Active {
			return fmt.Errorf("active_controls[%d]: must be active", i)
		}
	}
	if g.ComputedAt.IsZero() {
		return fmt.Errorf("computed_at is required")
	}
	return nil
}

func (g GovernanceSnapshot) normalizedForPersistenceComparison() GovernanceSnapshot {
	normalized := g
	normalized.ComputedAt = time.Time{}
	return normalized
}

// PersistedEqual returns true when two snapshots are materially identical for persistence.
func (g GovernanceSnapshot) PersistedEqual(other GovernanceSnapshot) bool {
	return reflect.DeepEqual(
		g.normalizedForPersistenceComparison(),
		other.normalizedForPersistenceComparison(),
	)
}
