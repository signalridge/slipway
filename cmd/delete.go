package cmd

import (
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type deleteView struct {
	Slug     string               `json:"slug"`
	Mode     string               `json:"mode"`
	DryRun   bool                 `json:"dry_run"`
	Executed bool                 `json:"executed"`
	Plan     []state.DeleteTarget `json:"plan,omitempty"`
	Removed  []state.DeleteTarget `json:"removed,omitempty"`
	Skipped  []state.DeleteTarget `json:"skipped,omitempty"`
}

func makeDeleteCmd() *cobra.Command {
	var changeSlug string
	var jsonOutput bool
	var removeWorktree bool
	var archived bool
	var yes bool
	var force bool
	cmd := &cobra.Command{
		Use:   "delete",
		Short: desc("delete"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			slug, err := resolveDeleteTargetSlug(root, changeSlug)
			if err != nil {
				return err
			}

			opts := state.DeleteOptions{
				RemoveWorktree:  removeWorktree,
				Archived:        archived,
				Force:           force,
				CurrentWorktree: currentWorktreeForDelete(),
			}

			return withChangeStateLock(root, slug, "delete", func() error {
				plan, err := state.BuildDeletePlan(root, slug, opts)
				if err != nil {
					return err
				}

				// Default (no --yes) is a non-destructive dry-run plan.
				if !yes {
					return emitDeleteView(cmd, deleteView{
						Slug:   plan.Slug,
						Mode:   string(plan.Mode),
						DryRun: true,
						Plan:   plan.Targets,
					}, jsonOutput)
				}

				// Fail closed: a refusal blocks the whole operation so nothing is
				// partially deleted.
				if plan.HasRefusals() {
					return deleteRefusedError(slug, plan)
				}
				if plan.NothingToDelete() {
					return emitDeleteView(cmd, deleteView{
						Slug:     plan.Slug,
						Mode:     string(plan.Mode),
						Executed: true,
						Skipped:  plan.Targets,
					}, jsonOutput)
				}

				result, err := state.ExecuteDeletePlan(root, plan, opts)
				if err != nil {
					return err
				}
				return emitDeleteView(cmd, deleteView{
					Slug:     result.Slug,
					Mode:     string(result.Mode),
					Executed: true,
					Removed:  result.Removed,
					Skipped:  result.Skipped,
				}, jsonOutput)
			})
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().BoolVar(&removeWorktree, "worktree", false, "Also remove the bound git worktree (refused if dirty unless --force)")
	cmd.Flags().BoolVar(&archived, "archived", false, "Purge an archived terminal record instead of active governed state")
	cmd.Flags().BoolVar(&yes, "yes", false, "Execute the deletion; without it `delete` prints a dry-run plan and removes nothing")
	cmd.Flags().BoolVar(&force, "force", false, "Override the uncommitted-tracked-changes refusal when removing a worktree")
	return cmd
}

// resolveDeleteTargetSlug resolves the change slug to delete. An explicit
// --change is used as-is, because delete intentionally operates on abandoned or
// broken changes that may no longer resolve as the active change. Without
// --change it resolves the active change and surfaces the standard actionable
// resolution errors (no_active_change, change_bound_to_other_worktree, ...).
func resolveDeleteTargetSlug(root, explicitSlug string) (string, error) {
	if s := strings.TrimSpace(explicitSlug); s != "" {
		return s, nil
	}
	ref, err := resolveActiveChangeRef(root, "")
	if err != nil {
		return "", err
	}
	return ref.Slug, nil
}

// currentWorktreeForDelete returns the worktree root the command is running
// inside, or "" when it cannot be determined (the guard then simply does not
// fire). Resolution failures are surfaced earlier via slug resolution.
func currentWorktreeForDelete() string {
	current, err := currentWorktreeRoot()
	if err != nil {
		return ""
	}
	return current
}

func deleteRefusedError(slug string, plan state.DeletePlan) error {
	refusals := plan.Refusals()
	reasons := make([]string, 0, len(refusals))
	for _, target := range refusals {
		reasons = append(reasons, fmt.Sprintf("%s: %s", target.Kind, target.Reason))
	}
	return newPreconditionError(
		"delete_refused",
		fmt.Sprintf("delete refused for %q: %s", slug, strings.Join(reasons, "; ")),
		"Resolve the refusal (commit or stash worktree changes, pass --force, or re-run from the repository root), then retry.",
		slug,
		map[string]any{
			"refusals": refusals,
		},
	)
}

func emitDeleteView(cmd *cobra.Command, view deleteView, jsonOutput bool) error {
	if jsonOutput {
		return encodeJSONResponse(cmd, view)
	}
	writer := newFormatWriter(cmd.OutOrStdout())
	if view.DryRun {
		writer.Writef("Dry-run delete plan for %s (mode: %s)\n", view.Slug, view.Mode)
		writeDeleteTargets(writer, view.Plan)
		writer.Writef("Nothing deleted. Re-run with --yes to execute.\n")
		return writer.Err()
	}
	writer.Writef("Deleted %s (mode: %s)\n", view.Slug, view.Mode)
	if len(view.Removed) > 0 {
		writer.Writef("Removed:\n")
		writeDeleteTargets(writer, view.Removed)
	}
	if len(view.Skipped) > 0 {
		writer.Writef("Skipped:\n")
		writeDeleteTargets(writer, view.Skipped)
	}
	return writer.Err()
}

func writeDeleteTargets(writer *formatWriter, targets []state.DeleteTarget) {
	for _, target := range targets {
		path := target.Path
		if strings.TrimSpace(path) == "" {
			path = "(none)"
		}
		line := fmt.Sprintf("  - [%s] %s %s", target.Action, target.Kind, path)
		if strings.TrimSpace(target.Reason) != "" {
			line += fmt.Sprintf(" (%s)", target.Reason)
		}
		writer.Writef("%s\n", line)
	}
}
