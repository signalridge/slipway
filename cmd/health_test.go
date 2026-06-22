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

func TestHealthCommandReportsLegacyRuntimeHandoff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		legacyPath := filepath.Join(state.GitRuntimeDir(root), "handoff-s3.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(legacyPath), 0o755))
		require.NoError(t, os.WriteFile(legacyPath, []byte("old handoff"), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		found := false
		for _, finding := range view.Findings {
			if finding.Category != "runtime_hygiene" {
				continue
			}
			if !healthFindingHasReasonCode(finding, "legacy_runtime_handoff") {
				continue
			}
			found = true
			assert.False(t, finding.Repairable)
			assert.Contains(t, finding.RepairHint, "runtime/changes/<slug>/handoff.md")
			require.NotEmpty(t, finding.Reasons)
			assert.Equal(t, "legacy_runtime_handoff", finding.Reasons[0].Code)
			assert.Contains(t, finding.Reasons[0].Detail, "handoff-s3.md")
		}
		assert.True(t, found, "expected legacy runtime handoff finding")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "doctor should surface wave repair")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` repairable wave artifact
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
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
			if action.Command == "slipway repair" && strings.Contains(strings.ToLower(action.Summary), "wave runs") {
				found = true
				assert.True(t, action.Repairable)
			}
		}
		assert.True(t, found, "expected doctor to recommend slipway repair for missing wave-run artifacts")
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

func TestHealthCommandDoctorDoesNotSuggestResumeBeforeWaveRunsExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "doctor should wait for wave runs before resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - depends_on: []
  - target_files: ["cmd/health.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
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
			if action.Category == "wave_execution" && action.Slug == slug && action.Command == "slipway repair" {
				foundRepair = true
			}
		}
		assert.True(t, foundRepair, "expected doctor to recommend repair before resume when wave runs are missing")
	})
}

func TestHealthCommandDoctorDoesNotSuggestResumeWhenWavePlanIsMissingBeforeExecutionSummaryReady(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "doctor should not suggest resume before pre-summary wave plan repair")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--doctor", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Doctor)

		for _, action := range view.Doctor.Actions {
			assert.NotEqual(t, "slipway run --resume", action.Command)
		}
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "doctor should explain interrupted execution")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		change.InterruptedExecutionAt = time.Date(2026, time.April, 11, 10, 30, 0, 0, time.UTC)
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - depends_on: []
  - target_files: ["cmd/health.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
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

func TestHealthCommandDoctorIgnoresMissingPersistedWavePlanDuringS2(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "doctor should ignore missing persisted wave cache")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` original execution task
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		tasksPath := filepath.Join(bundlePath, "tasks.md")
		updatedAt := time.Now().UTC().Add(2 * time.Second)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-02`"+` replacement task after drift
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
			if strings.Contains(strings.ToLower(action.Summary), "derived wave plan is missing") {
				found = true
			}
		}
		assert.False(t, found, "S2 doctor must not surface missing persisted wave-plan cache")
	})
}

func TestHealthCommandMarksUnreadableExecutionSummaryRepairableWhenWaveEvidenceExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health should promote repairable execution summary finding")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
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

func TestHealthCommandDoctorIgnoresPersistedWavePlanDriftDuringS2(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "doctor should ignore stale persisted wave cache")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve original task
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-02`"+` replace task after drift
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
			if strings.Contains(strings.Join([]string{action.Summary, action.Command}, "\n"), "wave_plan_drift") ||
				strings.Contains(strings.ToLower(action.Summary), "wave plan drift") {
				found = true
			}
		}
		assert.False(t, found, "S2 doctor must not treat stale persisted wave-plan cache as drift")

		foundFinding := false
		for _, finding := range view.Findings {
			if finding.Category != "wave_execution" || finding.Slug != slug {
				continue
			}
			if strings.Contains(strings.Join(model.ReasonSpecs(finding.Reasons), "\n"), "wave_plan_drift") {
				foundFinding = true
			}
		}
		assert.False(t, foundFinding, "S2 health must derive from current tasks instead of persisted wave-plan cache")
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
			if finding.Category != "skill_contract" || finding.Slug != "intake-clarification" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "skill_prompt_surface_missing" {
					found = true
					assert.Contains(t, reason.Detail, "slipway-intake-clarification")
					assert.Contains(t, finding.RepairHint, "slipway init --tools claude --refresh")
				}
			}
		}
		assert.True(t, found, "expected missing host skill surface finding")
	})
}

