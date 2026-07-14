package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDoctorResponse struct {
	output []byte
	err    error
}

type fakeDoctorCommandRunner struct {
	path      string
	pathErr   error
	responses map[string]fakeDoctorResponse
	calls     []string
	bounded   bool
}

func (runner *fakeDoctorCommandRunner) LookPath(string) (string, error) {
	if runner.pathErr != nil {
		return "", runner.pathErr
	}
	return runner.path, nil
}

func (runner *fakeDoctorCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	key := doctorCommandKey(name, args...)
	runner.calls = append(runner.calls, key)
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > doctorCommandTimeout+time.Second {
		runner.bounded = false
	}
	response, found := runner.responses[key]
	if !found {
		return nil, fmt.Errorf("unexpected command: %s", key)
	}
	return response.output, response.err
}

func doctorCommandKey(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), "\x00")
}

func TestSystemDoctorRunnerRejectsUnexpectedExecutables(t *testing.T) {
	t.Parallel()
	_, err := (systemDoctorRunner{}).Run(t.Context(), "sh", "-c", "echo unexpected")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "doctor executable")
}

func TestGitHubDoctorChecksUseStableCodesAndBoundedArgv(t *testing.T) {
	const (
		root   = "/safe/repository"
		secret = "ghp_must_not_leak"
	)
	gh := "/safe/bin/gh"
	versionKey := doctorCommandKey(gh, "--version")
	authKey := doctorCommandKey(gh, "auth", "status", "--hostname", "github.com")
	originKey := doctorCommandKey("git", "-C", root, "remote", "get-url", "origin")
	permissionsKey := doctorCommandKey(gh, "api", "--hostname", "github.com", "repos/signalridge/slipway", "--jq", ".permissions")

	tests := []struct {
		name       string
		runner     *fakeDoctorCommandRunner
		wantCodes  []string
		wantStatus []string
	}{
		{
			name: "executable absent",
			runner: &fakeDoctorCommandRunner{
				pathErr: errors.New("not found"), bounded: true,
			},
			wantCodes:  []string{"github_cli_unavailable"},
			wantStatus: []string{"warning"},
		},
		{
			name: "old version and auth unavailable",
			runner: &fakeDoctorCommandRunner{
				path: gh, bounded: true,
				responses: map[string]fakeDoctorResponse{
					versionKey: {output: []byte("gh version 2.93.1 (2026-01-01)\n")},
					authKey:    {output: []byte(secret), err: errors.New(secret)},
				},
			},
			wantCodes:  []string{"github_cli_rest_fallback_required", "github_auth_unavailable"},
			wantStatus: []string{"warning", "warning"},
		},
		{
			name: "compatible with full permissions",
			runner: &fakeDoctorCommandRunner{
				path: gh, bounded: true,
				responses: map[string]fakeDoctorResponse{
					versionKey:     {output: []byte("gh version 2.94.0 (2026-01-01)\n")},
					authKey:        {},
					originKey:      {output: []byte("git@github.com:signalridge/slipway.git\n")},
					permissionsKey: {output: []byte(`{"pull":true,"triage":true,"push":true,"maintain":false,"admin":false}`)},
				},
			},
			wantCodes:  []string{"github_cli_compatible", "github_auth_available", "github_issue_permissions_ok"},
			wantStatus: []string{"ok", "ok", "ok"},
		},
		{
			name: "limited permissions",
			runner: &fakeDoctorCommandRunner{
				path: gh, bounded: true,
				responses: map[string]fakeDoctorResponse{
					versionKey:     {output: []byte("gh version 3.0.0\n")},
					authKey:        {},
					originKey:      {output: []byte("https://github.com/signalridge/slipway.git\n")},
					permissionsKey: {output: []byte(`{"pull":true,"triage":false,"push":false}`)},
				},
			},
			wantCodes:  []string{"github_cli_compatible", "github_auth_available", "github_issue_permissions_limited"},
			wantStatus: []string{"ok", "ok", "warning"},
		},
		{
			name: "permission query unavailable",
			runner: &fakeDoctorCommandRunner{
				path: gh, bounded: true,
				responses: map[string]fakeDoctorResponse{
					versionKey:     {output: []byte("gh version 2.95.0\n")},
					authKey:        {},
					originKey:      {output: []byte("ssh://git@github.com/signalridge/slipway.git\n")},
					permissionsKey: {output: []byte(secret), err: errors.New(secret)},
				},
			},
			wantCodes:  []string{"github_cli_compatible", "github_auth_available", "github_issue_permissions_unknown"},
			wantStatus: []string{"ok", "ok", "warning"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checks := githubDoctorChecks(context.Background(), root, test.runner)
			codes := make([]string, 0, len(checks))
			statuses := make([]string, 0, len(checks))
			for _, check := range checks {
				codes = append(codes, check.Code)
				statuses = append(statuses, check.Status)
				assert.NotEmpty(t, check.Code)
				assert.Equal(t, "-", check.HostID)
				assert.NotEmpty(t, check.Name)
				assert.NotEmpty(t, check.Detail)
			}
			assert.Equal(t, test.wantCodes, codes)
			assert.Equal(t, test.wantStatus, statuses)
			encoded, err := json.Marshal(checks)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), secret)
			assert.True(t, test.runner.bounded)
			for _, call := range test.runner.calls {
				assert.NotContains(t, call, "sh\x00-c")
				assert.NotContains(t, call, secret)
			}
		})
	}
}

