package capability

import (
	"fmt"
	"strings"
)

// BuildSkillIndex renders the generated workflow-owned skill index. The index
// is a compact markdown file aimed at external agents that need a
// description-level map of exported Slipway host skills.
//
// It is a one-way reference. The kernel does not read this file back, so the
// output shape is free to evolve as authoring needs change — only the
// renderer must remain deterministic so regenerations produce stable diffs.
//
// Adapter-visible skill labels use the canonical `slipway-<id>` public name.
func BuildSkillIndex(reg *Registry) string {
	return BuildSkillIndexWithPaths(reg, func(id string) string {
		return "slipway-" + strings.TrimSpace(id) + "/SKILL.md"
	})
}

func BuildSkillIndexWithPaths(reg *Registry, hostSkillPath func(id string) string) string {
	if reg == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Slipway Skill Index\n\n")
	b.WriteString("Informational index only. Use `slipway next --json` for governed host selection, then load the real host skill path directly.\n\n")
	b.WriteString("Generated from the Go-owned capability registry. Refresh with `slipway init`.\n\n")
	b.WriteString(fmt.Sprintf("Indexed skills: %d\n\n", reg.Len()))

	b.WriteString("## Index\n\n")
	b.WriteString("| Skill | Host skill path | Tier | Bindings | Evidence | Hydrate refs | Use when |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, sk := range reg.All() {
		loadPath := ""
		if hostSkillPath != nil {
			loadPath = strings.TrimSpace(hostSkillPath(sk.ID))
		}
		b.WriteString(fmt.Sprintf(
			"| `%s` | `%s` | `%s` | %s | `%s` | %s | %s |\n",
			adapterSkillPublicName(sk.ID),
			loadPath,
			sk.Tier,
			formatBindings(sk.Bindings),
			sk.Evidence,
			formatHydrateReferences(sk),
			sk.Summary,
		))
	}
	b.WriteString("\n")
	return b.String()
}

func adapterSkillPublicName(id string) string {
	return "slipway-" + strings.TrimSpace(id)
}

func formatBindings(bindings []Binding) string {
	if len(bindings) == 0 {
		return "`none`"
	}
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		parts = append(parts, fmt.Sprintf(
			"`%s:%s:%s`",
			binding.Type,
			binding.Target,
			binding.Attachment,
		))
	}
	return strings.Join(parts, "<br>")
}

func formatHydrateReferences(sk Skill) string {
	if len(sk.HydrateReferences) == 0 {
		return "`none`"
	}
	parts := make([]string, 0, len(sk.HydrateReferences))
	for _, ref := range sk.HydrateReferences {
		parts = append(parts, fmt.Sprintf("`%s/%s`", sk.ID, ref.Name))
	}
	return strings.Join(parts, "<br>")
}
