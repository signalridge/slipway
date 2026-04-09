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
		&nextSkillView{Name: "plan-audit", PromptPath: "missing"},
		nextContext{WorkspaceRoot: "/tmp/workspace", ArtifactBundle: "artifacts/changes/demo"},
	)
	if budget == nil {
		t.Fatal("expected fallback context budget")
	}
	if budget.Breakdown.StateContext == 0 {
		t.Fatal("expected non-zero state context from fallback serialization")
	}
}
