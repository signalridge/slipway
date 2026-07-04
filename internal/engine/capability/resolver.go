package capability

import (
	"sort"
	"strings"
)

// Attachment is the resolver's output record for a single support skill.
// Host LLMs consume Kind (the attachment mode) to decide where to inject
// the referenced skill, and the resolver populates Reason from the skill's
// Summary field.
type Attachment struct {
	SkillID string
	Kind    AttachmentMode
	Reason  string
}

// RouteSelection names a routed command choice. Mode carries the public
// focus alias (or primary public name). BackingID is the catalog skill id
// that implements the route.
type RouteSelection struct {
	SkillID   string
	BackingID string
	Mode      string
	Reason    string
}

// Resolution is the full resolver output for one invocation. Supports are
// capped at three entries.
type Resolution struct {
	Route             *RouteSelection
	Supports          []Attachment
	HydrateReferences []string
}

// HostCapabilityRequirement describes a host/runtime capability a selected
// skill needs outside the Slipway kernel itself.
type HostCapabilityRequirement struct {
	SkillID             string
	Capability          string
	Required            bool
	Availability        string
	FallbackSelected    bool
	FallbackMode        string
	EvidenceRequirement string
	Remediation         string
}

// Signals carries the context inputs for capability resolution.
type Signals struct {
	Command  string
	Host     string
	Blockers []string
	Paths    []string
	// HostCapabilities names capabilities the current host explicitly reports,
	// e.g. "subagent". The token "none" means the host explicitly reports no
	// delegated-execution capability. Empty means unknown, not unavailable.
	HostCapabilities []string
	// Fallbacks names explicit host/operator-selected fallback modes.
	Fallbacks []string
	// Focus names an explicit `--focus <alias>` selection resolved through
	// surface policy. Empty means no explicit focus was requested.
	Focus string
}

// Resolve scans the registry using Binding metadata (not dynamic trigger DSL)
// and returns:
//   - the routed primary/explicit-focus selection (via surface policy),
//   - up to three supporting attachments from host-embedded / technique-hint
//     bindings,
//   - the union of hydrate references eligible on the current context.
func Resolve(reg *Registry, sig Signals) Resolution {
	if reg == nil {
		return Resolution{}
	}

	var resolution Resolution

	// Route selection consults surface policy.
	resolution.Route = resolveRoute(reg, sig)

	// Supports come from host-embedded / technique-hint bindings.
	resolution.Supports = collectSupports(reg, sig, resolution.Route)

	// Hydrate references from routed + support skills.
	resolution.HydrateReferences = collectHydrateReferences(reg, sig, resolution)

	return resolution
}

func ResolveHostCapabilityRequirement(skillID string, sig Signals) *HostCapabilityRequirement {
	return ResolveHostCapabilityRequirementFromRegistry(DefaultRegistry(), skillID, sig)
}

func ResolveHostCapabilityRequirementFromRegistry(
	reg *Registry,
	skillID string,
	sig Signals,
) *HostCapabilityRequirement {
	skillID = strings.TrimSpace(skillID)
	if reg == nil || skillID == "" {
		return nil
	}
	contract, ok := resolveHostCapabilityContract(reg, skillID)
	if !ok {
		return nil
	}
	req := &HostCapabilityRequirement{
		SkillID:             skillID,
		Capability:          strings.TrimSpace(contract.Capability),
		Required:            contract.Required,
		Availability:        hostCapabilityAvailability(sig.HostCapabilities, contract.Capability),
		EvidenceRequirement: strings.TrimSpace(contract.EvidenceRequirement),
		Remediation:         strings.TrimSpace(contract.Remediation),
	}
	if mode, ok := selectedFallbackMode(sig.Fallbacks, contract.FallbackModes); ok {
		req.FallbackSelected = true
		req.FallbackMode = mode
	}
	return req
}

// resolveHostCapabilityContract returns the host-capability contract for a
// skill. A catalog-registered skill's own HostCapabilities[0] wins; otherwise
// the built-in subagent-dispatch lever supplies a contract for the governance
// skills that REQUIRE a fresh subagent but are intentionally NOT registered in
// the capability catalog. Registering those governance skills would drag them
// into the catalog's surface-manifest, install-profile, and host-skill
// generation machinery (and the frozen DefaultRegistry ID snapshot); the lever
// keeps the engine signal without that surface drag. (#339 / #369)
func resolveHostCapabilityContract(reg *Registry, skillID string) (HostCapabilityContract, bool) {
	if reg != nil {
		if sk, ok := reg.Lookup(skillID); ok && len(sk.HostCapabilities) > 0 {
			return sk.HostCapabilities[0], true
		}
	}
	contract, ok := builtinSubagentDispatchContracts[skillID]
	return contract, ok
}

