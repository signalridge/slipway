package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/spf13/cobra"
)

const githubBackendEnvHelp = `Environment variables (env > file > default; see ` + "`slipway config list --env`" + `):
  SLIPWAY_GITHUB_API_URL  Override the GitHub REST/GraphQL API base URL used by
                          the --backend api / token-backed HTTP path (default
                          https://api.github.com). Overrides must be HTTPS and
                          allowlisted. Falls back to the github.api_url key in
                          .slipway.yaml when unset.
  SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS
                          Comma/space/semicolon separated HTTPS API base URLs
                          allowed for an API URL override. Falls back to the
                          github.api_allowed_base_urls key in .slipway.yaml when
                          unset. File-configured override URLs require this env
                          allowlist, or a matching SLIPWAY_GITHUB_API_URL, before
                          SLIPWAY_GITHUB_API_TOKEN is sent.
  SLIPWAY_GITHUB_API_TOKEN
                          Token used only for an allowed override URL. Ambient
                          GH_TOKEN/GITHUB_TOKEN are sent only to
                          https://api.github.com. Tokens are env-only and are
                          never read from .slipway.yaml. The gh backend ignores
                          these HTTP-only settings.`

func makeFetchPRChecksCmd() *cobra.Command {
	var backend string
	var repo string
	var pr int
	cmd := &cobra.Command{
		Use:   "fetch-pr-checks",
		Short: "Fetch GitHub PR check status using the selected GitHub backend",
		Long:  withGitHubBackendHelp("Fetch GitHub PR check status using the selected GitHub backend."),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFetchPRChecks(cmd, repo, pr, backend)
		},
	}
	addGitHubBackendFlag(cmd, &backend)
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository owner/name")
	cmd.Flags().IntVar(&pr, "pr", 0, "Pull request number")
	return cmd
}

func makeFetchPRFeedbackCmd() *cobra.Command {
	var backend string
	var repo string
	var pr int
	cmd := &cobra.Command{
		Use:   "fetch-pr-feedback",
		Short: "Fetch GitHub PR review feedback using the selected GitHub backend",
		Long:  withGitHubBackendHelp("Fetch GitHub PR review feedback using the selected GitHub backend."),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFetchPRFeedback(cmd, repo, pr, backend)
		},
	}
	addGitHubBackendFlag(cmd, &backend)
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository owner/name")
	cmd.Flags().IntVar(&pr, "pr", 0, "Pull request number")
	return cmd
}

func makeFetchReviewRequestsCmd() *cobra.Command {
	var backend string
	var org string
	var teams string
	cmd := &cobra.Command{
		Use:   "fetch-review-requests --org ORG --teams TEAM1,TEAM2",
		Short: "Fetch GitHub review-request notifications using the selected GitHub backend",
		Long:  withGitHubBackendHelp("Fetch GitHub review-request notifications using the selected GitHub backend."),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFetchReviewRequests(cmd, org, teams, backend)
		},
	}
	addGitHubBackendFlag(cmd, &backend)
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization slug for team membership lookup")
	cmd.Flags().StringVar(&teams, "teams", "", "Comma-separated requested team slugs")
	return cmd
}

func makeReplyToThreadCmd() *cobra.Command {
	var backend string
	var confirm bool
	var signature string
	cmd := &cobra.Command{
		Use:   "reply-to-thread [--confirm] THREAD_ID BODY [THREAD_ID BODY...]",
		Short: "Reply to GitHub PR review threads; dry-run by default",
		Long: withGitHubBackendHelp(`Reply to GitHub PR review threads.

Without --confirm, this command prints the intended GraphQL mutation and exits
non-zero with reply_to_thread_dry_run so automation does not mistake a preview
for a posted reply.`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || len(args)%2 != 0 {
				return newInvalidUsageError("reply_to_thread_usage", "Usage: slipway tool reply-to-thread [--confirm] THREAD_ID BODY [THREAD_ID BODY...]", "Pass one or more THREAD_ID BODY pairs.", nil)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReplyToThread(cmd, args, confirm, backend, signature)
		},
	}
	addGitHubBackendFlag(cmd, &backend)
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Post replies instead of printing a dry-run request")
	cmd.Flags().StringVar(&signature, "signature", "Slipway", "Attribution label appended to reply bodies; pass an empty string to disable")
	return cmd
}

func withGitHubBackendHelp(long string) string {
	return strings.TrimRight(long, "\n") + "\n\n" + githubBackendEnvHelp
}

const (
	githubBackendAPI   = "api"
	githubBackendAuto  = "auto"
	githubBackendGH    = "gh"
	githubBodyCap      = 16 << 20
	githubStderrCap    = 64 << 10
	githubRESTMaxPages = 100

	githubDefaultAPIBaseURL        = "https://api.github.com"
	githubAPIURLEnv                = "SLIPWAY_GITHUB_API_URL"
	githubAPIAllowedBaseURLsEnv    = "SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS"
	githubAPIOverrideTokenEnv      = "SLIPWAY_GITHUB_API_TOKEN" // #nosec G101 -- environment variable name, not a credential.
	githubAmbientTokenPrimaryEnv   = "GH_TOKEN"                 // #nosec G101 -- environment variable name, not a credential.
	githubAmbientTokenSecondaryEnv = "GITHUB_TOKEN"             // #nosec G101 -- environment variable name, not a credential.
)

var githubRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

var (
	githubLookPath      = exec.LookPath
	githubRunCLI        = runGitHubCLICommand
	githubHTTPTransport http.RoundTripper
)

type githubBackend interface {
	backendName() string
	getJSON(path string, out any) error
	getPaginated(path string) ([]map[string]any, error)
	getPaginatedCheckRuns(path string) (int, []map[string]any, error)
	getCombinedStatus(repoPath, sha string) (map[string]any, []map[string]any, error)
	postGraphQLChecked(query string, variables map[string]any) (json.RawMessage, error)
	summarizeFailedCheckRun(repoPath string, run map[string]any) map[string]any
}

func addGitHubBackendFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "backend", githubBackendAuto, "GitHub backend: auto, gh, or api")
}

// githubFileConfigFromCommand best-effort loads the repo-policy GitHub settings
// from .slipway.yaml for the command's project root. It is intentionally
// non-fatal: `slipway tool ...` GitHub helpers may run outside a governed
// workspace, so a missing root or unreadable config resolves to the zero
// ConfigGitHub (env-only behavior, unchanged from before this surface existed)
// rather than failing the GitHub call.
func githubFileConfigFromCommand(cmd *cobra.Command) model.ConfigGitHub {
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return model.ConfigGitHub{}
	}
	cfg, err := loadConfigAtRootWithStderr(root, cmd.ErrOrStderr())
	if err != nil {
		return model.ConfigGitHub{}
	}
	return cfg.GitHub
}

// newGitHubBackend builds the GitHub backend for mode. fileCfg carries the
// repo-policy GitHub settings loaded from .slipway.yaml (zero value when none /
// not in a workspace); the env vars still override these file values at
// resolution time (env > file > default).
func newGitHubBackend(ctx context.Context, mode string, fileCfg model.ConfigGitHub) (githubBackend, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", githubBackendAuto:
		if path, err := githubLookPath("gh"); err == nil && strings.TrimSpace(path) != "" {
			return &autoGitHubBackend{cli: githubCLIClient{ctx: ctx, path: path}, fileCfg: fileCfg}, nil
		}
		if githubTokenAvailable() {
			return newGitHubHTTPClient(fileCfg)
		}
		return nil, newPreconditionError(
			"github_auth_unavailable",
			"GitHub authentication unavailable: gh is not installed and GH_TOKEN/GITHUB_TOKEN is not set",
			"Install and authenticate GitHub CLI with `gh auth login`, or set GH_TOKEN/GITHUB_TOKEN and rerun with --backend api.",
			"",
			nil,
		)
	case githubBackendGH:
		path, err := githubLookPath("gh")
		if err != nil || strings.TrimSpace(path) == "" {
			return nil, newPreconditionError(
				"github_gh_missing",
				"GitHub CLI executable `gh` was not found",
				"Install GitHub CLI and run `gh auth login`, or use --backend api with GH_TOKEN/GITHUB_TOKEN.",
				"",
				nil,
			)
		}
		return githubCLIClient{ctx: ctx, path: path}, nil
	case githubBackendAPI:
		return newGitHubHTTPClient(fileCfg)
	default:
		return nil, newInvalidUsageError(
			"github_backend_invalid",
			fmt.Sprintf("invalid GitHub backend %q", mode),
			"Use --backend auto, --backend gh, or --backend api.",
			nil,
		)
	}
}

