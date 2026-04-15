package capability

import (
	"sort"
	"strings"
)

// Attachment is the resolver's output record for a single support skill.
// Host LLMs consume Kind (the attachment mode) to decide where to inject
// the referenced skill, and the resolver populates Reason from the matched
// trigger clause.
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

// SuggestedCapability is one entry in Resolution.SuggestedCapabilities. See
// route-surface plan §4.4: cap 3, stable order, disjoint from Supports.
type SuggestedCapability struct {
	Name    string // skill id today; PR-2 may remap to a public alias when one exists
	Summary string
	Reason  string
	Kind    string // "suggested" or "explicit_focus"
	Score   int
}

// Resolution is the full resolver output for one invocation. Supports are
// capped at three entries. SuggestedCapabilities are capped at three, stable-
// ordered, disjoint from Supports and from the routed skill (route-surface
// plan §4.4).
type Resolution struct {
	Route                 *RouteSelection
	Supports              []Attachment
	SuggestedCapabilities []SuggestedCapability
	HydrateReferences     []string
	LLMTiebreak           *TiebreakRecord
}

// TiebreakRecord captures a DSL-score tie that needs LLM adjudication.
// B1 never emits one; B7 introduces the first real producer.
type TiebreakRecord struct {
	Candidates []string
	Criterion  string
}

// Resolve scans the registry once against the supplied signals and returns:
//   - the routed primary/explicit-focus/view selection (via surface policy),
//   - up to three supporting attachments from host-embedded / technique-hint
//     bindings,
//   - up to three suggested capabilities from BindingCommandAuto matches that
//     were not promoted to the route or Supports,
//   - the union of hydrate references eligible on the current context.
//
// Resolve does not apply the status/health change-scoped gate described in
// route-surface plan §4.2 — that gate is the command layer's responsibility.
type resolverCandidate struct {
	skill  Skill
	clause TriggerClause
	score  int
}

func Resolve(reg *Registry, sig Signals) Resolution {
	if reg == nil {
		return Resolution{}
	}

	var matches []resolverCandidate
	for _, sk := range reg.All() {
		for _, clause := range sk.Triggers {
			if clause.Match(sig) {
				matches = append(matches, resolverCandidate{
					skill:  sk,
					clause: clause,
					score:  scoreClause(clause, sig),
				})
				break
			}
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].skill.ID < matches[j].skill.ID
	})

	var resolution Resolution

	// Route selection consults surface policy, not raw bindings.
	// Precedence: explicit Focus > explicit View > primary-for-command.
	resolution.Route = resolveRoute(reg, sig, matches)

	// Supports come from host-embedded / technique-hint bindings only.
	for _, m := range matches {
		if len(resolution.Supports) >= 3 {
			break
		}
		if resolution.Route != nil && m.skill.ID == resolution.Route.BackingID {
			continue
		}
		kind, ok := pickSupportAttachment(m.skill, sig)
		if !ok {
			continue
		}
		resolution.Supports = append(resolution.Supports, Attachment{
			SkillID: m.skill.ID,
			Kind:    kind,
			Reason:  m.clause.Reason,
			Score:   m.score,
		})
	}

	// Suggested capabilities come from command-auto matches that are not
	// already present as route/support. Cap 3, stable order.
	resolution.SuggestedCapabilities = collectSuggestedCapabilities(matches, sig, resolution)

	resolution.HydrateReferences = collectHydrateReferences(reg, sig, resolution)
	return resolution
}

