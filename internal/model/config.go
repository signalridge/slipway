package model

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Defaults        ConfigDefaults        `yaml:"defaults" json:"defaults"`
	Execution       ConfigExecution       `yaml:"execution" json:"execution"`
	Governance      ConfigGovernance      `yaml:"governance,omitempty" json:"governance,omitempty"`
	GitHub          ConfigGitHub          `yaml:"github,omitempty" json:"github,omitempty"`
	Subagents       ConfigSubagents       `yaml:"subagents,omitempty" json:"subagents,omitempty"`
	Context         ProjectContext        `yaml:"context,omitempty" json:"context,omitempty"`
	CustomArtifacts []ArtifactDefinition  `yaml:"custom_artifacts,omitempty" json:"custom_artifacts,omitempty"`
	UnknownTopLevel map[string]*yaml.Node `yaml:"-" json:"-"`
}

// ConfigGovernance holds governance-related configuration such as per-control
// mode overrides, disabled controls list, and activation thresholds.
type ConfigGovernance struct {
	DefaultPreset WorkflowPreset `yaml:"default_preset,omitempty" json:"default_preset,omitempty"`
	MinPreset     WorkflowPreset `yaml:"min_preset,omitempty" json:"min_preset,omitempty"`
	PolicyPacks   []PolicyPack   `yaml:"policy_packs,omitempty" json:"policy_packs,omitempty"`
	// Controls maps control IDs to their mode override (blocking/advisory).
	// Controls not listed here use their built-in default mode.
	Controls map[ControlID]ControlMode `yaml:"controls,omitempty" json:"controls,omitempty"`
	// DisabledControls explicitly disables built-in controls.
	DisabledControls []ControlID `yaml:"disabled_controls,omitempty" json:"disabled_controls,omitempty"`
	// Thresholds overrides the blast-radius activation thresholds for controls.
	Thresholds ConfigGovernanceThresholds `yaml:"thresholds,omitempty" json:"thresholds,omitempty"`
	// AutoProvisionWorktree controls whether `slipway new` provisions a dedicated
	// `.worktrees/<slug>` worktree (branch `feat/<slug>`) for every governed
	// change so the main checkout stays free for parallel work. nil means the
	// default (enabled); set false to keep governed changes in the project root.
	AutoProvisionWorktree *bool `yaml:"auto_provision_worktree,omitempty" json:"auto_provision_worktree,omitempty"`
}

// AutoProvisionWorktreeEnabled reports whether governed changes should bind a
// dedicated worktree at creation. Default (unset) is enabled.
func (g ConfigGovernance) AutoProvisionWorktreeEnabled() bool {
	return g.AutoProvisionWorktree == nil || *g.AutoProvisionWorktree
}

func (g ConfigGovernance) IsZero() bool {
	return g.DefaultPreset == "" &&
		g.MinPreset == "" &&
		len(g.PolicyPacks) == 0 &&
		len(g.Controls) == 0 &&
		len(g.DisabledControls) == 0 &&
		g.Thresholds.IsZero() &&
		g.AutoProvisionWorktree == nil
}

// PolicyPack registers an external advisory governance pack. Policy packs are
// intentionally read-only/advisory in this schema; built-in guardrail domains
// remain the fail-closed enforcement surface.
type PolicyPack struct {
	Name string      `yaml:"name" json:"name"`
	Path string      `yaml:"path" json:"path"`
	Mode ControlMode `yaml:"mode,omitempty" json:"mode,omitempty"`
}

// ConfigGovernanceThresholds allows project-level override of the blast-radius
// levels at which controls activate. Valid values: low, medium, high.
type ConfigGovernanceThresholds struct {
	// IndependentReviewBlastRadius is the minimum blast radius that triggers
	// the independent-review control. Default: high (10+ files).
	IndependentReviewBlastRadius SignalLevel `yaml:"independent_review_blast_radius,omitempty" json:"independent_review_blast_radius,omitempty"`
	// SecurityReviewBlastRadius is the minimum blast radius that triggers
	// the security-review control. Default: high (10+ files).
	SecurityReviewBlastRadius SignalLevel `yaml:"security_review_blast_radius,omitempty" json:"security_review_blast_radius,omitempty"`
	// WorktreeBlastRadius is the minimum blast radius that triggers
	// the worktree-isolation control. Default: high (10+ files).
	WorktreeBlastRadius SignalLevel `yaml:"worktree_blast_radius,omitempty" json:"worktree_blast_radius,omitempty"`
}

