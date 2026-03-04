package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/toolgen"
	"gopkg.in/yaml.v3"
)

type governanceFrontMatter struct {
	Type                string   `yaml:"type"`
	SkillName           string   `yaml:"skill_name"`
	State               string   `yaml:"state"`
	MitigationTarget    string   `yaml:"mitigation_target"`
	RunSummaryBound     bool     `yaml:"run_summary_bound"`
	RequiredLevels      []string `yaml:"required_levels"`
	AutoModeRequired    bool     `yaml:"auto_mode_required"`
	CloseoutConditional bool     `yaml:"closeout_conditional"`
	ReviewerIndependent bool     `yaml:"reviewer_independent"`
}

func LoadGovernanceRegistry(root string) ([]Definition, error) {
	definitions := defaultGovernanceRegistryMap()
	loadedAny := false

	dirs := candidateSkillDirs(root)
	for _, dir := range dirs {
		pattern := filepath.Join(dir, "spln-*", "SKILL.md")
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
		return Definition{}, false, fmt.Errorf("parse skill frontmatter %q: %w", path, err)
	}
	if strings.TrimSpace(strings.ToLower(fm.Type)) != "governance" {
		return Definition{}, false, nil
	}

	skillName := strings.TrimSpace(fm.SkillName)
	if skillName == "" {
		return Definition{}, false, fmt.Errorf("governance skill frontmatter missing skill_name: %q", path)
	}
	def, hasDefault := defaults[skillName]
	if !hasDefault {
		return Definition{}, false, fmt.Errorf("unknown governance skill_name %q in %q", skillName, path)
	}

	state := model.WorkflowState(strings.TrimSpace(fm.State))
	if strings.TrimSpace(fm.State) == "" {
		state = def.State
	}
	if state == "" {
		return Definition{}, false, fmt.Errorf("governance skill %q missing state in %q", skillName, path)
	}

	levels, err := parseRequiredLevels(fm.RequiredLevels)
	if err != nil {
		return Definition{}, false, fmt.Errorf("governance skill %q invalid required_levels: %w", skillName, err)
	}
	if len(levels) == 0 {
		levels = append([]model.Level(nil), def.RequiredLevels...)
	}

	mitigation := strings.TrimSpace(fm.MitigationTarget)
	if mitigation == "" {
		mitigation = def.Mitigation
	}

	return Definition{
		Name:                skillName,
		State:               state,
		Mitigation:          mitigation,
		RunSummaryBound:     fm.RunSummaryBound || def.RunSummaryBound,
		RequiredLevels:      levels,
		AutoModeRequired:    fm.AutoModeRequired || def.AutoModeRequired,
		CloseoutConditional: fm.CloseoutConditional || def.CloseoutConditional,
		ReviewerIndependent: fm.ReviewerIndependent || def.ReviewerIndependent,
	}, true, nil
}

func parseRequiredLevels(raw []string) ([]model.Level, error) {
	levels := []model.Level{}
	for _, item := range raw {
		level := model.Level(strings.ToUpper(strings.TrimSpace(item)))
		if level == "" {
			continue
		}
		if !level.IsValid() {
			return nil, fmt.Errorf("invalid level %q", item)
		}
		if slices.Contains(levels, level) {
			continue
		}
		levels = append(levels, level)
	}
	slices.Sort(levels)
	return levels, nil
}

func candidateSkillDirs(root string) []string {
	dirs := []string{
		filepath.Join(root, ".spln", "skills"),
	}
	for _, cfg := range toolgen.Registry() {
		dirs = append(dirs, filepath.Join(root, cfg.SkillsDir))
	}
	slices.Sort(dirs)
	return uniqueStrings(dirs)
}

func uniqueStrings(input []string) []string {
	out := make([]string, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func definitionsToSortedSlice(m map[string]Definition) []Definition {
	out := make([]Definition, 0, len(m))
	for _, def := range m {
		copyDef := def
		copyDef.RequiredLevels = append([]model.Level(nil), def.RequiredLevels...)
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
