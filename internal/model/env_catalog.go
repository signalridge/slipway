package model

import (
	"sort"
)

// Environment-variable scopes classify how a SLIPWAY_* (or ambient) variable
// relates to the file config surface. The scope tells an operator whether the
// setting belongs in version-controlled `.slipway.yaml`, must be injected by the
// runtime host per session, or is a secret that must never be written to a file.
const (
	// EnvScopeRepoPolicy marks a variable that expresses repository policy and is
	// promotable to a `.slipway.yaml` key (carried in FileConfigKey). The
	// environment value overrides the file value (env > file > default).
	EnvScopeRepoPolicy = "repo-policy"
	// EnvScopeRuntimeHost marks a variable the runtime host injects per session
	// (identity, host capabilities). It has no file equivalent because it
	// describes the host, not the project.
	EnvScopeRuntimeHost = "runtime-host"
	// EnvScopeSecret marks a credential that must stay environment-only and is
	// never representable in `.slipway.yaml`.
	EnvScopeSecret = "secret"
)

// EnvCatalogEntry describes one environment variable Slipway reads, giving the
// runtime env surface the same discoverability the file config surface gets from
// ConfigCatalog(). Unlike file keys, env vars are not derivable by reflection, so
// this catalog is curated; TestEnvCatalogCoversPublicEnvLiterals asserts public
// env names read in source have entries here.
type EnvCatalogEntry struct {
	// Name is the environment variable name, e.g. "SLIPWAY_GITHUB_API_URL".
	Name string `json:"name"`
	// Scope is one of repo-policy, runtime-host, or secret.
	Scope string `json:"scope"`
	// Default is the effective default applied when the variable is unset; empty
	// when there is no static default.
	Default string `json:"default,omitempty"`
	// FileConfigKey is the dotted `.slipway.yaml` key this variable overrides, for
	// repo-policy variables that are promotable to file config. Empty for
	// runtime-host and secret variables.
	FileConfigKey string `json:"file_config_key,omitempty"`
	// Description is a short human-facing note.
	Description string `json:"description,omitempty"`
	// Secret reports whether the value is a credential that must not be logged or
	// written to a file.
	Secret bool `json:"secret,omitempty"`
	// ValueSyntax describes the expected value shape, such as a positive integer,
	// an HTTPS URL, or a separated token list.
	ValueSyntax string `json:"value_syntax,omitempty"`
	// AcceptedValues describes constrained tokens and their runtime meaning.
	AcceptedValues []EnvAcceptedValue `json:"accepted_values,omitempty"`
	// Examples lists copyable declarations with placeholder values where needed.
	Examples []string `json:"examples,omitempty"`
	// UnsetBehavior describes the fallback path when the environment variable is
	// unset, empty, malformed, or intentionally omitted.
	UnsetBehavior string `json:"unset_behavior,omitempty"`
}

