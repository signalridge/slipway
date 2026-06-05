package stringutil

import (
	"regexp"
	"strings"
)

// StripHTMLComments removes <!-- ... --> blocks from the string.
func StripHTMLComments(s string) string {
	result := s
	for {
		start := strings.Index(result, "<!--")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], "-->")
		if end < 0 {
			break
		}
		result = result[:start] + result[start+end+3:]
	}
	return result
}

// LastMarkdownSectionContent returns the content for the last matching level-2
// markdown heading. This lets canonical intent sections win over duplicated
// headings embedded earlier in the document, such as a source document copied
// into the Summary section.
func LastMarkdownSectionContent(content, heading string) string {
	normalizedHeading := strings.TrimSpace(heading)
	if !strings.HasPrefix(normalizedHeading, "## ") {
		normalizedHeading = "## " + normalizedHeading
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	inSection := false
	section := make([]string, 0)
	last := ""

	flush := func() {
		last = strings.TrimSpace(StripHTMLComments(strings.Join(section, "\n")))
		section = section[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if inSection {
				flush()
				inSection = false
			}
			if trimmed == normalizedHeading {
				inSection = true
			}
			continue
		}
		if inSection {
			section = append(section, line)
		}
	}
	if inSection {
		flush()
	}
	return last
}

// HasBlockingOpenQuestions reports whether the canonical Open Questions section
// contains unresolved content. Documentation is not an intake blocker: a resolved
// checklist entry (`- [x]`), an indented continuation of a list item, an explicit
// none marker, or a line carrying a RESOLVED/ANSWERED marker all read as resolved.
// Only an explicitly unchecked checklist item (`- [ ]`) or an unmarked bare entry
// blocks. Continuations and resolution markers keep a wrapped or prose-documented
// answer from reading as a fresh question.
func HasBlockingOpenQuestions(content string) bool {
	section := LastMarkdownSectionContent(content, "## Open Questions")
	if section == "" {
		return false
	}
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// An indented line that does not itself start a list item is a
		// continuation of the previous entry; it inherits that entry's state and
		// never blocks on its own. This lets a resolved `- [x]` item wrap across
		// lines without the wrapped text reading as a new question.
		if isOpenQuestionContinuation(line, trimmed) {
			continue
		}
		lowerTrimmed := strings.ToLower(trimmed)
		if strings.HasPrefix(lowerTrimmed, "- [x]") || strings.HasPrefix(lowerTrimmed, "* [x]") {
			continue
		}
		if strings.HasPrefix(lowerTrimmed, "- [ ]") || strings.HasPrefix(lowerTrimmed, "* [ ]") {
			return true
		}
		if isExplicitNoneMarker(trimmed) {
			continue
		}
		if hasResolvedMarker(trimmed) {
			continue
		}
		return true
	}
	return false
}

// isOpenQuestionContinuation reports whether raw is an indented continuation of
// the previous list item rather than a new top-level entry. An indented line that
// itself starts a list marker is treated as its own (nested) entry, so a nested
// `- [ ]` still blocks.
func isOpenQuestionContinuation(raw, trimmed string) bool {
	if raw == trimmed {
		return false // no leading indentation: a top-level line
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return false // an indented list item is its own entry, not a continuation
	}
	return true
}

var resolvedMarkerPattern = regexp.MustCompile(`(?i)\b(resolved|answered)\b`)

// hasResolvedMarker reports whether a line carries an explicit resolution marker
// (RESOLVED or ANSWERED, case-insensitive, on a word boundary so "unresolved"
// does not match). Such a line documents an answered question.
func hasResolvedMarker(line string) bool {
	return resolvedMarkerPattern.MatchString(line)
}

func isExplicitNoneMarker(line string) bool {
	normalized := strings.TrimSpace(line)
	for {
		next := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(normalized, "-"), "*"))
		if next == normalized {
			break
		}
		normalized = next
	}
	normalized = strings.Trim(normalized, " \t().:")
	normalized = strings.TrimSuffix(normalized, ".")
	normalized = strings.ToLower(strings.TrimSpace(normalized))

	switch normalized {
	case "none", "n/a", "na", "not applicable", "no open questions", "no unresolved questions":
		return true
	default:
		return false
	}
}
