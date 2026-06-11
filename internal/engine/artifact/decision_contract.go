package artifact

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"unicode"

	"github.com/signalridge/slipway/internal/stringutil"
)

// DecisionContractStatus mirrors RequirementsContractStatus / TasksContractStatus
// for decision.md.
type DecisionContractStatus string

const (
	DecisionContractStatusValid   DecisionContractStatus = "valid"
	DecisionContractStatusInvalid DecisionContractStatus = "invalid"
	DecisionContractStatusMissing DecisionContractStatus = "missing"
)

// DecisionContractResult is the result of evaluating decision.md substance.
type DecisionContractResult struct {
	Status  DecisionContractStatus
	Source  string
	Message string
}

// ParsedDecisionContract is the machine-readable subset of decision.md used by
// readiness checks and next-skill constraints.
type ParsedDecisionContract struct {
	Status         string
	StatusExplicit bool
	StatusKnown    bool
	StatusRejected bool
	Decisions      []string
	StatusBlockers []string
}

var liveDecisionStatuses = map[string]struct{}{
	"accepted": {},
	"approved": {},
	"proposed": {},
	"draft":    {},
	"active":   {},
}

var rejectedDecisionStatuses = map[string]struct{}{
	"superseded": {},
	"deprecated": {},
	"rejected":   {},
}

var decisionStatusLabels = map[string]struct{}{
	"status":    {},
	"state":     {},
	"lifecycle": {},
	"stage":     {},
}

// EvaluateDecisionContract checks decision.md for substance, not just presence: a
// decision whose required sections are missing, structurally empty, or still the
// unwritten template (only the <!-- ... --> guidance comment) is the scaffold the
// authoring skill must replace before planning is ready (issue #119). The engine
// owns structure (the five required sections defined in schemas.yaml); the
// authoring skill owns substance.
func EvaluateDecisionContract(bundleDir string) (DecisionContractResult, error) {
	sourcePath := ResolveArtifactPath(bundleDir, "decision.md")
	if _, err := os.Stat(sourcePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DecisionContractResult{
				Status:  DecisionContractStatusMissing,
				Source:  sourcePath,
				Message: "decision.md is missing",
			}, nil
		}
		return DecisionContractResult{}, err
	}

	raw, err := os.ReadFile(sourcePath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return DecisionContractResult{}, err
	}

	if blockers := DecisionSubstanceBlockers(string(raw)); len(blockers) > 0 {
		return DecisionContractResult{
			Status:  DecisionContractStatusInvalid,
			Source:  sourcePath,
			Message: fmt.Sprintf("decision.md is not substantive: %s", strings.Join(blockers, "; ")),
		}, nil
	}

	return DecisionContractResult{
		Status:  DecisionContractStatusValid,
		Source:  sourcePath,
		Message: "decision.md validated",
	}, nil
}

// DecisionSubstanceBlockers returns substance problems in decision.md. An empty
// slice means every required section carries authored content.
//
// The engine owns structure (the five required sections from schemas.yaml) and a
// deterministic placeholder floor; the authoring skill owns substance. A
// decision.md that omits a required section, leaves one structurally empty, or
// leaves one as the unwritten template (only the <!-- ... --> guidance comment)
// is rejected so an unedited `instructions decision` scaffold cannot reach
// planning readiness (issue #119). The floor derives from the template's own
// comment-only section bodies — stripping HTML comments leaves an unedited
// section empty — so detection cannot drift from the template wording.
func DecisionSubstanceBlockers(content string) []string {
	headings := requiredSectionsForArtifact("decision.md")
	if len(headings) == 0 {
		return []string{"decision_structure_invalid:no required sections configured for decision.md"}
	}

	if err := validateSectionStructure(content, headings); err != nil {
		return []string{"decision_structure_invalid:" + err.Error()}
	}

	var blockers []string
	for _, heading := range headings {
		body := strings.Join(markdownSectionLines(content, heading), "\n")
		if artifactSectionBodyLooksPlaceholder(body) {
			blockers = append(blockers, "decision_section_placeholder:"+heading)
		}
	}
	blockers = append(blockers, ParseDecisionContract(content).StatusBlockers...)
	return blockers
}

// ParseDecisionContract parses the decision's selected text and optional
// lifecycle status. A missing status stays compatible with existing decision.md
// files; an explicit unknown or rejected status fails closed.
func ParseDecisionContract(content string) ParsedDecisionContract {
	status, explicit := parseDecisionStatus(content)
	known := true
	rejected := ShouldRejectDecisionStatus(status)
	var blockers []string

	if explicit {
		switch {
		case rejected:
			blockers = append(blockers, "decision_status_rejected:"+status)
		case status == "":
			known = false
			blockers = append(blockers, "decision_status_unknown:empty")
		case !isKnownDecisionStatus(status):
			known = false
			blockers = append(blockers, "decision_status_unknown:"+status)
		}
	}

	return ParsedDecisionContract{
		Status:         status,
		StatusExplicit: explicit,
		StatusKnown:    known,
		StatusRejected: rejected,
		Decisions:      parseDecisionSelectedItems(content),
		StatusBlockers: blockers,
	}
}

