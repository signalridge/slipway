package wave

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTaskPlanPreservesObjectiveLeadingMarkers(t *testing.T) {
	t.Parallel()

	plan, err := ParseTaskPlan(`# Tasks

- [ ] ` + "`t-01`" + ` --preserve-leading-dashes
`)
	require.NoError(t, err)
	require.Len(t, plan.Tasks, 1)
	assert.Equal(t, "--preserve-leading-dashes", plan.Tasks[0].Objective)
}
