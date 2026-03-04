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

	"github.com/signalridge/speclane/internal/bootstrap"
	"github.com/signalridge/speclane/internal/engine/gate"
	"github.com/signalridge/speclane/internal/fsutil"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDoneArchivesDirectLane(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		doCmd := newDoCmd()
		require.NoError(t, doCmd.Execute())

		doneCmd := newDoneCmd()
		require.NoError(t, doneCmd.Execute())

		entries, err := os.ReadDir(filepath.Join(root, ".spln", "runtime", "admissions"))
		require.NoError(t, err)
		assert.Len(t, entries, 0)

		archived, err := os.ReadDir(filepath.Join(root, ".spln", "archive", "admissions"))
		require.NoError(t, err)
		assert.Len(t, archived, 1)
	})
}

func TestDoneArchivesGovernedLane(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))
		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		change.CurrentState = model.StateS8Verify
		change.Gates[string(gate.GateShip)] = model.GateRecord{
			GateID:   string(gate.GateShip),
			Status:   model.GateStatusApproved,
			Decision: model.GateDecisionApprove,
		}
		require.NoError(t, state.SaveChange(root, change))

		doneCmd := newDoneCmd()
		require.NoError(t, doneCmd.Execute())

		_, err = os.Stat(filepath.Join(root, ".spln", "runtime", "changes", requestID+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(filepath.Join(root, ".spln", "archive", "changes", requestID+".yaml"))
		require.NoError(t, err)
	})
}

func TestCancelArchivesDirectLaneWithCancelledStatus(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "admissions"))

		cancelCmd := newCancelCmd()
		require.NoError(t, cancelCmd.Execute())

		archivedPath := filepath.Join(root, ".spln", "archive", "admissions", requestID+".yaml")
		raw, err := os.ReadFile(archivedPath)
		require.NoError(t, err)
		var archived model.AdmissionState
		require.NoError(t, yaml.Unmarshal(raw, &archived))
		assert.Equal(t, model.AdmissionStatusCancelled, archived.AdmissionStatus)
	})
}

func TestCancelArchivesGovernedLaneWithCancelledStatus(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		cancelCmd := newCancelCmd()
		require.NoError(t, cancelCmd.Execute())

		_, err := os.Stat(filepath.Join(root, ".spln", "runtime", "changes", requestID+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		archived, err := state.LoadArchivedChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusCancelled, archived.ChangeStatus)
	})
}

func TestPivotStateBoundaryRejected(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())

		pivotCmd := newPivotCmd()
		err := pivotCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "S6/S7/S8")
	})
}

func TestPivotRescopeRejectedOutsideS6(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		change.CurrentState = model.StateS7Review
		require.NoError(t, state.SaveChange(root, change))

		pivotCmd := newPivotCmd()
		pivotCmd.SetArgs([]string{"--kind", "rescope"})
		err = pivotCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires governed S6_RUN_WAVES")
	})
}

func TestAnalyzeRequiresActiveContext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		cmd := newAnalyzeCmd()
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active request")
	})
}

func TestReviewRequiresActiveContext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		cmd := newReviewCmd()
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active request")
	})
}

func TestReviewRejectsUnsupportedArtifactFlag(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		cmd := newReviewCmd()
		cmd.SetArgs([]string{"--artifact", "design.md"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported in MVP")
	})
}

func TestReviewFromS6WithoutSummaryFails(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		cmd := newReviewCmd()
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "frozen run summary")
	})
}

