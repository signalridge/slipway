package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/state"
)

var codebaseMapDocNames = []string{
	"STACK.md",
	"INTEGRATIONS.md",
	"ARCHITECTURE.md",
	"STRUCTURE.md",
	"CONVENTIONS.md",
	"TESTING.md",
	"CONCERNS.md",
}

var codebaseMapDocKeys = map[string]string{
	"STACK.md":        "stack",
	"INTEGRATIONS.md": "integrations",
	"ARCHITECTURE.md": "architecture",
	"STRUCTURE.md":    "structure",
	"CONVENTIONS.md":  "conventions",
	"TESTING.md":      "testing",
	"CONCERNS.md":     "concerns",
}

var codebaseMapDocTemplates = map[string]string{
	"STACK.md": `# Stack

- Languages:
- Frameworks and runtimes:
- Build and test tooling:
- Key dependencies:
- Notes:
`,
	"INTEGRATIONS.md": `# Integrations

- External APIs:
- Infrastructure bindings:
- Datastores and queues:
- File formats and protocols:
- Notes:
`,
	"ARCHITECTURE.md": `# Architecture

- Module responsibilities:
- Dependency flow:
- Coupling hotspots:
- Current change blast radius:
- Notes:
`,
	"STRUCTURE.md": `# Structure

- Directory layout:
- Entry points:
- Generated versus handwritten boundaries:
- Ownership hints:
- Notes:
`,
	"CONVENTIONS.md": `# Conventions

- Naming:
- File organization:
- Error handling:
- Configuration:
- State management:
- Notes:
`,
	"TESTING.md": `# Testing

- Test layout:
- Coverage hotspots:
- Coverage gaps:
- Verification commands:
- Fixture patterns:
- Notes:
`,
	"CONCERNS.md": `# Concerns

- Architectural pressure points:
- Brittle areas:
- Migration traps:
- Deferred concerns:
- Notes:
`,
}

const (
	CodebaseMapStatusMissing      = "missing"
	CodebaseMapStatusPartial      = "partial"
	CodebaseMapStatusScaffoldOnly = "scaffold_only"
	CodebaseMapStatusPopulated    = "populated"
)

type CodebaseMapAssessment struct {
	Status           string            `json:"status"`
	DocStates        map[string]string `json:"doc_states,omitempty"`
	MissingDocs      []string          `json:"missing_docs,omitempty"`
	ScaffoldOnlyDocs []string          `json:"scaffold_only_docs,omitempty"`
	PopulatedDocs    []string          `json:"populated_docs,omitempty"`
}

func CodebaseMapDisplayDocs(displayRoot, codebaseMapDir string) map[string]string {
	docs := make(map[string]string, len(codebaseMapDocNames))
	for _, name := range codebaseMapDocNames {
		docs[codebaseMapDocKeys[name]] = state.DisplayPath(displayRoot, filepath.Join(codebaseMapDir, name))
	}
	return docs
}

func EnsureCodebaseMapDocs(root string) (created []string, err error) {
	dir := state.CodebaseMapDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	for _, name := range codebaseMapDocNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		content, ok := codebaseMapDocTemplates[name]
		if !ok {
			return nil, fmt.Errorf("missing codebase map template for %s", name)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, err
		}
		created = append(created, path)
	}
	return created, nil
}

func AssessCodebaseMapDocs(root string) (CodebaseMapAssessment, error) {
	dir := state.CodebaseMapDir(root)
	assessment := CodebaseMapAssessment{
		Status:    CodebaseMapStatusMissing,
		DocStates: map[string]string{},
	}

	for _, name := range codebaseMapDocNames {
		key := codebaseMapDocKeys[name]
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				assessment.DocStates[key] = CodebaseMapStatusMissing
				assessment.MissingDocs = append(assessment.MissingDocs, name)
				continue
			}
			return CodebaseMapAssessment{}, err
		}
		if CodebaseMapDocIsScaffoldOnly(name, string(data)) {
			assessment.DocStates[key] = CodebaseMapStatusScaffoldOnly
			assessment.ScaffoldOnlyDocs = append(assessment.ScaffoldOnlyDocs, name)
			continue
		}
		assessment.DocStates[key] = CodebaseMapStatusPopulated
		assessment.PopulatedDocs = append(assessment.PopulatedDocs, name)
	}

	slices.Sort(assessment.MissingDocs)
	slices.Sort(assessment.ScaffoldOnlyDocs)
	slices.Sort(assessment.PopulatedDocs)

	switch {
	case len(assessment.PopulatedDocs) == len(codebaseMapDocNames):
		assessment.Status = CodebaseMapStatusPopulated
	case len(assessment.PopulatedDocs) == 0 && len(assessment.ScaffoldOnlyDocs) > 0 && len(assessment.MissingDocs) == 0:
		assessment.Status = CodebaseMapStatusScaffoldOnly
	case len(assessment.PopulatedDocs) == 0 && len(assessment.ScaffoldOnlyDocs) == 0:
		assessment.Status = CodebaseMapStatusMissing
	default:
		assessment.Status = CodebaseMapStatusPartial
	}
	return assessment, nil
}

func CodebaseMapDocIsScaffoldOnly(name, content string) bool {
	if template, ok := codebaseMapDocTemplates[name]; ok && normalizeCodebaseMapDoc(content) == normalizeCodebaseMapDoc(template) {
		return true
	}
	return !hasSubstantiveCodebaseMapContent(content)
}

func hasSubstantiveCodebaseMapContent(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			item := strings.TrimSpace(trimmed[2:])
			if idx := strings.Index(item, ":"); idx >= 0 {
				if strings.TrimSpace(item[idx+1:]) == "" {
					continue
				}
			}
			return true
		}
		return true
	}
	return false
}

func normalizeCodebaseMapDoc(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.TrimSpace(content)
}
