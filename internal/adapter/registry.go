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
	SurfaceKind   string   `json:"surface_kind"`
	OwnershipRoot string   `json:"ownership_root"`
	DetectPaths   []string `json:"detect_paths"`
}

// UnknownHostSelectionError reports a requested adapter ID that is not in the registry.
type UnknownHostSelectionError struct {
	HostID string
}

func (err *UnknownHostSelectionError) Error() string {
	return fmt.Sprintf("unknown host adapter %q", err.HostID)
}

var hosts = []Host{
	{ID: "claude", SkillsDir: ".claude/skills", SurfaceKind: "skill", OwnershipRoot: ".claude", DetectPaths: []string{".claude"}},
	{ID: "codex", SkillsDir: ".codex/skills", SurfaceKind: "skill", OwnershipRoot: ".codex", DetectPaths: []string{".codex"}},
	{ID: "copilot", SkillsDir: ".github/skills", SurfaceKind: "copilot_agent", OwnershipRoot: ".github/copilot", DetectPaths: []string{".github/copilot", ".github/prompts", ".github/skills"}},
	{ID: "cursor", SkillsDir: ".cursor/skills", SurfaceKind: "skill", OwnershipRoot: ".cursor", DetectPaths: []string{".cursor"}},
	{ID: "kilo", SkillsDir: ".kilocode/skills", SurfaceKind: "kilo_command", OwnershipRoot: ".kilocode", DetectPaths: []string{".kilocode"}},
	{ID: "kiro", SkillsDir: ".kiro/skills", SurfaceKind: "", OwnershipRoot: ".kiro", DetectPaths: []string{".kiro"}},
	{ID: "opencode", SkillsDir: ".opencode/skills", SurfaceKind: "opencode_command", OwnershipRoot: ".opencode", DetectPaths: []string{".opencode"}},
	{ID: "pi", SkillsDir: ".pi/skills", SurfaceKind: "skill", OwnershipRoot: ".pi", DetectPaths: []string{".pi"}},
	{ID: "qwen", SkillsDir: ".qwen/skills", SurfaceKind: "skill", OwnershipRoot: ".qwen", DetectPaths: []string{".qwen"}},
	{ID: "windsurf", SkillsDir: ".windsurf/skills", SurfaceKind: "windsurf_workflow", OwnershipRoot: ".windsurf", DetectPaths: []string{".windsurf"}},
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
					return nil, &UnknownHostSelectionError{HostID: id}
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
