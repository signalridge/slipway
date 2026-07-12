package adapter

import (
	"fmt"
	"path/filepath"

	"github.com/signalridge/slipway/internal/tmpl"
)

type generatedFile struct {
	Relative string
	Data     []byte
}

const codexExplicitInvocationPolicy = "policy:\n  allow_implicit_invocation: false\n"

func generateHostFiles(host Host) ([]generatedFile, error) {
	common, err := tmpl.Content("_partials/common.tmpl")
	if err != nil {
		return nil, fmt.Errorf("load shared capability boundaries: %w", err)
	}
	capacity := len(capabilityNames) + 1
	if host.ID == "codex" {
		capacity += len(capabilityNames)
	}
	files := make([]generatedFile, 0, capacity)
	for _, capability := range capabilityNames {
		templateName := "skills/" + capability[len("slipway-"):] + "/SKILL.md"
		content, err := tmpl.Content(templateName)
		if err != nil {
			return nil, fmt.Errorf("render %s for %s: %w", capability, host.ID, err)
		}
		capabilityRoot := filepath.Join(host.SkillsDir, capability)
		files = append(files, generatedFile{
			Relative: filepath.ToSlash(filepath.Join(capabilityRoot, "SKILL.md")),
			Data:     []byte(content + "\n" + common),
		})
		if host.ID == "codex" {
			files = append(files, generatedFile{
				Relative: filepath.ToSlash(filepath.Join(capabilityRoot, "agents", "openai.yaml")),
				Data:     []byte(codexExplicitInvocationPolicy),
			})
		}
	}
	reference, err := tmpl.Content("skills/clarify/references/decision-interview.md")
	if err != nil {
		return nil, fmt.Errorf("load clarification reference: %w", err)
	}
	files = append(files, generatedFile{
		Relative: filepath.ToSlash(filepath.Join(host.SkillsDir, "slipway-clarify", "references", "decision-interview.md")),
		Data:     []byte(reference),
	})
	return files, nil
}