func (t ConfigGovernanceThresholds) IsZero() bool {
	return t.IndependentReviewBlastRadius == "" &&
		t.SecurityReviewBlastRadius == "" &&
		t.WorktreeBlastRadius == ""
}

// Validate checks that threshold signal levels are valid.
func (t ConfigGovernanceThresholds) Validate() error {
	if t.IndependentReviewBlastRadius != "" && !t.IndependentReviewBlastRadius.IsValid() {
		return fmt.Errorf("governance.thresholds.independent_review_blast_radius: invalid signal level %q", t.IndependentReviewBlastRadius)
	}
	if t.SecurityReviewBlastRadius != "" && !t.SecurityReviewBlastRadius.IsValid() {
		return fmt.Errorf("governance.thresholds.security_review_blast_radius: invalid signal level %q", t.SecurityReviewBlastRadius)
	}
	if t.WorktreeBlastRadius != "" && !t.WorktreeBlastRadius.IsValid() {
		return fmt.Errorf("governance.thresholds.worktree_blast_radius: invalid signal level %q", t.WorktreeBlastRadius)
	}
	return nil
}

// ConfigGitHub holds the repo-policy GitHub API settings that may live in
// .slipway.yaml. These mirror the SLIPWAY_GITHUB_API_URL and
// SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS environment variables so a project can
// pin a GitHub Enterprise host in version control instead of relying on ambient
// environment alone. The matching environment variables still override these
// file values (env > file > default), and the override token
// (SLIPWAY_GITHUB_API_TOKEN) is intentionally NOT representable here: secrets
// stay environment-only.
type ConfigGitHub struct {
	// APIURL pins the GitHub REST/GraphQL API base URL used by the token-backed
	// HTTP backend. Empty means the default https://api.github.com unless the
	// SLIPWAY_GITHUB_API_URL environment variable overrides it.
	APIURL string `yaml:"api_url,omitempty" json:"api_url,omitempty"`
	// APIAllowedBaseURLs lists the HTTPS API base URLs allowed for an APIURL
	// override. The SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS environment variable, when
	// set, overrides this list wholesale.
	APIAllowedBaseURLs []string `yaml:"api_allowed_base_urls,omitempty" json:"api_allowed_base_urls,omitempty"`
}

func (g ConfigGitHub) IsZero() bool {
	return g.APIURL == "" && len(g.APIAllowedBaseURLs) == 0
}

// Validate checks that a configured github.api_url is an absolute https URL with
// a host. An empty api_url is valid: the env/default precedence applies at the
// call site. The allowed-base-URL list is normalized and allowlisted at the
// github call site (against the same URL rules the env path uses), so it is not
// re-validated here. The override token stays env-only and is never on this
// struct.
func (g ConfigGitHub) Validate() error {
	trimmed := strings.TrimSpace(g.APIURL)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("github.api_url: invalid URL %q: %w", g.APIURL, err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("github.api_url: must be an absolute https URL, got %q", g.APIURL)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("github.api_url: must include a host, got %q", g.APIURL)
	}
	return nil
}

// SubagentStage identifies the governed subagent dispatch surface a profile
// applies to. The host honors the resolved profile when it spawns the
// corresponding subagent; Slipway only emits the directive (it does not spawn
// or enforce model/skill/MCP selection itself).
type SubagentStage string

const (
	// SubagentStageReview covers the fresh-context S3 review peers
	// (spec-compliance / code-quality / independent / security review).
	SubagentStageReview SubagentStage = "review"
	// SubagentStageFix covers the fresh-context S3 review-finding repair subagent
	// dispatched by `slipway fix`.
	SubagentStageFix SubagentStage = "fix"
	// SubagentStageVerify covers the terminal ship-verification subagent.
	SubagentStageVerify SubagentStage = "verify"
)

