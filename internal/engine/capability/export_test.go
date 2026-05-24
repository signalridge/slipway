package capability

import (
	"strings"
	"testing"
)

func TestBuildCatalogManifest_NilRegistry(t *testing.T) {
	if got := BuildCatalogManifest(nil); got != "" {
		t.Fatalf("expected empty manifest for nil registry, got %q", got)
	}
}

func TestBuildCatalogManifest_Deterministic(t *testing.T) {
	reg := DefaultRegistry()
	a := BuildCatalogManifest(reg)
	b := BuildCatalogManifest(reg)
	if a != b {
		t.Fatal("manifest is non-deterministic across calls")
	}
}

func TestBuildCatalogManifest_IncludesEverySkill(t *testing.T) {
	reg := DefaultRegistry()
	manifest := BuildCatalogManifest(reg)
	for _, id := range reg.IDs() {
		publicName := adapterSkillPublicName(id)
		if !strings.Contains(manifest, "`"+publicName+"`") {
			t.Errorf("manifest missing public skill %q", publicName)
		}
	}
}

func TestBuildCatalogManifest_RendersShortDispatcherIndex(t *testing.T) {
	reg := DefaultRegistry()
	manifest := BuildCatalogManifest(reg)
	if len([]byte(manifest)) > 12000 {
		t.Fatalf("catalog manifest too large: got %d bytes, want <= 12000", len([]byte(manifest)))
	}
	if !strings.Contains(manifest, "# Slipway Catalog") {
		t.Fatal("manifest missing short catalog header")
	}
	if !strings.Contains(manifest, "Use only when no governed host already owns the step.") {
		t.Fatal("manifest missing usage boundary")
	}
	if !strings.Contains(manifest, "| Skill | Catalog artifact | Tier | Bindings | Evidence | Hydrate refs | Use when |") {
		t.Fatal("manifest missing dispatcher table")
	}
	if strings.Contains(manifest, "## Domain:") {
		t.Fatal("manifest should not expand domain sections")
	}
	if !strings.Contains(manifest, "references/catalog/") {
		t.Fatal("manifest missing catalog artifact paths")
	}
}

func TestBuildCatalogManifest_UsesCanonicalPublicSkillLabels(t *testing.T) {
	reg := DefaultRegistry()
	manifest := BuildCatalogManifest(reg)
	for _, sk := range reg.All() {
		publicName := adapterSkillPublicName(sk.ID)
		if !strings.Contains(manifest, "| `"+publicName+"` |") {
			t.Errorf("dispatcher index missing public skill label %q", publicName)
		}
	}
}
