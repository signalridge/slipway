package toolgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallProfileClosurePreservesFailClosedSkills(t *testing.T) {
	core, err := installProfileClosure(SkillInstallProfileCore)
	require.NoError(t, err)
	full, err := installProfileClosure(SkillInstallProfileFull)
	require.NoError(t, err)

	assert.Less(t, len(core.Skills), len(full.Skills),
		"core profile should reduce eager skill listing compared with full")

	for _, id := range governanceSurfaceIDs(func(governanceSurfaceDescriptor) bool { return true }) {
		assert.Truef(t, core.includesHostSkill(id), "%s must be installed in core", id)
		assert.Truef(t, full.includesHostSkill(id), "%s must be installed in full", id)
	}
	for _, id := range []string{
		"plan-audit",
		"security-review",
		"goal-verification",
		"final-closeout",
		"spec-compliance-review",
		"code-quality-review",
		"independent-review",
	} {
		assert.Truef(t, core.includesHostSkill(id), "%s must be fail-closed in core", id)
	}
	for _, id := range alwaysInstalledCommandIDs {
		assert.Truef(t, core.includesCommandSkill(id), "%s command surface must be installed in core", id)
		assert.Truef(t, full.includesCommandSkill(id), "%s command surface must be installed in full", id)
	}

	assert.False(t, core.includesCommandSkill("health"), "diagnostic command skills should route through core routers")
	assert.False(t, core.includesCommandSkill("learn"), "diagnostic command skills should route through core routers")
	assert.False(t, core.includesHostSkill("incident-response"), "optional diagnostic hosts should be full-profile only")
}

func TestCoreInstallProfileGeneratesRoutersAndPrunesOptionalCodexSkills(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, GenerateWithInstallProfile(root, []string{"codex"}, true, SkillInstallProfileCore))

	cfg := toolRegistry["codex"]
	for _, id := range []string{
		"workflow",
		"review",
		"evidence",
		"plan-audit",
		"security-review",
		"goal-verification",
		"final-closeout",
		"spec-trace",
		"coverage-analysis",
	} {
		_, err := os.Stat(filepath.Join(root, SkillPath(cfg, id)))
		assert.NoErrorf(t, err, "core profile missing required skill %s", id)
	}
	for _, id := range []string{
		"health",
		"learn",
		"cancel",
		"incident-response",
	} {
		_, err := os.Stat(filepath.Join(root, SkillPath(cfg, id)))
		assert.Truef(t, os.IsNotExist(err), "core profile should prune optional skill %s", id)
	}
	for _, id := range []string{
		"surface-recovery",
		"surface-diagnostics",
		"surface-review-quality",
	} {
		_, err := os.Stat(filepath.Join(root, SkillPath(cfg, id)))
		assert.NoErrorf(t, err, "core profile missing namespace router %s", id)
	}

	router, err := os.ReadFile(filepath.Join(root, SkillPath(cfg, "surface-diagnostics")))
	require.NoError(t, err)
	routerText := string(router)
	assert.Contains(t, routerText, "This is a namespace router")
	assert.Contains(t, routerText, "`slipway health`")
	assert.Contains(t, routerText, "`slipway next --json`")
	assert.Contains(t, routerText, "Never use this router as evidence")
	assert.NotContains(t, routerText, "satisfy a gate")

	index, err := os.ReadFile(filepath.Join(root, SkillIndexPath(cfg)))
	require.NoError(t, err)
	assert.Contains(t, string(index), "slipway-security-review/SKILL.md")
	assert.NotContains(t, string(index), "slipway-incident-response/SKILL.md")
}

func TestInstallProfileFrontmatterRecordsMetadata(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, GenerateWithInstallProfile(root, []string{"codex"}, true, SkillInstallProfileCore))

	cfg := toolRegistry["codex"]
	securityReview, err := os.ReadFile(filepath.Join(root, SkillPath(cfg, "security-review")))
	require.NoError(t, err)
	securityText := string(securityReview)
	assert.Contains(t, securityText, "install_profiles:")
	assert.Contains(t, securityText, "always_install: true")
	assert.Contains(t, securityText, "requires:")
	assert.Contains(t, securityText, "  - slipway-review")
	assert.Contains(t, securityText, "  - slipway-validate")

	router, err := os.ReadFile(filepath.Join(root, SkillPath(cfg, "surface-review-quality")))
	require.NoError(t, err)
	assert.Contains(t, string(router), "install_profiles:")
	assert.Contains(t, string(router), "  - core")
	assert.Contains(t, string(router), "requires:")
	assert.Contains(t, string(router), "  - slipway")
}

func TestInstallProfileRejectsUnknownProfile(t *testing.T) {
	_, err := installProfileClosure(SkillInstallProfile("tiny"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported skill install profile")
}
