package cmd

import "github.com/spf13/cobra"

func makeToolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Run Slipway helper tools",
	}
	cmd.AddCommand(makeMergeSARIFCmd())
	cmd.AddCommand(makePinActionsCmd())
	cmd.AddCommand(makeFindPolluterGoCmd())
	cmd.AddCommand(makeFindVariantCmd())
	cmd.AddCommand(makeFetchPRChecksCmd())
	cmd.AddCommand(makeFetchPRFeedbackCmd())
	cmd.AddCommand(makeFetchReviewRequestsCmd())
	cmd.AddCommand(makeReplyToThreadCmd())
	return cmd
}
