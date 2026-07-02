package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/tmpl"
)

var marshalContextBudgetInput = json.Marshal

// estimateContextBudget estimates the token cost of loading skill prompt,
// artifact bundle, and evidence files for the next skill invocation.
// Approximation: 1 token ≈ 4 bytes of content.
func estimateContextBudget(root string, skill *nextSkillView, inputContext nextContext) *contextBudget {
	if skill == nil {
		return nil
	}

	const bytesPerToken = 4
	const warnRemainingPercent = 50.0
	const stopRemainingPercent = 35.0

	estimateDir := func(dir string) int {
		total := 0
		entries, err := os.ReadDir(dir)
		if err != nil {
			return 0
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			total += int(info.Size())
		}
		return total / bytesPerToken
	}

	estimateSkillPromptTokens := func(skillName string) int {
		if strings.TrimSpace(skillName) == "" {
			return 0
		}
		for _, candidate := range []string{
			"skills/" + skillName + "/HOST_SKILL.md",
			"skills/" + skillName + "/HOST_SKILL.md.tmpl",
			"skills/" + skillName + "/CATALOG_SKILL.md",
		} {
			content, exists, err := tmpl.ContentIfExists(candidate)
			if err != nil || !exists {
				continue
			}
			return len(content) / bytesPerToken
		}
		return 0
	}

	promptTokens := estimateSkillPromptTokens(skill.Name)
	artifactTokens := 0
	if strings.TrimSpace(inputContext.ArtifactBundle) != "" {
		artifactTokens = estimateDir(resolveInputContextPath(root, inputContext.WorkspaceRoot, inputContext.ArtifactBundle))
	}
	statePayload, err := marshalContextBudgetInput(inputContext)
	if err != nil {
		statePayload = []byte(fmt.Sprintf("%+v", inputContext))
	}
	stateTokens := len(statePayload) / bytesPerToken

	total := promptTokens + artifactTokens + stateTokens
	if total == 0 {
		return nil
	}

	assumedContextWindow := model.DefaultContextWindowTokens
	// Honor a valid SLIPWAY_CONTEXT_WINDOW_TOKENS override; a malformed or
	// non-positive value falls through to the default window. Mirrors
	// contextPressureWindowTokens.
	if raw := strings.TrimSpace(os.Getenv("SLIPWAY_CONTEXT_WINDOW_TOKENS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			assumedContextWindow = parsed
		}
	}
	utilization := (float64(total) / float64(assumedContextWindow)) * 100
	remaining := 100 - utilization
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}

	return &contextBudget{
		EstimatedTokens:      total,
		AssumedContextWindow: assumedContextWindow,
		UtilizationPercent:   utilization,
		RemainingPercent:     remaining,
		Health:               classifyContextHealth(utilization),
		QualityCurve:         classifyContextQualityCurve(remaining),
		GuardAction:          classifyContextGuardAction(remaining),
		Thresholds: contextBudgetThresholds{
			WarnBelowRemainingPercent: warnRemainingPercent,
			StopBelowRemainingPercent: stopRemainingPercent,
		},
		Breakdown: contextBudgetBreakdown{
			SkillPrompt:     promptTokens,
			ArtifactContext: artifactTokens,
			StateContext:    stateTokens,
		},
	}
}

func classifyContextHealth(utilizationPercent float64) string {
	switch {
	case utilizationPercent <= 30:
		return "peak"
	case utilizationPercent <= 50:
		return "good"
	case utilizationPercent <= 70:
		return "degrading"
	default:
		return "poor"
	}
}

func classifyContextQualityCurve(remainingPercent float64) string {
	switch {
	case remainingPercent >= 70:
		return "peak"
	case remainingPercent >= 50:
		return "good"
	case remainingPercent >= 30:
		return "degrading"
	default:
		return "poor"
	}
}

func classifyContextGuardAction(remainingPercent float64) string {
	switch {
	case remainingPercent <= 35:
		return "stop"
	case remainingPercent <= 50:
		return "warn"
	default:
		return "ok"
	}
}

// writeContextGuardHookMessages outputs context budget guard messages in hook
// format (BLOCK/WARN RULE_ID: reason + Next: remediation). Produces no output
// when the context budget is healthy.
func writeContextGuardHookMessages(w io.Writer, view nextView) error {
	if view.ContextBudget == nil {
		return nil
	}
	writer := newFormatWriter(w)
	switch view.ContextBudget.GuardAction {
	case "stop":
		writer.Writef("WARN CONTEXT_WINDOW_STOP: remaining context %.1f%% is at or below the stop threshold\n", view.ContextBudget.RemainingPercent)
		writer.Writeln("Next: Consider pausing execution and resuming in a fresh session context.")
	case "warn":
		writer.Writef("WARN CONTEXT_WINDOW_WARN: remaining context %.1f%% is at or below the warning threshold\n", view.ContextBudget.RemainingPercent)
		writer.Writeln("Next: Trim context payload and continue with smaller, task-scoped inputs.")
	}
	return writer.Err()
}

func applyContextBudgetGuard(view *nextView) {
	if view == nil || view.ContextBudget == nil {
		return
	}

	switch view.ContextBudget.GuardAction {
	case "warn":
		view.Warnings = append(view.Warnings, fmt.Sprintf(
			"context window remaining %.1f%% <= %.1f%% threshold; trim context payload before continuing",
			view.ContextBudget.RemainingPercent,
			view.ContextBudget.Thresholds.WarnBelowRemainingPercent,
		))
	case "stop":
		view.Warnings = append(view.Warnings, fmt.Sprintf(
			"context window remaining %.1f%% <= %.1f%% stop threshold; write handoff and resume in fresh context",
			view.ContextBudget.RemainingPercent,
			view.ContextBudget.Thresholds.StopBelowRemainingPercent,
		))
	}
}
