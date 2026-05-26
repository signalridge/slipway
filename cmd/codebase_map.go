package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type codebaseMapView struct {
	ExecutionMode    string            `json:"execution_mode"`
	CodebaseMapDir   string            `json:"codebase_map_dir"`
	CodebaseMapDocs  map[string]string `json:"codebase_map_docs"`
	Status           string            `json:"status"`
	DocStates        map[string]string `json:"doc_states,omitempty"`
	MissingDocs      []string          `json:"missing_docs,omitempty"`
	ScaffoldOnlyDocs []string          `json:"scaffold_only_docs,omitempty"`
	PopulatedDocs    []string          `json:"populated_docs,omitempty"`
	Created          []string          `json:"created,omitempty"`
}

func makeCodebaseMapCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "codebase-map",
		Short: desc("codebase-map"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}

			created, err := artifact.EnsureCodebaseMapDocs(root)
			if err != nil {
				return err
			}
			assessment, err := artifact.AssessCodebaseMapDocs(root)
			if err != nil {
				return err
			}
			displayRoot := root
			dir := state.CodebaseMapDir(root)
			view := codebaseMapView{
				ExecutionMode:    "advisory",
				CodebaseMapDir:   state.DisplayPath(displayRoot, dir),
				CodebaseMapDocs:  artifact.CodebaseMapDisplayDocs(displayRoot, dir),
				Status:           assessment.Status,
				DocStates:        assessment.DocStates,
				MissingDocs:      assessment.MissingDocs,
				ScaffoldOnlyDocs: assessment.ScaffoldOnlyDocs,
				PopulatedDocs:    assessment.PopulatedDocs,
			}
			if len(created) > 0 {
				view.Created = make([]string, 0, len(created))
				for _, path := range created {
					view.Created = append(view.Created, state.DisplayPath(displayRoot, path))
				}
			}

			if jsonOutput {
				return encodeJSONResponse(cmd, view)
			}
			return writeCodebaseMapText(cmd.OutOrStdout(), view)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	return cmd
}

func writeCodebaseMapText(w io.Writer, view codebaseMapView) error {
	writer := newFormatWriter(w)
	writer.Writef("Mode: %s\n", view.ExecutionMode)
	writer.Writef("Codebase Map: %s\n", view.CodebaseMapDir)
	if view.Status != "" {
		writer.Writef("Status: %s\n", view.Status)
	}
	if len(view.ScaffoldOnlyDocs) > 0 {
		writer.Writef("Scaffold-only:\n")
		for _, name := range view.ScaffoldOnlyDocs {
			writer.Writef("  - %s\n", name)
		}
	}
	if len(view.Created) > 0 {
		writer.Writef("Created:\n")
		for _, path := range view.Created {
			writer.Writef("  - %s\n", path)
		}
	}
	if len(view.CodebaseMapDocs) > 0 {
		keys := []string{"stack", "integrations", "architecture", "structure", "conventions", "testing", "concerns"}
		writer.Writef("Documents:\n")
		for _, key := range keys {
			path := strings.TrimSpace(view.CodebaseMapDocs[key])
			if path == "" {
				continue
			}
			writer.Writef("  %-13s %s\n", fmt.Sprintf("%s:", key), path)
		}
	}
	return writer.Err()
}
