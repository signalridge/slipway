package model

import (
	"bytes"
	"fmt"
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
	Validation      ConfigValidation      `yaml:"validation,omitempty" json:"validation,omitempty"`
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
	// WorktreeBlastRadius is the minimum blast radius that triggers
	// the worktree-isolation control. Default: high (10+ files).
	WorktreeBlastRadius SignalLevel `yaml:"worktree_blast_radius,omitempty" json:"worktree_blast_radius,omitempty"`
}

// Validate checks that threshold signal levels are valid.
func (t ConfigGovernanceThresholds) Validate() error {
	if t.IndependentReviewBlastRadius != "" && !t.IndependentReviewBlastRadius.IsValid() {
		return fmt.Errorf("governance.thresholds.independent_review_blast_radius: invalid signal level %q", t.IndependentReviewBlastRadius)
	}
	if t.WorktreeBlastRadius != "" && !t.WorktreeBlastRadius.IsValid() {
		return fmt.Errorf("governance.thresholds.worktree_blast_radius: invalid signal level %q", t.WorktreeBlastRadius)
	}
	return nil
}

// ConfigValidation controls optional validation rules for spec merges.
type ConfigValidation struct {
	EnforceRFC2119              bool `yaml:"enforce_rfc2119,omitempty" json:"enforce_rfc2119,omitempty"`
	EnforceRequirementScenarios bool `yaml:"enforce_requirement_scenarios,omitempty" json:"enforce_requirement_scenarios,omitempty"`
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
		case "agents":
			return Config{}, fmt.Errorf("top-level agents configuration has been removed; governed handoff is skill-based via next_skill.name and slipway-{name}/SKILL.md host skill surfaces")
		case "validation":
			if err := decodeNodeStrict(value, &cfg.Validation); err != nil {
				return Config{}, fmt.Errorf("decode validation: %w", err)
			}
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

	hasGovernance := cfg.Governance.DefaultPreset != "" || cfg.Governance.MinPreset != "" ||
		len(cfg.Governance.PolicyPacks) > 0 ||
		len(cfg.Governance.Controls) > 0 || len(cfg.Governance.DisabledControls) > 0 ||
		cfg.Governance.Thresholds.IndependentReviewBlastRadius != "" ||
		cfg.Governance.Thresholds.WorktreeBlastRadius != ""
	if hasGovernance {
		governanceNode, err := encodeYAMLNode(cfg.Governance)
		if err != nil {
			return nil, err
		}
		appendMappingEntry(root, "governance", governanceNode)
	}

	if cfg.Validation.EnforceRFC2119 || cfg.Validation.EnforceRequirementScenarios {
		validationNode, err := encodeYAMLNode(cfg.Validation)
		if err != nil {
			return nil, err
		}
		appendMappingEntry(root, "validation", validationNode)
	}

	if cfg.Context.TechStack != "" || cfg.Context.Conventions != "" || cfg.Context.TestCmd != "" || cfg.Context.BuildCmd != "" || len(cfg.Context.Languages) > 0 {
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
	data, err := os.ReadFile(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
