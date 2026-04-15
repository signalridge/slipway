package capability

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Operator names the single operator used by a TriggerClause. The bounded
// set lives here so a schema-lint rejection of an unknown operator is
// directly traceable to this file.
type Operator string

const (
	OpAllOf               Operator = "all_of"
	OpAnyOf               Operator = "any_of"
	OpNot                 Operator = "not"
	OpCommand             Operator = "command"
	OpHost                Operator = "host"
	OpBlockerReason       Operator = "blocker_reason"
	OpChangedFilesInclude Operator = "changed_files_include"
	OpPathIncludes        Operator = "path_includes"
	OpUserTextMatches     Operator = "user_text_matches"
)

// TriggerClause is one declarative match rule. Only one operator may be
// populated per clause; nested operators put their children in Children.
// Every top-level clause must carry a Reason used to explain the match in
// TechniqueHints output.
type TriggerClause struct {
	Op       Operator
	Value    string
	Values   []string
	Children []TriggerClause
	Reason   string
}

func (c TriggerClause) validate() error {
	switch c.Op {
	case OpAllOf, OpAnyOf:
		if len(c.Children) == 0 {
			return fmt.Errorf("%s requires children", c.Op)
		}
		for i, child := range c.Children {
			if err := child.validateChild(); err != nil {
				return fmt.Errorf("%s.children[%d]: %w", c.Op, i, err)
			}
		}
	case OpNot:
		if len(c.Children) != 1 {
			return fmt.Errorf("not requires exactly one child")
		}
		if err := c.Children[0].validateChild(); err != nil {
			return fmt.Errorf("not.children[0]: %w", err)
		}
	case OpCommand, OpHost, OpBlockerReason,
		OpChangedFilesInclude, OpPathIncludes, OpUserTextMatches:
		if strings.TrimSpace(c.Value) == "" && len(c.Values) == 0 {
			return fmt.Errorf("%s requires a value", c.Op)
		}
	default:
		return fmt.Errorf("unknown operator %q", c.Op)
	}
	if strings.TrimSpace(c.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	return nil
}

// validateChild mirrors validate but allows nested clauses to skip the
// Reason requirement — only the top-level clause surfaces through the
// resolver output.
func (c TriggerClause) validateChild() error {
	switch c.Op {
	case OpAllOf, OpAnyOf:
		if len(c.Children) == 0 {
			return fmt.Errorf("%s requires children", c.Op)
		}
		for i, child := range c.Children {
			if err := child.validateChild(); err != nil {
				return fmt.Errorf("%s.children[%d]: %w", c.Op, i, err)
			}
		}
	case OpNot:
		if len(c.Children) != 1 {
			return fmt.Errorf("not requires exactly one child")
		}
		return c.Children[0].validateChild()
	case OpCommand, OpHost, OpBlockerReason,
		OpChangedFilesInclude, OpPathIncludes, OpUserTextMatches:
		if strings.TrimSpace(c.Value) == "" && len(c.Values) == 0 {
			return fmt.Errorf("%s requires a value", c.Op)
		}
	default:
		return fmt.Errorf("unknown operator %q", c.Op)
	}
	return nil
}

// Signals is the runtime context consumed by trigger evaluation. The
// resolver and capability tests populate it from command input, governed
// change data, and user text. Unset fields simply fail their matching
// clauses (no wildcard matching).
type Signals struct {
	Command      string
	Host         string
	Blockers     []string
	ChangedFiles []string
	Paths        []string
	UserText     string
	// Focus names an explicit `--focus <alias>` selection resolved through
	// surface policy. Empty means no explicit focus was requested; route
	// selection falls back to the primary surface for the command.
	Focus string
	// View names an explicit `--view <alias>` selection (status / health).
	// Empty means the command layer did not request an explicit view.
	View string
}

// Match returns true when every top-level clause operator semantics hold
// against the supplied Signals. Operator semantics mirror trigger.go's
// frozen DSL.
func (c TriggerClause) Match(sig Signals) bool {
	switch c.Op {
	case OpAllOf:
		for _, child := range c.Children {
			if !child.Match(sig) {
				return false
			}
		}
		return true
	case OpAnyOf:
		for _, child := range c.Children {
			if child.Match(sig) {
				return true
			}
		}
		return false
	case OpNot:
		return !c.Children[0].Match(sig)
	case OpCommand:
		return matchExactAny(sig.Command, c.Value, c.Values)
	case OpHost:
		return matchExactAny(sig.Host, c.Value, c.Values)
	case OpBlockerReason:
		for _, blocker := range sig.Blockers {
			if matchExactAny(blocker, c.Value, c.Values) {
				return true
			}
		}
		return false
	case OpChangedFilesInclude:
		return matchGlobAny(sig.ChangedFiles, c.Value, c.Values)
	case OpPathIncludes:
		joined := strings.Join(sig.Paths, "\n")
		return matchSubstringAny(joined, c.Value, c.Values)
	case OpUserTextMatches:
		return matchSubstringAnyFold(sig.UserText, c.Value, c.Values)
	}
	return false
}

func matchExactAny(candidate, single string, multi []string) bool {
	cand := strings.TrimSpace(candidate)
	if cand == "" {
		return false
	}
	if strings.TrimSpace(single) != "" && cand == strings.TrimSpace(single) {
		return true
	}
	for _, v := range multi {
		if cand == strings.TrimSpace(v) {
			return true
		}
	}
	return false
}

func matchGlobAny(files []string, single string, multi []string) bool {
	patterns := mergeNonEmpty(single, multi)
	for _, pattern := range patterns {
		for _, expanded := range expandBracePatterns(filepath.ToSlash(pattern)) {
			for _, f := range files {
				if globMatchPath(expanded, filepath.ToSlash(f)) {
					return true
				}
			}
		}
	}
	return false
}

func globMatchPath(pattern, candidate string) bool {
	p := strings.TrimSpace(pattern)
	c := strings.TrimSpace(candidate)
	if p == "" || c == "" {
		return false
	}
	re, err := regexp.Compile(globPatternToRegexp(p))
	if err != nil {
		return false
	}
	return re.MatchString(c)
}

// globPatternToRegexp converts the bounded trigger-glob subset into a regexp:
// `*`, `?`, `**`, and brace-expanded variants. `**/` also matches zero dirs.
func globPatternToRegexp(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		switch {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 3
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i += 2
		default:
			ch := pattern[i]
			switch ch {
			case '*':
				b.WriteString("[^/]*")
			case '?':
				b.WriteString("[^/]")
			case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
				b.WriteByte('\\')
				b.WriteByte(ch)
			default:
				b.WriteByte(ch)
			}
			i++
		}
	}
	b.WriteString("$")
	return b.String()
}

