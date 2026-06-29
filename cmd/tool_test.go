package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolMergeSARIFDeterministicAndSeparatedByTool(t *testing.T) {
	raw := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(raw, "a.sarif"), []byte(`{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"R1"}]}},
	    "results":[{"ruleId":"R1","locations":[{"physicalLocation":{"artifactLocation":{"uri":"x.go"},"region":{"startLine":1}}}]}]}]}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(raw, "b.sarif"), []byte(`{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"CodeQL","rules":[{"id":"R1"}]}},
	    "results":[{"ruleId":"R1","locations":[{"physicalLocation":{"artifactLocation":{"uri":"x.go"},"region":{"startLine":1}}}]}]}]}`), 0o644))

	outPath := filepath.Join(t.TempDir(), "merged.sarif")
	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "Merged 2 SARIF file(s)")

	first, err := os.ReadFile(outPath)
	require.NoError(t, err)
	_, stderr, err = runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.NoError(t, err, stderr)
	second, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))

	var merged struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Name string `json:"name"`
				} `json:"driver"`
			} `json:"tool"`
		} `json:"runs"`
	}
	require.NoError(t, json.Unmarshal(first, &merged))
	require.Len(t, merged.Runs, 2)
	assert.Equal(t, "CodeQL", merged.Runs[0].Tool.Driver.Name)
	assert.Equal(t, "semgrep", merged.Runs[1].Tool.Driver.Name)
}

func TestToolMergeSARIFRejectsUnsupportedVersion(t *testing.T) {
	raw := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(raw, "old.sarif"), []byte(`{"version":"2.0.0","runs":[]}`), 0o644))

	outPath := filepath.Join(t.TempDir(), "merged.sarif")
	_, stderr, err := runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "merge_sarif_version_unsupported")
}

func TestToolPinActionsNoPartialWriteOnUnresolved(t *testing.T) {
	dir := t.TempDir()
	mapping := filepath.Join(dir, "pins.tsv")
	require.NoError(t, os.WriteFile(mapping, []byte("actions/checkout@v4\tb4ffde65f46336ab88eb53be808477a3936bae11\n"), 0o644))

	workflow := filepath.Join(dir, "ci.yml")
	original := "jobs:\n  x:\n    steps:\n      - uses: custom/act@v9\n"
	require.NoError(t, os.WriteFile(workflow, []byte(original), 0o644))

	_, stderr, err := runRootCommandWithInput([]string{"tool", "pin-actions", "--mapping", mapping, workflow}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "pin_actions_unresolved")

	after, readErr := os.ReadFile(workflow)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(after))
}

func TestToolPinActionsBatchNoPartialWriteOnLaterUnresolved(t *testing.T) {
	dir := t.TempDir()
	mapping := filepath.Join(dir, "pins.tsv")
	require.NoError(t, os.WriteFile(mapping, []byte("actions/checkout@v4\tb4ffde65f46336ab88eb53be808477a3936bae11\n"), 0o644))

	first := filepath.Join(dir, "first.yml")
	firstOriginal := "jobs:\n  x:\n    steps:\n      - uses: actions/checkout@v4\n"
	require.NoError(t, os.WriteFile(first, []byte(firstOriginal), 0o644))
	second := filepath.Join(dir, "second.yml")
	secondOriginal := "jobs:\n  x:\n    steps:\n      - uses: custom/act@v9\n"
	require.NoError(t, os.WriteFile(second, []byte(secondOriginal), 0o644))

	_, stderr, err := runRootCommandWithInput([]string{"tool", "pin-actions", "--mapping", mapping, first, second}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "pin_actions_unresolved")

	afterFirst, readErr := os.ReadFile(first)
	require.NoError(t, readErr)
	assert.Equal(t, firstOriginal, string(afterFirst))
	afterSecond, readErr := os.ReadFile(second)
	require.NoError(t, readErr)
	assert.Equal(t, secondOriginal, string(afterSecond))
}

func TestToolPinActionsUnreadableWorkflowFailsBeforeWrites(t *testing.T) {
	dir := t.TempDir()
	mapping := filepath.Join(dir, "pins.tsv")
	require.NoError(t, os.WriteFile(mapping, []byte("actions/checkout@v4\tb4ffde65f46336ab88eb53be808477a3936bae11\n"), 0o644))

	workflow := filepath.Join(dir, "ci.yml")
	original := "jobs:\n  x:\n    steps:\n      - uses: actions/checkout@v4\n"
	require.NoError(t, os.WriteFile(workflow, []byte(original), 0o644))
	notAFile := filepath.Join(dir, "not-a-file")
	require.NoError(t, os.Mkdir(notAFile, 0o755))

	_, stderr, err := runRootCommandWithInput([]string{"tool", "pin-actions", "--mapping", mapping, workflow, notAFile}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "pin_actions_workflow_unreadable")

	after, readErr := os.ReadFile(workflow)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(after))
}

func TestToolFindVariantScaffoldsSemgrepGo(t *testing.T) {
	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "find-variant", "--engine=semgrep", "--language=go"}, "")
	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "id: variant-taint-go")
	assert.Contains(t, stdout, "mode: taint")
	assert.Contains(t, stdout, "languages: [go]")
	assert.Contains(t, stdout, "TODO(source)")
	assert.Contains(t, stdout, "TODO(sink)")
	assert.Contains(t, stdout, "TODO(sanitizer)")
}

func TestToolFindVariantValidatesInputsAndScaffoldsCodeQLPython(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    string
		wantStdout []string
	}{
		{
			name:    "missing required flags",
			args:    []string{"tool", "find-variant"},
			wantErr: "find_variant_missing_required_flags",
		},
		{
			name:    "unsupported engine",
			args:    []string{"tool", "find-variant", "--engine=unknown", "--language=go"},
			wantErr: "find_variant_unsupported",
		},
		{
			name: "codeql python scaffold",
			args: []string{"tool", "find-variant", "--engine=codeql", "--language=python"},
			wantStdout: []string{
				"@kind path-problem",
				"import python",
				"import semmle.python.dataflow.new.TaintTracking",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runRootCommandWithInput(tt.args, "")
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, stderr, tt.wantErr)
				return
			}
			require.NoError(t, err, stderr)
			for _, want := range tt.wantStdout {
				assert.Contains(t, stdout, want)
			}
		})
	}
}

func TestToolFindPolluterGoMissingGo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, stderr, err := runRootCommandWithInput([]string{"tool", "find-polluter-go", filepath.Join(t.TempDir(), "pollution"), "./..."}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "find_polluter_go_missing_go")
}

func TestToolFindPolluterGoRejectsExistingPollutionPath(t *testing.T) {
	pollutionPath := filepath.Join(t.TempDir(), "pollution")
	require.NoError(t, os.WriteFile(pollutionPath, []byte("already here"), 0o644))

	_, stderr, err := runRootCommandWithInput([]string{"tool", "find-polluter-go", pollutionPath, "./..."}, "")
	require.Error(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Equal(t, "find_polluter_go_pollution_present", payload["error_code"])
	assert.Contains(t, fmt.Sprint(payload["message"]), pollutionPath)
	details, ok := payload["details"].(map[string]any)
	require.True(t, ok, "details must be present")
	assert.Equal(t, pollutionPath, details["path"])
}

func TestToolFindPolluterGoFailsWhenGoListFails(t *testing.T) {
	_, stderr, err := runRootCommandWithInput([]string{"tool", "find-polluter-go", filepath.Join(t.TempDir(), "pollution"), "./definitely-missing-package/..."}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "find_polluter_go_list_failed")
}

func TestToolFindPolluterGoRejectsNoTestPackages(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/notests\n\ngo 1.21\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "pkg.go"), []byte("package pkg\n"), 0o644))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() {
		require.NoError(t, os.Chdir(previousWD))
	}()

	_, stderr, err := runRootCommandWithInput([]string{"tool", "find-polluter-go", filepath.Join(dir, "pollution"), "./..."}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "find_polluter_go_no_tests")
}

func TestToolReplyToThreadDryRunDoesNotRequireToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "reply-to-thread", "PRRT_abc", "dry-run body"}, "")
	require.Error(t, err)
	assert.Contains(t, stdout, "DRY-RUN")
	assert.Contains(t, stdout, "addPullRequestReviewThreadReply")
	assert.Contains(t, stdout, "PRRT_abc")
	assert.Contains(t, stderr, "reply_to_thread_dry_run")
}

func TestToolFetchPRChecksUsesTokenHTTP(t *testing.T) {
	var seenAuth string
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/7":
			_, _ = w.Write([]byte(`{"number":7,"head":{"sha":"abc123"}}`))
		case "/repos/owner/repo/commits/abc123/check-runs":
			_, _ = w.Write([]byte(`{"total_count":0,"check_runs":[]}`))
		case "/repos/owner/repo/commits/abc123/status":
			_, _ = w.Write([]byte(`{"state":"success","statuses":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")
	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "fetch-pr-checks", "--backend", "api", "--repo", "owner/repo", "--pr", "7"}, "")
	require.NoError(t, err, stderr)
	assert.Equal(t, "Bearer test-token", seenAuth)
	assert.Contains(t, stdout, `"head_sha": "abc123"`)
	assert.True(t, strings.Contains(stdout, `"total_count": 0`), stdout)
}

