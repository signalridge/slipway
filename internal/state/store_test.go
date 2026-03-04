package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadAdmissionRoundTrip(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	st := model.NewAdmissionState(requestID)
	st.CurrentState = model.StateS1Analyze
	st.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{
		Novelty:           1,
		Ambiguity:         1,
		Impact:            1,
		Risk:              1,
		ReversibilityCost: 1,
	}}
	st.IntakeAssessment = model.IntakeAssessment{
		IntentType:       "executable_change",
		IsExecutable:     true,
		Confidence:       0.9,
		ChangeTargets:    []string{"internal/model"},
		IntendedDelta:    "add model",
		AcceptanceAnchor: "tests pass",
		BlockingUnknowns: []string{},
	}

	require.NoError(t, SaveAdmission(root, st))

	loaded, err := LoadAdmission(root, requestID)
	require.NoError(t, err)
	assert.Equal(t, st.RequestID, loaded.RequestID)
	assert.Equal(t, st.CurrentState, loaded.CurrentState)
	assert.Equal(t, st.IntakeAssessment.IntentType, loaded.IntakeAssessment.IntentType)
	assert.NotNil(t, loaded.EvidenceRefs)
	assert.NotNil(t, loaded.LevelHistory)
}

func TestSaveLoadChangeRoundTrip(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	st := model.NewChangeState(requestID, "my-change")
	st.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveChange(root, st))

	loaded, err := LoadChange(root, requestID)
	require.NoError(t, err)
	assert.Equal(t, st.RequestID, loaded.RequestID)
	assert.Equal(t, st.Slug, loaded.Slug)
	assert.NotNil(t, loaded.EvidenceRefs)
	assert.NotNil(t, loaded.LevelHistory)
}

func TestResolveActiveRequestSingleAdmission(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	st := model.NewAdmissionState(requestID)
	st.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, st))

	resolved, err := ResolveActiveRequest(root)
	require.NoError(t, err)
	assert.Equal(t, requestID, resolved.RequestID)
	assert.Equal(t, ActiveResolutionModeAdmissionOnly, resolved.Mode)
}

func TestResolveActiveRequestNoActive(t *testing.T) {
	root := createRuntimeLayout(t)
	_, err := ResolveActiveRequest(root)
	require.ErrorIs(t, err, ErrNoActiveRequest)
}

func TestResolveActiveRequestDualActiveSameRequest(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	ad := model.NewAdmissionState(requestID)
	ad.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, ad))

	ch := model.NewChangeState(requestID, "my-change")
	ch.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveChange(root, ch))

	_, err := ResolveActiveRequest(root)
	require.ErrorIs(t, err, ErrSameRequestDualActive)
}

func TestResolveActiveRequestMultipleRequests(t *testing.T) {
	root := createRuntimeLayout(t)

	a := model.NewAdmissionState(mustRequestID(t))
	a.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, a))

	b := model.NewAdmissionState(mustRequestID(t))
	b.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, b))

	_, err := ResolveActiveRequest(root)
	require.ErrorIs(t, err, ErrMultipleActiveRequests)
}

func TestDiscoverActiveRecordsDiagnostics(t *testing.T) {
	root := createRuntimeLayout(t)

	st := model.NewAdmissionState(mustRequestID(t))
	st.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, SaveAdmission(root, st))

	records, err := DiscoverActiveRecords(root)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, LaneAdmission, records[0].Lane)
}

func TestSaveAdmissionExecutableRequiresRouteSnapshot(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	st := model.NewAdmissionState(requestID)
	st.IntakeAssessment = model.IntakeAssessment{
		IntentType:       "executable_change",
		IsExecutable:     true,
		Confidence:       0.9,
		ChangeTargets:    []string{"workspace"},
		IntendedDelta:    "fix auth middleware",
		AcceptanceAnchor: "tests green",
	}
	st.RouteSnapshot = model.RouteSnapshot{}

	err := SaveAdmission(root, st)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route_snapshot is required")
}

func TestRouteSnapshotSchemaRoundTrip(t *testing.T) {
	root := createRuntimeLayout(t)
	requestID := mustRequestID(t)

	st := model.NewAdmissionState(requestID)
	st.IntakeAssessment = model.IntakeAssessment{
		IntentType:       "executable_change",
		IsExecutable:     true,
		Confidence:       0.9,
		ChangeTargets:    []string{"workspace"},
		IntendedDelta:    "fix auth middleware",
		AcceptanceAnchor: "tests green",
	}
	st.RouteSnapshot = model.RouteSnapshot{
		Scores: model.Scores{
			Novelty:           2,
			Ambiguity:         3,
			Impact:            3,
			Risk:              2,
			ReversibilityCost: 2,
		},
		GuardrailDomain:   "security_credentials",
		RoutingRationale:  []string{"auto_route:guardrail_floor"},
		BlockingConflicts: []string{"fixed_level_guardrail_conflict"},
	}
	require.NoError(t, SaveAdmission(root, st))

	loaded, err := LoadAdmission(root, requestID)
	require.NoError(t, err)
	assert.Equal(t, st.RouteSnapshot.Scores, loaded.RouteSnapshot.Scores)
	assert.Equal(t, st.RouteSnapshot.GuardrailDomain, loaded.RouteSnapshot.GuardrailDomain)
	assert.Equal(t, st.RouteSnapshot.RoutingRationale, loaded.RouteSnapshot.RoutingRationale)
	assert.Equal(t, st.RouteSnapshot.BlockingConflicts, loaded.RouteSnapshot.BlockingConflicts)

	raw, err := os.ReadFile(AdmissionPath(root, requestID))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "required_artifacts")
	assert.NotContains(t, string(raw), "required_gates")
	assert.NotContains(t, string(raw), "required_skills")
}

func createRuntimeLayout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".spln", "runtime", "admissions"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".spln", "runtime", "changes"), 0o755))
	return root
}

func mustRequestID(t *testing.T) string {
	t.Helper()
	id, err := model.NewRequestID()
	require.NoError(t, err)
	return id
}
