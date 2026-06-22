package toolgen

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

// SurfaceManifestPath is the committed generated-surface inventory location.
const SurfaceManifestPath = "docs/SURFACE-MANIFEST.json"

// SurfaceManifest is a deterministic public inventory of Slipway product
// surfaces derived from the same authorities that generate skills, command
// prompts, JSON contracts, and documentation.
type SurfaceManifest struct {
	Version int                  `json:"version"`
	Rows    []SurfaceManifestRow `json:"rows"`
}

// SurfaceManifestRow ties one generated or documented public surface to the
// authority that creates it and the documentation token that keeps it visible.
type SurfaceManifestRow struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Docs   string `json:"docs"`
	Token  string `json:"token"`
}

// BuildSurfaceManifest derives the public surface inventory from Slipway-owned
// registries and stable documentation contracts.
func BuildSurfaceManifest() SurfaceManifest {
	rows := []SurfaceManifestRow{}

	for _, def := range commandRegistry {
		rows = append(rows, SurfaceManifestRow{
			Kind:   "command",
			Name:   def.ID,
			Source: "internal/toolgen/toolgen.go:commandRegistry",
			Docs:   "docs/reference/commands.md",
			Token:  "slipway " + def.ID,
		})
	}

	for _, cfg := range Registry() {
		rows = append(rows, SurfaceManifestRow{
			Kind:   "adapter",
			Name:   cfg.ID,
			Source: "internal/toolgen/toolgen.go:toolRegistry",
			Docs:   "docs/reference/ai-tools.md",
			Token:  "`" + cfg.ID + "`",
		})
	}

	rows = append(rows, commandSkillRows()...)

	seenSkills := map[string]struct{}{}
	for _, desc := range governanceSurfaceDescriptors {
		name := adapterSkillName(desc.ID)
		rows = append(rows, SurfaceManifestRow{
			Kind:   "skill",
			Name:   name,
			Source: "internal/toolgen/toolgen.go:governanceSurfaceDescriptors",
			Docs:   "README.md",
			Token:  skillDocsToken(name),
		})
		seenSkills[name] = struct{}{}
	}
	for _, id := range append(append([]string{}, standaloneNames...), techniqueNames...) {
		if !shouldExportAsHostSkill(id) {
			continue
		}
		name := adapterSkillName(id)
		if _, ok := seenSkills[name]; ok {
			continue
		}
		rows = append(rows, SurfaceManifestRow{
			Kind:   "skill",
			Name:   name,
			Source: "internal/toolgen/toolgen.go:hostSkillExportAllowlist",
			Docs:   "README.md",
			Token:  skillDocsToken(name),
		})
		seenSkills[name] = struct{}{}
	}
	for _, id := range catalogSkillIDs {
		if !shouldExportAsHostSkill(id) {
			continue
		}
		name := adapterSkillName(id)
		if _, ok := seenSkills[name]; ok {
			continue
		}
		rows = append(rows, SurfaceManifestRow{
			Kind:   "skill",
			Name:   name,
			Source: "internal/engine/capability.DefaultRegistry",
			Docs:   "README.md",
			Token:  skillDocsToken(name),
		})
		seenSkills[name] = struct{}{}
	}

	rows = append(rows, jsonContractRows()...)
	rows = append(rows, documentationRows()...)

	slices.SortFunc(rows, compareSurfaceManifestRows)
	return SurfaceManifest{
		Version: 1,
		Rows:    rows,
	}
}

