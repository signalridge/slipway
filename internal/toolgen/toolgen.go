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

type governanceSkillSpec struct {
	State               string
	Mitigation          string
	RunSummaryBound     bool
	RequiredLevels      []string
	AutoModeRequired    bool
	CloseoutConditional bool
	ReviewerIndependent bool
	ExecutionProtocol   []string
	FailureLoop         []string
}

var governanceSkillSpecs = map[string]governanceSkillSpec{
	"intake-analysis": {
		State:            "S1_ANALYZE",
		Mitigation:       "unclear intent and hidden guardrail risk",
		RunSummaryBound:  false,
		RequiredLevels:   []string{"L2", "L3"},
		AutoModeRequired: true,
		ExecutionProtocol: []string{
			"Normalize structured intake_assessment from model output and persist route_snapshot raw scores only.",
			"Emit executable vs non_spln classification rationale with confidence and blocking_unknowns.",
			"Do not mutate lane state beyond analyze ownership boundaries.",
		},
		FailureLoop: []string{
			"If executable anchors are missing, emit fail with remediation to clarify target and intended delta.",
			"If guardrail domain is uncertain, emit explicit uncertainty markers instead of forcing a domain.",
		},
	},
	"scope-confirmation": {
		State:           "S3_SCOPE_CONFIRMATION",
		Mitigation:      "L3 discovery/scope drift",
		RunSummaryBound: false,
		RequiredLevels:  []string{"L3"},
		ExecutionProtocol: []string{
			"Verify explore.md section completeness and non-empty entries for all required headings.",
			"Validate dedicated worktree metadata authenticity (path accessibility, registration, branch match).",
			"Emit explicit blockers for metadata missing/invalid/mismatch reasons.",
		},
		FailureLoop: []string{
			"If worktree authenticity fails, keep scope gate blocked and require metadata repair before progression.",
			"If explore structure is incomplete, emit actionable missing-section blockers.",
		},
	},
	"plan-audit": {
		State:           "S5_PLAN_AUDIT",
		Mitigation:      "stale or incomplete plan bundle",
		RunSummaryBound: false,
		RequiredLevels:  []string{"L2", "L3"},
		ExecutionProtocol: []string{
			"Validate governed artifact bundle readiness and stale propagation impacts before S6 entry.",
			"Confirm required planning artifacts exist and are structurally valid for current level.",
			"Emit deterministic blockers for missing/stale artifacts and pre-run contract violations.",
		},
		FailureLoop: []string{
			"On bundle failure, route back to S4 with explicit stale/missing artifact reasons.",
			"Re-run plan-audit only after artifact refresh evidence is available.",
		},
	},
	"wave-orchestration": {
		State:           "S6_RUN_WAVES",
		Mitigation:      "uncontrolled parallel execution drift",
		RunSummaryBound: true,
		RequiredLevels:  []string{"L2", "L3"},
		ExecutionProtocol: []string{
			"Execute dependency-derived waves with deterministic ordering and conflict partitioning.",
			"Record run_summary_version-bound outcomes, evidence pointers, and changed_files surfaces.",
			"Apply non-pass control loop decisions (retry/skip/abort/pivot) with retry budget enforcement.",
		},
		FailureLoop: []string{
			"Post-wave file overlap must downgrade conflicting tasks and require serialized retry path.",
			"Retry exhaustion must surface alternatives instead of silent retries.",
		},
	},
	"artifact-review": {
		State:               "S7_REVIEW",
		Mitigation:          "cross-artifact inconsistency",
		RunSummaryBound:     true,
		RequiredLevels:      []string{"L2", "L3"},
		ReviewerIndependent: true,
		ExecutionProtocol: []string{
			"Consume immutable frozen run summary for reviewed run_summary_version; never read in-flight wave state.",
			"Execute required review layers in order (IR1 before IR3, R0 before R3 when guardrail-sensitive).",
			"Emit pass/fail layer outcomes with blockers and intent-drift markers when applicable.",
		},
		FailureLoop: []string{
			"On review blocker, route back to S6 fix/re-run loop and require fresh review evidence.",
			"Two consecutive intent-drift failures must raise pivot_required guidance.",
		},
	},
	"goal-verification": {
		State:           "S8_VERIFY",
		Mitigation:      "false completion claims",
		RunSummaryBound: true,
		RequiredLevels:  []string{"L2", "L3"},
		ExecutionProtocol: []string{
			"Validate final verification signals against latest frozen run summary and unresolved blockers.",
			"Ensure claimed pass outcomes map to resolvable evidence references.",
			"Emit high-risk check outcomes for active guardrail domains when required.",
		},
		FailureLoop: []string{
			"If verification evidence is stale/missing, keep ship gate blocked until refreshed evidence is emitted.",
		},
	},
	"final-closeout": {
		State:               "S8_VERIFY",
		Mitigation:          "stale final evidence before governed ship decision",
		RunSummaryBound:     true,
		RequiredLevels:      []string{"L2", "L3"},
		CloseoutConditional: true,
		ReviewerIndependent: true,
		ExecutionProtocol: []string{
			"Run only when governed closeout refresh is required before ship approval.",
			"Re-check assurance and final evidence index freshness for current run_summary_version.",
			"Emit reviewer-independent closeout verdict bound to latest implementer baseline run.",
		},
		FailureLoop: []string{
			"If closeout evidence is stale, keep S8 blocked and require refreshed reviewer evidence.",
		},
	},
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
command_id: "%s"
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
`, commandID, cfg.ID, commandID, trigger, commandID, trigger, commandID))
}

func commandFileContent(cfg ToolConfig, commandID string) string {
	trigger := commandTrigger(cfg, commandID)
	return strings.TrimSpace(fmt.Sprintf(`
---
command_id: "%s"
trigger: "%s"
skill: "spln-%s"
---

# %s

- Trigger: "%s"
- Route to: "spln-%s" command skill
- Invocation: "spln %s"
- Policy source: runtime CLI/state engine (not this file)
`, commandID, trigger, commandID, commandID, trigger, commandID, commandID))
}

func governanceSkillContent(cfg ToolConfig, skillName string) string {
	spec := governanceSkillSpecs[skillName]
	requiredLevels := make([]string, 0, len(spec.RequiredLevels))
	for _, level := range spec.RequiredLevels {
		requiredLevels = append(requiredLevels, fmt.Sprintf("  - %s", level))
	}
	slices.Sort(requiredLevels)

	executionProtocol := []string{}
	for _, line := range spec.ExecutionProtocol {
		executionProtocol = append(executionProtocol, "- "+line)
	}
	failureLoop := []string{}
	for _, line := range spec.FailureLoop {
		failureLoop = append(failureLoop, "- "+line)
	}

	reviewerPolicy := "Reviewer/implementer session separation is advisory metadata for this skill."
	if spec.ReviewerIndependent {
		reviewerPolicy = "Reviewer evidence must use a different session_id from implementer baseline for same run_summary_version."
	}

	return strings.TrimSpace(fmt.Sprintf(`
---
name: "spln-%s"
type: governance
tool: "%s"
skill_name: "%s"
state: "%s"
mitigation_target: "%s"
run_summary_bound: %t
required_levels:
%s
auto_mode_required: %t
closeout_conditional: %t
reviewer_independent: %t
---

Governance contract skill "%s".

## Execution Protocol
%s

## Step Invariants
- Preserve lane/state ownership boundaries.
- Keep transitions deterministic and auditable.
- Mitigation target: "%s".

## Evidence Contract
- Required fields: skill_name, version, run_summary_version, session_id, state, verdict, blockers, references, timestamp.
- Run-summary-bound stages require "input_hash": %t.

## Failure Loop Rules
%s

## Context Budget
- Use compact, task-scoped context.
- Reference long logs via ".spln/evidence/*".

## Subagent Roles
- %s

## Helper Hints (Advisory)
- Discovery/review/verification hints are advisory metadata only.
- Gate decisions are determined by runtime state and evidence.
`, skillName, cfg.ID, skillName, spec.State, spec.Mitigation, spec.RunSummaryBound, strings.Join(requiredLevels, "\n"), spec.AutoModeRequired, spec.CloseoutConditional, spec.ReviewerIndependent, skillName, strings.Join(executionProtocol, "\n"), spec.Mitigation, spec.RunSummaryBound, strings.Join(failureLoop, "\n"), reviewerPolicy))
}

func techniqueSkillContent(cfg ToolConfig, skillName string) string {
	antiRationalization := []string{
		"- Do not mark pass based on intent alone; require concrete evidence references.",
		"- Do not skip required tests or reviews by claiming low risk without proof.",
		"- Do not collapse blockers into vague summaries; keep deterministic reason codes.",
	}

	csoDescription := map[string]string{
		"spln-tdd":                  "Prioritize behavior-contract tests before implementation deltas to minimize governance drift and rework debt.",
		"spln-systematic-debugging": "Use reproducible symptom->cause->fix loops with bounded hypotheses to reduce risk of speculative patches.",
		"spln-code-review-protocol": "Apply independent review discipline with explicit severity and evidence pointers to protect ship-stage safety.",
	}

	return strings.TrimSpace(fmt.Sprintf(`
---
name: "%s"
type: technique
tool: "%s"
---

Technique helper "%s".

## CSO-Optimized Purpose
- %s
- This technique is optional and non-gating.

## Anti-Rationalization
%s

## Guardrail
- Never override required runtime gate/evidence contracts.
- Use deterministic outputs and explicit evidence references.
`, skillName, cfg.ID, skillName, csoDescription[skillName], strings.Join(antiRationalization, "\n")))
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
