package model

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// ErrUnknownConfigKey is the sentinel wrapped by every "unknown config key"
// failure from the catalog resolver. Command surfaces use errors.Is to map an
// unknown key to one stable error code regardless of which path (get or set)
// surfaced it.
var ErrUnknownConfigKey = errors.New("unknown config key")

// configEffectiveDefaults maps keys whose effective default is owned by an
// accessor (a nil pointer or empty-string sentinel resolves to a semantic
// default) rather than stored on the struct, so it is invisible to a normalized
// DefaultConfig() walk. These are the resolved values reported by `config list`'s
// DEFAULT column and by `config get` when the key is unset. Normalize()-derived
// defaults (e.g. defaults.artifact_schema => expanded) are surfaced by
// normalizing the catalog's defaults config and need no entry here.
func configEffectiveDefaults() map[string]string {
	return map[string]string{
		// ConfigGovernance.AutoProvisionWorktreeEnabled(): nil => enabled.
		"governance.auto_provision_worktree": "true",
		// ConfigExecution.ForcedParallel(): "" => forced.
		"execution.parallelization": ParallelizationForced,
	}
}

// ConfigCatalogEntry describes one user-facing `.slipway.yaml` key as a flat,
// dotted leaf. The catalog is the single discoverable source of truth for the
// `slipway config` command surface; it is derived from the Config struct via
// reflection over yaml tags so it cannot drift from what strict decoding
// accepts.
type ConfigCatalogEntry struct {
	// Name is the dotted key, e.g. "execution.auto" or
	// "governance.thresholds.independent_review_blast_radius".
	Name string `json:"name"`
	// Type is the underlying scalar/collection kind: bool, int, string,
	// []string, or map.
	Type string `json:"type"`
	// Default is the rendered default value sourced from DefaultConfig(); false
	// booleans are explicit defaults, while zero ints and empty collections are
	// omitted when they do not carry a meaningful default.
	Default string `json:"default,omitempty"`
	// AllowedValues enumerates the permitted values for constrained keys; nil
	// when the key is free-form.
	AllowedValues []string `json:"allowed_values,omitempty"`
	// Scope is the top-level config section the key belongs to (its first dotted
	// segment), e.g. "execution" or "governance".
	Scope string `json:"scope"`
	// Description is a short human-facing note; empty when none is useful.
	Description string `json:"description,omitempty"`
}

// configAllowedValues maps a dotted key to its constrained value set. Kept as a
// per-key enrichment table (not reflection-derivable, since allowed values live
// in IsValid()/Validate() switches) so the catalog can surface them.
func configAllowedValues() map[string][]string {
	return map[string][]string{
		"defaults.artifact_schema":  {"core", "expanded", "custom"},
		"execution.parallelization": {ParallelizationForced, ParallelizationOff},
		"governance.default_preset": {
			string(WorkflowPresetLight), string(WorkflowPresetStandard), string(WorkflowPresetStrict),
		},
		"governance.min_preset": {
			string(WorkflowPresetLight), string(WorkflowPresetStandard), string(WorkflowPresetStrict),
		},
		"governance.thresholds.independent_review_blast_radius": {
			string(SignalLevelLow), string(SignalLevelMedium), string(SignalLevelHigh),
		},
		"governance.thresholds.security_review_blast_radius": {
			string(SignalLevelLow), string(SignalLevelMedium), string(SignalLevelHigh),
		},
		"governance.thresholds.worktree_blast_radius": {
			string(SignalLevelLow), string(SignalLevelMedium), string(SignalLevelHigh),
		},
	}
}

// configDescriptions maps a dotted key to a short description where one adds
// value beyond the key name. Absence is fine; descriptions are advisory.
func configDescriptions() map[string]string {
	return map[string]string{
		"defaults.artifact_schema":                              "Artifact schema for governed changes (custom requires custom_artifacts).",
		"execution.lock_wait_timeout_seconds":                   "Seconds to wait for a contended workspace lock before failing.",
		"execution.lock_stale_after_seconds":                    "Seconds after which a held lock is considered stale and reclaimable.",
		"execution.cancel_grace_period_seconds":                 "Grace period before a cancel forcibly tears down in-flight execution.",
		"execution.max_plan_audit_iterations":                   "Maximum plan-audit retry iterations before the gate fails closed.",
		"execution.parallelization":                             "Within-wave parallel dispatch: unset/forced runs concurrently, off opts out.",
		"execution.auto":                                        "Opt into auto-advance execution that auto-advances pass-through stages.",
		"governance.default_preset":                             "Default workflow preset applied to new governed changes.",
		"governance.min_preset":                                 "Minimum workflow preset a change may be downgraded to.",
		"governance.auto_provision_worktree":                    "Whether `slipway new` provisions a dedicated worktree per change (default enabled).",
		"governance.thresholds.independent_review_blast_radius": "Minimum blast radius that triggers the independent-review control.",
		"governance.thresholds.security_review_blast_radius":    "Minimum blast radius that triggers the security-review control.",
		"governance.thresholds.worktree_blast_radius":           "Minimum blast radius that triggers the worktree-isolation control.",
		"validation.enforce_rfc2119":                            "Require RFC-2119 keywords in merged requirements.",
		"validation.enforce_requirement_scenarios":              "Require scenarios for each merged requirement.",
		"context.tech_stack":                                    "Project tech stack injected into skill templates.",
		"context.conventions":                                   "Project conventions injected into skill templates.",
		"context.test_cmd":                                      "Project test command injected into skill templates.",
		"context.build_cmd":                                     "Project build command injected into skill templates.",
		"context.languages":                                     "Project languages injected into skill templates.",
		"context.recent_work":                                   "Recent-work summary injected into skill templates.",
	}
}

