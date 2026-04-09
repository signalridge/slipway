package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDoneArchivesGovernedExecution(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := makeDoneCmd()
		require.NoError(t, doneCmd.Execute())

		// Verify the change was archived.
		_, err = state.LoadChange(root, slug)
		require.Error(t, err)
	})
}

func TestDoneArchivesGovernedAsTerminalDoneState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "archive terminal governed state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := makeDoneCmd()
		require.NoError(t, doneCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusDone, archived.Status)
		assert.Equal(t, model.StateDone, archived.CurrentState)
	})
}

func TestDoneGovernedEmptyAssuranceReturnsInvalid(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		// Write an empty assurance.md (missing required headings).
		writeAssuranceMD(t, root, change.Slug, "")

		doneCmd := makeDoneCmd()
		err = doneCmd.Execute()
		require.Error(t, err)
		var cliErr *CLIError
		require.ErrorAs(t, err, &cliErr)
		assert.Equal(t, "assurance_invalid", cliErr.ErrorCode)
	})
}

func TestDoneGovernedValidAssuranceSucceeds(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := makeDoneCmd()
		require.NoError(t, doneCmd.Execute())
	})
}

func TestDoneLightPresetAllowsMissingAssurance(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := makeNewCmd()
		create.SetArgs([]string{"--preset", "light", "rename helper comment"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowPresetLight, change.WorkflowPreset)

		markChangeReadyForDone(t, root, &change)

		doneCmd := makeDoneCmd()
		require.NoError(t, doneCmd.Execute())

		_, err = state.LoadChange(root, slug)
		require.Error(t, err)
	})
}

func TestDoneQuickFullRevalidatesShipGateBeforeArchive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "quick full closeout must be fresh")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.QualityMode = model.QualityModeFull
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		doneCmd := makeDoneCmd()
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRequiresReviewEvidenceBeforeArchive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review evidence must be fresh")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		doneCmd := makeDoneCmd()
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		raw, readErr := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "change.yaml"))
		require.NoError(t, readErr)
		assert.NotContains(t, string(raw), "gates:")
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsReviewLayerBlockersBeforeArchive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review layer blockers must stop done")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)
		writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{"layer:R0=pass"},
		})

		doneCmd := makeDoneCmd()
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)
		assert.Contains(t, strings.ToLower(cliErr.Message), "review_layer_missing:ir1")

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsExecutionSummaryLevelBlockersBeforeArchive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "summary blockers must stop done")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		const blocker = "session_isolation_warning:session_id=abc:shared_by=task-a,task-b"
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        time.Now().UTC(),
			OverallVerdict:    model.ExecutionVerdictFail,
			OpenBlockers:      []model.ReasonCode{model.ReasonCodeFromSpec(blocker)},
		})
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		doneCmd := makeDoneCmd()
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Message, "session_isolation_warning")
		assert.Contains(t, cliErr.Message, "stale_execution_evidence")

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsChecklistBlockersBeforeArchive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "tasks checklist blockers must stop done")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte("# Tasks\n"), 0o644))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := makeDoneCmd()
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Message, "tasks_checklist_empty")

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsPlanAuditChanges(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "plan audit change")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		doneCmd := makeDoneCmd()
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "not_done_ready", cliErr.ErrorCode)
	})
}

func TestDoneRejectsAllReadyWithExplicitRequest(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		doneCmd := makeDoneCmd()
		doneCmd.SetArgs([]string{"--all-ready", "--change", "some-slug"})
		err := doneCmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "flag_conflict", cliErr.ErrorCode)
		assert.Equal(t, categoryInvalidUsage, cliErr.Category)
		assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
	})
}

func TestDoneRejectsMalformedConfigBeforeLockProtectedMutation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createActiveNonDiscoveryChange(t, root, "done malformed config guard")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("{invalid"), 0o644))

		doneCmd := makeDoneCmd()
		doneCmd.SetArgs([]string{"--change", slug})
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "config_parse_failure", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
	})
}

