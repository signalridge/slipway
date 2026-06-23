package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

const contextPressureMetricMaxAge = 60 * time.Second
const contextPressureTranscriptTailBytes = 1 << 20
const contextPressureDefaultWindowTokens = 200000

type contextPressureState string

const (
	contextPressureHealthy  contextPressureState = "healthy"
	contextPressureWarning  contextPressureState = "warning"
	contextPressureCritical contextPressureState = "critical"
)

type contextPressureResult struct {
	Percent int
	State   contextPressureState
}

type contextPressureMetric struct {
	tokensUsed    int
	contextWindow int
	timestamp     time.Time
	hasTimestamp  bool
}

// hookCommandName is the name of the parent command whose subtree is inlined
// into host automation. Errors from this subtree are swallowed to a silent
// exit 0 so a hook can never block the host (see isHookSubtreeCommand).
const hookCommandName = "hook"

func makeHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    hookCommandName,
		Short:  "Run internal Slipway hook helpers",
		Hidden: true,
		// Hooks are inlined into host automation and must stay inert under
		// version skew. If a generated config names a hook subcommand this
		// binary does not have, Cobra would otherwise print parent usage to the
		// host's hook output channel. Make the parent runnable so any
		// unresolved invocation returns an error that executeRootCommand
		// swallows to a silent exit 0 instead of emitting usage text.
		Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("no hook subcommand resolved")
		},
	}
	cmd.AddCommand(makeSessionStartHookCmd())
	cmd.AddCommand(makeContextPressureHookCmd())
	return cmd
}

func makeContextPressureHookCmd() *cobra.Command {
	var toolID string
	cmd := &cobra.Command{
		Use:    "context-pressure",
		Short:  "Evaluate PostToolUse context pressure",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			// Fail silent: this hook is inlined into automatic host hooks and must
			// never surface a blocking or non-zero failure. Any internal error
			// (read, JSON, path, classify, write) is swallowed to a clean exit 0.
			_ = runContextPressureHook(cmd.InOrStdin(), cmd.OutOrStdout(), time.Now(), toolID)
		},
	}
	cmd.Flags().StringVar(&toolID, "tool", "", "Host tool ID")
	return cmd
}

func classifyContextUtilization(tokensUsed, contextWindow int) (contextPressureResult, error) {
	if tokensUsed < 0 {
		return contextPressureResult{}, errors.New("tokensUsed must be non-negative")
	}
	if contextWindow <= 0 {
		return contextPressureResult{}, errors.New("contextWindow must be positive")
	}

	ratio := math.Min(float64(tokensUsed)/float64(contextWindow), 1)
	percent := int(math.Min(math.Round(ratio*100), 100))
	state := contextPressureHealthy
	switch {
	case ratio >= 0.70:
		state = contextPressureCritical
	case ratio >= 0.60:
		state = contextPressureWarning
	}
	return contextPressureResult{Percent: percent, State: state}, nil
}

func runContextPressureHook(r io.Reader, w io.Writer, now time.Time, toolID string) error {
	raw, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		if strings.EqualFold(strings.TrimSpace(toolID), "codex") {
			return runCodexPromptHandoffNudge(w)
		}
		return nil
	}

	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(toolID), "codex") || strings.EqualFold(hookEventName(input, ""), "UserPromptSubmit") {
		return runCodexPromptHandoffNudge(w)
	}

	metric, ok := resolveContextPressureMetric(input, now)
	if !ok {
		return nil
	}
	if metric.hasTimestamp && now.Sub(metric.timestamp) > contextPressureMetricMaxAge {
		return nil
	}

	result, err := classifyContextUtilization(metric.tokensUsed, metric.contextWindow)
	if err != nil || result.State == contextPressureHealthy {
		return nil
	}

	eventName := hookEventName(input, "PostToolUse")

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": contextPressureMessage(result),
		},
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil
	}
	_, _ = w.Write(encoded)
	return nil
}

func hookEventName(input map[string]any, fallback string) string {
	eventName, _ := input["hook_event_name"].(string)
	eventName = strings.TrimSpace(eventName)
	if eventName == "" {
		return fallback
	}
	return eventName
}

