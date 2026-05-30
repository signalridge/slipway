package cmd

import (
	"strings"

	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

func applyCommandInvocationWorkspacePath(cmd *cobra.Command, root string, diagnostics *state.ExecutionFreshnessDiagnostics) {
	if diagnostics == nil || diagnostics.PathAuthority == nil {
		return
	}
	workspace := invocationWorkspaceRootFromCommand(cmd, root)
	if strings.TrimSpace(workspace) == "" {
		return
	}
	diagnostics.PathAuthority.InvocationWorkspacePath = state.DisplayPath(root, workspace)
}

func applyNextInvocationWorkspacePath(cmd *cobra.Command, root string, view *nextView) {
	if view == nil {
		return
	}
	applyCommandInvocationWorkspacePath(cmd, root, view.FreshnessDiagnostics)
}

func applyValidateInvocationWorkspacePath(cmd *cobra.Command, root string, view *validateView) {
	if view == nil {
		return
	}
	applyCommandInvocationWorkspacePath(cmd, root, view.FreshnessDiagnostics)
}

func applyStatusInvocationWorkspacePath(cmd *cobra.Command, root string, view *statusView) {
	if view == nil {
		return
	}
	applyCommandInvocationWorkspacePath(cmd, root, view.FreshnessDiagnostics)
}

func attachFreshnessDiagnostics(diagnostics state.ExecutionFreshnessDiagnostics) *state.ExecutionFreshnessDiagnostics {
	if strings.TrimSpace(diagnostics.Status) == "" {
		return nil
	}
	if diagnostics.Status != "unknown" || diagnostics.PathAuthority != nil {
		return &diagnostics
	}
	return nil
}

func applyRepairInvocationWorkspacePath(cmd *cobra.Command, root string, summary *repairSummary) {
	if summary == nil || len(summary.PathAuthority) == 0 {
		return
	}
	workspace := invocationWorkspaceRootFromCommand(cmd, root)
	if strings.TrimSpace(workspace) == "" {
		return
	}
	display := state.DisplayPath(root, workspace)
	for _, authority := range summary.PathAuthority {
		if authority != nil {
			authority.InvocationWorkspacePath = display
		}
	}
}
