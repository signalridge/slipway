package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func TestAppendLanguageTestingHintsEmitsOnePerProjectContextLanguage(t *testing.T) {
	t.Parallel()

	change := &model.Change{
		ProjectContext: model.ProjectContext{
			Languages: []string{"Go", " TypeScript ", "go", " "},
		},
	}

	hints := appendLanguageTestingHints(seedTestDesignHints(), t.TempDir(), change)
	languageHints := collectLanguageTestingHints(hints)

	require.Len(t, languageHints, 2)
	assert.Equal(t, []string{"Go", "TypeScript"}, collectHintLanguages(languageHints))
	for _, hint := range languageHints {
		assert.Equal(t, "capability:language-testing", hint.Name)
		assert.Equal(t, "capability", hint.Kind)
		assert.Equal(t, "language-testing", hint.Capability)
		assert.True(t, hint.Optional)
		assert.Contains(t, hint.Reason, hint.Language)
	}
}

func TestAppendLanguageTestingHintsRequiresTestDesignHint(t *testing.T) {
	t.Parallel()

	change := &model.Change{
		ProjectContext: model.ProjectContext{Languages: []string{"Go"}},
	}

	hints := appendLanguageTestingHints(nil, t.TempDir(), change)
	assert.Empty(t, collectLanguageTestingHints(hints))
}

func TestAppendLanguageTestingHintsProjectContextPrecedesStack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeStackLanguages(t, root, "- Languages: Go, Python\n")
	change := &model.Change{
		ProjectContext: model.ProjectContext{Languages: []string{"Go"}},
	}

	hints := appendLanguageTestingHints(seedTestDesignHints(), root, change)
	assert.Equal(t, []string{"Go"}, collectHintLanguages(collectLanguageTestingHints(hints)))
}

func TestAppendLanguageTestingHintsFallsBackToStackWhenProjectContextMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeStackLanguages(t, root, "- Languages: Go, Python, go\n")

	hints := appendLanguageTestingHints(seedTestDesignHints(), root, &model.Change{})
	assert.Equal(t, []string{"Go", "Python"}, collectHintLanguages(collectLanguageTestingHints(hints)))
}

func TestAppendLanguageTestingHintsFallsBackToBoundWorktreeStack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "bound-worktree")
	writeStackLanguages(t, worktreeRoot, "- Languages: Go\n")

	change := &model.Change{WorktreePath: worktreeRoot}
	hints := appendLanguageTestingHints(seedTestDesignHints(), root, change)

	assert.Equal(t, []string{"Go"}, collectHintLanguages(collectLanguageTestingHints(hints)))
}

func TestAppendLanguageTestingHintsOmitsEmptyStackLanguages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeStackLanguages(t, root, "- Languages:\n")

	hints := appendLanguageTestingHints(seedTestDesignHints(), root, &model.Change{})
	assert.Empty(t, collectLanguageTestingHints(hints))
}

func TestAppendLanguageTestingHintsIgnoresDisabledControls(t *testing.T) {
	t.Parallel()

	change := &model.Change{
		ProjectContext:      model.ProjectContext{Languages: []string{"Go"}},
		CallerDisabledCtrls: []model.ControlID{model.ControlResearch},
	}

	hints := appendLanguageTestingHints(seedTestDesignHints(), t.TempDir(), change)
	assert.Equal(t, []string{"Go"}, collectHintLanguages(collectLanguageTestingHints(hints)))
}

func TestBuildNextHandoffViewPreservesLanguageTestingCapabilityFields(t *testing.T) {
	t.Parallel()

	view := nextView{
		NextSkill: &nextSkillView{
			Name: "wave-orchestration",
			TechniqueHints: []techniqueHint{
				{
					Name:              "capability:language-testing",
					Kind:              "capability",
					Capability:        "language-testing",
					Language:          "Go",
					Optional:          true,
					Reason:            "[idiom:Go] use installed Go testing guidance when available",
					HydrateReferences: []string{"test-design/test-doubles.md"},
				},
			},
		},
	}

	handoff := buildNextHandoffView(view)
	require.NotNil(t, handoff.NextSkill)
	require.Len(t, handoff.NextSkill.TechniqueHints, 1)
	assert.Equal(t, view.NextSkill.TechniqueHints[0], handoff.NextSkill.TechniqueHints[0])

	view.NextSkill.TechniqueHints[0].HydrateReferences[0] = "mutated.md"
	assert.Equal(t, []string{"test-design/test-doubles.md"}, handoff.NextSkill.TechniqueHints[0].HydrateReferences)
}

func seedTestDesignHints() []techniqueHint {
	return []techniqueHint{
		{
			Name:   "skill:test-design",
			Reason: "seeded by wave-orchestration catalog support",
		},
	}
}

func collectLanguageTestingHints(hints []techniqueHint) []techniqueHint {
	languageHints := make([]techniqueHint, 0, len(hints))
	for _, hint := range hints {
		if hint.Name == "capability:language-testing" {
			languageHints = append(languageHints, hint)
		}
	}
	return languageHints
}

func collectHintLanguages(hints []techniqueHint) []string {
	languages := make([]string, 0, len(hints))
	for _, hint := range hints {
		languages = append(languages, hint.Language)
	}
	return languages
}

func writeStackLanguages(t *testing.T, root string, languageLine string) {
	t.Helper()

	dir := state.CodebaseMapDir(root)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := "# Stack\n\n" + languageLine
	require.NoError(t, os.WriteFile(filepath.Join(dir, "STACK.md"), []byte(content), 0o644))
}
