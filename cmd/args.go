package cmd

import "github.com/spf13/cobra"

func usageNoArgs(command *cobra.Command, args []string) error {
	if err := cobra.NoArgs(command, args); err != nil {
		return newUsageError("invalid_usage", err.Error(), defaultErrorNext())
	}
	return nil
}

func usageMaximumNArgs(max int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(max)(command, args); err != nil {
			return newUsageError("invalid_usage", err.Error(), defaultErrorNext())
		}
		return nil
	}
}

func usageExactArgs(count int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(count)(command, args); err != nil {
			return newUsageError("invalid_usage", err.Error(), defaultErrorNext())
		}
		return nil
	}
}
