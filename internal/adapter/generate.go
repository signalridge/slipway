package adapter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/tmpl"
)

type generatedFile struct {
	Relative   string
	Data       []byte
	Capability string
}

const codexExplicitInvocationPolicy = "policy:\n  allow_implicit_invocation: false\n"

func canonicalCapabilityBody(capability string) (string, error) {
	content, err := capabilityTemplate(capability)
	if err != nil {
		return "", err
	}
	_, body, err := splitCapabilityTemplate(content)
	if err != nil {
		return "", err
	}
	common, err := tmpl.Content("_partials/common.tmpl")
	if err != nil {
		return "", fmt.Errorf("load shared capability boundaries: %w", err)
	}
	return body + "\n" + common, nil
}

func capabilityDescription(capability string) (string, error) {
	content, err := capabilityTemplate(capability)
	if err != nil {
		return "", err
	}
	frontmatter, _, err := splitCapabilityTemplate(content)
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(frontmatter, "\n") {
		key, value, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(key) == "description" {
			description := strings.TrimSpace(value)
			if description == "" {
				break
			}
			return strings.Trim(description, "\"'"), nil
		}
	}
	return "", fmt.Errorf("capability %s has no description", capability)
}

func capabilityTemplate(capability string) (string, error) {
	if !strings.HasPrefix(capability, "slipway-") {
		return "", fmt.Errorf("invalid capability name %q", capability)
	}
	content, err := tmpl.Content("skills/" + strings.TrimPrefix(capability, "slipway-") + "/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("load capability %s: %w", capability, err)
	}
	return content, nil
}

func splitCapabilityTemplate(content string) (frontmatter string, body string, err error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", fmt.Errorf("capability template has no frontmatter")
	}
	const delimiter = "\n---\n"
	end := strings.Index(content[len("---\n"):], delimiter)
	if end < 0 {
		return "", "", fmt.Errorf("capability template has unterminated frontmatter")
	}
	end += len("---\n")
	return content[len("---\n"):end], content[end+len(delimiter):], nil
}

func generateHostFiles(host Host) ([]generatedFile, error) {
	if host.SurfaceKind == "" {
		return nil, nil
	}

	files := make([]generatedFile, 0, len(capabilityNames)*2+1)
	for _, capability := range capabilityNames {
		body, err := canonicalCapabilityBody(capability)
		if err != nil {
			return nil, fmt.Errorf("render %s for %s: %w", capability, host.ID, err)
		}
		description, err := capabilityDescription(capability)
		if err != nil {
			return nil, fmt.Errorf("describe %s for %s: %w", capability, host.ID, err)
		}

		switch host.SurfaceKind {
		case "skill":
			content, err := capabilityTemplate(capability)
			if err != nil {
				return nil, fmt.Errorf("render %s for %s: %w", capability, host.ID, err)
			}
			common, err := tmpl.Content("_partials/common.tmpl")
			if err != nil {
				return nil, fmt.Errorf("load shared capability boundaries: %w", err)
			}
			capabilityRoot := filepath.Join(host.SkillsDir, capability)
			files = append(files, generatedFile{
				Relative:   filepath.ToSlash(filepath.Join(capabilityRoot, "SKILL.md")),
				Data:       []byte(content + "\n" + common),
				Capability: capability,
			})
			if host.ID == "codex" {
				files = append(files, generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(capabilityRoot, "agents", "openai.yaml")),
					Data:       []byte(codexExplicitInvocationPolicy),
					Capability: capability,
				})
			}
		case "copilot_agent":
			content := fmt.Sprintf("---\nname: %s\ndescription: %q\ndisable-model-invocation: true\n---\n\n%s", capability, description, body)
			files = append(files, generatedFile{
				Relative:   filepath.ToSlash(filepath.Join(".github/agents", capability+".agent.md")),
				Data:       []byte(content),
				Capability: capability,
			})
		case "kilo_command":
			files = append(files,
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".kilo/commands", capability+".md")),
					Data:       fmt.Appendf(nil, "---\ndescription: %q\nsubtask: false\n---\n\n@.kilocode/slipway/capabilities/%s.md\n", description, capability),
					Capability: capability,
				},
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".kilocode/slipway/capabilities", capability+".md")),
					Data:       []byte(body),
					Capability: capability,
				},
			)
		case "kiro_ide":
			files = append(files,
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".kiro/steering", capability+".md")),
					Data:       fmt.Appendf(nil, "---\ninclusion: manual\n---\n\n#[[file:.kiro/slipway/capabilities/%s.md]]\n", capability),
					Capability: capability,
				},
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".kiro/slipway/capabilities", capability+".md")),
					Data:       []byte(body),
					Capability: capability,
				},
			)
		case "kiro_cli":
			agent, err := json.MarshalIndent(struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Prompt      string   `json:"prompt"`
				Tools       []string `json:"tools"`
			}{
				Name:        capability,
				Description: description,
				Prompt:      "file://../slipway/capabilities/" + capability + ".md",
				Tools:       []string{"*"},
			}, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("encode %s agent: %w", capability, err)
			}
			files = append(files,
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".kiro/agents", capability+".json")),
					Data:       append(agent, '\n'),
					Capability: capability,
				},
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".kiro/slipway/capabilities", capability+".md")),
					Data:       []byte(body),
					Capability: capability,
				},
			)
		case "opencode_command":
			files = append(files,
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".opencode/commands", capability+".md")),
					Data:       fmt.Appendf(nil, "---\ndescription: %q\n---\n\n@.opencode/slipway/capabilities/%s.md\n", description, capability),
					Capability: capability,
				},
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".opencode/slipway/capabilities", capability+".md")),
					Data:       []byte(body),
					Capability: capability,
				},
			)
		case "windsurf_workflow":
			files = append(files,
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".windsurf/workflows", capability+".md")),
					Data:       fmt.Appendf(nil, "---\ndescription: %q\n---\n\n@.windsurf/slipway/capabilities/%s.md\n", description, capability),
					Capability: capability,
				},
				generatedFile{
					Relative:   filepath.ToSlash(filepath.Join(".windsurf/slipway/capabilities", capability+".md")),
					Data:       []byte(body),
					Capability: capability,
				},
			)
		default:
			return nil, fmt.Errorf("unsupported surface kind %q for %s", host.SurfaceKind, host.ID)
		}
	}

	reference, err := tmpl.Content("skills/clarify/references/decision-interview.md")
	if err != nil {
		return nil, fmt.Errorf("load clarification reference: %w", err)
	}
	files = append(files, generatedFile{
		Relative: referencePath(host),
		Data:     []byte(reference),
	})
	return files, nil
}

func referenceRoot(host Host) string {
	switch host.SurfaceKind {
	case "skill":
		return host.SkillsDir
	case "copilot_agent":
		return ".github/agents"
	case "kilo_command":
		return ".kilocode/slipway/capabilities"
	case "kiro_ide", "kiro_cli":
		return ".kiro/slipway/capabilities"
	case "opencode_command":
		return ".opencode/slipway/capabilities"
	case "windsurf_workflow":
		return ".windsurf/slipway/capabilities"
	default:
		return host.SkillsDir
	}
}

func referencePath(host Host) string {
	root := referenceRoot(host)
	if host.SurfaceKind == "skill" {
		return filepath.ToSlash(filepath.Join(root, "slipway-clarify", "references", "decision-interview.md"))
	}
	return filepath.ToSlash(filepath.Join(root, "references", "decision-interview.md"))
}