func TestDoneAllReadyArchivesEligibleChanges(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		readyL1 := model.NewChange("bulk-l1-ready")
		markChangeReadyForDone(t, root, &readyL1)
		writeAssuranceMD(t, root, readyL1.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, readyL1.Slug, 1, "t-01")

		readyL2 := model.NewChange("bulk-l2-ready")
		markChangeReadyForDone(t, root, &readyL2)
		writeAssuranceMD(t, root, readyL2.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, readyL2.Slug, 1, "t-01")

		notReady := model.NewChange("bulk-not-ready")
		notReady.CurrentState = model.StateS2Execute
		notReady.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, notReady))

		view := archiveAllDoneReady(root)
		require.Len(t, view.Archived, 2)
		assert.Equal(t, []doneBulkItem{
			newDoneBulkArchived("bulk-l1-ready"),
			newDoneBulkArchived("bulk-l2-ready"),
		}, view.Archived)
		require.Len(t, view.Skipped, 1)
		assert.Equal(t, newDoneBulkSkipped("bulk-not-ready", string(model.StateS2Execute), "not_done_ready"), view.Skipped[0])
		assert.Empty(t, view.Failed)

		_, err := state.LoadArchivedChange(root, readyL1.Slug)
		require.NoError(t, err)
		_, err = state.LoadArchivedChange(root, readyL2.Slug)
		require.NoError(t, err)
		_, err = state.LoadChange(root, notReady.Slug)
		require.NoError(t, err)
	})
}

func TestDoneAllReadySkipsShipGateBlockedChanges(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		ready := model.NewChange("bulk-ready")
		markChangeReadyForDone(t, root, &ready)
		writeAssuranceMD(t, root, ready.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, ready.Slug, 1, "t-01")

		blocked := model.NewChange("bulk-ship-blocked")
		blocked.CurrentState = model.StateS4Verify
		blocked.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, blocked))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, blocked, ""))
		writePassingExecutionSummary(t, root, blocked.Slug, 1, "t-01")
		writePassingWaveEvidence(t, root, blocked.Slug, 1)
		writePassingGoalVerificationEvidence(t, root, blocked.Slug, 1)
		writeAssuranceMD(t, root, blocked.Slug, validAssuranceContent())

		view := archiveAllDoneReady(root)
		require.Len(t, view.Archived, 1)
		assert.Equal(t, newDoneBulkArchived("bulk-ready"), view.Archived[0])
		require.Len(t, view.Skipped, 1)
		assert.Equal(t, "bulk-ship-blocked", view.Skipped[0].Slug)
		assert.Equal(t, string(model.StateS4Verify), view.Skipped[0].Status)
		assert.Equal(t, "ship_gate_blocked", view.Skipped[0].Reason)
		assert.Contains(t, model.ReasonSpecs(view.Skipped[0].ReasonCodes), "required_skill_missing:code-quality-review")
		assert.Contains(t, model.ReasonSpecs(view.Skipped[0].ReasonCodes), "required_skill_missing:spec-compliance-review")
		assert.Empty(t, view.Failed)

		_, err := state.LoadArchivedChange(root, ready.Slug)
		require.NoError(t, err)
		_, err = state.LoadChange(root, blocked.Slug)
		require.NoError(t, err)
	})
}

func TestDoneAllReadyRespectsPerChangeLocks(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		lockedChange := model.NewChange("bulk-locked")
		markChangeReadyForDone(t, root, &lockedChange)
		writeAssuranceMD(t, root, lockedChange.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, lockedChange.Slug, 1, "t-01")

		readyChange := model.NewChange("bulk-ready")
		markChangeReadyForDone(t, root, &readyChange)
		writeAssuranceMD(t, root, readyChange.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, readyChange.Slug, 1, "t-01")

		stopLockHolder := startStateLockHolder(t, state.ChangeStateLockPath(root, lockedChange.Slug))
		defer stopLockHolder()

		view := archiveAllDoneReady(root)
		require.Len(t, view.Archived, 1)
		assert.Equal(t, newDoneBulkArchived("bulk-ready"), view.Archived[0])
		require.Len(t, view.Failed, 1)
		assert.Equal(t, "bulk-locked", view.Failed[0].Slug)
		assert.Equal(t, "state_lock_timeout", view.Failed[0].Reason)
		assert.Contains(t, strings.ToLower(view.Failed[0].ErrorDetail), "state lock timeout")

		_, err := state.LoadArchivedChange(root, readyChange.Slug)
		require.NoError(t, err)
		_, err = state.LoadChange(root, lockedChange.Slug)
		require.NoError(t, err)
	})
}

func TestCancelArchivesDirectExecutionWithCancelledStatus(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := makeNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())
		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		cancelCmd := makeCancelCmd()
		require.NoError(t, cancelCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusCancelled, archived.Status)
	})
}

