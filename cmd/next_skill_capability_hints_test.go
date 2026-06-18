package cmd

import (
	"bytes"
	"encoding/json"
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

// TestCodebaseMapRelevanceAdvisoryMatrix pins #80: the relevance advisory fires
// for durable (populated/partial) maps consumed by research-orchestration or
// plan-audit, is absent for non-consuming skills, and does not fire for the
// non-durable statuses owned by the consume advisory.
func TestCodebaseMapRelevanceAdvisoryMatrix(t *testing.T) {
	t.Parallel()
	// All durable-map consumers — including wave-orchestration (S2), the exact
	// handoff issue #80 reproduces — receive the relevance advisory.
	for _, skillName := range []string{
		progression.SkillResearchOrchestration,
		progression.SkillPlanAudit,
		progression.SkillWaveOrchestration,
	} {
		for _, status := range []string{artifact.CodebaseMapStatusPopulated, artifact.CodebaseMapStatusPartial} {
			adv := codebaseMapRelevanceAdvisory(status, skillName)
			assert.NotEmpty(t, adv, "%s consumed by %s should surface a relevance advisory", status, skillName)
			assert.Contains(t, adv, "reflects content presence, not scope relevance")
			assert.Contains(t, adv, "does not block progression")
		}
		// A partial map mixes durable and non-durable docs, so the advisory routes
		// the host to the per-doc states; a populated map has no gaps, so it omits
		// that clause. This keeps the runtime message aligned with the skill docs.
		assert.Contains(t, codebaseMapRelevanceAdvisory(artifact.CodebaseMapStatusPartial, skillName),
			"codebase_map_doc_states", "partial advisory must route to per-doc states")
		assert.NotContains(t, codebaseMapRelevanceAdvisory(artifact.CodebaseMapStatusPopulated, skillName),
			"codebase_map_doc_states", "populated advisory must not mention per-doc states")
		// Non-durable statuses belong to the consume advisory, not this one.
		assert.Empty(t, codebaseMapRelevanceAdvisory(artifact.CodebaseMapStatusScaffoldOnly, skillName))
		assert.Empty(t, codebaseMapRelevanceAdvisory(artifact.CodebaseMapStatusBaseline, skillName))
		assert.Empty(t, codebaseMapRelevanceAdvisory(artifact.CodebaseMapStatusMissing, skillName))
	}
	// Non-consuming skills never receive the relevance advisory.
	for _, skillName := range []string{progression.SkillIntakeClarification, progression.SkillGoalVerification, ""} {
		assert.Empty(t, codebaseMapRelevanceAdvisory(artifact.CodebaseMapStatusPopulated, skillName))
		assert.Empty(t, codebaseMapRelevanceAdvisory(artifact.CodebaseMapStatusPartial, skillName))
	}
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

// TestCodebaseMapDiscoveryAdvisoryMatrix pins the discovery-phase advisory. For a
// discovery-scoped change (needs_discovery) it fires across every non-durable
// status — including the fully missing map the consume-time advisory omits — when
// a map-consuming planning skill is next, and routes to the slipway-codebase-mapping
// skill. It is silent for populated/partial maps, for non-consuming skills, and
// for every status when needs_discovery is false (a change that opted out of
// discovery must not be nagged to map the repository).
func TestCodebaseMapDiscoveryAdvisoryMatrix(t *testing.T) {
	t.Parallel()
	nonDurable := []string{
		artifact.CodebaseMapStatusMissing,
		artifact.CodebaseMapStatusScaffoldOnly,
		artifact.CodebaseMapStatusBaseline,
	}
	for _, skillName := range []string{progression.SkillResearchOrchestration, progression.SkillPlanAudit} {
		for _, status := range nonDurable {
			advisory := codebaseMapDiscoveryAdvisory(status, skillName, true)
			assert.NotEmpty(t, advisory,
				"%s map for discovery change consumed by %s should warn", status, skillName)
			assert.Contains(t, advisory, "slipway-codebase-mapping",
				"discovery advisory must route to the mapping skill")
			assert.Contains(t, advisory, "codebase_map_advisory")
			// needs_discovery=false silences every status.
			assert.Empty(t, codebaseMapDiscoveryAdvisory(status, skillName, false),
				"%s map must not warn when needs_discovery is false", status)
		}
		// Durable maps never warn.
		assert.Empty(t, codebaseMapDiscoveryAdvisory(artifact.CodebaseMapStatusPopulated, skillName, true))
		assert.Empty(t, codebaseMapDiscoveryAdvisory(artifact.CodebaseMapStatusPartial, skillName, true))
	}
	// Non-consuming skills never receive the discovery advisory, even for a
	// missing map on a discovery-scoped change.
	for _, skillName := range []string{progression.SkillWaveOrchestration, progression.SkillIntakeClarification, ""} {
		assert.Empty(t, codebaseMapDiscoveryAdvisory(artifact.CodebaseMapStatusMissing, skillName, true))
	}
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

// writeBaselineCodebaseMapDocs writes deterministic CLI-detected facts and the
// full map set so AssessCodebaseMapDocs classifies the whole map as baseline.
func writeBaselineCodebaseMapDocs(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(`module example.com/baseline

go 1.26.3
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	_, err := artifact.EnsureCodebaseMapDocs(root)
	require.NoError(t, err)
	assessment, err := artifact.AssessCodebaseMapDocs(root)
	require.NoError(t, err)
	require.Equal(t, artifact.CodebaseMapStatusBaseline, assessment.Status)
}

// writePartialCodebaseMapDocs writes a source-backed subset and leaves the rest
// missing so AssessCodebaseMapDocs classifies the whole map as partial — the
// mixed durable/non-durable case the relevance advisory must still fire for and
// the one the consume advisory deliberately skips.
func writePartialCodebaseMapDocs(t *testing.T, root string) {
	t.Helper()
	dir := state.CodebaseMapDir(root)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, name := range []string{"STACK.md", "ARCHITECTURE.md", "STRUCTURE.md", "CONCERNS.md"} {
		body := "# " + strings.TrimSuffix(name, ".md") + "\n- Reviewed: source-backed finding for " + name + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}
	// INTEGRATIONS.md, CONVENTIONS.md, TESTING.md left missing → partial.
	assessment, err := artifact.AssessCodebaseMapDocs(root)
	require.NoError(t, err)
	require.Equal(t, artifact.CodebaseMapStatusPartial, assessment.Status)
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

func handoffHasNoDurableCodebaseMapHint(view nextHandoffView) bool {
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

func decodeRunJSON(t *testing.T, root string, args []string, v any) {
	t.Helper()
	var out bytes.Buffer
	cmd := commandForRoot(t, root, makeRunCmd())
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	require.NoError(t, cmd.Execute(), out.String())
	require.NoError(t, json.Unmarshal(out.Bytes(), v), out.String())
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

// countCodebaseMapAdvisories counts how many warnings carry the
// codebase_map_advisory prefix. The consume/discovery and relevance advisories
// are disjoint by status, so a single `next` render must surface at most one;
// asserting the count pins that call-site mutual exclusivity, not just presence.
func countCodebaseMapAdvisories(warnings []string) int {
	n := 0
	for _, w := range warnings {
		if strings.Contains(w, "codebase_map_advisory") {
			n++
		}
	}
	return n
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

// TestRunSurfacesBaselineCodebaseMapStatusAndAdvisory closes the state-mutating
// run contract directly: run --json must carry the same compact handoff status
// fields and the baseline consume-time advisory, not only inherit coverage
// transitively through next --json. run may advance from research to plan-audit
// before returning, so the assertion is against the map-consuming planning
// skills rather than one fixed sub-step.
func TestRunSurfacesBaselineCodebaseMapStatusAndAdvisory(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "run surfaces baseline codebase map advisory")
		writeBaselineCodebaseMapDocs(t, root)

		var handoff nextHandoffView
		decodeRunJSON(t, root, []string{"--json", "--change", slug}, &handoff)
		assert.Equal(t, model.StateS1Plan, handoff.CurrentState)
		require.NotNil(t, handoff.NextSkill)
		assert.Contains(t,
			[]string{progression.SkillResearchOrchestration, progression.SkillPlanAudit},
			handoff.NextSkill.Name,
		)
		assert.Equal(t, artifact.CodebaseMapStatusBaseline, handoff.InputContext.CodebaseMapStatus)
		require.NotEmpty(t, handoff.InputContext.CodebaseMapDocStates)
		assert.True(t, warningsContainCodebaseMapAdvisory(handoff.Warnings),
			"baseline map consumed by run/%s should surface a codebase_map_advisory warning; got %v", handoff.NextSkill.Name, handoff.Warnings)
		assert.False(t, handoffHasNoDurableCodebaseMapHint(handoff),
			"baseline map must not surface the empty-map technique hint")
	})
}

// TestNextSurfacesCodebaseMapRelevanceAdvisoryForPopulatedMap is the #80 path: a
// populated map consumed by research-orchestration surfaces a non-blocking
// relevance advisory (status reflects content presence, not scope relevance).
func TestNextSurfacesCodebaseMapRelevanceAdvisoryForPopulatedMap(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		createGovernedRequest(t, root, "L2", "relevance advisory for populated map")
		writePopulatedCodebaseMapDocs(t, root)

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &view)
		require.NotNil(t, view.NextSkill)
		require.Equal(t, "research-orchestration", view.NextSkill.Name)
		assert.True(t, warningsContainCodebaseMapAdvisory(view.Warnings),
			"populated map must surface a codebase_map_advisory relevance warning; got %v", view.Warnings)
		found := false
		for _, w := range view.Warnings {
			if strings.Contains(w, "reflects content presence, not scope relevance") {
				found = true
			}
		}
		assert.True(t, found,
			"populated map advisory must be the relevance framing; got %v", view.Warnings)
		// Exactly one codebase_map_advisory fires: the relevance advisory and the
		// S1 consume/discovery advisories are disjoint by status, so they never
		// double-fire at the call site.
		assert.Equal(t, 1, countCodebaseMapAdvisories(view.Warnings),
			"populated map must surface exactly one codebase_map_advisory; got %v", view.Warnings)
		// The advisory stays non-blocking and the empty-map technique hint is absent.
		assert.False(t, hasNoDurableCodebaseMapHint(view),
			"populated map must not surface the empty-map technique hint")
	})
}

// TestNextSurfacesCodebaseMapRelevanceAdvisoryForWaveOrchestration is the exact
// issue #80 live reproduction: at S2_IMPLEMENT the next skill is wave-orchestration
// and a populated (stale prior-change) map must still surface the non-blocking
// relevance advisory — the advisory is not gated to S1 planning skills.
func TestNextSurfacesCodebaseMapRelevanceAdvisoryForWaveOrchestration(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		slug := createGovernedRequest(t, root, "L2", "relevance advisory at wave-orchestration")
		writePopulatedCodebaseMapDocs(t, root)

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		change.NeedsDiscovery = false
		require.NoError(t, state.SaveChange(root, change))

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &view)
		require.NotNil(t, view.NextSkill)
		require.Equal(t, "wave-orchestration", view.NextSkill.Name)
		assert.True(t, warningsContainCodebaseMapAdvisory(view.Warnings),
			"populated map consumed at wave-orchestration (S2) must surface the relevance advisory; got %v", view.Warnings)
		found := false
		for _, w := range view.Warnings {
			if strings.Contains(w, "reflects content presence, not scope relevance") {
				found = true
			}
		}
		assert.True(t, found,
			"wave-orchestration advisory must be the relevance framing; got %v", view.Warnings)
		assert.Equal(t, 1, countCodebaseMapAdvisories(view.Warnings),
			"wave-orchestration (S2) must surface exactly one codebase_map_advisory; got %v", view.Warnings)
	})
}

// TestNextSurfacesCodebaseMapRelevanceAdvisoryForPartialMap pins #80 for the
// partial case: a partial map (some docs durable, some missing) is a durable
// status the relevance advisory fires for — not the consume advisory — so exactly
// one codebase_map_advisory surfaces and the per-doc states stay visible so the
// host can complete the unfinished set.
func TestNextSurfacesCodebaseMapRelevanceAdvisoryForPartialMap(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		createGovernedRequest(t, root, "L2", "relevance advisory for partial map")
		writePartialCodebaseMapDocs(t, root)

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &view)
		require.NotNil(t, view.NextSkill)
		require.Equal(t, "research-orchestration", view.NextSkill.Name)
		require.Equal(t, artifact.CodebaseMapStatusPartial, view.InputContext.CodebaseMapStatus)

		assert.Equal(t, 1, countCodebaseMapAdvisories(view.Warnings),
			"partial map must surface exactly one codebase_map_advisory; got %v", view.Warnings)
		found := false
		for _, w := range view.Warnings {
			if strings.Contains(w, "reflects content presence, not scope relevance") {
				found = true
			}
		}
		assert.True(t, found,
			"partial map advisory must be the relevance framing; got %v", view.Warnings)
		// The partial advisory routes the host to the per-doc states, matching the
		// consuming skills' guidance, and those states stay visible in the context.
		assert.Contains(t, strings.Join(view.Warnings, "\n"), "codebase_map_doc_states",
			"partial advisory must route the host to per-doc states; got %v", view.Warnings)
		assert.NotEmpty(t, view.InputContext.CodebaseMapDocStates,
			"partial map must expose per-doc states for host inspection; got %v", view.InputContext.CodebaseMapDocStates)
	})
}

// TestNextSurfacesMissingMapDiscoveryAdvisoryEndToEnd drives a real next
// invocation for a discovery-scoped (L3) change with NO codebase map. The map is
// missing — the case the consume-time advisory omits — so the broader discovery
// advisory must fire, route to the slipway-codebase-mapping skill, and reach both
// the standard view and the compact handoff projection. The empty-map technique
// hint must also now point at the mapping skill rather than the scaffold command.
func TestNextSurfacesMissingMapDiscoveryAdvisoryEndToEnd(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		// L3 => needs_discovery; lands at S1_PLAN/research with no map written.
		createGovernedRequest(t, root, "L3", "missing map discovery advisory surfaces")

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &view)
		require.NotNil(t, view.NextSkill)
		require.Equal(t, "research-orchestration", view.NextSkill.Name)
		require.Equal(t, artifact.CodebaseMapStatusMissing, view.InputContext.CodebaseMapStatus)

		assert.True(t, warningsContainCodebaseMapAdvisory(view.Warnings),
			"missing map on a discovery change should surface a codebase_map_advisory warning; got %v", view.Warnings)
		assert.Contains(t, strings.Join(view.Warnings, "\n"), "slipway-codebase-mapping",
			"discovery advisory must route the host to the mapping skill")

		// The empty-map technique hint routes to the mapping skill now.
		var hintName string
		for _, hint := range view.NextSkill.TechniqueHints {
			if strings.Contains(hint.Reason, "No durable codebase-map documents found") {
				hintName = hint.Name
			}
		}
		assert.Equal(t, "skill:codebase-mapping", hintName,
			"empty-map hint should route to the mapping skill")

		var handoff nextHandoffView
		decodeNextJSON(t, []string{"--json"}, &handoff)
		assert.True(t, warningsContainCodebaseMapAdvisory(handoff.Warnings),
			"run/handoff surface should carry the discovery advisory; got %v", handoff.Warnings)
	})
}

// TestNextOmitsDiscoveryAdvisoryForNonDiscoveryChange is the negative path: a
// non-discovery (L2) change with a missing map must NOT surface a discovery
// advisory — opting out of discovery means no nag to map the repository.
func TestNextOmitsDiscoveryAdvisoryForNonDiscoveryChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))
		createGovernedRequest(t, root, "L2", "non-discovery change omits discovery advisory")

		var view nextView
		decodeNextJSON(t, []string{"--json", "--diagnostics"}, &view)
		require.NotNil(t, view.NextSkill)
		require.Equal(t, artifact.CodebaseMapStatusMissing, view.InputContext.CodebaseMapStatus)
		assert.False(t, warningsContainCodebaseMapAdvisory(view.Warnings),
			"non-discovery change with a missing map must not surface a codebase_map_advisory; got %v", view.Warnings)
	})
}

func TestAppendCatalogHintsIntakeHostDoesNotLeakRetiredScopeSkill(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "intake-clarification", nil, &nextView{})
	assert.Empty(t, hints)
}

func TestAppendCatalogHintsDoesNotDuplicateSelectedReviewPeers(t *testing.T) {
	t.Parallel()
	hints := appendCatalogHints(nil, "code-quality-review", nil, &nextView{})
	assert.Empty(t, hints)
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
	var testDesignHint *techniqueHint
	for i := range hints {
		switch hints[i].Name {
		case "skill:root-cause-tracing":
			rootCauseHint = &hints[i]
		case "skill:test-design":
			testDesignHint = &hints[i]
		}
	}
	require.NotNil(t, rootCauseHint, "expected root-cause-tracing support hint on wave-orchestration host")
	assert.Equal(t, []string{
		"root-cause-tracing/condition-based-waiting.md",
		"root-cause-tracing/hypothesis-testing.md",
		"root-cause-tracing/root-cause-tracing.md",
	}, rootCauseHint.HydrateReferences)
	assert.True(t, slices.IsSorted(rootCauseHint.HydrateReferences), "hydrate references should be stable-sorted")

	require.NotNil(t, testDesignHint, "expected test-design support hint on wave-orchestration host")
	assert.Equal(t, []string{
		"test-design/behavior-vs-implementation.md",
		"test-design/case-enumeration.md",
		"test-design/property-reasoning.md",
		"test-design/test-data.md",
		"test-design/test-doubles.md",
	}, testDesignHint.HydrateReferences)
	assert.True(t, slices.IsSorted(testDesignHint.HydrateReferences), "hydrate references should be stable-sorted")
}
