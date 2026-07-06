package toolgen

import "strings"

const legacyGeneratedCommandSkillTrigger = "<legacy-command-trigger>"

type legacyGeneratedCommandSkillSignature struct {
	commandID              string
	description            string
	class                  string
	tier                   string
	includeInstallMetadata bool
	body                   string
}

// legacyGeneratedCommandSkillSignatures is a generated-content signature set for
// pre-manifest command-skill residue. Names alone are never deletion authority:
// callers only reach these signatures after parsing a command_id, confirming it
// is absent from the current command registry, and checking the full file body.
var legacyGeneratedCommandSkillSignatures = []legacyGeneratedCommandSkillSignature{
	{
		commandID:              "stats",
		description:            "Show repo-wide governance freshness and workflow statistics",
		class:                  string(CommandClassQuery),
		tier:                   "diagnostics",
		includeInstallMetadata: true,
		body: legacyGeneratedCommandSkillBody(
			"# Stats",
			"",
			"Show repo-wide governance freshness and workflow statistics across every change.",
			"",
			"## Invocation",
			"```bash",
			"slipway stats --json",
			"```",
			"",
			"## Contract",
			"- Read-only repo-wide observability: active/archived counts, freshness summaries,",
			"  and workflow statistics. It is not scoped to a single change — use",
			"  `slipway status` for the active change.",
			"",
			"## Flags",
			"- `--json`: JSON output",
			"",
			"## Arguments",
			"```text",
			"[--json]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
		),
	},
	{
		commandID:              "learn",
		description:            "Preview governance learning proposals from lifecycle evidence",
		class:                  string(CommandClassQuery),
		tier:                   "diagnostics",
		includeInstallMetadata: true,
		body: legacyGeneratedCommandSkillBody(
			"# Learn",
			"",
			"Preview read-only governance learning proposals derived from accumulated",
			"lifecycle evidence.",
			"",
			"## Invocation",
			"```bash",
			"slipway learn --preview --json",
			"```",
			"",
			"## Contract",
			"- Read-only and non-mutating: it surfaces *proposed* governance adjustments for",
			"  human review; it never applies them. Proposals are advisory only.",
			"- `--preview` is the default. This is a maintainer/observability surface, not a",
			"  step in driving a single change.",
			"",
			"## Flags",
			"- `--preview`: generate read-only governance learning proposals (default true)",
			"- `--json`: JSON output",
			"",
			"## Arguments",
			"```text",
			"[--preview] [--json]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
		),
	},
	{
		commandID:              "checkpoint",
		description:            "Set an active checkpoint to pause wave execution and request user input",
		class:                  string(CommandClassMutation),
		tier:                   "situational",
		includeInstallMetadata: true,
		body: legacyGeneratedCommandSkillBody(
			"# Checkpoint",
			"",
			"Pause wave execution and request user input for a specific task.",
			"",
			"## Invocation",
			"```bash",
			"slipway checkpoint --task-id <task_id> [--type <type>] [--allowed-responses <responses>] --json",
			"```",
			"",
			"## Contract",
			"- Sets an active checkpoint on the current governed change.",
			"- Only valid during `S2_IMPLEMENT` state.",
			"- Only one checkpoint can be active at a time.",
			"- Resume with `slipway run --resume-response \"<response>\"`.",
			"- After checkpoint resume, a **fresh subagent MUST be spawned** — do NOT continue in the same context.",
			"",
			"## When to Use",
			"- Task encounters a blocker requiring human judgment (architectural decision, ambiguous requirement).",
			"- Task encounters deviation that requires user decision.",
			"- Retry budget exhausted — surface failure to user for decision.",
			"",
			"## Flags",
			"- `--task-id <id>`: ID of the paused task (required)",
			"- `--type <type>`: Checkpoint type — `human_verify` (default), `decision`, `human_action`",
			"- `--allowed-responses <responses>`: Comma-separated allowed response values (required for `type=decision`)",
			"- `--json`: JSON output",
			"- `--change <slug>`: target a specific active change",
			"",
			"## Arguments",
			"```text",
			"--task-id <id> [--type human_verify|decision|human_action] [--allowed-responses <value> ...] [--json] [--change <slug>]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
			"- An active governed change must be in S2_IMPLEMENT with a materialized wave plan (run `slipway repair` if `wave-plan.yaml` is missing).",
		),
	},
	{
		commandID:   "pivot",
		description: "Reroute or rescope an active change",
		class:       string(CommandClassMutation),
		tier:        "situational",
		body: legacyGeneratedCommandSkillBody(
			"# Pivot",
			"",
			"Reroute (re-evaluate the routing/discovery decision) or rescope (reopen intake to",
			"amend scope) an active change. Both set `needs_discovery=true` and clear",
			"execution residue.",
			"",
			"## Invocation",
			"```bash",
			"slipway pivot --reroute",
			"slipway pivot --rescope",
			"```",
			"",
			"## Contract",
			"- Show pivot summary with before/after state.",
			"- Confirm pivot action with user before executing.",
			"- `--reroute` (the default when no flag is given) is valid in `S1_PLAN`,",
			"  `S2_IMPLEMENT`, or `S3_REVIEW`; it returns the change to `S1_PLAN`",
			"  with discovery forced on. An invalid state is blocked (`pivot_state_invalid`).",
			"- `--rescope` is valid in `S2_IMPLEMENT` or `S3_REVIEW`; it returns",
			"  the change to `S0_INTAKE` (intake/clarify) and clears the intent",
			"  `## Approved Summary` so it must be re-confirmed. Before execution",
			"  (`S0_INTAKE`/`S1_PLAN`) and terminal states are blocked",
			"  (`rescope_state_invalid`).",
			"",
			"## Flags",
			"- `--reroute`: Re-evaluate routing/discovery and re-enter `S1_PLAN` (valid in S1_PLAN/S2_IMPLEMENT/S3_REVIEW).",
			"- `--rescope`: Reopen intake — return to `S0_INTAKE` to amend scope, clearing the Approved Summary (valid in S2_IMPLEMENT/S3_REVIEW).",
			"- `--json`: JSON output",
			"- `--change <slug>`: target a specific active change",
			"",
			"## Arguments",
			"```text",
			"[--reroute|--rescope] [--json] [--change <slug>]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
			"- an active change must exist, or pass `--change <slug>` when supported.",
		),
	},
}

