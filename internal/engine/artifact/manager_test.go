package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffoldGovernedBundleL2CreatesRequiredFiles(t *testing.T) {
	root := t.TempDir()
	err := ScaffoldGovernedBundle(root, "my-change", model.LevelL2)
	require.NoError(t, err)

	base := filepath.Join(root, "aircraft", "changes", "my-change")
	for _, file := range []string{
		"change.yaml",
		"proposal.md",
		"spec.md",
		"design.md",
		"tasks.md",
		"assurance.md",
	} {
		_, err := os.Stat(filepath.Join(base, file))
		require.NoError(t, err, file)
	}

	_, err = os.Stat(filepath.Join(base, "explore.md"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestScaffoldGovernedBundleL3AddsExplore(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, ScaffoldGovernedBundle(root, "my-change", model.LevelL3))

	_, err := os.Stat(filepath.Join(root, "aircraft", "changes", "my-change", "explore.md"))
	require.NoError(t, err)
}

func TestScaffoldGovernedBundleL1Noop(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, ScaffoldGovernedBundle(root, "my-change", model.LevelL1))

	_, err := os.Stat(filepath.Join(root, "aircraft", "changes", "my-change"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestTemplateRequiredSections(t *testing.T) {
	assurance, err := TemplateContent("assurance.md")
	require.NoError(t, err)
	assert.Contains(t, assurance, "## Scope Summary")
	assert.Contains(t, assurance, "## Verification Verdict")
	assert.Contains(t, assurance, "## Evidence Index")
	assert.Contains(t, assurance, "## Residual Risks and Exceptions")
	assert.Contains(t, assurance, "## Archive Decision")

	design, err := TemplateContent("design.md")
	require.NoError(t, err)
	assert.Contains(t, design, "## Risk")

	explore, err := TemplateContent("explore.md")
	require.NoError(t, err)
	assert.Contains(t, explore, "## Objectives")
	assert.Contains(t, explore, "## Unknowns")
	assert.Contains(t, explore, "## Assumptions")
	assert.Contains(t, explore, "## Scope Boundaries")
	assert.Contains(t, explore, "## Validation Plan")
}

func TestValidateExploreStructure(t *testing.T) {
	valid := `## Objectives
One

## Unknowns
Two

## Assumptions
Three

## Scope Boundaries
Four

## Validation Plan
Five`

	require.NoError(t, ValidateExploreStructure(valid))

	missing := `## Objectives
One
## Unknowns
Two`
	require.Error(t, ValidateExploreStructure(missing))

	reordered := `## Unknowns
Two
## Objectives
One
## Assumptions
Three
## Scope Boundaries
Four
## Validation Plan
Five`
	require.Error(t, ValidateExploreStructure(reordered))
}

func TestValidateAssuranceStructure(t *testing.T) {
	valid := `## Scope Summary
One

## Verification Verdict
Two

## Evidence Index
Three

## Residual Risks and Exceptions
Four

## Archive Decision
Five`
	require.NoError(t, ValidateAssuranceStructure(valid))

	invalid := `## Scope Summary
One

## Verification Verdict
Two`
	require.Error(t, ValidateAssuranceStructure(invalid))
}

func TestStalePropagationOrderBFS(t *testing.T) {
	order, err := StalePropagationOrder("proposal.md")
	require.NoError(t, err)
	require.NotEmpty(t, order)
	assert.Equal(t, "proposal.md", order[0])
	assert.Contains(t, order, "spec.md")
	assert.Contains(t, order, "design.md")
	assert.Contains(t, order, "tasks.md")
	assert.Contains(t, order, "assurance.md")
}

func TestArchiveBundleMovesToArchived(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "aircraft", "changes", "my-change")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "change.yaml"), []byte("x"), 0o644))

	require.NoError(t, ArchiveBundle(root, "my-change"))

	_, err := os.Stat(src)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(root, "aircraft", "changes", "archived", "my-change", "change.yaml"))
	require.NoError(t, err)
}