func TestPivotDefaultsToRerouteWithoutReason(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		var out bytes.Buffer
		cmd := newPivotCmd()
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
		writeActiveAdmission(t, root)
		writeActiveAdmission(t, root)

		commands := []func() *cobra.Command{
			newDoCmd,
			newDoneCmd,
			newCancelCmd,
			newPivotCmd,
			newAnalyzeCmd,
			newReviewCmd,
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
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "admissions"))

		// Ignore SIGINT so cancel must escalate to SIGKILL after grace period.
		proc := exec.Command("sh", "-c", "trap '' INT; sleep 30")
		require.NoError(t, proc.Start())
		t.Cleanup(func() {
			if proc.ProcessState == nil {
				_ = proc.Process.Kill()
				_, _ = proc.Process.Wait()
			}
		})

		pidDir := filepath.Join(root, ".spln", "runtime", "task_pids")
		require.NoError(t, os.MkdirAll(pidDir, 0o755))
		b, err := json.Marshal([]int{proc.Process.Pid})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(pidDir, requestID+".json"), b, 0o644))

		cfg := model.DefaultConfig()
		cfg.Execution.CancelGracePeriodSeconds = 1
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		cancelCmd := newCancelCmd()
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

		archivedRaw, err := os.ReadFile(state.ArchiveAdmissionPath(root, requestID))
		require.NoError(t, err)
		var archived model.AdmissionState
		require.NoError(t, yaml.Unmarshal(archivedRaw, &archived))
		foundPreemptionEvidence := ""
		for key, ref := range archived.EvidenceRefs {
			if strings.HasPrefix(key, "cancel_preemption_") {
				foundPreemptionEvidence = ref
				break
			}
		}
		require.NotEmpty(t, foundPreemptionEvidence)
		evidenceRaw, err := os.ReadFile(foundPreemptionEvidence)
		require.NoError(t, err)
		assert.Contains(t, string(evidenceRaw), "cancelled_or_aborted")
	})
}

func TestDoTriggersLowDiskOpportunisticGC(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		inactiveRequestID, err := model.NewRequestID()
		require.NoError(t, err)
		staleEvidence := filepath.Join(root, ".spln", "evidence", "tasks", inactiveRequestID, "rv1", "stale.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(staleEvidence), 0o755))
		require.NoError(t, os.WriteFile(staleEvidence, []byte("{}"), 0o644))
		old := time.Now().Add(-72 * time.Hour)
		require.NoError(t, os.Chtimes(staleEvidence, old, old))

		cfg := model.DefaultConfig()
		cfg.Execution.EvidenceRetentionDays = 1
		cfg.Execution.EvidenceGCLowDiskFreeMB = 1_000_000_000 // Force low-disk trigger in tests.
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		doCmd := newDoCmd()
		require.NoError(t, doCmd.Execute())

		_, err = os.Stat(staleEvidence)
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestTaskEvidenceReferenceDeterministicAndCollisionSafe(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "admissions"))
		taskID := "l1-" + shortRequestID(requestID) + "-01"
		basePath := filepath.Join(root, ".spln", "evidence", "tasks", requestID, "rv1")
		firstEvidencePath := filepath.Join(basePath, taskID+".json")

		require.NoError(t, os.MkdirAll(basePath, 0o755))
		require.NoError(t, os.WriteFile(firstEvidencePath, []byte("sentinel"), 0o644))

		doCmd := newDoCmd()
		require.NoError(t, doCmd.Execute())

		admission, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		require.Len(t, admission.TaskRuns, 1)

		var run model.TaskRun
		for _, candidate := range admission.TaskRuns {
			run = candidate
		}

		assert.Equal(t, taskID, run.TaskID)
		assert.Equal(t, 1, run.RunSummaryVersion)
		expectedSuffix := filepath.ToSlash(filepath.Join(
			".spln",
			"evidence",
			"tasks",
			requestID,
			"rv1",
			taskID+"--2.json",
		))
		assert.True(t, strings.HasSuffix(filepath.ToSlash(run.EvidenceRef), expectedSuffix))

		original, err := os.ReadFile(firstEvidencePath)
		require.NoError(t, err)
		assert.Equal(t, "sentinel", string(original))

		collisionEvidence, err := os.ReadFile(run.EvidenceRef)
		require.NoError(t, err)
		assert.Contains(t, string(collisionEvidence), `"task_id":"`+taskID+`"`)
		assert.Contains(t, string(collisionEvidence), `"run_summary_version":1`)
	})
}

