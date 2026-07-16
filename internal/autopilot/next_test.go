package autopilot

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/runstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveNextCoversEveryRunBranch(t *testing.T) {
	t.Parallel()
	workspace := filepath.Join(t.TempDir(), "workspace with spaces")
	workspaceID := "sha256:" + strings.Repeat("a", 64)
	action := &Action{ActionID: "action-1", Kind: ActionImplement}
	request := destructiveRequestForTest(t)

	tests := []struct {
		name       string
		run        Run
		operation  NextOperation
		variantIDs []string
	}{
		{
			name:       "active action",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunActive, CurrentAction: action},
			operation:  NextOperationAction,
			variantIDs: []string{"submit-outcome-file", "submit-outcome-stdin", "skip-action"},
		},
		{
			name:       "decision pause",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunPaused, PauseReason: PauseDecisionRequired, CurrentAction: action},
			operation:  NextOperationAnswer,
			variantIDs: []string{"answer-decision", "skip-action"},
		},
		{
			name: "destructive pause",
			run: Run{
				ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunPaused, PauseReason: PauseDestructiveConfirm,
				CurrentAction: action, PendingDestructiveRequest: &request,
			},
			operation:  NextOperationAnswer,
			variantIDs: []string{"confirm-destructive", "decline-or-feedback", "skip-action"},
		},
		{
			name:       "environment pause ad hoc",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunPaused, PauseReason: PauseEnvironmentUnavailable, CurrentAction: action},
			operation:  NextOperationResume,
			variantIDs: []string{"resume-ad-hoc", "skip-action"},
		},
		{
			name:       "environment pause issue bound",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunPaused, PauseReason: PauseEnvironmentUnavailable, CurrentAction: action, PinnedSource: &PinnedSource{}},
			operation:  NextOperationResume,
			variantIDs: []string{"refresh-source", "use-pinned-source", "skip-action"},
		},
		{
			name:       "budget pause ad hoc",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunPaused, PauseReason: PauseBudgetExhausted},
			operation:  NextOperationResume,
			variantIDs: []string{"resume-ad-hoc"},
		},
		{
			name:       "stopped ad hoc",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunStopped, CurrentAction: action},
			operation:  NextOperationResume,
			variantIDs: []string{"resume-ad-hoc"},
		},
		{
			name:       "issue-bound resume",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunStopped, PinnedSource: &PinnedSource{}},
			operation:  NextOperationResume,
			variantIDs: []string{"refresh-source", "use-pinned-source"},
		},
		{
			name: "valid source candidate",
			run: Run{
				ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunPaused, PauseReason: PauseDecisionRequired,
				PinnedSource: &PinnedSource{}, SourceCandidate: &SourceCandidate{CandidateID: "candidate-1", SourceCandidateInput: SourceCandidateInput{Valid: true}},
			},
			operation:  NextOperationResume,
			variantIDs: []string{"keep-pinned", "adopt"},
		},
		{
			name: "invalid source candidate",
			run: Run{
				ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunPaused, PauseReason: PauseDecisionRequired,
				PinnedSource: &PinnedSource{}, SourceCandidate: &SourceCandidate{CandidateID: "candidate-2", SourceCandidateInput: SourceCandidateInput{Valid: false}},
			},
			operation:  NextOperationResume,
			variantIDs: []string{"keep-pinned"},
		},
		{
			name:       "ended",
			run:        Run{ID: "run-1", Workspace: workspace, WorkspaceIdentity: runstore.WorkspaceIdentity{ID: workspaceID}, State: RunEnded},
			operation:  NextOperationNone,
			variantIDs: []string{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			next, err := DeriveNext(test.run)
			require.NoError(t, err)
			require.NoError(t, next.Validate())
			assert.Equal(t, test.operation, next.Operation)
			assert.Equal(t, workspaceID, next.WorkspaceIdentity)
			ids := make([]string, 0, len(next.Variants))
			for _, variant := range next.Variants {
				ids = append(ids, variant.ID)
				require.Contains(t, variant.BaseArgv, "--root")
				rootIndex := indexOfString(variant.BaseArgv, "--root")
				require.GreaterOrEqual(t, rootIndex, 0)
				require.Less(t, rootIndex+1, len(variant.BaseArgv))
				assert.Equal(t, workspace, variant.BaseArgv[rootIndex+1])
				for _, argument := range variant.BaseArgv {
					assert.False(t, isPseudoValue(argument), argument)
				}
			}
			assert.Equal(t, test.variantIDs, ids)
			encoded, err := json.Marshal(next)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), `"variants":null`)
			assert.NotContains(t, string(encoded), `"inputs":null`)
		})
	}
}

