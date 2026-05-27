package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateIncludesScopeContractDriftReport(t *testing.T) {
	t.Parallel()

	root, slug := writeScopeContractDriftFixture(t)

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	require.NotNil(t, view.ScopeContract)
	assert.Equal(t, "fail", view.ScopeContract.Status)
	assert.Equal(t, []string{"cmd/review.go"}, view.ScopeContract.OutOfScopeFiles)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "scope_contract_drift:cmd/review.go")
}

func TestStatusSurfacesScopeContractDriftBlocker(t *testing.T) {
	t.Parallel()

	root, slug := writeScopeContractDriftFixture(t)
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.ScopeContract)
	assert.Equal(t, "fail", view.ScopeContract.Status)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "scope_contract_drift:cmd/review.go")
}

func TestReviewFailsOnScopeContractDrift(t *testing.T) {
	t.Parallel()

	root, slug := writeScopeContractDriftFixture(t)
	writePassingWaveEvidence(t, root, slug, 1)

	view, err := buildReviewViewForSlug(root, slug, false, "", nil)
	require.NoError(t, err)
	require.NotNil(t, view.ScopeContract)
	assert.Equal(t, "fail", view.Verdict)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "scope_contract_drift:cmd/review.go")
	assert.Contains(t, strings.Join(view.Gaps.ArtifactToCode, "\n"), "scope_contract_drift")
}

func writeScopeContractDriftFixture(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "scope contract drift fixture")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement validation surface
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/validate.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))

	now := time.Now().UTC()
	writeExecutionSummary(t, root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "t-01",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/review.go"},
				TargetFiles:  []string{"cmd/validate.go"},
				CapturedAt:   now,
			},
		},
	})

	return root, slug
}