// ShouldRejectDecisionStatus reports whether a status explicitly marks a
// decision as unusable planning authority.
func ShouldRejectDecisionStatus(status string) bool {
	_, ok := rejectedDecisionStatuses[canonicalDecisionStatus(status)]
	return ok
}

func parseDecisionStatus(content string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	inSection := false
	sectionHasEntry := false
	statusExplicit := false
	selectedStatus := ""
	selectedKnown := true

	finishSection := func() {
		if inSection && !sectionHasEntry {
			selectedStatus, selectedKnown = selectDecisionStatus(selectedStatus, selectedKnown, "", false)
		}
		inSection = false
		sectionHasEntry = false
	}

	for _, line := range lines {
		if label, ok := markdownH2Label(line); ok {
			finishSection()
			inSection = isDecisionStatusHeading(label)
			if inSection {
				statusExplicit = true
			}
			continue
		}
		if !inSection {
			continue
		}
		entry := strings.TrimSpace(strings.TrimLeft(line, "-*+ "))
		if entry == "" {
			continue
		}
		candidate := canonicalDecisionStatus(entry)
		selectedStatus, selectedKnown = selectDecisionStatus(
			selectedStatus,
			selectedKnown,
			candidate,
			candidate != "" && isKnownDecisionStatus(candidate),
		)
		sectionHasEntry = true
	}
	finishSection()
	return selectedStatus, statusExplicit
}

func isDecisionStatusHeading(heading string) bool {
	_, ok := decisionStatusLabels[normalizeDecisionHeadingLabel(heading)]
	return ok
}

func selectDecisionStatus(current string, currentKnown bool, candidate string, candidateKnown bool) (string, bool) {
	if ShouldRejectDecisionStatus(current) {
		return current, true
	}
	if ShouldRejectDecisionStatus(candidate) {
		return candidate, true
	}
	if !currentKnown {
		return current, false
	}
	if !candidateKnown {
		return candidate, false
	}
	if current == "" {
		return candidate, true
	}
	return current, true
}

func markdownH2Label(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "###") {
		return "", false
	}
	if len(trimmed) == len("##") || !unicode.IsSpace(rune(trimmed[len("##")])) {
		return "", false
	}
	label := strings.TrimSpace(trimmed[len("##"):])
	label = strings.TrimSpace(strings.TrimRight(label, "#"))
	if label == "" {
		return "", false
	}
	return label, true
}

func normalizeDecisionHeadingLabel(heading string) string {
	label := strings.TrimSpace(heading)
	if parsed, ok := markdownH2Label(label); ok {
		label = parsed
	}
	return normalizeDecisionStatus(label)
}

func canonicalDecisionStatus(status string) string {
	normalized := normalizeDecisionStatus(status)
	if normalized == "" {
		return normalized
	}

	tokens := strings.Fields(normalized)
	if rejected := firstMatchingDecisionStatus(tokens, rejectedDecisionStatuses); rejected != "" {
		return rejected
	}

	statusTokens := trimDecisionStatusLabel(tokens)
	if len(statusTokens) == 0 {
		return normalized
	}
	if _, ok := liveDecisionStatuses[statusTokens[0]]; ok {
		return statusTokens[0]
	}
	return strings.Join(statusTokens, " ")
}

func normalizeDecisionStatus(status string) string {
	stripped := stringutil.StripHTMLComments(status)
	var b strings.Builder
	previousSpace := false
	for _, r := range strings.ToLower(stripped) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			previousSpace = false
		default:
			if previousSpace {
				continue
			}
			b.WriteByte(' ')
			previousSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func firstMatchingDecisionStatus(tokens []string, statuses map[string]struct{}) string {
	for _, token := range tokens {
		if _, ok := statuses[token]; ok {
			return token
		}
	}
	return ""
}

func trimDecisionStatusLabel(tokens []string) []string {
	if len(tokens) <= 1 {
		return tokens
	}
	if _, ok := decisionStatusLabels[tokens[0]]; ok {
		return tokens[1:]
	}
	return tokens
}

func isKnownDecisionStatus(status string) bool {
	if status == "" {
		return true
	}
	if _, ok := liveDecisionStatuses[status]; ok {
		return true
	}
	_, ok := rejectedDecisionStatuses[status]
	return ok
}

func parseDecisionSelectedItems(content string) []string {
	var decisions []string

	_, selected := parseResearchAlternatives(markdownSectionLines(content, "Alternatives Considered"))
	if selected != "" && !LooksLikeTemplatePlaceholder(selected) {
		decisions = append(decisions, "Selected Direction: "+selected)
	}

	approachLines := markdownSectionLines(content, "Selected Approach")
	approach := strings.TrimSpace(strings.Join(approachLines, "\n"))
	if approach != "" && !LooksLikeTemplatePlaceholder(approach) {
		decisions = append(decisions, "Selected Approach: "+approach)
	}

	if len(decisions) == 0 {
		return nil
	}
	return decisions
}
