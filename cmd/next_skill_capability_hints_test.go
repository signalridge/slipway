package cmd

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signalridge/slipway/internal/model"
)

func TestAppendCatalogHintsIntakeHostDoesNotLeakRetiredScopeSkill(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "intake-clarification", nil, &nextView{})
	assert.Empty(t, hints)
}

func TestAppendCatalogHintsAttachesOnReviewHost(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "code-quality-review", nil, &nextView{})
	require.NotEmpty(t, hints)
	assert.Equal(t, "skill:independent-review", hints[0].Name)
	assert.Contains(t, hints[0].Reason, "procedure")
}

func TestAppendCatalogHintsEmptyWhenNoMatch(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "", nil, &nextView{})
	assert.Empty(t, hints)
}

func TestAppendCatalogHintsPreservesExisting(t *testing.T) {
	t.Parallel()
	existing := []techniqueHint{{Name: "slipway codebase-map", Reason: "seed"}}
	hints := appendCatalogHints(existing, "intake-clarification", nil, &nextView{})
	require.Len(t, hints, 1)
	assert.Equal(t, "slipway codebase-map", hints[0].Name)
}

func TestAppendCatalogHintsBlockersAloneDoNotPopulateSupports(t *testing.T) {
	t.Parallel()
	// After trigger-DSL removal, blocker-only signals without a host do not
	// populate support attachments — host-embedded / technique-hint bindings
	// require an active host. Blocker-based suggestions are no longer surfaced
	// through a separate channel.
	view := &nextView{Blockers: []model.ReasonCode{{Code: "missing_red_proof"}}}
	hints := appendCatalogHints(nil, "", nil, view)
	assert.Empty(t, hints)
}

func TestAppendCatalogHintsAttachesHydrateReferencesOnWaveHost(t *testing.T) {
	t.Parallel()

	hints := appendCatalogHints(nil, "wave-orchestration", nil, &nextView{})
	require.NotEmpty(t, hints)

	var rootCauseHint *techniqueHint
	for i := range hints {
		if hints[i].Name == "skill:root-cause-tracing" {
			rootCauseHint = &hints[i]
			break
		}
	}
	require.NotNil(t, rootCauseHint, "expected root-cause-tracing support hint on wave-orchestration host")
	assert.Equal(t, []string{
		"root-cause-tracing/condition-based-waiting.md",
		"root-cause-tracing/hypothesis-testing.md",
		"root-cause-tracing/root-cause-tracing.md",
	}, rootCauseHint.HydrateReferences)
	assert.True(t, slices.IsSorted(rootCauseHint.HydrateReferences), "hydrate references should be stable-sorted")
}
