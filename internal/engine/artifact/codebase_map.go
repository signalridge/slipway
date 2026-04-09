package artifact

import (
	"fmt"
	"os"
	"path/filepath"

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