func TestResolveNextUsesSchemaOrderAndExactRawValues(t *testing.T) {
	t.Parallel()
	workspace := filepath.Join(t.TempDir(), "根 with spaces")
	next, err := NewCommandNext(
		NextOperationCommand,
		workspace,
		"typed-command",
		[]string{"slipway", "status", "--root", workspace},
		[]NextInput{
			{Name: "scope", Type: NextInputDigest, Flag: "--scope-sha256", Required: true},
			{Name: "text", Type: NextInputString, Flag: "--text", Required: false},
		},
	)
	require.NoError(t, err)
	digest := "sha256:" + strings.Repeat("a", 64)
	text := " spaces ' \" 界\r\n%!&^ "
	argv, err := next.Resolve("typed-command", map[string]NextInputValue{
		"text":  {Type: NextInputString, Value: text},
		"scope": {Type: NextInputDigest, Value: digest},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"slipway", "status", "--root", workspace,
		"--scope-sha256", digest, "--text", text,
	}, argv)
}

func TestResolveStartNextInsertsInputsBeforeGoalSeparator(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	next, err := NewCommandNext(
		NextOperationStart,
		workspace,
		"start-with-source",
		[]string{"slipway", "run", "--budget", "9", "--json", "--root", workspace, "--no-review", "--", "-leading goal"},
		[]NextInput{{Name: "source_file", Type: NextInputPath, Flag: "--source-file", Required: true}},
	)
	require.NoError(t, err)

	argv, err := next.Resolve("start-with-source", map[string]NextInputValue{
		"source_file": {Type: NextInputPath, Value: "/tmp/source.json"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"slipway", "run", "--budget", "9", "--json", "--root", workspace,
		"--no-review", "--source-file", "/tmp/source.json", "--", "-leading goal",
	}, argv)
}

func TestStartNextAllowsFlagLikeGoals(t *testing.T) {
	for _, goal := range []string{"--root", "--"} {
		goal := goal
		t.Run(goal, func(t *testing.T) {
			t.Parallel()
			workspace := t.TempDir()
			next, err := NewCommandNext(
				NextOperationStart,
				workspace,
				"retry-run",
				[]string{"slipway", "run", "--budget", "4", "--json", "--root", workspace, "--", goal},
				[]NextInput{},
			)
			require.NoError(t, err)

			argv, err := next.Resolve("retry-run", map[string]NextInputValue{})
			require.NoError(t, err)
			assert.Equal(t, []string{
				"slipway", "run", "--budget", "4", "--json", "--root", workspace, "--", goal,
			}, argv)
		})
	}
}

func TestResolveNextRejectsMalformedTypedInputs(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	next, err := NewCommandNext(
		NextOperationCommand,
		workspace,
		"resolve",
		[]string{"slipway", "status", "--root", workspace},
		[]NextInput{
			{Name: "path", Type: NextInputPath, Flag: "--source-file", Required: true},
			{Name: "mode", Type: NextInputEnum, Flag: "--mode", Required: false, Choices: []string{"one", "two"}},
			{Name: "digest", Type: NextInputDigest, Flag: "--digest", Required: false},
		},
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		values map[string]NextInputValue
		want   string
	}{
		{name: "missing required", values: map[string]NextInputValue{}, want: "required input"},
		{name: "unknown input", values: map[string]NextInputValue{"extra": {Type: NextInputString, Value: "x"}}, want: "unknown input"},
		{name: "wrong type", values: map[string]NextInputValue{"path": {Type: NextInputString, Value: "/tmp/x"}}, want: "requires type path"},
		{name: "empty", values: map[string]NextInputValue{"path": {Type: NextInputPath, Value: ""}}, want: "nonempty"},
		{name: "invalid enum", values: map[string]NextInputValue{"path": {Type: NextInputPath, Value: "/tmp/x"}, "mode": {Type: NextInputEnum, Value: "three"}}, want: "must be one of"},
		{name: "invalid digest", values: map[string]NextInputValue{"path": {Type: NextInputPath, Value: "/tmp/x"}, "digest": {Type: NextInputDigest, Value: "sha256:ABC"}}, want: "lowercase"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := next.Resolve("resolve", test.values)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestNextValidationRejectsAmbiguousSchemasAndPlaceholders(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	otherWorkspace := filepath.Join(filepath.Dir(workspace), "other")
	workspaceID := "sha256:" + strings.Repeat("b", 64)
	valid := Next{
		Operation: NextOperationResume, WorkspaceIdentity: workspaceID, workspaceRoot: workspace,
		Variants: []NextVariant{{
			ID: "resume-ad-hoc", BaseArgv: []string{"slipway", "_machine", "resume", "run-1", "--root", workspace}, Inputs: []NextInput{},
		}},
	}
	tests := []struct {
		name   string
		mutate func(*Next)
		want   string
	}{
		{name: "invalid workspace identity", mutate: func(next *Next) { next.WorkspaceIdentity = "relative" }, want: "lowercase sha256"},
		{name: "nil variants", mutate: func(next *Next) { next.Variants = nil }, want: "non-null"},
		{name: "duplicate variants", mutate: func(next *Next) { next.Variants = append(next.Variants, next.Variants[0]) }, want: "duplicated"},
		{name: "nil inputs", mutate: func(next *Next) { next.Variants[0].Inputs = nil }, want: "non-null"},
		{name: "missing root", mutate: func(next *Next) { next.Variants[0].BaseArgv = []string{"slipway", "status"} }, want: "exactly one"},
		{name: "wrong root", mutate: func(next *Next) { next.Variants[0].BaseArgv[len(next.Variants[0].BaseArgv)-1] = otherWorkspace }, want: "preserve"},
		{name: "public variants disagree on root", mutate: func(next *Next) {
			next.workspaceRoot = ""
			other := cloneNextForTest(*next).Variants[0]
			other.ID = "other-root"
			other.BaseArgv[len(other.BaseArgv)-1] = otherWorkspace
			next.Variants = append(next.Variants, other)
		}, want: "preserve"},
		{name: "placeholder", mutate: func(next *Next) { next.Variants[0].BaseArgv = append(next.Variants[0].BaseArgv, "<file>") }, want: "placeholder"},
		{name: "quoted placeholder", mutate: func(next *Next) { next.Variants[0].BaseArgv = append(next.Variants[0].BaseArgv, `"FILE"`) }, want: "placeholder"},
		{name: "NUL argument", mutate: func(next *Next) { next.Variants[0].BaseArgv[3] = "run\x00id" }, want: "without NUL"},
		{name: "invalid flag", mutate: func(next *Next) {
			next.Variants[0].Inputs = []NextInput{{Name: "value", Type: NextInputString, Flag: "-v"}}
		}, want: "invalid flag"},
		{name: "enum choices missing", mutate: func(next *Next) {
			next.Variants[0].Inputs = []NextInput{{Name: "value", Type: NextInputEnum, Flag: "--value", Choices: []string{}}}
		}, want: "requires nonempty"},
		{name: "choices on string", mutate: func(next *Next) {
			next.Variants[0].Inputs = []NextInput{{Name: "value", Type: NextInputString, Flag: "--value", Choices: []string{}}}
		}, want: "only permits choices"},
		{name: "duplicate input", mutate: func(next *Next) {
			next.Variants[0].Inputs = []NextInput{{Name: "value", Type: NextInputString, Flag: "--value"}, {Name: "value", Type: NextInputPath, Flag: "--path"}}
		}, want: "duplicated"},
		{name: "operation family mismatch", mutate: func(next *Next) { next.Operation = NextOperationAction }, want: "operation action"},
		{name: "answer rejects bogus prefix-only argv", mutate: func(next *Next) {
			next.Operation = NextOperationAnswer
			next.Variants[0].BaseArgv = []string{"slipway", "_machine", "answer", "--bogus", "x", "--root", workspace}
		}, want: "operation answer"},
		{name: "answer requires run", mutate: func(next *Next) {
			next.Operation = NextOperationAnswer
			next.Variants[0].BaseArgv = []string{"slipway", "_machine", "answer", "--action", "action-1", "--root", workspace}
		}, want: "operation answer"},
		{name: "answer requires action", mutate: func(next *Next) {
			next.Operation = NextOperationAnswer
			next.Variants[0].BaseArgv = []string{"slipway", "_machine", "answer", "--run", "run-1", "--root", workspace}
		}, want: "operation answer"},
		{name: "answer rejects unknown typed input", mutate: func(next *Next) {
			next.Operation = NextOperationAnswer
			next.Variants[0].BaseArgv = []string{
				"slipway", "_machine", "answer", "--run", "run-1", "--action", "action-1", "--root", workspace,
			}
			next.Variants[0].Inputs = []NextInput{{Name: "bogus", Type: NextInputString, Flag: "--bogus", Required: true}}
		}, want: "unsupported flag"},
		{name: "action rejects unknown flag", mutate: func(next *Next) {
			next.Operation = NextOperationAction
			next.Variants[0].BaseArgv = []string{
				"slipway", "_machine", "submit", "--run", "run-1", "--action", "action-1", "--root", workspace,
				"--outcome-stdin", "--bogus",
			}
		}, want: "unsupported flag"},
		{name: "action requires outcome mode", mutate: func(next *Next) {
			next.Operation = NextOperationAction
			next.Variants[0].BaseArgv = []string{
				"slipway", "_machine", "submit", "--run", "run-1", "--action", "action-1", "--root", workspace,
			}
		}, want: "exactly one"},
		{name: "resume rejects unknown flag", mutate: func(next *Next) {
			next.Variants[0].BaseArgv = append(next.Variants[0].BaseArgv, "--bogus", "x")
		}, want: "unsupported flag"},
		{name: "resume rejects missing flag value", mutate: func(next *Next) {
			next.Variants[0].BaseArgv = append(next.Variants[0].BaseArgv, "--source-file")
		}, want: "requires a value"},
		{name: "unknown resume variant", mutate: func(next *Next) { next.Variants[0].ID = "resume" }, want: "unsupported variant id"},
		{name: "skip action grammar mismatch", mutate: func(next *Next) { next.Variants[0].ID = "skip-action" }, want: "_machine skip"},
		{name: "skip action rejects extra argv", mutate: func(next *Next) {
			next.Variants[0].ID = "skip-action"
			next.Variants[0].BaseArgv = []string{
				"slipway", "_machine", "skip", "--run", "run-1", "--action", "action-1", "--root", workspace, "--extra",
			}
		}, want: "exact slipway _machine skip"},
		{name: "command cannot carry run grammar", mutate: func(next *Next) {
			next.Operation = NextOperationCommand
			next.Variants[0].BaseArgv = []string{"slipway", "run", "--root", workspace, "--", "goal"}
		}, want: "must not carry run"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			next := cloneNextForTest(valid)
			test.mutate(&next)
			err := next.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestNextValidationRejectsUnsafeOptionCombinations(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	digest := "sha256:" + strings.Repeat("a", 64)
	tests := []struct {
		name      string
		operation NextOperation
		argv      []string
		inputs    []NextInput
		want      string
	}{
		{
			name: "destructive confirmation without scope", operation: NextOperationAnswer,
			argv:   []string{"slipway", "_machine", "answer", "--run", "run-1", "--action", "action-1", "--root", workspace, "--confirm-destructive"},
			inputs: []NextInput{}, want: "together",
		},
		{
			name: "scope without destructive confirmation", operation: NextOperationAnswer,
			argv:   []string{"slipway", "_machine", "answer", "--run", "run-1", "--action", "action-1", "--root", workspace, "--scope-sha256", digest},
			inputs: []NextInput{}, want: "together",
		},
		{
			name: "invalid destructive scope digest", operation: NextOperationAnswer,
			argv:   []string{"slipway", "_machine", "answer", "--run", "run-1", "--action", "action-1", "--root", workspace, "--confirm-destructive", "--scope-sha256", "sha256:BAD"},
			inputs: []NextInput{}, want: "lowercase sha256",
		},
		{
			name: "zero resume budget", operation: NextOperationResume,
			argv:   []string{"slipway", "_machine", "resume", "run-1", "--root", workspace, "--budget", "0"},
			inputs: []NextInput{}, want: "positive base-10",
		},
		{
			name: "zero-padded resume budget", operation: NextOperationResume,
			argv:   []string{"slipway", "_machine", "resume", "run-1", "--root", workspace, "--budget", "01"},
			inputs: []NextInput{}, want: "canonical positive base-10",
		},
		{
			name: "oversized resume budget", operation: NextOperationResume,
			argv:   []string{"slipway", "_machine", "resume", "run-1", "--root", workspace, "--budget", "1001"},
			inputs: []NextInput{}, want: "no greater than 1000",
		},
		{
			name: "nonnumeric start budget", operation: NextOperationStart,
			argv:   []string{"slipway", "run", "--budget", "many", "--json", "--root", workspace, "--", "goal"},
			inputs: []NextInput{}, want: "positive base-10",
		},
		{
			name: "zero-padded start budget", operation: NextOperationStart,
			argv:   []string{"slipway", "run", "--budget", "01", "--json", "--root", workspace, "--", "goal"},
			inputs: []NextInput{}, want: "canonical positive base-10",
		},
		{
			name: "oversized start budget", operation: NextOperationStart,
			argv:   []string{"slipway", "run", "--budget", "1001", "--json", "--root", workspace, "--", "goal"},
			inputs: []NextInput{}, want: "no greater than 1000",
		},
		{
			name: "wrong start source type", operation: NextOperationStart,
			argv:   []string{"slipway", "run", "--budget", "4", "--json", "--root", workspace, "--", "goal"},
			inputs: []NextInput{{Name: "source_file", Type: NextInputString, Flag: "--source-file", Required: true}}, want: "required path",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewCommandNext(test.operation, workspace, "retry", test.argv, test.inputs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
	_, err := NewCommandNext(
		NextOperationStart,
		workspace,
		"retry-run",
		[]string{"slipway", "run", "--budget", "1000", "--json", "--root", workspace, "--", "goal"},
		nil,
	)
	require.NoError(t, err)
}

func cloneNextForTest(next Next) Next {
	clone := next
	clone.Variants = make([]NextVariant, len(next.Variants))
	copy(clone.Variants, next.Variants)
	for index := range clone.Variants {
		clone.Variants[index].BaseArgv = append([]string(nil), next.Variants[index].BaseArgv...)
		clone.Variants[index].Inputs = make([]NextInput, len(next.Variants[index].Inputs))
		copy(clone.Variants[index].Inputs, next.Variants[index].Inputs)
	}
	return clone
}

func indexOfString(values []string, value string) int {
	for index, candidate := range values {
		if candidate == value {
			return index
		}
	}
	return -1
}
