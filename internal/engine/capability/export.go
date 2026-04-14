package capability

import (
	"fmt"
	"slices"
	"strings"
)

// BuildCatalogManifest renders the outbound `using-slipway-catalog.md`
// document. The manifest is a single markdown file aimed at external agents
// that need a compact description-level map of the Slipway catalog.
//
// It is a one-way export. The kernel does not read this file back, so the
// output shape is free to evolve as authoring needs change — only the
// renderer must remain deterministic so regenerations produce stable diffs.
func BuildCatalogManifest(reg *Registry) string {
	if reg == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Using the Slipway Catalog\n\n")
	b.WriteString("This manifest is generated from the Go-owned catalog registry. ")
	b.WriteString("Do not edit by hand — run `slipway init` (or the toolgen regenerate hook) to refresh it.\n\n")
	b.WriteString(fmt.Sprintf("Registered skills: %d\n\n", reg.Len()))
	b.WriteString("Triage by reading the one-line `summary` under each skill — ")
	b.WriteString("the `Use when … / Triggers on …` phrasing is the dispatcher.\n\n")

	b.WriteString("## Triage index\n\n")
	b.WriteString("| Skill | Tier | Domain | Primary attachment | Routed bindings |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, sk := range reg.All() {
		b.WriteString(fmt.Sprintf(
			"| `%s` | %s | %s | %s | %s |\n",
			sk.ID,
			string(sk.Tier),
			string(sk.Domain),
			string(sk.PrimaryAttachment),
			formatBindingTargets(sk.Bindings),
		))
	}
	b.WriteString("\n")

	groups := groupSkillsByDomain(reg.All())
	for _, domain := range sortedDomains(groups) {
		b.WriteString(fmt.Sprintf("## Domain: %s\n\n", domain))
		for _, sk := range groups[domain] {
			writeSkillSection(&b, sk)
		}
	}
	return b.String()
}

func writeSkillSection(b *strings.Builder, sk Skill) {
	fmt.Fprintf(b, "### `%s` (%s)\n\n", sk.ID, string(sk.Tier))
	fmt.Fprintf(b, "- Function: %s\n", sk.Function)
	fmt.Fprintf(b, "- Primary attachment: %s\n", string(sk.PrimaryAttachment))
	fmt.Fprintf(b, "- Evidence contract: %s\n", string(sk.Evidence))
	fmt.Fprintf(b, "- Summary: %s\n", sk.Summary)
	if len(sk.Bindings) > 0 {
		b.WriteString("- Bindings:\n")
		for _, binding := range sortBindings(sk.Bindings) {
			fmt.Fprintf(b, "  - `%s` -> `%s` (%s)\n",
				string(binding.Type),
				binding.Target,
				string(binding.Attachment),
			)
		}
	}
	b.WriteString("\n")
}

func formatBindingTargets(bindings []Binding) string {
	if len(bindings) == 0 {
		return "-"
	}
	seen := map[string]struct{}{}
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		key := string(b.Type) + ":" + b.Target
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		parts = append(parts, fmt.Sprintf("%s→%s", string(b.Type), b.Target))
	}
	slices.Sort(parts)
	return strings.Join(parts, ", ")
}

func sortBindings(bindings []Binding) []Binding {
	out := append([]Binding(nil), bindings...)
	slices.SortFunc(out, func(a, b Binding) int {
		if a.Type != b.Type {
			return strings.Compare(string(a.Type), string(b.Type))
		}
		if a.Target != b.Target {
			return strings.Compare(a.Target, b.Target)
		}
		return strings.Compare(string(a.Attachment), string(b.Attachment))
	})
	return out
}

func groupSkillsByDomain(skills []Skill) map[Domain][]Skill {
	groups := map[Domain][]Skill{}
	for _, sk := range skills {
		groups[sk.Domain] = append(groups[sk.Domain], sk)
	}
	for domain, list := range groups {
		slices.SortFunc(list, func(a, b Skill) int {
			return strings.Compare(a.ID, b.ID)
		})
		groups[domain] = list
	}
	return groups
}

func sortedDomains(groups map[Domain][]Skill) []Domain {
	order := []Domain{
		DomainIntake,
		DomainExecution,
		DomainDebugging,
		DomainReviewQuality,
		DomainReviewSecurity,
		DomainReviewChangeShape,
		DomainVerification,
		DomainRepairCI,
		DomainOpsDiagnostics,
	}
	out := make([]Domain, 0, len(order))
	for _, d := range order {
		if _, ok := groups[d]; ok {
			out = append(out, d)
		}
	}
	return out
}
