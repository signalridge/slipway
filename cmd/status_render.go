package cmd

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func printStatusView(cmd *cobra.Command, root string, view statusView, format string, hydrate bool) error {
	if hydrate && format != "text" {
		return newInvalidUsageError(
			"mutually_exclusive_flags",
			"`--hydrate` requires text output (remove `--json`/`--format`).",
			"Drop `--json`/`--format` to emit hydrate bodies, or omit `--hydrate`.",
			nil,
		)
	}
	switch format {
	case "json":
		return encodeJSONResponse(cmd, view)
	case "yaml":
		return yaml.NewEncoder(cmd.OutOrStdout()).Encode(view)
	default:
		if err := writeStatusText(cmd.OutOrStdout(), view); err != nil {
			return err
		}
		if hydrate {
			return emitHydrateBlocks(root, cmd.OutOrStdout(), view.HydrateReferences)
		}
		return nil
	}
}

func renderStatusText(view statusView) string {
	var builder strings.Builder
	writeLine := func(format string, args ...any) {
		_, _ = fmt.Fprintf(&builder, format, args...)
	}

	if view.ExecutionMode == "diagnostics" {
		writeLine("Mode: diagnostics\n")
		if strings.TrimSpace(view.Mode) != "" {
			writeLine("Focus: %s\n", view.Mode)
		}
		writer := newFormatWriter(&builder)
		writeHydrateLine(writer, "", view.HydrateReferences)
		if writer.Err() != nil {
			return builder.String()
		}
		writeLine("Evidence Freshness: %s\n", view.EvidenceFreshness)
		for _, d := range view.Diagnostics {
			writeLine("  %s\n", d)
		}
		return builder.String()
	}

	label := view.Slug
	writeLine("# %s\n\n", label)
	writeLine("Phase: %s | Mode: %s | Status: %s\n", view.Phase, view.ExecutionMode, view.LifecycleStatus)
	if strings.TrimSpace(view.Mode) != "" {
		writeLine("Focus: %s\n", view.Mode)
	}
	writer := newFormatWriter(&builder)
	writeHydrateLine(writer, "", view.HydrateReferences)
	if writer.Err() != nil {
		return builder.String()
	}
	if view.QualityMode != "" {
		writeLine("Quality: %s | Discovery Required: %t\n", view.QualityMode, view.NeedsDiscovery)
	}
	for _, line := range renderWorkflowPresetLines(workflowPresetView{
		WorkflowPreset:            view.WorkflowPreset,
		SuggestedWorkflowPreset:   view.SuggestedWorkflowPreset,
		EffectiveWorkflowPreset:   view.EffectiveWorkflowPreset,
		PresetConfirmationPending: view.PresetConfirmationPending,
		PresetUpgradeReasons:      view.PresetUpgradeReasons,
		GovernanceForecast:        view.GovernanceForecast,
	}) {
		writeLine("%s\n", line)
	}
	writeLine("State: %s\n", workflowStateLabel(view.CurrentState, view.IntakeSubStep, view.PlanSubStep))
	if view.PlanningNote != "" {
		writeLine("Planning Note: %s\n", view.PlanningNote)
	}
	if view.InterruptedExecutionAt != "" {
		writeLine("Interrupted Execution: %s\n", view.InterruptedExecutionAt)
	}
	if view.Narrative != "" {
		writeLine("Narrative: %s\n", view.Narrative)
	}

	if view.Progress != nil {
		bar := progressBar(view.Progress.Percentage, 20)
		writeLine("Progress: %s %d%%  (stage %d/%d: %s)\n",
			bar, view.Progress.Percentage,
			view.Progress.StageIndex+1, view.Progress.StageTotal,
			view.Progress.StageName)
		if view.Progress.TotalWaves > 0 {
			writeLine("Waves: %d/%d complete", view.Progress.CompletedWaves, view.Progress.TotalWaves)
			if view.Progress.CurrentWaveIndex > 0 {
				writeLine("  (resume from wave %d)", view.Progress.CurrentWaveIndex)
			}
			if len(view.Progress.WavesByVerdict) > 0 {
				parts := make([]string, 0, len(view.Progress.WavesByVerdict))
				for verdict, count := range view.Progress.WavesByVerdict {
					parts = append(parts, fmt.Sprintf("%s=%d", verdict, count))
				}
				slices.Sort(parts)
				writeLine("  [%s]", strings.Join(parts, ", "))
			}
			writeLine("\n")
		}
		if view.Progress.TasksTotal > 0 {
			writeLine("Tasks: %d/%d completed", view.Progress.TasksCompleted, view.Progress.TasksTotal)
			if len(view.Progress.TasksByVerdict) > 0 {
				parts := make([]string, 0, len(view.Progress.TasksByVerdict))
				for verdict, count := range view.Progress.TasksByVerdict {
					parts = append(parts, fmt.Sprintf("%s=%d", verdict, count))
				}
				slices.Sort(parts)
				writeLine("  [%s]", strings.Join(parts, ", "))
			}
			writeLine("\n")
		}
	}

	writeLine("Evidence Freshness: %s\n", view.EvidenceFreshness)

	if view.GovernanceSignals != nil {
		writeLine("\nDetected Signals:\n")
		if len(view.GovernanceSignals.Domains) > 0 {
			writeLine("  Domains:       [%s]\n", strings.Join(view.GovernanceSignals.Domains, ", "))
		}
		writeLine("  Blast Radius:  %s\n", view.GovernanceSignals.BlastRadius)
	}

	if len(view.ActiveControls) > 0 {
		writeLine("\nActive Controls:\n")
		for _, ctrl := range view.ActiveControls {
			writeLine("  - %s (%s / %s)\n", ctrl.ControlID, ctrl.Mode, ctrl.Scope)
		}
	}

	if len(view.RequiredActions) > 0 {
		writeLine("\nRequired Actions:\n")
		for _, a := range view.RequiredActions {
			mark := " "
			if a.Satisfied {
				mark = "x"
			}
			writeLine("  [%s] %s: %s\n", mark, a.ControlID, a.Description)
		}
	}

	if len(view.SummaryBlockers) > 0 {
		writeLine("\nExecution Summary Issues:\n")
		for _, blocker := range renderReasonCodeLines(view.SummaryBlockers) {
			writeLine("  - %s\n", blocker)
		}
	}

	if blockers := visibleStatusBlockers(view); len(blockers) > 0 {
		writeLine("\nBlockers:\n")
		for _, blocker := range renderReasonCodeLines(blockers) {
			writeLine("  - %s\n", blocker)
		}
	}

	if len(view.AutoPassedStates) > 0 {
		writeLine("\nAuto-Passed States:\n")
		for _, state := range view.AutoPassedStates {
			writeLine("  - %s (%s)\n", state.State, state.Reason)
		}
	}

	if len(view.GateStatus) > 0 {
		writeLine("\nGates:\n")
		gateIDs := make([]string, 0, len(view.GateStatus))
		for id := range view.GateStatus {
			gateIDs = append(gateIDs, id)
		}
		slices.Sort(gateIDs)
		for _, id := range gateIDs {
			gate := view.GateStatus[id]
			writeLine("  %s: %s\n", id, gate.Status)
			for _, reason := range gate.ReasonCodes {
				writeLine("    - %s\n", reason.Message)
			}
		}
	}

	if len(view.ArtifactDAG) > 0 {
		writeLine("\nArtifacts:\n")
		for _, node := range view.ArtifactDAG {
			readyMark := " "
			if node.Ready {
				readyMark = "*"
			}
			writeLine("  [%s] %s (%s)\n", readyMark, node.Name, node.State)
		}
	}

	writeLine("\nWhat's Next:\n")
	writeLine("  %s\n", primaryActionHint(view))
	if len(view.NextReadyActions) > 1 {
		writeLine("  Also available: %s\n", strings.Join(view.NextReadyActions[1:], ", "))
	}

	return builder.String()
}

