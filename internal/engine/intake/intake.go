package intake

import (
	"fmt"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/stringutil"
)

const maxSourceDocumentLength = 4000

type DocSeed struct {
	Summary     string
	Scope       string
	OutOfScope  string
	Constraints string
	Acceptance  string
}

type PromptPayload struct {
	Header   string
	Lines    []string
	Question string
}

// ParseDoc extracts the deterministic seedable parts of a user-provided
// document for `slipway new --from-doc`.
func ParseDoc(doc string) DocSeed {
	return DocSeed{
		Summary:     ExtractSummary(doc),
		Scope:       firstMarkdownSection(doc, "## scope", "## in scope", "## requirements", "## goals", "### scope", "### in scope", "### requirements", "### goals"),
		OutOfScope:  firstMarkdownSection(doc, "## out of scope", "## non-goals", "## non goals", "### out of scope", "### non-goals", "### non goals"),
		Constraints: firstMarkdownSection(doc, "## constraints", "## limitations", "## technical constraints", "### constraints", "### limitations"),
		Acceptance:  firstMarkdownSection(doc, "## acceptance", "## acceptance criteria", "## success criteria", "## definition of done", "## done criteria", "### acceptance", "### acceptance criteria", "### success criteria"),
	}
}

// ExtractSummary returns the first meaningful one-line summary from a document.
func ExtractSummary(doc string) string {
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
		if strings.HasPrefix(trimmed, "## ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "##"))
		}
		if !strings.HasPrefix(trimmed, "<!--") && !strings.HasPrefix(trimmed, "---") {
			if len(trimmed) > 200 {
				trimmed = trimmed[:200]
			}
			return trimmed
		}
	}
	return ""
}

// SeedIntentFile applies document-derived content to a scaffolded intent.md.
func SeedIntentFile(intentPath, docContent string, doc DocSeed) error {
	data, err := os.ReadFile(intentPath)
	if err != nil {
		return fmt.Errorf("intent.md not found after scaffold: %w — this should not happen", err)
	}
	updated := SeedIntentContent(string(data), docContent, doc)
	return os.WriteFile(intentPath, []byte(updated), 0o644)
}

// SeedIntentContent mutates the scaffolded intent.md content deterministically.
func SeedIntentContent(content, docContent string, doc DocSeed) string {
	if doc.Scope != "" {
		content = populateIntentSection(content, "## In Scope", doc.Scope)
	}
	if doc.OutOfScope != "" {
		content = populateIntentSection(content, "## Out of Scope", doc.OutOfScope)
	}
	if doc.Constraints != "" {
		content = populateIntentSection(content, "## Constraints", doc.Constraints)
	}
	if doc.Acceptance != "" {
		content = populateIntentSection(content, "## Acceptance Signals", doc.Acceptance)
	}

	summaryIdx := strings.Index(content, "## Summary")
	if summaryIdx < 0 {
		return content
	}

	truncated := docContent
	if len(truncated) > maxSourceDocumentLength {
		truncated = truncated[:maxSourceDocumentLength] + "\n\n<!-- truncated: original document was longer -->"
	}

	rest := content[summaryIdx+len("## Summary"):]
	nextHeading := strings.Index(rest, "\n## ")
	insertPoint := len(content)
	if nextHeading >= 0 {
		insertPoint = summaryIdx + len("## Summary") + nextHeading
	}
	seedBlock := fmt.Sprintf("\n\n### Source Document\n\n%s\n", truncated)
	return content[:insertPoint] + seedBlock + content[insertPoint:]
}

// BuildInteractivePromptPayload prepares the deterministic prompt content for
// interactive `slipway new`.
func BuildInteractivePromptPayload(_ string, projectCtx model.ProjectContext) PromptPayload {
	lines := make([]string, 0, 4+strings.Count(projectCtx.RecentWork, "\n"))
	if projectCtx.TechStack != "" {
		lines = append(lines, fmt.Sprintf("  Tech Stack: %s", projectCtx.TechStack))
	}
	if len(projectCtx.Languages) > 0 {
		lines = append(lines, fmt.Sprintf("  Languages:  %s", strings.Join(projectCtx.Languages, ", ")))
	}
	if projectCtx.RecentWork != "" {
		lines = append(lines, "  Recent work:")
		for _, line := range strings.Split(projectCtx.RecentWork, "\n") {
			lines = append(lines, "    "+line)
		}
	}

	return PromptPayload{
		Header:   "Project context (auto-detected):",
		Lines:    lines,
		Question: "What change do you want to make? ",
	}
}

func firstMarkdownSection(doc string, headings ...string) string {
	for _, heading := range headings {
		if section := extractMarkdownSection(doc, heading); section != "" {
			return section
		}
	}
	return ""
}

func extractMarkdownSection(doc, heading string) string {
	target := normalizeMarkdownHeading(heading)
	if target == "" {
		return ""
	}

	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	inSection := false
	section := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if inSection {
				break
			}
			if normalizeMarkdownHeading(trimmed) == target {
				inSection = true
			}
			continue
		}
		if inSection {
			section = append(section, line)
		}
	}
	return strings.TrimSpace(strings.Join(section, "\n"))
}

func normalizeMarkdownHeading(heading string) string {
	trimmed := strings.TrimSpace(heading)
	trimmed = strings.TrimSpace(strings.TrimRight(trimmed, "#"))
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
	return strings.ToLower(trimmed)
}

func populateIntentSection(content, heading, sectionContent string) string {
	idx := strings.Index(content, heading)
	if idx < 0 {
		return content
	}
	afterHeading := content[idx+len(heading):]
	nextHeading := strings.Index(afterHeading, "\n## ")
	sectionEnd := len(content)
	if nextHeading >= 0 {
		sectionEnd = idx + len(heading) + nextHeading
	}

	existing := strings.TrimSpace(stringutil.StripHTMLComments(afterHeading[:sectionEnd-idx-len(heading)]))
	if existing != "" {
		return content
	}

	return content[:idx+len(heading)] + "\n" + sectionContent + "\n" + content[sectionEnd:]
}
