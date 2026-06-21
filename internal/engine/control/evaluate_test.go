package control

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeMonotonicRefreshesPolicyMetadata(t *testing.T) {
	t.Parallel()
	// Existing control has stale mode (blocking) and old policy version.
	existing := []model.ControlActivation{
		{
			ControlID:    model.ControlWorktreeIsolation,
			Mode:         model.ControlModeBlocking, // stale
			Scope:        model.ControlScopeExecution,
			Active:       true,
			TriggeredBy:  []string{"domain_present"},
			PolicySource: model.BuiltinPolicySource,
		},
	}

	// New candidate has updated mode (advisory) and new policy version.
	candidates := []model.ControlActivation{
		{
			ControlID:    model.ControlWorktreeIsolation,
			Mode:         model.ControlModeAdvisory, // current
			Scope:        model.ControlScopeExecution,
			Active:       true,
			TriggeredBy:  []string{"blast_radius=high", "domain_present"},
			PolicySource: model.BuiltinPolicySource,
		},
	}

	result := mergeMonotonic(existing, candidates)

	require.Len(t, result.ActiveControls, 1, "existing control must be preserved")
	ctrl := result.ActiveControls[0]

	// Metadata should be refreshed from the candidate.
	assert.Equal(t, model.ControlModeAdvisory, ctrl.Mode, "mode must be refreshed")
	assert.Equal(t, []string{"blast_radius=high", "domain_present"}, ctrl.TriggeredBy, "triggers must be refreshed")
}

func TestMergeMonotonicPreservesExistingWithoutCandidate(t *testing.T) {
	t.Parallel()

	// Existing control has no matching candidate (e.g., signals changed and
	// it would no longer trigger). Monotonic guarantee: it stays.
	existing := []model.ControlActivation{
		{
			ControlID:    model.ControlDomainReview,
			Mode:         model.ControlModeBlocking,
			Scope:        model.ControlScopeReview,
			Active:       true,
			TriggeredBy:  []string{"auth_authz"},
			PolicySource: model.BuiltinPolicySource,
		},
	}

	result := mergeMonotonic(existing, nil)

	require.Len(t, result.ActiveControls, 1)
	ctrl := result.ActiveControls[0]
	assert.Equal(t, model.ControlModeBlocking, ctrl.Mode, "mode unchanged when no candidate")
}

func TestDeriveControlsRefreshesExistingControlMode(t *testing.T) {
	t.Parallel()
	// Simulate a snapshot with worktree-isolation=blocking (old default).
	// DeriveControls should refresh it to advisory (new default).
	existing := []model.ControlActivation{
		{
			ControlID:    model.ControlWorktreeIsolation,
			Mode:         model.ControlModeBlocking, // stale
			Scope:        model.ControlScopeExecution,
			Active:       true,
			TriggeredBy:  []string{"domain_present"},
			PolicySource: model.BuiltinPolicySource,
		},
	}

	result := DeriveControls(DeriveControlsInput{
		GuardrailDomain:  "auth_authz",
		NeedsDiscovery:   false,
		ExistingControls: existing,
		Traceability:     model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
	})

	for _, c := range result.ActiveControls {
		if c.ControlID == model.ControlWorktreeIsolation {
			assert.Equal(t, model.ControlModeAdvisory, c.Mode,
				"worktree-isolation must be refreshed from blocking to advisory")
			return
		}
	}
	t.Fatal("worktree-isolation should be in active controls")
}