// ConfigCatalog returns the full, sorted catalog of user-facing `.slipway.yaml`
// keys. It is derived by walking the Config struct's yaml tags (the same shape
// strict decoding sees), so adding a struct field surfaces here automatically.
// A completeness test asserts every strict-decoded leaf has an entry.
func ConfigCatalog() []ConfigCatalogEntry {
	defaults := DefaultConfig()
	// Normalize so Normalize()-derived effective defaults (e.g. artifact_schema
	// => expanded) appear in the DEFAULT column instead of rendering blank.
	defaults.Normalize()
	defVal := reflect.ValueOf(defaults)
	allowed := configAllowedValues()
	descriptions := configDescriptions()
	effectiveDefaults := configEffectiveDefaults()

	var entries []ConfigCatalogEntry
	var walk func(rt reflect.Type, rv reflect.Value, prefix string)
	walk = func(rt reflect.Type, rv reflect.Value, prefix string) {
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

			fv := reflect.Value{}
			if rv.IsValid() {
				fv = rv.Field(i)
			}

			ft := field.Type
			elemVal := fv
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
				if elemVal.IsValid() && !elemVal.IsNil() {
					elemVal = elemVal.Elem()
				} else {
					elemVal = reflect.Value{}
				}
			}
			if ft.Kind() == reflect.Struct {
				walk(ft, elemVal, dotted)
				continue
			}

			scope := dotted
			if idx := strings.Index(dotted, "."); idx >= 0 {
				scope = dotted[:idx]
			}
			def := renderDefault(fv)
			if override, ok := effectiveDefaults[dotted]; ok {
				def = override
			}
			entries = append(entries, ConfigCatalogEntry{
				Name:          dotted,
				Type:          configTypeName(ft),
				Default:       def,
				AllowedValues: allowed[dotted],
				Scope:         scope,
				Description:   descriptions[dotted],
			})
		}
	}
	walk(reflect.TypeOf(defaults), defVal, "")

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// configTypeName renders a compact type label for a leaf field kind.
func configTypeName(ft reflect.Type) string {
	switch ft.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.String:
		return "string"
	case reflect.Slice:
		return "[]" + configTypeName(ft.Elem())
	case reflect.Map:
		return "map"
	default:
		return ft.Kind().String()
	}
}

// renderDefault renders a leaf's default value as a string. False booleans are
// explicit defaults; zero ints, empty strings, and empty collections render as
// "" so the catalog default column stays compact.
func renderDefault(fv reflect.Value) string {
	if !fv.IsValid() {
		return ""
	}
	switch fv.Kind() {
	case reflect.Bool:
		if fv.Bool() {
			return "true"
		}
		return "false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Int() == 0 {
			return ""
		}
		return strconv.FormatInt(fv.Int(), 10)
	case reflect.String:
		return fv.String()
	case reflect.Slice, reflect.Map:
		return ""
	default:
		return ""
	}
}

// configLeafField resolves the addressable reflect.Value for the leaf at the
// dotted key within cfg, walking nested structs (and dereferencing/allocating
// pointers). It returns an error naming the key when the key does not resolve
// to a settable scalar/collection leaf.
func configLeafField(rv reflect.Value, key string) (reflect.Value, error) {
	segments := strings.Split(key, ".")
	cur := rv
	for depth, seg := range segments {
		if cur.Kind() == reflect.Pointer {
			if cur.IsNil() {
				if !cur.CanSet() {
					return reflect.Value{}, fmt.Errorf("%w %q", ErrUnknownConfigKey, key)
				}
				cur.Set(reflect.New(cur.Type().Elem()))
			}
			cur = cur.Elem()
		}
		if cur.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("%w %q", ErrUnknownConfigKey, key)
		}
		field, ok := structFieldByYAMLName(cur, seg)
		if !ok {
			return reflect.Value{}, fmt.Errorf("%w %q", ErrUnknownConfigKey, key)
		}
		cur = field
		if depth == len(segments)-1 {
			if cur.Kind() == reflect.Pointer {
				if cur.IsNil() {
					if !cur.CanSet() {
						return reflect.Value{}, fmt.Errorf("%w %q", ErrUnknownConfigKey, key)
					}
					cur.Set(reflect.New(cur.Type().Elem()))
				}
				cur = cur.Elem()
			}
			return cur, nil
		}
	}
	return reflect.Value{}, fmt.Errorf("unknown config key %q", key)
}