func writeStatusText(w io.Writer, view statusView) error {
	rendered := renderStatusText(view)
	var err error
	_, err = io.WriteString(w, rendered)
	return err
}

func printMultiChangeSummary(cmd *cobra.Command, changes []model.Change, format string) error {
	view := buildMultiChangeSummaryView(changes)

	switch format {
	case "json":
		return encodeJSONResponse(cmd, view)
	case "yaml":
		return yaml.NewEncoder(cmd.OutOrStdout()).Encode(view)
	default:
		return writeMultiChangeText(cmd.OutOrStdout(), view)
	}
}

func renderMultiChangeText(view multiChangeSummaryView) string {
	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "Active Changes: %d\n\n", view.ActiveCount)
	_, _ = fmt.Fprintf(&builder, "%-38s  %-10s  %-16s  %s\n", "SLUG", "LANE", "STATE", "WORKTREE")
	_, _ = fmt.Fprintf(&builder, "%s\n", strings.Repeat("-", 89))
	for _, entry := range view.ActiveChanges {
		worktreePath := entry.WorktreePath
		if worktreePath == "" {
			worktreePath = "-"
		}
		label := entry.Slug
		_, _ = fmt.Fprintf(&builder, "%-38s  %-10s  %-16s  %s\n",
			label, entry.ExecMode, entry.CurrentState, worktreePath)
	}
	_, _ = fmt.Fprintf(&builder, "\n%s\n", view.Hint)
	return builder.String()
}