// resolveRoute applies the surface-policy-based route contract. It returns
// the explicit-focus alias when requested, else the explicit-view alias,
// else the primary route for the command. Returns nil when no surface
// policy applies.
func resolveRoute(reg *Registry, sig Signals, matches []resolverCandidate) *RouteSelection {
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

	// Borrow a reason from any matched clause on the primary skill for
	// transparency; fall back to the surface summary otherwise.
	reason := rec.Summary
	for _, m := range matches {
		if m.skill.ID == rec.BackingID {
			reason = m.clause.Reason
			break
		}
	}

	route := &RouteSelection{
		SkillID:   rec.BackingID,
		BackingID: rec.BackingID,
		Reason:    reason,
	}
	switch sig.Command {
	case "status", "health":
		route.View = rec.PublicName
	default:
		route.Mode = rec.PublicName
	}
	return route
}

// collectSuggestedCapabilities populates the bounded, deterministic
// suggested-capabilities channel from BindingCommandAuto matches (route-
// surface plan §4.4).
func collectSuggestedCapabilities(matches []resolverCandidate, sig Signals, res Resolution) []SuggestedCapability {
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
	focusBackings := ExplicitFocusBackingIDs()

	var out []SuggestedCapability
	for _, m := range matches {
		if _, skip := excluded[m.skill.ID]; skip {
			continue
		}
		if !skillHasCommandAutoFor(m.skill, sig.Command) {
			continue
		}
		kind := "suggested"
		if _, isFocus := focusBackings[m.skill.ID]; isFocus {
			kind = "explicit_focus"
		}
		out = append(out, SuggestedCapability{
			Name:    m.skill.ID,
			Summary: m.skill.Summary,
			Reason:  m.clause.Reason,
			Kind:    kind,
			Score:   m.score,
		})
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func skillHasCommandAutoFor(sk Skill, command string) bool {
	for _, b := range sk.Bindings {
		if b.Type == BindingCommandAuto && bindingMatchesCommand(b, command) {
			return true
		}
	}
	return false
}

// collectHydrateReferences unions the hydrate references from the routed
// skill (if any) and any support attachment that is eligible to surface
// hydrate on the current implicit path, returning stable-sorted deduplicated
// skill-relative keys `<skill-id>/<name>`. Runtime output always uses the
// collision-safe form; basename-only keys are never emitted.
//
// Explicit-focus-backed skills only hydrate when explicitly selected via
// Signals.Focus (route-surface plan §6: implicit host-embedded attachment
// preserves Supports but not hydrate).
func collectHydrateReferences(reg *Registry, sig Signals, res Resolution) []string {
	if reg == nil {
		return nil
	}
	focusBackings := ExplicitFocusBackingIDs()
	explicitFocus := strings.TrimSpace(sig.Focus) != ""

	var ids []string
	if res.Route != nil && res.Route.BackingID != "" {
		// The route itself always hydrates (whether primary, explicit focus,
		// or explicit view).
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

func scoreClause(c TriggerClause, sig Signals) int {
	switch c.Op {
	case OpAllOf:
		score := 0
		for _, child := range c.Children {
			if child.Match(sig) {
				score += scoreClause(child, sig)
			}
		}
		return score + len(c.Children)
	case OpAnyOf:
		best := 0
		for _, child := range c.Children {
			if !child.Match(sig) {
				continue
			}
			if s := scoreClause(child, sig); s > best {
				best = s
			}
		}
		return best
	case OpNot:
		return 1
	case OpCommand:
		return 3
	case OpHost:
		return 3
	case OpBlockerReason:
		return 4
	case OpChangedFilesInclude, OpPathIncludes:
		return 2
	case OpUserTextMatches:
		return 1
	}
	return 0
}

// pickSupportAttachment returns the attachment mode to emit for a technique
// hint or host-embedded support binding. Route-surface plan §5: CommandAuto
// bindings are excluded — they feed the suggested_capabilities[] channel.
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
	for _, b := range sk.Bindings {
		if b.Type == BindingTechniqueHint {
			return sk.PrimaryAttachment, true
		}
	}
	return "", false
}

// bindingMatchesCommand returns true when the binding target refers to the
// supplied command surface. Targets may carry an optional `mode:...`,
// `view:...`, or `command:<name>` prefix; bare command names also match.
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
