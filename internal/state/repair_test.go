package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAdmissionRejectsMutationWhenSealed(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	admission := model.NewAdmissionState(requestID)
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	admission.AdmissionStatus = model.AdmissionStatusSealedHandoff
	admission.CurrentState = model.StateS1Analyze
	now := time.Now().UTC()
	admission.SealedAt = &now
	require.NoError(t, SaveAdmission(root, admission))

	mutated := admission
	mutated.CurrentState = model.StateS6RunWaves
	err := SaveAdmission(root, mutated)
	require.ErrorIs(t, err, ErrSealedAdmissionImmutable)
}

func TestApplyLevelPivotUpdatesTopLevelMetadata(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "my-change")
	change.Level = model.LevelL2
	change.LevelSource = model.LevelSourceAuto
	change.LevelHistory = []model.LevelHistoryEvent{
		{Level: model.LevelL2, LevelSource: model.LevelSourceAuto, At: time.Now().Add(-time.Hour)},
	}

	manifest := ChangeManifest{CreatedAtLevel: model.LevelL2}
	at := time.Now().UTC()
	err := ApplyLevelPivot(&change, model.LevelL3, model.LevelSourceUserSelected, "pivot", at, 100)
	require.NoError(t, err)

	assert.Equal(t, model.LevelL3, change.Level)
	assert.Equal(t, model.LevelSourceUserSelected, change.LevelSource)
	require.NotNil(t, change.LastLevelUpdateAt)
	assert.Equal(t, at, *change.LastLevelUpdateAt)
	assert.Len(t, change.LevelHistory, 2)
	assert.Equal(t, model.LevelL2, manifest.CreatedAtLevel)
}

func TestApplyLevelPivotTruncatesLevelHistory(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "my-change")
	change.Level = model.LevelL1
	change.LevelSource = model.LevelSourceAuto
	change.LevelHistory = []model.LevelHistoryEvent{
		{Level: model.LevelL1, LevelSource: model.LevelSourceAuto, At: time.Now().Add(-4 * time.Hour)},
		{Level: model.LevelL1, LevelSource: model.LevelSourceAuto, At: time.Now().Add(-3 * time.Hour)},
	}

	require.NoError(t, ApplyLevelPivot(&change, model.LevelL2, model.LevelSourceAuto, "pivot-1", time.Now().Add(-2*time.Hour), 2))
	require.NoError(t, ApplyLevelPivot(&change, model.LevelL3, model.LevelSourceAuto, "pivot-2", time.Now().Add(-time.Hour), 2))

	assert.Len(t, change.LevelHistory, 2)
	assert.Equal(t, model.LevelL2, change.LevelHistory[0].Level)
	assert.Equal(t, model.LevelL3, change.LevelHistory[1].Level)
}

func TestRepairCorruptConfigBacksUpAndRewritesDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".spln", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("defaults: ["), 0o644))

	now := time.Date(2026, 3, 4, 1, 2, 3, 0, time.UTC)
	backupPath, err := RepairCorruptConfig(root, now)
	require.NoError(t, err)

	_, err = os.Stat(backupPath)
	require.NoError(t, err)

	cfg, err := model.LoadConfig(configPath)
	require.NoError(t, err)
	mode, fallback := cfg.EffectiveLevelMode()
	assert.Equal(t, model.LevelModeAuto, mode)
	assert.False(t, fallback)
}

func TestRunEvidenceRetentionGCExcludesActiveRequestEvidence(t *testing.T) {
	root := createRuntimeLayout(t)
	activeID := mustRequestID(t)
	inactiveID := mustRequestID(t)

	activeAdmission := model.NewAdmissionState(activeID)
	activeAdmission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, activeAdmission))

	inactiveAdmission := model.NewAdmissionState(inactiveID)
	inactiveAdmission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	inactiveAdmission.AdmissionStatus = model.AdmissionStatusDone
	inactiveAdmission.CurrentState = model.StateDone
	require.NoError(t, SaveAdmission(root, inactiveAdmission))

	activeEvidence := filepath.Join(root, ".spln", "evidence", "tasks", activeID, "rv1", "t1.json")
	inactiveEvidence := filepath.Join(root, ".spln", "evidence", "tasks", inactiveID, "rv1", "t1.json")
	for _, p := range []string{activeEvidence, inactiveEvidence} {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("{}"), 0o644))
	}

	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(activeEvidence, old, old))
	require.NoError(t, os.Chtimes(inactiveEvidence, old, old))

	result, err := RunEvidenceRetentionGC(root, 1, time.Now())
	require.NoError(t, err)
	assert.Contains(t, result.DeletedPaths, inactiveEvidence)

	_, err = os.Stat(activeEvidence)
	require.NoError(t, err)
	_, err = os.Stat(inactiveEvidence)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}
