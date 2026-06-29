package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runInstructions(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := makeInstructionsCmd()
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestInstructionsReturnsTemplateAndGuidance(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"requirements": "## Requirements",
		"tasks":        "## Task List",
	}
	for name, marker := range cases {
		name, marker := name, marker
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := runInstructions(t, name)
			require.NoError(t, err)
			assert.Contains(t, out, "Authoring instructions")
			assert.Contains(t, out, "## Template")
			assert.Contains(t, out, marker, "should include the artifact template body")
		})
	}
}

func TestInstructionsJSONIncludesTemplateAndGuidance(t *testing.T) {
	t.Parallel()
	out, err := runInstructions(t, "requirements", "--json")
	require.NoError(t, err)

	var view instructionsView
	require.NoError(t, json.Unmarshal([]byte(out), &view))
	assert.Equal(t, "requirements", view.Artifact)
	assert.NotEmpty(t, view.Guidance)
	assert.Contains(t, view.Template, "## Requirements")
	assert.Contains(t, strings.ToUpper(view.Guidance), "MUST")
	// Issue #91: the served template is a rendered exemplar, not the raw
	// Go-template source — its consumer is an authoring skill, so unresolved
	// `{{ … }}` actions must not leak through.
	assert.NotContains(t, view.Template, "{{", "served template must be rendered, not raw Go-template source")
}

func TestInstructionsGuidanceMatchesScaffoldOwnership(t *testing.T) {
	t.Parallel()

	// intent.md is genuinely engine-scaffolded during intake.
	for _, name := range []string{"intent"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := runInstructions(t, name, "--json")
			require.NoError(t, err)
			var view instructionsView
			require.NoError(t, json.Unmarshal([]byte(out), &view))
			assert.NotContains(t, view.Guidance, "no seeded body",
				"scaffolded artifacts must not claim there is no seeded body")
			assert.Contains(t, view.Guidance, "engine may",
				"guidance should acknowledge engine-owned scaffold behavior")
		})
	}

	// assurance.md is deferred to S3_REVIEW authoring (issue #141): like the other
	// skill-authored artifacts, its guidance must state the engine does not seed it.
	for _, name := range []string{"requirements", "decision", "research", "tasks", "assurance"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := runInstructions(t, name, "--json")
			require.NoError(t, err)
			var view instructionsView
			require.NoError(t, json.Unmarshal([]byte(out), &view))
			assert.Contains(t, view.Guidance, "engine does not seed",
				"skill-authored artifacts must not be described as engine scaffold outputs")
		})
	}
}

func TestInstructionsTasksGuidanceMatchesTargetFilesGate(t *testing.T) {
	t.Parallel()
	out, err := runInstructions(t, "tasks", "--json")
	require.NoError(t, err)

	var view instructionsView
	require.NoError(t, json.Unmarshal([]byte(out), &view))
	assert.Contains(t, view.Guidance, "Every task names concrete target_files")
	assert.NotContains(t, view.Guidance, "A `task_kind: code` task")
}

// TestInstructionsTasksGuidanceTeachesComputedWaves pins the computed-wave
// contract: wave is no longer authored metadata — the engine assigns waves from
// depends_on and target_files and rejects a hand-declared `wave:` line — and the
// guidance teaches the three width rules (real execution-order dependencies
// only, precise target_files, absorb same-file steps into one task).
func TestInstructionsTasksGuidanceTeachesComputedWaves(t *testing.T) {
	t.Parallel()
	out, err := runInstructions(t, "tasks", "--json")
	require.NoError(t, err)

	var view instructionsView
	require.NoError(t, json.Unmarshal([]byte(out), &view))

	// Authored metadata no longer includes wave.
	assert.Contains(t, view.Guidance, "with depends_on, target_files, task_kind (one of: code, test, doc, ops, verification, investigation, other")
	assert.Contains(t, view.Guidance, "a documentation-only task uses `doc`, not `docs`")
	assert.NotContains(t, view.Guidance, "with wave",
		"wave must not be taught as authored task metadata")

	// The engine owns wave assignment and rejects a hand-declared wave line.
	assert.Contains(t, view.Guidance, "Do not author a `wave:` line")
	assert.Contains(t, view.Guidance, "engine rejects it and assigns waves from depends_on and target_files")
	assert.Contains(t, view.Guidance, "parallel")

	// Width rules: real dependencies only, precise target_files, absorb
	// same-file steps into one task.
	assert.Contains(t, view.Guidance, "fabricated dependencies serialize execution")
	assert.Contains(t, view.Guidance, "exact files over directories or globs")
	assert.Contains(t, view.Guidance, "same file into one task")
}

