package progression

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func hasAdvanceReasonCode(reasons []model.ReasonCode, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func hasAdvanceReasonDetail(reasons []model.ReasonCode, code, detail string) bool {
	for _, reason := range reasons {
		if reason.Code == code && reason.Detail == detail {
			return true
		}
	}
	return false
}

func TestAdvance_NoChangeFile(t *testing.T) {
	t.Parallel()
	_, err := Advance("/tmp/nonexistent", "bogus-slug")
	if err == nil {
		t.Fatal("expected error for missing change file")
	}
}

func TestAdvance_DispatchS1Plan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create minimal config so governed advance can load it.
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("admission-test")
	// NewChange defaults to S1_PLAN/bundle.
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	// Should dispatch to AdvanceGoverned for S1_PLAN routing.
	_, advErr := Advance(root, "admission-test")
	// We don't assert specific success/failure here — just that it didn't
	// return a "not found" error, confirming dispatch happened.
	if advErr != nil && errors.Is(advErr, os.ErrNotExist) {
		t.Fatalf("unexpected not-exist error after saving change: %v", advErr)
	}
}

func TestAdvance_DispatchGoverned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	change := model.NewChange("governed-test")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	// Should dispatch to AdvanceGoverned (L2 path).
	_, advErr := Advance(root, "governed-test")
	// We don't assert specific success/failure here — just that it didn't
	// return a "not found" error, confirming dispatch happened.
	if advErr != nil && errors.Is(advErr, os.ErrNotExist) {
		t.Fatalf("unexpected not-exist error after saving change: %v", advErr)
	}
}

func TestComputeNextGovernedState_NoNextState(t *testing.T) {
	t.Parallel()
	change := model.Change{
		CurrentState: model.StateDone,
	}
	_, err := ComputeNextGovernedState(change)
	if err == nil {
		t.Fatal("expected error for no next state from DONE")
	}
	if !errors.Is(err, ErrNoNextState) {
		t.Fatalf("expected ErrNoNextState, got %v", err)
	}
}

func TestComputeNextGovernedState_Valid(t *testing.T) {
	t.Parallel()
	change := model.Change{
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepAudit,
	}
	next, err := ComputeNextGovernedState(change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next == "" {
		t.Fatal("expected non-empty next state")
	}
}

func TestCheckGateWithIteration_MissingEvidence(t *testing.T) {
	t.Parallel()
	change := model.Change{
		Slug: "test-slug",
	}
	passingSkills := map[string]model.VerificationRecord{}
	result := CheckGateWithIteration("/tmp/nonexistent", &change, passingSkills, 3)
	if !result.Blocked {
		t.Fatal("expected blocked when plan audit evidence is missing")
	}
	found := false
	for _, b := range result.Blockers {
		if b.Code == "plan_audit_evidence_missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected plan_audit_evidence_missing blocker, got %v", result.Blockers)
	}
	if change.PlanAuditIterations != 1 {
		t.Fatalf("expected PlanAuditIterations=1, got %d", change.PlanAuditIterations)
	}
}

func TestAdvanceGoverned_BlocksWhenBundleMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "bundle-missing"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	summary, err := AdvanceGoverned(root, slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	if len(summary.Blockers) == 0 {
		t.Fatalf("expected missing bundle blockers, got %+v", summary)
	}
}

func TestAdvanceGoverned_UsesLightPlanAuditBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("light-plan-budget")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	// Manually create bundle files (light preset excludes assurance.md).
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	for _, file := range []string{"intent.md", "requirements.md", "decision.md", "tasks.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	found := false
	for _, blocker := range summary.Blockers {
		if blocker.Code == "plan_audit_iteration" && blocker.Detail == "1/2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected light preset to use 2-step plan audit budget, got %+v", summary.Blockers)
	}
}

