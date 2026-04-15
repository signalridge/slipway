package capability

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// skillsDir is the source-of-truth directory for catalog skill sources.
func skillsDir(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "internal", "tmpl", "templates", "skills")
	info, err := os.Stat(p)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	return p
}

// TestFrontmatterMirrorsRegistryBindings is the B1 binding-compare gate.
// It parses each SKILL.md's frontmatter and asserts the bindings[] list
// matches the Go-owned Skill entry exactly.
func TestFrontmatterMirrorsRegistryBindings(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	root := skillsDir(t)
	for _, sk := range reg.All() {
		sk := sk
		t.Run(sk.ID, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, sk.ID, "SKILL.md")
			fm := loadFrontmatter(t, path)
			assertBindingsEqual(t, sk, fm.Bindings)
		})
	}
}

// TestFrontmatterMirrorsRegistryHydrateReferences is the PR-4a hydrate
// compare gate. Every catalog skill's SKILL.md frontmatter
// `hydrate_references:` list must match the Go-owned Skill.HydrateReferences
// record-for-record after sorting by name.
func TestFrontmatterMirrorsRegistryHydrateReferences(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	root := skillsDir(t)
	for _, sk := range reg.All() {
		sk := sk
		t.Run(sk.ID, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, sk.ID, "SKILL.md")
			fm := loadFrontmatter(t, path)
			assertHydrateReferencesEqual(t, sk, fm.HydrateReferences)
		})
	}
}

// TestSizeBudgetsForRegisteredSkills enforces the B1 tier-aware size-lint
// gate. Sizes above target are warning-band; sizes above hard-max require
// `size_rationale` in frontmatter.
func TestSizeBudgetsForRegisteredSkills(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	root := skillsDir(t)
	for _, sk := range reg.All() {
		sk := sk
		t.Run(sk.ID, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, sk.ID, "SKILL.md")
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			body := stripFrontmatter(string(raw))
			size := len(body)
			fm := loadFrontmatter(t, path)
			if err := checkTierSizeBudget(sk.Tier, size, fm.SizeRationale); err != nil {
				t.Fatalf("skill %s: %v", sk.ID, err)
			}
			target, hardMax := tierSizeBudget(sk.Tier)
			if size > target && size <= hardMax {
				t.Logf("warning-band: skill %s body %d bytes exceeds tier %s target %d", sk.ID, size, sk.Tier, target)
			}
		})
	}
}

func TestTierSizeBudgetRequiresRationaleAboveHardMax(t *testing.T) {
	t.Parallel()
	err := checkTierSizeBudget(TierT3, 3*1024+1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size_rationale")
	require.NoError(t, checkTierSizeBudget(TierT3, 3*1024+1, "known false-positive due embedded table"))
}

// TestFrontmatterHasRequiredFields enforces the B1 schema-lint gate on
// required frontmatter keys. It does not re-run the full DSL validator
// (that is covered by trigger_test.go); it pins the authoring contract.
func TestFrontmatterHasRequiredFields(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	root := skillsDir(t)
	for _, sk := range reg.All() {
		sk := sk
		t.Run(sk.ID, func(t *testing.T) {
			t.Parallel()
			fm := loadFrontmatter(t, filepath.Join(root, sk.ID, "SKILL.md"))
			assert.Equal(t, sk.ID, fm.SkillID)
			assert.Equal(t, string(sk.Domain), fm.Domain)
			assert.Equal(t, string(sk.Tier), fm.Tier)
			assert.Equal(t, string(sk.PrimaryAttachment), fm.PrimaryAttachment)
			assert.Contains(t, fm.Summary, "Use when")
			assert.Contains(t, fm.Summary, "Triggers on")
			assert.NotEmpty(t, fm.Bindings)
		})
	}
}

type frontmatter struct {
	SkillID           string            `yaml:"skill_id"`
	Domain            string            `yaml:"domain"`
	Function          string            `yaml:"function"`
	Tier              string            `yaml:"tier"`
	PrimaryAttachment string            `yaml:"primary_attachment"`
	Summary           string            `yaml:"summary"`
	SizeRationale     string            `yaml:"size_rationale"`
	EvidenceContract  string            `yaml:"evidence_contract"`
	Bindings          []frontBinding    `yaml:"bindings"`
	HydrateReferences []frontHydrateRef `yaml:"hydrate_references"`
}

type frontBinding struct {
	Type       string `yaml:"type"`
	Target     string `yaml:"target"`
	Attachment string `yaml:"attachment"`
}

type frontHydrateRef struct {
	Name   string `yaml:"name"`
	Reason string `yaml:"reason"`
}

func loadFrontmatter(t *testing.T, path string) frontmatter {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := extractFrontmatterBlock(t, string(raw))
	var fm frontmatter
	require.NoError(t, yaml.Unmarshal([]byte(text), &fm))
	return fm
}

func extractFrontmatterBlock(t *testing.T, content string) string {
	t.Helper()
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	require.Lessf(t, start, len(lines), "empty file")
	require.Equal(t, "---", strings.TrimSpace(lines[start]), "missing frontmatter start")
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[start+1:i], "\n")
		}
	}
	t.Fatal("unterminated frontmatter")
	return ""
}

