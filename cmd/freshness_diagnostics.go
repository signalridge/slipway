package cmd

import (
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

func applyCommandInvocationWorkspacePath(cmd *cobra.Command, root string, diagnostics *state.ExecutionFreshnessDiagnostics) {
	applyCommandInvocationWorkspacePathWithReadContext(cmd, newStateReadContext(root), diagnostics)
}

func applyCommandInvocationWorkspacePathWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, diagnostics *state.ExecutionFreshnessDiagnostics) {
	if diagnostics == nil || diagnostics.PathAuthority == nil {
		return
	}
	workspace := invocationWorkspaceRootFromCommandWithReadContext(cmd, readCtx)
	if strings.TrimSpace(workspace) == "" {
		return
	}
	diagnostics.PathAuthority.InvocationWorkspacePath = state.DisplayPath(readCtx.root, workspace)
}

func applyNextInvocationWorkspacePath(cmd *cobra.Command, root string, view *nextView) {
	applyNextInvocationWorkspacePathWithReadContext(cmd, newStateReadContext(root), view)
}

func applyNextInvocationWorkspacePathWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, view *nextView) {
	if view == nil {
		return
	}
	applyCommandInvocationWorkspacePathWithReadContext(cmd, readCtx, view.FreshnessDiagnostics)
}

func applyNextInvocationRouteWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, change model.Change, explicitChange bool, view *nextView) {
	if view == nil {
		return
	}
	view.InvocationRoute = commandInvocationRouteWithReadContext(cmd, readCtx, change, explicitChange)
}

func applyValidateInvocationWorkspacePathWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, view *validateView) {
	if view == nil {
		return
	}
	applyCommandInvocationWorkspacePathWithReadContext(cmd, readCtx, view.FreshnessDiagnostics)
}

func applyValidateInvocationRouteWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, change model.Change, explicitChange bool, view *validateView) {
	if view == nil {
		return
	}
	view.InvocationRoute = commandInvocationRouteWithReadContext(cmd, readCtx, change, explicitChange)
}

func applyStatusInvocationWorkspacePathWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, view *statusView) {
	if view == nil {
		return
	}
	applyCommandInvocationWorkspacePathWithReadContext(cmd, readCtx, view.FreshnessDiagnostics)
}

func applyStatusInvocationRoute(cmd *cobra.Command, root string, change model.Change, explicitChange bool, view *statusView) {
	applyStatusInvocationRouteWithReadContext(cmd, newStateReadContext(root), change, explicitChange, view)
}

func applyStatusInvocationRouteWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, change model.Change, explicitChange bool, view *statusView) {
	if view == nil {
		return
	}
	view.InvocationRoute = commandInvocationRouteWithReadContext(cmd, readCtx, change, explicitChange)
}

func commandInvocationRoute(cmd *cobra.Command, root string, change model.Change, explicitChange bool) *invocationRouteView {
	return commandInvocationRouteWithReadContext(cmd, newStateReadContext(root), change, explicitChange)
}

func commandInvocationRouteWithReadContext(cmd *cobra.Command, readCtx *stateReadContext, change model.Change, explicitChange bool) *invocationRouteView {
	workspace := invocationWorkspaceRootFromCommandWithReadContext(cmd, readCtx)
	return buildInvocationRouteViewWithReadContext(readCtx, change, workspace, explicitChange)
}

func invocationWorkspaceRootFromCommandWithReadContext(cmd *cobra.Command, readCtx *stateReadContext) string {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		return root
	}
	return readCtx.invocationWorkspace()
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