func TestInstructionsUnknownArtifactErrors(t *testing.T) {
	t.Parallel()
	_, err := runInstructions(t, "not-an-artifact")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown artifact")
}

// TestInstructionsCodebaseMapDoc covers issue #119 Phase 4: the repo-scoped
// codebase-map docs share the instructions->author contract. `slipway
// instructions architecture` returns the codebase-map template, a
// resolved_output_path under artifacts/codebase/, and the baseline facts as
// context — without requiring an active change.
func TestInstructionsCodebaseMapDoc(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		stdout, stderr, err := runRootCommandIn(root, []string{
			"instructions", "architecture", "--json",
		})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))

		assert.Equal(t, "architecture", view.Artifact)
		assert.Contains(t, view.Template, "# Architecture",
			"should serve the codebase-map template, not a bundle artifact")
		assert.Contains(t, view.Template, "Module responsibilities:")
		assert.Contains(t, filepath.ToSlash(view.ResolvedOutputPath), "artifacts/codebase/ARCHITECTURE.md",
			"resolved output path should point at the repo-scoped codebase-map doc")
		// No active change was created; codebase-map instructions resolve anyway.
		assert.Empty(t, view.Dependencies)
		assert.Empty(t, view.Unlocks)
	})
}

// TestInstructionsCodebaseMapAcceptsFileName confirms the file-name form
// ("STACK.md") resolves to the same codebase-map doc as the key ("stack"), so
// the staleness/missing-doc remediation can hand back either form.
func TestInstructionsCodebaseMapAcceptsFileName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		// Seed a manifest so the baseline scan detects real facts; STACK context
		// should then carry them as background the author preserves, not seed.
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
			[]byte("module example.com/demo\n\ngo 1.22\n"), 0o644))

		stdout, _, err := runRootCommandIn(root, []string{
			"instructions", "STACK.md", "--json",
		})
		require.NoError(t, err)

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))
		assert.Equal(t, "stack", view.Artifact)
		assert.Contains(t, view.Template, "# Stack")
		assert.Contains(t, filepath.ToSlash(view.ResolvedOutputPath), "artifacts/codebase/STACK.md")
		// The baseline scan detected this Go module, so STACK context carries the
		// real machine-extracted facts the author preserves rather than placeholder seed.
		assert.Contains(t, view.Context, "Baseline facts")
		assert.Contains(t, view.Context, "Go")
	})
}

// TestInstructionsCodebaseMapServesTemplateAndGuidance confirms codebase-map
// instructions always serve the template and guidance and never fail — the
// resolved output path is best-effort context layered on top, not a precondition.
func TestInstructionsCodebaseMapServesTemplateAndGuidance(t *testing.T) {
	t.Parallel()
	out, err := runInstructions(t, "conventions", "--json")
	require.NoError(t, err)

	var view instructionsView
	require.NoError(t, json.Unmarshal([]byte(out), &view))
	assert.Equal(t, "conventions", view.Artifact)
	assert.Contains(t, view.Template, "# Conventions")
	assert.NotEmpty(t, view.Guidance)
}

