package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/tmpl"
)

var baseGovernedFiles = []string{
	"change.yaml",
	"proposal.md",
	"spec.md",
	"design.md",
	"tasks.md",
	"assurance.md",
}

var exploreHeadings = []string{
	"## Objectives",
	"## Unknowns",
	"## Assumptions",
	"## Scope Boundaries",
	"## Validation Plan",
}

var assuranceHeadings = []string{
	"## Scope Summary",
	"## Verification Verdict",
	"## Evidence Index",
	"## Residual Risks and Exceptions",
	"## Archive Decision",
}

var staleGraph = map[string][]string{
	"change.yaml":  {"proposal.md", "spec.md", "design.md", "tasks.md", "assurance.md"},
	"proposal.md":  {"spec.md", "design.md", "tasks.md", "assurance.md"},
	"spec.md":      {"design.md", "tasks.md", "assurance.md"},
	"design.md":    {"tasks.md", "assurance.md"},
	"tasks.md":     {"assurance.md"},
	"assurance.md": {},
	"explore.md":   {"proposal.md", "spec.md", "design.md", "tasks.md", "assurance.md"},
}

func TemplateContent(name string) (string, error) {
	return tmpl.Content(name)
}

func ScaffoldGovernedBundle(root, slug string, level model.Level) error {
	if level == model.LevelL1 {
		return nil
	}
	if level != model.LevelL2 && level != model.LevelL3 {
		return fmt.Errorf("governed scaffold requires level L2/L3, got %q", level)
	}
	if strings.TrimSpace(slug) == "" {
		return fmt.Errorf("slug is required")
	}

	base := filepath.Join(root, "aircraft", "changes", slug)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}

	files := append([]string(nil), baseGovernedFiles...)
	if level == model.LevelL3 {
		files = append(files, "explore.md")
	}
	slices.Sort(files)

	for _, file := range files {
		path := filepath.Join(base, file)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}

		content, err := TemplateContent(file)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func StalePropagationOrder(start string) ([]string, error) {
	if _, ok := staleGraph[start]; !ok {
		return nil, fmt.Errorf("unknown artifact %q", start)
	}

	order := make([]string, 0)
	visited := map[string]struct{}{}
	queue := []string{start}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if _, seen := visited[node]; seen {
			continue
		}
		visited[node] = struct{}{}
		order = append(order, node)

		next := append([]string(nil), staleGraph[node]...)
		slices.Sort(next)
		queue = append(queue, next...)
	}

	return order, nil
}

func ValidateExploreStructure(content string) error {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	indices := make([]int, 0, len(exploreHeadings))
	searchFrom := 0

	for _, heading := range exploreHeadings {
		idx := -1
		for i := searchFrom; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == heading {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("missing required heading %q", heading)
		}
		indices = append(indices, idx)
		searchFrom = idx + 1
	}

	for i, heading := range exploreHeadings {
		start := indices[i] + 1
		end := len(lines)
		if i+1 < len(indices) {
			end = indices[i+1]
		}
		hasContent := false
		for _, line := range lines[start:end] {
			if strings.TrimSpace(line) != "" {
				hasContent = true
				break
			}
		}
		if !hasContent {
			return fmt.Errorf("section %q must have non-empty content", heading)
		}
	}
	return nil
}

func ValidateAssuranceStructure(content string) error {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	searchFrom := 0
	for _, heading := range assuranceHeadings {
		idx := -1
		for i := searchFrom; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == heading {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("missing required heading %q", heading)
		}
		searchFrom = idx + 1
	}
	return nil
}

func ArchiveBundle(root, slug string) error {
	src := filepath.Join(root, "aircraft", "changes", slug)
	dst := filepath.Join(root, "aircraft", "changes", "archived", slug)

	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}