func TestHealthCommandReportsInvalidHostSkillRegistry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))
		skillPath := filepath.Join(root, ".claude", "skills", "slipway-intake-clarification", "SKILL.md")
		require.NoError(t, os.WriteFile(skillPath, []byte("---\nskill_id: [\n---\n"), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		found := false
		for _, finding := range view.Findings {
			if finding.Category != "skill_contract" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "skill_registry_invalid" {
					found = true
					assert.NotEmpty(t, reason.Detail)
					assert.Contains(t, finding.RepairHint, "Inspect generated host skill surfaces")
				}
			}
		}
		assert.True(t, found, "expected invalid host skill registry finding")
	})
}

func TestHealthCommandDoctorSkipsNonCommandSkillContractRepairHints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))
		skillPath := filepath.Join(root, ".claude", "skills", "slipway-intake-clarification", "SKILL.md")
		require.NoError(t, os.WriteFile(skillPath, []byte("body\n"), 0o644))

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
			if action.Category != "skill_contract" {
				continue
			}
			found = true
			assert.Empty(t, action.Command)
			assert.False(t, action.Repairable)
			assert.Contains(t, action.Summary, "Governance skill registry is invalid")
		}
		assert.True(t, found, "expected non-repairable skill contract doctor action")
	})
}

func TestHealthCommandReportsInvalidHostSkillRegistryFromInvocationWorktree(t *testing.T) {
	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

	worktreeRoot := filepath.Join(t.TempDir(), "health-worktree")
	runGit(t, root, "worktree", "add", worktreeRoot, "-b", "feat/health-worktree", "HEAD")
	require.NoError(t, bootstrap.InitWorkspace(worktreeRoot, []string{"claude"}, false))

	skillPath := filepath.Join(worktreeRoot, ".claude", "skills", "slipway-intake-clarification", "SKILL.md")
	require.NoError(t, os.WriteFile(skillPath, []byte("body\n"), 0o644))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(worktreeRoot))
	defer func() {
		_ = os.Chdir(previousWD)
	}()

	var out bytes.Buffer
	cmd := makeHealthCmd()
	cmd.SetArgs([]string{"--json"})
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	var view healthView
	require.NoError(t, json.Unmarshal(out.Bytes(), &view))

	found := false
	for _, finding := range view.Findings {
		if finding.Category != "skill_contract" {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "skill_registry_invalid" {
				found = true
				assert.Contains(t, reason.Detail, "missing frontmatter")
				assert.Contains(t, reason.Detail, "health-worktree")
			}
		}
	}
	assert.True(t, found, "expected invocation worktree skill registry finding")
}

func TestHealthCommandReportsUnreadableHostSkillSurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))
		skillDir := filepath.Join(root, ".claude", "skills", "slipway-intake-clarification")
		require.NoError(t, os.RemoveAll(skillDir))
		require.NoError(t, os.WriteFile(skillDir, []byte("not a directory"), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		found := false
		for _, finding := range view.Findings {
			if finding.Category != "skill_contract" || finding.Slug != "intake-clarification" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "skill_prompt_surface_unreadable" {
					found = true
					assert.Contains(t, reason.Detail, "slipway-intake-clarification")
					assert.Contains(t, finding.RepairHint, "slipway init --tools claude --refresh")
				}
			}
		}
		assert.True(t, found, "expected unreadable host skill surface finding")
	})
}

