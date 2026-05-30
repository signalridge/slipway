package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHealthCommandReportsRepairableFindings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("defaults: ["), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotEmpty(t, view.Findings)
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		found := false
		for _, finding := range view.Findings {
			if finding.Category == "config" {
				found = true
				assert.True(t, finding.Repairable)
				assert.True(t, finding.ActiveChangeBlocking)
				assert.Equal(t, "blocking_for_active_change", finding.ActiveChangeImpact)
			}
		}
		assert.True(t, found)
	})
}

func TestHealthCommandMarksCodebaseMapWarningNonBlockingForActiveChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		found := false
		for _, finding := range view.Findings {
			if finding.Category != "codebase_map" {
				continue
			}
			found = true
			assert.False(t, finding.ActiveChangeBlocking)
			assert.Equal(t, "non_blocking_for_active_change", finding.ActiveChangeImpact)
			assert.Contains(t, finding.RepairHint, "slipway codebase-map")
		}
		assert.True(t, found, "expected missing codebase map health finding")
	})
}

func TestHealthCommandReportsMalformedLifecycleEventLog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		change := model.NewChange("malformed-events")
		require.NoError(t, state.SaveChange(root, change))
		eventPath, err := state.LifecycleEventLogPath(root, change)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(eventPath), 0o755))
		require.NoError(t, os.WriteFile(eventPath, []byte("{not-json\n"), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		found := false
		for _, finding := range view.Findings {
			if finding.Category == "lifecycle_event_log" && finding.Slug == change.Slug {
				found = true
				assert.False(t, finding.Repairable)
				assert.Contains(t, finding.Reasons[0].Code, "lifecycle_event_log_unreadable")
			}
		}
		assert.True(t, found, "expected malformed lifecycle event log finding")
	})
}

func TestHealthCommandDoctorOutputsPrioritizedRepairActions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "doctor should surface wave repair")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)
		assert.Equal(t, "doctor", view.ExecutionMode)

		found := false
		for _, action := range view.Doctor.Actions {
			if action.Command == "slipway repair" && strings.Contains(strings.ToLower(action.Summary), "wave plan") {
				found = true
				assert.True(t, action.Repairable)
			}
		}
		assert.True(t, found, "expected doctor to recommend slipway repair for missing wave artifacts")
	})
}

func TestHealthCommandDoctorUsesCommandSpecificRepairHint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		found := false
		for _, action := range view.Doctor.Actions {
			if action.Category != "codebase_map" {
				continue
			}
			found = true
			assert.Equal(t, "slipway codebase-map", action.Command)
			assert.True(t, action.Repairable)
		}
		assert.True(t, found, "expected doctor to preserve the codebase-map repair command")
	})
}

func TestHealthCommandDoctorDoesNotSuggestResumeBeforeWaveRepair(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "doctor should not suggest resume before wave repair")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			PausedAt:        time.Now().UTC(),
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/health.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending checkpointed wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/health.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		foundRepair := false
		for _, action := range view.Doctor.Actions {
			assert.NotEqual(t, `slipway run --resume-response "<response>"`, action.Command)
			if action.Category == "wave_execution" && action.Slug == slug && action.Command == "slipway repair" {
				foundRepair = true
			}
		}
		assert.True(t, foundRepair, "expected doctor to recommend repair instead of resume")
	})
}

func TestHealthCommandDoctorDoesNotSuggestResumeWhenWavePlanIsMissingBeforeExecutionSummaryReady(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "doctor should not suggest resume before pre-summary wave plan repair")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			PausedAt:        time.Now().UTC(),
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		foundRepair := false
		for _, action := range view.Doctor.Actions {
			assert.NotEqual(t, `slipway run --resume-response "<response>"`, action.Command)
			if action.Category == "wave_execution" && action.Slug == slug && action.Command == "slipway repair" {
				foundRepair = true
			}
		}
		assert.True(t, foundRepair, "expected doctor to recommend repair instead of resume")
	})
}

func TestHealthCommandDoctorExplainsNoActiveChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)
		assert.Contains(t, view.Diagnostics, "No active change; governance health not applicable.")

		found := false
		for _, action := range view.Doctor.Actions {
			if action.Category != "governance" {
				continue
			}
			if action.Summary == "No active change; governance health not applicable." {
				found = true
				assert.False(t, action.Repairable)
				assert.Empty(t, action.Command)
			}
		}
		assert.True(t, found, "expected doctor to emit an informational no-active-change action")
	})
}

