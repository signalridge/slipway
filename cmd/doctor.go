package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func makeDoctorCmd() *cobra.Command {
	return makeDoctorCmdWithRunner(systemDoctorRunner{})
}

func makeDoctorCmdWithRunner(runner doctorCommandRunner) *cobra.Command {
	var root string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose host installation and runtime availability",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := resolveRoot(root)
			if err != nil {
				return err
			}
			report, err := collectDoctorReport(cmd.Context(), resolved, runner)
			if err != nil {
				return newRuntimeError("doctor_failed", err.Error(), inputlessCommandNext(resolved, "retry-doctor", "slipway", "doctor", "--root", resolved), nil)
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), makeDoctorOutput(report))
			}
			for _, check := range report.Checks {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-7s %-10s %-28s %s\n", check.Status, check.HostID, check.Code, check.Detail); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "workspace root (default: current directory)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON")
	return cmd
}