// SubagentProfile is an advisory directive describing the model, allowed skills,
// and allowed MCP servers a host should apply when it spawns a governed
// subagent. Every field is optional; an empty field means "inherit the host
// default" for that dimension. This is a contract the host honors when spawning
// subagents, not an enforcement surface — Slipway emits it via the `slipway
// next --json` and `slipway fix --json` envelopes and the generated dispatch
// skills, but the host owns spawning.
type SubagentProfile struct {
	// Model names the model the host should run the subagent on (e.g. a faster or
	// cheaper model for mechanical review). Empty inherits the host default.
	Model string `yaml:"model,omitempty" json:"model,omitempty"`
	// AllowedSkills restricts the skills the spawned subagent may load. Empty (nil)
	// inherits the host default skill set.
	AllowedSkills []string `yaml:"allowed_skills,omitempty" json:"allowed_skills,omitempty"`
	// AllowedMCPServers restricts the MCP servers the spawned subagent may reach.
	// Empty (nil) inherits the host default MCP set.
	AllowedMCPServers []string `yaml:"allowed_mcp_servers,omitempty" json:"allowed_mcp_servers,omitempty"`
}

func (p SubagentProfile) IsZero() bool {
	return p.Model == "" && len(p.AllowedSkills) == 0 && len(p.AllowedMCPServers) == 0
}

// ConfigSubagents carries optional per-stage subagent directives. The Default
// profile supplies the fallback for any dimension a stage profile leaves unset,
// so a project can set one model/skill/MCP policy once and override only the
// stages that differ. The Default profile also applies to wave executors, whose
// dispatch reference instructs the host to honor it.
type ConfigSubagents struct {
	// Default supplies the fallback profile for every stage and for wave
	// executors.
	Default SubagentProfile `yaml:"default,omitempty" json:"default,omitempty"`
	// Review overrides the Default for the S3 review peers.
	Review SubagentProfile `yaml:"review,omitempty" json:"review,omitempty"`
	// Fix overrides the Default for the S3 review-finding repair subagent.
	Fix SubagentProfile `yaml:"fix,omitempty" json:"fix,omitempty"`
	// Verify overrides the Default for the terminal ship-verification subagent.
	Verify SubagentProfile `yaml:"verify,omitempty" json:"verify,omitempty"`
}

func (s ConfigSubagents) IsZero() bool {
	return s.Default.IsZero() && s.Review.IsZero() && s.Fix.IsZero() && s.Verify.IsZero()
}

// Resolve returns the effective profile for stage, merging each dimension over
// the Default profile: a stage value wins when set, otherwise the Default
// value applies, otherwise the dimension is empty (host inherits). An unknown
// stage resolves to the Default profile.
func (s ConfigSubagents) Resolve(stage SubagentStage) SubagentProfile {
	var override SubagentProfile
	switch stage {
	case SubagentStageReview:
		override = s.Review
	case SubagentStageFix:
		override = s.Fix
	case SubagentStageVerify:
		override = s.Verify
	default:
		override = SubagentProfile{}
	}
	resolved := s.Default
	if override.Model != "" {
		resolved.Model = override.Model
	}
	if override.AllowedSkills != nil {
		resolved.AllowedSkills = override.AllowedSkills
	}
	if override.AllowedMCPServers != nil {
		resolved.AllowedMCPServers = override.AllowedMCPServers
	}
	return resolved
}

// ProjectContext provides project-specific context injected into skill templates.
type ProjectContext struct {
	TechStack   string   `yaml:"tech_stack,omitempty" json:"tech_stack,omitempty"`
	Conventions string   `yaml:"conventions,omitempty" json:"conventions,omitempty"`
	TestCmd     string   `yaml:"test_cmd,omitempty" json:"test_cmd,omitempty"`
	BuildCmd    string   `yaml:"build_cmd,omitempty" json:"build_cmd,omitempty"`
	Languages   []string `yaml:"languages,omitempty" json:"languages,omitempty"`
	RecentWork  string   `yaml:"recent_work,omitempty" json:"recent_work,omitempty"`
}

