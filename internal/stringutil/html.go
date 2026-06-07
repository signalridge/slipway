package stringutil

import (
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
// holds an unresolved entry. Open questions are recorded as a markdown checklist:
// an unchecked item (`- [ ]` / `* [ ]`) is unresolved and blocks intake; a checked
// item (`- [x]`) is resolved. Everything else — an empty section, an explicit
// `None`, or free-form prose — is documentation, not a blocker. Deciding what
// counts as a real open question is a semantic judgment owned by the
// intake-clarification host skill, which records it as a checklist item; the
// engine gates only on that structure, never on prose.
func HasBlockingOpenQuestions(content string) bool {
	return FirstBlockingOpenQuestion(content) != ""
}

// FirstBlockingOpenQuestion returns the trimmed text of the first unchecked
// checklist entry in the canonical Open Questions section, or "" when nothing
// blocks. It backs HasBlockingOpenQuestions and lets callers name the specific
// entry that is holding intake in clarification, so routing is not silent.
func FirstBlockingOpenQuestion(content string) string {
	section := LastMarkdownSectionContent(content, "## Open Questions")
	if section == "" {
		return ""
	}
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "- [ ]") || strings.HasPrefix(lower, "* [ ]") {
			return trimmed
		}
	}
	return ""
}
