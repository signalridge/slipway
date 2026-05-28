package artifact

import (
	"regexp"
	"slices"
	"strings"
)

var (
	requirementHeadingPattern = regexp.MustCompile(`(?i)^###\s*Requirement:\s*(.+?)\s*$`)
	requirementIDPattern      = regexp.MustCompile(`(?i)\bREQ-([A-Z0-9_-]+)\b`)
)

// RequirementBlock describes one checkbox-native requirement block in requirements.md.
type RequirementBlock struct {
	Name     string
	StableID string
}

// ParseRequirementBlocks extracts requirement headings and their first stable REQ-* ID.
func ParseRequirementBlocks(content string) []RequirementBlock {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	type heading struct {
		name string
		line int
	}
	headings := make([]heading, 0)
	for i, line := range lines {
		matches := requirementHeadingPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) != 2 {
			continue
		}
		name := strings.TrimSpace(matches[1])
		if name == "" {
			continue
		}
		headings = append(headings, heading{name: name, line: i})
	}

	blocks := make([]RequirementBlock, 0, len(headings))
	for i, heading := range headings {
		end := len(lines)
		if i+1 < len(headings) {
			end = headings[i+1].line
		}
		block := strings.Join(lines[heading.line:end], "\n")
		blocks = append(blocks, RequirementBlock{
			Name:     heading.name,
			StableID: firstRequirementID(block),
		})
	}
	return blocks
}

// RequirementBlocksMissingStableIDs returns the requirement headings whose block
// is missing a stable REQ-* identifier.
func RequirementBlocksMissingStableIDs(content string) []string {
	missing := make([]string, 0)
	for _, block := range ParseRequirementBlocks(content) {
		if strings.TrimSpace(block.StableID) == "" {
			missing = append(missing, block.Name)
		}
	}
	return missing
}

// ExtractRequirementStableIDs returns the unique stable REQ-* IDs found in content.
func ExtractRequirementStableIDs(content string) []string {
	matches := requirementIDPattern.FindAllStringSubmatch(content, -1)
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		id := NormalizeRequirementID("REQ-" + match[1])
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// NormalizeRequirementID canonicalizes requirement references to REQ-*.
func NormalizeRequirementID(ref string) string {
	trimmed := strings.TrimSpace(strings.Trim(ref, "`"))
	if trimmed == "" {
		return ""
	}
	matches := requirementIDPattern.FindStringSubmatch(trimmed)
	if len(matches) != 2 {
		return ""
	}
	return "REQ-" + strings.ToUpper(strings.TrimSpace(matches[1]))
}

func firstRequirementID(content string) string {
	matches := requirementIDPattern.FindStringSubmatch(content)
	if len(matches) != 2 {
		return ""
	}
	return NormalizeRequirementID("REQ-" + matches[1])
}