func (c ProjectContext) IsZero() bool {
	return c.TechStack == "" &&
		c.Conventions == "" &&
		c.TestCmd == "" &&
		c.BuildCmd == "" &&
		len(c.Languages) == 0 &&
		c.RecentWork == ""
}

type ArtifactSchemaName string

const (
	ArtifactSchemaCore     ArtifactSchemaName = "core"
	ArtifactSchemaExpanded ArtifactSchemaName = "expanded"
	ArtifactSchemaCustom   ArtifactSchemaName = "custom"
)

func (s ArtifactSchemaName) IsValid() bool {
	switch s {
	case ArtifactSchemaCore, ArtifactSchemaExpanded, ArtifactSchemaCustom:
		return true
	default:
		return false
	}
}

type ArtifactDefinition struct {
	Name              string   `yaml:"name" json:"name"`
	Template          string   `yaml:"template,omitempty" json:"template,omitempty"`
	RequiresDiscovery bool     `yaml:"requires_discovery,omitempty" json:"requires_discovery,omitempty"`
	DependsOn         []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
}

type ConfigDefaults struct {
	ArtifactSchema ArtifactSchemaName `yaml:"artifact_schema,omitempty" json:"artifact_schema,omitempty"`
}

type ConfigExecution struct {
	LockWaitTimeoutSeconds   int `yaml:"lock_wait_timeout_seconds" json:"lock_wait_timeout_seconds"`
	LockStaleAfterSeconds    int `yaml:"lock_stale_after_seconds" json:"lock_stale_after_seconds"`
	CancelGracePeriodSeconds int `yaml:"cancel_grace_period_seconds" json:"cancel_grace_period_seconds"`
	MaxPlanAuditIterations   int `yaml:"max_plan_audit_iterations" json:"max_plan_audit_iterations"`
	// Parallelization controls whether within-wave parallel execution is forced.
	// Empty (unset) and "forced" mean a multi-task wave is dispatched concurrently
	// by default; "off" opts the project out so the host may run waves sequentially
	// without recording a degradation.
	Parallelization string `yaml:"parallelization,omitempty" json:"parallelization,omitempty"`
	// Auto opts the project into auto-advance execution that auto-advances pure
	// pass-through stages. Default (unset) is false; it is emitted only when
	// enabled.
	Auto bool `yaml:"auto,omitempty" json:"auto,omitempty"`
}

const (
	ParallelizationForced = "forced"
	ParallelizationOff    = "off"
)

// ForcedParallel reports whether forced within-wave parallel execution is in
// effect. Default (unset) is forced; only an explicit "off" disables it.
func (e ConfigExecution) ForcedParallel() bool {
	return e.Parallelization != ParallelizationOff
}

// AutoEnabled reports whether auto-advance execution is opted in. Default
// (unset) is false.
func (e ConfigExecution) AutoEnabled() bool {
	return e.Auto
}

func DefaultConfig() Config {
	return Config{
		Defaults: ConfigDefaults{},
		Execution: ConfigExecution{
			LockWaitTimeoutSeconds:   10,
			LockStaleAfterSeconds:    120,
			CancelGracePeriodSeconds: 10,
			MaxPlanAuditIterations:   3,
		},
		UnknownTopLevel: map[string]*yaml.Node{},
	}
}

