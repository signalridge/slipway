package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"github.com/signalridge/slipway/internal/tmpl"
	"github.com/signalridge/slipway/internal/toolgen"
	"gopkg.in/yaml.v3"
)

type GovernanceRegistryError struct {
	Path string
	Err  error
}

func (e *GovernanceRegistryError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("parse skill frontmatter or registry config %q: %v", e.Path, e.Err)
}

func (e *GovernanceRegistryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type governanceFrontMatter struct {
	// Active fields (used by loader)
	SkillID     string `yaml:"skill_id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func LoadGovernanceRegistry(root string) ([]Definition, error) {
	// The Go registry is the routing authority. Generated SKILL.md files are an
	// optional runtime overlay for adapter-facing prompt text only; they may not
	// override workflow state, hard gates, or agent defaults.
	definitions := defaultGovernanceRegistryMap()

	dirs := candidateSkillDirs(root)
	for _, dir := range dirs {
		pattern := filepath.Join(dir, "slipway-*", "SKILL.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		slices.Sort(matches)

		for _, path := range matches {
			def, ok, err := parseGovernanceSkillFromFile(path, definitions)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			definitions[def.Name] = def
		}
	}

	configPath, err := governanceConfigPath(root)
	if err != nil {
		return nil, err
	}
	cfg, err := model.LoadConfig(configPath)
	switch {
	case err == nil:
		if err := applyConfiguredAgentMappings(definitions, cfg.Agents.Mappings, configPath); err != nil {
			return nil, err
		}
	case errors.Is(err, fs.ErrNotExist):
		// Config is optional for governance registry overrides.
	default:
		return nil, &GovernanceRegistryError{Path: configPath, Err: err}
	}

	return definitionsToSortedSlice(definitions), nil
}

func governanceConfigPath(root string) (string, error) {
	canonicalRoot, err := fsutil.ResolveCanonicalScopeRoot(root)
	switch {
	case err == nil:
		return state.ConfigPath(canonicalRoot), nil
	case errors.Is(err, fsutil.ErrProjectRootNotFound):
		return state.ConfigPath(root), nil
	default:
		return "", err
	}
}

func applyConfiguredAgentMappings(definitions map[string]Definition, mappings map[string]string, path string) error {
	if len(mappings) == 0 {
		return nil
	}

	validAgents := map[string]struct{}{}
	for _, name := range tmpl.AgentNames() {
		validAgents[name] = struct{}{}
	}

	for skillName, agentName := range mappings {
		skillName = strings.TrimSpace(skillName)
		agentName = strings.TrimSpace(agentName)

		def, ok := definitions[skillName]
		if !ok {
			return &GovernanceRegistryError{
				Path: path,
				Err:  fmt.Errorf("agents.mappings.%s: unknown governance skill", skillName),
			}
		}
		if _, ok := validAgents[agentName]; !ok {
			return &GovernanceRegistryError{
				Path: path,
				Err:  fmt.Errorf("agents.mappings.%s: unknown agent %q", skillName, agentName),
			}
		}
		status, err := configuredAgentStatus(agentName)
		if err != nil {
			return err
		}
		if status == "manual_only" {
			return &GovernanceRegistryError{
				Path: path,
				Err:  fmt.Errorf("agents.mappings.%s: agent %q is manual-only and cannot be mapped to governance skills", skillName, agentName),
			}
		}
		def.AgentHint = agentName
		definitions[skillName] = def
	}
	return nil
}

func configuredAgentStatus(name string) (string, error) {
	templatePath := filepath.ToSlash(filepath.Join("internal", "tmpl", "templates", "agents", name+".md"))
	content, err := tmpl.Content("agents/" + name + ".md")
	if err != nil {
		return "", &GovernanceRegistryError{Path: templatePath, Err: err}
	}
	frontMatter, ok := extractFrontMatter(content)
	if !ok {
		return "", nil
	}
	var fm struct {
		AgentStatus string `yaml:"agent_status"`
	}
	if err := yaml.Unmarshal([]byte(frontMatter), &fm); err != nil {
		return "", &GovernanceRegistryError{Path: templatePath, Err: err}
	}
	return strings.TrimSpace(fm.AgentStatus), nil
}

func parseGovernanceSkillFromFile(
	path string,
	defaults map[string]Definition,
) (Definition, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, false, err
	}
	frontmatterText, ok := extractFrontMatter(string(raw))
	if !ok {
		return Definition{}, false, nil
	}
	var fm governanceFrontMatter
	if err := yaml.Unmarshal([]byte(frontmatterText), &fm); err != nil {
		return Definition{}, false, &GovernanceRegistryError{Path: path, Err: err}
	}

	skillID := strings.TrimSpace(fm.SkillID)
	publicName := strings.TrimSpace(fm.Name)
	if skillID == "" {
		if bareID, ok := bareGovernanceIDFromOverlayPath(path, defaults); ok {
			return Definition{}, false, &GovernanceRegistryError{
				Path: path,
				Err:  fmt.Errorf("missing skill_id for governance overlay %q", bareID),
			}
		}
		if bareID, ok := bareGovernanceIDFromPublicName(publicName, defaults); ok {
			return Definition{}, false, &GovernanceRegistryError{
				Path: path,
				Err:  fmt.Errorf("missing skill_id for governance overlay %q", bareID),
			}
		}
		return Definition{}, false, nil
	}

	def, hasDefault := defaults[skillID]
	if !hasDefault {
		return Definition{}, false, nil
	}
	if publicName != "" && publicName != toolgen.AdapterSkillName(skillID) {
		return Definition{}, false, &GovernanceRegistryError{
			Path: path,
			Err:  fmt.Errorf("governance overlay name %q must equal %q", publicName, toolgen.AdapterSkillName(skillID)),
		}
	}

	// Return the default definition — routing metadata is exclusively owned by the Go registry.
	return def, true, nil
}

func bareGovernanceIDFromOverlayPath(path string, defaults map[string]Definition) (string, bool) {
	publicName := filepath.Base(filepath.Dir(path))
	return bareGovernanceIDFromPublicName(publicName, defaults)
}

func bareGovernanceIDFromPublicName(publicName string, defaults map[string]Definition) (string, bool) {
	publicName = strings.TrimSpace(publicName)
	if !strings.HasPrefix(publicName, "slipway-") {
		return "", false
	}
	bareID := strings.TrimPrefix(publicName, "slipway-")
	_, ok := defaults[bareID]
	return bareID, ok
}

func candidateSkillDirs(root string) []string {
	// These are generated adapter trees under the active workspace/worktree.
	// Missing directories are expected; loader falls back to Go defaults.
	var dirs []string
	for _, cfg := range toolgen.Registry() {
		dirs = append(dirs, filepath.Join(root, cfg.SkillsDir))
	}
	slices.Sort(dirs)
	return stringutil.Unique(dirs)
}

func definitionsToSortedSlice(m map[string]Definition) []Definition {
	out := make([]Definition, 0, len(m))
	for _, def := range m {
		copyDef := def
		// Definition fields are value types, no deep copy needed.
		out = append(out, copyDef)
	}
	slices.SortFunc(out, func(a, b Definition) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return out
}

func extractFrontMatter(content string) (string, bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 {
		return "", false
	}

	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) || strings.TrimSpace(lines[start]) != "---" {
		return "", false
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "---" {
			continue
		}
		return strings.Join(lines[start+1:i], "\n"), true
	}
	return "", false
}
