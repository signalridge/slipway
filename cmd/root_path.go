package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func makeRootPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "root",
		Short:  "Print the canonical slipway scope root",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), root)
			return err
		},
	}
}