// expandBracePatterns expands one brace group at a time, e.g.
// `**/*.{yml,yaml}` -> `**/*.yml`, `**/*.yaml`.
func expandBracePatterns(pattern string) []string {
	start, end, parts, ok := firstBraceGroup(pattern)
	if !ok {
		return []string{pattern}
	}
	prefix := pattern[:start]
	suffix := pattern[end+1:]
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, expandBracePatterns(prefix+part+suffix)...)
	}
	return out
}

func firstBraceGroup(pattern string) (start int, end int, parts []string, ok bool) {
	start = -1
	depth := 0
	partStart := 0
	for i, r := range pattern {
		switch r {
		case '{':
			if depth == 0 {
				start = i
				partStart = i + 1
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 {
				parts = append(parts, pattern[partStart:i])
				return start, i, parts, true
			}
		case ',':
			if depth == 1 {
				parts = append(parts, pattern[partStart:i])
				partStart = i + 1
			}
		}
	}
	return 0, 0, nil, false
}

func matchSubstringAny(hay, single string, multi []string) bool {
	needles := mergeNonEmpty(single, multi)
	for _, needle := range needles {
		if strings.Contains(hay, needle) {
			return true
		}
	}
	return false
}

func matchSubstringAnyFold(hay, single string, multi []string) bool {
	needles := mergeNonEmpty(single, multi)
	hayFold := strings.ToLower(hay)
	for _, needle := range needles {
		if strings.Contains(hayFold, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func mergeNonEmpty(single string, multi []string) []string {
	out := make([]string, 0, 1+len(multi))
	if s := strings.TrimSpace(single); s != "" {
		out = append(out, s)
	}
	for _, v := range multi {
		if s := strings.TrimSpace(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}
