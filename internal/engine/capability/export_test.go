package capability

import (
	"strings"
	"testing"
)

func TestBuildSkillIndex_NilRegistry(t *testing.T) {
	if got := BuildSkillIndex(nil); got != "" {
		t.Fatalf("expected empty index for nil registry, got %q", got)
	}
}

func TestBuildSkillIndex_Deterministic(t *testing.T) {
	reg := DefaultRegistry()
	a := BuildSkillIndex(reg)
	b := BuildSkillIndex(reg)
	if a != b {
		t.Fatal("skill index is non-deterministic across calls")
	}
}

func TestBuildSkillIndex_IncludesEverySkillInRegistry(t *testing.T) {
	reg := DefaultRegistry()
	index := BuildSkillIndex(reg)
	for _, id := range reg.IDs() {
		publicName := adapterSkillPublicName(id)
		if !strings.Contains(index, "`"+publicName+"`") {
			t.Errorf("skill index missing public skill %q", publicName)
		}
	}
}

func TestBuildSkillIndex_RendersShortInformationalIndex(t *testing.T) {
	reg := DefaultRegistry()
	index := BuildSkillIndex(reg)
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
}

func TestBuildSkillIndex_UsesCanonicalPublicSkillLabels(t *testing.T) {
	reg := DefaultRegistry()
	index := BuildSkillIndex(reg)
	for _, sk := range reg.All() {
		publicName := adapterSkillPublicName(sk.ID)
		if !strings.Contains(index, "| `"+publicName+"` |") {
			t.Errorf("dispatcher index missing public skill label %q", publicName)
		}
	}
}
