package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// TestCodebaseMapConsumeAdvisoryMatrix pins the advisory set against every
// status the classifier can return. The advisory fires for scaffold_only AND
// baseline (the case HasEmptyCodebaseMap misses) when a map-consuming planning
// skill is next, and is absent for populated/partial/missing and for
// non-consuming skills.
func TestCodebaseMapConsumeAdvisoryMatrix(t *testing.T) {
	t.Parallel()
	for _, skillName := range []string{progression.SkillResearchOrchestration, progression.SkillPlanAudit} {
		assert.NotEmpty(t, codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusScaffoldOnly, skillName),
			"scaffold_only consumed by %s should warn", skillName)
		assert.NotEmpty(t, codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusBaseline, skillName),
			"baseline consumed by %s should warn (HasEmptyCodebaseMap misses this)", skillName)
		assert.Empty(t, codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusPopulated, skillName),
			"populated consumed by %s must not warn", skillName)
		assert.Empty(t, codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusPartial, skillName),
			"partial consumed by %s gets no whole-map advisory", skillName)
		assert.Empty(t, codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusMissing, skillName),
			"missing consumed by %s gets the hint, not a whole-map advisory", skillName)
	}

	// Non-consuming skills never receive the advisory, even for non-durable maps.
	for _, skillName := range []string{progression.SkillWaveOrchestration, progression.SkillIntakeClarification, ""} {
		assert.Empty(t, codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusScaffoldOnly, skillName))
		assert.Empty(t, codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusBaseline, skillName))
	}

	// The advisory adds consume-time framing, not the hint's "no durable docs".
	advisory := codebaseMapConsumeAdvisory(artifact.CodebaseMapStatusScaffoldOnly, progression.SkillPlanAudit)
	assert.NotContains(t, advisory, "No durable codebase-map documents found")
}

// TestCodebaseMapStatusHasNoDurableDocs pins the empty-map technique-hint set:
// it fires for missing/scaffold_only and not for baseline/partial/populated.
func TestCodebaseMapStatusHasNoDurableDocs(t *testing.T) {
	t.Parallel()
	assert.True(t, codebaseMapStatusHasNoDurableDocs(artifact.CodebaseMapStatusMissing))
	assert.True(t, codebaseMapStatusHasNoDurableDocs(artifact.CodebaseMapStatusScaffoldOnly))
	assert.False(t, codebaseMapStatusHasNoDurableDocs(artifact.CodebaseMapStatusBaseline))
	assert.False(t, codebaseMapStatusHasNoDurableDocs(artifact.CodebaseMapStatusPartial))
	assert.False(t, codebaseMapStatusHasNoDurableDocs(artifact.CodebaseMapStatusPopulated))
}

// writePopulatedCodebaseMapDocs writes the full durable doc set with
// substantive, non-baseline content so AssessCodebaseMapDocs classifies the
// whole map populated.
func writePopulatedCodebaseMapDocs(t *testing.T, root string) {
	t.Helper()
	dir := state.CodebaseMapDir(root)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, name := range []string{
		"STACK.md", "INTEGRATIONS.md", "ARCHITECTURE.md", "STRUCTURE.md",
		"CONVENTIONS.md", "TESTING.md", "CONCERNS.md",
	} {
		body := "# " + strings.TrimSuffix(name, ".md") + "\n- Reviewed: source-backed finding for " + name + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}
}

func hasNoDurableCodebaseMapHint(view nextView) bool {
	if view.NextSkill == nil {
		return false
	}
	for _, hint := range view.NextSkill.TechniqueHints {
		if strings.Contains(hint.Reason, "No durable codebase-map documents found") {
			return true
		}
	}
	return false
}

// TestNextChangeFromRootDerivesCodebaseMapStatusFromWorktree exercises REQ-009:
// a root-checkout `slipway next --change <slug>` against a bound worktree whose
// map is durable (populated) must report that status AND must NOT emit the
// "no durable codebase-map documents found" hint. The pre-fix hint probed the
// invocation root (empty map here) while the status read the worktree map, so
// the two could contradict each other.
func TestNextChangeFromRootDerivesCodebaseMapStatusFromWorktree(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L3", "workspace consistent codebase map status")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		worktreePath := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreePath, "-b", branch)

		changeBeforeWT := change
		change.WorktreePath = worktreePath
		change.WorktreeBranch = branch
		require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
		require.NoError(t, state.SaveChange(root, change))
		writeWorktreePreflightEvidence(t, root, slug, worktreePath, branch)

		// Worktree map is durable (populated); the root checkout has no map.
		writePopulatedCodebaseMapDocs(t, worktreePath)
		require.NoFileExists(t, filepath.Join(root, "artifacts", "codebase", "ARCHITECTURE.md"))

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics", "--change", slug}, &view)

		// Status reflects the WORKTREE map, not the empty root checkout.
		assert.Equal(t, artifact.CodebaseMapStatusPopulated, view.InputContext.CodebaseMapStatus)
		// The empty-map hint must agree with the status: no "no durable docs".
		assert.False(t, hasNoDurableCodebaseMapHint(view),
			"empty-map technique hint must not contradict a populated worktree status")
	})
}

func warningsContainCodebaseMapAdvisory(warnings []string) bool {
	for _, w := range warnings {
		if strings.Contains(w, "codebase_map_advisory") {
			return true
		}
	}
	return false
}