func TestHealthCommandDoesNotReportToolResolutionFailureForMultiAdapterWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex", "claude"}, false))

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health should stay query-only in multi-adapter workspace")

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))

		for _, finding := range view.Findings {
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
			assert.NotContains(t, model.ReasonSpecs(finding.Reasons), ".claude/agents/slipway-planner.md")
			for _, reason := range finding.Reasons {
				assert.NotEqual(t, "agent_generated_surface_missing", reason.Code)
				assert.NotEqual(t, "agent_generated_surface_unreadable", reason.Code)
				assert.NotEqual(t, ".claude/agents/slipway-planner.md", reason.Detail)
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
			if finding.Category != "skill_contract" || finding.Slug != "intake-clarification" {
				continue
			}
			for _, reason := range finding.Reasons {
				if reason.Code == "skill_prompt_surface_missing" {
					found = true
					assert.Contains(t, reason.Detail, ".claude")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "doctor should include governance failures")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health observations")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health unreadable snapshot")
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

// TestHealthCommandDoctorSurfacesGaplessTraceabilityWarning is the end-to-end
// regression for the over-broad #92 doctor suppression: an unreadable governance
// snapshot yields a gapless traceability_coherence WARN ("data unavailable"),
// which must still surface as a doctor action. Only traceability checks that
// carry advisory (non-blocking) gaps are suppressed.
func TestHealthCommandDoctorSurfacesGaplessTraceabilityWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health doctor gapless traceability")
		snapshotPath := governance.SnapshotPath(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
		require.NoError(t, os.WriteFile(snapshotPath, []byte("version: ["), 0o644))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeHealthCmd())
		cmd.SetArgs([]string{"--json", "--governance", "--doctor", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)
		require.NotNil(t, view.Doctor)

		var traceFound bool
		for _, check := range view.Governance.Checks {
			if check.Name == "traceability_coherence" {
				traceFound = true
				assert.Equal(t, "WARN", check.Status)
				assert.Empty(t, check.TraceabilityGaps, "data-unavailable WARN carries no gaps")
			}
		}
		require.True(t, traceFound, "expected a traceability_coherence check")

		hasTraceAction := false
		for _, action := range view.Doctor.Actions {
			if action.Category == "governance_traceability_coherence" {
				hasTraceAction = true
			}
		}
		assert.True(t, hasTraceAction,
			"a gapless traceability WARN (unreadable snapshot) must still surface as a doctor action")
	})
}

func TestHealthCommandGovernanceObservationsStillRenderWhenSnapshotUnreadable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health unreadable snapshot observations")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health invalid bound worktree")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health live recompute")
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
				require.NotEmpty(t, check.TraceabilityGaps)
				foundGapDetails := false
				for _, gap := range check.TraceabilityGaps {
					if gap.ID == "update auth middleware" && gap.Type == "task" {
						foundGapDetails = true
						assert.Contains(t, gap.Issue, "task covers no requirement")
						assert.True(t, gap.Blocking)
					}
				}
				assert.True(t, foundGapDetails, "expected actionable task traceability gap details")
			}
		}
		assert.True(t, found, "expected traceability_coherence check")
	})
}