func TestHealthCommandDoctorExplainsInterruptedExecution(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "doctor should explain interrupted execution")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.InterruptedExecutionAt = time.Date(2026, time.April, 11, 10, 30, 0, 0, time.UTC)
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/health.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/health.go"]
  - task_kind: code
`)))
		materializeWaveExecutionForSummary(t, root, slug)

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		foundInterrupted := false
		foundResume := false
		for _, action := range view.Doctor.Actions {
			if action.Category == "execution_session" && action.Command == "slipway status" {
				foundInterrupted = true
				assert.Contains(t, action.Summary, "2026-04-11T10:30:00Z")
			}
			if action.Category == "execution_resume" && action.Command == "slipway run --resume" {
				foundResume = true
			}
		}
		assert.True(t, foundInterrupted, "expected doctor to explain the interrupted execution")
		assert.True(t, foundResume, "expected doctor to preserve the resume action")
	})
}

func TestHealthCommandDoctorBlocksWavePlanRepairWhenCurrentTasksDrifted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "doctor should not auto-repair drifted wave state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` original execution task
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		tasksPath := filepath.Join(bundlePath, "tasks.md")
		updatedAt := time.Now().UTC().Add(2 * time.Second)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-02`"+` replacement task after drift
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		require.NoError(t, os.Chtimes(tasksPath, updatedAt, updatedAt))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		found := false
		for _, action := range view.Doctor.Actions {
			if action.Category != "wave_execution" || action.Slug != slug {
				continue
			}
			if strings.Contains(strings.ToLower(action.Summary), "cannot be safely reconstructed") {
				found = true
				assert.False(t, action.Repairable)
				assert.Equal(t, "slipway pivot --rescope", action.Command)
			}
		}
		assert.True(t, found, "expected doctor to block auto-repair for drifted wave-plan reconstruction")
	})
}

func TestHealthCommandMarksUnreadableExecutionSummaryRepairableWhenWaveEvidenceExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "health should promote repairable execution summary finding")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		materializeWaveExecutionForSummary(t, root, slug)

		summaryPath := state.ExecutionSummaryPathForRead(root, slug)
		require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		found := false
		for _, finding := range view.Findings {
			if finding.Category != "execution_summary" || finding.Slug != slug {
				continue
			}
			found = true
			assert.True(t, finding.Repairable)
			assert.Equal(t, "Run `slipway repair` to rebuild execution-summary.yaml from wave-backed execution evidence.", finding.RepairHint)
		}
		assert.True(t, found, "expected unreadable execution summary finding")
	})
}

func TestHealthCommandDoctorUsesPivotForWavePlanDrift(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "doctor should surface pivot for wave drift")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve original task
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-02`"+` replace task after drift
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		found := false
		for _, action := range view.Doctor.Actions {
			if action.Category != "wave_execution" || action.Slug != slug {
				continue
			}
			if strings.Contains(strings.ToLower(action.Summary), "wave plan drift") {
				found = true
				assert.False(t, action.Repairable)
				assert.Equal(t, "slipway pivot --rescope", action.Command)
			}
		}
		assert.True(t, found, "expected doctor to recommend pivot for wave plan drift")

		foundFinding := false
		for _, finding := range view.Findings {
			if finding.Category != "wave_execution" || finding.Slug != slug {
				continue
			}
			if strings.Contains(strings.Join(model.ReasonSpecs(finding.Reasons), "\n"), "wave_plan_drift") {
				foundFinding = true
				assert.True(t, finding.ActiveChangeBlocking)
				assert.Equal(t, "blocking_for_active_change", finding.ActiveChangeImpact)
			}
		}
		assert.True(t, foundFinding, "expected wave drift finding to be marked blocking")
	})
}

func TestHealthCommandReportsInvalidAgentMapping(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Agents.Mappings = map[string]string{}
		cfg.Agents.Mappings["wave-orchestration"] = "slipway-missing"
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		found := false
		for _, finding := range view.Findings {
			if finding.Category != "agent_contract" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "agent_mapping_invalid" {
					found = true
					assert.False(t, finding.Repairable)
				}
			}
		}
		assert.True(t, found, "expected invalid agent mapping finding")
	})
}