// TestNextSurfacesCodebaseMapAdvisoryWarningEndToEnd drives a real next/run
// invocation (not just the helper) to prove the consume-time advisory lands in
// the surfaced warnings on BOTH the standard next view and the compact
// run/handoff projection when research-orchestration consumes a scaffold_only
// map. The technique hint and the advisory both fire here by design; the
// advisory is the top-level warnings entry.
func TestNextSurfacesCodebaseMapAdvisoryWarningEndToEnd(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		createGovernedRequest(t, root, "L2", "advisory warning surfaces end to end")
		// createGovernedRequest lands at S1_PLAN/research, so research-orchestration
		// is next; a scaffold_only map must trigger the consume-time advisory.
		writeScaffoldCodebaseMapDocs(t, root)

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &view)
		require.NotNil(t, view.NextSkill)
		require.Equal(t, "research-orchestration", view.NextSkill.Name)
		assert.True(t, warningsContainCodebaseMapAdvisory(view.Warnings),
			"scaffold_only map consumed by research-orchestration should surface a codebase_map_advisory warning; got %v", view.Warnings)

		var handoff nextHandoffView
		decodeNextJSON(t, []string{"--json"}, &handoff)
		assert.True(t, warningsContainCodebaseMapAdvisory(handoff.Warnings),
			"run/handoff surface should carry the codebase_map_advisory warning; got %v", handoff.Warnings)
	})
}

// TestNextOmitsCodebaseMapAdvisoryForPopulatedMap is the negative path: a
// populated map consumed by research-orchestration must NOT add the advisory.
func TestNextOmitsCodebaseMapAdvisoryForPopulatedMap(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		createGovernedRequest(t, root, "L2", "advisory absent for populated map")
		writePopulatedCodebaseMapDocs(t, root)

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &view)
		require.NotNil(t, view.NextSkill)
		require.Equal(t, "research-orchestration", view.NextSkill.Name)
		assert.False(t, warningsContainCodebaseMapAdvisory(view.Warnings),
			"populated map must not surface a codebase_map_advisory warning; got %v", view.Warnings)
		assert.False(t, hasNoDurableCodebaseMapHint(view),
			"populated map must not surface the empty-map technique hint")
	})
}

func TestAppendCatalogHintsIntakeHostDoesNotLeakRetiredScopeSkill(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "intake-clarification", nil, &nextView{})
	assert.Empty(t, hints)
}

func TestAppendCatalogHintsAttachesOnReviewHost(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "code-quality-review", nil, &nextView{})
	require.NotEmpty(t, hints)
	assert.Equal(t, "skill:independent-review", hints[0].Name)
	assert.Contains(t, hints[0].Reason, "procedure")
}

func TestAppendCatalogHintsGoalVerificationDropsRetiredFreshEvidence(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "goal-verification", nil, &nextView{})
	require.Len(t, hints, 1)
	assert.Equal(t, "skill:coverage-analysis", hints[0].Name)
}

func TestSupportHintNameUsesExportedSkillPrefix(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "skill:security-review", supportHintName("security-review"))
	assert.Equal(t, "skill:ci-triage", supportHintName("ci-triage"))
}

func TestAppendCatalogHintsVerifyHostsDoNotEmitRetiredFreshEvidence(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"final-closeout", "tdd-governance"} {
		host := host
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			hints := appendCatalogHints(nil, host, nil, &nextView{})
			assert.Empty(t, hints)
		})
	}
}

func TestAppendCatalogHintsEmptyWhenNoMatch(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "", nil, &nextView{})
	assert.Empty(t, hints)
}

func TestAppendCatalogHintsPreservesExisting(t *testing.T) {
	t.Parallel()
	existing := []techniqueHint{{Name: "slipway codebase-map", Reason: "seed"}}
	hints := appendCatalogHints(existing, "intake-clarification", nil, &nextView{})
	require.Len(t, hints, 1)
	assert.Equal(t, "slipway codebase-map", hints[0].Name)
}

func TestAppendCatalogHintsBlockersAloneDoNotPopulateSupports(t *testing.T) {
	t.Parallel()
	// After trigger-DSL removal, blocker-only signals without a host do not
	// populate support attachments — host-embedded / technique-hint bindings
	// require an active host. Blocker-based suggestions are no longer surfaced
	// through a separate channel.
	view := &nextView{Blockers: []model.ReasonCode{{Code: "missing_red_proof"}}}
	hints := appendCatalogHints(nil, "", nil, view)
	assert.Empty(t, hints)
}

func TestAppendCatalogHintsAttachesHydrateReferencesOnWaveHost(t *testing.T) {
	t.Parallel()

	hints := appendCatalogHints(nil, "wave-orchestration", nil, &nextView{})
	require.NotEmpty(t, hints)

	var rootCauseHint *techniqueHint
	for i := range hints {
		if hints[i].Name == "skill:root-cause-tracing" {
			rootCauseHint = &hints[i]
			break
		}
	}
	require.NotNil(t, rootCauseHint, "expected root-cause-tracing support hint on wave-orchestration host")
	assert.Equal(t, []string{
		"root-cause-tracing/condition-based-waiting.md",
		"root-cause-tracing/hypothesis-testing.md",
		"root-cause-tracing/root-cause-tracing.md",
	}, rootCauseHint.HydrateReferences)
	assert.True(t, slices.IsSorted(rootCauseHint.HydrateReferences), "hydrate references should be stable-sorted")
}
