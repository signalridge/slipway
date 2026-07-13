package runstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverWorkspaceIdentityIsCanonicalDeterministicAndWorktreeSpecific(t *testing.T) {
	repository := createRepository(t)
	identity, err := DiscoverWorkspaceIdentity(filepath.Join(repository, "README.md"))
	require.NoError(t, err)
	require.NoError(t, identity.Validate())
	canonicalRepository, err := filepath.EvalSymlinks(repository)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(canonicalRepository), identity.WorktreeRoot)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, identity.ID)

	repeated, err := DiscoverWorkspaceIdentity(repository)
	require.NoError(t, err)
	assert.Equal(t, identity, repeated)

	linked := filepath.Join(t.TempDir(), "linked worktree")
	runGitCommand(t, repository, "worktree", "add", "--detach", linked, "HEAD")
	linkedIdentity, err := DiscoverWorkspaceIdentity(linked)
	require.NoError(t, err)
	require.NoError(t, linkedIdentity.Validate())
	assert.Equal(t, identity.GitCommonDir, linkedIdentity.GitCommonDir)
	assert.NotEqual(t, identity.WorktreeRoot, linkedIdentity.WorktreeRoot)
	assert.NotEqual(t, identity.GitDir, linkedIdentity.GitDir)
	assert.NotEqual(t, identity.ID, linkedIdentity.ID)
}

func TestWorkspaceIdentityValidationBindsDigestToEveryCanonicalPath(t *testing.T) {
	t.Parallel()
	repository := createRepository(t)
	identity, err := DiscoverWorkspaceIdentity(repository)
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(*WorkspaceIdentity)
	}{
		{name: "version", mutate: func(value *WorkspaceIdentity) { value.Version++ }},
		{name: "relative worktree", mutate: func(value *WorkspaceIdentity) { value.WorktreeRoot = "relative" }},
		{name: "git directory", mutate: func(value *WorkspaceIdentity) { value.GitDir = filepath.Join(value.GitDir, "other") }},
		{name: "common directory", mutate: func(value *WorkspaceIdentity) { value.GitCommonDir = filepath.Join(value.GitCommonDir, "other") }},
		{name: "uppercase digest", mutate: func(value *WorkspaceIdentity) { value.ID = strings.ToUpper(value.ID) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := identity
			test.mutate(&changed)
			require.Error(t, changed.Validate())
		})
	}
}

