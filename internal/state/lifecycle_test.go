package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHandoffAdmissionToGovernedSealsAdmissionAndKeepsLaneLocalTraces(t *testing.T) {
	admission := model.NewAdmissionState(mustRequestID(t))
	admission.CurrentState = model.StateS6RunWaves
	admission.Level = model.LevelL1
	admission.LevelSource = model.LevelSourceUserSelected
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	admission.TaskRuns = map[string]model.TaskRun{
		"t1__rv1": {
			TaskID:            "t1",
			RunSummaryVersion: 1,
			Verdict:           model.TaskVerdictPass,
		},
	}
	admission.ActionHistory = []model.ActionEvent{{Action: "do", State: model.StateS6RunWaves}}
	admission.EvidenceRefs = map[string]string{"non-task": ".spln/evidence/skills/e1.json"}

	sealed, change, err := HandoffAdmissionToGoverned(admission, "my-change", model.LevelL2)
	require.NoError(t, err)

	assert.Equal(t, admission.RequestID, change.RequestID)
	assert.Equal(t, model.ChangeStatusActive, change.ChangeStatus)
	assert.Equal(t, model.StateS4SpecBundle, change.CurrentState)
	assert.Empty(t, change.TaskRuns)
	assert.Empty(t, change.ActionHistory)
	assert.NotNil(t, change.EvidenceRefs)

	assert.Equal(t, model.AdmissionStatusSealedHandoff, sealed.AdmissionStatus)
	assert.Equal(t, model.StateS1Analyze, sealed.CurrentState)
	require.NotNil(t, sealed.SealedAt)
	assert.Equal(t, admission.TaskRuns, sealed.TaskRuns)
	assert.Equal(t, admission.ActionHistory, sealed.ActionHistory)
}

func TestArchiveGovernedDoneFreezesArtifactsAndMigratesPaths(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	admission := model.NewAdmissionState(requestID)
	admission.AdmissionStatus = model.AdmissionStatusSealedHandoff
	admission.CurrentState = model.StateS1Analyze
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, admission))

	change := model.NewChangeState(requestID, "my-change")
	change.Level = model.LevelL2
	change.LevelSource = model.LevelSourceAuto
	change.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	change.Artifacts = map[string]model.ArtifactState{
		"proposal": {ID: "proposal", State: model.ArtifactLifecycleFresh},
		"spec":     {ID: "spec", State: model.ArtifactLifecycleStale},
	}
	require.NoError(t, SaveChange(root, change))

	artifactDir := filepath.Join(root, "aircraft", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(artifactDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(artifactDir, "change.yaml"), []byte("id: x"), 0o644))

	archived, err := ArchiveGoverned(root, change, &admission, model.ChangeStatusDone)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusDone, archived.ChangeStatus)
	assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["proposal"].State)
	assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["spec"].State)

	_, err = os.Stat(ChangePath(root, requestID))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(AdmissionPath(root, requestID))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(root, "aircraft", "changes", change.Slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(ArchiveChangePath(root, requestID))
	require.NoError(t, err)
	_, err = os.Stat(ArchiveAdmissionPath(root, requestID))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, "aircraft", "changes", "archived", change.Slug, "change.yaml"))
	require.NoError(t, err)
}

func TestArchiveGovernedDoneRequiresActiveStatus(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)
	change := model.NewChangeState(requestID, "my-change")
	change.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	change.ChangeStatus = model.ChangeStatusCancelled

	_, err := ArchiveGoverned(root, change, nil, model.ChangeStatusDone)
	require.Error(t, err)
}

func TestArchiveGovernedCancelAcceptsCancelled(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	change := model.NewChangeState(requestID, "my-change")
	change.Level = model.LevelL2
	change.LevelSource = model.LevelSourceAuto
	change.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	change.ChangeStatus = model.ChangeStatusCancelled
	change.Artifacts = map[string]model.ArtifactState{
		"proposal": {ID: "proposal", State: model.ArtifactLifecycleFresh},
	}
	require.NoError(t, SaveChange(root, change))

	artifactDir := filepath.Join(root, "aircraft", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(artifactDir, 0o755))

	archived, err := ArchiveGoverned(root, change, nil, model.ChangeStatusCancelled)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusCancelled, archived.ChangeStatus)
	assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["proposal"].State)
}

