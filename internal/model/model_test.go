package model

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestScoresDerivedAndValidation(t *testing.T) {
	s := Scores{
		Novelty:           2,
		Ambiguity:         3,
		Impact:            1,
		Risk:              4,
		ReversibilityCost: 2,
	}

	require.NoError(t, s.Validate())
	assert.Equal(t, 5, s.DiscoveryScore())
	assert.Equal(t, 7, s.ControlScore())

	invalid := s
	invalid.Risk = 5
	require.Error(t, invalid.Validate())
}

func TestRequestIDAndSlugCollision(t *testing.T) {
	id, err := NewRequestID()
	require.NoError(t, err)
	assert.True(t, IsUUIDv7(id))

	base := SlugifyTitle("Add Auth Flow")
	assert.Equal(t, "add-auth-flow", base)

	existing := map[string]struct{}{
		"add-auth-flow":   {},
		"add-auth-flow-2": {},
	}
	got := ResolveSlugCollision(base, func(candidate string) bool {
		_, ok := existing[candidate]
		return ok
	})
	assert.Equal(t, "add-auth-flow-3", got)
}

func TestTaskRunKeyContract(t *testing.T) {
	taskRuns := map[string]TaskRun{
		"task-a__rv2": {
			TaskID:            "task-a",
			RunSummaryVersion: 2,
			TaskKind:          TaskKindImplementation,
			Verdict:           TaskVerdictPass,
		},
	}
	require.NoError(t, ValidateTaskRunMap(taskRuns))

	taskRuns["task-a__rv2"] = TaskRun{
		TaskID:            "task-a",
		RunSummaryVersion: 1,
		TaskKind:          TaskKindImplementation,
		Verdict:           TaskVerdictPass,
	}
	require.Error(t, ValidateTaskRunMap(taskRuns))
}

func TestInsertTaskRunRejectsOverwrite(t *testing.T) {
	taskRuns := map[string]TaskRun{}
	first := TaskRun{
		TaskID:            "task-a",
		RunSummaryVersion: 1,
		Verdict:           TaskVerdictPass,
	}
	var err error
	taskRuns, err = InsertTaskRun(taskRuns, first)
	require.NoError(t, err)

	// Same key, different payload should be rejected.
	updated := TaskRun{
		TaskID:            "task-a",
		RunSummaryVersion: 1,
		Verdict:           TaskVerdictFail,
	}
	_, err = InsertTaskRun(taskRuns, updated)
	require.Error(t, err)
}

func TestEvidenceValidationRules(t *testing.T) {
	sessionID, err := NewRequestID()
	require.NoError(t, err)

	preRun := EvidenceRecord{
		RunSummaryVersion: 0,
		SessionID:         sessionID,
		SkillName:         "plan-audit",
		Version:           "v1",
		State:             StateS5PlanAudit,
		Verdict:           EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
	}
	require.NoError(t, preRun.Validate())

	badPreRun := preRun
	badPreRun.RunSummaryVersion = 1
	require.Error(t, badPreRun.Validate())

	runBound := EvidenceRecord{
		RunSummaryVersion: 1,
		SessionID:         sessionID,
		SkillName:         "artifact-review",
		Version:           "v1",
		State:             StateS7Review,
		Verdict:           EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
	}
	require.Error(t, runBound.Validate())

	runBound.InputHash = strings.Repeat("a", 64)
	require.NoError(t, runBound.Validate())
}

func TestEvidenceMitigationConsistency(t *testing.T) {
	sessionID, err := NewRequestID()
	require.NoError(t, err)

	record := EvidenceRecord{
		RunSummaryVersion: 0,
		SessionID:         sessionID,
		SkillName:         "plan-audit",
		Version:           "v1",
		State:             StateS5PlanAudit,
		Verdict:           EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
		MitigationTarget:  "wrong-target",
	}
	require.Error(t, record.Validate())

	record.MitigationTarget = ""
	require.NoError(t, record.Validate())
}

func TestWriteEvidenceFileCollision(t *testing.T) {
	dir := t.TempDir()
	sessionID, err := NewRequestID()
	require.NoError(t, err)

	record := EvidenceRecord{
		RunSummaryVersion: 0,
		SessionID:         sessionID,
		SkillName:         "plan-audit",
		Version:           "v1",
		State:             StateS5PlanAudit,
		Verdict:           EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
	}

	firstPath, err := WriteEvidenceFile(dir, record)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(firstPath, "--plan-audit.json"))

	record.Timestamp = time.Now().UTC().Add(time.Second)
	secondPath, err := WriteEvidenceFile(dir, record)
	require.NoError(t, err)
	assert.NotEqual(t, firstPath, secondPath)
	assert.True(t, strings.HasSuffix(secondPath, "--plan-audit--1.json"))
}

func TestConfigUnknownTopLevelPreserved(t *testing.T) {
	raw := []byte(`
defaults:
  level_mode: L2
execution:
  lock_wait_timeout_seconds: 15
custom_block:
  enabled: true
`)

	cfg, err := ParseConfigYAML(raw)
	require.NoError(t, err)
	require.Contains(t, cfg.UnknownTopLevel, "custom_block")

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "custom_block:")
	assert.Contains(t, string(out), "enabled: true")
}

func TestEffectiveLevelModeFallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Defaults.LevelMode = "invalid"
	mode, fallback := cfg.EffectiveLevelMode()
	assert.Equal(t, LevelModeAuto, mode)
	assert.True(t, fallback)
}

func TestAdmissionAndChangeSerializeRequiredCollections(t *testing.T) {
	admission := AdmissionState{
		RequestID:       mustRequestID(t),
		AdmissionStatus: AdmissionStatusActive,
		CurrentState:    StateS1Analyze,
		RouteSnapshot: RouteSnapshot{
			Scores: Scores{},
		},
	}
	b, err := yaml.Marshal(admission)
	require.NoError(t, err)
	assert.Contains(t, string(b), "level_history: []")
	assert.Contains(t, string(b), "evidence_refs: {}")

	change := ChangeState{
		RequestID:    mustRequestID(t),
		Slug:         "example",
		ChangeStatus: ChangeStatusActive,
		CurrentState: StateS4SpecBundle,
		RouteSnapshot: RouteSnapshot{
			Scores: Scores{},
		},
	}
	b, err = yaml.Marshal(change)
	require.NoError(t, err)
	assert.Contains(t, string(b), "level_history: []")
	assert.Contains(t, string(b), "evidence_refs: {}")
}

func TestEvidenceOwnershipBoundaryValidation(t *testing.T) {
	requestID := mustRequestID(t)
	admission := NewAdmissionState(requestID)
	admission.TaskRuns = map[string]TaskRun{
		"task-a__rv1": {
			TaskID:            "task-a",
			RunSummaryVersion: 1,
			Verdict:           TaskVerdictPass,
			EvidenceRef:       filepath.ToSlash(".spln/evidence/tasks/r1/rv1/task-a.json"),
		},
	}
	admission.EvidenceRefs = map[string]string{
		"dup": filepath.ToSlash(".spln/evidence/tasks/r1/rv1/task-a.json"),
	}
	require.Error(t, admission.Validate())
}

func mustRequestID(t *testing.T) string {
	t.Helper()
	id, err := NewRequestID()
	require.NoError(t, err)
	return id
}
