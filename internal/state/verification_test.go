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

func testVerificationReason(code, detail string) model.ReasonCode {
	return model.NewReasonCode(code, detail)
}

func writeVerificationForTest(t *testing.T, root, slug, skillName string, rec model.VerificationRecord) {
	t.Helper()

	_, err := SaveVerification(root, slug, skillName, rec)
	require.NoError(t, err)
}

func TestSaveLoadVerificationRoundTrip(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "my-change"
	saveActiveChangeForTest(t, root, slug)

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC().Truncate(time.Second),
		Notes:     "Plan covers all requirements.",
	}
	writeVerificationForTest(t, root, slug, "plan-audit", rec)

	loaded, err := LoadVerification(root, slug, "plan-audit")
	require.NoError(t, err)
	assert.Equal(t, rec.Verdict, loaded.Verdict)
	assert.Equal(t, rec.Notes, loaded.Notes)
	assert.Empty(t, loaded.Blockers)
}

func TestSaveVerificationWritesValidatedRecord(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "save-verification"
	saveActiveChangeForTest(t, root, slug)

	rec := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC().Truncate(time.Second),
		References: []string{"plan-audit:pass"},
		Notes:      "Plan audit passed.",
	}
	path, err := SaveVerification(root, slug, "plan-audit", rec)
	require.NoError(t, err)
	expectedPath, err := NormalizePath(filepath.Join(VerificationDir(root, slug), "plan-audit.yaml"))
	require.NoError(t, err)
	assert.Equal(t, expectedPath, path)

	loaded, err := LoadVerification(root, slug, "plan-audit")
	require.NoError(t, err)
	assert.Equal(t, model.VerificationVerdictPass, loaded.Verdict)
	assert.Empty(t, loaded.Blockers)
	assert.Equal(t, []string{"plan-audit:pass"}, loaded.References)
	assert.Equal(t, "Plan audit passed.", loaded.Notes)
}

func TestSaveVerificationOverwritesExisting(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "overwrite-test"
	saveActiveChangeForTest(t, root, slug)

	rec1 := model.VerificationRecord{
		Verdict:   model.VerificationVerdictFail,
		Blockers:  []model.ReasonCode{testVerificationReason("missing_coverage", "")},
		Timestamp: time.Now().UTC(),
	}
	writeVerificationForTest(t, root, slug, "goal-verification", rec1)

	rec2 := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	}
	writeVerificationForTest(t, root, slug, "goal-verification", rec2)

	loaded, err := LoadVerification(root, slug, "goal-verification")
	require.NoError(t, err)
	assert.Equal(t, model.VerificationVerdictPass, loaded.Verdict)
}

func TestLoadVerificationReturnsErrNotExist(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	_, err := LoadVerification(root, "no-change", "plan-audit")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestLoadVerificationRejectsInvalidRecord(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "bad-record"
	saveActiveChangeForTest(t, root, slug)

	dir := VerificationDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "plan-audit.yaml"),
		[]byte("verdict: maybe\nblockers: []\n"),
		0o644,
	))

	_, err := LoadVerification(root, slug, "plan-audit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid verification")
}

func TestLoadVerificationRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "unknown-field"
	saveActiveChangeForTest(t, root, slug)

	dir := VerificationDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "plan-audit.yaml"),
		[]byte("verdict: pass\nblockers: []\ntimestamp: 2026-04-08T00:00:00Z\nunexpected: true\n"),
		0o644,
	))

	_, err := LoadVerification(root, slug, "plan-audit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field unexpected")
}

func TestLoadVerificationAcceptsStructuredReasonCodes(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "structured-blockers"
	saveActiveChangeForTest(t, root, slug)

	dir := VerificationDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "plan-audit.yaml"),
		[]byte("verdict: fail\nblockers:\n  - code: missing_required_artifact\n    severity: error\n    message: Missing required artifact\n    detail: research.md\ntimestamp: 2026-04-08T00:00:00Z\n"),
		0o644,
	))

	loaded, err := LoadVerification(root, slug, "plan-audit")
	require.NoError(t, err)
	require.Len(t, loaded.Blockers, 1)
	assert.Equal(t, "missing_required_artifact", loaded.Blockers[0].Code)
	assert.Equal(t, "research.md", loaded.Blockers[0].Detail)
}

func TestListVerificationsEmpty(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	result, err := ListVerifications(root, "no-change")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListVerificationsMultiple(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "multi-skill"
	now := time.Now().UTC()
	saveActiveChangeForTest(t, root, slug)

	skills := []string{"plan-audit", "research-orchestration", "wave-orchestration"}
	for _, s := range skills {
		rec := model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: now,
		}
		writeVerificationForTest(t, root, slug, s, rec)
	}

	result, err := ListVerifications(root, slug)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Contains(t, result, "plan-audit")
	assert.Contains(t, result, "research-orchestration")
	assert.Contains(t, result, "wave-orchestration")
}