func TestHealthCommandRejectsManualOnlyAgentMapping(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Agents.Mappings = map[string]string{}
		cfg.Agents.Mappings["wave-orchestration"] = "slipway-executor"
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		found := false
		for _, finding := range view.Findings {
			if finding.Category != "agent_contract" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "agent_mapping_invalid" {
					found = true
					assert.Equal(t, model.ReasonSeverityError, finding.Severity)
					assert.False(t, finding.Repairable)
					assert.Contains(t, reason.Message, "manual-only")
					assert.Contains(t, finding.RepairHint, "governance-mapped")
				}
			}
		}
		assert.True(t, found, "expected manual-only governance mapping rejection")
	})
}

func TestHealthCommandReportsMissingHostSkillSurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))
		require.NoError(t, os.Remove(filepath.Join(root, ".claude", "skills", "slipway-intake-clarification", "SKILL.md")))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		found := false
		for _, finding := range view.Findings {
			if finding.Category != "agent_contract" || finding.Slug != "intake-clarification" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "skill_prompt_surface_missing" {
					found = true
					assert.Contains(t, finding.Message, "missing host skill surface")
					assert.Contains(t, finding.RepairHint, "slipway init --tools claude --refresh")
				}
			}
		}
		assert.True(t, found, "expected missing host skill surface finding")
	})
}

func TestHealthCommandDoesNotReportToolResolutionFailureForMultiAdapterWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex", "claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "health should stay query-only in multi-adapter workspace")

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		for _, finding := range view.Findings {
			assert.NotEqual(t, "Tool adapter resolution failed", finding.Message)
			for _, reason := range finding.Reasons {
				assert.NotEqual(t, "tool_resolution_failed", reason.Code)
			}
		}
	})
}

func TestHealthCommandIgnoresMissingProjectAgentSurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		agentPath := filepath.Join(root, ".claude", "agents", "slipway-planner.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
		require.NoError(t, os.WriteFile(agentPath, []byte("agent"), 0o644))
		require.NoError(t, os.Remove(agentPath))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		for _, finding := range view.Findings {
			assert.NotContains(t, finding.Message, ".claude/agents/slipway-planner.md")
			for _, reason := range finding.Reasons {
				assert.NotEqual(t, "agent_generated_surface_missing", reason.Code)
				assert.NotEqual(t, "agent_generated_surface_unreadable", reason.Code)
				assert.NotEqual(t, ".claude/agents/slipway-planner.md", reason.Message)
			}
		}
	})
}

func TestHealthCommandReportsMissingHostSkillSurfaceForMultiAdapterWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex", "claude"}, false))
		require.NoError(t, os.Remove(filepath.Join(root, ".claude", "skills", "slipway-intake-clarification", "SKILL.md")))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		found := false
		for _, finding := range view.Findings {
			if finding.Category != "agent_contract" || finding.Slug != "intake-clarification" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "skill_prompt_surface_missing" {
					found = true
					assert.Contains(t, finding.Message, "missing host skill surface for claude")
					assert.Contains(t, finding.RepairHint, "slipway init --tools claude --refresh")
				}
			}
		}
		assert.True(t, found, "expected missing host skill surface finding in multi-adapter workspace")
	})
}

func TestHealthCommandDoctorIncludesGovernanceFailuresWithoutExtraFlags(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L2", "doctor should include governance failures")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.WorktreePath = root
		change.WorktreeBranch = currentGitBranch(t, root)
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)
		require.NotNil(t, view.Governance)

		found := false
		for _, action := range view.Doctor.Actions {
			if action.Category == "governance_worktree_binding" {
				found = true
				assert.Contains(t, action.Summary, "dedicated_worktree_required")
			}
		}
		assert.True(t, found, "expected governance worktree failure in doctor actions")
	})
}

func TestHealthCommandDoctorSkipsNonCommandRepairHints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Agents.Mappings = map[string]string{
			"wave-orchestration": "slipway-missing",
		}
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		for _, action := range view.Doctor.Actions {
			if action.Category == "agent_contract" {
				assert.NotEqual(t, ".slipway.yaml", action.Command)
			}
		}
	})
}

func TestHealthCommandRejectsUninitializedGitRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		err := cmd.Execute()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)
		assert.Contains(t, err.Error(), "run `slipway init`")
	})
}

func TestHealthCommandObservationsFlagIncludesSignalProvenance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "health observations")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--observations", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))

		observations, ok := payload["observations"].([]any)
		require.True(t, ok, "expected observations in health output")
		require.NotEmpty(t, observations)

		obsMap, ok := observations[0].(map[string]any)
		require.True(t, ok)
		assert.NotEmpty(t, obsMap["id"])
		assert.NotEmpty(t, obsMap["signal"])
		assert.NotEmpty(t, obsMap["source"])
	})
}

