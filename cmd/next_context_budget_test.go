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

// estimateContextBudget must honor a VALID SLIPWAY_CONTEXT_WINDOW_TOKENS
// override and fall back to the default window when the value is malformed or
// non-positive — a malformed override must not silently weaken the context
// hard-stop guard.
func TestEstimateContextBudgetEnvWindow(t *testing.T) {
	const defaultWindow = 200000
	skill := &nextSkillView{Name: "plan-audit"}
	ctx := nextContext{WorkspaceRoot: "/tmp/workspace", ArtifactBundle: "artifacts/changes/demo"}

	cases := []struct {
		name    string
		slipway string
		want    int
	}{
		{"valid override honored", "300000", 300000},
		{"malformed override falls back to default", "bad", defaultWindow},
		{"non-positive override falls back to default", "0", defaultWindow},
		{"unset falls back to default", "", defaultWindow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SLIPWAY_CONTEXT_WINDOW_TOKENS", tc.slipway)

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