func TestWaveRuntimeOverlapBlocksUntilSerializedRetry(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		change.CurrentState = model.StateS6RunWaves
		change.LatestFrozenRunSummaryVersion = 1
		change.TaskRuns = map[string]model.TaskRun{
			"task-a__rv1": {
				TaskID:            "task-a",
				RunSummaryVersion: 1,
				TaskKind:          model.TaskKindImplementation,
				Verdict:           model.TaskVerdictPass,
				ChangedFiles:      []string{"internal/service/auth.go"},
				EvidenceRef:       ".spln/evidence/tasks/r/rv1/task-a.json",
			},
			"task-b__rv1": {
				TaskID:            "task-b",
				RunSummaryVersion: 1,
				TaskKind:          model.TaskKindImplementation,
				Verdict:           model.TaskVerdictPass,
				ChangedFiles:      []string{"internal/service/auth.go"},
				EvidenceRef:       ".spln/evidence/tasks/r/rv1/task-b.json",
			},
		}
		require.NoError(t, state.SaveChange(root, change))

		var firstOut bytes.Buffer
		doCmd := newDoCmd()
		doCmd.SetOut(&firstOut)
		require.NoError(t, doCmd.Execute())

		var firstPayload map[string]any
		require.NoError(t, json.Unmarshal(firstOut.Bytes(), &firstPayload))
		assert.Equal(t, "S6_RUN_WAVES", firstPayload["current_state"])
		blockers, ok := firstPayload["blockers"].([]any)
		require.True(t, ok)
		assert.Contains(t, blockers, any("post_wave_file_conflict:internal/service/auth.go"))

		overlapBlocked, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		for _, run := range overlapBlocked.TaskRuns {
			assert.Equal(t, model.TaskVerdictBlocked, run.Verdict)
			assert.Contains(t, run.Blockers, "post_wave_file_conflict:internal/service/auth.go")
		}

		doCmd = newDoCmd()
		err = doCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--resume-response")

		doCmd = newDoCmd()
		doCmd.SetArgs([]string{"--resume-response", "retry"})
		require.NoError(t, doCmd.Execute())

		resumed, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS7Review, resumed.CurrentState)
		assert.Equal(t, 2, resumed.LatestFrozenRunSummaryVersion)
	})
}

func TestCheckpointContinuationAndFreshnessStaleAfterAnalyzeUpdate(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		change.CurrentState = model.StateS6RunWaves
		taskID := "gov-" + shortRequestID(requestID) + "-01"
		change.LatestFrozenRunSummaryVersion = 1
		change.TaskRuns = map[string]model.TaskRun{
			taskID + "__rv1": {
				TaskID:            taskID,
				RunSummaryVersion: 1,
				TaskKind:          model.TaskKindImplementation,
				Verdict:           model.TaskVerdictBlocked,
				Blockers:          []string{"requires_human_decision"},
			},
		}
		require.NoError(t, state.SaveChange(root, change))

		doCmd := newDoCmd()
		err = doCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--resume-response")

		var resumeOut bytes.Buffer
		doCmd = newDoCmd()
		doCmd.SetOut(&resumeOut)
		doCmd.SetArgs([]string{"--resume-response", "approved"})
		require.NoError(t, doCmd.Execute())

		var resumePayload map[string]any
		require.NoError(t, json.Unmarshal(resumeOut.Bytes(), &resumePayload))
		details, ok := resumePayload["details"].(map[string]any)
		require.True(t, ok)
		continuationRef, ok := details["continuation_evidence_ref"].(string)
		require.True(t, ok)
		require.NotEmpty(t, continuationRef)
		continuationRaw, err := os.ReadFile(continuationRef)
		require.NoError(t, err)
		assert.Contains(t, string(continuationRaw), `"user_response_payload":"approved"`)

		analyze := newAnalyzeCmd()
		require.NoError(t, analyze.Execute())

		var statusOut bytes.Buffer
		statusCmd := newStatusCmd()
		statusCmd.SetOut(&statusOut)
		require.NoError(t, statusCmd.Execute())
		var statusPayload map[string]any
		require.NoError(t, json.Unmarshal(statusOut.Bytes(), &statusPayload))
		assert.Equal(t, "stale", statusPayload["evidence_freshness"])

		var contextOut bytes.Buffer
		contextCmd := newContextCmd()
		contextCmd.SetOut(&contextOut)
		contextCmd.SetArgs([]string{"--format", "json"})
		require.NoError(t, contextCmd.Execute())
		var contextPayload map[string]any
		require.NoError(t, json.Unmarshal(contextOut.Bytes(), &contextPayload))
		assert.Equal(t, "stale", contextPayload["evidence_freshness"])
	})
}

