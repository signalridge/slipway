package cmd

import (
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/adapter"
	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
)

type changeReportOutput struct {
	ContractVersion int      `json:"contract_version"`
	Hosts           []string `json:"hosts"`
	Written         []string `json:"written"`
	Removed         []string `json:"removed"`
	Preserved       []string `json:"preserved"`
	Warnings        []string `json:"warnings"`
}

func makeChangeReportOutput(report adapter.ChangeReport) changeReportOutput {
	return changeReportOutput{
		ContractVersion: machineContractVersion,
		Hosts:           append([]string{}, report.Hosts...),
		Written:         append([]string{}, report.Written...),
		Removed:         append([]string{}, report.Removed...),
		Preserved:       append([]string{}, report.Preserved...),
		Warnings:        append([]string{}, report.Warnings...),
	}
}

func makeInstallCmd() *cobra.Command {
	var root string
	var tools []string
	var refresh bool
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Slipway capabilities for detected AI coding hosts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := resolveRoot(root)
			if err != nil {
				return err
			}
			report, err := adapter.Install(adapter.InstallOptions{Root: resolved, Tools: tools, Refresh: refresh})
			if err != nil {
				return newRuntimeError("install_failed", err.Error(), installNext(resolved, tools), nil)
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), makeChangeReportOutput(report))
			}
			return writeChangeReport(cmd, "Installed", report)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "workspace root (default: current directory)")
	cmd.Flags().StringSliceVar(&tools, "tool", nil, "host adapter to install; repeat or use all")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "refresh managed files owned by Slipway")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON")
	return cmd
}

func installNext(root string, tools []string) autopilot.Next {
	arguments := []string{"slipway", "install", "--root", root, "--refresh"}
	for _, tool := range tools {
		arguments = append(arguments, "--tool", tool)
	}
	return inputlessCommandNext(root, "refresh-install", arguments...)
}

func writeChangeReport(cmd *cobra.Command, verb string, report adapter.ChangeReport) error {
	w := cmd.OutOrStdout()
	if _, err := fmt.Fprintf(w, "%s capabilities for: %s\n", verb, strings.Join(report.Hosts, ", ")); err != nil {
		return err
	}
	for _, path := range report.Written {
		if _, err := fmt.Fprintf(w, "  wrote %s\n", path); err != nil {
			return err
		}
	}
	for _, path := range report.Removed {
		if _, err := fmt.Fprintf(w, "  removed %s\n", path); err != nil {
			return err
		}
	}
	for _, path := range report.Preserved {
		if _, err := fmt.Fprintf(w, "  preserved user-modified %s\n", path); err != nil {
			return err
		}
	}
	for _, warning := range report.Warnings {
		if _, err := fmt.Fprintf(w, "  warning: %s\n", warning); err != nil {
			return err
		}
	}
	return nil
}