// EncodeSurfaceManifest renders stable, newline-terminated JSON for the
// committed manifest and check-mode comparisons.
func EncodeSurfaceManifest(manifest SurfaceManifest) ([]byte, error) {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(raw)
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func compareSurfaceManifestRows(a, b SurfaceManifestRow) int {
	switch {
	case a.Kind < b.Kind:
		return -1
	case a.Kind > b.Kind:
		return 1
	case a.Name < b.Name:
		return -1
	case a.Name > b.Name:
		return 1
	case a.Source < b.Source:
		return -1
	case a.Source > b.Source:
		return 1
	default:
		return 0
	}
}

func jsonContractRows() []SurfaceManifestRow {
	rows := []SurfaceManifestRow{}
	for _, def := range commandRegistry {
		if !strings.Contains(def.Arguments, "--json") {
			continue
		}
		if def.ID == "evidence" {
			rows = append(rows,
				SurfaceManifestRow{
					Kind:   "json-contract",
					Name:   "evidence-task-json",
					Source: commandSourcePath(def.ID),
					Docs:   "docs/reference/commands.md",
					Token:  "slipway evidence task --task-id t-01 --run-summary-version 1 --task-kind code --verdict pass --evidence-ref \"test:go-test\" --json",
				},
				SurfaceManifestRow{
					Kind:   "json-contract",
					Name:   "evidence-skill-json",
					Source: commandSourcePath(def.ID),
					Docs:   "docs/reference/commands.md",
					Token:  "slipway evidence skill --skill <name> --verdict pass --json",
				},
			)
			continue
		}
		rows = append(rows, SurfaceManifestRow{
			Kind:   "json-contract",
			Name:   def.ID + "-json",
			Source: commandSourcePath(def.ID),
			Docs:   "docs/reference/commands.md",
			Token:  jsonContractDocsToken(def.ID),
		})
	}
	return rows
}

func commandSkillRows() []SurfaceManifestRow {
	hasCommandSkillSurface := false
	for _, cfg := range Registry() {
		if cfg.CommandSkillSurface {
			hasCommandSkillSurface = true
			break
		}
	}
	if !hasCommandSkillSurface {
		return nil
	}

	rows := []SurfaceManifestRow{}
	for _, id := range commandIDs() {
		name := adapterSkillName(id)
		rows = append(rows, SurfaceManifestRow{
			Kind:   "skill",
			Name:   name,
			Source: "internal/toolgen/toolgen.go:commandRegistry",
			Docs:   "docs/reference/ai-tools.md",
			Token:  commandSkillDocsToken(id),
		})
	}
	return rows
}

func commandSourcePath(commandID string) string {
	return "cmd/" + strings.ReplaceAll(commandID, "-", "_") + ".go"
}

func commandSkillDocsToken(commandID string) string {
	return "$" + adapterSkillName(commandID)
}

func jsonContractDocsToken(commandID string) string {
	switch commandID {
	case "delete":
		return "slipway delete --change <slug> --json"
	case "instructions":
		return "slipway instructions <artifact> --json"
	case "preset":
		return "slipway preset <level> --json"
	}
	return "slipway " + commandID + " --json"
}

func skillDocsToken(name string) string {
	return "`" + name + "/SKILL.md`"
}

func documentationRows() []SurfaceManifestRow {
	return []SurfaceManifestRow{
		{
			Kind:   "documentation",
			Name:   "README.md",
			Source: "README.md",
			Docs:   "README.md",
			Token:  "Generated surfaces",
		},
		{
			Kind:   "documentation",
			Name:   "docs/reference/ai-tools.md",
			Source: "docs/reference/ai-tools.md",
			Docs:   "docs/reference/ai-tools.md",
			Token:  "Generated Command Surface",
		},
		{
			Kind:   "documentation",
			Name:   "docs/reference/commands.md",
			Source: "docs/reference/commands.md",
			Docs:   "docs/reference/commands.md",
			Token:  "Command Reference", // #nosec G101 -- manifest token is a docs search string, not a credential.
		},
		{
			Kind:   "documentation",
			Name:   "docs/how-to/recover-and-troubleshoot.md",
			Source: "docs/how-to/recover-and-troubleshoot.md",
			Docs:   "docs/how-to/recover-and-troubleshoot.md",
			Token:  "Diagnostic JSON",
		},
	}
}