func TestToolFetchPRChecksRequiresExplicitRepo(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	_, stderr, err := runRootCommandWithInput([]string{"tool", "fetch-pr-checks", "--pr", "7"}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "github_repo_required")
}

func TestToolGitHubBackendHelpDisclosesAPIURL(t *testing.T) {
	tests := []string{
		"fetch-pr-checks",
		"fetch-pr-feedback",
		"fetch-review-requests",
		"reply-to-thread",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := runRootCommandWithInput([]string{"tool", name, "--help"}, "")
			require.NoError(t, err)
			assert.Empty(t, stderr)
			assert.Contains(t, stdout, "SLIPWAY_GITHUB_API_URL")
			assert.Contains(t, stdout, "--backend api / token-backed HTTP path")
			assert.Contains(t, stdout, "https://api.github.com")
		})
	}
}

func TestToolGitHubFlagOnlyHelpersRejectUnexpectedArgsBeforeToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "fetch-pr-checks",
			args: []string{"tool", "fetch-pr-checks", "--repo", "owner/repo", "--pr", "7", "unexpected"},
		},
		{
			name: "fetch-pr-feedback",
			args: []string{"tool", "fetch-pr-feedback", "--repo", "owner/repo", "--pr", "7", "unexpected"},
		},
		{
			name: "fetch-review-requests",
			args: []string{"tool", "fetch-review-requests", "--org", "owner-org", "--teams", "team-a", "unexpected"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, err := runRootCommandWithInput(tt.args, "")
			require.Error(t, err)
			assert.Contains(t, stderr, "invalid_usage")
			assert.NotContains(t, stderr, "github_token_missing")
		})
	}
}

func TestGitHubRepoValidationRejectsDotOnlySegments(t *testing.T) {
	tests := []struct {
		name string
		repo string
		want bool
	}{
		{name: "normal", repo: "owner/repo", want: true},
		{name: "repo punctuation", repo: "owner-name/repo.name", want: true},
		{name: "owner dot", repo: "./repo", want: false},
		{name: "owner dot dot", repo: "../repo", want: false},
		{name: "repo dot", repo: "owner/.", want: false},
		{name: "repo dot dot", repo: "owner/..", want: false},
		{name: "both traversal", repo: "../..", want: false},
		{name: "extra slash", repo: "owner/repo/extra", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, validGitHubRepo(tt.repo))
		})
	}
}

func TestGitHubBackendSelection(t *testing.T) {
	originalLookPath := githubLookPath
	t.Cleanup(func() { githubLookPath = originalLookPath })

	tests := []struct {
		name     string
		mode     string
		ghPath   string
		ghErr    error
		token    string
		wantName string
		wantErr  string
	}{
		{
			name:     "auto prefers gh when present",
			mode:     "auto",
			ghPath:   "/usr/local/bin/gh",
			wantName: "gh",
		},
		{
			name:     "auto falls back to token api when gh missing",
			mode:     "auto",
			ghErr:    errors.New("not found"),
			token:    "test-token",
			wantName: "api",
		},
		{
			name:    "auto fails closed without gh or token",
			mode:    "auto",
			ghErr:   errors.New("not found"),
			wantErr: "github_auth_unavailable",
		},
		{
			name:    "explicit gh does not fall back",
			mode:    "gh",
			ghErr:   errors.New("not found"),
			token:   "test-token",
			wantErr: "github_gh_missing",
		},
		{
			name:    "explicit api requires token",
			mode:    "api",
			ghPath:  "/usr/local/bin/gh",
			wantErr: "github_token_missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", tt.token)
			t.Setenv("GITHUB_TOKEN", "")
			githubLookPath = func(name string) (string, error) {
				require.Equal(t, "gh", name)
				if tt.ghErr != nil {
					return "", tt.ghErr
				}
				return tt.ghPath, nil
			}

			backend, err := newGitHubBackend(context.Background(), tt.mode, model.ConfigGitHub{})
			if tt.wantErr != "" {
				require.Error(t, err)
				var cliErr *CLIError
				require.True(t, errors.As(err, &cliErr), "expected CLIError, got %T", err)
				assert.Equal(t, tt.wantErr, cliErr.ErrorCode)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, backend.backendName())
		})
	}
}

func TestGitHubAPIOverrideRejectsUnsafeURLs(t *testing.T) {
	t.Setenv(githubAmbientTokenPrimaryEnv, "ambient-token")
	t.Setenv(githubAmbientTokenSecondaryEnv, "")
	t.Setenv(githubAPIOverrideTokenEnv, "override-token")
	t.Setenv(githubAPIAllowedBaseURLsEnv, "")

	tests := []struct {
		name     string
		baseURL  string
		wantCode string
	}{
		{
			name:     "http rejected",
			baseURL:  "http://api.github.com",
			wantCode: "github_api_url_invalid",
		},
		{
			name:     "unknown https host rejected",
			baseURL:  "https://api.github.invalid",
			wantCode: "github_api_url_not_allowed",
		},
		{
			name:     "path-confused public host rejected",
			baseURL:  "https://api.github.com/evil",
			wantCode: "github_api_url_invalid",
		},
		{
			name:     "path-confused public host with default port rejected",
			baseURL:  "https://api.github.com:443/evil",
			wantCode: "github_api_url_invalid",
		},
		{
			name:     "path-confused mixed-case public host with default port rejected",
			baseURL:  "https://API.GITHUB.COM:443/evil",
			wantCode: "github_api_url_invalid",
		},
		{
			name:     "query rejected",
			baseURL:  "https://api.github.com?x=1",
			wantCode: "github_api_url_invalid",
		},
		{
			name:     "userinfo rejected",
			baseURL:  "https://token@api.github.com",
			wantCode: "github_api_url_invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(githubAPIURLEnv, tt.baseURL)
			_, err := newGitHubHTTPClient(model.ConfigGitHub{})
			require.Error(t, err)
			var cliErr *CLIError
			require.True(t, errors.As(err, &cliErr), "expected CLIError, got %T", err)
			assert.Equal(t, tt.wantCode, cliErr.ErrorCode)
		})
	}
}