func writeMultiChangeText(w io.Writer, view multiChangeSummaryView) error {
	_, err := io.WriteString(w, renderMultiChangeText(view))
	return err
}

func progressBar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := (pct * width) / 100
	empty := width - filled
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", empty)
}

var actionHints = map[model.WorkflowState]string{
	model.StateS1Plan:    "slipway next  (planning phase)",
	model.StateS2Execute: "slipway run  (execute governed loop)",
	model.StateS3Review:  "slipway review  (bidirectional alignment review)",
	model.StateS4Verify:  "slipway next  (verify and close out)",
	model.StateDone:      "(complete)",
}

func primaryActionHint(view statusView) string {
	if view.PresetConfirmationPending {
		return "slipway preset <light|standard|strict>  (confirm governance preset before continuing)"
	}
	if len(view.NextReadyActions) == 0 {
		return nextActionHint(view.CurrentState)
	}
	first := strings.TrimSpace(view.NextReadyActions[0])
	switch {
	case first == "":
		return nextActionHint(view.CurrentState)
	case strings.HasPrefix(first, "slipway "):
		return first
	case first == "next":
		return nextActionHint(view.CurrentState)
	default:
		return "slipway " + first
	}
}

func nextActionHint(state model.WorkflowState) string {
	if hint, ok := actionHints[state]; ok {
		return hint
	}
	return "slipway next"
}

func visibleStatusBlockers(view statusView) []model.ReasonCode {
	if len(view.Blockers) == 0 {
		return nil
	}
	if len(view.SummaryBlockers) == 0 {
		return append([]model.ReasonCode(nil), view.Blockers...)
	}
	summarySet := make(map[string]struct{}, len(view.SummaryBlockers))
	for _, blocker := range view.SummaryBlockers {
		summarySet[blocker.Key()] = struct{}{}
	}
	filtered := make([]model.ReasonCode, 0, len(view.Blockers))
	for _, blocker := range view.Blockers {
		if _, ok := summarySet[blocker.Key()]; ok {
			continue
		}
		filtered = append(filtered, blocker)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func renderReasonCodeLines(reasons []model.ReasonCode) []string {
	if len(reasons) == 0 {
		return nil
	}
	lines := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		line := reason.Message
		if reason.Code != "" {
			line = reason.Code + ": " + reason.Message
		}
		lines = append(lines, line)
	}
	return lines
}
