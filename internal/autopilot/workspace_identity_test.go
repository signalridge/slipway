package autopilot

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRejectsWrongLinkedWorktreeBeforeLoadAndEveryMutation(t *testing.T) {
	repository := newTestRepository(t)
	owner := openTestService(t, repository)
	run := startTestRun(t, owner, 8, false)
	journal := filepath.Join(owner.store.CommonDir(), "slipway", "runs", run.ID, "journal.jsonl")
	before, err := os.ReadFile(journal)
	require.NoError(t, err)

	linked := filepath.Join(t.TempDir(), "other linked worktree")
	runGit(t, repository, "worktree", "add", "--detach", linked, "HEAD")
	other := openTestService(t, linked)
	assertWorkspaceMismatch := func(err error) {
		t.Helper()
		var protocolErr *ProtocolError
		require.ErrorAs(t, err, &protocolErr)
		assert.Equal(t, "workspace_identity_mismatch", protocolErr.Code)
		assert.Equal(t, NextOperationNone, protocolErr.Next.Operation)
		assert.Empty(t, protocolErr.Next.Variants)
		assert.Equal(t, run.WorkspaceIdentity.ID, protocolErr.Next.WorkspaceIdentity)
		assert.NotContains(t, protocolErr.Message, "retry")
	}

	_, err = other.Load(run.ID)
	assertWorkspaceMismatch(err)
	operations := []struct {
		name string
		call func() error
	}{
		{name: "submit", call: func() error {
			outcome := withEnvelope(run.CurrentAction.ActionID, Outcome{Status: OutcomeCompleted, Summary: "facts"})
			_, operationErr := other.Submit(run.ID, run.CurrentAction.ActionID, outcome)
			return operationErr
		}},
		{name: "answer", call: func() error {
			_, operationErr := other.Answer(run.ID, run.CurrentAction.ActionID, AnswerOptions{Text: "answer"})
			return operationErr
		}},
		{name: "skip", call: func() error {
			_, operationErr := other.Skip(run.ID, run.CurrentAction.ActionID)
			return operationErr
		}},
		{name: "stop", call: func() error {
			_, operationErr := other.Stop(run.ID)
			return operationErr
		}},
		{name: "resume", call: func() error {
			_, operationErr := other.Resume(run.ID, ResumeOptions{})
			return operationErr
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			assertWorkspaceMismatch(operation.call())
			after, readErr := os.ReadFile(journal)
			require.NoError(t, readErr)
			assert.Equal(t, before, after, "identity mismatch must occur before journal mutation")
		})
	}

	owned, err := owner.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.WorkspaceIdentity, owned.WorkspaceIdentity)
}

func TestServiceRejectsSamePathReusedForDifferentGitIdentityBeforeMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an open Git directory is not portable on Windows")
	}
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	oldCanonicalRoot := run.Workspace
	oldRootParent := filepath.Dir(oldCanonicalRoot)
	movedRoot := filepath.Join(oldRootParent, filepath.Base(oldCanonicalRoot)+"-original")
	require.NoError(t, os.Rename(oldCanonicalRoot, movedRoot))
	t.Cleanup(func() { _ = os.RemoveAll(movedRoot) })
	require.NoError(t, os.MkdirAll(oldCanonicalRoot, 0o700))
	replacementGitDir := movedRoot + "-replacement-git"
	t.Cleanup(func() { _ = os.RemoveAll(replacementGitDir) })
	runGit(t, oldCanonicalRoot, "init", "--separate-git-dir", replacementGitDir)
	runGit(t, oldCanonicalRoot, "config", "user.email", "replacement@example.com")
	runGit(t, oldCanonicalRoot, "config", "user.name", "Replacement")
	require.NoError(t, os.WriteFile(filepath.Join(oldCanonicalRoot, "README.md"), []byte("replacement\n"), 0o600))
	runGit(t, oldCanonicalRoot, "add", "README.md")
	runGit(t, oldCanonicalRoot, "commit", "-m", "replacement")

	oldJournal := filepath.Join(movedRoot, ".git", "slipway", "runs", run.ID, "journal.jsonl")
	before, err := os.ReadFile(oldJournal)
	require.NoError(t, err)
	_, err = service.Stop(run.ID)
	var protocolErr *ProtocolError
	require.True(t, errors.As(err, &protocolErr))
	assert.Equal(t, "workspace_identity_mismatch", protocolErr.Code)
	assert.Equal(t, NextOperationNone, protocolErr.Next.Operation)
	assert.Equal(t, run.WorkspaceIdentity.ID, protocolErr.Next.WorkspaceIdentity)
	after, readErr := os.ReadFile(oldJournal)
	require.NoError(t, readErr)
	assert.Equal(t, before, after)
}

func TestRunPersistsCanonicalWorkspaceIdentityAndImmutableInitialObservation(t *testing.T) {
	repository := newTestRepository(t)
	const preexistingContent = "preexisting raw content must not enter journal\n"
	require.NoError(t, os.WriteFile(filepath.Join(repository, "preexisting.txt"), []byte(preexistingContent), 0o600))
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	require.NoError(t, run.WorkspaceIdentity.Validate())
	assert.Equal(t, run.Workspace, run.WorkspaceIdentity.WorktreeRoot)
	initial := cloneGitObservation(run.InitialGit)
	journal := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID, "journal.jsonl")
	journalBytes, err := os.ReadFile(journal)
	require.NoError(t, err)
	assert.NotContains(t, string(journalBytes), preexistingContent)
	assert.Contains(t, string(journalBytes), run.InitialGit.PathObservations[0].ContentSHA256)

	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	require.NoError(t, os.WriteFile(filepath.Join(repository, "after.txt"), []byte("after\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{
		Status: OutcomeCompleted, Summary: "implemented",
		Implementation: implementationReport(ImplementationApplied, "after.txt"),
	})
	assert.Equal(t, initial, run.InitialGit)
	assert.True(t, run.CurrentGit.ChangedFrom(run.InitialGit))

	replayed, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.WorkspaceIdentity, replayed.WorkspaceIdentity)
	assert.Equal(t, initial, replayed.InitialGit)
	assert.Equal(t, run.CurrentGit, replayed.CurrentGit)
}