func (c *Config) Normalize() {
	defaults := DefaultConfig()
	if c.Defaults.ArtifactSchema == "" {
		c.Defaults.ArtifactSchema = ArtifactSchemaExpanded
	}
	if c.Execution.LockWaitTimeoutSeconds <= 0 {
		c.Execution.LockWaitTimeoutSeconds = defaults.Execution.LockWaitTimeoutSeconds
	}
	if c.Execution.LockStaleAfterSeconds <= 0 {
		c.Execution.LockStaleAfterSeconds = defaults.Execution.LockStaleAfterSeconds
	}
	if c.Execution.CancelGracePeriodSeconds <= 0 {
		c.Execution.CancelGracePeriodSeconds = defaults.Execution.CancelGracePeriodSeconds
	}
	if c.Execution.MaxPlanAuditIterations <= 0 {
		c.Execution.MaxPlanAuditIterations = defaults.Execution.MaxPlanAuditIterations
	}
	if c.UnknownTopLevel == nil {
		c.UnknownTopLevel = map[string]*yaml.Node{}
	}
	for i := range c.Governance.PolicyPacks {
		if c.Governance.PolicyPacks[i].Mode == "" {
			c.Governance.PolicyPacks[i].Mode = ControlModeAdvisory
		}
	}
}

func (c Config) Validate() error {
	if c.Defaults.ArtifactSchema != "" && !c.Defaults.ArtifactSchema.IsValid() {
		return fmt.Errorf("defaults.artifact_schema must be one of: core, expanded, custom")
	}
	if c.Defaults.ArtifactSchema == ArtifactSchemaCustom && len(c.CustomArtifacts) == 0 {
		return fmt.Errorf("custom_artifacts must be non-empty when artifact_schema is custom")
	}
	for id, mode := range c.Governance.Controls {
		if !id.IsValid() {
			return fmt.Errorf("governance.controls: unknown control_id %q", id)
		}
		if !mode.IsValid() {
			return fmt.Errorf("governance.controls.%s: invalid mode %q (must be blocking or advisory)", id, mode)
		}
	}
	for _, id := range c.Governance.DisabledControls {
		if !id.IsValid() {
			return fmt.Errorf("governance.disabled_controls: unknown control_id %q", id)
		}
	}
	for i, pack := range c.Governance.PolicyPacks {
		if strings.TrimSpace(pack.Name) == "" {
			return fmt.Errorf("governance.policy_packs[%d].name is required", i)
		}
		if strings.TrimSpace(pack.Path) == "" {
			return fmt.Errorf("governance.policy_packs[%d].path is required", i)
		}
		if pack.Mode != "" && pack.Mode != ControlModeAdvisory {
			return fmt.Errorf("governance.policy_packs[%d].mode must be advisory", i)
		}
	}
	if p := c.Governance.DefaultPreset; p != "" && !p.IsValid() {
		return fmt.Errorf("governance.default_preset: invalid preset %q", p)
	}
	if p := c.Governance.MinPreset; p != "" && !p.IsValid() {
		return fmt.Errorf("governance.min_preset: invalid preset %q", p)
	}
	if err := c.Governance.Thresholds.Validate(); err != nil {
		return err
	}
	switch c.Execution.Parallelization {
	case "", ParallelizationForced, ParallelizationOff:
	default:
		return fmt.Errorf("execution.parallelization must be unset or one of: forced, off")
	}
	if err := c.GitHub.Validate(); err != nil {
		return err
	}
	return nil
}

func ParseConfigYAML(data []byte) (Config, error) {
	cfg := DefaultConfig()
	if len(bytes.TrimSpace(data)) == 0 {
		return cfg, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Config{}, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return Config{}, fmt.Errorf("config root must be a YAML mapping")
	}

	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i].Value
		value := root.Content[i+1]
		switch key {
		case "defaults":
			if err := decodeNodeStrict(value, &cfg.Defaults); err != nil {
				return Config{}, fmt.Errorf("decode defaults: %w", err)
			}
		case "execution":
			if err := decodeNodeStrict(value, &cfg.Execution); err != nil {
				return Config{}, fmt.Errorf("decode execution: %w", err)
			}
		case "governance":
			if err := decodeNodeStrict(value, &cfg.Governance); err != nil {
				return Config{}, fmt.Errorf("decode governance: %w", err)
			}
		case "github":
			if err := decodeNodeStrict(value, &cfg.GitHub); err != nil {
				return Config{}, fmt.Errorf("decode github: %w", err)
			}
		case "subagents":
			if err := decodeNodeStrict(value, &cfg.Subagents); err != nil {
				return Config{}, fmt.Errorf("decode subagents: %w", err)
			}
		case "agents":
			return Config{}, fmt.Errorf("top-level agents configuration has been removed; governed handoff is skill-based via next_skill.name and slipway-{name}/SKILL.md host skill surfaces")
		case "context":
			if err := decodeNodeStrict(value, &cfg.Context); err != nil {
				return Config{}, fmt.Errorf("decode context: %w", err)
			}
		case "custom_artifacts":
			if err := decodeNodeStrict(value, &cfg.CustomArtifacts); err != nil {
				return Config{}, fmt.Errorf("decode custom_artifacts: %w", err)
			}
		default:
			cfg.UnknownTopLevel[key] = cloneYAMLNode(value)
		}
	}

	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

