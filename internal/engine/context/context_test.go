package context

import (
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAdmissionPack(t *testing.T) {
	admission := model.NewAdmissionState(mustRequestID(t))
	admission.Level = model.LevelL1
	admission.LevelSource = model.LevelSourceUserSelected
	admission.CurrentState = model.StateS6RunWaves
	admission.IntakeAssessment = model.IntakeAssessment{
		IntendedDelta: "refactor auth middleware",
		ChangeTargets: []string{"internal/auth/middleware.go"},
	}
	admission.RouteSnapshot = model.RouteSnapshot{
		Scores:           model.Scores{},
		RoutingRationale: []string{"auto_route:default_l1"},
	}

	pack := BuildAdmissionPack(
		admission,
		".spln/runtime/admissions/"+admission.RequestID+".yaml",
		[]string{"do"},
		[]string{"missing_evidence_ref:task-1"},
		EvidenceFreshnessFresh,
	)

	assert.Equal(t, LaneModeAdmissionOnly, pack.LaneMode)
	assert.Equal(t, EvidenceFreshnessFresh, pack.EvidenceFreshness)
	assert.Equal(t, "refactor auth middleware", pack.IntentSummary)
	assert.Equal(t, []string{"internal/auth/middleware.go"}, pack.ScopeFiles)
	assert.Equal(t, []string{"do"}, pack.NextReadyActions)
	assert.Equal(t, []string{"missing_evidence_ref:task-1"}, pack.Blockers)
}

func TestBuildGovernedAndDiagnosticsPack(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "slug")
	change.Level = model.LevelL3
	change.LevelSource = model.LevelSourceAuto
	change.CurrentState = model.StateS7Review
	change.RouteSnapshot = model.RouteSnapshot{
		Scores:           model.Scores{},
		RoutingRationale: []string{"auto_route:guardrail_floor"},
	}

	governed := BuildGovernedPack(
		change,
		".spln/runtime/changes/"+change.RequestID+".yaml",
		[]string{"review"},
		nil,
		"tighten pii controls",
		[]string{"internal/privacy/service.go"},
		EvidenceFreshnessStale,
	)
	assert.Equal(t, LaneModeGoverned, governed.LaneMode)
	assert.Equal(t, EvidenceFreshnessStale, governed.EvidenceFreshness)

	diag := BuildDiagnosticsPack([]string{"run spln repair"})
	assert.Equal(t, LaneModeDiagnostics, diag.LaneMode)
	assert.Equal(t, EvidenceFreshnessUnknown, diag.EvidenceFreshness)
	assert.Equal(t, []string{"run spln repair"}, diag.Remediation)
}

func TestInjectSubagentContextScopesToTaskAndGeneratesUniqueSession(t *testing.T) {
	base := BuildDiagnosticsPack(nil)
	envelope := WaveEnvelope{
		WaveID:      "wave-1",
		TaskID:      "task-a",
		DependsOn:   []string{"task-0"},
		TargetFiles: []string{"internal/a.go"},
		TaskKind:    model.TaskKindImplementation,
		Autonomous:  true,
		MustHaves:   []string{"tests_pass"},
	}

	ctxA, err := InjectSubagentContext(base, envelope, []string{"spln-tdd"})
	require.NoError(t, err)
	ctxB, err := InjectSubagentContext(base, envelope, []string{"spln-tdd"})
	require.NoError(t, err)

	assert.True(t, model.IsUUIDv7(ctxA.SessionID))
	assert.True(t, model.IsUUIDv7(ctxB.SessionID))
	assert.NotEqual(t, ctxA.SessionID, ctxB.SessionID)
	assert.Equal(t, []string{"internal/a.go"}, ctxA.TaskScopeFiles)
	assert.Equal(t, []string{"internal/a.go"}, ctxA.Pack.ScopeFiles)
	require.NotNil(t, ctxA.Pack.WaveEnvelope)
	assert.Equal(t, "task-a", ctxA.Pack.WaveEnvelope.TaskID)
}

func TestCheckpointResumeBundleAttach(t *testing.T) {
	pack := BuildDiagnosticsPack(nil)
	bundle := BuildCheckpointResumeBundle(
		"run-123",
		"task-1",
		"approval_required",
		"yes proceed",
		[]string{"need_user_confirmation"},
	)
	require.NoError(t, AttachCheckpointResume(&pack, bundle))
	require.NotNil(t, pack.CheckpointResume)
	assert.Equal(t, "run-123", pack.CheckpointResume.PriorRunID)
	assert.Equal(t, []string{"need_user_confirmation"}, pack.CheckpointResume.PauseBlockers)
}

func TestEvaluateEvidenceFreshness(t *testing.T) {
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

func mustRequestID(t *testing.T) string {
	t.Helper()
	id, err := model.NewRequestID()
	require.NoError(t, err)
	return id
}