func TestAdvanceIntake_ClarifyBlocksOnMissingSections(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("intake-clarify-test")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	// Create intent.md with only a summary (missing In Scope, Acceptance Signals)
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\n\n## Summary\nTest change\n"), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}

	// Provide skill evidence so the test reaches section validation
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked, got %+v", summary)
	}
	found := false
	for _, b := range summary.Blockers {
		if b.Code == "intake_clarification_incomplete" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected intake_clarification_incomplete blocker, got %v", summary.Blockers)
	}
}

func TestAdvanceIntake_ClarifyToConfirm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("intake-confirm-test")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## In Scope\nAdd logging\n\n## Out of Scope\nNothing\n\n## Acceptance Signals\nTests pass\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}

	// Write intake-clarification evidence so skill check passes.
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advanced, got %+v", summary)
	}
	if !strings.Contains(summary.Message, "confirm") {
		t.Fatalf("expected advance to confirm, got message: %s", summary.Message)
	}

	// Reload and verify substep changed
	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.IntakeSubStep != model.IntakeSubStepConfirm {
		t.Fatalf("expected IntakeSubStepConfirm, got %s", reloaded.IntakeSubStep)
	}
}

func TestSectionNonEmptyPrefersCanonicalIntentSectionOverSummarySourceDocument(t *testing.T) {
	t.Parallel()

	content := `# Intent

## Summary
Source document excerpt.

## Acceptance Signals
Source document says verification exists.

## Guardrail Domains
auth_authz

## In Scope
Update session timeout handling.

## Out of Scope
No session store changes.

## Acceptance Signals
<!-- What verifiable signals indicate completion -->
`

	if sectionNonEmpty(content, "## Acceptance Signals") {
		t.Fatal("expected canonical empty Acceptance Signals section to win over the earlier summary copy")
	}
}

func TestAdvanceGoverned_PresetPendingBlocksBeforeIntakeAdvance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("preset-pending-intake")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "critical"
	change.SuggestedWorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## In Scope\nAdd logging\n\n## Out of Scope\nNothing\n\n## Acceptance Signals\nTests pass\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked, got %+v", summary)
	}
	if len(summary.Blockers) != 1 || summary.Blockers[0].Code != "preset_confirmation_required" {
		t.Fatalf("expected preset_confirmation_required blocker, got %v", summary.Blockers)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.CurrentState != model.StateS0Intake {
		t.Fatalf("expected CurrentState=S0_INTAKE, got %s", reloaded.CurrentState)
	}
	if reloaded.IntakeSubStep != model.IntakeSubStepClarify {
		t.Fatalf("expected IntakeSubStepClarify, got %s", reloaded.IntakeSubStep)
	}
}

func TestAdvanceGoverned_SyncDoesNotRewriteUnchangedChangeAuthority(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("sync-no-change-rewrite")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	if err := artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""); err != nil {
		t.Fatalf("scaffold bundle: %v", err)
	}
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		t.Fatalf("bundle dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`task-a`"+` preserve change authority
  - target_files: ["cmd/next.go"]
  - task_kind: code
`), 0o644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}
	if _, err := state.MaterializeWavePlan(root, change); err != nil {
		t.Fatalf("materialize wave plan: %v", err)
	}

	changePath := state.BundleChangeFilePath(root, change.Slug)
	before := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	if err := os.Chtimes(changePath, before, before); err != nil {
		t.Fatalf("chtimes change.yaml: %v", err)
	}

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  before.Add(time.Minute),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, record)

	taskEvidence := []byte(`{
  "task_id": "task-a",
  "run_summary_version": 1,
  "task_kind": "code",
  "verdict": "fail",
  "blockers": [],
  "captured_at": "2026-04-06T10:01:00Z"
}`)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, change.Slug, 1), "task-a.json")
	if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	if err := os.WriteFile(taskPath, taskEvidence, 0o644); err != nil {
		t.Fatalf("write task evidence: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	if !hasAdvanceReasonDetail(summary.Blockers, "non_pass_task", "task-a") {
		t.Fatalf("expected non_pass_task blocker, got %v", summary.Blockers)
	}

	info, err := os.Stat(changePath)
	if err != nil {
		t.Fatalf("stat change.yaml: %v", err)
	}
	if !info.ModTime().Equal(before) {
		t.Fatalf("expected unchanged change.yaml mtime %s, got %s", before, info.ModTime())
	}

	if _, err := state.LoadExecutionSummary(root, change.Slug); err != nil {
		t.Fatalf("load execution summary: %v", err)
	}
}

