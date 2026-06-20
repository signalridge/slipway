package cmd

import (
	"errors"
	"testing"
)

func TestEstimateContextBudgetFallsBackWhenMarshalFails(t *testing.T) {
	original := marshalContextBudgetInput
	marshalContextBudgetInput = func(v any) ([]byte, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() {
		marshalContextBudgetInput = original
	})

	budget := estimateContextBudget(
		t.TempDir(),
		&nextSkillView{Name: "plan-audit"},
		nextContext{WorkspaceRoot: "/tmp/workspace", ArtifactBundle: "artifacts/changes/demo"},
	)
	if budget == nil {
		t.Fatal("expected fallback context budget")
	}
	if budget.Breakdown.StateContext == 0 {
		t.Fatal("expected non-zero state context from fallback serialization")
	}
}

// estimateContextBudget must honor the first VALID context-window override in
// priority order. A malformed or non-positive higher-priority alias
// (SLIPWAY_CONTEXT_WINDOW_TOKENS) must NOT mask a valid lower-priority alias
// (SPECLANE_CONTEXT_WINDOW_TOKENS) or silently revert to the default window —
// that would weaken the context hard-stop guard. Regression for the alias loop
// that short-circuited on the first non-empty value regardless of validity.
func TestEstimateContextBudgetEnvWindowPrecedence(t *testing.T) {
	const defaultWindow = 200000
	skill := &nextSkillView{Name: "plan-audit"}
	ctx := nextContext{WorkspaceRoot: "/tmp/workspace", ArtifactBundle: "artifacts/changes/demo"}

	cases := []struct {
		name     string
		slipway  string
		speclane string
		want     int
	}{
		{"valid new alias wins over legacy", "300000", "1", 300000},
		{"invalid new alias must not mask valid legacy alias", "bad", "1", 1},
		{"non-positive new alias falls through to legacy", "0", "1", 1},
		{"legacy alias honored when new alias unset", "", "1", 1},
		{"both invalid falls back to default", "bad", "nope", defaultWindow},
		{"neither set falls back to default", "", "", defaultWindow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SLIPWAY_CONTEXT_WINDOW_TOKENS", tc.slipway)
			t.Setenv("SPECLANE_CONTEXT_WINDOW_TOKENS", tc.speclane)

			budget := estimateContextBudget(t.TempDir(), skill, ctx)
			if budget == nil {
				t.Fatal("expected a context budget")
			}
			if budget.AssumedContextWindow != tc.want {
				t.Fatalf("AssumedContextWindow = %d, want %d", budget.AssumedContextWindow, tc.want)
			}
		})
	}
}
