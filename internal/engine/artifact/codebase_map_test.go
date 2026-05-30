package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureCodebaseMapDocsDetectsRustCargoBaseline(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(`[package]
name = "lattice-demo"
version = "0.1.0"

[dependencies]
serde = "1"
`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "lib.rs"), []byte("pub fn run() {}\n"), 0o644))

	created, err := EnsureCodebaseMapDocs(root)
	require.NoError(t, err)
	require.Len(t, created, len(codebaseMapDocNames))

	stack := readCodebaseMapDoc(t, root, "STACK.md")
	assert.Contains(t, stack, "- Languages: Rust")
	assert.Contains(t, stack, "cargo build --workspace; cargo test --workspace")
	assert.Contains(t, stack, "serde")
	assert.NotContains(t, stack, "Languages: Go")
	assert.NotContains(t, stack, "go build ./...")

	architecture := readCodebaseMapDoc(t, root, "ARCHITECTURE.md")
	assert.True(t, CodebaseMapDocIsScaffoldOnly("ARCHITECTURE.md", architecture))
	assert.NotContains(t, architecture, "cmd/ owns CLI surfaces")
	assert.NotContains(t, architecture, "internal/state owns change authority")

	assessment, err := AssessCodebaseMapDocs(root)
	require.NoError(t, err)
	assert.Equal(t, CodebaseMapStatusBaseline, assessment.Status)
	assert.Contains(t, assessment.BaselineDocs, "STACK.md")
	assert.Contains(t, assessment.BaselineDocs, "STRUCTURE.md")
	assert.Contains(t, assessment.BaselineDocs, "TESTING.md")
	assert.Contains(t, assessment.ScaffoldOnlyDocs, "ARCHITECTURE.md")
	assert.Empty(t, assessment.PopulatedDocs)
}

func TestEnsureCodebaseMapDocsDetectsGoModuleBaseline(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(`module example.com/slipway-like

go 1.26.3

require github.com/spf13/cobra v1.10.1
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))

	_, err := EnsureCodebaseMapDocs(root)
	require.NoError(t, err)

	stack := readCodebaseMapDoc(t, root, "STACK.md")
	assert.Contains(t, stack, "- Languages: Go")
	assert.Contains(t, stack, "Go module example.com/slipway-like")
	assert.Contains(t, stack, "go build ./...; go test ./...")
	assert.Contains(t, stack, "github.com/spf13/cobra")
	assert.Contains(t, stack, "go.mod declares Go 1.26.3")

	assessment, err := AssessCodebaseMapDocs(root)
	require.NoError(t, err)
	assert.Equal(t, CodebaseMapStatusBaseline, assessment.Status)
	assert.Contains(t, assessment.BaselineDocs, "STACK.md")
}

func TestEnsureCodebaseMapDocsUsesBlankScaffoldWhenNoFactsDetected(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# notes\n"), 0o644))

	_, err := EnsureCodebaseMapDocs(root)
	require.NoError(t, err)

	for _, name := range codebaseMapDocNames {
		assert.Equal(t, normalizeCodebaseMapDoc(codebaseMapDocTemplates[name]), normalizeCodebaseMapDoc(readCodebaseMapDoc(t, root, name)))
	}
	assessment, err := AssessCodebaseMapDocs(root)
	require.NoError(t, err)
	assert.Equal(t, CodebaseMapStatusScaffoldOnly, assessment.Status)
	assert.Empty(t, assessment.BaselineDocs)
	assert.Empty(t, assessment.PopulatedDocs)
}

func TestEnsureCodebaseMapDocsDetectsNodeAndPythonFacts(t *testing.T) {
	t.Run("node typescript package", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "name": "web-client",
  "scripts": {"build": "tsc", "test": "vitest"},
  "dependencies": {"react": "latest"},
  "devDependencies": {"typescript": "latest", "vitest": "latest"}
}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte("{}\n"), 0o644))

		_, err := EnsureCodebaseMapDocs(root)
		require.NoError(t, err)

		stack := readCodebaseMapDoc(t, root, "STACK.md")
		assert.Contains(t, stack, "JavaScript")
		assert.Contains(t, stack, "TypeScript")
		assert.Contains(t, stack, "Node.js package web-client")
		assert.Contains(t, stack, "npm run build; npm test")
		assert.Contains(t, stack, "react")
	})

	t.Run("python requirements project", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("pytest==8.0.0\nrequests>=2\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "main.py"), []byte("print('ok')\n"), 0o644))

		_, err := EnsureCodebaseMapDocs(root)
		require.NoError(t, err)

		stack := readCodebaseMapDoc(t, root, "STACK.md")
		assert.Contains(t, stack, "Python")
		assert.Contains(t, stack, "Python project")
		assert.Contains(t, stack, "python -m pytest")
		assert.Contains(t, stack, "pytest")
		assert.Contains(t, stack, "requests")
	})
}

func TestEnsureCodebaseMapDocsRefreshesLegacyGeneratedGoSlipwayDocs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(`[package]
name = "lattice-demo"
version = "0.1.0"
`), 0o644))
	docDir := state.CodebaseMapDir(root)
	require.NoError(t, os.MkdirAll(docDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(docDir, "STACK.md"), []byte(`# Stack

- Languages: Go
- Frameworks and runtimes: Cobra-based CLI module github.com/signalridge/slipway
- Build and test tooling: go build ./...; go test ./...
- Key dependencies: github.com/spf13/cobra
- Notes: go.mod declares Go not detected
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(docDir, "ARCHITECTURE.md"), []byte(`# Architecture

- Module responsibilities: cmd/ owns CLI surfaces; internal/state owns change authority and filesystem layout; internal/engine owns progression, governance, artifact, and gate logic.
- Dependency flow: CLI commands assemble model state and delegate durable state changes to internal/state and workflow decisions to internal/engine.
- Coupling hotspots: lifecycle progression, artifact readiness, worktree binding, and archive migration share change.yaml path authority.
- Current change blast radius: governed workflow creation, codebase-map context, and done/archive reporting.
- Notes: Baseline was generated from repository layout and known Slipway package boundaries.
`), 0o644))

	_, err := EnsureCodebaseMapDocs(root)
	require.NoError(t, err)

	stack := readCodebaseMapDoc(t, root, "STACK.md")
	assert.Contains(t, stack, "- Languages: Rust")
	assert.NotContains(t, stack, "Languages: Go")
	assert.NotContains(t, stack, "go build ./...")

	architecture := readCodebaseMapDoc(t, root, "ARCHITECTURE.md")
	assert.True(t, CodebaseMapDocIsScaffoldOnly("ARCHITECTURE.md", architecture))
	assert.NotContains(t, architecture, "cmd/ owns CLI surfaces")
}

