package toolgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSurfaceManifestDerivesRowsFromSlipwayAuthorities(t *testing.T) {
	t.Parallel()

	manifest := BuildSurfaceManifest()
	raw, err := EncodeSurfaceManifest(manifest)
	require.NoError(t, err)

	var decoded SurfaceManifest
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, manifest, decoded)

	assert.Equal(t, 1, manifest.Version)
	assert.NotEmpty(t, manifest.Rows)
	assert.True(t, slices.IsSortedFunc(manifest.Rows, compareSurfaceManifestRows),
		"manifest rows must be sorted for deterministic JSON")

	rowsByKey := map[string]SurfaceManifestRow{}
	for _, row := range manifest.Rows {
		require.NotEmpty(t, row.Kind)
		require.NotEmpty(t, row.Name)
		require.NotEmpty(t, row.Source)
		require.NotEmpty(t, row.Docs)
		require.NotEmpty(t, row.Token)
		key := row.Kind + "/" + row.Name
		require.NotContains(t, rowsByKey, key, "duplicate manifest row %s", key)
		rowsByKey[key] = row
	}

	for _, def := range commandRegistry {
		row, ok := rowsByKey["command/"+def.ID]
		require.Truef(t, ok, "missing command row for %s", def.ID)
		assert.Equal(t, "internal/toolgen/toolgen.go:commandRegistry", row.Source)
		assert.Equal(t, "docs/reference/commands.md", row.Docs)
		assert.Equal(t, "slipway "+def.ID, row.Token)
	}

	for _, cfg := range Registry() {
		row, ok := rowsByKey["adapter/"+cfg.ID]
		require.Truef(t, ok, "missing adapter row for %s", cfg.ID)
		assert.Equal(t, "internal/toolgen/toolgen.go:toolRegistry", row.Source)
		assert.Equal(t, "docs/reference/ai-tools.md", row.Docs)
		assert.Equal(t, "`"+cfg.ID+"`", row.Token)
	}

	for _, cfg := range Registry() {
		if !cfg.CommandSkillSurface {
			continue
		}
		for _, id := range commandIDs() {
			name := adapterSkillName(id)
			row, ok := rowsByKey["skill/"+name]
			require.Truef(t, ok, "missing %s command skill row for %s", cfg.ID, id)
			assert.Equal(t, "internal/toolgen/toolgen.go:commandRegistry", row.Source)
			assert.Equal(t, "docs/reference/ai-tools.md", row.Docs)
			assert.Equal(t, commandSkillDocsToken(id), row.Token)
		}
	}

	for _, id := range governanceSurfaceIDs(func(governanceSurfaceDescriptor) bool { return true }) {
		row, ok := rowsByKey["skill/"+adapterSkillName(id)]
		require.Truef(t, ok, "missing governance skill row for %s", id)
		assert.Equal(t, "internal/toolgen/toolgen.go:governanceSurfaceDescriptors", row.Source)
		assert.Equal(t, "README.md", row.Docs)
		assert.Equal(t, skillDocsToken(adapterSkillName(id)), row.Token)
	}

	for _, id := range append(append([]string{}, standaloneNames...), techniqueNames...) {
		if !shouldExportAsHostSkill(id) {
			continue
		}
		row, ok := rowsByKey["skill/"+adapterSkillName(id)]
		require.Truef(t, ok, "missing standalone or technique skill row for %s", id)
		assert.Equal(t, "internal/toolgen/toolgen.go:hostSkillExportAllowlist", row.Source)
		assert.Equal(t, "README.md", row.Docs)
		assert.Equal(t, skillDocsToken(adapterSkillName(id)), row.Token)
	}

	for _, id := range catalogSkillIDs {
		if !shouldExportAsHostSkill(id) {
			continue
		}
		if isGovernanceSurfaceID(id) {
			continue
		}
		row, ok := rowsByKey["skill/"+adapterSkillName(id)]
		require.Truef(t, ok, "missing exported catalog skill row for %s", id)
		assert.Equal(t, "internal/engine/capability.DefaultRegistry", row.Source)
	}

	for _, def := range commandRegistry {
		if !strings.Contains(def.Arguments, "--json") {
			continue
		}
		if def.ID == "evidence" {
			for _, id := range []string{"evidence-task-json", "evidence-skill-json", "evidence-skill-refresh-current-json"} {
				row, ok := rowsByKey["json-contract/"+id]
				require.Truef(t, ok, "missing json contract row for %s", id)
				assert.Equal(t, "cmd/evidence.go", row.Source)
				assert.Equal(t, "docs/reference/commands.md", row.Docs)
				assert.Contains(t, row.Token, "json")
			}
			continue
		}
		row, ok := rowsByKey["json-contract/"+def.ID+"-json"]
		require.Truef(t, ok, "missing json contract row for %s", def.ID)
		assert.Equal(t, commandSourcePath(def.ID), row.Source)
		assert.Equal(t, "docs/reference/commands.md", row.Docs)
		assert.Contains(t, row.Token, "json")
	}
	assert.NotContains(t, rowsByKey, "json-contract/done-json")
	assert.NotContains(t, rowsByKey, "json-contract/validate-json")

	for _, path := range []string{"README.md", "docs/reference/ai-tools.md", "docs/reference/commands.md", "docs/how-to/recover-and-troubleshoot.md"} {
		key := "documentation/" + path
		row, ok := rowsByKey[key]
		require.Truef(t, ok, "missing documentation row for %s", path)
		assert.Equal(t, path, row.Docs)
		assert.Equal(t, path, row.Name)
	}
}