func TestHealthCommandGovernanceReportsUnreadableSnapshotInsteadOfFailing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "health unreadable snapshot")
		snapshotPath := governance.SnapshotPath(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
		require.NoError(t, os.WriteFile(
			snapshotPath,
			[]byte("version: ["),
			0o644,
		))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)
		// Snapshot absence is now non-blocking (Plan B: delete-safe audit cache).
		// Health degrades to WARN, not FAIL, so overall may remain healthy.

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "signal_freshness" {
				found = true
				assert.Equal(t, "WARN", check.Status)
				assert.Contains(t, check.Message, "governance_audit_data_unavailable")
			}
		}
		assert.True(t, found, "expected unreadable snapshot warning to surface in governance health")
	})
}

func TestHealthCommandGovernanceObservationsStillRenderWhenSnapshotUnreadable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "health unreadable snapshot observations")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		snapshotPath := governance.SnapshotPath(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
		require.NoError(t, os.WriteFile(snapshotPath, []byte("version: ["), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--observations", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))

		observations, ok := payload["observations"].([]any)
		require.True(t, ok, "expected observations in health output")
		require.NotEmpty(t, observations)

		rawSnapshot, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, "version: [", string(rawSnapshot), "health should diagnose unreadable snapshot without rewriting it")
	})
}

func TestHealthCommandGovernanceSkipsRecomputeWhenBoundWorktreeInvalid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L2", "health invalid bound worktree")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.GuardrailDomain = "auth_authz"
		change.WorktreePath = root
		change.WorktreeBranch = currentGitBranch(t, root)
		require.NoError(t, state.SaveChange(root, change))
		writeAuthReviewGovernedBundle(t, root, slug)

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		_, err = governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		snapshotPath := governance.SnapshotPath(root, slug)
		before, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--observations", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)
		assert.Empty(t, view.Observations)

		foundCoherence := false
		foundWorktree := false
		for _, check := range view.Governance.Checks {
			switch check.Name {
			case "signal_control_coherence":
				foundCoherence = true
				assert.Equal(t, "WARN", check.Status)
				assert.Contains(t, check.Message, "dedicated_worktree_required")
			case "worktree_binding":
				foundWorktree = true
				assert.Equal(t, "FAIL", check.Status)
				assert.Contains(t, check.Message, "dedicated_worktree_required")
			}
		}
		assert.True(t, foundCoherence, "expected signal_control_coherence check")
		assert.True(t, foundWorktree, "expected worktree_binding check")

		after, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "health should not recompute snapshots when worktree binding is invalid")
	})
}

func TestHealthCommandGovernanceRecomputesCurrentArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "health live recompute")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, state.SaveChange(root, change))

		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: stabilize auth middleware
## Open Questions
(none)
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: auth stability
REQ-001: Preserve auth middleware behavior. Traces to INT-001.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Patch the current middleware.
### Option B
Rewrite the middleware.

## Selected Approach
Choose Option A.

## Interfaces and Data Flow
Existing auth entry points remain stable.

## Rollout and Rollback
Deploy gradually and roll back to the prior middleware.

## Risk
Auth regressions remain localized.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
  covers: [REQ-001]
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth middleware only.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Rollback path documented.

## Archive Decision
Not ready.
`), 0o644))

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		_, err = governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
`), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "traceability_coherence" {
				found = true
				assert.Equal(t, "FAIL", check.Status)
				assert.Contains(t, check.Message, "blocking traceability gaps")
			}
		}
		assert.True(t, found, "expected traceability_coherence check")
	})
}

func TestHealthCommandGovernancePreservesPersistedFreshnessSignal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "health stale persisted snapshot")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: stabilize auth middleware
## Open Questions
(none)
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: auth stability
REQ-001: Preserve auth middleware behavior. Traces to INT-001.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Patch the current middleware.

## Selected Approach
Choose Option A.

## Interfaces and Data Flow
Existing auth entry points remain stable.

## Rollout and Rollback
Deploy gradually and roll back to the prior middleware.

## Risk
Auth regressions remain localized.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
  covers: [REQ-001]
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth middleware only.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Rollback path documented.