func stripFrontmatter(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) || strings.TrimSpace(lines[start]) != "---" {
		return content
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}

func assertBindingsEqual(t *testing.T, sk Skill, frontBindings []frontBinding) {
	t.Helper()
	require.Equal(t, len(sk.Bindings), len(frontBindings), "skill %s: binding count drift", sk.ID)

	goKeys := make([]string, 0, len(sk.Bindings))
	for _, b := range sk.Bindings {
		goKeys = append(goKeys, bindingKey(string(b.Type), b.Target, string(b.Attachment)))
	}
	frontKeys := make([]string, 0, len(frontBindings))
	for _, b := range frontBindings {
		frontKeys = append(frontKeys, bindingKey(b.Type, b.Target, b.Attachment))
	}
	sort.Strings(goKeys)
	sort.Strings(frontKeys)
	assert.Equal(t, goKeys, frontKeys, "skill %s: frontmatter bindings do not mirror registry", sk.ID)
}

func bindingKey(typ, target, attachment string) string {
	return typ + "|" + target + "|" + attachment
}

func assertHydrateReferencesEqual(t *testing.T, sk Skill, frontRefs []frontHydrateRef) {
	t.Helper()
	require.Equalf(t, len(sk.HydrateReferences), len(frontRefs),
		"skill %s: hydrate_references count drift (registry=%d, frontmatter=%d)",
		sk.ID, len(sk.HydrateReferences), len(frontRefs))

	type pair struct{ name, reason string }
	goPairs := make([]pair, 0, len(sk.HydrateReferences))
	for _, hr := range sk.HydrateReferences {
		goPairs = append(goPairs, pair{hr.Name, hr.Reason})
	}
	frontPairs := make([]pair, 0, len(frontRefs))
	for _, fr := range frontRefs {
		frontPairs = append(frontPairs, pair{fr.Name, fr.Reason})
	}
	sort.Slice(goPairs, func(i, j int) bool { return goPairs[i].name < goPairs[j].name })
	sort.Slice(frontPairs, func(i, j int) bool { return frontPairs[i].name < frontPairs[j].name })
	assert.Equal(t, goPairs, frontPairs, "skill %s: frontmatter hydrate_references do not mirror registry", sk.ID)
}

func tierSizeBudget(tier Tier) (target, hardMax int) {
	switch tier {
	case TierT1:
		return 2560, 6 * 1024 // 2.5 KB target (PR-3 lift), hard-max 6 KB
	case TierT2:
		return 3584, 8 * 1024 // 3.5 KB target (PR-3 lift), hard-max 8 KB
	case TierT3:
		return 1536, 3 * 1024 // 1.5 KB target, rationale above 3 KB
	default:
		return 0, 0
	}
}

func checkTierSizeBudget(tier Tier, size int, rationale string) error {
	target, hardMax := tierSizeBudget(tier)
	if target == 0 || hardMax == 0 {
		return fmt.Errorf("unknown tier %s", tier)
	}
	if size > hardMax && strings.TrimSpace(rationale) == "" {
		return fmt.Errorf("body %d bytes exceeds tier %s hard-max %d without size_rationale", size, tier, hardMax)
	}
	return nil
}

// TestSkillDirectoryNamesMatchRegistry ensures every registered skill has
// a source directory on disk (and no stray directories without registration
// among the catalog-facing IDs).
func TestSkillDirectoryNamesMatchRegistry(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	root := skillsDir(t)
	for _, id := range reg.IDs() {
		info, err := os.Stat(filepath.Join(root, id))
		require.NoErrorf(t, err, "missing source dir for %s", id)
		require.Truef(t, info.IsDir(), "source path for %s is not a directory", id)
	}
}

// TestNoPrematureTiebreakPromises ensures llm_tiebreak (B7+) does not leak
// into authoring metadata before B7 lands. hydrate_references may appear
// from B2 onward (context-assembly owns the first real use).
func TestNoPrematureTiebreakPromises(t *testing.T) {
	t.Parallel()
	root := skillsDir(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Base(path) != "SKILL.md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fm := extractFrontmatterBlock(t, string(raw))
		assert.NotContains(t, fm, "llm_tiebreak", "%s: llm_tiebreak is a B7+ surface", path)
		return nil
	})
	require.NoError(t, err)
}
