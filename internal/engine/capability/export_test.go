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
		if !strings.Contains(manifest, "`"+id+"`") {
			t.Errorf("manifest missing skill %q", id)
		}
	}
}

func TestBuildCatalogManifest_GroupsByDomain(t *testing.T) {
	reg := DefaultRegistry()
	manifest := BuildCatalogManifest(reg)
	seen := map[Domain]bool{}
	for _, sk := range reg.All() {
		seen[sk.Domain] = true
	}
	for domain := range seen {
		header := "## Domain: " + string(domain)
		if !strings.Contains(manifest, header) {
			t.Errorf("manifest missing domain header %q", header)
		}
	}
}

func TestBuildCatalogManifest_ListsBindings(t *testing.T) {
	reg := DefaultRegistry()
	manifest := BuildCatalogManifest(reg)
	for _, sk := range reg.All() {
		for _, binding := range sk.Bindings {
			fragment := "`" + string(binding.Type) + "` -> `" + binding.Target + "`"
			if !strings.Contains(manifest, fragment) {
				t.Errorf("skill %s: manifest missing binding fragment %q", sk.ID, fragment)
			}
		}
	}
}