func TestArchiveDirectAdmissionMovesRuntimeToArchive(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	admission := model.NewAdmissionState(requestID)
	admission.AdmissionStatus = model.AdmissionStatusDone
	admission.CurrentState = model.StateDone
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, admission))

	require.NoError(t, ArchiveDirectAdmission(root, admission))

	_, err := os.Stat(AdmissionPath(root, requestID))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(ArchiveAdmissionPath(root, requestID))
	require.NoError(t, err)
}

func TestRouteSnapshotDurabilityAcrossHandoff(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	admission := model.NewAdmissionState(requestID)
	admission.Level = model.LevelL2
	admission.LevelSource = model.LevelSourceAuto
	admission.CurrentState = model.StateS6RunWaves
	admission.RouteSnapshot = model.RouteSnapshot{
		Scores: model.Scores{
			Novelty:           2,
			Ambiguity:         2,
			Impact:            3,
			Risk:              2,
			ReversibilityCost: 2,
		},
		GuardrailDomain:   "security_credentials",
		RoutingRationale:  []string{"route:auto"},
		BlockingConflicts: []string{"needs_review"},
	}
	admission.IntakeAssessment = model.IntakeAssessment{
		IntentType:       "executable_change",
		IsExecutable:     true,
		Confidence:       0.9,
		ChangeTargets:    []string{"workspace"},
		IntendedDelta:    "update auth middleware",
		AcceptanceAnchor: "tests green",
		BlockingUnknowns: []string{},
	}
	require.NoError(t, SaveAdmission(root, admission))

	sealed, change, err := HandoffAdmissionToGoverned(admission, "my-change", model.LevelL2)
	require.NoError(t, err)
	require.NoError(t, SaveAdmission(root, sealed))
	require.NoError(t, SaveChange(root, change))

	loadedAdmission, err := LoadAdmission(root, requestID)
	require.NoError(t, err)
	loadedChange, err := LoadChange(root, requestID)
	require.NoError(t, err)
	assert.Equal(t, admission.RouteSnapshot, loadedAdmission.RouteSnapshot)
	assert.Equal(t, admission.RouteSnapshot, loadedChange.RouteSnapshot)
	assert.Equal(t, admission.IntakeAssessment, loadedAdmission.IntakeAssessment)
}

func TestIntakeAssessmentPersistsThroughGovernedArchive(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	admission := model.NewAdmissionState(requestID)
	admission.Level = model.LevelL2
	admission.LevelSource = model.LevelSourceAuto
	admission.AdmissionStatus = model.AdmissionStatusSealedHandoff
	admission.CurrentState = model.StateS1Analyze
	now := time.Now().UTC()
	admission.SealedAt = &now
	admission.IntakeAssessment = model.IntakeAssessment{
		IntentType:       "executable_change",
		IsExecutable:     true,
		Confidence:       0.88,
		ChangeTargets:    []string{"service/auth"},
		IntendedDelta:    "harden auth policy",
		AcceptanceAnchor: "security checks pass",
		BlockingUnknowns: []string{"clarify migration order"},
	}
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{
		Novelty:           2,
		Ambiguity:         2,
		Impact:            3,
		Risk:              2,
		ReversibilityCost: 2,
	}}
	require.NoError(t, SaveAdmission(root, admission))

	change := model.NewChangeState(requestID, "my-change")
	change.Level = model.LevelL2
	change.LevelSource = model.LevelSourceAuto
	change.RouteSnapshot = admission.RouteSnapshot
	change.CurrentState = model.StateS8Verify
	change.Gates[string("G_ship")] = model.GateRecord{
		GateID:   "G_ship",
		Status:   model.GateStatusApproved,
		Decision: model.GateDecisionApprove,
	}
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "aircraft", "changes", change.Slug), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "aircraft", "changes", change.Slug, "change.yaml"), []byte("x"), 0o644))

	_, err := ArchiveGoverned(root, change, &admission, model.ChangeStatusDone)
	require.NoError(t, err)

	raw, err := os.ReadFile(ArchiveAdmissionPath(root, requestID))
	require.NoError(t, err)
	var archived model.AdmissionState
	require.NoError(t, yaml.Unmarshal(raw, &archived))
	assert.Equal(t, admission.IntakeAssessment, archived.IntakeAssessment)
}
