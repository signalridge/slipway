package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadEvidenceDigestsRoundTripAndListVerificationsSkipsStore(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "digest-store"
	saveActiveChangeForTest(t, root, slug)

	digests := model.EvidenceDigests{
		Version: model.EvidenceDigestsVersion,
		Skills: map[string]model.SkillDigest{
			"plan-audit": {
				RunVersion: 0,
				Inputs: map[string]string{
					"assurance.md": "sha256:assurance",
					"tasks.md":     "sha256:tasks",
				},
			},
		},
	}
	require.NoError(t, SaveEvidenceDigests(root, slug, digests))

	loaded, err := LoadEvidenceDigests(root, slug)
	require.NoError(t, err)
	require.Contains(t, loaded.Skills, "plan-audit")
	assert.Equal(t, digests.Skills["plan-audit"].Inputs, loaded.Skills["plan-audit"].Inputs)

	raw, err := os.ReadFile(filepath.Join(VerificationDir(root, slug), EvidenceDigestsFileName))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "assurance.md")

	writeVerificationForTest(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
	})
	records, err := ListVerifications(root, slug)
	require.NoError(t, err)
	assert.Contains(t, records, "plan-audit")
	assert.NotContains(t, records, "evidence-digests")
}

func TestLoadEvidenceDigestsRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "digest-store-unknown"
	saveActiveChangeForTest(t, root, slug)

	dir := VerificationDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, EvidenceDigestsFileName), []byte(`version: 1
unexpected: true
skills: {}
`), 0o644))

	_, err := LoadEvidenceDigests(root, slug)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field unexpected")
}