func TestListVerificationsSkipsWavePlanArtifacts(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "skip-wave-plan"
	saveActiveChangeForTest(t, root, slug)

	writeVerificationForTest(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	})
	require.NoError(t, saveWavePlanForTest(root, slug, model.WavePlan{
		Version:       model.WavePlanVersion,
		GeneratedAt:   time.Now().UTC(),
		TasksPlanHash: "abc123",
		TotalTasks:    1,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Tasks: []model.WavePlanTask{{
				TaskID: "t-01",
			}},
		}},
	}))
	require.NoError(t, os.WriteFile(filepath.Join(VerificationDir(root, slug), "suite-result.yaml"), []byte(`version: 1
run_summary_version: 1
full_suite_digest: "sha256:full-suite"
`), 0o644))

	result, err := ListVerifications(root, slug)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "plan-audit")
	assert.NotContains(t, result, "wave-plan")
	assert.NotContains(t, result, "suite-result")
}

func TestListVerificationsRejectsInvalidFiles(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "skip-bad"
	saveActiveChangeForTest(t, root, slug)

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	}
	writeVerificationForTest(t, root, slug, "plan-audit", rec)

	dir := VerificationDir(root, slug)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "broken.yaml"),
		[]byte("not valid yaml: [[["),
		0o644,
	))

	_, err := ListVerifications(root, slug)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse verification broken")
	var loadErr *VerificationLoadError
	require.ErrorAs(t, err, &loadErr)
	expectedPath := filepath.Join(dir, "broken.yaml")
	normalizedExpectedPath, normalizeErr := NormalizePath(expectedPath)
	if normalizeErr == nil {
		expectedPath = normalizedExpectedPath
	}
	assert.Equal(t, expectedPath, loadErr.Path)
}

func TestVerificationDirPath(t *testing.T) {
	t.Parallel()
	dir := VerificationDir("/project", "my-change")
	assert.Equal(t, filepath.Join("/project", "artifacts", "changes", "my-change", "verification"), dir)
}

func TestVerificationFilePathPath(t *testing.T) {
	t.Parallel()
	path := VerificationFilePath("/project", "my-change", "plan-audit")
	assert.Equal(t, filepath.Join("/project", "artifacts", "changes", "my-change", "verification", "plan-audit.yaml"), path)
}

func TestVerificationFilePathForReadPrefersHiddenSiblingWorktreeBundle(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "hidden-worktree-verification-path"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, PersistScopeWorktreeMetadata(&change, worktreeRoot, "feature"))
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	assert.Equal(
		t,
		filepath.Join(change.WorktreePath, "artifacts", "changes", slug, "verification", "plan-audit.yaml"),
		filepath.Join(verificationDirPathForRead(root, slug), "plan-audit.yaml"),
	)
}

func TestSaveVerificationWithReferencesAndRunVersion(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "full-record"
	saveActiveChangeForTest(t, root, slug)

	rec := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Truncate(time.Second),
		RunVersion: 3,
		References: []string{"layer:CORRECTNESS=pass", "layer:SAFETY=pass"},
		Notes:      "All layers pass.",
	}
	writeVerificationForTest(t, root, slug, "spec-compliance-review", rec)

	loaded, err := LoadVerification(root, slug, "spec-compliance-review")
	require.NoError(t, err)
	assert.Equal(t, 3, loaded.RunVersion)
	assert.Equal(t, []string{"layer:CORRECTNESS=pass", "layer:SAFETY=pass"}, loaded.References)
	assert.Equal(t, "All layers pass.", loaded.Notes)
}

func TestListVerificationsResolvesWorktreeBundleFromProjectRoot(t *testing.T) {
	t.Parallel()
	root := createRuntimeRepoLayout(t)
	worktreeRoot := addGitWorktree(t, root, "worktree-verification-branch")
	slug := "worktree-verification"

	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	require.NoError(t, SaveChange(root, change))

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}

	// Agents operating inside the bound worktree write verification beside the
	// canonical governed bundle there, not under the project root.
	writeVerificationForTest(t, worktreeRoot, slug, "plan-audit", rec)

	result, err := ListVerifications(root, slug)
	require.NoError(t, err)
	require.Contains(t, result, "plan-audit")
}

func TestLoadVerificationRejectsHiddenSiblingWorktreeFallback(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "hidden-worktree-verification-read"

	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	staleRootDir := filepath.Join(root, "artifacts", "changes", slug, "verification")
	require.NoError(t, os.MkdirAll(staleRootDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleRootDir, "plan-audit.yaml"), []byte(`verdict: pass
timestamp: 2026-04-06T00:00:00Z
`), 0o644))

	_, err := LoadVerification(root, slug, "plan-audit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authoritative bundle")
}

func TestListVerificationsRejectsHiddenSiblingWorktreeFallback(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "hidden-worktree-verification-list"

	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	staleRootDir := filepath.Join(root, "artifacts", "changes", slug, "verification")
	require.NoError(t, os.MkdirAll(staleRootDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleRootDir, "plan-audit.yaml"), []byte(`verdict: pass
timestamp: 2026-04-06T00:00:00Z
`), 0o644))

	_, err := ListVerifications(root, slug)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authoritative bundle")
}

func TestFindSiblingBundleDirRegardlessOfVisibilityIgnoresOrphanBundleWithoutAuthorityFile(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "orphan-sibling-bundle"
	orphanDir := filepath.Join(worktreeRoot, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))

	got, err := findSiblingBundleDirRegardlessOfVisibility(root, slug)
	require.NoError(t, err)
	assert.Empty(t, got)
}
