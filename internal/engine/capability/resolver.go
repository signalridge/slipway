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

// RouteSelection names a routed command choice (e.g. review mode,
// status view). Empty Mode and View mean "resolver declined to route".
type RouteSelection struct {
	SkillID string
	Mode    string // for review / validate / repair
	View    string // for status / health
	Reason  string
}

// Resolution is the full resolver output for one invocation. Supports are
// capped at three entries. HydrateReferences is reserved for B2+ and stays
// nil at B1. LLMTiebreak is reserved for B7+.
type Resolution struct {
	Route             *RouteSelection
	Supports          []Attachment
	HydrateReferences []string
	LLMTiebreak       *TiebreakRecord
}

// TiebreakRecord captures a DSL-score tie that needs LLM adjudication.
// B1 never emits one; B7 introduces the first real producer.
type TiebreakRecord struct {
	Candidates []string
	Criterion  string
}

// Resolve scans the registry once against the supplied signals and returns
// the highest-ranked routed binding (if any) plus up to three supporting
// attachments. Support attachments are sorted by score then by skill id.
func Resolve(reg *Registry, sig Signals) Resolution {
	if reg == nil {
		return Resolution{}
	}

	type candidate struct {
		skill  Skill
		clause TriggerClause
		score  int
	}

	var matches []candidate
	for _, sk := range reg.All() {
		for _, clause := range sk.Triggers {
			if clause.Match(sig) {
				matches = append(matches, candidate{
					skill:  sk,
					clause: clause,
					score:  scoreClause(clause, sig),
				})
				// Keep only the first matched top-level clause per skill.
				// Later matching clauses for the same skill do not stack score.
				break
			}
		}
	}

	if len(matches) == 0 {
		return Resolution{}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].skill.ID < matches[j].skill.ID
	})

	var resolution Resolution

	for _, m := range matches {
		if resolution.Route != nil {
			break
		}
		route := pickRoute(m.skill, sig)
		if route == nil {
			continue
		}
		route.Reason = m.clause.Reason
		resolution.Route = route
	}

	for _, m := range matches {
		if len(resolution.Supports) >= 3 {
			break
		}
		if resolution.Route != nil && m.skill.ID == resolution.Route.SkillID {
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

	resolution.HydrateReferences = collectHydrateReferences(reg, sig, resolution)
	return resolution
}

// collectHydrateReferences unions the hydrate references from the routed
// skill (if any) and any support attachment that is eligible to surface
// hydrate on the current implicit path, returning stable-sorted deduplicated
// skill-relative keys `<skill-id>/<name>`. Runtime output always uses the
// collision-safe form; basename-only keys are never emitted.
func collectHydrateReferences(reg *Registry, sig Signals, res Resolution) []string {
	if reg == nil {
		return nil
	}
	var ids []string
	if res.Route != nil && res.Route.SkillID != "" {
		ids = append(ids, res.Route.SkillID)
	}
	for _, s := range res.Supports {
		if !supportHydratesInContext(reg, s.SkillID, sig) {
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
// shelf should surface on the implicit path represented by sig. Command-manual
// attachments remain explicit-only: their hydrate is advertised only through
// `--mode` / `--view`, not through default routed command output.
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
// registry. Callers that bypass Resolve() - for example, the manual-explicit
// route in cmd/route_flags.go - use this helper to derive hydrate output
// without re-running signal matching.
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

// pickRoute returns a routed command selection when the skill has an
// applicable command-auto (mode) or command-view (view) binding for the
// current command surface.
func pickRoute(sk Skill, sig Signals) *RouteSelection {
	if sig.Command == "" {
		return nil
	}
	for _, b := range sk.Bindings {
		switch b.Type {
		case BindingCommandAuto:
			if !bindingMatchesCommand(b, sig.Command) {
				continue
			}
			return &RouteSelection{
				SkillID: sk.ID,
				Mode:    sk.ID,
			}
		case BindingCommandView:
			if !bindingMatchesCommand(b, sig.Command) {
				continue
			}
			return &RouteSelection{
				SkillID: sk.ID,
				View:    sk.ID,
			}
		}
	}
	return nil
}

// pickSupportAttachment returns the attachment mode to emit for a technique
// hint or host-embedded support binding, given the current context. It
// prefers the most specific binding that applies to the current host or
// command surface.
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
		case BindingCommandAuto, BindingCommandManual:
			if sig.Command != "" && bindingMatchesCommand(b, sig.Command) {
				return b.Attachment, true
			}
		}
	}
	// Fall back to the primary attachment for technique hints when no
	// per-context binding matched.
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
//
// To avoid accidental cross-command matches, prefixed targets are command-
// scoped:
// - `mode:<command>` or `mode:<command>:<route-id>`
// - `view:<command>` or `view:<command>:<view-id>`
// - `command:<command>`
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
