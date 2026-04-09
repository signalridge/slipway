package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/stringutil"
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
	return fmt.Sprintf("parse skill frontmatter %q: %v", e.Path, e.Err)
}

func (e *GovernanceRegistryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type governanceFrontMatter struct {
	// Active fields (used by loader)
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func LoadGovernanceRegistry(root string) ([]Definition, error) {
	definitions := defaultGovernanceRegistryMap()
	loadedAny := false

	dirs := candidateSkillDirs(root)
	for _, dir := range dirs {
		pattern := filepath.Join(dir, "slipway", "*", "SKILL.md")
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
			loadedAny = true
			definitions[def.Name] = def
		}
	}

	if !loadedAny {
		return GovernanceRegistry(), nil
	}
	return definitionsToSortedSlice(definitions), nil
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

	skillName := strings.TrimSpace(fm.Name)
	if skillName == "" {
		return Definition{}, false, nil
	}

	// Name-based lookup replaces the old type == "governance" filter.
	// Non-governance skills are simply not in the defaults map.
	def, hasDefault := defaults[skillName]
	if !hasDefault {
		return Definition{}, false, nil
	}

	// Return the default definition — routing metadata is exclusively owned by the Go registry.
	return def, true, nil
}

func candidateSkillDirs(root string) []string {
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