func TestCancelArchivesGovernedExecutionWithCancelledStatus(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")

		cancelCmd := makeCancelCmd()
		require.NoError(t, cancelCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusCancelled, archived.Status)
	})
}

func TestCancelArchivesUnboundL3Change(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := makeNewCmd()
		create.SetArgs([]string{"investigate stale state cleanup"})
		require.NoError(t, create.Execute())
		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		cancelCmd := makeCancelCmd()
		require.NoError(t, cancelCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusCancelled, archived.Status)
		assert.True(t, archived.NeedsDiscovery)
		assert.Empty(t, archived.WorktreePath)
	})
}

func TestPivotStateBoundaryRejected(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := makeNewCmd()
		create.SetArgs([]string{"refactor service modules"})
		require.NoError(t, create.Execute())

		// S1_PLAN now allows reroute pivot; test that rescope IS rejected.
		pivotCmd := makePivotCmd()
		pivotCmd.SetArgs([]string{"--rescope"})
		err := pivotCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "pivot_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestPivotRescopeRejectedOutsideS6(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		pivotCmd := makePivotCmd()
		pivotCmd.SetArgs([]string{"--rescope"})
		err = pivotCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "rescope_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestPivotRescopeRejectedAtIntakeState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := makeNewCmd()
		create.SetArgs([]string{"narrow intake request"})
		require.NoError(t, create.Execute())

		pivotCmd := makePivotCmd()
		pivotCmd.SetArgs([]string{"--rescope"})
		err := pivotCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "pivot_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestPivotDefaultsToRerouteWithoutReason(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createActiveNonDiscoveryChange(t, root, "fix login timeout")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makePivotCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "reroute", payload["kind"])
	})
}

func TestRequestScopedCommandsRejectAmbiguousActiveContext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		writeActiveChange(t, root, "ambig-a")
		writeActiveChange(t, root, "ambig-b")

		commands := []func() *cobra.Command{
			makeDoneCmd,
			makeCancelCmd,
			makePivotCmd,
			makeNextCmd,
		}
		for _, factory := range commands {
			cmd := factory()
			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ambiguous")
		}
	})
}

func TestCancelPreemptsInFlightTasks(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := makeNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())
		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		// Ignore SIGINT so cancel must escalate to SIGKILL after grace period.
		proc := exec.Command("sh", "-c", "trap '' INT; sleep 30")
		require.NoError(t, proc.Start())
		t.Cleanup(func() {
			if proc.ProcessState == nil {
				_ = proc.Process.Kill()
				_, _ = proc.Process.Wait()
			}
		})

		b, err := json.Marshal([]int{proc.Process.Pid})
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(state.TaskPIDFilePath(root, slug)), 0o755))
		require.NoError(t, os.WriteFile(state.TaskPIDFilePath(root, slug), b, 0o644))

		cfg := model.DefaultConfig()
		cfg.Execution.CancelGracePeriodSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		cancelCmd := makeCancelCmd()
		require.NoError(t, cancelCmd.Execute())

		waited := make(chan struct{})
		go func() {
			_, _ = proc.Process.Wait()
			close(waited)
		}()
		select {
		case <-waited:
		case <-time.After(4 * time.Second):
			t.Fatalf("process %d was not terminated by cancel preemption", proc.Process.Pid)
		}

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		// Archive scrubs absolute-path evidence refs (runtime-local paths become
		// dangling after per-change runtime state is removed). The preemption
		// evidence was written to a git-local absolute path, so it must not
		// survive into the archived change.
		for key := range archived.EvidenceRefs {
			assert.False(t, strings.HasPrefix(key, "cancel_preemption_"),
				"archived change must not retain runtime-local cancel preemption evidence ref")
		}
	})
}

