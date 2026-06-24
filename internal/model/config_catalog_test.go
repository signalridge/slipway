package model

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// strictConfigLeafKeys walks the Config struct the same way strict decoding
// (KnownFields(true)) sees it: every nested struct field that carries a yaml
// tag contributes a dotted leaf key. Fields tagged `yaml:"-"` (engine-internal,
// e.g. UnknownTopLevel) are not user-facing config keys and are skipped. Slice
// and map fields are treated as leaves (they are configured as a whole, not by
// dotted sub-path) so the catalog stays a flat key surface.
//
// This mirrors the --list-focuses completeness precedent: the catalog is
// derived from the struct, and the test asserts the struct and catalog cannot
// drift apart.
func strictConfigLeafKeys(t *testing.T) []string {
	t.Helper()
	var keys []string
	var walk func(rt reflect.Type, prefix string)
	walk = func(rt reflect.Type, prefix string) {
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			tag := field.Tag.Get("yaml")
			if tag == "-" || tag == "" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" {
				continue
			}
			dotted := name
			if prefix != "" {
				dotted = prefix + "." + name
			}
			ft := field.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			// Recurse into nested structs to produce dotted leaf keys; slices/maps
			// and scalars are leaves.
			if ft.Kind() == reflect.Struct {
				walk(ft, dotted)
				continue
			}
			keys = append(keys, dotted)
		}
	}
	walk(reflect.TypeOf(Config{}), "")
	sort.Strings(keys)
	return keys
}

// TestConfigCatalogCoversEveryStructLeaf is the drift guard: every strict-decoded
// Config leaf field must have a catalog entry. Adding a struct field without a
// catalog entry FAILS here and names the missing field.
func TestConfigCatalogCoversEveryStructLeaf(t *testing.T) {
	catalog := ConfigCatalog()
	have := map[string]bool{}
	for _, entry := range catalog {
		if have[entry.Name] {
			t.Errorf("duplicate catalog entry for key %q", entry.Name)
		}
		have[entry.Name] = true
	}

	for _, key := range strictConfigLeafKeys(t) {
		if !have[key] {
			t.Errorf("config struct leaf %q has no ConfigCatalog() entry; add one to internal/model/config_catalog.go", key)
		}
	}
}

// TestConfigCatalogParityPasses asserts the positive case: the catalog is in
// parity with the struct (no missing keys), and every catalog entry maps to a
// real struct leaf (no stale/extra entries).
func TestConfigCatalogParityPasses(t *testing.T) {
	leaves := map[string]bool{}
	for _, key := range strictConfigLeafKeys(t) {
		leaves[key] = true
	}
	for _, entry := range ConfigCatalog() {
		if !leaves[entry.Name] {
			t.Errorf("catalog entry %q does not correspond to any Config struct leaf (stale entry?)", entry.Name)
		}
	}
}

// TestConfigCatalogEntriesAreWellFormed asserts each entry carries the required
// metadata: a name, a type, and a scope; default values are sourced from
// DefaultConfig().
func TestConfigCatalogEntriesAreWellFormed(t *testing.T) {
	for _, entry := range ConfigCatalog() {
		if strings.TrimSpace(entry.Name) == "" {
			t.Errorf("catalog entry has empty name: %+v", entry)
		}
		if strings.TrimSpace(entry.Type) == "" {
			t.Errorf("catalog entry %q has empty type", entry.Name)
		}
		if strings.TrimSpace(entry.Scope) == "" {
			t.Errorf("catalog entry %q has empty scope", entry.Name)
		}
	}
}

// TestConfigCatalogDefaultsMatchDefaultConfig spot-checks that defaults are
// sourced from DefaultConfig() rather than hand-typed.
func TestConfigCatalogDefaultsMatchDefaultConfig(t *testing.T) {
	byName := map[string]ConfigCatalogEntry{}
	for _, entry := range ConfigCatalog() {
		byName[entry.Name] = entry
	}
	if got := byName["execution.lock_wait_timeout_seconds"].Default; got != "10" {
		t.Errorf("execution.lock_wait_timeout_seconds default = %q, want %q", got, "10")
	}
	if got := byName["execution.max_plan_audit_iterations"].Default; got != "3" {
		t.Errorf("execution.max_plan_audit_iterations default = %q, want %q", got, "3")
	}
}

// TestConfigCatalogAllowedValuesEnriched verifies constrained keys carry their
// allowed-values set.
func TestConfigCatalogAllowedValuesEnriched(t *testing.T) {
	byName := map[string]ConfigCatalogEntry{}
	for _, entry := range ConfigCatalog() {
		byName[entry.Name] = entry
	}
	cases := map[string][]string{
		"defaults.artifact_schema":                              {"core", "expanded", "custom"},
		"execution.parallelization":                             {"forced", "off"},
		"governance.thresholds.independent_review_blast_radius": {"low", "medium", "high"},
	}
	for key, want := range cases {
		entry, ok := byName[key]
		if !ok {
			t.Errorf("expected catalog entry for %q", key)
			continue
		}
		if !reflect.DeepEqual(entry.AllowedValues, want) {
			t.Errorf("%s allowed values = %v, want %v", key, entry.AllowedValues, want)
		}
	}
}