func TestAdvanceIntake_ConfirmToS1Plan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("intake-to-plan-test")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepConfirm
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## Approved Summary\nConfirmed: add logging to service layer\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advanced, got %+v", summary)
	}
	if summary.ToState != model.StateS1Plan {
		t.Fatalf("expected transition to S1_PLAN, got %s", summary.ToState)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.CurrentState != model.StateS1Plan {
		t.Fatalf("expected S1_PLAN, got %s", reloaded.CurrentState)
	}
	if reloaded.IntakeSubStep != model.IntakeSubStepNone {
		t.Fatalf("expected IntakeSubStep cleared, got %s", reloaded.IntakeSubStep)
	}
}

func TestAdvanceGoverned_GScopeDoesNotBlockOnEmptyWorktreeAtS1Research(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("gscope-worktree-test")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	change.IntakeSubStep = model.IntakeSubStepNone
	change.NeedsDiscovery = true
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	// WorktreePath intentionally empty — worktree is not yet created at S1_PLAN.
	change.WorktreePath = ""
	change.WorktreeBranch = ""
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	// Write core bundle artifacts so bundle check passes.
	for _, file := range []string{"intent.md", "requirements.md", "tasks.md", "assurance.md", "decision.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	// Write a valid research.md so G_scope structure check passes.
	research := `## Alternatives Considered
### Option A
Approach A

### Selected Direction
Use Option A

## Unknowns
None critical.

## Assumptions
Standard env.

## Canonical References
Internal docs.
`
	if err := os.WriteFile(filepath.Join(bundleDir, "research.md"), []byte(research), 0o644); err != nil {
		t.Fatalf("write research.md: %v", err)
	}

	// Write research-orchestration evidence so skill check passes.
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// G_scope should NOT block due to empty worktree; worktree gate is at S2_EXECUTE.
	// The change should advance from S1_PLAN/research to S1_PLAN/bundle.
	if summary.Action == "blocked" {
		for _, b := range summary.Blockers {
			if strings.Contains(b.Code, "worktree") || strings.Contains(b.Code, "dedicated_worktree") {
				t.Fatalf("G_scope must not block on missing worktree at S1_PLAN/research: %v", summary.Blockers)
			}
		}
	}
}

func TestAdvanceGoverned_BlocksWhenAssuranceContractInvalidAtReview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	change := model.NewChange("assurance-invalid")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaCore
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	// Manually create bundle files (core schema: intent, requirements, tasks, assurance).
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	for _, file := range []string{"intent.md", "requirements.md", "tasks.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	// Write an incomplete assurance.md to trigger the structure validation blocker.
	if err := os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("## Scope Summary\nIncomplete\n"), 0o644); err != nil {
		t.Fatalf("write assurance.md: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	found := false
	for _, blocker := range summary.Blockers {
		if blocker.Code == "assurance_structure_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected assurance structure blocker, got %+v", summary.Blockers)
	}
}

func TestAdvanceGoverned_BlocksWhenExecutionSummaryHasSummaryLevelBlockers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("summary-level-blockers")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	for _, file := range []string{"intent.md", "requirements.md", "decision.md", "tasks.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	if err := state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        change.CreatedAt.Add(time.Second),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      []model.ReasonCode{model.NewReasonCode("session_isolation_warning", "session_id=abc:shared_by=task-a,task-b")},
	}); err != nil {
		t.Fatalf("save execution summary: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	if !hasAdvanceReasonDetail(summary.Blockers, "session_isolation_warning", "session_id=abc:shared_by=task-a,task-b") {
		t.Fatalf("expected summary-level execution blocker, got %+v", summary.Blockers)
	}
}