func TestGitHubAPIOverrideRejectsAllowlistedPublicPathConfusion(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{
			name:    "path",
			baseURL: "https://api.github.com/evil",
		},
		{
			name:    "default port path",
			baseURL: "https://api.github.com:443/evil",
		},
		{
			name:    "mixed-case default port path",
			baseURL: "https://API.GITHUB.COM:443/evil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(githubAPIURLEnv, tt.baseURL)
			t.Setenv(githubAPIAllowedBaseURLsEnv, tt.baseURL)
			t.Setenv(githubAPIOverrideTokenEnv, "override-token")
			t.Setenv(githubAmbientTokenPrimaryEnv, "ambient-token")
			t.Setenv(githubAmbientTokenSecondaryEnv, "")

			_, err := newGitHubHTTPClient(model.ConfigGitHub{})
			require.Error(t, err)
			var cliErr *CLIError
			require.True(t, errors.As(err, &cliErr), "expected CLIError, got %T", err)
			assert.Equal(t, "github_api_url_invalid", cliErr.ErrorCode)
		})
	}
}

func TestGitHubAPIOverrideRequiresOverrideTokenAndDoesNotUseAmbient(t *testing.T) {
	var hits int
	var seenAuth string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		seenAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"number":7}`)
	}))
	defer server.Close()
	previousTransport := githubHTTPTransport
	githubHTTPTransport = server.Client().Transport
	t.Cleanup(func() { githubHTTPTransport = previousTransport })

	t.Setenv(githubAPIURLEnv, server.URL)
	t.Setenv(githubAPIAllowedBaseURLsEnv, server.URL)
	t.Setenv(githubAmbientTokenPrimaryEnv, "ambient-token")
	t.Setenv(githubAmbientTokenSecondaryEnv, "secondary-ambient-token")
	t.Setenv(githubAPIOverrideTokenEnv, "")

	_, err := newGitHubHTTPClient(model.ConfigGitHub{})
	require.Error(t, err)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr), "expected CLIError, got %T", err)
	assert.Equal(t, "github_api_override_token_missing", cliErr.ErrorCode)
	assert.Equal(t, 0, hits, "ambient tokens must not be sent to override hosts")

	t.Setenv(githubAPIOverrideTokenEnv, "override-token")
	client, err := newGitHubHTTPClient(model.ConfigGitHub{})
	require.NoError(t, err)
	var out struct {
		Number int `json:"number"`
	}
	require.NoError(t, client.getJSON("/repos/o/r/pulls/7", &out))
	assert.Equal(t, 7, out.Number)
	assert.Equal(t, "Bearer override-token", seenAuth)
}

// TestGitHubAPIConfigFileFallback proves the env > file > default precedence:
// when SLIPWAY_GITHUB_API_URL is unset, github.api_url from .slipway.yaml is
// used and its allowlist (github.api_allowed_base_urls) authorizes the override;
// when the env var IS set it wins over the file value.
func TestGitHubAPIConfigFileFallback(t *testing.T) {
	var seenAuth string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"number":11}`)
	}))
	defer server.Close()
	previousTransport := githubHTTPTransport
	githubHTTPTransport = server.Client().Transport
	t.Cleanup(func() { githubHTTPTransport = previousTransport })

	// No env override URL/allowlist; the file config supplies both.
	t.Setenv(githubAPIURLEnv, "")
	t.Setenv(githubAPIAllowedBaseURLsEnv, "")
	t.Setenv(githubAPIOverrideTokenEnv, "override-token")
	t.Setenv(githubAmbientTokenPrimaryEnv, "ambient-token")
	t.Setenv(githubAmbientTokenSecondaryEnv, "")

	fileCfg := model.ConfigGitHub{
		APIURL:             server.URL,
		APIAllowedBaseURLs: []string{server.URL},
	}
	client, err := newGitHubHTTPClient(fileCfg)
	require.NoError(t, err, "file github.api_url + allowlist should authorize the override")
	var out struct {
		Number int `json:"number"`
	}
	require.NoError(t, client.getJSON("/repos/o/r/pulls/11", &out))
	assert.Equal(t, 11, out.Number)
	assert.Equal(t, "Bearer override-token", seenAuth)

	// A file override host NOT in the file allowlist is rejected.
	_, err = newGitHubHTTPClient(model.ConfigGitHub{APIURL: server.URL})
	require.Error(t, err)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Equal(t, "github_api_url_not_allowed", cliErr.ErrorCode)

	// Env URL overrides the file value: an env override that is NOT allowlisted
	// (env list empty, file allowlist only covers the file URL) is rejected,
	// proving the env value, not the file value, drove resolution.
	t.Setenv(githubAPIURLEnv, "https://ghe.env-only.example/api/v3")
	_, err = newGitHubHTTPClient(fileCfg)
	require.Error(t, err)
	require.True(t, errors.As(err, &cliErr))
	assert.Equal(t, "github_api_url_not_allowed", cliErr.ErrorCode)
}

func TestGitHubAPIOverrideRejectsUnsafePaginationLink(t *testing.T) {
	tests := []struct {
		name string
		link string
	}{
		{
			name: "cross host",
			link: "https://attacker.example/repos/owner/repo/issues?per_page=100&page=2",
		},
		{
			name: "base path escape",
			link: "https://api.enterprise.example/api/repos/owner/repo/issues?per_page=100&page=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests []*http.Request
			client := githubHTTPClient{
				baseURL: "https://api.enterprise.example/api/v3",
				token:   "override-token",
				client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests = append(requests, req.Clone(req.Context()))
					resp := &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`[]`)),
						Request:    req,
					}
					resp.Header.Set("Link", fmt.Sprintf("<%s>; rel=\"next\"", tt.link))
					return resp, nil
				})},
			}

			_, err := client.getPaginated("/repos/owner/repo/issues")
			require.Error(t, err)
			var cliErr *CLIError
			require.True(t, errors.As(err, &cliErr), "expected CLIError, got %T", err)
			assert.Equal(t, "github_api_pagination_url_not_allowed", cliErr.ErrorCode)
			require.Len(t, requests, 1, "unsafe Link target must be rejected before any token-bearing follow-up request")
			assert.Equal(t, "api.enterprise.example", requests[0].URL.Host)
			assert.Equal(t, "Bearer override-token", requests[0].Header.Get("Authorization"))
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGitHubCLIBackendUsesGHAPIEndpointArgs(t *testing.T) {
	originalRunCLI := githubRunCLI
	t.Cleanup(func() { githubRunCLI = originalRunCLI })

	var gotPath string
	var gotArgs []string
	githubRunCLI = func(_ context.Context, path string, args ...string) ([]byte, error) {
		gotPath = path
		gotArgs = append([]string(nil), args...)
		return []byte(`{"number":7}`), nil
	}

	var out struct {
		Number int `json:"number"`
	}
	client := githubCLIClient{ctx: context.Background(), path: "/opt/bin/gh"}
	require.NoError(t, client.getJSON("/repos/owner/repo/pulls/7", &out))

	assert.Equal(t, "/opt/bin/gh", gotPath)
	assert.Equal(t, []string{"api", "repos/owner/repo/pulls/7"}, gotArgs)
	assert.Equal(t, 7, out.Number)
}

func TestGitHubCLIBackendGraphQLUsesFlagArgs(t *testing.T) {
	originalRunCLI := githubRunCLI
	t.Cleanup(func() { githubRunCLI = originalRunCLI })

	var gotArgs []string
	githubRunCLI = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		gotArgs = append([]string(nil), args...)
		return []byte(`{"data":{"ok":true}}`), nil
	}

	client := githubCLIClient{ctx: context.Background(), path: "/opt/bin/gh"}
	data, err := client.postGraphQLChecked("query Example { viewer { login } }", map[string]any{
		"body":     "looks good",
		"count":    2,
		"threadID": "PRRT_abc",
	})

	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(data))
	assert.Equal(t, []string{
		"api", "graphql",
		"-f", "query=query Example { viewer { login } }",
		"-f", "body=looks good",
		"-F", "count=2",
		"-f", "threadID=PRRT_abc",
	}, gotArgs)
}