func TestHealthCommandGovernanceRecomputeDropsResolvedClarificationControl(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health resolved clarification")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.NeedsDiscovery = true
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
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
  covers: [REQ-001]
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Patch the current middleware.

## Selected Approach
Choose Option A.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Requirement Coverage
REQ-001: pending
`), 0o644))

		require.NoError(t, governance.SaveSnapshot(root, slug, model.GovernanceSnapshot{
			Version: model.GovernanceSnapshotVersion,
			Summary: model.SignalSummary{
				BlastRadius: model.SignalLevelLow,
			},
			Traceability: model.TraceabilitySummary{
				Status: model.TraceabilityStatusFail,
				Gaps: []model.TraceabilityGap{
					{ID: "INT-001", Type: "intent", Issue: "blocking open questions remain unresolved", Blocking: true},
				},
			},
			ActiveControls: []model.ControlActivation{
				{
					ControlID:    model.ControlClarification,
					Mode:         model.ControlModeBlocking,
					Scope:        model.ControlScopeDiscovery,
					Active:       true,
					TriggeredBy:  []string{"traceability: blocking open questions remain unresolved"},
					PolicySource: model.BuiltinPolicySource,
				},
			},
			ComputedAt: time.Now().UTC().Add(-time.Hour),
		}))

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
			if check.Name == "signal_control_coherence" {
				found = true
				assert.Equal(t, "OK", check.Status)
				assert.NotContains(t, check.Message, "clarification")
			}
		}
		assert.True(t, found, "expected signal_control_coherence check")
	})
}

func TestHealthCommandGovernancePreservesPersistedFreshnessSignal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health stale persisted snapshot")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health refreshed snapshot")
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

		slug := createGovernedRequest(t, root, levelNonDiscovery, "health lock")
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
		{Category: "skill_contract", Severity: model.ReasonSeverityError},
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

// TestGovernanceDoctorActionsSuppressNonBlockingTraceabilityWarning pins the
// doctor-synthesis contract for issue #92: a traceability_coherence check whose
// gaps are all non-blocking (advisory, e.g. pre-review assurance coverage) must
// NOT become a doctor action, while a check with a blocking gap still does.
func TestGovernanceDoctorActionsSuppressNonBlockingTraceabilityWarning(t *testing.T) {
	t.Parallel()

	const assuranceIssue = "requirement missing assurance coverage verdict"

	advisory := &governance.GovernanceHealthReport{
		Slug: "stage-aware",
		Checks: []governance.GovernanceHealthCheck{{
			Name:    "traceability_coherence",
			Status:  "WARN",
			Message: "1 traceability warnings",
			TraceabilityGaps: []model.TraceabilityGap{{
				ID:       "REQ-002",
				Type:     "assurance",
				Issue:    assuranceIssue,
				Blocking: false,
			}},
		}},
	}
	for _, action := range governanceDoctorActions(advisory) {
		assert.NotEqual(t, "governance_traceability_coherence", action.Category,
			"a non-blocking traceability warning must not become a doctor action (#92)")
	}

	blocking := &governance.GovernanceHealthReport{
		Slug: "stage-aware",
		Checks: []governance.GovernanceHealthCheck{{
			Name:    "traceability_coherence",
			Status:  "FAIL",
			Message: "1 blocking traceability gaps",
			TraceabilityGaps: []model.TraceabilityGap{{
				ID:       "REQ-002",
				Type:     "assurance",
				Issue:    assuranceIssue,
				Blocking: true,
			}},
		}},
	}
	found := false
	for _, action := range governanceDoctorActions(blocking) {
		if action.Category == "governance_traceability_coherence" {
			found = true
			assert.False(t, action.Repairable)
		}
	}
	assert.True(t, found, "a blocking traceability gap must still surface as a doctor action")

	// Regression: a gapless traceability_coherence WARN (e.g. no-snapshot or
	// unreadable-snapshot "data unavailable") is NOT an advisory gap and must
	// still surface as a doctor action. The suppression is scoped to checks
	// that actually carry advisory gaps, not to every non-blocking WARN.
	gapless := &governance.GovernanceHealthReport{
		Slug: "stage-aware",
		Checks: []governance.GovernanceHealthCheck{{
			Name:    "traceability_coherence",
			Status:  "WARN",
			Message: "governance_audit_data_unavailable: parse governance snapshot",
		}},
	}
	foundGapless := false
	for _, action := range governanceDoctorActions(gapless) {
		if action.Category == "governance_traceability_coherence" {
			foundGapless = true
		}
	}
	assert.True(t, foundGapless,
		"a gapless traceability WARN (missing/unreadable governance data) must still surface as a doctor action")
}

// TestHealthCommandDoctorTracksAssuranceBlockingState is the end-to-end
// regression for #92 and issue #141's deferred assurance file: at S2_IMPLEMENT
// incomplete assurance coverage is an advisory WARN with no doctor action,
// while at S3_REVIEW incomplete or missing assurance fails closed and surfaces a
// doctor action. The bundle is authored so the assurance gap is the only
// traceability gap, so the WARN/FAIL difference is attributable to the
// stage-aware rule alone.
func TestHealthCommandDoctorTracksAssuranceBlockingState(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		state          model.WorkflowState
		writeAssurance bool
		assuranceBody  string
		wantStatus     string
		wantAction     bool
		wantGapIssue   string
	}{
		{
			name:           "S2 assurance pending is advisory",
			state:          model.StateS2Implement,
			writeAssurance: true,
			assuranceBody: `# Assurance
## Requirement Coverage
REQ-001: verified via tests
`,
			wantStatus:   "WARN",
			wantAction:   false,
			wantGapIssue: "requirement missing assurance coverage verdict",
		},
		{
			name:           "S3 assurance pending blocks",
			state:          model.StateS3Review,
			writeAssurance: true,
			assuranceBody: `# Assurance
## Requirement Coverage
REQ-001: verified via tests
`,
			wantStatus:   "FAIL",
			wantAction:   true,
			wantGapIssue: "requirement missing assurance coverage verdict",
		},
		{
			name:         "S3 missing assurance blocks",
			state:        model.StateS3Review,
			wantStatus:   "FAIL",
			wantAction:   true,
			wantGapIssue: "assurance.md missing at review/verify phase",
		},
		{
			name:         "DONE missing assurance blocks",
			state:        model.StateDone,
			wantStatus:   "FAIL",
			wantAction:   true,
			wantGapIssue: "assurance.md missing at review/verify phase",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			withCommandWorkspace(t, root, func() {
				initTestWorkspace(t, root)

				slug := createGovernedRequest(t, root, levelNonDiscovery, "assurance stage-aware doctor")
				change, err := state.LoadChange(root, slug)
				require.NoError(t, err)
				change.CurrentState = tc.state
				change.ArtifactSchema = model.ArtifactSchemaExpanded
				require.NoError(t, state.SaveChange(root, change))

				bundleDir := filepath.Join(root, "artifacts", "changes", slug)
				require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: stabilize the surface
## Open Questions
(none)
`), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: First
REQ-001: First requirement. Traces to INT-001.
### Requirement: Second
REQ-002: Second requirement. Traces to INT-001.
`), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] `+"`t-01`"+` do the work
  covers: [REQ-001, REQ-002]
`), 0o644))
				assurancePath := filepath.Join(bundleDir, "assurance.md")
				if tc.writeAssurance {
					require.NoError(t, os.WriteFile(assurancePath, []byte(tc.assuranceBody), 0o644))
				} else if err := os.Remove(assurancePath); err != nil && !os.IsNotExist(err) {
					require.NoError(t, err)
				}

				// Persist a snapshot at the change's current state so the health
				// command has authoritative governance state to report even if
				// it skips recompute for the (non-dedicated) worktree binding.
				_, err = governance.RecomputeGovernanceSnapshot(root, change, bundleDir)
				require.NoError(t, err)

				var out bytes.Buffer
				cmd := commandForRoot(t, root, makeHealthCmd())
				cmd.SetArgs([]string{"--json", "--governance", "--doctor", "--change", slug})
				cmd.SetOut(&out)
				require.NoError(t, cmd.Execute())

				var view healthView
				require.NoError(t, json.Unmarshal(out.Bytes(), &view))
				require.NotNil(t, view.Governance)
				require.NotNil(t, view.Doctor)

				var traceStatus string
				var traceGaps []model.TraceabilityGap
				for _, c := range view.Governance.Checks {
					if c.Name == "traceability_coherence" {
						traceStatus = c.Status
						traceGaps = c.TraceabilityGaps
					}
				}
				assert.Equal(t, tc.wantStatus, traceStatus,
					"assurance gap should be the only traceability gap")
				assert.True(t, hasTraceabilityGapIssue(traceGaps, tc.wantGapIssue),
					"traceability gap should identify the assurance problem")

				hasTraceAction := false
				for _, action := range view.Doctor.Actions {
					if action.Category == "governance_traceability_coherence" {
						hasTraceAction = true
					}
				}
				assert.Equal(t, tc.wantAction, hasTraceAction,
					"doctor traceability action presence must track blocking state (#92)")
			})
		})
	}
}

func hasTraceabilityGapIssue(gaps []model.TraceabilityGap, issue string) bool {
	for _, gap := range gaps {
		if gap.Issue == issue {
			return true
		}
	}
	return false
}
