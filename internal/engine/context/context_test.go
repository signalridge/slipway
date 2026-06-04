package enginecontext

import (
	"testing"
	"time"

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
			EvidenceTimestamp:       time.Now().UTC(),
			LatestRelevantUpdateAt:  time.Now().UTC().Add(-time.Second),
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

	timestampOnly := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{
			EvidenceTimestamp:      time.Now().UTC().Add(-2 * time.Minute),
			LatestRelevantUpdateAt: time.Now().UTC().Add(-time.Minute),
		},
	})
	assert.Equal(t, EvidenceFreshnessUnknown, timestampOnly)

	unknownInsufficient := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{},
	})
	assert.Equal(t, EvidenceFreshnessUnknown, unknownInsufficient)
}
