package capability

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func buildDefaultSkillIndex(reg *Registry) string {
	return BuildSkillIndexWithPaths(reg, func(id string) string {
		return filepath.Join("slipway-"+strings.TrimSpace(id), "SKILL.md")
	})
}

func TestBuildSkillIndex_NilRegistry(t *testing.T) {
	if got := buildDefaultSkillIndex(nil); got != "" {
		t.Fatalf("expected empty index for nil registry, got %q", got)
	}
}

func TestBuildSkillIndex_Deterministic(t *testing.T) {
	reg := DefaultRegistry()
	a := buildDefaultSkillIndex(reg)
	b := buildDefaultSkillIndex(reg)
	if a != b {
		t.Fatal("skill index is non-deterministic across calls")
	}
}

func TestBuildSkillIndex_IncludesEverySkillInRegistry(t *testing.T) {
	reg := DefaultRegistry()
	index := buildDefaultSkillIndex(reg)
	for _, id := range reg.IDs() {
		publicName := adapterSkillPublicName(id)
		if !strings.Contains(index, "`"+publicName+"`") {
			t.Errorf("skill index missing public skill %q", publicName)
		}
	}
}

func TestBuildSkillIndex_RendersShortInformationalIndex(t *testing.T) {
	reg := DefaultRegistry()
	index := buildDefaultSkillIndex(reg)
	if len([]byte(index)) > 12000 {
		t.Fatalf("skill index too large: got %d bytes, want <= 12000", len([]byte(index)))
	}
	if !strings.Contains(index, "# Slipway Skill Index") {
		t.Fatal("skill index missing header")
	}
	if !strings.Contains(index, "Informational index only.") {
		t.Fatal("skill index missing usage boundary")
	}
	if !strings.Contains(index, "| Skill | Host skill path | Tier | Bindings | Evidence | Hydrate refs | Use when |") {
		t.Fatal("skill index missing table")
	}
	if strings.Contains(index, "## Domain:") {
		t.Fatal("skill index should not expand domain sections")
	}
	if strings.Contains(index, "references/catalog/") {
		t.Fatal("skill index must not expose catalog artifact paths")
	}
	if !strings.Contains(index, "slipway-security-review/SKILL.md") {
		t.Fatal("skill index missing direct host skill path")
	}
	if !strings.Contains(index, "## Public Focus Aliases") {
		t.Fatal("skill index missing public focus alias section")
	}
}

func TestBuildSkillIndex_UsesCanonicalPublicSkillLabels(t *testing.T) {
	reg := DefaultRegistry()
	index := buildDefaultSkillIndex(reg)
	for _, sk := range reg.All() {
		publicName := adapterSkillPublicName(sk.ID)
		if !strings.Contains(index, "| `"+publicName+"` |") {
			t.Errorf("dispatcher index missing public skill label %q", publicName)
		}
	}
}

func TestBuildSkillIndex_RendersPublicFocusAliasesWithoutHostPaths(t *testing.T) {
	defaultReg := DefaultRegistry()
	var exportedLike []Skill
	for _, id := range []string{"incident-response", "independent-review", "root-cause-tracing", "security-review"} {
		sk, ok := defaultReg.Lookup(id)
		if !ok {
			t.Fatalf("default registry missing %q", id)
		}
		exportedLike = append(exportedLike, sk)
	}
	reg, err := NewRegistry(exportedLike...)
	if err != nil {
		t.Fatal(err)
	}

	index := buildDefaultSkillIndex(reg)
	for _, rec := range ExplicitFocusSurfaces() {
		selector := fmt.Sprintf("`slipway %s --focus %s`", rec.Command, rec.PublicName)
		if !strings.Contains(index, selector) {
			t.Errorf("skill index missing public focus selector %s", selector)
		}
		if !strings.Contains(index, "`"+rec.BackingID+"`") {
			t.Errorf("skill index missing backing skill %q", rec.BackingID)
		}
	}
	if strings.Contains(index, "slipway-sast-orchestration/SKILL.md") {
		t.Fatal("skill index must not expose non-exported focus backing as a host path")
	}
}