// TestInstructionsChangeAwareAuthoringPayload covers issue #119: inside a
// governed change `instructions` returns the resolved output path, the
// dependency graph with done-status, and the unlock set — so the authoring
// skill writes the real file straight to the right path and reads upstream
// inputs by path instead of having them seeded into the body.
func TestInstructionsChangeAwareAuthoringPayload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "instructions change-aware payload")

		// Exercise both done-status branches. decision.md depends on intent.md
		// and requirements.md in the expanded schema. Author intent.md (present →
		// done); requirements.md is deferred to skill authoring so it is absent by
		// default (absent → not done). The RemoveAll keeps that absence explicit
		// and robust regardless of scaffold behavior.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "intent.md",
			[]byte("# Intent\n\n## Summary\nchange-aware payload\n")))
		require.NoError(t, os.RemoveAll(filepath.Join(bundlePath, "requirements.md")))

		stdout, stderr, err := runRootCommandIn(root, []string{
			"instructions", "decision", "--json", "--change", slug,
		})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))

		assert.Equal(t, "decision", view.Artifact)
		assert.Contains(t, view.ResolvedOutputPath, "decision.md",
			"resolved output path should point at the artifact in the active bundle")

		done := map[string]bool{}
		for _, dep := range view.Dependencies {
			assert.NotEmpty(t, dep.Path, "each dependency carries a readable path")
			done[dep.Artifact] = dep.Done
		}
		require.Contains(t, done, "intent.md")
		require.Contains(t, done, "requirements.md")
		assert.True(t, done["intent.md"], "authored upstream dependency is done")
		assert.False(t, done["requirements.md"], "un-authored upstream dependency is not done")

		// tasks.md depends_on decision.md → authoring decision unlocks tasks.
		assert.Contains(t, view.Unlocks, "tasks.md")
	})
}

func TestInstructionsDependencyDoneRequiresValidUpstreamArtifact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "instructions dependency validity")

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "intent.md",
			[]byte("# Intent\n\n## Summary\ninvalid upstream dependency\n")))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "requirements.md"), []byte(`# Requirements

## Requirements

<!-- template-only; no authored requirement blocks -->
`), 0o644))

		stdout, stderr, err := runRootCommandIn(root, []string{
			"instructions", "decision", "--json", "--change", slug,
		})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))

		done := map[string]bool{}
		for _, dep := range view.Dependencies {
			done[dep.Artifact] = dep.Done
		}
		require.Contains(t, done, "requirements.md")
		assert.False(t, done["requirements.md"],
			"invalid requirements.md must not satisfy the decision dependency")
	})
}

func TestInstructionsUnlocksUseEffectivePreset(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Governance.MinPreset = model.WorkflowPresetStandard
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		slug := "effective-preset-unlocks"
		change := model.NewChange(slug)
		change.WorkflowPreset = model.WorkflowPresetLight
		change.ArtifactSchema = model.ArtifactSchemaCore
		require.NoError(t, state.SaveChange(root, change))

		stdout, stderr, err := runRootCommandIn(root, []string{
			"instructions", "tasks", "--json", "--change", slug,
		})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))
		assert.Contains(t, view.Unlocks, "assurance.md",
			"unlocks must use the effective preset, not the raw confirmed preset")
	})
}

func TestInstructionsDependenciesSkipOptionalMissingArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "optional-dependency-filter"
		change := model.NewChange(slug)
		change.ArtifactSchema = model.ArtifactSchemaCustom
		change.CustomArtifacts = []model.ArtifactDefinition{
			{Name: "tasks.md"},
			{Name: "research.md", RequiresDiscovery: true},
			{Name: "assurance.md", DependsOn: []string{"tasks.md", "research.md"}},
		}
		change.NeedsDiscovery = false
		require.NoError(t, state.SaveChange(root, change))

		stdout, stderr, err := runRootCommandIn(root, []string{
			"instructions", "assurance", "--json", "--change", slug,
		})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))
		deps := map[string]bool{}
		for _, dep := range view.Dependencies {
			deps[dep.Artifact] = dep.Done
		}
		require.Contains(t, deps, "tasks.md", "required missing upstream remains visible")
		assert.False(t, deps["tasks.md"])
		assert.NotContains(t, deps, "research.md", "optional missing upstream must not be shown as a required dependency")
	})
}

