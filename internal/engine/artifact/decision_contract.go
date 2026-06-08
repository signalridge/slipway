package artifact

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
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

	raw, err := os.ReadFile(sourcePath)
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
	return blockers
}