// readBody returns the request body as a string (for routing GraphQL by op).
func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	return string(raw)
}

func newGitHubAPITestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	previousTransport := githubHTTPTransport
	githubHTTPTransport = server.Client().Transport
	t.Cleanup(func() {
		githubHTTPTransport = previousTransport
		server.Close()
	})
	t.Setenv(githubAPIURLEnv, server.URL)
	t.Setenv(githubAPIAllowedBaseURLsEnv, server.URL)
	t.Setenv(githubAPIOverrideTokenEnv, "test-token")
	t.Setenv(githubAmbientTokenPrimaryEnv, "")
	t.Setenv(githubAmbientTokenSecondaryEnv, "")
	return server
}

// --- reply-to-thread --confirm -------------------------------------------------

func TestToolReplyToThreadConfirmSuccessReturnsCommentID(t *testing.T) {
	var sawAuth string
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		require.Equal(t, "/graphql", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":{"addPullRequestReviewThreadReply":{"comment":{"id":"IC_123","url":"https://github.com/o/r/pull/1#discussion_r1"}}}}`))
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "reply-to-thread", "--backend", "api", "--confirm", "PRRT_abc", "looks good"}, "")
	require.NoError(t, err, stderr)
	assert.Equal(t, "Bearer test-token", sawAuth)
	assert.Contains(t, stdout, `"comment_id": "IC_123"`)
	assert.Contains(t, stdout, `"replied": 1`)
	assert.Contains(t, stdout, `"status": "ok"`)
}

func TestToolReplyToThreadConfirmGraphQLErrorsNotReportedPosted(t *testing.T) {
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// HTTP 200 but GraphQL errors array is non-empty.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Could not resolve to a node with the global id of 'PRRT_bad'"}]}`))
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "reply-to-thread", "--backend", "api", "--confirm", "PRRT_bad", "hi"}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "reply_to_thread_partial_failure")
	assert.Contains(t, stderr, "Could not resolve to a node")
	assert.Contains(t, stdout, `"replied": 0`)
	assert.Contains(t, stdout, `"failed": 1`)
	assert.Contains(t, stdout, `"status": "failed"`)
}

func TestToolReplyToThreadConfirmNullPayloadFails(t *testing.T) {
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// HTTP 200, no errors, but the mutation payload is null/missing.
		_, _ = w.Write([]byte(`{"data":{"addPullRequestReviewThreadReply":null}}`))
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "reply-to-thread", "--backend", "api", "--confirm", "PRRT_abc", "hi"}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "reply_to_thread_partial_failure")
	assert.Contains(t, stdout, `"failed": 1`)
	assert.Contains(t, stdout, `"mutation returned no comment id"`)
	assert.NotContains(t, stdout, `"replied": 1`)
}

func TestToolReplyToThreadConfirmPartialFailureReportsPostedReply(t *testing.T) {
	var calls int
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"data":{"addPullRequestReviewThreadReply":{"comment":{"id":"IC_1","url":"https://github.com/o/r/pull/1#discussion_r1"}}}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Could not resolve to a node with the global id of 'PRRT_bad'"}]}`))
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "reply-to-thread", "--backend", "api", "--confirm", "PRRT_ok", "first", "PRRT_bad", "second"}, "")
	require.Error(t, err)
	assert.Equal(t, 2, calls)
	assert.Contains(t, stderr, "reply_to_thread_partial_failure")
	assert.Contains(t, stdout, `"replied": 1`)
	assert.Contains(t, stdout, `"failed": 1`)
	assert.Contains(t, stdout, `"comment_id": "IC_1"`)
	assert.Contains(t, stdout, `"thread_id": "PRRT_bad"`)
	assert.Contains(t, stdout, `"status": "failed"`)
}

// --- fetch-pr-feedback ---------------------------------------------------------