func TestLockTimeoutBlocksDoUntilHolderReleases(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		cfg.Execution.LockStaleAfterSeconds = 1
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		stopLockHolder := startStateLockHolder(t, root)

		doCmd := newDoCmd()
		err := doCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "state lock timeout")

		stopLockHolder()

		doCmd = newDoCmd()
		require.NoError(t, doCmd.Execute())
	})
}

func TestStaleLockMetadataDoesNotBlockDo(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		cfg.Execution.LockStaleAfterSeconds = 1
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		lockPath := filepath.Join(root, ".spln", "state.lock")
		require.NoError(t, os.WriteFile(lockPath, []byte(""), 0o644))
		lock := fsutil.NewStateLock(lockPath)
		require.NoError(t, lock.WriteMeta(fsutil.LockMeta{
			HolderPID:  999999,
			AcquiredAt: time.Now().UTC().Add(-5 * time.Minute),
			Command:    "spln do",
		}))

		doCmd := newDoCmd()
		require.NoError(t, doCmd.Execute())
	})
}

func TestMutatingCommandsBlockOnStateLock(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		stopLockHolder := startStateLockHolder(t, root)
		defer stopLockHolder()

		cases := []struct {
			name string
			cmd  *cobra.Command
		}{
			{name: "new", cmd: func() *cobra.Command {
				c := newNewCmd()
				c.SetArgs([]string{"--level", "L1", "add follow-up"})
				return c
			}()},
			{name: "do", cmd: newDoCmd()},
			{name: "done", cmd: newDoneCmd()},
			{name: "cancel", cmd: newCancelCmd()},
			{name: "pivot", cmd: newPivotCmd()},
			{name: "analyze", cmd: newAnalyzeCmd()},
			{name: "review", cmd: newReviewCmd()},
			{name: "repair", cmd: newRepairCmd()},
		}
		for _, tc := range cases {
			err := tc.cmd.Execute()
			require.Error(t, err, tc.name)
			assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout", tc.name)
		}
	})
}

func TestGovernedVerifyBlocksOnInvalidAssuranceStructure(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		change.CurrentState = model.StateS8Verify
		require.NoError(t, state.SaveChange(root, change))

		assurancePath := filepath.Join(root, "aircraft", "changes", change.Slug, "assurance.md")
		require.NoError(t, os.WriteFile(assurancePath, []byte("## Scope Summary\nonly one heading"), 0o644))

		var out bytes.Buffer
		doCmd := newDoCmd()
		doCmd.SetOut(&out)
		require.NoError(t, doCmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "S6_RUN_WAVES", payload["current_state"])
		blockers, ok := payload["blockers"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, blockers)
	})
}

func TestL2FlowEnforcesPlanAndShip(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		for i := 0; i < 5; i++ {
			doCmd := newDoCmd()
			require.NoError(t, doCmd.Execute())
		}

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS8Verify, change.CurrentState)
		assert.Equal(t, model.GateStatusApproved, change.Gates[string(gate.GatePlan)].Status)
		assert.Equal(t, model.GateStatusApproved, change.Gates[string(gate.GateShip)].Status)
	})
}

