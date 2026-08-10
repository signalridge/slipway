package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusReadsPinnedRequirementsInEveryRunState covers the transparency gap
// that made a Run's own pinned Requirements unreadable the moment it stopped:
// `protocol material` is the execution path and only serves the current
// non-void Action, while `status --json` names the chapters without containing
// them. Inspection has to keep working after the Run can no longer execute,
// because that is exactly when a user asks what it was working from.
func TestStatusReadsPinnedRequirementsInEveryRunState(t *testing.T) {
	repository := newCLIRepository(t)
	sourcePath := writeCLISource(t, cliSourceEnvelope())
	stdout, stderr, err := executeForTest(
		t, "run", "inspect pinned requirements", "--root", repository, "--source-file", sourcePath, "--json",
	)
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)

	read := func(t *testing.T, section string) autopilot.PinnedMaterial {
		t.Helper()
		out, errOut, readErr := executeForTest(t, "status", action.RunID, "--root", repository, "--section", section, "--json")
		require.NoError(t, readErr, errOut)
		assertMachineSchemaOutput(t, "pinnedMaterial", out)
		var material autopilot.PinnedMaterial
		require.NoError(t, json.Unmarshal([]byte(out), &material))
		return material
	}

	active := read(t, "requirements")
	assert.Equal(t, autopilot.ContractVersion, active.ContractVersion)
	assert.Equal(t, "pinned_material", active.MessageType)
	assert.Equal(t, action.RunID, active.RunID)
	assert.Equal(t, autopilot.RunActive, active.RunState)
	assert.Equal(t, "requirements", active.Section.Key)
	assert.Contains(t, active.Section.Markdown, "Keep the exact CLI contract.")
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, active.RequirementsRevision)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, active.SourceRevision)

	_, stderr, err = executeForTest(t, "stop", action.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)

	stopped := read(t, "requirements")
	assert.Equal(t, autopilot.RunStopped, stopped.RunState)
	assert.Equal(t, active.Section, stopped.Section, "stopping must not change what the Run pinned")
	assert.Equal(t, active.RequirementsRevision, stopped.RequirementsRevision)

	// Every pinned role stays readable, not just requirements.
	for _, section := range []string{"outcome", "acceptance-examples", "constraints", "non-goals"} {
		assert.NotEmpty(t, read(t, section).Section.Markdown, section)
	}
}

func TestStatusPinnedSectionIsReadOnlyAndAuthorizesNothing(t *testing.T) {
	repository := newCLIRepository(t)
	sourcePath := writeCLISource(t, cliSourceEnvelope())
	stdout, stderr, err := executeForTest(
		t, "run", "read-only inspection", "--root", repository, "--source-file", sourcePath, "--json",
	)
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)

	before, err := os.ReadFile(filepath.Join(repository, ".git", "slipway", "runs", action.RunID, "journal.jsonl"))
	require.NoError(t, err)

	human, stderr, err := executeForTest(t, "status", action.RunID, "--root", repository, "--section", "requirements")
	require.NoError(t, err, stderr)
	assert.Contains(t, human, "Keep the exact CLI contract.")
	assert.Contains(t, human, "authorizes no work")
	assert.NotContains(t, human, "\"contract_version\"", "the human surface is not the machine surface")

	after, err := os.ReadFile(filepath.Join(repository, ".git", "slipway", "runs", action.RunID, "journal.jsonl"))
	require.NoError(t, err)
	assert.Equal(t, before, after, "inspection must not append a journal event")

	// The Run is untouched: its current Action still accepts its outcome.
	outcome := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts gathered")
	outcome.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Apply the change."}}
	_, stderr, err = executeForTest(
		t, "protocol", "submit", "--root", repository,
		"--run", action.RunID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, outcome),
	)
	require.NoError(t, err, stderr)
}

func TestStatusPinnedSectionRejectionsNameAnExecutableNext(t *testing.T) {
	repository := newCLIRepository(t)
	sourcePath := writeCLISource(t, cliSourceEnvelope())
	stdout, stderr, err := executeForTest(
		t, "run", "rejection shapes", "--root", repository, "--source-file", sourcePath, "--json",
	)
	require.NoError(t, err, stderr)
	issueBacked := decodeMutationAction(t, stdout)

	stdout, stderr, err = executeForTest(t, "run", "ad-hoc work", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	adHoc := decodeMutationAction(t, stdout)

	for _, test := range []struct {
		name    string
		args    []string
		code    string
		message string
	}{
		{
			name: "unknown section",
			args: []string{"status", issueBacked.RunID, "--root", repository, "--section", "not-a-chapter", "--json"},
			code: "material_section_not_found",
		},
		{
			name: "malformed section key",
			args: []string{"status", issueBacked.RunID, "--root", repository, "--section", "Not A Key", "--json"},
			code: "material_section_invalid",
		},
		{
			name: "empty section",
			args: []string{"status", issueBacked.RunID, "--root", repository, "--section", "", "--json"},
			code: "material_section_required",
		},
		{
			name: "ad-hoc run has no pinned source",
			args: []string{"status", adHoc.RunID, "--root", repository, "--section", "requirements", "--json"},
			code: "material_unavailable",
		},
		{
			name: "section without a run id",
			args: []string{"status", "--root", repository, "--section", "requirements", "--json"},
			code: "run_id_required",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := executeForTest(t, test.args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			assertMachineSchemaOutput(t, "cliError", stderr)
			var cliErr CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
			assert.Equal(t, test.code, cliErr.Code)
			require.NoError(t, cliErr.Next.Validate())
			assert.NotEmpty(t, cliErr.Next.Variants)
		})
	}
}
