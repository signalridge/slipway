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

type syncView struct {
	Slug    string `json:"slug"`
	Source  string `json:"source"`
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

func makeSyncCmd() *cobra.Command {
	var jsonOutput bool
	var changeSlug string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Validate change requirements (read-only)",
		Long:  "Validate the change's requirements.md exists and is well-formed. Does not write any files.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withBestEffortChangeStateLock(root, ref.Slug, "sync", func() error {
				view, err := runArtifactSync(root, ref.Slug)
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
	addChangeSelectorFlags(cmd, &changeSlug, "Governed change slug to sync (defaults to active change)")
	return cmd
}

func runArtifactSync(root, slug string) (syncView, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return syncView{}, err
	}

	changeDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return syncView{}, err
	}

	// Source: the change's requirements.md (flat in bundle).
	sourcePath := artifact.ResolveArtifactPath(changeDir, change.Slug, "requirements.md")
	_, err = os.Stat(sourcePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return syncView{
				Slug:    change.Slug,
				Valid:   false,
				Message: "no requirements.md found; nothing to sync",
			}, nil
		}
		return syncView{}, err
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return syncView{}, err
	}
	requirementCount := len(artifact.ParseRequirementBlocks(string(raw)))
	if requirementCount == 0 {
		return syncView{
			Slug:    change.Slug,
			Source:  sourcePath,
			Valid:   false,
			Message: "requirements.md is not well-formed: no Requirement blocks found",
		}, nil
	}
	missingStableIDs := artifact.RequirementBlocksMissingStableIDs(string(raw))
	if len(missingStableIDs) > 0 {
		return syncView{
			Slug:   change.Slug,
			Source: sourcePath,
			Valid:  false,
			Message: fmt.Sprintf(
				"requirements.md is not well-formed: requirement blocks missing stable REQ-* IDs: %s",
				strings.Join(missingStableIDs, ", "),
			),
		}, nil
	}

	return syncView{
		Slug:    change.Slug,
		Source:  sourcePath,
		Valid:   true,
		Message: fmt.Sprintf("requirements.md validated (%d requirements)", requirementCount),
	}, nil
}
