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
			EvidenceInputHash:      "abc",
			CurrentInputHash:       "abc",
			EvidenceTimestamp:      time.Now().UTC(),
			LatestRelevantUpdateAt: time.Now().UTC().Add(-time.Second),
		},
	})
	assert.Equal(t, EvidenceFreshnessFresh, fresh)

	staleByHash := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{
			EvidenceInputHash: "abc",
			CurrentInputHash:  "def",
		},
	})
	assert.Equal(t, EvidenceFreshnessStale, staleByHash)

	staleByTime := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{
			EvidenceTimestamp:      time.Now().UTC().Add(-2 * time.Minute),
			LatestRelevantUpdateAt: time.Now().UTC().Add(-time.Minute),
		},
	})
	assert.Equal(t, EvidenceFreshnessStale, staleByTime)

	unknownInsufficient := EvaluateEvidenceFreshness(true, []EvidenceFreshnessInput{
		{},
	})
	assert.Equal(t, EvidenceFreshnessUnknown, unknownInsufficient)
}