func TestL3FlowScopeReadinessAndMetadata(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		runGit(t, root, "init", "--initial-branch=main")
		runGit(t, root, "config", "user.email", "test@example.com")
		runGit(t, root, "config", "user.name", "Test User")
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")

		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L3", "introduce auth guardrails"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		doCmd := newDoCmd() // S2 -> S3
		require.NoError(t, doCmd.Execute())

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		require.Equal(t, model.StateS3ScopeConfirmation, change.CurrentState)

		explorePath := filepath.Join(root, "aircraft", "changes", change.Slug, "explore.md")
		require.NoError(t, os.WriteFile(explorePath, []byte("## Objectives\nonly"), 0o644))
		doCmd = newDoCmd() // blocked at S3
		require.NoError(t, doCmd.Execute())
		change, err = state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3ScopeConfirmation, change.CurrentState)

		require.NoError(t, os.WriteFile(explorePath, []byte(validExploreContent()), 0o644))
		doCmd = newDoCmd() // S3 -> S4 with G_scope approved
		require.NoError(t, doCmd.Execute())

		change, err = state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS4SpecBundle, change.CurrentState)
		assert.NotEmpty(t, change.WorktreePath)
		assert.NotEmpty(t, change.WorktreeBranch)
		assert.Equal(t, model.GateStatusApproved, change.Gates[string(gate.GateScope)].Status)
	})
}

func TestL1PivotEscalatesToGovernedLane(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "admissions"))

		admission, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		admission.IntakeAssessment.IntendedDelta = "update auth middleware policy"
		admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{Novelty: 1, Ambiguity: 1, Impact: 1, Risk: 1, ReversibilityCost: 1}}
		require.NoError(t, state.SaveAdmission(root, admission))

		pivot := newPivotCmd()
		require.NoError(t, pivot.Execute())

		_, err = state.LoadChange(root, requestID)
		require.NoError(t, err)
		sealed, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.AdmissionStatusSealedHandoff, sealed.AdmissionStatus)
	})
}

func TestAnalyzeHardConflictKeepsLaneAndLevel(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "admissions"))

		admission, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		admission.IntakeAssessment.IntendedDelta = "update auth middleware policy"
		require.NoError(t, state.SaveAdmission(root, admission))

		analyze := newAnalyzeCmd()
		require.NoError(t, analyze.Execute())

		updated, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL1, updated.Level)
		assert.Equal(t, model.StateS1Analyze, updated.CurrentState)
		assert.NotEmpty(t, updated.RouteSnapshot.BlockingConflicts)
	})
}

func TestActiveContextMultiActiveDiagnosticsSurfaces(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		again := newNewCmd()
		again.SetArgs([]string{"--level", "L1", "add metrics"})
		err := again.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "active request exists")

		writeActiveAdmission(t, root)

		var statusOut bytes.Buffer
		statusCmd := newStatusCmd()
		statusCmd.SetOut(&statusOut)
		require.NoError(t, statusCmd.Execute())
		var statusPayload map[string]any
		require.NoError(t, json.Unmarshal(statusOut.Bytes(), &statusPayload))
		assert.Equal(t, "diagnostics", statusPayload["lane_mode"])
		assert.Equal(t, "unknown", statusPayload["evidence_freshness"])

		var contextOut bytes.Buffer
		contextCmd := newContextCmd()
		contextCmd.SetOut(&contextOut)
		contextCmd.SetArgs([]string{"--format", "json"})
		require.NoError(t, contextCmd.Execute())
		var contextPayload map[string]any
		require.NoError(t, json.Unmarshal(contextOut.Bytes(), &contextPayload))
		assert.Equal(t, "diagnostics", contextPayload["lane_mode"])
		assert.Equal(t, "unknown", contextPayload["evidence_freshness"])
	})
}