func TestGitHubDoctorRejectsUnsafeOriginWithoutPermissionQuery(t *testing.T) {
	gh := "/safe/bin/gh"
	root := "/safe/repository"
	runner := &fakeDoctorCommandRunner{
		path: gh, bounded: true,
		responses: map[string]fakeDoctorResponse{
			doctorCommandKey(gh, "--version"):                                  {output: []byte("gh version 2.94.0\n")},
			doctorCommandKey(gh, "auth", "status", "--hostname", "github.com"): {},
			doctorCommandKey("git", "-C", root, "remote", "get-url", "origin"): {
				output: []byte("https://token@github.com/signalridge/slipway.git\n"),
			},
		},
	}

	checks := githubDoctorChecks(context.Background(), root, runner)
	assert.Equal(t, []string{"github_cli_compatible", "github_auth_available"}, doctorCodes(checks))
	assert.Len(t, runner.calls, 3)
}

func TestGitHubRepositoryParsingAcceptsOnlyCredentialFreeGitHubCoordinates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		remote string
		want   string
		ok     bool
	}{
		{remote: "git@github.com:signalridge/slipway.git", want: "signalridge/slipway", ok: true},
		{remote: "https://github.com/signalridge/slipway", want: "signalridge/slipway", ok: true},
		{remote: "ssh://git@github.com/signalridge/slipway.git", want: "signalridge/slipway", ok: true},
		{remote: "https://token@github.com/signalridge/slipway.git"},
		{remote: "ssh://git:secret@github.com/signalridge/slipway.git"},
		{remote: "https://github.com/signalridge/slipway.git?token=secret"},
		{remote: "https://example.com/signalridge/slipway.git"},
		{remote: "git@github.com:../slipway.git"},
		{remote: "https://github.com/signalridge/too/many.git"},
	}
	for _, test := range tests {
		t.Run(test.remote, func(t *testing.T) {
			actual, ok := githubRepositoryFromRemote([]byte(test.remote))
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestDoctorCommandEmitsVersionedExactChecksAndWarningsExitSuccessfully(t *testing.T) {
	repository := newCLIRepository(t)
	runner := &fakeDoctorCommandRunner{pathErr: errors.New("not found"), bounded: true}
	command := makeDoctorCmdWithRunner(runner)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"--root", repository, "--json"})

	require.NoError(t, command.Execute(), stderr.String())
	var output doctorOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &output))
	assert.Equal(t, machineContractVersion, output.ContractVersion)
	require.NotEmpty(t, output.Checks)
	assert.Contains(t, doctorCodes(output.Checks), "github_cli_unavailable")
	durabilityChecks := 0
	for _, check := range output.Checks {
		assert.NotEmpty(t, check.Code)
		assert.Contains(t, []string{"ok", "warning", "error"}, check.Status)
		expectedKeys := []string{"code", "status", "host_id", "name", "detail"}
		if check.Durability != nil {
			durabilityChecks++
			expectedKeys = append(expectedKeys, "durability")
			assert.Contains(t, []string{"runstore_durability_full", "runstore_durability_limited"}, check.Code)
			assert.Contains(t, []string{"file_and_directory_fsync", "file_fsync_only"}, check.Durability.Level)
			assert.True(t, check.Durability.FileSync)
			assert.Equal(t, check.Code == "runstore_durability_full", check.Durability.DirectorySync)
		}
		encoded, err := json.Marshal(check)
		require.NoError(t, err)
		var shape map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &shape))
		assert.ElementsMatch(t, expectedKeys, mapKeys(shape))
	}
	assert.Equal(t, 1, durabilityChecks)
}

func TestDoctorAggregationKeepsHostDurabilityAndGitHubChecksAfterInspectionError(t *testing.T) {
	repository := newCLIRepository(t)
	runner := &fakeDoctorCommandRunner{pathErr: errors.New("not found"), bounded: true}
	doctor := func(string) (adapter.DoctorReport, error) {
		return adapter.DoctorReport{Checks: []adapter.DoctorCheck{
			{Code: "adapter_inspection_unavailable", Status: "error", HostID: "claude", Name: "adapter inspection", Detail: "injected inspection failure"},
			{Code: "adapter_healthy", Status: "ok", HostID: "codex", Name: "adapter", Detail: "8 managed files"},
		}}, nil
	}

	report, err := collectDoctorReportWithAdapterDoctor(context.Background(), repository, runner, doctor)
	require.NoError(t, err)
	codes := doctorCodes(report.Checks)
	assert.Contains(t, codes, "adapter_inspection_unavailable")
	assert.Contains(t, codes, "adapter_healthy")
	assert.Contains(t, codes, "github_cli_unavailable")
	assert.Condition(t, func() bool {
		return containsString(codes, "runstore_durability_full") || containsString(codes, "runstore_durability_limited")
	})
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func doctorCodes(checks []adapter.DoctorCheck) []string {
	codes := make([]string, 0, len(checks))
	for _, check := range checks {
		codes = append(codes, check.Code)
	}
	return codes
}

func mapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