func runCodexPromptHandoffNudge(w io.Writer) error {
	root, err := projectRootFromWD()
	if err != nil {
		return nil
	}
	ref, ok, err := resolveHandoffChangeRef(root, "")
	if err != nil || !ok {
		return nil
	}
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return nil
	}
	doc, err := state.ReadHandoff(root, change)
	if err == nil && doc.Header.Staleness == "fresh" {
		return nil
	}
	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": "HANDOFF NUDGE: the active change handoff is missing or stale; run `slipway handoff write` before stopping or splitting context. `slipway status` and `slipway next` remain authoritative.",
		},
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil
	}
	_, _ = w.Write(encoded)
	return nil
}

func resolveContextPressureMetric(input map[string]any, now time.Time) (contextPressureMetric, bool) {
	if rawMetric, ok := input["context_utilization"].(map[string]any); ok {
		if metric, ok := parseContextPressureMetric(rawMetric); ok {
			return metric, true
		}
	}
	if rawMetric, ok := input["context_window"].(map[string]any); ok {
		if metric, ok := parseContextPressureMetric(rawMetric); ok {
			return metric, true
		}
	}

	for _, path := range contextPressureMetricPaths(input) {
		content, err := os.ReadFile(path) // #nosec G304 -- metrics path is operator-supplied or scoped by sanitized session ID under os.TempDir.
		if err != nil {
			continue
		}
		var rawMetric map[string]any
		if err := json.Unmarshal(content, &rawMetric); err != nil {
			continue
		}
		metric, ok := parseContextPressureMetric(rawMetric)
		if ok && (!metric.hasTimestamp || now.Sub(metric.timestamp) <= contextPressureMetricMaxAge) {
			return metric, true
		}
	}
	if metric, ok := contextPressureMetricFromTranscript(input, now); ok {
		return metric, true
	}
	return contextPressureMetric{}, false
}

