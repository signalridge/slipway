package progression

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTasksChecklist_Missing(t *testing.T) {
	t.Parallel()
	change := model.Change{Slug: "nonexistent"}
	blockers := ValidateTasksChecklistDetailed("/tmp/nonexistent", change).Blockers
	if len(blockers) != 1 || blockers[0] != "tasks_checklist_missing" {
		t.Fatalf("expected tasks_checklist_missing, got %v", blockers)
	}
}

func TestValidateTasksChecklist_Valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "test-change"
	tasksDir := filepath.Join(dir, "artifacts", "changes", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `# Tasks

- [ ] ` + "`t-01`" + ` do something
  - target_files: [main.go]
  - task_kind: code
`
	if err := os.WriteFile(filepath.Join(tasksDir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	change := model.Change{Slug: slug}
	blockers := ValidateTasksChecklistDetailed(dir, change).Blockers
	if len(blockers) != 0 {
		t.Fatalf("expected no blockers, got %v", blockers)
	}
}

func TestValidateTasksChecklist_MissingFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "test-change"
	tasksDir := filepath.Join(dir, "artifacts", "changes", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `# Tasks

- [ ] ` + "`t-01`" + `
  - target_files: []
  - task_kind:
`
	if err := os.WriteFile(filepath.Join(tasksDir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	change := model.Change{Slug: slug}
	blockers := ValidateTasksChecklistDetailed(dir, change).Blockers
	if len(blockers) == 0 {
		t.Fatal("expected blockers for missing fields")
	}
}

func TestValidateTasksChecklist_RejectsInstructionPlaceholderTargetFilesAtPlanAudit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "test-change"
	tasksDir := filepath.Join(dir, "artifacts", "changes", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `# Tasks

- [ ] ` + "`t-01`" + ` implement target placeholder rejection
  - target_files: [<path/to/file.go>]
  - task_kind: code
`
	if err := os.WriteFile(filepath.Join(tasksDir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepAudit,
	}
	blockers := ValidateTasksChecklistDetailed(dir, change).Blockers
	for _, blocker := range blockers {
		if blocker == "plan_dimension_key_links_missing_target_files:t-01" {
			return
		}
	}
	t.Fatalf("expected placeholder target_files to block as missing concrete target_files, got %v", blockers)
}

func TestValidateTasksChecklist_RejectsPlaceholderObjectiveAtPlanAudit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "test-change"
	tasksDir := filepath.Join(dir, "artifacts", "changes", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `# Tasks

- [ ] ` + "`t-01`" + ` Pending task objective
  - target_files: [main.go]
  - task_kind: verification
`
	if err := os.WriteFile(filepath.Join(tasksDir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepAudit,
	}
	blockers := ValidateTasksChecklistDetailed(dir, change).Blockers
	for _, blocker := range blockers {
		if blocker == "plan_dimension_completeness_missing_objective:t-01" {
			return
		}
	}
	t.Fatalf("expected placeholder objective to block as missing concrete objective, got %v", blockers)
}

func assertChecklistCoverageBlocker(t *testing.T, slug, coverRef, specContent, wantBlocker string) {
	t.Helper()

	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "artifacts", "changes", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`# Tasks

- [ ] %s implement auth flow
  - target_files: [main.go]
  - task_kind: code
  - covers: [%s]
`, "`t-01`", coverRef)
	if err := os.WriteFile(filepath.Join(tasksDir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	specPath := artifact.ResolveArtifactPath(tasksDir, "requirements.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatal(err)
	}

	change := model.Change{Slug: slug}
	blockers := ValidateTasksChecklistDetailed(dir, change).Blockers
	if len(blockers) == 0 {
		t.Fatalf("expected blockers containing %s", wantBlocker)
	}
	for _, blocker := range blockers {
		if blocker == wantBlocker {
			return
		}
	}
	t.Fatalf("expected blocker %s, got %v", wantBlocker, blockers)
}

func TestValidateTasksChecklist_CoverageValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		slug        string
		coverRef    string
		specContent string
		wantBlocker string
	}{
		{
			name:     "uncovered requirements",
			slug:     "coverage-gap",
			coverRef: "REQ-001",
			specContent: `## Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.

### Requirement: Logging
REQ-002: The system must emit audit logs.
`,
			wantBlocker: "plan_dimension_coverage_missing_requirement:REQ-002",
		},
		{
			name:     "unknown requirement",
			slug:     "coverage-unknown",
			coverRef: "REQ-999",
			specContent: `## Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`,
			wantBlocker: "plan_dimension_coverage_unknown_requirement:t-01->REQ-999",
		},
		{
			name:     "heading instead of stable id",
			slug:     "coverage-heading-name",
			coverRef: "Auth",
			specContent: `## Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`,
			wantBlocker: "plan_dimension_coverage_unknown_requirement:t-01->Auth",
		},
		{
			name:     "requirement missing stable id",
			slug:     "coverage-missing-requirement-id",
			coverRef: "REQ-001",
			specContent: `## Requirements

### Requirement: Auth
The system must authenticate requests.
`,
			wantBlocker: "plan_dimension_coverage_requirement_id_missing:Auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertChecklistCoverageBlocker(t, tt.slug, tt.coverRef, tt.specContent, tt.wantBlocker)
		})
	}
}

func TestValidateTasksChecklist_RejectsUnknownRequirementRefsInArtifactProse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	slug := "unknown-prose-req"
	bundleDir := filepath.Join(dir, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Known behavior
REQ-001: The system MUST keep known requirements traceable.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement known behavior
  - target_files: [main.go]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision

The selected approach claims to satisfy REQ-999.
`), 0o644))

	result := ValidateTasksChecklistDetailed(dir, model.Change{
		Slug:           slug,
		ArtifactSchema: model.ArtifactSchemaExpanded,
	})
	assert.Contains(t, result.Blockers, "plan_dimension_consistency_unknown_requirement_ref:decision.md:REQ-999")
}

func TestValidateTasksChecklist_RejectsUnknownRequirementRefsInCustomArtifactProse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	slug := "unknown-custom-prose-req"
	bundleDir := filepath.Join(dir, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Known behavior
REQ-001: The system MUST keep known requirements traceable.

#### Scenario: Known requirement remains traceable
GIVEN a known requirement
WHEN governed artifacts cite it
THEN validation accepts the reference.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement known behavior
  - target_files: [main.go]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "custom-notes.md"), []byte(`# Custom Notes

The custom acceptance narrative claims to satisfy REQ-999.
`), 0o644))

	result := ValidateTasksChecklistDetailed(dir, model.Change{
		Slug:           slug,
		ArtifactSchema: model.ArtifactSchemaCustom,
		WorkflowPreset: model.WorkflowPresetStandard,
		CustomArtifacts: []model.ArtifactDefinition{
			{Name: "requirements.md"},
			{Name: "tasks.md", DependsOn: []string{"requirements.md"}},
			{Name: "custom-notes.md", DependsOn: []string{"requirements.md"}},
		},
	})
	assert.Contains(t, result.Blockers, "plan_dimension_consistency_unknown_requirement_ref:custom-notes.md:REQ-999")
}

func TestValidateTasksChecklist_DoesNotOwnAssuranceProseRequirementReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	slug := "assurance-owned-by-dedicated-gate"
	bundleDir := filepath.Join(dir, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Known behavior
REQ-001: The system MUST keep known requirements traceable.

#### Scenario: Known requirement remains traceable
GIVEN a known requirement
WHEN governed artifacts cite it
THEN validation accepts the reference.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement known behavior
  - target_files: [main.go]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance

This S3-owned closeout document cites REQ-999 while still being outside the
task-checklist prose scanner's ownership boundary.
`), 0o644))

	result := ValidateTasksChecklistDetailed(dir, model.Change{
		Slug:           slug,
		ArtifactSchema: model.ArtifactSchemaExpanded,
		WorkflowPreset: model.WorkflowPresetStandard,
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepAudit,
	})
	for _, blocker := range result.Blockers {
		assert.NotEqual(t, "plan_dimension_consistency_unknown_requirement_ref:assurance.md:REQ-999", blocker)
	}
}

func TestValidateTasksChecklist_IgnoresLowercaseReqHyphenWordsInArtifactProse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	slug := "lowercase-req-words"
	bundleDir := filepath.Join(dir, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Known behavior
REQ-001: The system MUST keep known requirements traceable.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement known behavior
  - target_files: [main.go]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision

The selected approach handles req-timeout, req-id, and req-body prose without
treating those ordinary lowercase API words as stable requirement references.
`), 0o644))

	result := ValidateTasksChecklistDetailed(dir, model.Change{Slug: slug})
	for _, blocker := range result.Blockers {
		assert.NotContains(t, blocker, "plan_dimension_consistency_unknown_requirement_ref")
	}
}

func TestValidateTasksChecklist_DoesNotDuplicateUnknownCoversAsProseConsistency(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	slug := "unknown-covers-only"
	bundleDir := filepath.Join(dir, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Known behavior
REQ-001: The system MUST keep known requirements traceable.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement known behavior
  - target_files: [main.go]
  - task_kind: code
  - covers: [REQ-999]
`), 0o644))

	result := ValidateTasksChecklistDetailed(dir, model.Change{Slug: slug})
	assert.Contains(t, result.Blockers, "plan_dimension_coverage_unknown_requirement:t-01->REQ-999")
	for _, blocker := range result.Blockers {
		assert.NotEqual(t, "plan_dimension_consistency_unknown_requirement_ref:tasks.md:REQ-999", blocker)
	}
}