func githubTokenAvailable() bool {
	return strings.TrimSpace(os.Getenv(githubAmbientTokenPrimaryEnv)) != "" ||
		strings.TrimSpace(os.Getenv(githubAmbientTokenSecondaryEnv)) != "" ||
		strings.TrimSpace(os.Getenv(githubAPIOverrideTokenEnv)) != ""
}

// autoGitHubBackend implements `--backend auto`. It prefers the gh CLI and
// falls back to the token-backed HTTP API for an individual request only when
// that request fails specifically because gh is unauthenticated
// (github_gh_auth_required) and a GH_TOKEN/GITHUB_TOKEN is available.
//
// gh inherits this process's environment and itself honors
// GH_TOKEN/GITHUB_TOKEN, so when a token is present gh is normally already
// authenticated and this fallback never engages. It exists for the residual
// case where gh is installed but cannot use the token for a request (for
// example gh restricting an operation under GITHUB_TOKEN), so `auto` stays
// usable without forcing the caller to rerun with --backend api. The success
// path is a pure passthrough; the fallback only runs on an auth-classified gh
// error. Helper commands run a single request sequence per invocation, so the
// lazily-built HTTP client needs no synchronization.
type autoGitHubBackend struct {
	cli          githubBackend
	fileCfg      model.ConfigGitHub
	httpBE       githubBackend
	httpErr      error
	built        bool
	usedFallback bool
}

func (a *autoGitHubBackend) backendName() string {
	if a.usedFallback && a.httpBE != nil {
		return a.httpBE.backendName()
	}
	return a.cli.backendName()
}

// fallbackFor returns the HTTP backend when err is a gh auth failure and a
// token is available, lazily constructing the HTTP client once. newGitHubHTTPClient
// returns a concrete value (never a nil interface) even on error, so the
// decision is gated on httpErr rather than a nil check.
func (a *autoGitHubBackend) fallbackFor(err error) (githubBackend, bool) {
	if !isGitHubGHAuthError(err) || !githubTokenAvailable() {
		return nil, false
	}
	if !a.built {
		a.httpBE, a.httpErr = newGitHubHTTPClient(a.fileCfg)
		a.built = true
	}
	if a.httpErr != nil {
		return nil, false
	}
	a.usedFallback = true
	return a.httpBE, true
}

func (a *autoGitHubBackend) getJSON(path string, out any) error {
	err := a.cli.getJSON(path, out)
	if fb, ok := a.fallbackFor(err); ok {
		return fb.getJSON(path, out)
	}
	return err
}

func (a *autoGitHubBackend) getPaginated(path string) ([]map[string]any, error) {
	res, err := a.cli.getPaginated(path)
	if fb, ok := a.fallbackFor(err); ok {
		return fb.getPaginated(path)
	}
	return res, err
}

func (a *autoGitHubBackend) getPaginatedCheckRuns(path string) (int, []map[string]any, error) {
	total, res, err := a.cli.getPaginatedCheckRuns(path)
	if fb, ok := a.fallbackFor(err); ok {
		return fb.getPaginatedCheckRuns(path)
	}
	return total, res, err
}

func (a *autoGitHubBackend) getCombinedStatus(repoPath, sha string) (map[string]any, []map[string]any, error) {
	status, runs, err := a.cli.getCombinedStatus(repoPath, sha)
	if fb, ok := a.fallbackFor(err); ok {
		return fb.getCombinedStatus(repoPath, sha)
	}
	return status, runs, err
}

func (a *autoGitHubBackend) postGraphQLChecked(query string, variables map[string]any) (json.RawMessage, error) {
	res, err := a.cli.postGraphQLChecked(query, variables)
	if fb, ok := a.fallbackFor(err); ok {
		return fb.postGraphQLChecked(query, variables)
	}
	return res, err
}

func (a *autoGitHubBackend) summarizeFailedCheckRun(repoPath string, run map[string]any) map[string]any {
	// Route the inner annotation fetches through the auto backend so they share
	// the same gh-then-token fallback as the rest of the request sequence.
	return summarizeFailedCheckRun(a, repoPath, run)
}

func isGitHubGHAuthError(err error) bool {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.ErrorCode == "github_gh_auth_required"
	}
	return false
}

type githubHTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func (c githubHTTPClient) backendName() string { return githubBackendAPI }

