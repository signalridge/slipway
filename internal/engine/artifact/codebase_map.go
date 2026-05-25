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
- Recheck routing:
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
		content, ok := codebaseMapDocTemplates[name]
		if !ok {
			return nil, fmt.Errorf("missing codebase map template for %s", name)
		}
		baseline := codebaseMapBaselineDoc(root, name)
		if data, err := os.ReadFile(path); err == nil {
			if CodebaseMapDocIsScaffoldOnly(name, string(data)) {
				if err := os.WriteFile(path, []byte(baseline), 0o644); err != nil {
					return nil, err
				}
			}
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		if strings.TrimSpace(baseline) == "" {
			baseline = content
		}
		if err := os.WriteFile(path, []byte(baseline), 0o644); err != nil {
			return nil, err
		}
		created = append(created, path)
	}
	return created, nil
}

type codebaseMapFacts struct {
	Module      string
	GoVersion   string
	Requires    []string
	TopDirs     []string
	EntryPoints []string
	HasTests    bool
}

func inspectCodebaseMapFacts(root string) codebaseMapFacts {
	facts := codebaseMapFacts{}
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "module "):
				facts.Module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			case strings.HasPrefix(line, "go "):
				facts.GoVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
			case strings.HasPrefix(line, "github.com/") || strings.HasPrefix(line, "golang.org/") || strings.HasPrefix(line, "gopkg.in/"):
				fields := strings.Fields(line)
				if len(fields) > 0 {
					facts.Requires = append(facts.Requires, fields[0])
				}
			}
		}
	}
	if entries, err := os.ReadDir(root); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "artifacts" {
				continue
			}
			if entry.IsDir() {
				facts.TopDirs = append(facts.TopDirs, name+"/")
				continue
			}
			if name == "main.go" || name == "go.mod" || strings.HasSuffix(name, ".md") {
				facts.EntryPoints = append(facts.EntryPoints, name)
			}
		}
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || facts.HasTests {
			return nil
		}
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "artifacts" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			facts.HasTests = true
		}
		return nil
	})
	slices.Sort(facts.Requires)
	facts.Requires = slices.Compact(facts.Requires)
	slices.Sort(facts.TopDirs)
	slices.Sort(facts.EntryPoints)
	return facts
}

func codebaseMapBaselineDoc(root, name string) string {
	facts := inspectCodebaseMapFacts(root)
	module := nonEmptyFact(facts.Module, "not detected")
	goVersion := nonEmptyFact(facts.GoVersion, "not detected")
	topDirs := strings.Join(facts.TopDirs, ", ")
	if topDirs == "" {
		topDirs = "not detected"
	}
	entryPoints := strings.Join(facts.EntryPoints, ", ")
	if entryPoints == "" {
		entryPoints = "not detected"
	}
	deps := firstFacts(facts.Requires, 6)
	tests := "Go tests not detected by baseline scan"
	if facts.HasTests {
		tests = "Go *_test.go files are present"
	}

	switch name {
	case "STACK.md":
		return fmt.Sprintf(`# Stack

- Languages: Go
- Frameworks and runtimes: Cobra-based CLI module %s
- Build and test tooling: go build ./...; go test ./...
- Key dependencies: %s
- Notes: go.mod declares Go %s
`, module, deps, goVersion)
	case "INTEGRATIONS.md":
		return `# Integrations

- External APIs: Git CLI is used for repository and worktree inspection.
- Infrastructure bindings: Local filesystem state under artifacts/, .slipway.yaml, and git-local runtime directories.
- Datastores and queues: No service datastore detected by baseline scan; Slipway stores YAML, JSON, JSONL, and Markdown artifacts on disk.
- File formats and protocols: YAML change authority, JSON CLI output, JSONL lifecycle events, Markdown governed artifacts.
- Notes: Integration inventory is deterministic baseline context; refine with project-specific external services when present.
`
	case "ARCHITECTURE.md":
		return `# Architecture

- Module responsibilities: cmd/ owns CLI surfaces; internal/state owns change authority and filesystem layout; internal/engine owns progression, governance, artifact, and gate logic.
- Dependency flow: CLI commands assemble model state and delegate durable state changes to internal/state and workflow decisions to internal/engine.
- Coupling hotspots: lifecycle progression, artifact readiness, worktree binding, and archive migration share change.yaml path authority.
- Current change blast radius: governed workflow creation, codebase-map context, and done/archive reporting.
- Notes: Baseline was generated from repository layout and known Slipway package boundaries.
`
	case "STRUCTURE.md":
		return fmt.Sprintf(`# Structure

- Directory layout: %s
- Entry points: %s
- Generated versus handwritten boundaries: internal/tmpl contains generated prompt/skill surfaces; cmd/ and internal/ contain handwritten Go runtime code.
- Ownership hints: Tests are colocated as *_test.go files under cmd/ and internal/.
- Notes: %s.
`, topDirs, entryPoints, tests)
	case "CONVENTIONS.md":
		return `# Conventions

- Naming: CLI commands live in cmd/ with make<Command>Cmd constructors; workflow states and durable schemas live in internal/model.
- File organization: Runtime state helpers belong under internal/state; progression decisions belong under internal/engine/progression.
- Error handling: CLI-facing failures use structured reason codes and typed CLI errors where user remediation matters.
- Configuration: .slipway.yaml is the project-local governance configuration authority.
- State management: change.yaml is current-state authority; lifecycle.jsonl is append-only audit evidence.
- Notes: Generated host-skill templates should stay synchronized with runtime contracts.
`
	case "TESTING.md":
		return `# Testing

- Test layout: cmd/*_test.go covers CLI contracts; internal/**/*_test.go covers state, artifact, progression, governance, and template behavior.
- Coverage hotspots: next/run/status JSON contracts, governed lifecycle gates, archive migration, worktree binding, and generated skill/template drift.
- Coverage gaps: End-to-end governed workflow tests are intentionally heavier and should use explicit timeouts.
- Verification commands: go test -timeout=20m ./... -count=1; go build ./...
- Fixture patterns: Tests commonly create temp workspaces, seed governed bundles, write verification YAML, and assert JSON command output.
- Notes: Prefer focused regression tests before full-suite verification.
`
	case "CONCERNS.md":
		return `# Concerns

- Architectural pressure points: Worktree binding must happen before canonical governed artifacts are treated as reviewed execution inputs.
- Brittle areas: Scaffold-only codebase maps, planning-vs-execution evidence freshness, and archive relocation can create misleading authority signals.
- Migration traps: Changing artifact roots must preserve repairability, archive discoverability, and change.yaml as the single current-state authority.
- Recheck routing: Planning artifacts invalidate planning evidence; task execution drift invalidates execution evidence; assurance-only edits stay in verification/closeout checks.
- Notes: Treat placeholder files as advisory until populated with concrete repository facts.
`
	default:
		return codebaseMapDocTemplates[name]
	}
}

func nonEmptyFact(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstFacts(values []string, max int) string {
	if len(values) == 0 {
		return "not detected"
	}
	if len(values) > max {
		values = values[:max]
	}
	return strings.Join(values, ", ")
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