func contextPressureMetricPaths(input map[string]any) []string {
	var paths []string
	if explicit := strings.TrimSpace(os.Getenv("SLIPWAY_CONTEXT_METRICS_PATH")); explicit != "" {
		paths = append(paths, explicit)
	}

	sessionID, _ := input["session_id"].(string)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || strings.Contains(sessionID, "..") || strings.ContainsAny(sessionID, `/\`) {
		return paths
	}

	tmp := os.TempDir()
	paths = append(paths,
		filepath.Join(tmp, "slipway-ctx-"+sessionID+".json"),
		filepath.Join(tmp, "claude-ctx-"+sessionID+".json"),
	)
	return paths
}

func parseContextPressureMetric(raw map[string]any) (contextPressureMetric, bool) {
	var metric contextPressureMetric

	if ts, ok := parseContextMetricTimestamp(raw); ok {
		metric.timestamp = ts
		metric.hasTimestamp = true
	}

	if tokens, ok := numericField(raw, "tokens_used"); ok {
		if window, ok := firstNumericField(raw, "context_window", "context_window_size"); ok {
			if tokens < 0 || window <= 0 || !isIntegral(tokens) || !isIntegral(window) {
				return contextPressureMetric{}, false
			}
			metric.tokensUsed = int(tokens)
			metric.contextWindow = int(window)
			return metric, true
		}
	}

	if usedPct, ok := numericField(raw, "used_pct"); ok {
		return metricFromUsedPercent(metric, usedPct)
	}
	if usedPct, ok := numericField(raw, "used_percentage"); ok {
		return metricFromUsedPercent(metric, usedPct)
	}
	if remaining, ok := numericField(raw, "remaining_percentage"); ok {
		return metricFromUsedPercent(metric, 100-remaining)
	}
	return contextPressureMetric{}, false
}

func metricFromUsedPercent(metric contextPressureMetric, usedPct float64) (contextPressureMetric, bool) {
	if usedPct < 0 {
		return contextPressureMetric{}, false
	}
	if usedPct > 100 {
		usedPct = 100
	}
	window := contextPressureWindowTokens()
	metric.tokensUsed = int(math.Round((usedPct / 100) * float64(window)))
	metric.contextWindow = window
	return metric, true
}

func contextPressureMetricFromTranscript(input map[string]any, now time.Time) (contextPressureMetric, bool) {
	transcriptPath, _ := input["transcript_path"].(string)
	raw, modTime, ok := readContextPressureTranscriptTail(strings.TrimSpace(transcriptPath))
	if !ok {
		return contextPressureMetric{}, false
	}

	lines := bytes.Split(raw, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			continue
		}
		metric, ok := parseContextPressureTranscriptMetric(row, modTime)
		if !ok {
			continue
		}
		if metric.hasTimestamp && now.Sub(metric.timestamp) > contextPressureMetricMaxAge {
			return contextPressureMetric{}, false
		}
		return metric, true
	}
	return contextPressureMetric{}, false
}

func readContextPressureTranscriptTail(path string) ([]byte, time.Time, bool) {
	if path == "" {
		return nil, time.Time{}, false
	}

	// #nosec G304 -- transcript_path is supplied by the local Claude hook payload;
	// tail parsing is read-only and bounded.
	file, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return nil, time.Time{}, false
	}

	start := info.Size() - contextPressureTranscriptTailBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, time.Time{}, false
	}

	raw, err := io.ReadAll(io.LimitReader(file, contextPressureTranscriptTailBytes))
	if err != nil {
		return nil, time.Time{}, false
	}
	if start > 0 {
		idx := bytes.IndexByte(raw, '\n')
		if idx < 0 {
			return nil, time.Time{}, false
		}
		raw = raw[idx+1:]
	}
	return raw, info.ModTime(), true
}

func parseContextPressureTranscriptMetric(row map[string]any, fallbackTimestamp time.Time) (contextPressureMetric, bool) {
	usage, ok := nestedMap(row, "message", "usage")
	if !ok {
		usage, ok = rawMap(row, "usage")
	}
	if !ok {
		return contextPressureMetric{}, false
	}

	metric, ok := metricFromClaudeUsage(usage)
	if !ok {
		return contextPressureMetric{}, false
	}
	if ts, ok := parseContextMetricTimestamp(row); ok {
		metric.timestamp = ts
		metric.hasTimestamp = true
	} else if !fallbackTimestamp.IsZero() {
		metric.timestamp = fallbackTimestamp
		metric.hasTimestamp = true
	}
	return metric, true
}

func metricFromClaudeUsage(usage map[string]any) (contextPressureMetric, bool) {
	total := 0
	for _, key := range []string{"input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"} {
		value, ok := numericField(usage, key)
		if !ok {
			continue
		}
		if value < 0 || !isIntegral(value) {
			return contextPressureMetric{}, false
		}
		total += int(value)
	}
	if total <= 0 {
		return contextPressureMetric{}, false
	}
	return contextPressureMetric{
		tokensUsed:    total,
		contextWindow: contextPressureWindowTokens(),
	}, true
}

func contextPressureWindowTokens() int {
	if raw := strings.TrimSpace(os.Getenv("SLIPWAY_CONTEXT_WINDOW_TOKENS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return contextPressureDefaultWindowTokens
}

func firstNumericField(raw map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := numericField(raw, key); ok {
			return value, true
		}
	}
	return 0, false
}

func numericField(raw map[string]any, key string) (float64, bool) {
	value, ok := raw[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		parsed, err := v.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func rawMap(raw map[string]any, key string) (map[string]any, bool) {
	value, ok := raw[key]
	if !ok {
		return nil, false
	}
	nested, ok := value.(map[string]any)
	return nested, ok
}

func nestedMap(raw map[string]any, keys ...string) (map[string]any, bool) {
	current := raw
	for i, key := range keys {
		next, ok := rawMap(current, key)
		if !ok {
			return nil, false
		}
		if i == len(keys)-1 {
			return next, true
		}
		current = next
	}
	return nil, false
}

func isIntegral(value float64) bool {
	return math.Trunc(value) == value
}

func parseContextMetricTimestamp(raw map[string]any) (time.Time, bool) {
	for _, key := range []string{"timestamp", "captured_at"} {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case string:
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
			if err == nil {
				return parsed, true
			}
		case float64:
			return time.Unix(int64(v), 0), true
		case json.Number:
			parsed, err := v.Int64()
			if err == nil {
				return time.Unix(parsed, 0), true
			}
		}
	}
	return time.Time{}, false
}

func contextPressureMessage(result contextPressureResult) string {
	switch result.State {
	case contextPressureCritical:
		return fmt.Sprintf(
			"CONTEXT CRITICAL: usage is approximately %d%%. Context pressure is high; "+
				"run `slipway handoff write` to refresh the per-change advisory handoff "+
				"before continuing in a fresh context. "+
				"The handoff is advisory; fresh sessions still run `slipway status --json` "+
				"and `slipway next --json`.",
			result.Percent,
		)
	default:
		return fmt.Sprintf(
			"CONTEXT WARNING: usage is approximately %d%%. Avoid starting new complex work; "+
				"run `slipway handoff write` to refresh the per-change advisory handoff "+
				"before continuing.",
			result.Percent,
		)
	}
}