func TestToolFetchPRFeedbackCategorizesAndCarriesThreadIDs(t *testing.T) {
	reviewThreadsPayload := `{
      "data": {"repository": {"pullRequest": {"reviewDecision": "CHANGES_REQUESTED", "reviewThreads": {
        "pageInfo": {"hasNextPage": false, "endCursor": null},
        "nodes": [
          {"id": "PRRT_high", "isResolved": false, "isOutdated": false, "path": "a.go", "line": 10,
           "comments": {"nodes": [{"id": "c1", "body": "h: this must change before merge", "author": {"login": "alice"}}]}},
          {"id": "PRRT_low", "isResolved": false, "isOutdated": false, "path": "b.go", "line": 20,
           "comments": {"nodes": [{"id": "c2", "body": "l: nit: rename this", "author": {"login": "bob"}}]}},
          {"id": "PRRT_med", "isResolved": false, "isOutdated": false, "path": "c.go", "line": 30,
           "comments": {"nodes": [{"id": "c3", "body": "m: please double check the bounds", "author": {"login": "carol"}}]}},
          {"id": "PRRT_bot", "isResolved": false, "isOutdated": false, "path": "d.go", "line": 40,
           "comments": {"nodes": [{"id": "c4", "body": "Coverage decreased by 2%", "author": {"login": "codecov[bot]"}}]}},
          {"id": "PRRT_done", "isResolved": true, "isOutdated": false, "path": "e.go", "line": 50,
           "comments": {"nodes": [{"id": "c5", "body": "this was already fixed", "author": {"login": "dave"}}]}}
        ]
      }}}}
    }`

	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/graphql":
			body := readBody(t, r)
			require.Contains(t, body, "reviewThreads")
			require.Contains(t, body, "reviewDecision")
			_, _ = io.WriteString(w, reviewThreadsPayload)
		case r.URL.Path == "/repos/owner/repo/pulls/9":
			_, _ = io.WriteString(w, `{"number":9,"html_url":"https://github.com/owner/repo/pull/9","user":{"login":"prowner"}}`)
		case r.URL.Path == "/repos/owner/repo/pulls/9/comments":
			if isPageTwoOrLater(r) {
				_, _ = io.WriteString(w, `[]`)
				return
			}
			_, _ = io.WriteString(w, `[{"id":111,"body":"l: minor: spacing","user":{"login":"alice"},"path":"a.go"}]`)
		case r.URL.Path == "/repos/owner/repo/pulls/9/reviews":
			if isPageTwoOrLater(r) {
				_, _ = io.WriteString(w, `[]`)
				return
			}
			_, _ = io.WriteString(w, `[{"id":333,"state":"CHANGES_REQUESTED","body":"This must change before merge","user":{"login":"reviewer"},"html_url":"https://github.com/owner/repo/pull/9#pullrequestreview-333"}]`)
		case r.URL.Path == "/repos/owner/repo/issues/9/comments":
			if isPageTwoOrLater(r) {
				_, _ = io.WriteString(w, `[]`)
				return
			}
			_, _ = io.WriteString(w, `[{"id":222,"body":"This is a critical blocker for release","user":{"login":"erin"},"html_url":"https://github.com/owner/repo/pull/9#issuecomment-222"}]`)
		default:
			http.NotFound(w, r)
		}
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "fetch-pr-feedback", "--backend", "api", "--repo", "owner/repo", "--pr", "9"}, "")
	require.NoError(t, err, stderr)

	var out struct {
		PR struct {
			ReviewDecision string `json:"review_decision"`
		} `json:"pr"`
		Summary struct {
			High     int `json:"high"`
			Medium   int `json:"medium"`
			Low      int `json:"low"`
			Bot      int `json:"bot_comments"`
			Resolved int `json:"resolved"`
		} `json:"summary"`
		Feedback map[string][]map[string]any `json:"feedback"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	assert.Equal(t, "CHANGES_REQUESTED", out.PR.ReviewDecision)

	// Buckets populated: high (thread h: + issue "critical blocker"), medium (m:),
	// low (l: thread), bot (codecov), resolved (resolved thread). The
	// CHANGES_REQUESTED review body is also surfaced as high priority.
	assert.GreaterOrEqual(t, out.Summary.High, 1, stdout)
	assert.GreaterOrEqual(t, out.Summary.Medium, 1, stdout)
	assert.GreaterOrEqual(t, out.Summary.Low, 1, stdout)
	assert.GreaterOrEqual(t, out.Summary.Bot, 1, stdout)
	assert.GreaterOrEqual(t, out.Summary.Resolved, 1, stdout)

	// Thread ids must be present so the skill can call reply-to-thread.
	assert.Contains(t, stdout, `"thread_id": "PRRT_high"`)
	assert.Contains(t, stdout, `"thread_id": "PRRT_low"`)
	assert.Contains(t, stdout, `"thread_id": "PRRT_med"`)

	// resolved item carries its resolved flag and thread id.
	require.NotEmpty(t, out.Feedback["resolved"])
	assert.Equal(t, "PRRT_done", out.Feedback["resolved"][0]["thread_id"])
	assert.Equal(t, true, out.Feedback["resolved"][0]["resolved"])

	var sawChangesRequestedReview bool
	for _, item := range out.Feedback["high"] {
		if item["type"] == "changes_requested" && item["author"] == "reviewer" {
			sawChangesRequestedReview = true
		}
	}
	assert.True(t, sawChangesRequestedReview, stdout)
}

// --- fetch-review-requests -----------------------------------------------------

func TestToolFetchReviewRequestsFiltersClosedAndBuildsReasons(t *testing.T) {
	mux := http.NewServeMux()
	server := newGitHubAPITestServer(t, mux)
	base := server.URL

	mux.HandleFunc("/orgs/acme/teams/team-a/members", func(w http.ResponseWriter, r *http.Request) {
		if isPageTwoOrLater(r) {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		_, _ = io.WriteString(w, `[{"login":"author-open"}]`)
	})
	mux.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		if isPageTwoOrLater(r) {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		// Subject URLs are absolute REST PR URLs on this same server, so the
		// helper resolves them back to /repos/.../pulls/N here.
		_, _ = io.WriteString(w, fmt.Sprintf(`[
              {"id":"n-open","reason":"review_requested","unread":true,
               "subject":{"title":"Open PR","url":"%s/repos/owner/repo/pulls/100"}},
              {"id":"n-team","reason":"review_requested","unread":true,
               "subject":{"title":"Open PR via team","url":"%s/repos/owner/repo/pulls/200"}},
              {"id":"n-closed","reason":"review_requested","unread":true,
               "subject":{"title":"Closed PR","url":"%s/repos/owner/repo/pulls/300"}},
              {"id":"n-issue","reason":"review_requested","unread":true,
               "subject":{"title":"An Issue","url":"%s/repos/owner/repo/issues/400"}},
              {"id":"n-read","reason":"review_requested","unread":false,
               "subject":{"title":"Already read","url":"%s/repos/owner/repo/pulls/100"}}
            ]`, base, base, base, base, base))
	})
	mux.HandleFunc("/repos/owner/repo/pulls/100", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"merged_at":null,"state":"open","user":{"login":"author-open"}}`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls/100/requested_reviewers", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"users":[],"teams":[]}`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls/200", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"merged_at":null,"state":"open","user":{"login":"someone-else"}}`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls/200/requested_reviewers", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"users":[],"teams":[{"slug":"team-a"}]}`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls/300", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"merged_at":"2026-01-01T00:00:00Z","state":"closed","user":{"login":"someone-else"}}`)
	})

	t.Setenv("SLIPWAY_GITHUB_API_URL", base)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "fetch-review-requests", "--backend", "api", "--org", "acme", "--teams", "team-a"}, "")
	require.NoError(t, err, stderr)

	var out struct {
		Total int `json:"total"`
		PRs   []struct {
			Repo    string   `json:"repo"`
			Number  int      `json:"number"`
			Title   string   `json:"title"`
			URL     string   `json:"url"`
			Author  string   `json:"author"`
			Reasons []string `json:"reasons"`
		} `json:"prs"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)

	// Closed/merged PR (300) dropped; two open PRs kept (100 author-match, 200 team-match).
	assert.Equal(t, 2, out.Total, stdout)
	byNumber := map[int]struct {
		Reasons []string
		Author  string
	}{}
	for _, p := range out.PRs {
		byNumber[p.Number] = struct {
			Reasons []string
			Author  string
		}{p.Reasons, p.Author}
	}
	require.Contains(t, byNumber, 100)
	require.Contains(t, byNumber, 200)
	assert.NotContains(t, byNumber, 300)

	// 100 kept because author is a team member -> "opened by:" reason.
	assert.Equal(t, "author-open", byNumber[100].Author)
	assert.Contains(t, strings.Join(byNumber[100].Reasons, "|"), "opened by: author-open")

	// 200 kept because team-a is a requested reviewer -> "review requested from:" reason.
	assert.Contains(t, strings.Join(byNumber[200].Reasons, "|"), "review requested from: team-a")
}

func TestToolFetchReviewRequestsRequiresExplicitOrg(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	_, stderr, err := runRootCommandWithInput([]string{"tool", "fetch-review-requests", "--teams", "team-a"}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "fetch_review_requests_org_required")
	assert.NotContains(t, stderr, "github_token_missing")
}

// isPageTwoOrLater reports whether the request asks for ?page=2 or higher, used
// by test handlers to terminate page-number pagination with an empty page.
func isPageTwoOrLater(r *http.Request) bool {
	page := r.URL.Query().Get("page")
	return page != "" && page != "1"
}

// --- fetch-pr-checks -----------------------------------------------------------

func TestToolFetchPRChecksSurfacesFailureSnippetAndPaginates(t *testing.T) {
	var checkRunPages int
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/7":
			_, _ = io.WriteString(w, `{"number":7,"html_url":"https://github.com/owner/repo/pull/7","head":{"sha":"abc123","ref":"feature"},"base":{"ref":"main"}}`)
		case "/repos/owner/repo/commits/abc123/check-runs":
			checkRunPages++
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				// First page advertises a rel="next" Link header (pagination).
				w.Header().Set("Link", fmt.Sprintf(`<%s/repos/owner/repo/commits/abc123/check-runs?per_page=100&page=2>; rel="next"`, baseFromRequest(r)))
				_, _ = io.WriteString(w, `{"total_count":2,"check_runs":[{"id":555,"name":"build","status":"completed","conclusion":"failure","html_url":"https://github.com/owner/repo/runs/555","output":{"summary":"build failed"}}]}`)
				return
			}
			// Second page: one more (passing) run, no further Link.
			_, _ = io.WriteString(w, `{"total_count":2,"check_runs":[{"id":556,"name":"lint","status":"completed","conclusion":"success"}]}`)
		case "/repos/owner/repo/commits/abc123/status":
			_, _ = io.WriteString(w, `{"state":"success","statuses":[]}`)
		case "/repos/owner/repo/check-runs/555/annotations":
			if isPageTwoOrLater(r) {
				_, _ = io.WriteString(w, `[]`)
				return
			}
			_, _ = io.WriteString(w, `[{"path":"pkg/foo.go","start_line":42,"end_line":42,"annotation_level":"failure","message":"undefined: Bar"}]`)
		default:
			http.NotFound(w, r)
		}
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "fetch-pr-checks", "--backend", "api", "--repo", "owner/repo", "--pr", "7"}, "")
	require.NoError(t, err, stderr)

	// Pagination followed: at least 2 pages of check-runs requested.
	assert.GreaterOrEqual(t, checkRunPages, 2, "expected >=2 check-run pages, got %d", checkRunPages)

	var out struct {
		Summary struct {
			Total  int `json:"total"`
			Passed int `json:"passed"`
			Failed int `json:"failed"`
		} `json:"summary"`
		Failed []struct {
			ID       int `json:"id"`
			Snippets []struct {
				Location string `json:"location"`
				Message  string `json:"message"`
				Snippet  string `json:"snippet"`
			} `json:"snippets"`
		} `json:"failed"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)

	assert.Equal(t, 2, out.Summary.Total, stdout)
	assert.Equal(t, 1, out.Summary.Failed, stdout)
	assert.Equal(t, 1, out.Summary.Passed, stdout)

	require.Len(t, out.Failed, 1)
	require.NotEmpty(t, out.Failed[0].Snippets)
	// file:line + message snippet from the annotation.
	assert.Equal(t, "pkg/foo.go:42", out.Failed[0].Snippets[0].Location)
	assert.Contains(t, out.Failed[0].Snippets[0].Message, "undefined: Bar")
	assert.Contains(t, stdout, "pkg/foo.go:42")
}