func TestMutatingCommandsBlockOnStateLock(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := makeNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		// Resolve the active change for per-change lock path.
		change, resolveErr := state.FindActiveChange(root)
		require.NoError(t, resolveErr)
		changeLockPath := state.ChangeStateLockPath(root, change.Slug)

		// Test per-change lock blocking for commands that use withChangeStateLock.
		t.Run("per_change_lock", func(t *testing.T) {
			stopLockHolder := startStateLockHolder(t, changeLockPath)
			defer stopLockHolder()

			perRequestCases := []struct {
				name string
				cmd  *cobra.Command
			}{
				{name: "next", cmd: makeNextCmd()},
				{name: "done", cmd: makeDoneCmd()},
				{name: "cancel", cmd: makeCancelCmd()},
				{name: "pivot", cmd: makePivotCmd()},
			}
			for _, tc := range perRequestCases {
				err := tc.cmd.Execute()
				require.Error(t, err, tc.name)
				assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout", tc.name)
			}
		})

		// Test change-create lock blocking for change creation.
		t.Run("change_create_lock", func(t *testing.T) {
			createLockPath := state.ChangeCreateLockPath(root)
			stopLockHolder := startStateLockHolder(t, createLockPath)
			defer stopLockHolder()

			c := makeNewCmd()
			c.SetArgs([]string{"add follow-up"})
			err := c.Execute()
			require.Error(t, err, "new")
			assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout", "new")
		})

		// Test repair lock blocking.
		t.Run("repair_lock", func(t *testing.T) {
			repairLockPath := state.RepairLockPath(root)
			stopLockHolder := startStateLockHolder(t, repairLockPath)
			defer stopLockHolder()

			err := makeRepairCmd().Execute()
			require.Error(t, err, "repair")
			assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout", "repair")
		})
	})
}

func TestRequestCommandBlocksOnChangeCreateLock(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		lockPath := state.ChangeCreateLockPath(root)
		stopLockHolder := startStateLockHolder(t, lockPath)
		defer stopLockHolder()

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"change lock follow-up"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout")
	})
}

func TestGovernedPivotRerouteUpdatesGuardrailDomain(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createActiveNonDiscoveryChange(t, root, "fix login timeout")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		// Advance to S2_EXECUTE and modify for pivot reroute test.
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.Description = "update auth middleware policy"
		require.NoError(t, state.SaveChange(root, change))

		pivot := makePivotCmd()
		require.NoError(t, pivot.Execute())

		updated, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.Equal(t, model.ChangeStatusActive, updated.Status)
		assert.Equal(t, model.StateS1Plan, updated.CurrentState)
		// After simplification, pivot uses direct model: GuardrailDomain is inferred from description.
		assert.Equal(t, "auth_authz", updated.GuardrailDomain)
	})
}

func TestChangeYamlStableAfterSave(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "refactor service modules")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, slug, change.Slug)
		assert.NotEmpty(t, change.BaseRef)

		// Save and verify change.yaml is stable after re-save.
		originalBaseRef := change.BaseRef
		change.NeedsDiscovery = true
		require.NoError(t, state.SaveChange(root, change))

		updatedChange, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, originalBaseRef, updatedChange.BaseRef, "base_ref must be preserved across saves")
		assert.Equal(t, slug, updatedChange.Slug)
		assert.True(t, updatedChange.NeedsDiscovery)
	})
}

func TestRequestCreationCreatesCanonicalBundleState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := makeNewCmd()
		create.SetArgs([]string{"verify change dir structure"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		_, err := os.Stat(state.BundleChangeFilePath(root, slug))
		require.NoError(t, err)

		_, err = os.Stat(state.ChangeDir(root, slug))
		assert.True(t, os.IsNotExist(err), "new should not eagerly create runtime sidecar dirs")

		// Verify change is loadable via state package.
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, slug, change.Slug)
	})
}

func TestArchiveMovesChangeDirAndArtifacts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "archive migration test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Advance to archivable state.
		markChangeReadyForDone(t, root, &change)
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		runtimeEvidence := filepath.Join(state.ChangeDir(root, slug), "evidence", "runs", "latest.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(runtimeEvidence), 0o755))
		require.NoError(t, os.WriteFile(runtimeEvidence, []byte("{}"), 0o644))

		_, err = os.Stat(filepath.Join(root, "artifacts", "changes", slug))
		require.NoError(t, err, "artifact dir must exist before archive")

		doneCmd := makeDoneCmd()
		require.NoError(t, doneCmd.Execute())

		// Post-conditions: artifact dir moved to archived location.
		_, err = os.Stat(filepath.Join(root, "artifacts", "changes", slug))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err), "active artifact dir must not exist after archive")

		_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug))
		require.NoError(t, err, "archived bundle dir must exist after archive")

		_, err = os.Stat(state.ChangeDir(root, slug))
		assert.True(t, os.IsNotExist(err), "archive should delete obsolete runtime sidecar dirs")
	})
}