// builtinSubagentDispatchContracts carries the subagent-dispatch host-capability
// contract for the governance skills that mandate a fresh subagent yet are not
// catalog-registered. Every contract names a generic `same_context_degraded`
// fallback so a single fallback selection can satisfy a whole S3 review batch,
// plus a skill-specific manual fallback for explicit single-skill degradation.
var builtinSubagentDispatchContracts = map[string]HostCapabilityContract{
	"plan-audit": {
		Capability:          "subagent",
		Required:            true,
		FallbackModes:       []string{"manual_plan_audit", "same_context_degraded"},
		EvidenceRequirement: "record plan-audit evidence from a fresh auditor context whose audit_origin handle is distinct from the plan author",
		Remediation:         "Run plan-audit in a host with subagent capability, or explicitly select manual_plan_audit / same_context_degraded fallback and record fresh auditor evidence with a distinct audit_origin handle.",
	},
	"spec-compliance-review": {
		Capability:          "subagent",
		Required:            true,
		FallbackModes:       []string{"manual_spec_compliance_review", "same_context_degraded"},
		EvidenceRequirement: "record spec-compliance-review evidence from a fresh independent reviewer context",
		Remediation:         "Run spec-compliance-review in a host with subagent capability, or explicitly select manual_spec_compliance_review / same_context_degraded fallback and record fresh reviewer evidence with context_origin:stage=review=<handle> plus a fallback:<mode> reference when degraded.",
	},
	"code-quality-review": {
		Capability:          "subagent",
		Required:            true,
		FallbackModes:       []string{"manual_code_quality_review", "same_context_degraded"},
		EvidenceRequirement: "record code-quality-review evidence from a fresh independent reviewer context",
		Remediation:         "Run code-quality-review in a host with subagent capability, or explicitly select manual_code_quality_review / same_context_degraded fallback and record fresh reviewer evidence with context_origin:stage=review=<handle> plus a fallback:<mode> reference when degraded.",
	},
	"ship-verification": {
		Capability:          "subagent",
		Required:            true,
		FallbackModes:       []string{"manual_ship_verification", "same_context_degraded"},
		EvidenceRequirement: "record ship-verification evidence from a fresh verifier context",
		Remediation:         "Run ship-verification in a host with subagent capability, or explicitly select manual_ship_verification / same_context_degraded fallback and record fresh ship-verification evidence.",
	},
}

func hostCapabilityAvailability(tokens []string, capabilityName string) string {
	capabilityName = strings.TrimSpace(capabilityName)
	if capabilityName == "" {
		return "unknown"
	}
	declared := false
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		switch token {
		case "":
			continue
		case "none", "unavailable":
			declared = true
		default:
			declared = true
			if token == capabilityName || (capabilityName == "subagent" && token == "delegation") {
				return "available"
			}
		}
	}
	if declared {
		return "unavailable"
	}
	return "unknown"
}

func selectedFallbackMode(tokens []string, fallbackModes []string) (string, bool) {
	allowed := make(map[string]string, len(fallbackModes))
	for _, mode := range fallbackModes {
		mode = strings.TrimSpace(mode)
		if mode != "" {
			allowed[strings.ToLower(mode)] = mode
		}
	}
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if mode, ok := allowed[token]; ok {
			return mode, true
		}
	}
	return "", false
}

// resolveRoute applies the surface-policy-based route contract. It returns
// the explicit-focus alias when requested, else the primary route for the
// command. Returns nil when no surface policy applies.
func resolveRoute(reg *Registry, sig Signals) *RouteSelection {
	if sig.Command == "" {
		return nil
	}

	if alias := strings.TrimSpace(sig.Focus); alias != "" {
		if rec, ok := LookupFocus(sig.Command, alias); ok {
			if _, regOk := reg.Lookup(rec.BackingID); regOk {
				return &RouteSelection{
					SkillID:   rec.BackingID,
					BackingID: rec.BackingID,
					Mode:      rec.PublicName,
					Reason:    "explicit focus: " + rec.PublicName,
				}
			}
		}
	}

	rec, ok := PrimaryForCommand(sig.Command)
	if !ok {
		return nil
	}
	if _, regOk := reg.Lookup(rec.BackingID); !regOk {
		return nil
	}

	return &RouteSelection{
		SkillID:   rec.BackingID,
		BackingID: rec.BackingID,
		Mode:      rec.PublicName,
		Reason:    rec.Summary,
	}
}

