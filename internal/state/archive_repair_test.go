package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairInterruptedTerminalArchiveGovernedCancel(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	admission := model.NewAdmissionState(requestID)
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	admission.AdmissionStatus = model.AdmissionStatusSealedHandoff
	admission.CurrentState = model.StateS1Analyze
	require.NoError(t, SaveAdmission(root, admission))

	change := model.NewChangeState(requestID, "my-change")
	change.Level = model.LevelL2
	change.LevelSource = model.LevelSourceAuto
	change.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	change.ChangeStatus = model.ChangeStatusCancelled
	change.Artifacts = map[string]model.ArtifactState{
		"proposal": {ID: "proposal", State: model.ArtifactLifecycleFresh},
	}
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "aircraft", "changes", change.Slug), 0o755))

	repaired, err := RepairInterruptedTerminalArchive(root, requestID)
	require.NoError(t, err)
	assert.True(t, repaired)

	_, err = os.Stat(ChangePath(root, requestID))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	archived, err := LoadArchivedChange(root, requestID)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusCancelled, archived.ChangeStatus)
	assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["proposal"].State)
}

func TestRepairInterruptedTerminalArchiveDirectDone(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	admission := model.NewAdmissionState(requestID)
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	admission.AdmissionStatus = model.AdmissionStatusDone
	admission.CurrentState = model.StateDone
	require.NoError(t, SaveAdmission(root, admission))

	repaired, err := RepairInterruptedTerminalArchive(root, requestID)
	require.NoError(t, err)
	assert.True(t, repaired)

	_, err = os.Stat(AdmissionPath(root, requestID))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(ArchiveAdmissionPath(root, requestID))
	require.NoError(t, err)
}
