package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyContextUtilization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		tokensUsed    int
		contextWindow int
		wantPercent   int
		wantState     contextPressureState
	}{
		{
			name:          "healthy below warning threshold",
			tokensUsed:    119_999,
			contextWindow: 200_000,
			wantPercent:   60,
			wantState:     contextPressureHealthy,
		},
		{
			name:          "warning at inclusive lower bound",
			tokensUsed:    120_000,
			contextWindow: 200_000,
			wantPercent:   60,
			wantState:     contextPressureWarning,
		},
		{
			name:          "critical at inclusive lower bound",
			tokensUsed:    140_000,
			contextWindow: 200_000,
			wantPercent:   70,
			wantState:     contextPressureCritical,
		},
		{
			name:          "critical clamps at 100 percent",
			tokensUsed:    250_000,
			contextWindow: 200_000,
			wantPercent:   100,
			wantState:     contextPressureCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := classifyContextUtilization(tt.tokensUsed, tt.contextWindow)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPercent, got.Percent)
			assert.Equal(t, tt.wantState, got.State)
		})
	}
}

func TestContextPressureHookCommandEmitsAdditionalContextAtCriticalThreshold(t *testing.T) {
	t.Parallel()

	payload := `{"hook_event_name":"PostToolUse","context_utilization":{"tokens_used":140000,"context_window":200000,"timestamp":` +
		time.Now().UTC().Format(`"`+time.RFC3339+`"`) +
		`}}`

	stdout, stderr, err := runRootCommandWithInput([]string{"hook", "context-pressure"}, payload)
	require.NoError(t, err)
	assert.Empty(t, stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	hookOutput, ok := out["hookSpecificOutput"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "PostToolUse", hookOutput["hookEventName"])
	additionalContext, ok := hookOutput["additionalContext"].(string)
	require.True(t, ok)
	assert.Contains(t, additionalContext, "CONTEXT CRITICAL")
	assert.Contains(t, additionalContext, ".git/slipway/runtime/changes/<slug>/handoff.md")
	assert.Contains(t, additionalContext, "workflow handoff contract")
	assert.Contains(t, additionalContext, "The handoff is advisory")
	assert.Contains(t, additionalContext, "slipway status --json")
	assert.Contains(t, additionalContext, "slipway next --json")
	assert.NotContains(t, additionalContext, "lifecycle authority")
	assert.NotContains(t, additionalContext, "governed evidence")
}

func TestContextPressureHookCommandReadsLiveUsageFromClaudeTranscript(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	transcriptPath := filepath.Join(t.TempDir(), "transcript.jsonl")
	transcriptLine := `{"type":"assistant","timestamp":` +
		now.Format(`"`+time.RFC3339+`"`) +
		`,"message":{"usage":{"input_tokens":120000,"cache_creation_input_tokens":10000,"cache_read_input_tokens":10000,"output_tokens":9000}}}` +
		"\n"
	require.NoError(t, os.WriteFile(transcriptPath, []byte(transcriptLine), 0o644))
	payload := `{"hook_event_name":"PostToolUse","session_id":"abc123","transcript_path":` +
		jsonString(transcriptPath) +
		`,"tool_name":"Write","tool_input":{"file_path":"README.md"},"tool_response":{"success":true}}`

	stdout, stderr, err := runRootCommandWithInput([]string{"hook", "context-pressure"}, payload)
	require.NoError(t, err)
	assert.Empty(t, stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	hookOutput, ok := out["hookSpecificOutput"].(map[string]any)
	require.True(t, ok)
	additionalContext, ok := hookOutput["additionalContext"].(string)
	require.True(t, ok)
	assert.Contains(t, additionalContext, "CONTEXT CRITICAL")
	assert.Contains(t, additionalContext, "70%")
	assert.Contains(t, additionalContext, "workflow handoff contract")
}

func TestContextPressureHookCommandIgnoresStaleTranscriptUsage(t *testing.T) {
	t.Parallel()

	transcriptPath := filepath.Join(t.TempDir(), "transcript.jsonl")
	transcriptLine := `{"type":"assistant","timestamp":"2000-01-01T00:00:00Z",` +
		`"message":{"usage":{"input_tokens":120000,"cache_creation_input_tokens":10000,` +
		`"cache_read_input_tokens":10000}}}` + "\n"
	require.NoError(t, os.WriteFile(transcriptPath, []byte(transcriptLine), 0o644))
	payload := `{"hook_event_name":"PostToolUse","session_id":"abc123","transcript_path":` +
		jsonString(transcriptPath) +
		`,"tool_name":"Write"}`

	stdout, stderr, err := runRootCommandWithInput([]string{"hook", "context-pressure"}, payload)
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestContextPressureHookCommandIgnoresMissingAndStaleMetrics(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Format(`"` + time.RFC3339 + `"`)
	tests := []struct {
		name    string
		payload string
	}{
		{
			name: "healthy metrics",
			payload: `{"hook_event_name":"PostToolUse","context_utilization":{"tokens_used":119999,"context_window":200000,"timestamp":` +
				now +
				`}}`,
		},
		{
			name:    "missing metrics",
			payload: `{"hook_event_name":"PostToolUse","session_id":"abc123"}`,
		},
		{
			name:    "stale metrics",
			payload: `{"hook_event_name":"PostToolUse","context_utilization":{"tokens_used":140000,"context_window":200000,"timestamp":"2000-01-01T00:00:00Z"}}`,
		},
		{
			name:    "malformed json",
			payload: `{not-json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, err := runRootCommandWithInput([]string{"hook", "context-pressure"}, tt.payload)
			require.NoError(t, err)
			assert.Empty(t, stdout)
			assert.Empty(t, stderr)
		})
	}
}

// TestContextPressureHookCommandFailsSilentOnUnusableInput pins REQ-003: the
// PostToolUse hook is inlined into automatic host hooks, so it must always exit
// 0 (return nil, write nothing, never panic) on empty or malformed stdin rather
// than surfacing a blocking or non-zero failure.
func TestContextPressureHookCommandFailsSilentOnUnusableInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "empty stdin",
			payload: "",
		},
		{
			name:    "whitespace only stdin",
			payload: "   \n\t  \n",
		},
		{
			name:    "garbage non-json stdin",
			payload: "this is not json at all <<>>",
		},
		{
			name:    "truncated json",
			payload: `{"hook_event_name":"PostToolUse","context_utilization":{`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.NotPanics(t, func() {
				stdout, stderr, err := runRootCommandWithInput([]string{"hook", "context-pressure"}, tt.payload)
				require.NoError(t, err)
				assert.Empty(t, stdout)
				assert.Empty(t, stderr)
			})
		})
	}
}

func TestContextPressureMessagesKeepRuntimeHandoffAdvisory(t *testing.T) {
	t.Parallel()

	critical := contextPressureMessage(contextPressureResult{
		Percent: 72,
		State:   contextPressureCritical,
	})
	warning := contextPressureMessage(contextPressureResult{
		Percent: 63,
		State:   contextPressureWarning,
	})

	assert.Contains(t, critical, "workflow handoff contract")
	assert.Contains(t, critical, "The handoff is advisory")
	assert.Contains(t, critical, "slipway status --json")
	assert.Contains(t, critical, "slipway next --json")
	assert.Contains(t, warning, "workflow handoff contract")

	for name, message := range map[string]string{
		"critical": critical,
		"warning":  warning,
	} {
		assert.NotContains(t, message, "lifecycle authority", name)
		assert.NotContains(t, message, "governed evidence", name)
		assert.NotContains(t, message, "freshness input", name)
		assert.NotContains(t, message, "handoff is a gate", name)
		assert.NotContains(t, message, "governed host skill", name)
	}
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func runRootCommandWithInput(args []string, stdin string) (string, string, error) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := newRootCmd()
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := executeRootCommand(cmd)
	return out.String(), errOut.String(), err
}