## Archive Decision
Not ready.
`), 0o644))

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		snap, err := governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		snap.ComputedAt = time.Now().UTC().Add(-2 * time.Hour)
		raw, err := yaml.Marshal(snap)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(governance.SnapshotPath(root, slug), raw, 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "signal_freshness" {
				found = true
				assert.Equal(t, "WARN", check.Status)
				assert.Contains(t, check.Message, "stale")
			}
		}
		assert.True(t, found, "expected signal_freshness check")
	})
}

func TestHealthCommandGovernanceUsesFreshnessFromRecomputedSnapshotWhenMaterialStateChanges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "health refreshed snapshot")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: stabilize auth middleware
## Open Questions
(none)
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: auth stability
REQ-001: Preserve auth middleware behavior. Traces to INT-001.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Patch the current middleware.

## Selected Approach
Choose Option A.

## Interfaces and Data Flow
Existing auth entry points remain stable.

## Rollout and Rollback
Deploy gradually and roll back to the prior middleware.

## Risk
Auth regressions remain localized.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
  covers: [REQ-001]
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth middleware only.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Rollback path documented.

## Archive Decision
Not ready.
`), 0o644))

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		snap, err := governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		snap.ComputedAt = time.Now().UTC().Add(-2 * time.Hour)
		raw, err := yaml.Marshal(snap)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(governance.SnapshotPath(root, slug), raw, 0o644))

		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
`), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "signal_freshness" {
				found = true
				assert.Equal(t, "OK", check.Status)
				assert.NotContains(t, check.Message, "stale")
			}
		}
		assert.True(t, found, "expected signal_freshness check")
	})
}

func TestHealthCommandGovernanceBlocksOnStateLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		slug := createGovernedRequest(t, root, "L2", "health lock")
		lockPath := state.ChangeStateLockPath(root, slug)
		stopLockHolder := startStateLockHolder(t, lockPath)
		defer stopLockHolder()

		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--governance", "--change", slug})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "state lock timeout")
	})
}

func TestHealthCommandGovernanceSurfacesMultipleActiveChangeError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Create two active changes at the state layer to trigger ErrMultipleActiveChanges.
		changeA := model.NewChange("health-multi-a")
		require.NoError(t, state.SaveChange(root, changeA))

		changeB := model.NewChange("health-multi-b")
		require.NoError(t, state.SaveChange(root, changeB))

		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--governance"})

		execErr := cmd.Execute()
		require.Error(t, execErr, "governance health should surface multiple active changes error")
		assert.Contains(t, execErr.Error(), "multiple active changes")
	})
}

func TestHealthCommandDoctorDoesNotFailWhenMultipleActiveChangesExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		changeA := model.NewChange("health-doctor-multi-a")
		require.NoError(t, state.SaveChange(root, changeA))

		changeB := model.NewChange("health-doctor-multi-b")
		require.NoError(t, state.SaveChange(root, changeB))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)
		assert.Contains(t, strings.Join(view.Diagnostics, "\n"), "explicit `--change` target")

		foundSelectionAction := false
		for _, action := range view.Doctor.Actions {
			assert.NotEqual(t, `slipway run --resume-response "<response>"`, action.Command)
			if action.Category == "active_change_selection" && action.Command == "slipway status" {
				foundSelectionAction = true
			}
		}
		assert.True(t, foundSelectionAction, "expected doctor to include active-change selection recovery action")
	})
}

func TestHealthCommandGovernanceWithNoActiveChangeDoesNotRenderRepoHealthFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--governance"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		assert.NotContains(t, out.String(), "Repo Health:")
		assert.NotContains(t, out.String(), "Governance Health")
	})
}

func TestClassifyGlobalHealthImpactBySeverity(t *testing.T) {
	t.Parallel()

	findings := []state.HealthFinding{
		{Category: "agent_contract", Severity: model.ReasonSeverityError},
		{Category: "lifecycle_event", Severity: model.ReasonSeverityWarning},
		{
			Category:             "codebase_map",
			Severity:             model.ReasonSeverityError,
			ActiveChangeImpact:   "non_blocking_for_active_change",
			ActiveChangeBlocking: false,
		},
	}

	classifyGlobalHealthImpact(findings)

	// Error-severity global findings fail closed: they block governed handoffs
	// for the active change.
	assert.True(t, findings[0].ActiveChangeBlocking)
	assert.Equal(t, "blocking_for_active_change", findings[0].ActiveChangeImpact)

	// Warnings and below are advisory.
	assert.False(t, findings[1].ActiveChangeBlocking)
	assert.Equal(t, "non_blocking_for_active_change", findings[1].ActiveChangeImpact)

	// A finding that already carries an explicit impact is left untouched even
	// when its severity is error.
	assert.False(t, findings[2].ActiveChangeBlocking)
	assert.Equal(t, "non_blocking_for_active_change", findings[2].ActiveChangeImpact)
}
