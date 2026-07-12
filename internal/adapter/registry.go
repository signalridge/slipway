// Package adapter installs host-native capabilities without ambient activation.
package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var capabilityNames = []string{
	"slipway-run",
	"slipway-clarify",
	"slipway-propose",
	"slipway-decompose",
	"slipway-implement",
	"slipway-review",
}

type Host struct {
	ID            string   `json:"id"`
	SkillsDir     string   `json:"skills_dir"`
	OwnershipRoot string   `json:"ownership_root"`
	DetectPaths   []string `json:"detect_paths"`
	SettingsPath  string   `json:"settings_path,omitempty"`
	SettingsKind  string   `json:"settings_kind,omitempty"`
}

var hosts = []Host{
	{ID: "claude", SkillsDir: ".claude/skills", OwnershipRoot: ".claude", DetectPaths: []string{".claude"}, SettingsPath: ".claude/settings.json", SettingsKind: "json-hooks"},
	{ID: "codex", SkillsDir: ".codex/skills", OwnershipRoot: ".codex", DetectPaths: []string{".codex"}, SettingsPath: ".codex/config.toml", SettingsKind: "codex-block"},
	{ID: "copilot", SkillsDir: ".github/skills", OwnershipRoot: ".github/copilot", DetectPaths: []string{".github/copilot"}},
	{ID: "cursor", SkillsDir: ".cursor/skills", OwnershipRoot: ".cursor", DetectPaths: []string{".cursor"}},
	{ID: "kilo", SkillsDir: ".kilocode/skills", OwnershipRoot: ".kilocode", DetectPaths: []string{".kilocode"}},
	{ID: "kiro", SkillsDir: ".kiro/skills", OwnershipRoot: ".kiro", DetectPaths: []string{".kiro"}},
	{ID: "opencode", SkillsDir: ".opencode/skills", OwnershipRoot: ".opencode", DetectPaths: []string{".opencode"}},
	{ID: "pi", SkillsDir: ".pi/skills", OwnershipRoot: ".pi", DetectPaths: []string{".pi"}, SettingsPath: ".pi/settings.json", SettingsKind: "preserve"},
	{ID: "qwen", SkillsDir: ".qwen/skills", OwnershipRoot: ".qwen", DetectPaths: []string{".qwen"}, SettingsPath: ".qwen/settings.json", SettingsKind: "json-hooks"},
	{ID: "windsurf", SkillsDir: ".windsurf/skills", OwnershipRoot: ".windsurf", DetectPaths: []string{".windsurf"}},
}

func Registry() []Host {
	result := make([]Host, len(hosts))
	copy(result, hosts)
	return result
}

func lookupHost(id string) (Host, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, host := range hosts {
		if host.ID == id {
			return host, true
		}
	}
	return Host{}, false
}

func resolveHosts(root string, requested []string, defaultDetected bool) ([]Host, error) {
	selected := map[string]Host{}
	if len(requested) == 0 && defaultDetected {
		for _, host := range hosts {
			if hostDetected(root, host) {
				selected[host.ID] = host
			}
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("no AI coding hosts detected; select one with --tool")
		}
	} else {
		for _, raw := range requested {
			for _, id := range strings.Split(raw, ",") {
				id = strings.ToLower(strings.TrimSpace(id))
				if id == "" {
					continue
				}
				if id == "all" {
					for _, host := range hosts {
						selected[host.ID] = host
					}
					continue
				}
				host, ok := lookupHost(id)
				if !ok {
					return nil, fmt.Errorf("unknown host adapter %q", id)
				}
				selected[id] = host
			}
		}
	}
	result := make([]Host, 0, len(selected))
	for _, host := range hosts {
		if _, ok := selected[host.ID]; ok {
			result = append(result, host)
		}
	}
	return result, nil
}

func hostDetected(root string, host Host) bool {
	for _, relative := range host.DetectPaths {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
