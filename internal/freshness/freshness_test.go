package freshness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateEvidenceFreshness(t *testing.T) {
	t.Parallel()
	assert.Equal(t, EvidenceFreshnessUnknown, EvaluateEvidenceFreshness(false, nil))
	assert.Equal(t, EvidenceFreshnessUnknown, EvaluateEvidenceFreshness(true, nil))

	fresh := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{
			ExpectedStructuralInput: map[string]string{"task_id": "task-a"},
			CurrentStructuralInput:  map[string]string{"task_id": "task-a"},
		},
	})
	assert.Equal(t, EvidenceFreshnessFresh, fresh)

	staleByStructuralInput := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{
			ExpectedStructuralInput: map[string]string{"task_id": "task-a"},
			CurrentStructuralInput:  map[string]string{"task_id": "task-b"},
		},
	})
	assert.Equal(t, EvidenceFreshnessStale, staleByStructuralInput)

	unknownInsufficient := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{},
	})
	assert.Equal(t, EvidenceFreshnessUnknown, unknownInsufficient)
}