func TestToolFetchPRChecksSurfacesFailedCommitStatuses(t *testing.T) {
	var requestedStatus bool
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/7":
			_, _ = io.WriteString(w, `{"number":7,"html_url":"https://github.com/owner/repo/pull/7","head":{"sha":"abc123","ref":"feature"},"base":{"ref":"main"}}`)
		case "/repos/owner/repo/commits/abc123/check-runs":
			_, _ = io.WriteString(w, `{"total_count":0,"check_runs":[]}`)
		case "/repos/owner/repo/commits/abc123/status":
			requestedStatus = true
			_, _ = io.WriteString(w, `{"state":"failure","statuses":[{"context":"legacy-ci","state":"failure","description":"legacy status failed","target_url":"https://ci.example.test/1"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	stdout, stderr, err := runRootCommandWithInput([]string{"tool", "fetch-pr-checks", "--backend", "api", "--repo", "owner/repo", "--pr", "7"}, "")
	require.NoError(t, err, stderr)
	assert.True(t, requestedStatus, "expected combined commit status endpoint to be requested")

	var out struct {
		Summary struct {
			Total  int `json:"total"`
			Failed int `json:"failed"`
		} `json:"summary"`
		Failed []struct {
			Type    string `json:"type"`
			Context string `json:"context"`
			State   string `json:"state"`
		} `json:"failed"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	assert.Equal(t, 1, out.Summary.Total, stdout)
	assert.Equal(t, 1, out.Summary.Failed, stdout)
	require.Len(t, out.Failed, 1)
	assert.Equal(t, "commit_status", out.Failed[0].Type)
	assert.Equal(t, "legacy-ci", out.Failed[0].Context)
	assert.Equal(t, "failure", out.Failed[0].State)
}

// baseFromRequest reconstructs the server base URL from a request for building
// absolute Link headers.
func baseFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// mergedSARIFDoc is a minimal decode target shared by the remap tests.
type mergedSARIFDoc struct {
	Runs []struct {
		Tool struct {
			Driver struct {
				Name  string `json:"name"`
				Rules []struct {
					ID string `json:"id"`
				} `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Artifacts []struct {
			Location struct {
				URI string `json:"uri"`
			} `json:"location"`
		} `json:"artifacts"`
		Results []struct {
			RuleID    string `json:"ruleId"`
			RuleIndex *int   `json:"ruleIndex"`
			Rule      *struct {
				Index *int `json:"index"`
			} `json:"rule"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI   string `json:"uri"`
						Index *int   `json:"index"`
					} `json:"artifactLocation"`
					Region struct {
						StartLine int `json:"startLine"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

func runMergeSARIFForTest(t *testing.T, files map[string]string) []byte {
	t.Helper()
	raw := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(raw, name), []byte(body), 0o644))
	}
	outPath := filepath.Join(t.TempDir(), "merged.sarif")
	_, stderr, err := runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.NoError(t, err, stderr)
	got, err := os.ReadFile(outPath)
	require.NoError(t, err)
	return got
}

// TestToolMergeSARIFRemapsRuleIndexAcrossRuns proves that when two runs of the
// SAME tool/profile merge into one run, results that reference rules by
// ruleIndex (including one with ruleId absent, to exercise backfill) resolve to
// the CORRECT rule after the merged rules table is deduped and id-sorted.
func TestToolMergeSARIFRemapsRuleIndexAcrossRuns(t *testing.T) {
	// Run A declares rules in id order [A1, A2]. Run B declares rules in a
	// DIFFERENT order [B2, B1] and references them by ruleIndex. After merge the
	// deduped id-sorted table is [A1, A2, B1, B2].
	runA := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"A1"},{"id":"A2"}]}},
	    "results":[
	      {"ruleId":"A1","ruleIndex":0,"locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.go"},"region":{"startLine":1}}}]}
	    ]}]}`
	// Run B: index 0 -> B2, index 1 -> B1. First result omits ruleId entirely
	// (must be backfilled from source rules[0].id == "B2").
	runB := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"B2"},{"id":"B1"}]}},
	    "results":[
	      {"ruleIndex":0,"locations":[{"physicalLocation":{"artifactLocation":{"uri":"b.go"},"region":{"startLine":2}}}]},
	      {"ruleId":"B1","ruleIndex":1,"locations":[{"physicalLocation":{"artifactLocation":{"uri":"b.go"},"region":{"startLine":3}}}]}
	    ]}]}`

	got := runMergeSARIFForTest(t, map[string]string{"a.sarif": runA, "b.sarif": runB})

	var doc mergedSARIFDoc
	require.NoError(t, json.Unmarshal(got, &doc))
	require.Len(t, doc.Runs, 1, "same tool/profile must merge into one run")
	run := doc.Runs[0]

	// Merged deduped id-sorted rule table.
	gotRuleIDs := make([]string, 0, len(run.Tool.Driver.Rules))
	for _, r := range run.Tool.Driver.Rules {
		gotRuleIDs = append(gotRuleIDs, r.ID)
	}
	assert.Equal(t, []string{"A1", "A2", "B1", "B2"}, gotRuleIDs)

	require.Len(t, run.Results, 3)
	for _, res := range run.Results {
		require.NotEmpty(t, res.RuleID, "ruleId must be present/backfilled so consumers are not forced onto the index")
		require.NotNil(t, res.RuleIndex, "ruleIndex retained when resolvable")
		idx := *res.RuleIndex
		require.GreaterOrEqual(t, idx, 0)
		require.Less(t, idx, len(run.Tool.Driver.Rules))
		assert.Equal(t, res.RuleID, run.Tool.Driver.Rules[idx].ID,
			"merged rules[result.ruleIndex].id must equal result.ruleId")
	}

	// Spot-check the backfilled result specifically: the run-B result with no
	// ruleId must have been backfilled to B2 and point at the merged B2 slot.
	var backfilled bool
	for _, res := range run.Results {
		if res.RuleID == "B2" {
			backfilled = true
			require.NotNil(t, res.RuleIndex)
			assert.Equal(t, "B2", run.Tool.Driver.Rules[*res.RuleIndex].ID)
		}
	}
	assert.True(t, backfilled, "expected a result backfilled to B2")
}

// TestToolMergeSARIFRemapsRuleObjectIndex covers the result-level rule.index
// surface (not just the top-level ruleIndex), including backfill of ruleId from
// the source run's rules[rule.index].
func TestToolMergeSARIFRemapsRuleObjectIndex(t *testing.T) {
	runA := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"codeql","rules":[{"id":"Q1"}]}},
	    "results":[{"ruleId":"Q1","locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.go"},"region":{"startLine":1}}}]}]}]}`
	// Run B references its rule via rule.index only, ruleId absent. Source
	// rules[0].id == "Q0" -> must backfill ruleId=Q0 and remap rule.index to the
	// merged slot for Q0 (merged order [Q0, Q1]).
	runB := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"codeql","rules":[{"id":"Q0"}]}},
	    "results":[{"rule":{"index":0},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"b.go"},"region":{"startLine":9}}}]}]}]}`

	got := runMergeSARIFForTest(t, map[string]string{"a.sarif": runA, "b.sarif": runB})

	var doc mergedSARIFDoc
	require.NoError(t, json.Unmarshal(got, &doc))
	require.Len(t, doc.Runs, 1)
	run := doc.Runs[0]

	gotRuleIDs := make([]string, 0, len(run.Tool.Driver.Rules))
	for _, r := range run.Tool.Driver.Rules {
		gotRuleIDs = append(gotRuleIDs, r.ID)
	}
	assert.Equal(t, []string{"Q0", "Q1"}, gotRuleIDs)

	var checkedQ0 bool
	for _, res := range run.Results {
		if res.RuleID == "Q0" {
			checkedQ0 = true
			require.NotNil(t, res.Rule)
			require.NotNil(t, res.Rule.Index)
			assert.Equal(t, "Q0", run.Tool.Driver.Rules[*res.Rule.Index].ID)
		}
	}
	assert.True(t, checkedQ0, "expected the rule.index-backfilled Q0 result")
}

// TestToolMergeSARIFDropsUnresolvableRuleIndex proves a result whose rule id
// cannot be resolved in the merged table keeps its ruleId but has the stale
// ruleIndex removed rather than left pointing at the wrong rule.
func TestToolMergeSARIFDropsUnresolvableRuleIndex(t *testing.T) {
	// Single run, but the result's ruleIndex points outside the run's rules
	// table and its ruleId ("GHOST") is not in any rule -> index is stale and
	// must be removed; ruleId retained.
	run := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"R1"}]}},
	    "results":[{"ruleId":"GHOST","ruleIndex":7,"locations":[{"physicalLocation":{"artifactLocation":{"uri":"x.go"},"region":{"startLine":1}}}]}]}]}`

	got := runMergeSARIFForTest(t, map[string]string{"a.sarif": run})

	var doc mergedSARIFDoc
	require.NoError(t, json.Unmarshal(got, &doc))
	require.Len(t, doc.Runs, 1)
	require.Len(t, doc.Runs[0].Results, 1)
	res := doc.Runs[0].Results[0]
	assert.Equal(t, "GHOST", res.RuleID, "ruleId retained")
	assert.Nil(t, res.RuleIndex, "stale ruleIndex must be removed, not left pointing at wrong rule")
}