func legacyGeneratedCommandSkillBody(lines ...string) string {
	return strings.Join(lines, "\n")
}

func matchesLegacyGeneratedCommandSkillSignature(content string, cfg ToolConfig, commandID string) bool {
	content, ok := normalizeLegacyGeneratedCommandSkillTrigger(content, cfg, commandID)
	if !ok {
		return false
	}
	for _, signature := range legacyGeneratedCommandSkillSignatures {
		if signature.commandID != commandID {
			continue
		}
		if content == signature.content() {
			return true
		}
	}
	return false
}

func normalizeLegacyGeneratedCommandSkillTrigger(content string, cfg ToolConfig, commandID string) (string, bool) {
	triggerLine := "trigger: \"" + commandTrigger(cfg, commandID) + "\"\n"
	if strings.Count(content, triggerLine) != 1 {
		return "", false
	}
	normalized := "trigger: \"" + legacyGeneratedCommandSkillTrigger + "\"\n"
	return strings.Replace(content, triggerLine, normalized, 1), true
}

func (s legacyGeneratedCommandSkillSignature) content() string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(adapterSkillName(s.commandID))
	b.WriteByte('\n')
	b.WriteString("description: ")
	b.WriteString(yamlDoubleQuoted(s.description))
	b.WriteByte('\n')
	if s.includeInstallMetadata {
		b.WriteString("install_profiles:\n")
		b.WriteString("  - full\n")
		b.WriteString("requires: []\n")
	}
	b.WriteString("command_id: \"")
	b.WriteString(s.commandID)
	b.WriteString("\"\n")
	b.WriteString("trigger: \"")
	b.WriteString(legacyGeneratedCommandSkillTrigger)
	b.WriteString("\"\n")
	b.WriteString("class: \"")
	b.WriteString(s.class)
	b.WriteString("\"\n")
	b.WriteString("tier: \"")
	b.WriteString(s.tier)
	b.WriteString("\"\n")
	b.WriteString("surface: \"skill\"\n")
	b.WriteString("---\n")
	b.WriteString(strings.TrimRight(s.body, "\n"))
	b.WriteString("\n\n")
	b.WriteString(commandSkillFooter(s.commandID))
	b.WriteByte('\n')
	return b.String()
}
