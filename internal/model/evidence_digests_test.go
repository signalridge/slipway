package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenceFreshnessComparesNamedInputDigests(t *testing.T) {
	t.Parallel()

	stored := SkillDigest{
		RunVersion: 1,
		Inputs: map[string]string{
			"assurance.md": "sha256:assurance-a",
			"tasks.md":     "sha256:tasks-a",
		},
	}

	fresh, changed := EvidenceFreshness(stored, map[string]string{
		"assurance.md": "sha256:assurance-a",
		"tasks.md":     "sha256:tasks-a",
	})
	require.True(t, fresh)
	assert.Empty(t, changed)

	fresh, changed = EvidenceFreshness(stored, map[string]string{
		"assurance.md": "sha256:assurance-b",
		"tasks.md":     "sha256:tasks-a",
	})
	require.False(t, fresh)
	assert.Equal(t, []string{"assurance.md"}, changed)

	fresh, changed = EvidenceFreshness(stored, map[string]string{
		"tasks.md": "sha256:tasks-a",
	})
	require.False(t, fresh)
	assert.Equal(t, []string{"assurance.md"}, changed)

	fresh, changed = EvidenceFreshness(stored, map[string]string{
		"assurance.md": "sha256:assurance-a",
		"tasks.md":     "sha256:tasks-a",
		"decision.md":  "sha256:decision-a",
	})
	require.False(t, fresh)
	assert.Equal(t, []string{"decision.md"}, changed)
}
