package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type validateRequirementsView struct {
	Slug    string `json:"slug"`
	Source  string `json:"source"`
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

func makeValidateRequirementsCmd() *cobra.Command {
	var jsonOutput bool
	var changeSlug string

	cmd := &cobra.Command{
		Use:   "validate-requirements",
		Short: desc("validate-requirements"),
		Long: desc("validate-requirements") + ".\n\n" +
			"This command is read-only and verifies that the governed requirements.md exists and is well-formed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withBestEffortChangeStateLock(root, ref.Slug, "validate-requirements", func() error {
				view, err := runValidateRequirements(root, ref.Slug)
				if err != nil {
					return err
				}

				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}

				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef("Change: %s\n", view.Slug)
				writer.Writef("Valid: %t\n", view.Valid)
				writer.Writef("%s\n", view.Message)
				return writer.Err()
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	addChangeSelectorFlags(cmd, &changeSlug, "Governed change slug to validate (defaults to active change)")
	return cmd
}

func runValidateRequirements(root, slug string) (validateRequirementsView, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return validateRequirementsView{}, err
	}

	changeDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return validateRequirementsView{}, err
	}

	sourcePath := artifact.ResolveArtifactPath(changeDir, change.Slug, "requirements.md")
	_, err = os.Stat(sourcePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return validateRequirementsView{
				Slug:    change.Slug,
				Valid:   false,
				Message: "requirements.md is missing",
			}, nil
		}
		return validateRequirementsView{}, err
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return validateRequirementsView{}, err
	}
	requirementCount := len(artifact.ParseRequirementBlocks(string(raw)))
	if requirementCount == 0 {
		return validateRequirementsView{
			Slug:    change.Slug,
			Source:  sourcePath,
			Valid:   false,
			Message: "requirements.md is not well-formed: no Requirement blocks found",
		}, nil
	}
	missingStableIDs := artifact.RequirementBlocksMissingStableIDs(string(raw))
	if len(missingStableIDs) > 0 {
		return validateRequirementsView{
			Slug:   change.Slug,
			Source: sourcePath,
			Valid:  false,
			Message: fmt.Sprintf(
				"requirements.md is not well-formed: requirement blocks missing stable REQ-* IDs: %s",
				strings.Join(missingStableIDs, ", "),
			),
		}, nil
	}

	return validateRequirementsView{
		Slug:    change.Slug,
		Source:  sourcePath,
		Valid:   true,
		Message: fmt.Sprintf("requirements.md validated (%d requirements)", requirementCount),
	}, nil
}