// writeActiveChange creates a minimal active change to use in multi-change tests.
func writeActiveChange(t *testing.T, root, slug string) {
	t.Helper()
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
}

// createGovernedRequest creates and routes a governed (L2/L3) request.
// Returns the slug. The change exists in artifacts/changes/<slug>/change.yaml after routing.
// The change is advanced to S1_PLAN to simulate having passed intake.
func createGovernedRequest(t *testing.T, root, level, description string) string {
	t.Helper()
	create := makeNewCmd()
	create.SetArgs([]string{"--preset", "standard", description})
	require.NoError(t, create.Execute())

	slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	// Advance past S0 intake to S1_PLAN (simulating intake completion).
	change.CurrentState = model.StateS1Plan
	change.IntakeSubStep = ""
	change.PlanSubStep = model.PlanSubStepResearch
	change.NeedsDiscovery = level == "L3"
	require.NoError(t, state.SaveChange(root, change))

	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))
	return slug
}

// createActiveNonDiscoveryChange creates a non-discovery governed change and advances it to S5_RUN_WAVES.
// Returns the slug.
func createActiveNonDiscoveryChange(t *testing.T, root, description string) string {
	t.Helper()
	create := makeNewCmd()
	create.SetArgs([]string{"--preset", "standard", description})
	require.NoError(t, create.Execute())

	slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.IntakeSubStep = ""
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	return slug
}

func validAssuranceContent() string {
	return `## Scope Summary
Summary of scope

## Verification Verdict
All verified

## Evidence Index
Evidence listed

## Requirement Coverage
All requirements covered

## Residual Risks and Exceptions
None identified

## Rollback Readiness
Rollback remains available and documented.

## Archive Decision
Approved for archive`
}

func writeAssuranceMD(t *testing.T, root, slug, content string) {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assurance.md"), []byte(content), 0o644))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}

func writeSkillVerification(t *testing.T, root, slug, skillName string, rec model.VerificationRecord) {
	t.Helper()

	rec.Normalize()
	require.NoError(t, rec.Validate())

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	paths, err := state.ResolveChangePaths(root, change)
	require.NoError(t, err)

	dir := filepath.Join(paths.GovernedBundleDir, "verification")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	raw, err := yaml.Marshal(rec)
	require.NoError(t, err)
	require.NoError(t, fsutil.WriteFileAtomic(filepath.Join(dir, skillName+".yaml"), raw, 0o644))
}

func writeShipReadyGovernedBundle(t *testing.T, root string, change model.Change) {
	t.Helper()

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte(`# Intent
INT-001: finalize governed closeout
## Open Questions
(none)
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte(`# Requirements
### Requirement: ShipReadyCloseout
REQ-001: The change MUST satisfy ship-ready governance evidence before finalization.
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte(`# Decision
## Alternatives Considered
### Option A
Keep finalization gate-local.
### Option B
Reuse shared readiness across finalization.

## Selected Approach
Use shared readiness so finalization and read surfaces stay aligned.

## Interfaces and Data Flow
G_ship consumes the shared readiness surface.

## Rollout and Rollback
Roll forward by enforcing the same blockers across commands.

## Risk
Low risk; failures should surface as explicit readiness blockers.
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` verify ship readiness parity
  - depends_on: []
  - target_files: ["cmd/done.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
}

func markChangeReadyForDone(t *testing.T, root string, change *model.Change) {
	t.Helper()
	require.NotNil(t, change)
	change.CurrentState = model.StateS4Verify
	change.IntakeSubStep = ""
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, *change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, *change, ""))
	writeShipReadyGovernedBundle(t, root, *change)
	writePassingExecutionSummary(t, root, change.Slug, 1, "t-01")
	writePassingWaveEvidence(t, root, change.Slug, 1)
	writePassingReviewEvidencePack(t, root, change.Slug, 1)
	writePassingGoalVerificationEvidence(t, root, change.Slug, 1)
}

func writePassingWaveEvidence(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"run_summary:rv1"},
	})
}

func writePassingReviewEvidencePack(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"layer:R0=pass", "layer:IR1=pass"},
	})
	writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(time.Second),
		RunVersion: runSummaryVersion,
		References: []string{"layer:QUALITY=pass"},
	})
}

func writePassingGoalVerificationEvidence(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, "goal-verification", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"verification:pass"},
	})
}