func TestConfigGetValue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Execution.LockWaitTimeoutSeconds = 42
	cfg.Defaults.ArtifactSchema = ArtifactSchemaExpanded

	got, err := ConfigGetValue(cfg, "execution.lock_wait_timeout_seconds")
	if err != nil {
		t.Fatalf("ConfigGetValue returned error: %v", err)
	}
	if got != "42" {
		t.Errorf("execution.lock_wait_timeout_seconds = %q, want %q", got, "42")
	}

	got, err = ConfigGetValue(cfg, "defaults.artifact_schema")
	if err != nil {
		t.Fatalf("ConfigGetValue returned error: %v", err)
	}
	if got != "expanded" {
		t.Errorf("defaults.artifact_schema = %q, want %q", got, "expanded")
	}

	if _, err := ConfigGetValue(cfg, "execution.does_not_exist"); err == nil {
		t.Error("expected error for unknown key, got nil")
	} else if !strings.Contains(err.Error(), "execution.does_not_exist") {
		t.Errorf("unknown-key error should name the key, got: %v", err)
	}
}

func TestConfigSetValueValidApplies(t *testing.T) {
	cfg := DefaultConfig()
	updated, err := ConfigSetValue(cfg, "execution.lock_wait_timeout_seconds", "55")
	if err != nil {
		t.Fatalf("ConfigSetValue returned error: %v", err)
	}
	if updated.Execution.LockWaitTimeoutSeconds != 55 {
		t.Errorf("after set, lock_wait_timeout_seconds = %d, want 55", updated.Execution.LockWaitTimeoutSeconds)
	}

	updated, err = ConfigSetValue(cfg, "defaults.artifact_schema", "core")
	if err != nil {
		t.Fatalf("ConfigSetValue returned error: %v", err)
	}
	if updated.Defaults.ArtifactSchema != ArtifactSchemaCore {
		t.Errorf("after set, artifact_schema = %q, want core", updated.Defaults.ArtifactSchema)
	}
}

func TestConfigSetValueInvalidNoMutation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Execution.LockWaitTimeoutSeconds = 10

	// Invalid enum value must fail via Config.Validate() with a clear error and
	// leave the input untouched.
	_, err := ConfigSetValue(cfg, "defaults.artifact_schema", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid artifact_schema value, got nil")
	}
	if cfg.Defaults.ArtifactSchema != "" {
		t.Errorf("input config was mutated on failure: artifact_schema = %q", cfg.Defaults.ArtifactSchema)
	}

	// Unknown key must fail and name the key.
	if _, err := ConfigSetValue(cfg, "execution.nope", "1"); err == nil {
		t.Error("expected error for unknown key, got nil")
	} else if !strings.Contains(err.Error(), "execution.nope") {
		t.Errorf("unknown-key error should name the key, got: %v", err)
	}

	// Non-integer value for an int field must fail.
	if _, err := ConfigSetValue(cfg, "execution.lock_wait_timeout_seconds", "abc"); err == nil {
		t.Error("expected error for non-integer int value, got nil")
	}
}

// TestConfigCatalogEffectiveDefaults asserts the DEFAULT column reflects the
// resolved effective default, including Normalize()-derived (artifact_schema)
// and accessor-derived (auto_provision_worktree, parallelization) defaults that
// a bare DefaultConfig() walk would render blank/false.
func TestConfigCatalogEffectiveDefaults(t *testing.T) {
	byName := map[string]ConfigCatalogEntry{}
	for _, entry := range ConfigCatalog() {
		byName[entry.Name] = entry
	}
	cases := map[string]string{
		"defaults.artifact_schema":           "expanded",
		"governance.auto_provision_worktree": "true",
		"execution.parallelization":          "forced",
	}
	for key, want := range cases {
		if got := byName[key].Default; got != want {
			t.Errorf("catalog default for %q = %q, want %q", key, got, want)
		}
	}
}

// TestConfigGetValueEffectiveDefaults asserts `config get` reports the resolved
// effective value for accessor-defaulted keys when unset, while an explicitly
// set value (including false) renders as-is.
func TestConfigGetValueEffectiveDefaults(t *testing.T) {
	unset := DefaultConfig()
	for key, want := range map[string]string{
		"governance.auto_provision_worktree": "true",
		"execution.parallelization":          "forced",
	} {
		got, err := ConfigGetValue(unset, key)
		if err != nil {
			t.Fatalf("ConfigGetValue(%q) error: %v", key, err)
		}
		if got != want {
			t.Errorf("unset %q = %q, want effective default %q", key, got, want)
		}
	}

	// An explicit false must NOT be masked by the nil => enabled default.
	cfg := DefaultConfig()
	no := false
	cfg.Governance.AutoProvisionWorktree = &no
	if got, err := ConfigGetValue(cfg, "governance.auto_provision_worktree"); err != nil || got != "false" {
		t.Errorf("explicit false auto_provision_worktree = %q (err %v), want \"false\"", got, err)
	}
	yes := true
	cfg.Governance.AutoProvisionWorktree = &yes
	if got, err := ConfigGetValue(cfg, "governance.auto_provision_worktree"); err != nil || got != "true" {
		t.Errorf("explicit true auto_provision_worktree = %q (err %v), want \"true\"", got, err)
	}
	cfg2 := DefaultConfig()
	cfg2.Execution.Parallelization = ParallelizationOff
	if got, err := ConfigGetValue(cfg2, "execution.parallelization"); err != nil || got != "off" {
		t.Errorf("explicit off parallelization = %q (err %v), want \"off\"", got, err)
	}
}
