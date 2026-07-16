package cmd

import (
	"errors"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
)

func makeMachineCmd() *cobra.Command {
	var root string
	command := &cobra.Command{
		Use:    "_machine",
		Short:  "Versioned host-machine operations",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return newUsageError("machine_operation_required", "a machine operation is required", defaultErrorNext())
		},
	}
	command.PersistentFlags().StringVar(&root, "root", "", "workspace root (default: current Git worktree)")
	command.AddCommand(
		makeRunSubmitCmd(&root),
		makeRunAnswerCmd(&root),
		makeRunSkipCmd(&root),
		makeRunResumeCmd(&root),
		makeMachineMaterialCmd(&root),
	)
	return command
}

func makeMachineMaterialCmd(root *string) *cobra.Command {
	var runID string
	var actionID string
	var section string
	command := &cobra.Command{
		Use:    "material",
		Short:  "Read one locally pinned Action source chapter",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			next := autopilot.NoneNext("")
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", next)
			}
			if validationErr := validateRunIDArgument(*root, runID); validationErr != nil {
				return validationErr
			}
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", next)
			}
			if section == "" {
				return newUsageError("material_section_required", "section cannot be empty", next)
			}
			service, err := openAutopilotReadOnly(*root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			material, err := service.ReadActionMaterial(runID, actionID, section)
			if err != nil {
				return err
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
