package toolgen

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type ToolConfig struct {
	ID             string
	SkillsDir      string
	CommandsDir    string
	TriggerPrefix  string
	TriggerStyle   string
	AutoDetectPath []string
}

var toolRegistry = map[string]ToolConfig{
	"claude": {
		ID:            "claude",
		SkillsDir:     ".claude/skills",
		CommandsDir:   ".claude/commands",
		TriggerPrefix: "/spln",
		TriggerStyle:  "slash-colon",
		AutoDetectPath: []string{
			".claude",
		},
	},
	"cursor": {
		ID:            "cursor",
		SkillsDir:     ".cursor/skills",
		CommandsDir:   ".cursor/commands",
		TriggerPrefix: "/spln-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".cursor",
		},
	},
	"codex": {
		ID:            "codex",
		SkillsDir:     ".codex/skills",
		CommandsDir:   ".codex/commands",
		TriggerPrefix: "$spln-",
		TriggerStyle:  "dollar-mention",
		AutoDetectPath: []string{
			".codex",
		},
	},
	"opencode": {
		ID:            "opencode",
		SkillsDir:     ".opencode/skills",
		CommandsDir:   ".opencode/commands",
		TriggerPrefix: "/spln-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".opencode",
		},
	},
}

var commandIDs = []string{
	"init",
	"new",
	"do",
	"status",
	"context",
	"done",
	"cancel",
	"pivot",
	"repair",
	"analyze",
	"review",
}

var governanceSkills = []string{
	"intake-analysis",
	"scope-confirmation",
	"plan-audit",
	"wave-orchestration",
	"artifact-review",
	"goal-verification",
	"final-closeout",
}

var techniqueSkills = []string{
	"spln-tdd",
	"spln-systematic-debugging",
	"spln-code-review-protocol",
}

func Registry() []ToolConfig {
	out := make([]ToolConfig, 0, len(toolRegistry))
	for _, cfg := range toolRegistry {
		out = append(out, cfg)
	}
	slices.SortFunc(out, func(a, b ToolConfig) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out
}

func ResolveTools(selection string) ([]string, error) {
	selection = strings.TrimSpace(selection)
	if selection == "" || strings.EqualFold(selection, "all") {
		return []string{"claude", "cursor", "codex", "opencode"}, nil
	}
	if strings.EqualFold(selection, "none") {
		return nil, nil
	}

	parts := strings.Split(selection, ",")
	seen := map[string]struct{}{}
	tools := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if _, ok := toolRegistry[name]; !ok {
			return nil, fmt.Errorf("unsupported tool %q", name)
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tools = append(tools, name)
	}
	slices.Sort(tools)
	return tools, nil
}

func Generate(root string, tools []string, refresh bool) error {
	for _, tool := range tools {
		cfg, ok := toolRegistry[tool]
		if !ok {
			return fmt.Errorf("unsupported tool %q", tool)
		}
		if err := generateForTool(root, cfg, refresh); err != nil {
			return err
		}
	}
	return nil
}

func generateForTool(root string, cfg ToolConfig, refresh bool) error {
	for _, commandID := range commandIDs {
		skillPath := filepath.Join(root, cfg.SkillsDir, "spln-"+commandID, "SKILL.md")
		if err := writeDeterministic(skillPath, commandSkillContent(cfg, commandID), refresh); err != nil {
			return err
		}

		commandPath := filepath.Join(root, cfg.CommandsDir, "spln-"+commandID+".md")
		if err := writeDeterministic(commandPath, commandFileContent(cfg, commandID), refresh); err != nil {
			return err
		}
	}

	for _, skill := range governanceSkills {
		path := filepath.Join(root, cfg.SkillsDir, "spln-"+skill, "SKILL.md")
		if err := writeDeterministic(path, governanceSkillContent(cfg, skill), refresh); err != nil {
			return err
		}
	}

	for _, skill := range techniqueSkills {
		path := filepath.Join(root, cfg.SkillsDir, skill, "SKILL.md")
		if err := writeDeterministic(path, techniqueSkillContent(cfg, skill), refresh); err != nil {
			return err
		}
	}

	return nil
}

func writeDeterministic(path, content string, refresh bool) error {
	if !refresh {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func commandSkillContent(cfg ToolConfig, commandID string) string {
	trigger := commandTrigger(cfg, commandID)
	return strings.TrimSpace(fmt.Sprintf(`
---
name: "spln-%s"
type: command
tool: "%s"
trigger: "%s"
---

Route to the CLI command "%s".

## Invocation
- Use "%s" to invoke this command.
- Pass arguments directly to "spln %s".

## Contract
- Follow runtime state and gate contracts from SpecLane CLI.
- Do not inline governance policy tables in command routing skills.

## Helper Hints (Advisory)
- Discovery/uncertainty reduction hints are advisory only.
- Worktree/review/verification hints are advisory only.
- Runtime gate and evidence checks remain authoritative.
`, commandID, cfg.ID, trigger, commandID, trigger, commandID))
}

func commandFileContent(cfg ToolConfig, commandID string) string {
	trigger := commandTrigger(cfg, commandID)
	return strings.TrimSpace(fmt.Sprintf(`
# %s

- Trigger: "%s"
- Route to: "spln-%s" command skill
- Invocation: "spln %s"
- Policy source: runtime CLI/state engine (not this file)
`, commandID, trigger, commandID, commandID))
}

func governanceSkillContent(cfg ToolConfig, skillName string) string {
	return strings.TrimSpace(fmt.Sprintf(`
---
name: "spln-%s"
type: governance
tool: "%s"
---

Governance contract skill "%s".

## Execution Protocol
- Validate required state preconditions.
- Produce evidence with deterministic schema.

## Step Invariants
- Preserve lane/state ownership boundaries.
- Keep transitions deterministic and auditable.

## Evidence Contract
- Required fields: skill_name, version, run_summary_version, session_id, state, verdict, blockers, references, timestamp.
- Run-summary-bound stages require "input_hash".

## Failure Loop Rules
- On blockers, return to remediation loop and re-run required checks.
- Do not claim pass without fresh evidence.

## Context Budget
- Use compact, task-scoped context.
- Reference long logs via ".spln/evidence/*".

## Subagent Roles
- Implementer and reviewer sessions must remain independent for governed review/closeout.

## Helper Hints (Advisory)
- Discovery/review/verification hints are advisory metadata only.
- Gate decisions are determined by runtime state and evidence.
`, skillName, cfg.ID, skillName))
}

func techniqueSkillContent(cfg ToolConfig, skillName string) string {
	return strings.TrimSpace(fmt.Sprintf(`
---
name: "%s"
type: technique
tool: "%s"
---

Technique helper "%s".

## Use
- Apply this technique to improve implementation quality.
- This technique is optional and non-gating.

## Guardrail
- Never override required runtime gate/evidence contracts.
- Use deterministic outputs and explicit evidence references.
`, skillName, cfg.ID, skillName))
}

func commandTrigger(cfg ToolConfig, commandID string) string {
	switch cfg.TriggerStyle {
	case "slash-colon":
		return fmt.Sprintf("%s:%s", cfg.TriggerPrefix, commandID)
	case "slash-hyphen", "dollar-mention":
		return fmt.Sprintf("%s%s", cfg.TriggerPrefix, commandID)
	default:
		return fmt.Sprintf("%s%s", cfg.TriggerPrefix, commandID)
	}
}
