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

func TestBuildCatalogManifest_UsesCanonicalPublicSkillLabels(t *testing.T) {
	reg := DefaultRegistry()
	manifest := BuildCatalogManifest(reg)
	for _, sk := range reg.All() {
		publicName := adapterSkillPublicName(sk.ID)
		if !strings.Contains(manifest, "| `"+publicName+"` |") {
			t.Errorf("triage index missing public skill label %q", publicName)
		}
		if !strings.Contains(manifest, "### `"+publicName+"` ("+string(sk.Tier)+")") {
			t.Errorf("skill section missing public skill label %q", publicName)
		}
	}
}

func TestBuildCatalogManifest_LeavesBindingTargetsOnBareInternalIDs(t *testing.T) {
	reg := DefaultRegistry()
	manifest := BuildCatalogManifest(reg)
	for _, sk := range reg.All() {
		for _, binding := range sk.Bindings {
			bareFragment := "`" + string(binding.Type) + "` -> `" + binding.Target + "`"
			if !strings.Contains(manifest, bareFragment) {
				t.Errorf("skill %s: manifest missing bare binding fragment %q", sk.ID, bareFragment)
			}
			prefixedFragment := "`" + string(binding.Type) + "` -> `" + adapterSkillPublicName(binding.Target) + "`"
			if strings.Contains(manifest, prefixedFragment) {
				t.Errorf("skill %s: manifest should keep runtime binding target bare, found %q", sk.ID, prefixedFragment)
			}
		}
	}
}