// structFieldByYAMLName returns the struct field whose yaml tag name matches
// seg. The returned value shares storage with rv (settable when rv is).
func structFieldByYAMLName(rv reflect.Value, seg string) (reflect.Value, bool) {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("yaml")
		if tag == "-" || tag == "" {
			continue
		}
		if strings.Split(tag, ",")[0] == seg {
			return rv.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// configLeafFieldRead resolves the leaf value at key for READING. Unlike
// configLeafField (the settable write path), it never allocates nil pointers:
// when the leaf or an ancestor pointer is nil it reports unset=true and returns
// the zero value of the leaf type, so callers can substitute an effective
// default instead of rendering a misleading zero (e.g. a nil *bool as "false").
func configLeafFieldRead(rv reflect.Value, key string) (leaf reflect.Value, unset bool, err error) {
	segments := strings.Split(key, ".")
	cur := rv
	for depth, seg := range segments {
		if cur.Kind() == reflect.Pointer {
			if cur.IsNil() {
				unset = true
				cur = reflect.Zero(cur.Type().Elem())
			} else {
				cur = cur.Elem()
			}
		}
		if cur.Kind() != reflect.Struct {
			return reflect.Value{}, false, fmt.Errorf("%w %q", ErrUnknownConfigKey, key)
		}
		field, ok := structFieldByYAMLName(cur, seg)
		if !ok {
			return reflect.Value{}, false, fmt.Errorf("%w %q", ErrUnknownConfigKey, key)
		}
		cur = field
		if depth == len(segments)-1 {
			if cur.Kind() == reflect.Pointer {
				if cur.IsNil() {
					return reflect.Zero(cur.Type().Elem()), true, nil
				}
				cur = cur.Elem()
			}
			return cur, unset, nil
		}
	}
	return reflect.Value{}, false, fmt.Errorf("%w %q", ErrUnknownConfigKey, key)
}

// ConfigGetValue resolves the effective string value for a dotted key against a
// loaded Config (the value from `.slipway.yaml` merged over defaults). An
// unknown key returns a clear error naming the key. Keys whose default is owned
// by an accessor (a nil/empty sentinel) report that resolved effective default
// when unset, rather than the bare zero value.
func ConfigGetValue(cfg Config, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("config key is required")
	}
	leaf, unset, err := configLeafFieldRead(reflect.ValueOf(cfg), key)
	if err != nil {
		return "", err
	}
	// Report the accessor-owned effective default when the stored field is at its
	// unset sentinel: a nil pointer (unset), or an empty string for an
	// accessor-defaulted string. An explicitly set value — including false via a
	// non-nil *bool — falls through and renders as-is.
	if def, ok := configEffectiveDefaults()[key]; ok && (unset || (leaf.Kind() == reflect.String && leaf.String() == "")) {
		return def, nil
	}
	return renderEffective(leaf), nil
}

// renderEffective renders a leaf value for display (effective value, including
// zero values, unlike renderDefault which compacts them).
func renderEffective(fv reflect.Value) string {
	switch fv.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(fv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(fv.Int(), 10)
	case reflect.String:
		return fv.String()
	case reflect.Slice:
		parts := make([]string, fv.Len())
		for i := range parts {
			parts[i] = fmt.Sprintf("%v", fv.Index(i).Interface())
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", fv.Interface())
	}
}

// ConfigSetValue parses a string value for a dotted key, applies it to a copy of
// cfg, and validates the result via the same strict contract Config.Validate()
// enforces. On any failure (unknown key, unparseable value, or validation
// rejection) it returns a clear error and the input cfg is left unmodified
// (callers receive a copy; the original is never mutated).
func ConfigSetValue(cfg Config, key, value string) (Config, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return cfg, fmt.Errorf("config key is required")
	}
	// Operate on a copy so a parse/validate failure never mutates the caller's
	// config.
	updated := cfg
	leaf, err := configLeafField(reflect.ValueOf(&updated).Elem(), key)
	if err != nil {
		return cfg, err
	}
	if !leaf.CanSet() {
		return cfg, fmt.Errorf("config key %q is not settable", key)
	}
	if err := assignScalar(leaf, key, value); err != nil {
		return cfg, err
	}
	if err := updated.Validate(); err != nil {
		return cfg, err
	}
	return updated, nil
}

// assignScalar parses value into the leaf according to its kind. Collection
// leaves (slices/maps) are not settable through the scalar `set` path.
func assignScalar(leaf reflect.Value, key, value string) error {
	switch leaf.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("config key %q expects a boolean (true/false), got %q", key, value)
		}
		leaf.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return fmt.Errorf("config key %q expects an integer, got %q", key, value)
		}
		leaf.SetInt(n)
	case reflect.String:
		// Preserve the underlying named string type (e.g. ArtifactSchemaName,
		// SignalLevel) so Validate() sees the right type.
		leaf.SetString(value)
	default:
		return fmt.Errorf("config key %q is not settable via `set` (type %s)", key, leaf.Kind())
	}
	return nil
}
