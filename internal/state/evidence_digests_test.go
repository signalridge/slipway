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

// A legacy evidence-digests cache that still carries the removed per-skill
// run_version key must not hard-fail the read-only readiness surfaces.
// evidence-digests.yaml is a gitignored, regenerable runtime cache, so the
// optional change-based reader treats an unusable cache as absent and lets the
// owning stage regenerate it. The strict slug-based reader still rejects the
// unknown legacy key.
func TestLoadOptionalEvidenceDigestsSelfHealsUnusableLegacyCache(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "digest-legacy-run-version"
	change := saveActiveChangeForTest(t, root, slug)

	dir := VerificationDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	legacy := []byte(`version: 1
skills:
  goal-verification:
    run_version: 1
    inputs:
      some-input: sha256:abc
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, EvidenceDigestsFileName), legacy, 0o644))

	// Optional change-based read self-heals: the unusable legacy cache is treated
	// as absent (regenerate), not surfaced as a parse error that would block
	// status/next/validate.
	got, err := LoadOptionalEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Nil(t, got, "an unusable legacy run_version cache must read as absent so the owning stage regenerates it")

	// The strict slug-based reader still rejects the removed legacy key.
	_, strictErr := LoadEvidenceDigests(root, slug)
	require.Error(t, strictErr)
	assert.Contains(t, strictErr.Error(), "run_version")
}

// LoadEvidenceDigests and evidenceDigestsReadPath are slug-based read helpers
// used only by these tests; production reads digests through the change-based
// LoadEvidenceDigestsForChange path.
func LoadEvidenceDigests(root, slug string) (model.EvidenceDigests, error) {
	displayPath := EvidenceDigestsPathForRead(root, slug)
	path, err := evidenceDigestsReadPath(root, slug)
	if err != nil {
		return model.EvidenceDigests{}, wrapExecutionSummaryLoadError(displayPath, err)
	}
	digests, err := loadEvidenceDigestsFromPath(path)
	if err != nil {
		return model.EvidenceDigests{}, wrapExecutionSummaryLoadError(path, err)
	}
	return digests, nil
}

func evidenceDigestsReadPath(root, slug string) (string, error) {
	dir, err := resolveExistingVerificationDir(root, slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, EvidenceDigestsFileName), nil
}
