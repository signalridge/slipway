package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/spf13/cobra"
)

// instructionsArtifacts maps a public artifact name to its embedded template
// file name. The engine owns structure (these templates); the authoring skill
// owns substance. `slipway instructions <artifact>` serves a rendered exemplar
// plus the quality bar so a skill reads it before writing (issue #91).
var instructionsArtifacts = map[string]string{
	"intent":       "intent.md",
	"requirements": "requirements.md",
	"tasks":        "tasks.md",
	"decision":     "decision.md",
	"research":     "research.md",
	"assurance":    "assurance.md",
}

type instructionsView struct {
	Artifact string `json:"artifact"`
	Guidance string `json:"guidance"`
	Template string `json:"template"`
}

func instructionsArtifactNames() []string {
	names := make([]string, 0, len(instructionsArtifacts))
	for name := range instructionsArtifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func instructionsGuidance(name string) string {
	switch name {
	case "requirements":
		return "Author each requirement as `### Requirement: <title>` followed by a `REQ-` " +
			"body that states what the system MUST, SHALL, or is REQUIRED to do (an RFC-2119 " +
			"strong-obligation keyword), and at least one concrete `#### Scenario:` with real " +
			"GIVEN/WHEN/THEN lines. The engine seeds an obviously-not-real placeholder; replace " +
			"it — an unedited scaffold is rejected by the requirements substance gate and cannot " +
			"reach done."
	case "tasks":
		return "Author each task as a checklist line (- [ ] t-NN <objective>) with wave, " +
			"depends_on, target_files, task_kind, and covers metadata. Replace the placeholder " +
			"objective; a placeholder tasks list is rejected by the tasks substance gate."
	default:
		return "Replace the seeded placeholder with concrete, substantive content. The engine " +
			"owns structure; you own substance."
	}
}

func makeInstructionsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "instructions <artifact>",
		Short: desc("instructions"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.ToLower(strings.TrimSpace(args[0]))
			templateName, ok := instructionsArtifacts[name]
			if !ok {
				return newInvalidUsageError(
					"unknown_artifact",
					fmt.Sprintf("unknown artifact %q", name),
					"Choose a governed artifact: "+strings.Join(instructionsArtifactNames(), ", "),
					nil,
				)
			}
			content, err := artifact.RenderArtifactExample(templateName)
			if err != nil {
				return err
			}
			view := instructionsView{
				Artifact: name,
				Guidance: instructionsGuidance(name),
				Template: content,
			}
			if jsonOutput {
				return encodeJSONResponse(cmd, view)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "# Authoring instructions: %s\n\n", name)
			fmt.Fprintln(out, view.Guidance)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "## Template")
			fmt.Fprintln(out)
			fmt.Fprintln(out, content)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	return cmd
}