func TestEnsureCodebaseMapDocsPreservesAuthoredAnalysis(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[workspace]\nmembers = [\"crates/core\"]\n"), 0o644))
	docDir := state.CodebaseMapDir(root)
	require.NoError(t, os.MkdirAll(docDir, 0o755))
	authored := "# Architecture\n\n- Module responsibilities: crates/core owns the domain model.\n"
	require.NoError(t, os.WriteFile(filepath.Join(docDir, "ARCHITECTURE.md"), []byte(authored), 0o644))

	_, err := EnsureCodebaseMapDocs(root)
	require.NoError(t, err)

	assert.Equal(t, normalizeCodebaseMapDoc(authored), normalizeCodebaseMapDoc(readCodebaseMapDoc(t, root, "ARCHITECTURE.md")))
	assessment, err := AssessCodebaseMapDocs(root)
	require.NoError(t, err)
	assert.Contains(t, assessment.PopulatedDocs, "ARCHITECTURE.md")
}

func TestEnsureCodebaseMapDocsPreservesLegacyDocWithAuthoredSupplement(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[workspace]\nmembers = []\n"), 0o644))
	docDir := state.CodebaseMapDir(root)
	require.NoError(t, os.MkdirAll(docDir, 0o755))
	authored := `# Architecture

- Module responsibilities: cmd/ owns CLI surfaces; internal/state owns change authority and filesystem layout; internal/engine owns progression, governance, artifact, and gate logic.
- Dependency flow: CLI commands assemble model state and delegate durable state changes to internal/state and workflow decisions to internal/engine.
- Coupling hotspots: lifecycle progression, artifact readiness, worktree binding, and archive migration share change.yaml path authority.
- Current change blast radius: governed workflow creation, codebase-map context, and done/archive reporting.
- Notes: Baseline was generated from repository layout and known Slipway package boundaries.
- Project-specific finding: crates/core owns the domain model.
`
	require.NoError(t, os.WriteFile(filepath.Join(docDir, "ARCHITECTURE.md"), []byte(authored), 0o644))

	_, err := EnsureCodebaseMapDocs(root)
	require.NoError(t, err)

	assert.Equal(t, normalizeCodebaseMapDoc(authored), normalizeCodebaseMapDoc(readCodebaseMapDoc(t, root, "ARCHITECTURE.md")))
	assessment, err := AssessCodebaseMapDocs(root)
	require.NoError(t, err)
	assert.Contains(t, assessment.PopulatedDocs, "ARCHITECTURE.md")
}

func readCodebaseMapDoc(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(state.CodebaseMapDir(root), name))
	require.NoError(t, err)
	return string(data)
}