func TestInstructionsCustomArtifactUsesActiveChangeSchema(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "custom-artifact-instructions"
		change := model.NewChange(slug)
		change.ArtifactSchema = model.ArtifactSchemaCustom
		change.CustomArtifacts = []model.ArtifactDefinition{
			{Name: "intent.md"},
			{Name: "my-widget.md", DependsOn: []string{"intent.md"}},
			{Name: "widget-report.md", DependsOn: []string{"my-widget.md", "my-widget.md"}},
		}
		require.NoError(t, state.SaveChange(root, change))

		stdout, stderr, err := runRootCommandIn(root, []string{
			"instructions", "my-widget.md", "--json",
		})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))
		assert.Equal(t, "my-widget", view.Artifact)
		assert.Contains(t, view.Guidance, "Author concrete, substantive content")
		assert.Equal(t, "# my-widget\n", view.Template)
		assert.Contains(t, filepath.ToSlash(view.ResolvedOutputPath),
			"artifacts/changes/"+slug+"/my-widget.md")

		require.Len(t, view.Dependencies, 1)
		assert.Equal(t, "intent.md", view.Dependencies[0].Artifact)
		assert.False(t, view.Dependencies[0].Done)
		assert.Equal(t, []string{"widget-report.md"}, view.Unlocks,
			"unlocks must be sorted and deduplicated even if a malformed custom schema repeats dependencies")
	})
}

// TestInstructionsExplicitMissingChangeFailsClosed covers issue #119 F2: an
// explicit --change that cannot be resolved must fail closed, not silently serve
// the static exemplar — a typo'd or missing slug should not look successful.
func TestInstructionsExplicitMissingChangeFailsClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_, _, err := runRootCommandIn(root, []string{
			"instructions", "decision", "--change", "definitely-missing", "--json",
		})
		require.Error(t, err, "an unresolvable explicit --change must fail closed")
	})
}

// TestInstructionsOmittedChangeFallsBackToStatic confirms the complement: with no
// --change selector and no active change, instructions still serves the static
// exemplar and exits 0 (issue #119 F2).
func TestInstructionsOmittedChangeFallsBackToStatic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		stdout, stderr, err := runRootCommandIn(root, []string{"instructions", "decision", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))
		assert.Equal(t, "decision", view.Artifact)
		assert.Empty(t, view.ResolvedOutputPath, "no active change resolves to the static exemplar")
	})
}

func TestInstructionsOmittedChangeWarnsWhenProjectRootUnavailable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	stdout, stderr, err := runRootCommandIn(root, []string{"instructions", "decision", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stderr, "warning: serving static instructions")
	assert.Contains(t, stderr, "workspace is not initialized")

	var view instructionsView
	require.NoError(t, json.Unmarshal([]byte(stdout), &view))
	assert.Equal(t, "decision", view.Artifact)
	assert.Empty(t, view.ResolvedOutputPath,
		"uninitialized workspace falls back to the static exemplar, not a resolved payload")
}

func TestInstructionsOmittedChangeWarnsWhenActiveContextAmbiguous(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, state.SaveChange(root, model.NewChange("ambiguous-a")))
		require.NoError(t, state.SaveChange(root, model.NewChange("ambiguous-b")))

		stdout, stderr, err := runRootCommandIn(root, []string{"instructions", "decision", "--json"})
		require.NoError(t, err)
		assert.Contains(t, stderr, "warning: serving static instructions")
		assert.Contains(t, stderr, "active change context is ambiguous")

		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))
		assert.Equal(t, "decision", view.Artifact)
		assert.Empty(t, view.ResolvedOutputPath,
			"ambiguous active context falls back to the static exemplar, not a resolved payload")
	})
}

// TestInstructionsCodebaseMapTextLabelsBaselinePreserve covers issue #119 F3: the
// text output for a codebase-map doc must label its baseline context "preserve
// and extend", never "do NOT copy" — the baseline is real detected facts the
// author seeds INTO the doc, the opposite of a governed artifact's do-not-copy
// project context.
func TestInstructionsCodebaseMapTextLabelsBaselinePreserve(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
			[]byte("module example.com/demo\n\ngo 1.22\n"), 0o644))

		stdout, _, err := runRootCommandIn(root, []string{"instructions", "stack"})
		require.NoError(t, err)
		assert.Contains(t, stdout, "Baseline facts")
		assert.Contains(t, stdout, "preserve and extend")
		assert.NotContains(t, stdout, "do NOT copy",
			"codebase-map baseline must not be labeled do-not-copy")

		// JSON marks the baseline copy policy explicitly.
		jsonOut, _, err := runRootCommandIn(root, []string{"instructions", "stack", "--json"})
		require.NoError(t, err)
		var view instructionsView
		require.NoError(t, json.Unmarshal([]byte(jsonOut), &view))
		assert.True(t, view.ContextIsBaseline, "codebase-map context is baseline to preserve and extend")
	})
}
