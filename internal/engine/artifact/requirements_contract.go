package artifact

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

// rfc2119Pattern matches an RFC-2119 normative keyword (uppercase, as authored
// requirements use them) anywhere in a requirement body. MUST/SHALL also cover
// their negative forms (MUST NOT / SHALL NOT); REQUIRED is accepted as an
// equivalent strong-obligation keyword so a legitimately-authored requirement is
// not hard-blocked for phrasing the obligation as "is REQUIRED to …".
var rfc2119Pattern = regexp.MustCompile(`\b(MUST|SHALL|REQUIRED)\b`)

// scenarioHeadingPattern matches a "#### Scenario:" heading that separates a
// requirement's normative statement from its acceptance scenarios. The RFC-2119
// substance check applies to the statement region only, so a MUST/SHALL that
// appears in a scenario's GIVEN/WHEN/THEN line does not satisfy the gate
// (issue #91 blocker).
var scenarioHeadingPattern = regexp.MustCompile(`(?i)^####\s*Scenario\b`)

type RequirementsContractStatus string

const (
	RequirementsContractStatusValid   RequirementsContractStatus = "valid"
	RequirementsContractStatusInvalid RequirementsContractStatus = "invalid"
	RequirementsContractStatusMissing RequirementsContractStatus = "missing"
)

type RequirementsContractResult struct {
	Status  RequirementsContractStatus
	Source  string
	Message string
}

func EvaluateRequirementsContract(bundleDir string) (RequirementsContractResult, error) {
	sourcePath := ResolveArtifactPath(bundleDir, "requirements.md")
	if _, err := os.Stat(sourcePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return RequirementsContractResult{
				Status:  RequirementsContractStatusMissing,
				Source:  sourcePath,
				Message: "requirements.md is missing",
			}, nil
		}
		return RequirementsContractResult{}, err
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return RequirementsContractResult{}, err
	}

	content := string(raw)
	requirementCount := len(ParseRequirementBlocks(content))
	if requirementCount == 0 {
		return RequirementsContractResult{
			Status:  RequirementsContractStatusInvalid,
			Source:  sourcePath,
			Message: "requirements.md is not well-formed: no Requirement blocks found",
		}, nil
	}

	missingStableIDs := RequirementBlocksMissingStableIDs(content)
	if len(missingStableIDs) > 0 {
		return RequirementsContractResult{
			Status: RequirementsContractStatusInvalid,
			Source: sourcePath,
			Message: fmt.Sprintf(
				"requirements.md is not well-formed: requirement blocks missing stable REQ-* IDs: %s",
				strings.Join(missingStableIDs, ", "),
			),
		}, nil
	}

	if substanceBlockers := RequirementSubstanceBlockers(content); len(substanceBlockers) > 0 {
		return RequirementsContractResult{
			Status: RequirementsContractStatusInvalid,
			Source: sourcePath,
			Message: fmt.Sprintf(
				"requirements.md is not substantive: %s",
				strings.Join(substanceBlockers, "; "),
			),
		}, nil
	}

	return RequirementsContractResult{
		Status:  RequirementsContractStatusValid,
		Source:  sourcePath,
		Message: fmt.Sprintf("requirements.md validated (%d requirements)", requirementCount),
	}, nil
}

// RequirementSubstanceBlockers returns substance problems in requirements.md: no
// Requirement blocks, template/seed placeholder content, a requirement body with
// no RFC-2119 MUST/SHALL/REQUIRED keyword, or a requirement without a concrete
// GIVEN/WHEN/THEN scenario. An empty slice means the requirements carry real
// substance. This is the gate that stops a mechanical scaffold from reaching
// done (issue #91): the engine owns structure, the authoring skill owns substance.
//
// Placeholder detection uses the requirements-specific LooksLikeRequirementsPlaceholder
// (not the broad LooksLikeTemplatePlaceholder) so a legitimately-authored
// requirement containing a generic phrase — e.g. a real requirement about cases
// in "pending investigation" status — is not false-flagged (issue #91: avoid
// false positives that block real work).
func RequirementSubstanceBlockers(content string) []string {
	content = stripRequirementGuidanceComments(content)
	var blockers []string
	if LooksLikeRequirementsPlaceholder(content) {
		blockers = append(blockers,
			"contains template/seed placeholder content; author concrete requirements")
	}
	blocks := splitRequirementBlocksForSubstance(content)
	if len(blocks) == 0 {
		blockers = append(blockers, "requirements.md declares no Requirement blocks; author concrete requirements")
	}
	for _, block := range blocks {
		label := block.stableID
		if label == "" {
			label = block.name
		}
		if !rfc2119Pattern.MatchString(block.statementBody) {
			blockers = append(blockers,
				fmt.Sprintf("requirement %s body has no RFC-2119 MUST/SHALL/REQUIRED keyword", label))
		}
		if !hasConcreteScenario(block) {
			blockers = append(blockers,
				fmt.Sprintf("requirement %s has no concrete #### Scenario (needs GIVEN/WHEN/THEN)", label))
		}
	}
	return blockers
}

