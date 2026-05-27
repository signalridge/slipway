package stringutil

import "strings"

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
// contains unresolved content. A resolved checklist entry, an empty/comment-only
// section, or an explicit none marker is documentation, not an intake blocker.
func HasBlockingOpenQuestions(content string) bool {
	section := LastMarkdownSectionContent(content, "## Open Questions")
	if section == "" {
		return false
	}
	lines := strings.Split(section, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
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
		return true
	}
	return false
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