// TestToolMergeSARIFRemapsArtifactIndex proves a result carrying an
// artifactLocation.index into a non-trivial artifacts table from a NON-FIRST run
// is remapped to the same uri in the merged artifacts table (or stripped),
// never left pointing at the wrong uri.
func TestToolMergeSARIFRemapsArtifactIndex(t *testing.T) {
	// Run A (first, sorts first by uri) has artifacts [a.go]. Run B has a
	// non-trivial artifacts table [zero.go, one.go, two.go]; its result indexes
	// into slot 2 (two.go). After merge the index must resolve to two.go.
	runA := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"R1"}]}},
	    "artifacts":[{"location":{"uri":"a.go"}}],
	    "results":[{"ruleId":"R1","locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.go","index":0},"region":{"startLine":1}}}]}]}]}`
	runB := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"R1"}]}},
	    "artifacts":[{"location":{"uri":"zero.go"}},{"location":{"uri":"one.go"}},{"location":{"uri":"two.go"}}],
	    "results":[{"ruleId":"R1","locations":[{"physicalLocation":{"artifactLocation":{"index":2},"region":{"startLine":42}}}]}]}]}`

	got := runMergeSARIFForTest(t, map[string]string{"a.sarif": runA, "b.sarif": runB})

	var doc mergedSARIFDoc
	require.NoError(t, json.Unmarshal(got, &doc))
	require.Len(t, doc.Runs, 1, "same tool/profile merges into one run")
	run := doc.Runs[0]

	// Merged artifacts table: first-seen across runs in run order = a.go (from A),
	// then zero.go, one.go, two.go (from B).
	gotURIs := make([]string, 0, len(run.Artifacts))
	for _, a := range run.Artifacts {
		gotURIs = append(gotURIs, a.Location.URI)
	}
	assert.Equal(t, []string{"a.go", "zero.go", "one.go", "two.go"}, gotURIs)

	// Find the result that came from run B (startLine 42) and assert its index
	// resolves to two.go in the merged table; uri must have been recovered.
	var checked bool
	for _, res := range run.Results {
		require.Len(t, res.Locations, 1)
		al := res.Locations[0].PhysicalLocation.ArtifactLocation
		if res.Locations[0].PhysicalLocation.Region.StartLine == 42 {
			checked = true
			if al.Index != nil {
				require.GreaterOrEqual(t, *al.Index, 0)
				require.Less(t, *al.Index, len(run.Artifacts))
				assert.Equal(t, "two.go", run.Artifacts[*al.Index].Location.URI,
					"artifact index must resolve to the original uri in the merged table")
				assert.Equal(t, "two.go", al.URI, "uri recovered from source artifacts table")
			} else {
				// Strip approach would have removed the index but kept uri.
				assert.Equal(t, "two.go", al.URI)
			}
		}
	}
	assert.True(t, checked, "expected to inspect the run-B result")
}

func TestToolMergeSARIFEmitsMinimalArtifactsWhenSourceOmitsArtifacts(t *testing.T) {
	run := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"R1"}]}},
	    "results":[{"ruleId":"R1","locations":[{"physicalLocation":{"artifactLocation":{"uri":"x.go"},"region":{"startLine":7}}}]}]}]}`

	got := runMergeSARIFForTest(t, map[string]string{"a.sarif": run})

	var doc mergedSARIFDoc
	require.NoError(t, json.Unmarshal(got, &doc))
	require.Len(t, doc.Runs, 1)
	mergedRun := doc.Runs[0]
	require.Len(t, mergedRun.Artifacts, 1)
	assert.Equal(t, "x.go", mergedRun.Artifacts[0].Location.URI)

	require.Len(t, mergedRun.Results, 1)
	require.Len(t, mergedRun.Results[0].Locations, 1)
	al := mergedRun.Results[0].Locations[0].PhysicalLocation.ArtifactLocation
	require.NotNil(t, al.Index, "artifactLocation.index should point into the synthesized artifacts table")
	assert.Equal(t, 0, *al.Index)
	assert.Equal(t, "x.go", al.URI)
}

// TestToolMergeSARIFRemapDeterministic merges twice and asserts byte-identical
// output, covering the rule+artifact remap paths.
func TestToolMergeSARIFRemapDeterministic(t *testing.T) {
	runA := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"A2"},{"id":"A1"}]}},
	    "artifacts":[{"location":{"uri":"a.go"}},{"location":{"uri":"b.go"}}],
	    "results":[
	      {"ruleIndex":0,"locations":[{"physicalLocation":{"artifactLocation":{"index":1},"region":{"startLine":5}}}]},
	      {"ruleId":"A1","ruleIndex":1,"locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.go","index":0},"region":{"startLine":7}}}]}
	    ]}]}`
	runB := `{
	  "version":"2.1.0",
	  "runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"A1"},{"id":"C1"}]}},
	    "artifacts":[{"location":{"uri":"c.go"}}],
	    "results":[{"ruleId":"C1","ruleIndex":1,"locations":[{"physicalLocation":{"artifactLocation":{"index":0},"region":{"startLine":3}}}]}]}]}`

	files := map[string]string{"a.sarif": runA, "b.sarif": runB}
	first := runMergeSARIFForTest(t, files)
	second := runMergeSARIFForTest(t, files)
	assert.Equal(t, string(first), string(second), "merge output must be byte-stable across reruns")

	// Sanity: the merged output still parses and every retained ruleIndex is
	// internally consistent.
	var doc mergedSARIFDoc
	require.NoError(t, json.Unmarshal(first, &doc))
	require.Len(t, doc.Runs, 1)
	run := doc.Runs[0]
	for _, res := range run.Results {
		if res.RuleIndex != nil {
			require.Less(t, *res.RuleIndex, len(run.Tool.Driver.Rules))
			assert.Equal(t, res.RuleID, run.Tool.Driver.Rules[*res.RuleIndex].ID)
		}
		for _, loc := range res.Locations {
			al := loc.PhysicalLocation.ArtifactLocation
			if al.Index != nil {
				require.Less(t, *al.Index, len(run.Artifacts))
				assert.Equal(t, al.URI, run.Artifacts[*al.Index].Location.URI)
			}
		}
	}
}