func TestValidateTasksChecklistDetailed_DowngradesOptionalFieldsAndCoverageToWarningsForLightPreset(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := model.DefaultConfig()
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

	slug := "light-advisories"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - depends_on: []
  - target_files: [main.go]
`), 0o644))
	// Substantive requirements (MUST body + concrete scenario) so this test
	// isolates the light-preset coverage downgrade from the substance gate
	// (issue #91): the only advisory left is REQ-001 being uncovered by t-01.
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`## Requirements

### Requirement: Auth
REQ-001: The system MUST authenticate requests.

#### Scenario: Rejects anonymous request
GIVEN a request without credentials
WHEN it reaches a protected route
THEN the system returns 401.
`), 0o644))

	change := model.Change{
		Slug:           slug,
		WorkflowPreset: model.WorkflowPresetLight,
	}
	result := ValidateTasksChecklistDetailed(root, change)
	if len(result.Blockers) != 0 {
		t.Fatalf("expected no blockers for light preset optional metadata, got %v", result.Blockers)
	}
	for _, want := range []string{
		"plan_dimension_context_missing_task_kind_warning:t-01",
		"plan_dimension_coverage_missing_requirement_warning:REQ-001",
	} {
		found := false
		for _, warning := range result.Warnings {
			if warning == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected warning %s, got %v", want, result.Warnings)
		}
	}
}

func writeSubstanceGateBundle(t *testing.T, root, slug, requirements string) {
	t.Helper()
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - depends_on: []
  - target_files: [main.go]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(requirements), 0o644))
}

// Issue #91 (P1): from plan-audit onward the requirements substance gate is a
// hard blocker — not advisory — and is independent of the workflow preset. A
// requirement that is non-placeholder yet non-substantive (a MUST body with no
// concrete scenario, or a body with no RFC-2119 keyword) must block, and the
// light preset must NOT downgrade that validity failure to a warning.
func TestValidateTasksChecklistDetailed_RequirementsSubstanceGateIsHardAtPlanAudit(t *testing.T) {
	t.Parallel()

	const substantive = `## Requirements

### Requirement: Auth
REQ-001: The system MUST authenticate requests.

#### Scenario: Rejects anonymous request
GIVEN a request without credentials
WHEN it reaches a protected route
THEN the system returns 401.
`
	const noScenario = `## Requirements

### Requirement: Auth
REQ-001: The system MUST authenticate requests.
`
	const noNormative = `## Requirements

### Requirement: Auth
REQ-001: The system handles authentication for requests.
`

	planAudit := func(slug string, preset model.WorkflowPreset) model.Change {
		return model.Change{
			Slug:           slug,
			WorkflowPreset: preset,
			CurrentState:   model.StateS1Plan,
			PlanSubStep:    model.PlanSubStepAudit,
		}
	}
	contains := func(s []string, want string) bool {
		for _, v := range s {
			if v == want {
				return true
			}
		}
		return false
	}

	t.Run("MUST body with no concrete scenario blocks", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSubstanceGateBundle(t, root, "sub-no-scenario", noScenario)
		result := ValidateTasksChecklistDetailed(root, planAudit("sub-no-scenario", model.WorkflowPresetStandard))
		require.True(t, contains(result.Blockers, "plan_dimension_coverage_requirements_invalid"),
			"a non-placeholder requirement with no concrete scenario must block at plan-audit, got %v", result.Blockers)
	})

	t.Run("substantive requirements pass", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSubstanceGateBundle(t, root, "sub-ok", substantive)
		result := ValidateTasksChecklistDetailed(root, planAudit("sub-ok", model.WorkflowPresetStandard))
		require.False(t, contains(result.Blockers, "plan_dimension_coverage_requirements_invalid"),
			"substantive requirements must not trip the substance gate, got %v", result.Blockers)
	})

	t.Run("light preset does not downgrade the substance gate", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))
		writeSubstanceGateBundle(t, root, "sub-light", noNormative)
		result := ValidateTasksChecklistDetailed(root, planAudit("sub-light", model.WorkflowPresetLight))
		require.True(t, contains(result.Blockers, "plan_dimension_coverage_requirements_invalid"),
			"requirements validity must stay a hard blocker under the light preset, got blockers=%v warnings=%v", result.Blockers, result.Warnings)
		require.False(t, contains(result.Warnings, "plan_dimension_coverage_requirements_invalid_warning"),
			"requirements validity must not be downgraded to a warning, got warnings=%v", result.Warnings)
	})
}

// The hand-declared `wave:` task metadata is retired: the engine computes
// execution waves from depends_on + target_files. A checklist whose tasks
// declare objective, depends_on, target_files, task_kind, and covers — but no
// `wave:` lines — is therefore structurally complete, and validation must not
// emit the retired plan_dimension_execution_missing_wave blocker for it, even
// at plan-audit where enforcement is strictest.
func TestValidateTasksChecklistDetailed_WavelessChecklistPassesStructuralValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "waveless-valid"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - depends_on: []
  - target_files: [main.go]
  - task_kind: code
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` test auth flow
  - depends_on: [t-01]
  - target_files: [main_test.go]
  - task_kind: test
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`## Requirements

### Requirement: Auth
REQ-001: The system MUST authenticate requests.

#### Scenario: Rejects anonymous request
GIVEN a request without credentials
WHEN it reaches a protected route
THEN the system returns 401.
`), 0o644))

	change := model.Change{
		Slug:           slug,
		WorkflowPreset: model.WorkflowPresetStandard,
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepAudit,
	}

	result := ValidateTasksChecklistDetailed(root, change)
	for _, blocker := range result.Blockers {
		assert.NotContains(t, blocker, "plan_dimension_execution_missing_wave",
			"the declared-wave blocker vocabulary is retired; waveless tasks must not be blocked for a missing wave")
	}
	require.Empty(t, result.Blockers,
		"a checklist declaring objective, depends_on, target_files, task_kind, and covers but no wave: lines must pass structural validation")
}