// collectSupports finds skills with BindingHostEmbedded or BindingTechniqueHint
// matching the current host signal. Capped at 3, stable order by skill ID.
func collectSupports(reg *Registry, sig Signals, route *RouteSelection) []Attachment {
	if sig.Host == "" {
		return nil
	}
	routeID := ""
	if route != nil {
		routeID = route.BackingID
	}

	type match struct {
		skill Skill
		mode  AttachmentMode
	}
	all := reg.All()
	matches := make([]match, 0, len(all))
	for _, sk := range all {
		if sk.ID == routeID {
			continue
		}
		if mode, ok := pickSupportAttachment(sk, sig); ok {
			matches = append(matches, match{skill: sk, mode: mode})
		}
	}

	out := make([]Attachment, 0, 3)
	for _, m := range matches {
		if len(out) >= 3 {
			break
		}
		out = append(out, Attachment{
			SkillID: m.skill.ID,
			Kind:    m.mode,
			Reason:  m.skill.Summary,
		})
	}
	return out
}

// collectHydrateReferences unions the hydrate references from the routed
// skill (if any) and any support attachment that is eligible to surface
// hydrate on the current implicit path, returning stable-sorted deduplicated
// skill-relative keys `<skill-id>/<name>`.
func collectHydrateReferences(reg *Registry, sig Signals, res Resolution) []string {
	if reg == nil {
		return nil
	}
	focusBackings := ExplicitFocusBackingIDs()
	explicitFocus := strings.TrimSpace(sig.Focus) != ""

	var ids []string
	if res.Route != nil && res.Route.BackingID != "" {
		ids = append(ids, res.Route.BackingID)
	}
	for _, s := range res.Supports {
		if !supportHydratesInContext(reg, s.SkillID, sig) {
			continue
		}
		if _, focusOnly := focusBackings[s.SkillID]; focusOnly && !explicitFocus {
			continue
		}
		ids = append(ids, s.SkillID)
	}
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, id := range ids {
		for _, key := range HydrateReferenceKeysForSkill(reg, id) {
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// supportHydratesInContext returns true when a supporting skill's hydrate
// shelf should surface on the implicit path represented by sig.
func supportHydratesInContext(reg *Registry, skillID string, sig Signals) bool {
	if reg == nil || skillID == "" {
		return false
	}
	sk, ok := reg.Lookup(skillID)
	if !ok {
		return false
	}
	for _, b := range sk.Bindings {
		switch b.Type {
		case BindingHostEmbedded:
			if sig.Host != "" && b.Target == sig.Host {
				return true
			}
		case BindingTechniqueHint:
			if sig.Host != "" && (b.Target == sig.Host || b.Target == "") {
				return true
			}
		case BindingCommandAuto:
			if sig.Command != "" && bindingMatchesCommand(b, sig.Command) {
				return true
			}
		}
	}
	return false
}

// HydrateReferenceKeysForSkill returns the ordered list of skill-relative
// hydrate keys (`<skill-id>/<name>`) declared by the named skill in the
// registry.
func HydrateReferenceKeysForSkill(reg *Registry, skillID string) []string {
	if reg == nil || skillID == "" {
		return nil
	}
	sk, ok := reg.Lookup(skillID)
	if !ok {
		return nil
	}
	if len(sk.HydrateReferences) == 0 {
		return nil
	}
	out := make([]string, 0, len(sk.HydrateReferences))
	for _, hr := range sk.HydrateReferences {
		out = append(out, sk.ID+"/"+hr.Name)
	}
	return out
}

// pickSupportAttachment returns the attachment mode to emit for a technique
// hint or host-embedded support binding. CommandAuto bindings are excluded —
// they feed the suggested_capabilities[] channel.
func pickSupportAttachment(sk Skill, sig Signals) (AttachmentMode, bool) {
	for _, b := range sk.Bindings {
		switch b.Type {
		case BindingHostEmbedded:
			if sig.Host != "" && b.Target == sig.Host {
				return b.Attachment, true
			}
		case BindingTechniqueHint:
			if sig.Host != "" && (b.Target == sig.Host || b.Target == "") {
				return b.Attachment, true
			}
		}
	}
	return "", false
}

// bindingMatchesCommand returns true when the binding target refers to the
// supplied command surface.
func bindingMatchesCommand(b Binding, command string) bool {
	target := strings.TrimSpace(b.Target)
	command = strings.TrimSpace(command)
	if target == "" || command == "" {
		return false
	}
	if target == command {
		return true
	}
	switch {
	case len(target) > len("command:") && target[:len("command:")] == "command:":
		return strings.TrimSpace(target[len("command:"):]) == command
	case len(target) > len("mode:") && target[:len("mode:")] == "mode:":
		return commandScopeFromPrefixedTarget(target[len("mode:"):]) == command
	}
	return false
}

func commandScopeFromPrefixedTarget(rest string) string {
	trimmed := strings.TrimSpace(rest)
	if trimmed == "" {
		return ""
	}
	if i := strings.Index(trimmed, ":"); i >= 0 {
		return strings.TrimSpace(trimmed[:i])
	}
	return trimmed
}