func (c Config) ToYAML() ([]byte, error) {
	cfg := c
	cfg.Normalize()

	root := &yaml.Node{Kind: yaml.MappingNode}
	defaultsNode, err := encodeYAMLNode(cfg.Defaults)
	if err != nil {
		return nil, err
	}
	executionNode, err := encodeYAMLNode(cfg.Execution)
	if err != nil {
		return nil, err
	}
	appendMappingEntry(root, "defaults", defaultsNode)
	appendMappingEntry(root, "execution", executionNode)

	// Emit the governance section whenever any governance leaf is set. Reuse
	// IsZero() as the single empty-section predicate so newly added fields do not
	// need a second hand-maintained omission list.
	if !cfg.Governance.IsZero() {
		governanceNode, err := encodeYAMLNode(cfg.Governance)
		if err != nil {
			return nil, err
		}
		appendMappingEntry(root, "governance", governanceNode)
	}

	// Emit the github section only when a repo-policy GitHub setting is present.
	// Reuse IsZero() as the single empty-section predicate so the section never
	// round-trips into UnknownTopLevel as an empty mapping.
	if !cfg.GitHub.IsZero() {
		githubNode, err := encodeYAMLNode(cfg.GitHub)
		if err != nil {
			return nil, err
		}
		appendMappingEntry(root, "github", githubNode)
	}

	// Emit the subagents section only when at least one profile dimension is set,
	// gated on the same IsZero() authority used for decode/round-trip parity.
	if !cfg.Subagents.IsZero() {
		subagentsNode, err := encodeYAMLNode(cfg.Subagents)
		if err != nil {
			return nil, err
		}
		appendMappingEntry(root, "subagents", subagentsNode)
	}

	// Emit the context section whenever any context leaf is set. Reuse IsZero()
	// (the single authority for "context is empty") instead of a hand-maintained
	// field list, which previously omitted recent_work and dropped it on save.
	if !cfg.Context.IsZero() {
		contextNode, err := encodeYAMLNode(cfg.Context)
		if err != nil {
			return nil, err
		}
		appendMappingEntry(root, "context", contextNode)
	}

	if len(cfg.CustomArtifacts) > 0 {
		customNode, err := encodeYAMLNode(cfg.CustomArtifacts)
		if err != nil {
			return nil, err
		}
		appendMappingEntry(root, "custom_artifacts", customNode)
	}
	keys := make([]string, 0, len(cfg.UnknownTopLevel))
	for key := range cfg.UnknownTopLevel {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		appendMappingEntry(root, key, cloneYAMLNode(cfg.UnknownTopLevel[key]))
	}

	doc := yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{root},
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return Config{}, err
	}
	return ParseConfigYAML(data)
}

func SaveConfig(path string, cfg Config) error {
	data, err := cfg.ToYAML()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

func encodeYAMLNode(v any) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		return nil, err
	}
	return &node, nil
}

func appendMappingEntry(root *yaml.Node, key string, value *yaml.Node) {
	root.Content = append(root.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}, value)
}

// decodeNodeStrict decodes a yaml.Node into dst with KnownFields(true),
// rejecting any YAML keys that don't map to struct fields.
func decodeNodeStrict(node *yaml.Node, dst interface{}) error {
	raw, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	return dec.Decode(dst)
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i := range node.Content {
			clone.Content[i] = cloneYAMLNode(node.Content[i])
		}
	}
	return &clone
}
