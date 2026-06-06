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

// placeholderTautologyPatterns match the legacy fabricated requirements scenario
// lines (issue #91) by their full, content-free shape. Each pattern is anchored
// to a whole line (`(?im)^…$`) so a concrete authored line that merely shares a
// leading fragment is NOT misread as a placeholder — e.g.
//   - "THEN the expected behavior for an expired token is a 401 response."
//   - "THEN the expected behavior for an audit-log write is observed in the audit log"
//   - "GIVEN the relevant workflow is exercised by an admin during off-hours"
//
// all carry real substance and must pass. Only the exact vacuous legacy lines
// (whole-line, optionally with trailing punctuation/whitespace) are flagged.
// These are regex-based rather than substring sentinels precisely because a bare
// substring ("the relevant workflow is exercised", "… is observed") would also
// match authored prose that continues past the legacy phrasing.
var placeholderTautologyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?im)^\s*GIVEN the relevant workflow is exercised\s*$`),
	regexp.MustCompile(`(?im)^\s*WHEN the requirement is implemented in the target flow\s*$`),
	regexp.MustCompile(`(?im)^\s*THEN the expected behavior for .+ is observed\.?\s*$`),
}

// requirementsPlaceholderPhrases are the engine's own requirements-scaffold seed
// markers (issue #91). They are scoped to what the requirements seed actually
// emits — deliberately NOT the broad decision/research/task sentinels in
// LooksLikeTemplatePlaceholder — so a legitimately-authored requirement that
// happens to contain a generic phrase (e.g. a real requirement about cases in
// "pending investigation" status) is not false-flagged as a placeholder.
var requirementsPlaceholderPhrases = []string{
	"pending — replace with the normative requirement",
	"pending — replace with the precondition",
	"pending — replace with the triggering action",
	"pending — replace with the observable expected outcome",
	"define requirements based on the initial request",
}

// LooksLikeRequirementsPlaceholder reports whether text still carries the
// engine's requirements scaffold seed — a requirements-specific seed marker or
// one of the legacy fabricated GIVEN/WHEN/THEN tautology lines. It is
// deliberately NARROWER than LooksLikeTemplatePlaceholder: it does not match the
// generic decision/research/task sentinels, so authored requirement prose that
// shares a phrase with those seeds is not rejected (issue #91 — avoid false
// positives that block real work). The requirements substance gate uses this
// matcher; LooksLikeTemplatePlaceholder remains a superset that folds these in
// for the decision/runtime and task-objective paths.
func LooksLikeRequirementsPlaceholder(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range requirementsPlaceholderPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	for _, pattern := range placeholderTautologyPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// requirementHeading is a recognized "### Requirement: <name>" heading.
type requirementHeading struct {
	line int
	name string
}

// requirementHeadingsIn returns the requirement headings (line index + title)
// in the given lines. It is the single recognizer shared by ParseRequirementBlocks
// and splitRequirementBlocksForSubstance so heading detection cannot drift
// between the structure check and the substance check.
func requirementHeadingsIn(lines []string) []requirementHeading {
	headings := make([]requirementHeading, 0)
	for i, line := range lines {
		matches := requirementHeadingPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) != 2 {
			continue
		}
		name := strings.TrimSpace(matches[1])
		if name == "" {
			continue
		}
		headings = append(headings, requirementHeading{line: i, name: name})
	}
	return headings
}

// RequirementBlock describes one checkbox-native requirement block in requirements.md.
type RequirementBlock struct {
	Name     string
	StableID string
}

// ParseRequirementBlocks extracts requirement headings and their first stable REQ-* ID.
func ParseRequirementBlocks(content string) []RequirementBlock {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	headings := requirementHeadingsIn(lines)

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
