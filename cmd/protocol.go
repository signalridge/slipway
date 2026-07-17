package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func makeProtocolCmd() *cobra.Command {
	var root string
	command := &cobra.Command{
		Use:   "protocol",
		Short: "Machine protocol operations that generated adapters call to drive a Run",
		// Public on purpose: these operations are the published machine-protocol
		// contract, not an implementation detail, so the CLI must not present
		// them as one. They stay a distinct group because their caller is a
		// generated adapter rather than a person: each needs a Run and Action it
		// can only learn from the Action it was handed, and every response
		// already carries the exact next command to run.
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return newUsageError("protocol_operation_required", "a protocol operation is required", defaultErrorNext())
		},
	}
	command.PersistentFlags().StringVar(&root, "root", "", "workspace root (default: current Git worktree)")
	command.AddCommand(
		makeRunSubmitCmd(&root),
		makeRunAnswerCmd(&root),
		makeRunSkipCmd(&root),
		makeRunResumeCmd(&root),
		makeProtocolMaterialCmd(&root),
	)
	return command
}

func makeProtocolMaterialCmd(root *string) *cobra.Command {
	var runID string
	var actionID string
	var section string
	command := &cobra.Command{
		Use:   "material",
		Short: "Read one locally pinned Action source chapter",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", statusInspectionNextForRawRoot(*root, ""))
			}
			if validationErr := validateRunIDArgument(*root, runID); validationErr != nil {
				return validationErr
			}
			recoveryNext := statusInspectionNextForRawRoot(*root, runID)
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", recoveryNext)
			}
			if section == "" {
				return newUsageError("material_section_required", "section cannot be empty", recoveryNext)
			}
			service, err := openAutopilotReadOnly(*root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			material, err := service.ReadActionMaterial(runID, actionID, section)
			if err != nil {
				return withCLIErrorContext(err, service.RepositoryRoot(), runID)
			}
			if err := writeJSON(command.OutOrStdout(), material); err != nil {
				return errors.New("write action material: " + err.Error())
			}
			return nil
		},
	}
	command.Flags().StringVar(&runID, "run", "", "Run ID")
	command.Flags().StringVar(&actionID, "action", "", "Action ID")
	command.Flags().StringVar(&section, "section", "", "source section key")
	return command
}