func TestSurfaceManifestExposesEvidenceRefreshCurrentJSONContract(t *testing.T) {
	t.Parallel()

	manifest := BuildSurfaceManifest()
	for _, row := range manifest.Rows {
		if row.Kind != "json-contract" || row.Name != "evidence-skill-refresh-current-json" {
			continue
		}

		assert.Equal(t, "cmd/evidence.go", row.Source)
		assert.Equal(t, "docs/reference/commands.md", row.Docs)
		assert.Equal(t,
			"slipway evidence skill --skill <selected-review-skill> --verdict pass --refresh-current --reference \"context_origin:stage=review=<handle>\" --notes-file artifacts/changes/<slug>/verification/<selected-review-skill>-notes.md --json",
			row.Token)
		return
	}

	t.Fatal("surface manifest must expose the evidence skill refresh-current JSON contract")
}

func TestSurfaceManifestCommandBoundariesUseLifecycleMetadata(t *testing.T) {
	t.Parallel()

	rows := map[string]SurfaceManifestRow{}
	for _, row := range BuildSurfaceManifest().Rows {
		if row.Kind == "command" {
			rows[row.Name] = row
		}
	}

	repair := rows["repair"].CommandBoundary
	require.NotNil(t, repair)
	assert.True(t, repair.StateMutating)
	assert.True(t, repair.ParallelSafe)
	assert.False(t, repair.Exclusive)
	assert.False(t, repair.PreflightRequired)

	fix := rows["fix"].CommandBoundary
	require.NotNil(t, fix)
	assert.True(t, fix.StateMutating)
	assert.False(t, fix.ParallelSafe)
	assert.True(t, fix.Exclusive)
	assert.True(t, fix.PreflightRequired)
	require.NotEmpty(t, fix.Modes)
	assert.Equal(t, "--start-reexecution", fix.Modes[0].Name)
	assert.True(t, fix.Modes[0].Destructive)
}

func TestCommittedSurfaceManifestMatchesBuilder(t *testing.T) {
	t.Parallel()

	live, err := EncodeSurfaceManifest(BuildSurfaceManifest())
	require.NoError(t, err)

	committedPath := filepath.Join(toolgenRepoRoot(t), SurfaceManifestPath)
	committed := readSurfaceManifestFixture(t, committedPath)
	if string(live) != committed {
		t.Fatalf("%s is stale; run `go run ./internal/toolgen/cmd/gen-surface-manifest --write`.\n--- diff sample ---\n%s",
			SurfaceManifestPath,
			firstNDiffLines(committed, string(live), 20))
	}
}