func newGitHubHTTPClient(fileCfg model.ConfigGitHub) (githubHTTPClient, error) {
	cfg, err := resolveGitHubAPIConfig(fileCfg)
	if err != nil {
		return githubHTTPClient{}, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	if githubHTTPTransport != nil {
		client.Transport = githubHTTPTransport
	}
	return githubHTTPClient{baseURL: cfg.baseURL, token: cfg.token, client: client}, nil
}

type githubAPIConfig struct {
	baseURL string
	token   string
}

type githubAPIConfigSource string

const (
	githubAPIConfigSourceDefault githubAPIConfigSource = "default"
	githubAPIConfigSourceEnv     githubAPIConfigSource = "env"
	githubAPIConfigSourceFile    githubAPIConfigSource = "file"
)

type githubAPIBaseURLResolution struct {
	value  string
	source githubAPIConfigSource
}

type githubAPIAllowlistDecision struct {
	allowed bool
	source  githubAPIConfigSource
}

// resolveGitHubAPIConfig resolves the effective GitHub API base URL and token
// with env > file > default precedence: the base URL is the env override
// (SLIPWAY_GITHUB_API_URL) when set, else the file value (github.api_url), else
// https://api.github.com. An override host must be allowlisted by the env list
// (SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS) when set, else the file allowlist
// (github.api_allowed_base_urls). The override token stays env-only and is sent
// to a file-configured override only after an operator-controlled env value
// confirms the destination.
func resolveGitHubAPIConfig(fileCfg model.ConfigGitHub) (githubAPIConfig, error) {
	baseURL, err := resolveGitHubAPIBaseURL(fileCfg)
	if err != nil {
		return githubAPIConfig{}, err
	}
	override := baseURL.value != githubDefaultAPIBaseURL
	var allowlist githubAPIAllowlistDecision
	if override {
		allowlist, err = githubAPIBaseURLAllowed(baseURL.value, fileCfg)
		if err != nil {
			return githubAPIConfig{}, err
		}
		if !allowlist.allowed {
			return githubAPIConfig{}, newPreconditionError(
				"github_api_url_not_allowed",
				fmt.Sprintf("GitHub API override %q is not allowlisted", baseURL.value),
				"Allowlist the exact HTTPS API base URL via SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS or github.api_allowed_base_urls before using a github.api_url override.",
				"",
				map[string]any{"base_url": baseURL.value},
			)
		}
	}
	token := githubAPITokenForBaseURL(override)
	if token == "" {
		if override {
			return githubAPIConfig{}, newPreconditionError(
				"github_api_override_token_missing",
				"GitHub API override token missing; ambient GH_TOKEN/GITHUB_TOKEN are not sent to override hosts",
				"Set SLIPWAY_GITHUB_API_TOKEN for the allowed override host, or clear the SLIPWAY_GITHUB_API_URL / github.api_url override to use https://api.github.com.",
				"",
				map[string]any{"base_url": baseURL.value},
			)
		}
		return githubAPIConfig{}, newPreconditionError(
			"github_token_missing",
			"GitHub token missing; set GH_TOKEN or GITHUB_TOKEN",
			"Set GH_TOKEN or GITHUB_TOKEN before invoking this helper.",
			"",
			nil,
		)
	}
	if override && !githubAPIOverrideTokenDestinationAuthorized(baseURL, allowlist) {
		return githubAPIConfig{}, newPreconditionError(
			"github_api_override_token_destination_unconfirmed",
			fmt.Sprintf("GitHub API override token destination %q is not operator-confirmed", baseURL.value),
			"Set SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS to the exact HTTPS API base URL, or set SLIPWAY_GITHUB_API_URL to the same value, before sending SLIPWAY_GITHUB_API_TOKEN to a file-configured github.api_url.",
			"",
			map[string]any{
				"base_url":         baseURL.value,
				"base_url_source":  string(baseURL.source),
				"allowlist_source": string(allowlist.source),
			},
		)
	}
	return githubAPIConfig{baseURL: baseURL.value, token: token}, nil
}

func resolveGitHubAPIBaseURL(fileCfg model.ConfigGitHub) (githubAPIBaseURLResolution, error) {
	envURL := strings.TrimSpace(os.Getenv(githubAPIURLEnv))
	if envURL != "" {
		value, err := normalizeGitHubAPIBaseURL(envURL)
		if err != nil {
			return githubAPIBaseURLResolution{}, err
		}
		return githubAPIBaseURLResolution{value: value, source: githubAPIConfigSourceEnv}, nil
	}
	fileURL := strings.TrimSpace(fileCfg.APIURL)
	if fileURL != "" {
		value, err := normalizeGitHubAPIBaseURL(fileURL)
		if err != nil {
			return githubAPIBaseURLResolution{}, err
		}
		return githubAPIBaseURLResolution{value: value, source: githubAPIConfigSourceFile}, nil
	}
	value, err := normalizeGitHubAPIBaseURL(githubDefaultAPIBaseURL)
	if err != nil {
		return githubAPIBaseURLResolution{}, err
	}
	return githubAPIBaseURLResolution{value: value, source: githubAPIConfigSourceDefault}, nil
}

func githubAPIOverrideTokenDestinationAuthorized(baseURL githubAPIBaseURLResolution, allowlist githubAPIAllowlistDecision) bool {
	return allowlist.source == githubAPIConfigSourceEnv || baseURL.source == githubAPIConfigSourceEnv
}

func normalizeGitHubAPIBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = githubDefaultAPIBaseURL
	}
	normalized, err := model.NormalizeGitHubAPIBaseURL(value)
	if err != nil {
		return "", newInvalidGitHubAPIURLError(value, err.Error())
	}
	return normalized, nil
}

func newInvalidGitHubAPIURLError(value, reason string) error {
	return newInvalidUsageError(
		"github_api_url_invalid",
		fmt.Sprintf("invalid GitHub API base URL %q: %s", value, reason),
		"Use https://api.github.com, or an exact HTTPS GitHub Enterprise API base URL allowlisted by SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS or github.api_allowed_base_urls.",
		map[string]any{"value": value, "reason": reason},
	)
}

// githubAPIBaseURLAllowed reports whether baseURL is allowlisted, preferring
// the env list (SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS) when set and otherwise
// falling back to the file list (github.api_allowed_base_urls). Both lists are
// normalized through the same URL rules as the override itself before matching.
func githubAPIBaseURLAllowed(baseURL string, fileCfg model.ConfigGitHub) (githubAPIAllowlistDecision, error) {
	raw := strings.TrimSpace(os.Getenv(githubAPIAllowedBaseURLsEnv))
	source := githubAPIConfigSourceEnv
	if raw == "" {
		// File fallback: join the list with newlines (a recognized separator) so
		// the same parser/normalizer covers both surfaces.
		raw = strings.Join(fileCfg.APIAllowedBaseURLs, "\n")
		source = githubAPIConfigSourceFile
	}
	entries, err := parseGitHubAPIAllowedBaseURLs(raw)
	if err != nil {
		return githubAPIAllowlistDecision{}, err
	}
	for _, entry := range entries {
		if entry == baseURL {
			return githubAPIAllowlistDecision{allowed: true, source: source}, nil
		}
	}
	return githubAPIAllowlistDecision{allowed: false, source: source}, nil
}

