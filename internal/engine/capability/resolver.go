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
	Score   int
}

// RouteSelection names a routed command choice. Mode carries the public
// focus alias (or primary public name); View carries the public view alias
// (or primary public name for status/health). BackingID is the catalog
// skill id that implements the route.
type RouteSelection struct {
	SkillID   string
	BackingID string
	Mode      string // public focus/primary alias for review/validate/repair
	View      string // public view/primary alias for status/health
	Reason    string
}

// SuggestedCapability is one entry in Resolution.SuggestedCapabilities.
// Cap 3, stable order, disjoint from Supports.
type SuggestedCapability struct {
	Name    string // public surface name exposed to operators
	Summary string
	Reason  string
	Kind    string // "suggested" or "explicit_focus"
	Score   int
}

// Resolution is the full resolver output for one invocation. Supports are
// capped at three entries. SuggestedCapabilities are capped at three, stable-
// ordered, disjoint from Supports.
type Resolution struct {
	Route                 *RouteSelection
	Supports              []Attachment
	SuggestedCapabilities []SuggestedCapability
	HydrateReferences     []string
}

// Signals carries the context inputs for capability resolution.
type Signals struct {
	Command      string
	Host         string
	Blockers     []string
	ChangedFiles []string
	Paths        []string
	UserText     string
	// Focus names an explicit `--focus <alias>` selection resolved through
	// surface policy. Empty means no explicit focus was requested.
	Focus string
	// View names an explicit `--view <alias>` selection (status / health).
	View string
}

// Resolve scans the registry using Binding metadata (not dynamic trigger DSL)
// and returns:
//   - the routed primary/explicit-focus/view selection (via surface policy),
//   - up to three supporting attachments from host-embedded / technique-hint
//     bindings,
//   - up to three suggested capabilities from command-auto bindings,
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

	// Suggested capabilities from command-auto bindings.
	resolution.SuggestedCapabilities = collectSuggestedCapabilities(reg, sig, resolution)

	// Hydrate references from routed + support skills.
	resolution.HydrateReferences = collectHydrateReferences(reg, sig, resolution)

	return resolution
}

// resolveRoute applies the surface-policy-based route contract. It returns
// the explicit-focus alias when requested, else the explicit-view alias,
// else the primary route for the command. Returns nil when no surface
// policy applies.
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
	if alias := strings.TrimSpace(sig.View); alias != "" {
		if rec, ok := LookupView(sig.Command, alias); ok {
			if _, regOk := reg.Lookup(rec.BackingID); regOk {
				return &RouteSelection{
					SkillID:   rec.BackingID,
					BackingID: rec.BackingID,
					View:      rec.PublicName,
					Reason:    "explicit view: " + rec.PublicName,
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

	route := &RouteSelection{
		SkillID:   rec.BackingID,
		BackingID: rec.BackingID,
		Reason:    rec.Summary,
	}
	switch sig.Command {
	case "status", "health":
		route.View = rec.PublicName
	default:
		route.Mode = rec.PublicName
	}
	return route
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
	var matches []match
	for _, sk := range reg.All() {
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

// collectSuggestedCapabilities finds skills with BindingCommandAuto matching
// the command, filtered through surface policy. Capped at 3, disjoint from
// route and supports.
func collectSuggestedCapabilities(reg *Registry, sig Signals, res Resolution) []SuggestedCapability {
	if sig.Command == "" {
		return nil
	}
	excluded := make(map[string]struct{})
	if res.Route != nil && res.Route.BackingID != "" {
		excluded[res.Route.BackingID] = struct{}{}
	}
	for _, s := range res.Supports {
		excluded[s.SkillID] = struct{}{}
	}

	var out []SuggestedCapability
	for _, sk := range reg.All() {
		if _, skip := excluded[sk.ID]; skip {
			continue
		}
		if !skillHasCommandAutoFor(sk, sig.Command) {
			continue
		}
		surface, ok := suggestionSurfaceForBacking(sig.Command, sk.ID)
		if !ok {
			continue
		}
		kind := "suggested"
		if surface.Class == SurfaceExplicitFocus {
			kind = "explicit_focus"
		}
		summary := strings.TrimSpace(surface.Summary)
		if summary == "" {
			summary = sk.Summary
		}
		out = append(out, SuggestedCapability{
			Name:    surface.PublicName,
			Summary: summary,
			Reason:  sk.Summary,
			Kind:    kind,
		})
		if len(out) >= 3 {
			break
		}
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

// skillHasCommandAutoFor checks whether the skill has a BindingCommandAuto
// for the given command.
func skillHasCommandAutoFor(sk Skill, command string) bool {
	for _, b := range sk.Bindings {
		if b.Type == BindingCommandAuto && bindingMatchesCommand(b, command) {
			return true
		}
	}
	return false
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
	case len(target) > len("view:") && target[:len("view:")] == "view:":
		return commandScopeFromPrefixedTarget(target[len("view:"):]) == command
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