func TestSurfaceManifestDocsTokensExist(t *testing.T) {
	t.Parallel()

	repoRoot := toolgenRepoRoot(t)
	docsCache := map[string]string{}

	for _, row := range BuildSurfaceManifest().Rows {
		content, ok := docsCache[row.Docs]
		if !ok {
			content = readSurfaceManifestFixture(t, filepath.Join(repoRoot, row.Docs))
			docsCache[row.Docs] = content
		}
		assert.Containsf(t, content, row.Token,
			"%s/%s expects token %q in %s", row.Kind, row.Name, row.Token, row.Docs)
	}
}

func TestJSONContractTokensExistInDetailedCommandDocs(t *testing.T) {
	t.Parallel()

	repoRoot := toolgenRepoRoot(t)
	docsPaths := []string{
		"docs/commands.md",
		"docs/ja/commands.md",
		"docs/zh/commands.md",
	}
	docsContent := map[string]string{}
	for _, docsPath := range docsPaths {
		docsContent[docsPath] = readSurfaceManifestFixture(t, filepath.Join(repoRoot, docsPath))
	}

	for _, row := range BuildSurfaceManifest().Rows {
		if row.Kind != "json-contract" {
			continue
		}
		for _, docsPath := range docsPaths {
			assert.Containsf(t, docsContent[docsPath], row.Token,
				"%s must include JSON token %q for %s/%s", docsPath, row.Token, row.Kind, row.Name)
		}
	}
}

func TestLocalizedReferenceDocsCarryRecoveryEvidenceHighlights(t *testing.T) {
	t.Parallel()

	repoRoot := toolgenRepoRoot(t)
	requiredReferenceTokens := []string{
		"slipway fix --start-reexecution",
		"--discard-prior-evidence",
		"slipway run",
		"slipway evidence task --task-id",
		"slipway validate",
		"wave_plan",
	}
	for _, docsPath := range []string{
		"docs/reference/commands.md",
		"docs/ja/reference/commands.md",
		"docs/zh/reference/commands.md",
	} {
		content := readSurfaceManifestFixture(t, filepath.Join(repoRoot, docsPath))
		for _, token := range requiredReferenceTokens {
			assert.Containsf(t, content, token, "%s must carry recovery/evidence highlight token %q", docsPath, token)
		}
	}

	for _, docsPath := range []string{
		"docs/commands.md",
		"docs/ja/commands.md",
		"docs/zh/commands.md",
	} {
		content := readSurfaceManifestFixture(t, filepath.Join(repoRoot, docsPath))
		assert.Containsf(t, content, "--discard-prior-evidence", "%s must document --discard-prior-evidence", docsPath)
	}
}

func TestDesignDocsNameEveryRegisteredAdapter(t *testing.T) {
	t.Parallel()

	repoRoot := toolgenRepoRoot(t)
	docsPaths := []string{
		"docs/design.md",
		"docs/ja/design.md",
		"docs/zh/design.md",
		"docs/explanation/design.md",
		"docs/ja/explanation/design.md",
		"docs/zh/explanation/design.md",
		"docs/assets/diagrams/tool-adapters.svg",
	}
	for _, docsPath := range docsPaths {
		content := readSurfaceManifestFixture(t, filepath.Join(repoRoot, docsPath))
		for _, cfg := range Registry() {
			name := adapterDocsDisplayName(cfg.ID)
			assert.Containsf(t, content, name, "%s must name adapter %s (id %s)", docsPath, name, cfg.ID)
		}
	}
}

func adapterDocsDisplayName(id string) string {
	switch strings.TrimSpace(id) {
	case "opencode":
		return "OpenCode"
	}
	return adapterDocsTitleCase(id)
}

func adapterDocsTitleCase(id string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(id), func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func readSurfaceManifestFixture(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}
