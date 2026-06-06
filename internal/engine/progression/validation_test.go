package progression

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
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
  - wave: 1
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

func assertChecklistCoverageBlocker(t *testing.T, slug, coverRef, specContent, wantBlocker string) {
	t.Helper()

	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "artifacts", "changes", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`# Tasks

- [ ] %s implement auth flow
  - wave: 1
  - target_files: [main.go]
  - task_kind: code
  - covers: [%s]
`, "`t-01`", coverRef)
	if err := os.WriteFile(filepath.Join(tasksDir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	specPath := artifact.ResolveArtifactPath(tasksDir, slug, "requirements.md")
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
  - wave: 1
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

func TestValidateTasksChecklistDetailed_RejectsMechanicalScaffoldAtPlanAudit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("mechanical-reject")
	change.Description = "do the thing"
	change.WorkflowPreset = model.WorkflowPresetStandard
	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, model.WorkflowPresetStandard))

	// At plan-audit, the engine's placeholder scaffold must fail closed:
	// placeholder task objective + placeholder requirements (issue #91).
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	result := ValidateTasksChecklistDetailed(root, change)

	hasObjective, hasReq := false, false
	for _, b := range result.Blockers {
		if b == "plan_dimension_completeness_missing_objective:t-01" {
			hasObjective = true
		}
		if b == "plan_dimension_coverage_requirements_invalid" {
			hasReq = true
		}
	}
	require.True(t, hasObjective, "placeholder task objective must block at plan-audit, got %v", result.Blockers)
	require.True(t, hasReq, "placeholder requirements must block at plan-audit, got %v", result.Blockers)
}

func writeSubstanceGateBundle(t *testing.T, root, slug, requirements string) {
	t.Helper()
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - wave: 1
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

func TestValidateTasksChecklistDetailed_ScaffoldedGuardrailRequirementsStayCovered(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("auth-timeout")
	change.Description = "update auth middleware timeout strategy"
	change.GuardrailDomain = "auth_authz"
	change.WorkflowPreset = model.WorkflowPresetStandard

	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, model.WorkflowPresetStandard))

	result := ValidateTasksChecklistDetailed(root, change)
	for _, blocker := range result.Blockers {
		require.NotContains(t, blocker, "plan_dimension_coverage_missing_requirement")
	}
}

func TestGovernedBundleBlockers_UsesEffectivePresetForAssuranceRequirement(t *testing.T) {
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
		CurrentState:   model.StateS2Execute,
		ArtifactSchema: model.ArtifactSchemaCore,
		WorkflowPreset: model.WorkflowPresetLight,
	}
	blockers := GovernedBundleBlockers(root, change)
	found := false
	for _, blocker := range blockers {
		if blocker == "missing_required_artifact:assurance.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing_required_artifact:assurance.md under effective standard preset, got %v", blockers)
	}
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
				CurrentState: model.StateS2Execute,
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
	change.CurrentState = model.StateS2Execute
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
	for _, st := range []model.WorkflowState{model.StateS1Plan, model.StateS2Execute} {
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
	root := t.TempDir()
	change := model.Change{
		Slug:         "test-slug",
		CurrentState: model.StateS3Review,
	}
	blockers := AssuranceContractBlockers(root, change)
	if len(blockers) != 1 {
		t.Fatalf("expected 1 blocker for missing assurance.md, got %v", blockers)
	}
	if blockers[0] != "assurance_contract_missing" {
		t.Fatalf("expected assurance_contract_missing, got %s", blockers[0])
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
		CurrentState: model.StateS4Verify,
	}
	blockers := AssuranceContractBlockers(root, change)
	if len(blockers) != 0 {
		t.Fatalf("expected no blockers for valid assurance.md, got %v", blockers)
	}
}