func TestObserveGitUsesExactPorcelainV2AndIndexFingerprints(t *testing.T) {
	repository := createRepository(t)
	initial, err := ObserveGit(repository)
	require.NoError(t, err)
	require.NotNil(t, initial.DirtyFiles)
	require.NotNil(t, initial.PathObservations)

	tracked := filepath.Join(repository, "README.md")
	require.NoError(t, os.WriteFile(tracked, []byte("staged content\n"), 0o600))
	runGitCommand(t, repository, "add", "README.md")
	staged, err := ObserveGit(repository)
	require.NoError(t, err)
	index, err := gitBytes(repository, "ls-files", "--stage", "-z")
	require.NoError(t, err)
	status, err := gitBytes(repository, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	require.NoError(t, err)
	assert.Equal(t, digestBytes(index), staged.IndexFingerprint)
	assert.Equal(t, digestBytes(status), staged.StatusFingerprint)
	assert.True(t, staged.ChangedFrom(initial))
	require.Equal(t, []string{"README.md"}, staged.DirtyFiles)
	require.Len(t, staged.PathObservations, 1)
	assert.Equal(t, "ordinary", staged.PathObservations[0].Category)
	assert.Equal(t, "regular", staged.PathObservations[0].Observation)
	assert.Equal(t, digestBytes([]byte("staged content\n")), staged.PathObservations[0].ContentSHA256)

	require.NoError(t, os.WriteFile(tracked, []byte("unstaged content\n"), 0o600))
	unstaged, err := ObserveGit(repository)
	require.NoError(t, err)
	assert.Equal(t, staged.IndexFingerprint, unstaged.IndexFingerprint, "unstaged content must not alter the index fingerprint")
	assert.NotEqual(t, staged.StatusFingerprint, unstaged.StatusFingerprint)
	assert.True(t, unstaged.ChangedFrom(staged))
}

func TestObserveGitHashesUntrackedUnicodeContentAndRenameOrigins(t *testing.T) {
	repository := createRepository(t)
	untrackedName := "notes 空 with spaces.txt"
	untrackedPath := filepath.Join(repository, untrackedName)
	require.NoError(t, os.WriteFile(untrackedPath, []byte("first secret value\n"), 0o600))
	first, err := ObserveGit(repository)
	require.NoError(t, err)
	item := requirePathObservation(t, first, untrackedName)
	assert.Equal(t, "untracked", item.Category)
	assert.Equal(t, "??", item.State)
	assert.Equal(t, digestBytes([]byte("first secret value\n")), item.ContentSHA256)

	require.NoError(t, os.WriteFile(untrackedPath, []byte("second secret value\n"), 0o600))
	second, err := ObserveGit(repository)
	require.NoError(t, err)
	assert.True(t, second.ChangedFrom(first))

	renameDestination := "renamed 名 with spaces.md"
	runGitCommand(t, repository, "mv", "README.md", renameDestination)
	renamed, err := ObserveGit(repository)
	require.NoError(t, err)
	destination := requirePathObservation(t, renamed, renameDestination)
	origin := requirePathObservation(t, renamed, "README.md")
	assert.Equal(t, "rename", destination.Category)
	assert.Equal(t, "regular", destination.Observation)
	assert.Equal(t, "rename_origin", origin.Category)
	assert.Equal(t, "missing", origin.Observation)
}

func TestParsePorcelainV2PreservesRecordKindsAndNULFraming(t *testing.T) {
	raw := strings.Join([]string{
		"1 M. N... 100644 100644 100644 a b ordinary path.txt",
		"2 R. N... 100644 100644 100644 a b R100 renamed 名.txt",
		"origin name.txt",
		"2 C. N... 100644 100644 100644 a b C75 copied path.txt",
		"copy origin.txt",
		"u UU N... 100644 100644 100644 100644 a b c conflict path.txt",
		"? untracked 空.txt",
		"",
	}, "\x00")
	parsed, err := parsePorcelainV2([]byte(raw))
	require.NoError(t, err)
	parsed = deduplicateStatusPaths(parsed)

	actual := make(map[string]statusPath, len(parsed))
	for _, item := range parsed {
		actual[item.path] = item
	}
	assert.Equal(t, statusPath{path: "ordinary path.txt", category: "ordinary", state: "M."}, actual["ordinary path.txt"])
	assert.Equal(t, statusPath{path: "renamed 名.txt", category: "rename", state: "R. R100"}, actual["renamed 名.txt"])
	assert.Equal(t, statusPath{path: "origin name.txt", category: "rename_origin", state: "R. R100"}, actual["origin name.txt"])
	assert.Equal(t, statusPath{path: "copied path.txt", category: "copy", state: "C. C75"}, actual["copied path.txt"])
	assert.Equal(t, statusPath{path: "copy origin.txt", category: "copy_origin", state: "C. C75"}, actual["copy origin.txt"])
	assert.Equal(t, statusPath{path: "conflict path.txt", category: "unmerged", state: "UU"}, actual["conflict path.txt"])
	assert.Equal(t, statusPath{path: "untracked 空.txt", category: "untracked", state: "??"}, actual["untracked 空.txt"])
}

func TestObservePathRecordsMissingUnreadableNonRegularAndOversize(t *testing.T) {
	repository := createRepository(t)
	root, err := os.OpenRoot(repository)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	missing := observePath(root, statusPath{path: "missing.txt", category: "ordinary", state: ".D"})
	assert.Equal(t, "missing", missing.Observation)
	assert.Nil(t, missing.Size)
	assert.Empty(t, missing.ContentSHA256)

	directory := filepath.Join(repository, "directory")
	require.NoError(t, os.Mkdir(directory, 0o700))
	nonRegular := observePath(root, statusPath{path: "directory", category: "untracked", state: "??"})
	assert.Equal(t, "non_regular", nonRegular.Observation)
	assert.Empty(t, nonRegular.ContentSHA256)

	oversizePath := filepath.Join(repository, "oversize.bin")
	file, err := os.OpenFile(oversizePath, os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(t, err)
	require.NoError(t, file.Truncate(MaxObservedFileBytes+1))
	require.NoError(t, file.Close())
	oversize := observePath(root, statusPath{path: "oversize.bin", category: "untracked", state: "??"})
	assert.Equal(t, "oversize", oversize.Observation)
	require.NotNil(t, oversize.Size)
	assert.Equal(t, MaxObservedFileBytes+1, *oversize.Size)
	assert.True(t, validSHA256(oversize.ContentSHA256))

	if runtime.GOOS != "windows" {
		unreadablePath := filepath.Join(repository, "unreadable.txt")
		require.NoError(t, os.WriteFile(unreadablePath, []byte("private\n"), 0o600))
		require.NoError(t, os.Chmod(unreadablePath, 0))
		t.Cleanup(func() { _ = os.Chmod(unreadablePath, 0o600) })
		unreadable := observePath(root, statusPath{path: "unreadable.txt", category: "untracked", state: "??"})
		if unreadable.Observation == "regular" {
			t.Log("current user can read mode-000 files; unreadable assertion is not feasible")
		} else {
			assert.Equal(t, "unreadable", unreadable.Observation)
			assert.Empty(t, unreadable.ContentSHA256)
		}
	}
}

func TestObserveGitIsDeterministicAndNeverRetainsRawContent(t *testing.T) {
	repository := createRepository(t)
	const secret = "raw-value-that-must-not-enter-the-observation"
	require.NoError(t, os.WriteFile(filepath.Join(repository, "secret.txt"), []byte(secret), 0o600))

	first, err := ObserveGit(repository)
	require.NoError(t, err)
	second, err := ObserveGit(repository)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	require.NoError(t, first.Validate())

	encoded, err := json.Marshal(first)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), secret)
	assert.Contains(t, string(encoded), digestBytes([]byte(secret)))
}

func requirePathObservation(t *testing.T, observation GitObservation, path string) PathObservation {
	t.Helper()
	for _, item := range observation.PathObservations {
		if item.Path == filepath.ToSlash(path) {
			return item
		}
	}
	require.FailNow(t, "path observation not found", path)
	return PathObservation{}
}