func TestToolMergeSARIFFailsClosedWhenAllInputsUnparseable(t *testing.T) {
	raw := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(raw, "broken.sarif"), []byte("{not valid json"), 0o644))

	outPath := filepath.Join(t.TempDir(), "merged.sarif")
	_, stderr, err := runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "merge_sarif_parse_failed")

	// Fail-closed: a scan whose only inputs were unparseable must not leave an
	// empty merged document behind that downstream evidence could treat as clean.
	_, statErr := os.Stat(outPath)
	assert.True(t, os.IsNotExist(statErr), "expected no merged output to be written on fail-closed")
}

func TestToolMergeSARIFFailsClosedWhenAnyInputUnparseable(t *testing.T) {
	raw := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(raw, "valid.sarif"), []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"semgrep"}},"results":[]}]}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(raw, "broken.sarif"), []byte("{not valid json"), 0o644))

	outPath := filepath.Join(t.TempDir(), "merged.sarif")
	_, stderr, err := runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.Error(t, err)
	assert.Contains(t, stderr, "merge_sarif_parse_failed")
	assert.Contains(t, stderr, "failed to parse")

	_, statErr := os.Stat(outPath)
	assert.True(t, os.IsNotExist(statErr), "expected no merged output when any input is unparseable")
}

func TestToolMergeSARIFZeroResultsStillSucceeds(t *testing.T) {
	raw := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(raw, "clean.sarif"), []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"semgrep"}},"results":[]}]}`), 0o644))

	outPath := filepath.Join(t.TempDir(), "merged.sarif")
	_, stderr, err := runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.NoError(t, err, stderr)

	// A valid SARIF that genuinely found nothing is a legitimate clean scan and
	// must still produce a merged document.
	_, statErr := os.Stat(outPath)
	require.NoError(t, statErr)
}

func TestToolMergeSARIFSortsResultsByNumericStartLine(t *testing.T) {
	raw := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(raw, "a.sarif"), []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"semgrep","rules":[{"id":"R1"}]}},"results":[
	  {"ruleId":"R1","locations":[{"physicalLocation":{"artifactLocation":{"uri":"x.go"},"region":{"startLine":10}}}]},
	  {"ruleId":"R1","locations":[{"physicalLocation":{"artifactLocation":{"uri":"x.go"},"region":{"startLine":2}}}]}
	]}]}`), 0o644))

	outPath := filepath.Join(t.TempDir(), "merged.sarif")
	_, stderr, err := runRootCommandWithInput([]string{"tool", "merge-sarif", raw, outPath}, "")
	require.NoError(t, err, stderr)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	var merged struct {
		Runs []struct {
			Results []struct {
				Locations []struct {
					PhysicalLocation struct {
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	require.NoError(t, json.Unmarshal(data, &merged))
	require.Len(t, merged.Runs, 1)
	require.Len(t, merged.Runs[0].Results, 2)
	// Numeric ordering: line 2 must precede line 10 (lexical keys would invert this).
	assert.Equal(t, 2, merged.Runs[0].Results[0].Locations[0].PhysicalLocation.Region.StartLine)
	assert.Equal(t, 10, merged.Runs[0].Results[1].Locations[0].PhysicalLocation.Region.StartLine)
}

func TestNormalizeReplyBody(t *testing.T) {
	t.Run("appends attribution and converts escaped newlines", func(t *testing.T) {
		assert.Equal(t, "line1\nline2\n\n*- Slipway*", normalizeReplyBody(`line1\nline2`, "Slipway"))
	})
	t.Run("appends custom attribution", func(t *testing.T) {
		assert.Equal(t, "body\n\n*- Codex*", normalizeReplyBody("body", "Codex"))
	})
	t.Run("allows attribution to be disabled", func(t *testing.T) {
		assert.Equal(t, "body", normalizeReplyBody("body", ""))
	})
	t.Run("does not double-append when already signed", func(t *testing.T) {
		body := "already signed\n\n*— Claude Code*"
		assert.Equal(t, body, normalizeReplyBody(body, "Slipway"))
	})
	t.Run("recognizes a hyphen signature line as signed", func(t *testing.T) {
		body := "x\n\n*- Some Bot*"
		assert.Equal(t, body, normalizeReplyBody(body, "Slipway"))
	})
}

func TestAutoGitHubBackendFallsBackToTokenOnGHAuthError(t *testing.T) {
	originalRunCLI := githubRunCLI
	t.Cleanup(func() { githubRunCLI = originalRunCLI })

	var ghCalls int
	githubRunCLI = func(context.Context, string, ...string) ([]byte, error) {
		ghCalls++
		return nil, newPreconditionError("github_gh_auth_required", "gh: not logged in", "run gh auth login", "", nil)
	}

	var httpHits int
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHits++
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"number":7}`))
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	backend := &autoGitHubBackend{cli: githubCLIClient{ctx: context.Background(), path: "/opt/bin/gh"}}
	var out struct {
		Number int `json:"number"`
	}
	require.NoError(t, backend.getJSON("/repos/o/r/pulls/7", &out))
	assert.Equal(t, 7, out.Number)
	assert.Equal(t, 1, ghCalls, "gh should be attempted exactly once")
	assert.Equal(t, 1, httpHits, "request should fall back to the token API")
	assert.Equal(t, "api", backend.backendName(), "backendName should reflect the backend that served the request")
}

func TestAutoGitHubBackendPropagatesGHAuthErrorWithoutToken(t *testing.T) {
	originalRunCLI := githubRunCLI
	t.Cleanup(func() { githubRunCLI = originalRunCLI })
	githubRunCLI = func(context.Context, string, ...string) ([]byte, error) {
		return nil, newPreconditionError("github_gh_auth_required", "gh: not logged in", "run gh auth login", "", nil)
	}

	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	backend := &autoGitHubBackend{cli: githubCLIClient{ctx: context.Background(), path: "/opt/bin/gh"}}
	var out struct {
		Number int `json:"number"`
	}
	err := backend.getJSON("/repos/o/r/pulls/7", &out)
	require.Error(t, err)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Equal(t, "github_gh_auth_required", cliErr.ErrorCode)
	assert.Equal(t, "gh", backend.backendName())
}

func TestAutoGitHubBackendDoesNotFallBackOnNonAuthError(t *testing.T) {
	originalRunCLI := githubRunCLI
	t.Cleanup(func() { githubRunCLI = originalRunCLI })
	githubRunCLI = func(context.Context, string, ...string) ([]byte, error) {
		return nil, newPreconditionError("github_gh_api_failed", "gh: server error", "retry later", "", nil)
	}

	var httpHits int
	server := newGitHubAPITestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpHits++
		_, _ = w.Write([]byte(`{"number":7}`))
	}))

	t.Setenv("SLIPWAY_GITHUB_API_URL", server.URL)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_TOKEN", "")

	backend := &autoGitHubBackend{cli: githubCLIClient{ctx: context.Background(), path: "/opt/bin/gh"}}
	var out struct {
		Number int `json:"number"`
	}
	err := backend.getJSON("/repos/o/r/pulls/7", &out)
	require.Error(t, err)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Equal(t, "github_gh_api_failed", cliErr.ErrorCode)
	assert.Equal(t, 0, httpHits, "non-auth gh failures must not silently fall back and mask the error")
}