type requirementSubstanceBlock struct {
	name          string
	stableID      string
	text          string // full block, including the heading line
	statementBody string // requirement statement region: after the heading, before the first scenario
	scenarioText  string // scenario region: from the first "#### Scenario" to the end of the block ("" if none)
}

func splitRequirementBlocksForSubstance(content string) []requirementSubstanceBlock {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	headings := requirementHeadingsIn(lines)
	blocks := make([]requirementSubstanceBlock, 0, len(headings))
	for i, heading := range headings {
		start := heading.line
		end := len(lines)
		if i+1 < len(headings) {
			end = headings[i+1].line
		}
		blockLines := lines[start:end]
		text := strings.Join(blockLines, "\n")

		// Split the block into the requirement-statement region (the REQ-* body,
		// which may span multiple lines) and the scenario region. The statement
		// ends at the first "#### Scenario" heading; everything from there is the
		// scenario region. RFC-2119 substance must appear in the statement, not in
		// a scenario line (issue #91 blocker).
		scenarioStart := -1
		for j := 1; j < len(blockLines); j++ {
			if scenarioHeadingPattern.MatchString(strings.TrimSpace(blockLines[j])) {
				scenarioStart = j
				break
			}
		}
		var statementBody, scenarioText string
		if scenarioStart >= 0 {
			statementBody = strings.Join(blockLines[1:scenarioStart], "\n")
			scenarioText = strings.Join(blockLines[scenarioStart:], "\n")
		} else {
			statementBody = strings.Join(blockLines[1:], "\n")
		}

		blocks = append(blocks, requirementSubstanceBlock{
			name:          heading.name,
			stableID:      firstRequirementID(text),
			text:          text,
			statementBody: statementBody,
			scenarioText:  scenarioText,
		})
	}
	return blocks
}

// scenarioSegments splits a requirement's scenario region into one segment per
// "#### Scenario" heading, so a scenario's GIVEN/WHEN/THEN completeness is judged
// within a single scenario rather than scattered across several.
func scenarioSegments(scenarioText string) []string {
	if strings.TrimSpace(scenarioText) == "" {
		return nil
	}
	lines := strings.Split(scenarioText, "\n")
	var segments []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			segments = append(segments, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range lines {
		if scenarioHeadingPattern.MatchString(strings.TrimSpace(line)) {
			flush()
		}
		cur = append(cur, line)
	}
	flush()
	return segments
}

// hasConcreteScenario reports whether a requirement has at least one real
// acceptance scenario: a single "#### Scenario" segment that carries non-empty
// GIVEN, WHEN, and THEN step lines and is not placeholder/tautology prose. The
// completeness check is per-scenario so GIVEN/WHEN/THEN scattered across
// separate scenarios does not count, and a placeholder scenario is rejected via
// the requirements-specific matcher (issue #91).
func hasConcreteScenario(block requirementSubstanceBlock) bool {
	for _, segment := range scenarioSegments(block.scenarioText) {
		if LooksLikeRequirementsPlaceholder(segment) {
			continue
		}
		if scenarioSegmentHasConcreteSteps(segment) {
			return true
		}
	}
	return false
}

func scenarioSegmentHasConcreteSteps(segment string) bool {
	return scenarioStepHasContent(segment, "GIVEN") &&
		scenarioStepHasContent(segment, "WHEN") &&
		scenarioStepHasContent(segment, "THEN")
}

func scenarioStepHasContent(segment, keyword string) bool {
	for _, line := range strings.Split(segment, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) <= len(keyword) {
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(trimmed), keyword) {
			continue
		}

		rest := trimmed[len(keyword):]
		switch rest[0] {
		case ' ', '\t', ':':
		default:
			continue
		}
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, ":") {
			rest = strings.TrimSpace(rest[1:])
		}
		if rest != "" {
			return true
		}
	}
	return false
}
