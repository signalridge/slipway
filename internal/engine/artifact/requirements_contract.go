package artifact

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

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

func EvaluateRequirementsContract(bundleDir, slug string) (RequirementsContractResult, error) {
	sourcePath := ResolveArtifactPath(bundleDir, slug, "requirements.md")
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

	return RequirementsContractResult{
		Status:  RequirementsContractStatusValid,
		Source:  sourcePath,
		Message: fmt.Sprintf("requirements.md validated (%d requirements)", requirementCount),
	}, nil
}
