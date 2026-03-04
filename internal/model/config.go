package model

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

type LevelMode string

const (
	LevelModeAuto LevelMode = "auto"
)

func (m LevelMode) IsValid() bool {
	switch m {
	case LevelModeAuto, LevelMode(LevelL1), LevelMode(LevelL2), LevelMode(LevelL3):
		return true
	default:
		return false
	}
}

type Config struct {
	Defaults        ConfigDefaults        `yaml:"defaults" json:"defaults"`
	Execution       ConfigExecution       `yaml:"execution" json:"execution"`
	UnknownTopLevel map[string]*yaml.Node `yaml:"-" json:"-"`
}

type ConfigDefaults struct {
	LevelMode LevelMode `yaml:"level_mode" json:"level_mode"`
}

type ConfigExecution struct {
	LockWaitTimeoutSeconds   int `yaml:"lock_wait_timeout_seconds" json:"lock_wait_timeout_seconds"`
	LockStaleAfterSeconds    int `yaml:"lock_stale_after_seconds" json:"lock_stale_after_seconds"`
	CancelGracePeriodSeconds int `yaml:"cancel_grace_period_seconds" json:"cancel_grace_period_seconds"`
	EvidenceRetentionDays    int `yaml:"evidence_retention_days" json:"evidence_retention_days"`
	EvidenceGCLowDiskFreeMB  int `yaml:"evidence_gc_low_disk_free_mb" json:"evidence_gc_low_disk_free_mb"`
	MaxLevelHistoryEntries   int `yaml:"max_level_history_entries" json:"max_level_history_entries"`
}

func DefaultConfig() Config {
	return Config{
		Defaults: ConfigDefaults{
			LevelMode: LevelModeAuto,
		},
		Execution: ConfigExecution{
			LockWaitTimeoutSeconds:   10,
			LockStaleAfterSeconds:    120,
			CancelGracePeriodSeconds: 10,
			EvidenceRetentionDays:    30,
			EvidenceGCLowDiskFreeMB:  512,
			MaxLevelHistoryEntries:   100,
		},
		UnknownTopLevel: map[string]*yaml.Node{},
	}
}

func (c *Config) Normalize() {
	defaults := DefaultConfig()
	if c.Defaults.LevelMode == "" {
		c.Defaults.LevelMode = defaults.Defaults.LevelMode
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
	if c.Execution.EvidenceRetentionDays <= 0 {
		c.Execution.EvidenceRetentionDays = defaults.Execution.EvidenceRetentionDays
	}
	if c.Execution.EvidenceGCLowDiskFreeMB <= 0 {
		c.Execution.EvidenceGCLowDiskFreeMB = defaults.Execution.EvidenceGCLowDiskFreeMB
	}
	if c.Execution.MaxLevelHistoryEntries <= 0 {
		c.Execution.MaxLevelHistoryEntries = defaults.Execution.MaxLevelHistoryEntries
	}
	if c.UnknownTopLevel == nil {
		c.UnknownTopLevel = map[string]*yaml.Node{}
	}
}

func (c Config) EffectiveLevelMode() (mode LevelMode, usedFallback bool) {
	if c.Defaults.LevelMode.IsValid() {
		return c.Defaults.LevelMode, false
	}
	return LevelModeAuto, true
}

func (c Config) Validate() error {
	if c.Execution.LockWaitTimeoutSeconds <= 0 {
		return fmt.Errorf("execution.lock_wait_timeout_seconds must be > 0")
	}
	if c.Execution.LockStaleAfterSeconds <= 0 {
		return fmt.Errorf("execution.lock_stale_after_seconds must be > 0")
	}
	if c.Execution.CancelGracePeriodSeconds <= 0 {
		return fmt.Errorf("execution.cancel_grace_period_seconds must be > 0")
	}
	if c.Execution.EvidenceRetentionDays <= 0 {
		return fmt.Errorf("execution.evidence_retention_days must be > 0")
	}
	if c.Execution.EvidenceGCLowDiskFreeMB <= 0 {
		return fmt.Errorf("execution.evidence_gc_low_disk_free_mb must be > 0")
	}
	if c.Execution.MaxLevelHistoryEntries <= 0 {
		return fmt.Errorf("execution.max_level_history_entries must be > 0")
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
			if err := value.Decode(&cfg.Defaults); err != nil {
				return Config{}, fmt.Errorf("decode defaults: %w", err)
			}
		case "execution":
			if err := value.Decode(&cfg.Execution); err != nil {
				return Config{}, fmt.Errorf("decode execution: %w", err)
			}
		default:
			cfg.UnknownTopLevel[key] = cloneYAMLNode(value)
		}
	}

	cfg.Normalize()
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
	return os.WriteFile(path, data, 0o644)
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