func parseGitHubAPIAllowedBaseURLs(raw string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		normalized, err := normalizeGitHubAPIBaseURL(value)
		if err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

func githubAPITokenForBaseURL(override bool) string {
	if override {
		return strings.TrimSpace(os.Getenv(githubAPIOverrideTokenEnv))
	}
	token := strings.TrimSpace(os.Getenv(githubAmbientTokenPrimaryEnv))
	if token == "" {
		token = strings.TrimSpace(os.Getenv(githubAmbientTokenSecondaryEnv))
	}
	return token
}

type githubPageHandler func(raw []byte) (int, error)

type githubPageWalker func(githubPageHandler) error

func collectGitHubPaginatedObjects(walk githubPageWalker) ([]map[string]any, error) {
	collected := make([]map[string]any, 0)
	err := walk(func(raw []byte) (int, error) {
		batch, err := decodeGitHubObjectPage(raw)
		if err != nil {
			return 0, err
		}
		collected = append(collected, batch...)
		return len(batch), nil
	})
	return collected, err
}

func collectGitHubPaginatedCheckRuns(walk githubPageWalker) (int, []map[string]any, error) {
	runs := make([]map[string]any, 0)
	totalCount := 0
	first := true
	err := walk(func(raw []byte) (int, error) {
		if len(bytes.TrimSpace(raw)) == 0 {
			return 0, nil
		}
		pageTotal, batch, err := decodeGitHubCheckRunsPage(raw)
		if err != nil {
			return 0, err
		}
		if first {
			totalCount = pageTotal
			first = false
		}
		runs = append(runs, batch...)
		return len(batch), nil
	})
	return totalCount, runs, err
}

func decodeGitHubObjectPage(raw []byte) ([]map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var batch []map[string]any
	if err := json.Unmarshal(raw, &batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func decodeGitHubCheckRunsPage(raw []byte) (int, []map[string]any, error) {
	var envelope struct {
		TotalCount int              `json:"total_count"`
		CheckRuns  []map[string]any `json:"check_runs"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, nil, err
	}
	return envelope.TotalCount, envelope.CheckRuns, nil
}

func extractGitHubCombinedStatuses(combined map[string]any) []map[string]any {
	statuses := make([]map[string]any, 0)
	for _, raw := range ghSlice(combined, "statuses") {
		if m := ghMap(raw); m != nil {
			statuses = append(statuses, m)
		}
	}
	return statuses
}

func (c githubHTTPClient) getJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	c.authorize(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = decodeGitHubResponse(resp, out)
	return err
}

// getPaginated fetches a paginated REST collection of JSON objects. It follows
// RFC5988 Link rel="next" when present and otherwise walks ?per_page=100&page=N
// until an empty page is returned. Pages are bounded to avoid runaway loops.
func (c githubHTTPClient) getPaginated(path string) ([]map[string]any, error) {
	return collectGitHubPaginatedObjects(func(handle githubPageHandler) error {
		return c.walkPages(path, handle)
	})
}

// getPaginatedCheckRuns walks the object-wrapped check-runs endpoint
// ({total_count, check_runs:[...]}) accumulating check_runs across pages while
// preserving total_count from the first page.
func (c githubHTTPClient) getPaginatedCheckRuns(path string) (int, []map[string]any, error) {
	return collectGitHubPaginatedCheckRuns(func(handle githubPageHandler) error {
		return c.walkPages(path, handle)
	})
}

// walkPages drives REST pagination, invoking handle with each page's raw body
// and using its returned item count to decide when a page-number walk is empty.
// It follows RFC5988 Link rel="next" when the server provides it.
func (c githubHTTPClient) walkPages(path string, handle func(raw []byte) (int, error)) error {
	next := c.firstPageURL(path)
	usedLink := false
	for page := 1; next != "" && page <= githubRESTMaxPages; page++ {
		req, err := http.NewRequest(http.MethodGet, next, nil)
		if err != nil {
			return err
		}
		c.authorize(req)
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		linkHeader := resp.Header.Get("Link")
		raw, decErr := decodeGitHubResponse(resp, nil)
		_ = resp.Body.Close() // body is fully consumed by decodeGitHubResponse above; a close error here is not actionable.
		if decErr != nil {
			return decErr
		}
		count, err := handle(raw)
		if err != nil {
			return err
		}
		if linkNext := parseLinkNext(linkHeader); linkNext != "" {
			authorizedNext, err := c.authorizePaginationURL(linkNext)
			if err != nil {
				return err
			}
			next = authorizedNext
			usedLink = true
			continue
		}
		if usedLink {
			// The server speaks Link headers and stopped advertising next.
			break
		}
		if count == 0 {
			break
		}
		next = c.nextPageURL(path, page+1)
	}
	return nil
}

func (c githubHTTPClient) firstPageURL(path string) string {
	return c.baseURL + addGitHubPaginationParams(path, 0)
}

func (c githubHTTPClient) nextPageURL(path string, page int) string {
	return c.baseURL + addGitHubPaginationParams(path, page)
}

func (c githubHTTPClient) authorizePaginationURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || !parsed.IsAbs() {
		return "", newUnsafeGitHubPaginationURLError(value, "URL must be absolute")
	}
	base, err := url.Parse(c.baseURL)
	if err != nil || base == nil {
		return "", newUnsafeGitHubPaginationURLError(value, "configured API base URL is invalid")
	}
	if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Scheme, base.Scheme) {
		return "", newUnsafeGitHubPaginationURLError(value, "scheme must remain https")
	}
	if !strings.EqualFold(parsed.Host, base.Host) {
		return "", newUnsafeGitHubPaginationURLError(value, "host must match the configured API base URL")
	}
	if parsed.User != nil {
		return "", newUnsafeGitHubPaginationURLError(value, "URL must not include userinfo")
	}
	if parsed.Fragment != "" {
		return "", newUnsafeGitHubPaginationURLError(value, "URL must not include a fragment")
	}
	if parsed.RawPath != "" {
		return "", newUnsafeGitHubPaginationURLError(value, "encoded path is not accepted")
	}
	if !githubAPIPathWithinBase(parsed.Path, base.Path) {
		return "", newUnsafeGitHubPaginationURLError(value, "path must stay under the configured API base URL")
	}
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), nil
}

func githubAPIPathWithinBase(candidatePath, basePath string) bool {
	candidate := cleanGitHubAPIPath(candidatePath)
	base := cleanGitHubAPIPath(basePath)
	if base == "/" {
		return strings.HasPrefix(candidate, "/")
	}
	return candidate == base || strings.HasPrefix(candidate, base+"/")
}

func cleanGitHubAPIPath(value string) string {
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}

func newUnsafeGitHubPaginationURLError(value, reason string) error {
	return newPreconditionError(
		"github_api_pagination_url_not_allowed",
		fmt.Sprintf("unsafe GitHub API pagination URL %q: %s", value, reason),
		"Retry with a GitHub API endpoint that keeps pagination links on the configured HTTPS API base URL.",
		"",
		map[string]any{"url": value},
	)
}

func addQueryParam(path, key, value string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}

func addGitHubPaginationParams(path string, page int) string {
	withPerPage := addQueryParam(path, "per_page", "100")
	if page <= 0 {
		return withPerPage
	}
	return addQueryParam(withPerPage, "page", fmt.Sprint(page))
}

// parseLinkNext extracts the URL of the rel="next" entry from an RFC5988 Link
// header, or "" when none is present.
func parseLinkNext(header string) string {
	if strings.TrimSpace(header) == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		rawURL := strings.TrimSpace(segments[0])
		rawURL = strings.TrimPrefix(rawURL, "<")
		rawURL = strings.TrimSuffix(rawURL, ">")
		for _, attr := range segments[1:] {
			attr = strings.TrimSpace(attr)
			if attr == `rel="next"` || attr == "rel=next" {
				return rawURL
			}
		}
	}
	return ""
}

func (c githubHTTPClient) postGraphQL(query string, variables map[string]any, out any) error {
	payload := map[string]any{"query": query, "variables": variables}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/graphql", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = decodeGitHubResponse(resp, out)
	return err
}

func (c githubHTTPClient) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// decodeGitHubResponse streams the response body (bounded) into out and returns
// the raw bytes read. A non-2xx status becomes a precondition error.
func decodeGitHubResponse(resp *http.Response, out any) ([]byte, error) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, githubBodyCap))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return body, newPreconditionError("github_api_failed", fmt.Sprintf("GitHub API failed: %s", message), "Check GH_TOKEN/GITHUB_TOKEN permissions and retry.", "", map[string]any{"status": resp.StatusCode})
	}
	if out == nil {
		return body, nil
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return body, nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return body, err
	}
	return body, nil
}

// --- GraphQL envelope handling -------------------------------------------------

type githubGraphQLError struct {
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

type githubGraphQLEnvelope struct {
	Data   json.RawMessage      `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

// checkGraphQLEnvelope inspects a decoded {data, errors} GraphQL envelope. A
// non-empty errors array fails with a precondition error quoting the messages;
// an empty or null data payload fails with a no-data precondition error;
// otherwise the raw data payload is returned for the caller to decode. Both the
// HTTP and CLI clients had byte-identical baseline bodies (errors guard + no-data
// guard), so both route through this shared helper with identical behavior.
func checkGraphQLEnvelope(env githubGraphQLEnvelope) (json.RawMessage, error) {
	if len(env.Errors) > 0 {
		messages := make([]string, 0, len(env.Errors))
		for _, e := range env.Errors {
			msg := strings.TrimSpace(e.Message)
			if msg == "" {
				msg = "unspecified GraphQL error"
			}
			messages = append(messages, msg)
		}
		return nil, newPreconditionError(
			"github_graphql_errors",
			"GitHub GraphQL returned errors: "+strings.Join(messages, "; "),
			"Resolve the GraphQL errors (permissions, ids, or schema) and retry.",
			"",
			map[string]any{"errors": messages},
		)
	}
	if len(bytes.TrimSpace(env.Data)) == 0 || string(env.Data) == "null" {
		return nil, newPreconditionError(
			"github_graphql_no_data",
			"GitHub GraphQL response carried no data payload",
			"The mutation or query returned a null payload; verify the request and retry.",
			"",
			nil,
		)
	}
	return env.Data, nil
}

// postGraphQLChecked decodes the {data, errors} envelope. A non-empty errors
// array fails with a precondition error quoting the messages. On success the
// raw data payload is returned for the caller to decode.
func (c githubHTTPClient) postGraphQLChecked(query string, variables map[string]any) (json.RawMessage, error) {
	var env githubGraphQLEnvelope
	if err := c.postGraphQL(query, variables, &env); err != nil {
		return nil, err
	}
	return checkGraphQLEnvelope(env)
}

type githubCLIClient struct {
	ctx  context.Context
	path string
}

func (c githubCLIClient) backendName() string { return githubBackendGH }

func (c githubCLIClient) getJSON(path string, out any) error {
	raw, err := c.api(path)
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c githubCLIClient) getPaginated(path string) ([]map[string]any, error) {
	return collectGitHubPaginatedObjects(func(handle githubPageHandler) error {
		return c.walkPages(path, handle)
	})
}

func (c githubCLIClient) getPaginatedCheckRuns(path string) (int, []map[string]any, error) {
	return collectGitHubPaginatedCheckRuns(func(handle githubPageHandler) error {
		return c.walkPages(path, handle)
	})
}

func (c githubCLIClient) walkPages(path string, handle func(raw []byte) (int, error)) error {
	for page := 1; page <= githubRESTMaxPages; page++ {
		raw, err := c.api(addGitHubPaginationParams(path, page))
		if err != nil {
			return err
		}
		count, err := handle(raw)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}
	return nil
}

func (c githubCLIClient) getCombinedStatus(repoPath, sha string) (map[string]any, []map[string]any, error) {
	var combined map[string]any
	if err := c.getJSON("/repos/"+repoPath+"/commits/"+url.PathEscape(sha)+"/status", &combined); err != nil {
		return nil, nil, err
	}
	return combined, extractGitHubCombinedStatuses(combined), nil
}

func (c githubCLIClient) summarizeFailedCheckRun(repoPath string, run map[string]any) map[string]any {
	return summarizeFailedCheckRun(c, repoPath, run)
}

func (c githubCLIClient) postGraphQLChecked(query string, variables map[string]any) (json.RawMessage, error) {
	args := []string{"api", "graphql", "-f", "query=" + query}
	for _, key := range sortedMapKeys(variables) {
		value := variables[key]
		if value == nil {
			continue
		}
		flag := "-f"
		switch value.(type) {
		case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			flag = "-F"
		}
		args = append(args, flag, fmt.Sprintf("%s=%v", key, value))
	}
	raw, err := githubRunCLI(c.ctx, c.path, args...)
	if err != nil {
		return nil, err
	}
	var env githubGraphQLEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	// CLI path rejects empty/null data, matching its baseline behavior.
	return checkGraphQLEnvelope(env)
}

func (c githubCLIClient) api(path string) ([]byte, error) {
	return githubRunCLI(c.ctx, c.path, "api", strings.TrimPrefix(path, "/"))
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.overflow = true
		return len(p), nil
	}
	_, _ = b.Buffer.Write(p)
	return len(p), nil
}

func runGitHubCLICommand(ctx context.Context, ghPath string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, ghPath, args...) // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	stdout := &limitedBuffer{limit: githubBodyCap}
	stderr := &limitedBuffer{limit: githubStderrCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if runCtx.Err() == context.DeadlineExceeded {
		return nil, newPreconditionError(
			"github_gh_timeout",
			"GitHub CLI request timed out",
			"Check GitHub connectivity and retry, or use --backend api with GH_TOKEN/GITHUB_TOKEN.",
			"",
			nil,
		)
	}
	if stdout.overflow {
		return nil, newPreconditionError(
			"github_gh_output_too_large",
			"GitHub CLI response exceeded the helper response limit",
			"Narrow the request and retry.",
			"",
			nil,
		)
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		code := "github_gh_api_failed"
		remediation := "Check `gh auth status`, permissions, and the requested repository or organization."
		if githubCLIAuthError(message) {
			code = "github_gh_auth_required"
			remediation = "Run `gh auth login`, or set GH_TOKEN/GITHUB_TOKEN and rerun with --backend api."
		}
		return nil, newPreconditionError(code, "GitHub CLI request failed: "+message, remediation, "", nil)
	}
	return stdout.Bytes(), nil
}

func githubCLIAuthError(message string) bool {
	lower := strings.ToLower(message)
	for _, needle := range []string{"authentication", "authenticate", "not logged in", "login", "gh auth", "http 401", "bad credentials"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// --- small JSON-shape accessors (self-contained to avoid cross-file coupling) --

func ghMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func ghMapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	return ghMap(m[key])
}

func ghString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func ghBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	if b, ok := m[key].(bool); ok {
		return b
	}
	return false
}

func ghSlice(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	if s, ok := m[key].([]any); ok {
		return s
	}
	return nil
}

// --- fetch-pr-checks -----------------------------------------------------------

var checkRunFailureConclusions = map[string]bool{
	"failure":         true,
	"timed_out":       true,
	"cancelled":       true,
	"action_required": true,
}

var commitStatusFailureStates = map[string]bool{
	"failure": true,
	"error":   true,
}

func runFetchPRChecks(cmd *cobra.Command, repo string, pr int, backend string) error {
	if pr <= 0 {
		return newInvalidUsageError("fetch_pr_checks_pr_required", "--pr is required", "Pass --pr with a pull request number.", nil)
	}
	repo, err := resolveGitHubRepo(repo)
	if err != nil {
		return err
	}
	client, err := newGitHubBackend(cmd.Context(), backend, githubFileConfigFromCommand(cmd))
	if err != nil {
		return err
	}
	repoPath := githubRepoAPIPath(repo)
	var prView struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
		Head   struct {
			SHA string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := client.getJSON("/repos/"+repoPath+"/pulls/"+fmt.Sprint(pr), &prView); err != nil {
		return err
	}

	totalCount, checkRuns, err := client.getPaginatedCheckRuns("/repos/" + repoPath + "/commits/" + url.PathEscape(prView.Head.SHA) + "/check-runs")
	if err != nil {
		return err
	}
	combinedStatus, commitStatuses, err := client.getCombinedStatus(repoPath, prView.Head.SHA)
	if err != nil {
		return err
	}

	failed := make([]map[string]any, 0)
	var passed, failedCount, pending int
	for _, run := range checkRuns {
		status := ghString(run, "status")
		conclusion := ghString(run, "conclusion")
		if status != "completed" {
			pending++
			continue
		}
		if checkRunFailureConclusions[conclusion] {
			failedCount++
			failed = append(failed, client.summarizeFailedCheckRun(repoPath, run))
			continue
		}
		if conclusion == "success" {
			passed++
		}
	}
	for _, status := range commitStatuses {
		state := ghString(status, "state")
		switch {
		case commitStatusFailureStates[state]:
			failedCount++
			failed = append(failed, summarizeFailedCommitStatus(status))
		case state == "success":
			passed++
		default:
			pending++
		}
	}
	totalSignals := len(checkRuns) + len(commitStatuses)

	output := map[string]any{
		"backend":  client.backendName(),
		"repo":     repo,
		"pr":       pr,
		"head_sha": prView.Head.SHA,
		"pr_info": map[string]any{
			"number": prView.Number,
			"url":    prView.URL,
			"branch": prView.Head.Ref,
			"base":   prView.Base.Ref,
		},
		"summary": map[string]any{
			"total":   totalSignals,
			"passed":  passed,
			"failed":  failedCount,
			"pending": pending,
		},
		"checks": map[string]any{
			"total_count": totalCount,
			"check_runs":  checkRuns,
			"statuses":    commitStatuses,
		},
		"check_runs": checkRuns,
		"status":     combinedStatus,
		"statuses":   commitStatuses,
		"failed":     failed,
	}
	return encodeJSONResponse(cmd, output)
}

func (c githubHTTPClient) getCombinedStatus(repoPath, sha string) (map[string]any, []map[string]any, error) {
	var combined map[string]any
	if err := c.getJSON("/repos/"+repoPath+"/commits/"+url.PathEscape(sha)+"/status", &combined); err != nil {
		return nil, nil, err
	}
	return combined, extractGitHubCombinedStatuses(combined), nil
}

// summarizeFailedCheckRun produces a compact record of a failed check run,
// preferring annotation file:line snippets and falling back to output summary.
func (c githubHTTPClient) summarizeFailedCheckRun(repoPath string, run map[string]any) map[string]any {
	return summarizeFailedCheckRun(c, repoPath, run)
}

func summarizeFailedCheckRun(client githubBackend, repoPath string, run map[string]any) map[string]any {
	id := jsonNumberToInt(run["id"])
	record := map[string]any{
		"id":         id,
		"name":       ghString(run, "name"),
		"conclusion": ghString(run, "conclusion"),
		"url":        ghString(run, "html_url"),
	}
	snippets := make([]map[string]any, 0)
	if id != 0 {
		annotations, err := client.getPaginated("/repos/" + repoPath + "/check-runs/" + fmt.Sprint(id) + "/annotations")
		if err == nil {
			for _, ann := range annotations {
				snippets = append(snippets, summarizeAnnotation(ann))
			}
		}
	}
	if len(snippets) == 0 {
		if out := ghMapField(run, "output"); out != nil {
			summary := strings.TrimSpace(ghString(out, "summary"))
			text := strings.TrimSpace(ghString(out, "text"))
			fallback := summary
			if text != "" {
				if fallback != "" {
					fallback += "\n"
				}
				fallback += text
			}
			if fallback != "" {
				record["output_summary"] = fallback
			}
		}
	}
	record["snippets"] = snippets
	return record
}

func summarizeAnnotation(ann map[string]any) map[string]any {
	path := ghString(ann, "path")
	start := jsonNumberToInt(ann["start_line"])
	message := strings.TrimSpace(ghString(ann, "message"))
	location := path
	if path != "" && start != 0 {
		location = fmt.Sprintf("%s:%d", path, start)
	}
	return map[string]any{
		"path":             path,
		"start_line":       start,
		"end_line":         jsonNumberToInt(ann["end_line"]),
		"annotation_level": ghString(ann, "annotation_level"),
		"message":          message,
		"location":         location,
		"snippet":          strings.TrimSpace(location + " " + message),
	}
}

func summarizeFailedCommitStatus(status map[string]any) map[string]any {
	context := ghString(status, "context")
	description := strings.TrimSpace(ghString(status, "description"))
	location := context
	if location == "" {
		location = ghString(status, "state")
	}
	record := map[string]any{
		"type":        "commit_status",
		"context":     context,
		"state":       ghString(status, "state"),
		"url":         ghString(status, "target_url"),
		"description": description,
		"snippets": []map[string]any{{
			"location": location,
			"message":  description,
			"snippet":  strings.TrimSpace(location + " " + description),
		}},
	}
	return record
}

func jsonNumberToInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

// --- fetch-pr-feedback ---------------------------------------------------------

var reviewBotPatterns = compilePatterns([]string{
	`(?i)^sentry`,
	`(?i)^warden`,
	`(?i)^cursor`,
	`(?i)^bugbot`,
	`(?i)^seer`,
	`(?i)^copilot`,
	`(?i)^codex`,
	`(?i)^claude`,
	`(?i)^codeql`,
})

var infoBotPatterns = compilePatterns([]string{
	`(?i)^codecov`,
	`(?i)^dependabot`,
	`(?i)^renovate`,
	`(?i)^github-actions`,
	`(?i)^mergify`,
	`(?i)^semantic-release`,
	`(?i)^sonarcloud`,
	`(?i)^snyk`,
	`(?i)bot$`,
	`(?i)\[bot\]$`,
})

func compilePatterns(raw []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(raw))
	for _, p := range raw {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

func matchesAny(patterns []*regexp.Regexp, s string) bool {
	for _, p := range patterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

func isReviewBot(username string) bool { return matchesAny(reviewBotPatterns, username) }
func isInfoBot(username string) bool   { return matchesAny(infoBotPatterns, username) }

var logafPatterns = []struct {
	re    *regexp.Regexp
	level string
}{
	{regexp.MustCompile(`(?i)^\s*(?:h:|h\s*:|high:|\[h\])`), "high"},
	{regexp.MustCompile(`(?i)^\s*(?:m:|m\s*:|medium:|\[m\])`), "medium"},
	{regexp.MustCompile(`(?i)^\s*(?:l:|l\s*:|low:|\[l\])`), "low"},
}

func detectLogaf(body string) string {
	for _, lp := range logafPatterns {
		if lp.re.MatchString(body) {
			return lp.level
		}
	}
	return ""
}

var highKeywordPatterns = compilePatterns([]string{
	`(?i)must\s+(fix|change|update|address)`,
	`(?i)this\s+(is\s+)?(wrong|incorrect|broken|buggy)`,
	`(?i)security\s+(issue|vulnerability|concern)`,
	`(?i)will\s+(break|cause|fail)`,
	`(?i)critical`,
	`(?i)blocker`,
})

var lowKeywordPatterns = compilePatterns([]string{
	`(?i)nit[:\s]`,
	`(?i)nitpick`,
	`(?i)suggestion[:\s]`,
	`(?i)consider\s+`,
	`(?i)could\s+(also\s+)?`,
	`(?i)might\s+(want\s+to|be\s+better)`,
	`(?i)optional[:\s]`,
	`(?i)minor[:\s]`,
	`(?i)style[:\s]`,
	`(?i)prefer\s+`,
	`(?i)what\s+do\s+you\s+think`,
	`(?i)up\s+to\s+you`,
	`(?i)take\s+it\s+or\s+leave`,
	`(?i)fwiw`,
})

// categorizeComment mirrors the Python categorize_comment: info-bot -> "bot",
// LOGAF prefix, high keyword patterns, low keyword patterns, default "medium".
func categorizeComment(author, body string) string {
	if isInfoBot(author) && !isReviewBot(author) {
		return "bot"
	}
	if level := detectLogaf(body); level != "" {
		return level
	}
	if matchesAny(highKeywordPatterns, body) {
		return "high"
	}
	if matchesAny(lowKeywordPatterns, body) {
		return "low"
	}
	return "medium"
}

func extractFeedbackItem(body, author string, opts feedbackItemOpts) map[string]any {
	summary := body
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	summary = strings.TrimSpace(strings.ReplaceAll(summary, "\n", " "))
	item := map[string]any{"author": author, "body": summary, "full_body": body}
	if opts.path != "" {
		item["path"] = opts.path
	}
	if opts.line != 0 {
		item["line"] = opts.line
	}
	if opts.url != "" {
		item["url"] = opts.url
	}
	if opts.resolved {
		item["resolved"] = true
	}
	if opts.outdated {
		item["outdated"] = true
	}
	if opts.threadID != "" {
		item["thread_id"] = opts.threadID
	}
	return item
}

type feedbackItemOpts struct {
	path     string
	line     int
	url      string
	resolved bool
	outdated bool
	threadID string
}

func runFetchPRFeedback(cmd *cobra.Command, repo string, pr int, backend string) error {
	if pr <= 0 {
		return newInvalidUsageError("fetch_pr_feedback_pr_required", "--pr is required", "Pass --pr with a pull request number.", nil)
	}
	repo, err := resolveGitHubRepo(repo)
	if err != nil {
		return err
	}
	client, err := newGitHubBackend(cmd.Context(), backend, githubFileConfigFromCommand(cmd))
	if err != nil {
		return err
	}
	owner, name := splitRepo(repo)
	repoPath := githubRepoAPIPath(repo)

	var prView struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	if err := client.getJSON("/repos/"+repoPath+"/pulls/"+fmt.Sprint(pr), &prView); err != nil {
		return err
	}
	prAuthor := prView.User.Login

	reviewComments, err := client.getPaginated("/repos/" + repoPath + "/pulls/" + fmt.Sprint(pr) + "/comments")
	if err != nil {
		return err
	}
	issueComments, err := client.getPaginated("/repos/" + repoPath + "/issues/" + fmt.Sprint(pr) + "/comments")
	if err != nil {
		return err
	}
	reviews, err := client.getPaginated("/repos/" + repoPath + "/pulls/" + fmt.Sprint(pr) + "/reviews")
	if err != nil {
		return err
	}
	reviewContext, err := fetchPullReviewContext(client, owner, name, pr)
	if err != nil {
		return err
	}

	feedback := map[string][]map[string]any{
		"high":     {},
		"medium":   {},
		"low":      {},
		"bot":      {},
		"resolved": {},
	}
	add := func(bucket string, item map[string]any) {
		feedback[bucket] = append(feedback[bucket], item)
	}

	// Pull request reviews (REST): CHANGES_REQUESTED review bodies are actionable
	// even when the review has no inline thread comment.
	for _, review := range reviews {
		if ghString(review, "state") != "CHANGES_REQUESTED" {
			continue
		}
		author := ghString(ghMapField(review, "user"), "login")
		body := ghString(review, "body")
		if author == prAuthor {
			continue
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		item := extractFeedbackItem(body, author, feedbackItemOpts{url: ghString(review, "html_url")})
		item["type"] = "changes_requested"
		add("high", item)
	}

	// Review threads (GraphQL): carry thread id + resolved/outdated flags.
	for _, thread := range reviewContext.threads {
		nodes := ghSlice(ghMapField(thread, "comments"), "nodes")
		if len(nodes) == 0 {
			continue
		}
		first := ghMap(nodes[0])
		author := ghString(ghMapField(first, "author"), "login")
		body := ghString(first, "body")
		if author == prAuthor {
			continue
		}
		if len(strings.TrimSpace(body)) < 3 {
			continue
		}
		resolved := ghBool(thread, "isResolved")
		outdated := ghBool(thread, "isOutdated")
		threadID := ghString(thread, "id")
		opts := feedbackItemOpts{
			path:     ghString(thread, "path"),
			line:     jsonNumberToInt(thread["line"]),
			resolved: resolved,
			outdated: outdated,
			threadID: threadID,
		}
		item := extractFeedbackItem(body, author, opts)
		switch {
		case resolved:
			add("resolved", item)
		case isReviewBot(author):
			item["review_bot"] = true
			add(categorizeComment(author, body), item)
		case isInfoBot(author):
			add("bot", item)
		default:
			add(categorizeComment(author, body), item)
		}
	}

	// Issue comments (REST).
	for _, comment := range issueComments {
		author := ghString(ghMapField(comment, "user"), "login")
		body := ghString(comment, "body")
		if author == prAuthor {
			continue
		}
		if len(strings.TrimSpace(body)) < 3 {
			continue
		}
		opts := feedbackItemOpts{url: ghString(comment, "html_url")}
		item := extractFeedbackItem(body, author, opts)
		switch {
		case isReviewBot(author):
			item["review_bot"] = true
			add(categorizeComment(author, body), item)
		case isInfoBot(author):
			add("bot", item)
		default:
			add(categorizeComment(author, body), item)
		}
	}

	reviewBotCount := 0
	for _, bucket := range []string{"high", "medium", "low"} {
		for _, item := range feedback[bucket] {
			if rb, ok := item["review_bot"].(bool); ok && rb {
				reviewBotCount++
			}
		}
	}

	output := map[string]any{
		"backend": client.backendName(),
		"repo":    repo,
		"pr": map[string]any{
			"number":          prView.Number,
			"url":             prView.URL,
			"author":          prAuthor,
			"review_decision": reviewContext.reviewDecision,
		},
		"summary": map[string]any{
			"high":                 len(feedback["high"]),
			"medium":               len(feedback["medium"]),
			"low":                  len(feedback["low"]),
			"bot_comments":         len(feedback["bot"]),
			"resolved":             len(feedback["resolved"]),
			"review_bot_feedback":  reviewBotCount,
			"needs_attention":      len(feedback["high"]) + len(feedback["medium"]),
			"review_comment_count": len(reviewComments),
		},
		"feedback":        feedback,
		"review_comments": reviewComments,
	}
	switch {
	case len(feedback["high"]) > 0:
		output["action_required"] = "Address high-priority feedback before merge"
	case len(feedback["medium"]) > 0:
		output["action_required"] = "Address medium-priority feedback"
	case len(feedback["low"]) > 0:
		output["action_required"] = "Review low-priority suggestions - ask user which to address"
	default:
		output["action_required"] = nil
	}
	return encodeJSONResponse(cmd, output)
}

const reviewThreadsQuery = `query($owner: String!, $repo: String!, $pr: Int!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $pr) {
      reviewDecision
      reviewThreads(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id isResolved isOutdated path line
          comments(first: 10) {
            nodes { id body author { login } createdAt }
          }
        }
      }
    }
  }
}`

type pullReviewContext struct {
	reviewDecision string
	threads        []map[string]any
}

// fetchPullReviewContext pulls reviewDecision and every reviewThreads page,
// following GraphQL cursors.
func fetchPullReviewContext(client githubBackend, owner, repo string, pr int) (pullReviewContext, error) {
	const maxPages = 50
	context := pullReviewContext{threads: make([]map[string]any, 0)}
	var cursor any
	for page := 0; page < maxPages; page++ {
		vars := map[string]any{"owner": owner, "repo": repo, "pr": pr, "cursor": cursor}
		data, err := client.postGraphQLChecked(reviewThreadsQuery, vars)
		if err != nil {
			return pullReviewContext{}, err
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return pullReviewContext{}, err
		}
		pullRequest := ghMapField(ghMapField(decoded, "repository"), "pullRequest")
		if context.reviewDecision == "" {
			context.reviewDecision = ghString(pullRequest, "reviewDecision")
		}
		reviewThreads := ghMapField(pullRequest, "reviewThreads")
		for _, node := range ghSlice(reviewThreads, "nodes") {
			if m := ghMap(node); m != nil {
				context.threads = append(context.threads, m)
			}
		}
		pageInfo := ghMapField(reviewThreads, "pageInfo")
		if !ghBool(pageInfo, "hasNextPage") {
			break
		}
		end := ghString(pageInfo, "endCursor")
		if end == "" {
			break
		}
		cursor = end
	}
	return context, nil
}

// --- fetch-review-requests -----------------------------------------------------

func runFetchReviewRequests(cmd *cobra.Command, org, teams, backend string) error {
	teamList := splitCSV(teams)
	if len(teamList) == 0 {
		return newInvalidUsageError("fetch_review_requests_teams_required", "--teams is required", "Pass --teams with one or more team slugs.", nil)
	}
	org = strings.TrimSpace(org)
	if org == "" {
		return newInvalidUsageError("fetch_review_requests_org_required", "--org is required", "Pass --org with the GitHub organization that owns the requested teams.", nil)
	}
	client, err := newGitHubBackend(cmd.Context(), backend, githubFileConfigFromCommand(cmd))
	if err != nil {
		return err
	}

	// Collect team members across the requested team slugs.
	members := make(map[string]bool)
	for _, slug := range teamList {
		memberList, err := client.getPaginated("/orgs/" + url.PathEscape(org) + "/teams/" + url.PathEscape(slug) + "/members")
		if err != nil {
			return err
		}
		for _, m := range memberList {
			if login := ghString(m, "login"); login != "" {
				members[login] = true
			}
		}
	}

	teamSlugLower := make(map[string]bool, len(teamList))
	for _, slug := range teamList {
		teamSlugLower[strings.ToLower(slug)] = true
	}

	notifications, err := client.getPaginated("/notifications")
	if err != nil {
		return err
	}

	prs := make([]map[string]any, 0)
	for _, notif := range notifications {
		if ghString(notif, "reason") != "review_requested" || !ghBool(notif, "unread") {
			continue
		}
		subject := ghMapField(notif, "subject")
		subjectURL := ghString(subject, "url")
		repoFull, prNumber, ok := parsePullSubjectURL(subjectURL)
		if !ok {
			continue
		}

		var prData struct {
			MergedAt any    `json:"merged_at"`
			State    string `json:"state"`
			User     struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		if err := client.getJSON("/repos/"+repoFull+"/pulls/"+prNumber, &prData); err != nil {
			return err
		}
		if prData.State == "closed" || prData.MergedAt != nil {
			continue
		}
		author := prData.User.Login

		var reviewers struct {
			Teams []struct {
				Slug string `json:"slug"`
			} `json:"teams"`
		}
		if err := client.getJSON("/repos/"+repoFull+"/pulls/"+prNumber+"/requested_reviewers", &reviewers); err != nil {
			return err
		}
		matchingTeams := make([]string, 0)
		for _, t := range reviewers.Teams {
			if teamSlugLower[strings.ToLower(t.Slug)] {
				matchingTeams = append(matchingTeams, t.Slug)
			}
		}
		byTeamMember := members[author]
		if !byTeamMember && len(matchingTeams) == 0 {
			continue
		}

		reasons := make([]string, 0, 2)
		if len(matchingTeams) > 0 {
			reasons = append(reasons, "review requested from: "+strings.Join(matchingTeams, ", "))
		}
		if byTeamMember {
			reasons = append(reasons, "opened by: "+author)
		}

		prs = append(prs, map[string]any{
			"notification_id": ghString(notif, "id"),
			"title":           ghString(subject, "title"),
			"url":             "https://github.com/" + repoFull + "/pull/" + prNumber,
			"repo":            repoFull,
			"number":          atoiOrZero(prNumber),
			"author":          author,
			"reasons":         reasons,
		})
	}

	return encodeJSONResponse(cmd, map[string]any{"backend": client.backendName(), "total": len(prs), "prs": prs})
}

var pullSubjectURLRe = regexp.MustCompile(`/repos/([^/]+/[^/]+)/pulls/(\d+)$`)

// parsePullSubjectURL extracts owner/repo and PR number from a notification
// subject URL like https://api.github.com/repos/o/r/pulls/123. Non-PR subjects
// (e.g. .../issues/123) return ok=false.
func parsePullSubjectURL(subjectURL string) (repo, number string, ok bool) {
	m := pullSubjectURLRe.FindStringSubmatch(strings.TrimSpace(subjectURL))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func atoiOrZero(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// --- reply-to-thread -----------------------------------------------------------

const replyThreadMutation = `mutation AddReviewThreadReply($threadID: ID!, $body: String!) {
  addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $threadID, body: $body}) {
    comment { id url }
  }
}`

var replyBotSignatureRe = regexp.MustCompile(`^\*[\x{2014}-]\s+.+\*$`)

// normalizeReplyBody mirrors the retired reply-to-thread.py _normalize_body:
// it converts literal "\r\n"/"\n" escape sequences in the argument into real
// newlines and appends an attribution signature unless the body already ends
// with a bot-signature line. This keeps posted review-thread replies attributed
// and newline-correct for callers that pass escaped bodies, while avoiding a
// host-specific hardcoded label in the helper itself.
func normalizeReplyBody(body, signature string) string {
	normalized := strings.ReplaceAll(body, `\r\n`, `\n`)
	normalized = strings.ReplaceAll(normalized, `\n`, "\n")
	trimmed := strings.TrimRight(normalized, " \t\r\n")
	lines := strings.Split(trimmed, "\n")
	lastLine := ""
	if len(lines) > 0 {
		lastLine = lines[len(lines)-1]
	}
	if replyBotSignatureRe.MatchString(strings.TrimSpace(lastLine)) {
		return normalized
	}
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return normalized
	}
	if normalized != "" && !strings.HasSuffix(normalized, "\n") {
		normalized += "\n"
	}
	if normalized != "" && !strings.HasSuffix(normalized, "\n\n") {
		normalized += "\n"
	}
	normalized += "*- " + signature + "*"
	return normalized
}

func runReplyToThread(cmd *cobra.Command, args []string, confirm bool, backend string, signature string) error {
	requests := make([]map[string]any, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		requests = append(requests, map[string]any{
			"thread_id": args[i],
			"body":      normalizeReplyBody(args[i+1], signature),
		})
	}

	if !confirm {
		fmt.Fprintln(cmd.OutOrStdout(), "DRY-RUN: reply-to-thread would post the following GraphQL mutation.")
		_ = encodeJSONResponse(cmd, map[string]any{"query": replyThreadMutation, "requests": requests})
		return newPreconditionError("reply_to_thread_dry_run", "reply-to-thread dry-run only; pass --confirm to post", "Review the request and rerun with --confirm.", "", nil)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "BEGIN REQUEST")
	_ = encodeJSONResponse(cmd, map[string]any{"query": replyThreadMutation, "requests": requests})

	client, err := newGitHubBackend(cmd.Context(), backend, githubFileConfigFromCommand(cmd))
	if err != nil {
		return err
	}

	operations := make([]map[string]any, 0, len(requests))
	postedOperations := make([]map[string]any, 0, len(requests))
	posted := 0
	failed := 0
	for _, request := range requests {
		threadID, _ := request["thread_id"].(string)
		op := map[string]any{"thread_id": threadID}
		vars := map[string]any{"threadID": request["thread_id"], "body": request["body"]}
		data, err := client.postGraphQLChecked(replyThreadMutation, vars)
		if err != nil {
			failed++
			op["status"] = "failed"
			op["error"] = err.Error()
			operations = append(operations, op)
			continue
		}
		var decoded struct {
			AddReply struct {
				Comment struct {
					ID  string `json:"id"`
					URL string `json:"url"`
				} `json:"comment"`
			} `json:"addPullRequestReviewThreadReply"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			failed++
			op["status"] = "failed"
			op["error"] = "decode mutation response: " + err.Error()
			operations = append(operations, op)
			continue
		}
		commentID := strings.TrimSpace(decoded.AddReply.Comment.ID)
		if commentID == "" {
			failed++
			op["status"] = "failed"
			op["error"] = "mutation returned no comment id"
			operations = append(operations, op)
			continue
		}
		op["status"] = "ok"
		op["comment_id"] = commentID
		op["comment_url"] = decoded.AddReply.Comment.URL
		operations = append(operations, op)
		postedOperations = append(postedOperations, op)
		posted++
	}

	output := map[string]any{
		"backend":    client.backendName(),
		"replied":    posted,
		"failed":     failed,
		"operations": operations,
		"posted":     postedOperations,
	}
	if err := encodeJSONResponse(cmd, output); err != nil {
		return err
	}
	if failed > 0 {
		return newPreconditionError(
			"reply_to_thread_partial_failure",
			fmt.Sprintf("reply-to-thread posted %d replies and failed %d replies", posted, failed),
			"Review the operations output and retry only failed thread ids.",
			"",
			map[string]any{"operations": operations},
		)
	}
	return nil
}

// --- shared repo helpers -------------------------------------------------------

func resolveGitHubRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo != "" {
		if validGitHubRepo(repo) {
			return repo, nil
		}
		return "", newInvalidUsageError("github_repo_invalid", fmt.Sprintf("invalid GitHub repo %q", repo), "Use owner/name.", nil)
	}
	return "", newInvalidUsageError("github_repo_required", "--repo is required", "Pass --repo owner/name so the helper does not rely on git remotes or ambient CLI state.", nil)
}

func validGitHubRepo(repo string) bool {
	if !githubRepoPattern.MatchString(repo) {
		return false
	}
	owner, name := splitRepo(repo)
	return validGitHubRepoSegment(owner) && validGitHubRepoSegment(name)
}

func validGitHubRepoSegment(segment string) bool {
	return strings.Trim(segment, ".") != ""
}

func splitRepo(repo string) (owner, name string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return repo, ""
	}
	return parts[0], parts[1]
}

func splitCSV(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func githubRepoAPIPath(repo string) string {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return url.PathEscape(repo)
	}
	return url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
}