// EnvAcceptedValue describes one accepted token or literal value for an
// environment variable whose value space is constrained.
type EnvAcceptedValue struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// EnvCatalog returns the curated, name-sorted catalog of environment variables
// Slipway reads. Repo-policy entries carry the FileConfigKey they override so an
// operator can see the env > file > default relationship at a glance. Ambient
// fallback variables are included even though they are not SLIPWAY_-prefixed.
func EnvCatalog() []EnvCatalogEntry {
	entries := []EnvCatalogEntry{
		{
			Name:          "SLIPWAY_GITHUB_API_URL",
			Scope:         EnvScopeRepoPolicy,
			Default:       "https://api.github.com",
			FileConfigKey: "github.api_url",
			Description:   "GitHub REST/GraphQL API base URL for the token-backed HTTP backend; overrides github.api_url.",
			ValueSyntax:   "HTTPS GitHub REST/GraphQL API base URL.",
			Examples:      []string{"SLIPWAY_GITHUB_API_URL=https://github.example.com/api/v3"},
			UnsetBehavior: "Falls back to github.api_url from .slipway.yaml; if that is unset, uses https://api.github.com.",
		},
		{
			Name:          "SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS",
			Scope:         EnvScopeRepoPolicy,
			FileConfigKey: "github.api_allowed_base_urls",
			Description:   "Comma, semicolon, space, tab, newline, or carriage-return separated HTTPS API base URLs allowed for a GitHub API override; overrides github.api_allowed_base_urls and confirms file-configured token destinations.",
			ValueSyntax:   "Comma, semicolon, space, tab, newline, or carriage-return separated HTTPS GitHub API base URLs.",
			Examples:      []string{"SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS=https://github.example.com/api/v3"},
			UnsetBehavior: "Falls back to github.api_allowed_base_urls from .slipway.yaml; an override URL still must be allowlisted before override-token egress.",
		},
		{
			Name:          "SLIPWAY_GITHUB_API_TOKEN",
			Scope:         EnvScopeSecret,
			Secret:        true,
			Description:   "Token used only for an allowlisted and env-confirmed GitHub API override host; never read from .slipway.yaml.",
			ValueSyntax:   "GitHub API token string for an allowlisted override host.",
			Examples:      []string{"SLIPWAY_GITHUB_API_TOKEN=<token>"},
			UnsetBehavior: "No override-host token is available; ambient GH_TOKEN/GITHUB_TOKEN are not sent to override hosts.",
		},
		{
			Name:          "SLIPWAY_SESSION_OWNER",
			Scope:         EnvScopeRuntimeHost,
			Description:   "Session identity recorded on handoff notes; falls back to USER/USERNAME when unset, empty, or whitespace.",
			ValueSyntax:   "Non-empty session owner label.",
			Examples:      []string{"SLIPWAY_SESSION_OWNER=agent-codex"},
			UnsetBehavior: "Unset, empty, or whitespace-only values fall back to USER, then USERNAME, then machine hostname, then unknown.",
		},
		{
			Name:        "SLIPWAY_HOST_CAPABILITIES",
			Scope:       EnvScopeRuntimeHost,
			Description: "Host-advertised capability tokens (e.g. subagent fan-out) the engine reads when shaping dispatch.",
			ValueSyntax: "Comma, semicolon, space, tab, newline, or carriage-return separated host capability tokens; token matching is case-insensitive.",
			AcceptedValues: []EnvAcceptedValue{
				{Value: "subagent", Description: "Declares subagent dispatch capability available."},
				{Value: "delegation", Description: "Alias that declares subagent dispatch capability available."},
				{Value: "none", Description: "Declares host capability unavailable."},
				{Value: "unavailable", Description: "Declares host capability unavailable."},
			},
			Examples:      []string{"SLIPWAY_HOST_CAPABILITIES=subagent", "SLIPWAY_HOST_CAPABILITIES=none"},
			UnsetBehavior: "Empty or unset means capability availability is unknown; any non-empty unrecognized token declares the host capability space and leaves unsatisfied capabilities unavailable rather than unknown.",
		},
		{
			Name:        "SLIPWAY_HOST_CAPABILITY_FALLBACKS",
			Scope:       EnvScopeRuntimeHost,
			Description: "Host-advertised capability fallbacks applied when a primary capability is unavailable.",
			ValueSyntax: "Comma, semicolon, space, tab, newline, or carriage-return separated fallback-mode tokens; token matching is case-insensitive.",
			AcceptedValues: []EnvAcceptedValue{
				{Value: "manual_plan_audit", Description: "Explicit degraded fallback for plan-audit."},
				{Value: "manual_spec_compliance_review", Description: "Explicit degraded fallback for spec-compliance-review."},
				{Value: "manual_code_quality_review", Description: "Explicit degraded fallback for code-quality-review."},
				{Value: "manual_security_review", Description: "Explicit degraded fallback for security-review."},
				{Value: "manual_independent_review", Description: "Explicit degraded fallback for independent-review."},
				{Value: "manual_ship_verification", Description: "Explicit degraded fallback for ship-verification."},
				{Value: "same_context_degraded", Description: "Generic degraded fallback accepted by subagent-required governance skills; record fallback evidence when used."},
			},
			Examples:      []string{"SLIPWAY_HOST_CAPABILITY_FALLBACKS=same_context_degraded"},
			UnsetBehavior: "No recognized degraded fallback is selected; unavailable required capabilities remain blocked and unknown capabilities require authorization.",
		},
		{
			Name:          "GH_TOKEN",
			Scope:         EnvScopeSecret,
			Secret:        true,
			Description:   "Ambient GitHub token sent only to https://api.github.com (and honored by the gh backend); never read from .slipway.yaml.",
			ValueSyntax:   "GitHub token string for the default https://api.github.com API host.",
			Examples:      []string{"GH_TOKEN=<token>"},
			UnsetBehavior: "Falls back to GITHUB_TOKEN for the API backend; if both are unset, token-backed default API calls fail unless the gh backend supplies auth.",
		},
		{
			Name:          "GITHUB_TOKEN",
			Scope:         EnvScopeSecret,
			Secret:        true,
			Description:   "Ambient GitHub token fallback sent only to https://api.github.com; never read from .slipway.yaml.",
			ValueSyntax:   "GitHub token string for the default https://api.github.com API host.",
			Examples:      []string{"GITHUB_TOKEN=<token>"},
			UnsetBehavior: "No ambient fallback token is available when GH_TOKEN is also unset; token-backed default API calls fail unless the gh backend supplies auth.",
		},
		{
			Name:          "USER",
			Scope:         EnvScopeRuntimeHost,
			Description:   "Ambient OS username fallback for handoff session owner when SLIPWAY_SESSION_OWNER is unset.",
			ValueSyntax:   "Non-empty ambient OS username.",
			Examples:      []string{"USER=<username>"},
			UnsetBehavior: "Falls back to USERNAME for handoff ownership when SLIPWAY_SESSION_OWNER is unset.",
		},
		{
			Name:          "USERNAME",
			Scope:         EnvScopeRuntimeHost,
			Description:   "Ambient OS username fallback for handoff session owner when SLIPWAY_SESSION_OWNER and USER are unset.",
			ValueSyntax:   "Non-empty ambient OS username.",
			Examples:      []string{"USERNAME=<username>"},
			UnsetBehavior: "Falls back to machine hostname, then unknown, when SLIPWAY_SESSION_OWNER and USER are unset.",
		},
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}
