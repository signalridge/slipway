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
	return BuildCatalogManifestWithPaths(reg, func(id string) string {
		return "references/catalog/" + strings.TrimSpace(id) + ".md"
	})
}

func BuildCatalogManifestWithPaths(reg *Registry, catalogArtifactPath func(id string) string) string {
	if reg == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Slipway Catalog\n\n")
	b.WriteString("Use only when no governed host already owns the step. Catalog paths are authoritative for catalog-only skills.\n\n")
	b.WriteString("Generated from the Go-owned catalog registry. Refresh with `slipway init`.\n\n")
	b.WriteString(fmt.Sprintf("Registered skills: %d\n\n", reg.Len()))

	b.WriteString("## Index\n\n")
	b.WriteString("| Skill | Catalog artifact | Tier | Bindings | Evidence | Hydrate refs | Use when |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, sk := range reg.All() {
		artifactPath := ""
		if catalogArtifactPath != nil {
			artifactPath = strings.TrimSpace(catalogArtifactPath(sk.ID))
		}
		b.WriteString(fmt.Sprintf(
			"| `%s` | `%s` | `%s` | %s | `%s` | %s | %s |\n",
			adapterSkillPublicName(sk.ID),
			artifactPath,
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
