package capability

import (
	"fmt"
	"strings"
)

// BuildCatalogManifest renders the outbound `using-slipway-catalog.md`
// document. The manifest is a single markdown file aimed at external agents
// that need a compact description-level map of the Slipway catalog.
//
// It is a one-way export. The kernel does not read this file back, so the
// output shape is free to evolve as authoring needs change — only the
// renderer must remain deterministic so regenerations produce stable diffs.
//
// Adapter-visible skill labels use the canonical `slipway-<id>` public name.
func BuildCatalogManifest(reg *Registry) string {
	if reg == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Slipway Catalog\n\n")
	b.WriteString("Use only when no governed host already owns the step.\n\n")
	b.WriteString("Generated from the Go-owned catalog registry. Refresh with `slipway init`.\n\n")
	b.WriteString(fmt.Sprintf("Registered skills: %d\n\n", reg.Len()))

	b.WriteString("## Index\n\n")
	b.WriteString("| Skill | Use when |\n")
	b.WriteString("| --- | --- |\n")
	for _, sk := range reg.All() {
		b.WriteString(fmt.Sprintf(
			"| `%s` | %s |\n",
			adapterSkillPublicName(sk.ID),
			sk.Summary,
		))
	}
	b.WriteString("\n")
	return b.String()
}

func adapterSkillPublicName(id string) string {
	return "slipway-" + strings.TrimSpace(id)
}