func TestValidateTasksChecklistDetailed_InvalidFormatCarriesParserDetail(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "legacy-wave-detail"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - depends_on: []
  - target_files: [main.go]
  - task_kind: code

- [ ] `+"`t-02`"+` test auth flow
  - wave: 2
  - depends_on: [t-01]
  - target_files: [main_test.go]
  - task_kind: test
`), 0o644))

	result := ValidateTasksChecklistDetailed(root, model.Change{Slug: slug})
	require.Len(t, result.Blockers, 1)

	blocker := result.Blockers[0]
	assert.True(t, strings.HasPrefix(blocker, "tasks_checklist_invalid_format:"),
		"invalid format blocker must carry the parser detail")
	assert.Contains(t, blocker, "t-02")
	assert.Contains(t, blocker, "retired metadata key")
	assert.Contains(t, blocker, "delete the wave line")
	assert.Contains(t, blocker, "depends_on")
}

func TestValidateTasksChecklistDetailed_DependencyPrecheckSuppressesDuplicateWavePlanBlocker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tasks       string
		wantBlocker string
	}{
		{
			name: "unknown dependency",
			tasks: `# Tasks

- [ ] ` + "`t-01`" + ` implement auth flow
  - depends_on: [t-99]
  - target_files: [main.go]
  - task_kind: code
`,
			wantBlocker: "plan_dimension_dependency_unknown:t-01->t-99",
		},
		{
			name: "self dependency",
			tasks: `# Tasks

- [ ] ` + "`t-01`" + ` implement auth flow
  - depends_on: [t-01]
  - target_files: [main.go]
  - task_kind: code
`,
			wantBlocker: "plan_dimension_dependency_self_reference:t-01",
		},
		{
			name: "dependency cycle",
			tasks: `# Tasks

- [ ] ` + "`t-01`" + ` implement auth flow
  - depends_on: [t-02]
  - target_files: [main.go]
  - task_kind: code

- [ ] ` + "`t-02`" + ` test auth flow
  - depends_on: [t-01]
  - target_files: [main_test.go]
  - task_kind: test
`,
			wantBlocker: "plan_dimension_dependency_cycle_detected",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			slug := strings.ReplaceAll(tt.name, " ", "-")
			bundleDir := filepath.Join(root, "artifacts", "changes", slug)
			require.NoError(t, os.MkdirAll(bundleDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(tt.tasks), 0o644))

			result := ValidateTasksChecklistDetailed(root, model.Change{Slug: slug})
			assert.Contains(t, result.Blockers, tt.wantBlocker)
			for _, blocker := range result.Blockers {
				assert.Falsef(t, strings.HasPrefix(blocker, "plan_dimension_execution_invalid_wave_plan:"),
					"dependency precheck failures should not be duplicated as wave-plan blockers: %v", result.Blockers)
			}
		})
	}
}

func TestValidateTasksChecklistDetailed_ScaffoldedGuardrailRequirementsStayCovered(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("auth-timeout")
	change.Description = "update auth middleware timeout strategy"
	change.GuardrailDomain = "auth_authz"
	change.WorkflowPreset = model.WorkflowPresetStandard

	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, model.WorkflowPresetStandard))

	result := ValidateTasksChecklistDetailed(root, change)
	for _, blocker := range result.Blockers {
		require.NotContains(t, blocker, "plan_dimension_coverage_missing_requirement")
	}
}

func TestDecisionContractBlockers(t *testing.T) {
	t.Parallel()

	template, err := artifact.RenderArtifactExample("decision.md")
	require.NoError(t, err)
	const authored = "# Decision\n\n## Alternatives Considered\nA vs B; B wins on latency.\n\n" +
		"## Selected Approach\nB, grounded in the alternatives above.\n\n" +
		"## Interfaces and Data Flow\nnone\n\n" +
		"## Rollout and Rollback\nFeature flag; rollback flips it off. Verify with go test.\n\n" +
		"## Risk\nDuplicate delivery; idempotency keys.\n"

	expandedAudit := func(slug string) model.Change {
		return model.Change{
			Slug:           slug,
			ArtifactSchema: model.ArtifactSchemaExpanded,
			WorkflowPreset: model.WorkflowPresetStandard,
			CurrentState:   model.StateS1Plan,
			PlanSubStep:    model.PlanSubStepAudit,
		}
	}
	writeDecision := func(t *testing.T, slug, content string) string {
		t.Helper()
		root := t.TempDir()
		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.MkdirAll(bundleDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(content), 0o644))
		return root
	}

	t.Run("template-only decision blocks at plan-audit", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-tmpl", template)
		blockers := DecisionContractBlockers(root, expandedAudit("dec-tmpl"))
		require.NotEmpty(t, blockers, "an unedited template-only decision.md must block planning readiness")
		require.Contains(t, blockers[0], "decision_structure_invalid:")
		require.Contains(t, blockers[0], "non-empty content")
	})

	t.Run("authored decision passes", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-ok", authored)
		require.Empty(t, DecisionContractBlockers(root, expandedAudit("dec-ok")))
	})

	t.Run("superseded status blocks at plan audit", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-superseded", authored+"## Status\nSuperseded by DEC-001\n")
		blockers := DecisionContractBlockers(root, expandedAudit("dec-superseded"))
		require.Contains(t, blockers, "decision_status_rejected:superseded")
	})

	t.Run("lowercase superseded status heading blocks at plan audit", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-superseded-lowercase-heading", authored+"## status\nSuperseded by DEC-001\n")
		blockers := DecisionContractBlockers(root, expandedAudit("dec-superseded-lowercase-heading"))
		require.Contains(t, blockers, "decision_status_rejected:superseded")
	})

	t.Run("unknown explicit status blocks at plan audit", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-unknown-status", authored+"## Status\nRetired-ish\n")
		blockers := DecisionContractBlockers(root, expandedAudit("dec-unknown-status"))
		require.Contains(t, blockers, "decision_status_unknown:retired ish")
	})

	t.Run("conflicting status aliases fail closed at plan audit", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name     string
			content  string
			expected string
		}{
			{
				name: "accepted status superseded by lifecycle",
				content: authored + `## Status
Accepted

## Lifecycle
Superseded by DEC-001
`,
				expected: "decision_status_rejected:superseded",
			},
			{
				name: "accepted status unknown by state",
				content: authored + `## Status
Accepted

## State
Retired-ish
`,
				expected: "decision_status_unknown:retired ish",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				root := writeDecision(t, "dec-conflicting-status", tt.content)
				blockers := DecisionContractBlockers(root, expandedAudit("dec-conflicting-status"))
				require.Contains(t, blockers, tt.expected)
			})
		}
	})

	t.Run("status near misses block at plan audit", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name     string
			status   string
			expected string
		}{
			{
				name:     "inactive is unknown",
				status:   "Inactive",
				expected: "decision_status_unknown:inactive",
			},
			{
				name:     "unaccepted is unknown",
				status:   "unaccepted",
				expected: "decision_status_unknown:unaccepted",
			},
			{
				name:     "drafted is unknown",
				status:   "drafted",
				expected: "decision_status_unknown:drafted",
			},
			{
				name:     "mixed accepted superseded is rejected",
				status:   "Accepted, superseded by DEC-001",
				expected: "decision_status_rejected:superseded",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				root := writeDecision(t, "dec-near-miss", authored+"## Status\n"+tt.status+"\n")
				blockers := DecisionContractBlockers(root, expandedAudit("dec-near-miss"))
				require.Contains(t, blockers, tt.expected)
			})
		}
	})

	t.Run("status blockers are canonical readiness reasons", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-status-canonical", authored+"## Status\nDeprecated\n")
		reasons := model.ReasonCodesFromSpecs(DecisionContractBlockers(root, expandedAudit("dec-status-canonical")))
		require.Len(t, reasons, 1)
		assert.Equal(t, "decision_status_rejected", reasons[0].Code)
		assert.Equal(t, "deprecated", reasons[0].Detail)
	})

	t.Run("pre-audit draft stays lenient", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-pre", template)
		change := expandedAudit("dec-pre")
		change.PlanSubStep = model.PlanSubStepResearch
		require.Empty(t, DecisionContractBlockers(root, change),
			"a template-only decision before plan-audit must stay lenient")
	})

	t.Run("bundle draft stays lenient", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-bundle", template)
		change := expandedAudit("dec-bundle")
		change.PlanSubStep = model.PlanSubStepBundle
		require.Empty(t, DecisionContractBlockers(root, change),
			"a template-only decision before plan-audit must stay lenient")
	})

	t.Run("post-plan state enforces decision substance", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-exec", template)
		change := expandedAudit("dec-exec")
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		blockers := DecisionContractBlockers(root, change)
		require.NotEmpty(t, blockers, "post-plan states must still enforce decision substance")
		require.Contains(t, blockers[0], "decision_structure_invalid:")
		require.Contains(t, blockers[0], "non-empty content")
	})

	t.Run("core schema does not require decision substance", func(t *testing.T) {
		t.Parallel()
		root := writeDecision(t, "dec-core", template)
		change := expandedAudit("dec-core")
		change.ArtifactSchema = model.ArtifactSchemaCore
		require.Empty(t, DecisionContractBlockers(root, change),
			"decision.md is not required under the core schema")
	})

	t.Run("missing decision is owned by the bundle gate", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", "dec-missing"), 0o755))
		require.Empty(t, DecisionContractBlockers(root, expandedAudit("dec-missing")),
			"a missing decision.md is reported by GovernedBundleBlockers, not double-reported here")
	})

	t.Run("invalid bundle path returns path blocker", func(t *testing.T) {
		t.Parallel()
		blockers := DecisionContractBlockers(t.TempDir(), expandedAudit(""))
		require.Len(t, blockers, 1)
		require.Contains(t, blockers[0], "decision_contract_path_invalid:")
	})

	t.Run("unreadable decision returns unreadable blocker", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		bundleDir := filepath.Join(root, "artifacts", "changes", "dec-unreadable")
		require.NoError(t, os.MkdirAll(filepath.Join(bundleDir, "decision.md"), 0o755))
		require.Equal(t,
			[]string{"decision_contract_unreadable"},
			DecisionContractBlockers(root, expandedAudit("dec-unreadable")),
		)
	})
}

// After issue #141, assurance.md existence is owned solely by
// AssuranceContractBlockers at S3_REVIEW and later — not the generic
// GovernedBundleBlockers existence gate, which also runs before review. Even when
// the effective preset (via MinPreset) upgrades a light change to standard, the
// generic gate must NOT report a deferred assurance.md as missing and strand the
// change before S3; the dedicated contract gate fails closed once review begins.
func TestGovernedBundleBlockers_DefersAssuranceToContractGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := model.DefaultConfig()
	cfg.Governance.MinPreset = model.WorkflowPresetStandard
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

	slug := "effective-preset-assurance"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte("# Requirements"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks"), 0o644))

	change := model.Change{
		Slug:           slug,
		CurrentState:   model.StateS2Implement,
		ArtifactSchema: model.ArtifactSchemaCore,
		WorkflowPreset: model.WorkflowPresetLight,
	}

	// Pre-S3: the generic bundle gate does not strand the change on a deferred
	// assurance.md, even under the upgraded effective standard preset.
	assert.NotContains(t, GovernedBundleBlockers(root, change), "missing_required_artifact:assurance.md")

	// At S3_REVIEW the dedicated contract gate owns it and fails closed when absent.
	change.CurrentState = model.StateS3Review
	assert.Contains(t, AssuranceContractBlockers(root, change), "assurance_contract_missing")
}

func TestValidatePlanningReadinessChecksBoundWorktreeWithoutDiscovery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForValidationTests(t, root)

	change := model.NewChange("bound-non-discovery-worktree")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepValidate
	change.NeedsDiscovery = false
	change.WorktreePath = root
	change.WorktreeBranch = "main"

	result := ValidatePlanningReadiness(root, change)
	if len(result.Blockers) == 0 {
		t.Fatal("expected bound worktree validation blocker")
	}
	found := false
	for _, blocker := range result.Blockers {
		if blocker.Code == state.WorktreeReasonDedicatedRequired {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dedicated worktree blocker, got %v", result.Blockers)
	}
}

func TestShouldCheckGovernedBundle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change model.Change
		want   bool
	}{
		{
			name: "research substep defers bundle checks",
			change: model.Change{
				CurrentState: model.StateS1Plan,
				PlanSubStep:  model.PlanSubStepResearch,
			},
			want: false,
		},
		{
			name: "none substep defers bundle checks",
			change: model.Change{
				CurrentState: model.StateS1Plan,
				PlanSubStep:  model.PlanSubStepNone,
			},
			want: false,
		},
		{
			name: "bundle substep enables bundle checks",
			change: model.Change{
				CurrentState: model.StateS1Plan,
				PlanSubStep:  model.PlanSubStepBundle,
			},
			want: true,
		},
		{
			name: "audit substep enables bundle checks",
			change: model.Change{
				CurrentState: model.StateS1Plan,
				PlanSubStep:  model.PlanSubStepAudit,
			},
			want: true,
		},
		{
			name: "validate substep defers to planning readiness",
			change: model.Change{
				CurrentState: model.StateS1Plan,
				PlanSubStep:  model.PlanSubStepValidate,
			},
			want: false,
		},
		{
			name: "execution still requires governed bundle",
			change: model.Change{
				CurrentState: model.StateS2Implement,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldCheckGovernedBundle(tt.change); got != tt.want {
				t.Fatalf("ShouldCheckGovernedBundle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func initGitRepoForValidationTests(t *testing.T, root string) {
	t.Helper()
	runGitForValidationTests(t, root, "init", "-b", "main")
	runGitForValidationTests(t, root, "config", "user.email", "test@example.com")
	runGitForValidationTests(t, root, "config", "user.name", "Slipway Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("validation"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForValidationTests(t, root, "add", ".")
	runGitForValidationTests(t, root, "commit", "-m", "init")
}

func TestDeriveAndApplyWorktreeMetadataSeedsScopeMetadataForBoundWorktree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForValidationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("worktree-marker")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGitForValidationTests(t, root, "worktree", "add", worktreeRoot, "-b", branch)

	updated := change
	derivation, err := DeriveWorktreeBlockers(root, updated, map[string]model.VerificationRecord{
		SkillWorktreePreflight: {
			Verdict:   model.VerificationVerdictPass,
			Timestamp: time.Now().UTC(),
			References: []string{
				"worktree_path:" + worktreeRoot,
				"worktree_branch:" + branch,
				"baseline_verify_cmd:go test ./...",
			},
		},
	})
	require.NoError(t, err)
	require.Empty(t, derivation.Blockers)
	require.NoError(t, ApplyWorktreeMetadata(&updated, derivation))
	require.NoError(t, state.RelocateGovernedBundle(root, change, updated))
	require.NoError(t, state.SaveChange(root, updated))

	_, err = os.Stat(state.ConfigPath(worktreeRoot))
	require.NoError(t, err)
	_, err = os.Stat(state.WorkspaceScopeMarkerPath(worktreeRoot))
	require.NoError(t, err)

	loaded, err := state.LoadChange(root, change.Slug)
	require.NoError(t, err)
	wantWorktree, err := state.NormalizePath(updated.WorktreePath)
	require.NoError(t, err)
	require.Equal(t, wantWorktree, loaded.WorktreePath)
}

func runGitForValidationTests(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, string(out))
	}
}

func TestHasDependencyCycle_NoCycle(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {},
	}
	if HasDependencyCycle(deps) {
		t.Fatal("expected no cycle")
	}
}

func TestHasDependencyCycle_WithCycle(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}
	if !HasDependencyCycle(deps) {
		t.Fatal("expected cycle")
	}
}

func TestHasDependencyCycle_SelfReference(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"a": {"a"},
	}
	if !HasDependencyCycle(deps) {
		t.Fatal("expected cycle for self-reference")
	}
}

func TestGovernedBundleBlockers_MissingArtifacts(t *testing.T) {
	t.Parallel()
	change := model.Change{
		Slug: "test-slug",
	}
	blockers := GovernedBundleBlockers("/tmp/nonexistent", change)
	if len(blockers) == 0 {
		t.Fatal("expected blockers for missing artifacts")
	}
}

func TestResolveChangeSchemaDiagnosticsBlocksWhenChangeHasNoFrozenSchema(t *testing.T) {
	t.Parallel()
	resolution := ResolveChangeSchemaDiagnostics(model.Change{})
	if len(resolution.Schema) != 0 {
		t.Fatalf("expected no schema, got %v", resolution.Schema)
	}
	if len(resolution.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", resolution.Warnings)
	}
	if len(resolution.Blockers) != 1 || resolution.Blockers[0] != "artifact_schema_missing" {
		t.Fatalf("expected artifact_schema_missing blocker, got %v", resolution.Blockers)
	}
}

func TestAssuranceContractBlockers_SkippedBeforeReview(t *testing.T) {
	t.Parallel()
	// AssuranceContractBlockers should return nil for states before S3_REVIEW.
	for _, st := range []model.WorkflowState{model.StateS0Intake, model.StateS1Plan, model.StateS2Implement} {
		change := model.Change{
			Slug:         "test-slug",
			CurrentState: st,
		}
		blockers := AssuranceContractBlockers(t.TempDir(), change)
		if len(blockers) != 0 {
			t.Fatalf("expected no blockers at state %s, got %v", st, blockers)
		}
	}
}

func TestAssuranceContractBlockers_MissingFile(t *testing.T) {
	t.Parallel()
	for _, st := range []model.WorkflowState{
		model.StateS3Review,
		model.StateS3Review,
		model.StateDone,
		"S5_UNKNOWN",
	} {
		st := st
		t.Run("state "+string(st), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			change := model.Change{
				Slug:         "test-slug",
				CurrentState: st,
			}
			blockers := AssuranceContractBlockers(root, change)
			if len(blockers) != 1 {
				t.Fatalf("expected 1 blocker for missing assurance.md, got %v", blockers)
			}
			if blockers[0] != "assurance_contract_missing" {
				t.Fatalf("expected assurance_contract_missing, got %s", blockers[0])
			}
		})
	}
}

func TestAssuranceContractBlockers_InvalidStructure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-slug"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write assurance.md with only one heading.
	if err := os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("## Scope Summary\nOne\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS3Review,
	}
	blockers := AssuranceContractBlockers(root, change)
	if len(blockers) == 0 {
		t.Fatal("expected blockers for incomplete assurance.md")
	}
	found := false
	for _, b := range blockers {
		if len(b) > 0 && b[:len("assurance_structure_invalid:")] == "assurance_structure_invalid:" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected assurance_structure_invalid blocker, got %v", blockers)
	}
}

func TestAssuranceContractBlockers_ValidStructure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-slug"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `## Scope Summary
One

## Verification Verdict
Two

## Evidence Index
Three

## Requirement Coverage
Coverage mapping

## Residual Risks and Exceptions
Four

## Rollback Readiness
Rollback remains available.

## Archive Decision
Five`
	if err := os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS3Review,
	}
	blockers := AssuranceContractBlockers(root, change)
	if len(blockers) != 0 {
		t.Fatalf("expected no blockers for valid assurance.md, got %v", blockers)
	}
}
