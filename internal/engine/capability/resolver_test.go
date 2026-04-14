package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSelectsCommandRoute(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Command: "review"})
	require.NotNil(t, res.Route)
	assert.Equal(t, "independent-review", res.Route.SkillID)
	assert.Equal(t, "independent-review", res.Route.Mode)
	assert.NotEmpty(t, res.Route.Reason)
}

func TestResolveSelectsCommandViewRoute(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Command: "status"})
	require.NotNil(t, res.Route)
	assert.Equal(t, "incident-response", res.Route.SkillID)
	assert.Equal(t, "incident-response", res.Route.View)
	assert.NotEmpty(t, res.Route.Reason)
}

func TestResolveAttachesIntakeSupportOnIntakeHost(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Host: "intake-clarification"})
	assert.Nil(t, res.Route)
	require.NotEmpty(t, res.Supports)
	assert.Equal(t, "scope-clarification", res.Supports[0].SkillID)
	assert.Equal(t, AttachmentPosture, res.Supports[0].Kind)
	assert.NotEmpty(t, res.Supports[0].Reason)
}

func TestResolveCapsSupportsAtThree(t *testing.T) {
	t.Parallel()
	// Signals that match several skills' triggers at once.
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{
		Host:         "goal-verification",
		Command:      "validate",
		ChangedFiles: []string{"docs/plans/2026-04-11-foo.md"},
		Blockers:     []string{"stale_verification_evidence"},
	})
	assert.LessOrEqual(t, len(res.Supports), 3)
}

func TestResolveNoMatchReturnsEmpty(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{})
	assert.Nil(t, res.Route)
	assert.Empty(t, res.Supports)
}

func TestResolveReviewHostAttachesIndependentReview(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "code-quality-review"})
	require.NotEmpty(t, res.Supports)
	foundIR := false
	for _, s := range res.Supports {
		if s.SkillID == "independent-review" {
			foundIR = true
			assert.NotEmpty(t, s.Kind)
		}
	}
	assert.True(t, foundIR, "expected independent-review attached at code-quality-review host")
}

func TestResolveB1DoesNotEmitHydrateOrTiebreak(t *testing.T) {
	t.Parallel()
	// B1 must not implement hydrate_references or llm_tiebreak yet.
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{
		Host:    "plan-audit",
		Command: "review",
	})
	assert.Empty(t, res.HydrateReferences)
	assert.Nil(t, res.LLMTiebreak)
}