func TestL3PivotRescopeReturnsToS3AndReevaluatesScopeBeforeS4(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		runGit(t, root, "init", "--initial-branch=main")
		runGit(t, root, "config", "user.email", "test@example.com")
		runGit(t, root, "config", "user.name", "Test User")
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")

		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L3", "introduce auth guardrails"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		change.CurrentState = model.StateS6RunWaves
		change.Gates[string(gate.GateScope)] = model.GateRecord{
			GateID:   string(gate.GateScope),
			Status:   model.GateStatusApproved,
			Decision: model.GateDecisionApprove,
		}
		require.NoError(t, state.SaveChange(root, change))

		pivot := newPivotCmd()
		pivot.SetArgs([]string{"--kind", "rescope"})
		require.NoError(t, pivot.Execute())

		updated, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3ScopeConfirmation, updated.CurrentState)

		explorePath := filepath.Join(root, "aircraft", "changes", updated.Slug, "explore.md")
		require.NoError(t, os.WriteFile(explorePath, []byte("## Objectives\nonly"), 0o644))

		doCmd := newDoCmd()
		require.NoError(t, doCmd.Execute())

		blocked, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3ScopeConfirmation, blocked.CurrentState)
		assert.Equal(t, model.GateStatusBlocked, blocked.Gates[string(gate.GateScope)].Status)

		require.NoError(t, os.WriteFile(explorePath, []byte(validExploreContent()), 0o644))
		doCmd = newDoCmd()
		require.NoError(t, doCmd.Execute())

		final, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS4SpecBundle, final.CurrentState)
		assert.Equal(t, model.GateStatusApproved, final.Gates[string(gate.GateScope)].Status)
	})
}

func TestManifestCreatedAtLevelSnapshotStableAfterRuntimePivot(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		manifestPath := filepath.Join(root, "aircraft", "changes", change.Slug, "change.yaml")
		raw, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		var manifest state.ChangeManifest
		require.NoError(t, yaml.Unmarshal(raw, &manifest))
		assert.Equal(t, model.LevelL2, manifest.CreatedAtLevel)

		cfg := model.DefaultConfig()
		require.NoError(t, state.ApplyLevelPivot(
			&change,
			model.LevelL3,
			model.LevelSourceAuto,
			"pivot",
			time.Now().UTC(),
			cfg.Execution.MaxLevelHistoryEntries,
		))
		require.NoError(t, state.SaveChange(root, change))

		updatedRaw, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		var updatedManifest state.ChangeManifest
		require.NoError(t, yaml.Unmarshal(updatedRaw, &updatedManifest))
		assert.Equal(t, model.LevelL2, updatedManifest.CreatedAtLevel)

		updatedChange, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL3, updatedChange.Level)
	})
}

func TestLevelHistoryCapAndMonotonicLastLevelUpdateAt(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())
		requestID := singleRequestID(t, filepath.Join(root, ".spln", "runtime", "changes"))

		cfg := model.DefaultConfig()
		cfg.Execution.MaxLevelHistoryEntries = 2
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		setChangeState := func() {
			change, err := state.LoadChange(root, requestID)
			require.NoError(t, err)
			change.Level = model.LevelL2
			change.LevelSource = model.LevelSourceAuto
			change.CurrentState = model.StateS6RunWaves
			require.NoError(t, state.SaveChange(root, change))
		}

		for i := 0; i < 3; i++ {
			setChangeState()
			pivot := newPivotCmd()
			require.NoError(t, pivot.Execute())
		}

		final, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Len(t, final.LevelHistory, 2)
		require.NotNil(t, final.LastLevelUpdateAt)
		assert.False(t, final.LastLevelUpdateAt.IsZero())
		assert.Equal(t, model.LevelL1, final.Level)
		assert.Equal(t, model.LevelSourceAuto, final.LevelSource)
		assert.False(t, final.LastLevelUpdateAt.Before(final.LevelHistory[1].At))
		assert.False(t, final.LevelHistory[1].At.Before(final.LevelHistory[0].At))
	})
}

func writeActiveAdmission(t *testing.T, root string) {
	t.Helper()
	requestID, err := model.NewRequestID()
	require.NoError(t, err)
	admission := model.NewAdmissionState(requestID)
	admission.Level = model.LevelL1
	admission.LevelSource = model.LevelSourceUserSelected
	admission.CurrentState = model.StateS6RunWaves
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, state.SaveAdmission(root, admission))
}

func validExploreContent() string {
	return `## Objectives
One

## Unknowns
Two

## Assumptions
Three

## Scope Boundaries
Four

## Validation Plan
Five`
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}
