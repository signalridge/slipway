package model

import "sort"

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
	// (identity, context-window size, metrics path, host capabilities). It has no
	// file equivalent because it describes the host, not the project.
	EnvScopeRuntimeHost = "runtime-host"
	// EnvScopeSecret marks a credential that must stay environment-only and is
	// never representable in `.slipway.yaml`.
	EnvScopeSecret = "secret"
)

// EnvCatalogEntry describes one environment variable Slipway reads, giving the
// runtime env surface the same discoverability the file config surface gets from
// ConfigCatalog(). Unlike file keys, env vars are not derivable by reflection, so
// this catalog is curated; a contract test (TestEnvCatalogCoversEveryGetenv)
// asserts every SLIPWAY_* name read in source has an entry here.
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
		},
		{
			Name:          "SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS",
			Scope:         EnvScopeRepoPolicy,
			FileConfigKey: "github.api_allowed_base_urls",
			Description:   "Comma/space/semicolon separated HTTPS API base URLs allowed for a SLIPWAY_GITHUB_API_URL override; overrides github.api_allowed_base_urls.",
		},
		{
			Name:        "SLIPWAY_GITHUB_API_TOKEN",
			Scope:       EnvScopeSecret,
			Secret:      true,
			Description: "Token used only for an allowlisted SLIPWAY_GITHUB_API_URL override host; never read from .slipway.yaml.",
		},
		{
			Name:        "SLIPWAY_CONTEXT_WINDOW_TOKENS",
			Scope:       EnvScopeRuntimeHost,
			Description: "Host model context-window size (tokens) used to estimate context-budget pressure.",
		},
		{
			Name:        "SLIPWAY_CONTEXT_METRICS_PATH",
			Scope:       EnvScopeRuntimeHost,
			Description: "Path the host writes live context-usage metrics to for the context-pressure hook.",
		},
		{
			Name:        "SLIPWAY_SESSION_OWNER",
			Scope:       EnvScopeRuntimeHost,
			Description: "Session identity recorded on handoff notes; falls back to USER/USERNAME when unset.",
		},
		{
			Name:        "SLIPWAY_HOST_CAPABILITIES",
			Scope:       EnvScopeRuntimeHost,
			Description: "Host-advertised capability tokens (e.g. subagent fan-out) the engine reads when shaping dispatch.",
		},
		{
			Name:        "SLIPWAY_HOST_CAPABILITY_FALLBACKS",
			Scope:       EnvScopeRuntimeHost,
			Description: "Host-advertised capability fallbacks applied when a primary capability is unavailable.",
		},
		{
			Name:        "GH_TOKEN",
			Scope:       EnvScopeSecret,
			Secret:      true,
			Description: "Ambient GitHub token sent only to https://api.github.com (and honored by the gh backend); never read from .slipway.yaml.",
		},
		{
			Name:        "GITHUB_TOKEN",
			Scope:       EnvScopeSecret,
			Secret:      true,
			Description: "Ambient GitHub token fallback sent only to https://api.github.com; never read from .slipway.yaml.",
		},
		{
			Name:        "USER",
			Scope:       EnvScopeRuntimeHost,
			Description: "Ambient OS username fallback for handoff session owner when SLIPWAY_SESSION_OWNER is unset.",
		},
		{
			Name:        "USERNAME",
			Scope:       EnvScopeRuntimeHost,
			Description: "Ambient OS username fallback for handoff session owner when SLIPWAY_SESSION_OWNER and USER are unset.",
		},
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}
