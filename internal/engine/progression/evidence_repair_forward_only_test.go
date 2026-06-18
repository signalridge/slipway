package progression

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

func TestS3DefersStalePlanningEvidenceToReviewWithoutMutatingAuthorityOrEvidence(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))

	change := model.NewChange("stale-repair-transaction-rollback")
	change.CurrentState = model.StateS3Review
	change.WorkflowPreset = model.WorkflowPresetLight
	require.NoError(t, state.SaveChange(root, change))
	writeValidPlanningBundleForTransactionTest(t, root, change)

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, record)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillPlanAudit, record, nil))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	verificationPath := filepath.Join(bundleDir, "verification", SkillPlanAudit+".yaml")
	digestsPath := filepath.Join(bundleDir, "verification", state.EvidenceDigestsFileName)

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

## Task List

- [ ] `+"`t-01`"+` changed task plan
  - target_files: ["internal/fsutil/transaction.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))

	summary, err := AdvanceGoverned(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, "blocked", summary.Action)
	assert.Empty(t, summary.Reason)
	assert.Contains(t, model.ReasonSpecs(summary.Blockers), "required_skill_missing:spec-compliance-review")
	assert.NotContains(t, strings.Join(model.ReasonSpecs(summary.Blockers), "\n"), "required_skill_stale:plan-audit")

	_, statErr := os.Stat(verificationPath)
	require.NoError(t, statErr)
	_, statErr = os.Stat(digestsPath)
	require.NoError(t, statErr)

	digests, loadErr := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, loadErr)
	assert.Contains(t, digests.Skills, SkillPlanAudit)

	reloaded, loadErr := state.LoadChange(root, change.Slug)
	require.NoError(t, loadErr)
	assert.Equal(t, model.StateS3Review, reloaded.CurrentState)
}
