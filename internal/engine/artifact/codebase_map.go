package artifact

import (
	"encoding/json"
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
	CodebaseMapStatusBaseline     = "baseline"
	CodebaseMapStatusPopulated    = "populated"
)

type CodebaseMapAssessment struct {
	Status           string            `json:"status"`
	DocStates        map[string]string `json:"doc_states,omitempty"`
	MissingDocs      []string          `json:"missing_docs,omitempty"`
	ScaffoldOnlyDocs []string          `json:"scaffold_only_docs,omitempty"`
	BaselineDocs     []string          `json:"baseline_docs,omitempty"`
	PopulatedDocs    []string          `json:"populated_docs,omitempty"`
}

func CodebaseMapDisplayDocs(displayRoot, codebaseMapDir string) map[string]string {
	docs := make(map[string]string, len(codebaseMapDocNames))
	for _, name := range codebaseMapDocNames {
		docs[codebaseMapDocKeys[name]] = state.DisplayPath(displayRoot, filepath.Join(codebaseMapDir, name))
	}
	return docs
}

// CodebaseMapInstructionKeys returns the sorted public doc keys (e.g. "stack",
// "architecture") so `slipway instructions <key>` can route codebase-map docs
// through the same authoring contract as the governed bundle artifacts. These
// docs are repo-scoped and advisory — they do not gate any stage.
func CodebaseMapInstructionKeys() []string {
	keys := make([]string, 0, len(codebaseMapDocKeys))
	for _, key := range codebaseMapDocKeys {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// CodebaseMapDocInstruction resolves a codebase-map doc by public key
// ("stack") or file name ("STACK.md", case-insensitive) and returns its file
// name and static template. ok is false when nameOrKey matches no codebase-map
// doc, letting the caller fall through to the bundle-artifact path.
func CodebaseMapDocInstruction(nameOrKey string) (file string, template string, ok bool) {
	name := codebaseMapDocFileName(nameOrKey)
	if name == "" {
		return "", "", false
	}
	tmpl, ok := codebaseMapDocTemplates[name]
	if !ok {
		return "", "", false
	}
	return name, tmpl, true
}

// CodebaseMapDocKey returns the public key ("stack") for a codebase-map doc
// given its key or file name. It returns the normalized input when the doc is
// unknown so callers always get a stable label.
func CodebaseMapDocKey(nameOrKey string) string {
	if name := codebaseMapDocFileName(nameOrKey); name != "" {
		return codebaseMapDocKeys[name]
	}
	return strings.TrimSpace(nameOrKey)
}

// CodebaseMapBaselineDoc returns the machine-extracted baseline content for a
// codebase-map doc (resolved by key or file name) computed from the workspace
// manifests under root. These are real detected facts, not placeholder seed, so
// `slipway instructions` can offer them as background the author preserves and
// extends. ok is false when nameOrKey matches no codebase-map doc.
func CodebaseMapBaselineDoc(root, nameOrKey string) (content string, ok bool) {
	name := codebaseMapDocFileName(nameOrKey)
	if name == "" {
		return "", false
	}
	if _, exists := codebaseMapDocTemplates[name]; !exists {
		return "", false
	}
	return renderCodebaseMapBaselineDoc(inspectCodebaseMapFacts(root), name), true
}

// codebaseMapDocFileName normalizes a public key ("stack") or file name
// ("stack.md", "STACK.md") to the canonical codebase-map file name ("STACK.md"),
// or "" when it matches no codebase-map doc.
func codebaseMapDocFileName(nameOrKey string) string {
	value := strings.TrimSpace(nameOrKey)
	if value == "" {
		return ""
	}
	if _, ok := codebaseMapDocTemplates[value]; ok {
		return value
	}
	lower := strings.ToLower(value)
	for _, name := range codebaseMapDocNames {
		if strings.ToLower(name) == lower || codebaseMapDocKeys[name] == lower {
			return name
		}
	}
	return ""
}

func EnsureCodebaseMapDocs(root string) (created []string, err error) {
	dir := state.CodebaseMapDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	baselines := codebaseMapBaselines(root)

	for _, name := range codebaseMapDocNames {
		path := filepath.Join(dir, name)
		if _, ok := codebaseMapDocTemplates[name]; !ok {
			return nil, fmt.Errorf("missing codebase map template for %s", name)
		}
		baseline := baselines[name]
		if data, err := os.ReadFile(path); err == nil {
			if CodebaseMapDocIsScaffoldOnly(name, string(data)) || codebaseMapDocIsLegacyGenerated(name, string(data)) {
				if err := os.WriteFile(path, []byte(baseline), 0o644); err != nil {
					return nil, err
				}
			}
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		if err := os.WriteFile(path, []byte(baseline), 0o644); err != nil {
			return nil, err
		}
		created = append(created, path)
	}
	return created, nil
}

type codebaseMapFacts struct {
	Languages        []string
	Frameworks       []string
	BuildTestTooling []string
	KeyDependencies  []string
	Notes            []string
	TopDirs          []string
	EntryPoints      []string
	TestLayout       []string
}

func inspectCodebaseMapFacts(root string) codebaseMapFacts {
	facts := codebaseMapFacts{}
	recordRootLayoutFacts(root, &facts)
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		inspectGoModFacts(string(data), &facts)
	}
	if data, err := os.ReadFile(filepath.Join(root, "Cargo.toml")); err == nil {
		inspectCargoFacts(string(data), &facts)
	}
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		inspectPackageJSONFacts(root, data, &facts)
	} else if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
		facts.addLanguage("TypeScript")
		facts.addFramework("TypeScript project")
	}
	inspectPythonFacts(root, &facts)
	inspectJvmFacts(root, &facts)
	inspectOtherManifestFacts(root, &facts)
	inspectSourceExtensionFacts(root, &facts)
	facts.sortAndCompact()
	return facts
}

func inspectGoModFacts(content string, facts *codebaseMapFacts) {
	facts.addLanguage("Go")
	module := ""
	goVersion := ""
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "module "):
			module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		case strings.HasPrefix(line, "go "):
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		case strings.HasPrefix(line, "require "):
			fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "require ")))
			if len(fields) > 0 && fields[0] != "(" {
				facts.addDependency(fields[0])
			}
		case strings.HasPrefix(line, "github.com/") || strings.HasPrefix(line, "golang.org/") || strings.HasPrefix(line, "gopkg.in/"):
			fields := strings.Fields(line)
			if len(fields) > 0 {
				facts.addDependency(fields[0])
			}
		}
	}
	if module != "" {
		facts.addFramework("Go module " + module)
	} else {
		facts.addFramework("Go module")
	}
	facts.addBuildTestTooling("go build ./...; go test ./...")
	if goVersion != "" {
		facts.addNote("go.mod declares Go " + goVersion)
	}
}

func inspectCargoFacts(content string, facts *codebaseMapFacts) {
	facts.addLanguage("Rust")
	if strings.Contains(content, "[workspace]") {
		facts.addFramework("Cargo workspace")
	} else if name := firstTOMLStringValue(content, "name"); name != "" {
		facts.addFramework("Cargo crate " + name)
	} else {
		facts.addFramework("Cargo project")
	}
	facts.addBuildTestTooling("cargo build --workspace; cargo test --workspace")
	facts.addNote("Cargo.toml detected")
	for _, dep := range firstTOMLSectionKeys(content, "dependencies", 8) {
		facts.addDependency(dep)
	}
}

func inspectPackageJSONFacts(root string, data []byte, facts *codebaseMapFacts) {
	facts.addLanguage("JavaScript")
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
		facts.addLanguage("TypeScript")
	}
	var pkg struct {
		Name            string            `json:"name"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]any    `json:"dependencies"`
		DevDependencies map[string]any    `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		facts.addFramework("Node.js package")
		return
	}
	if strings.TrimSpace(pkg.Name) != "" {
		facts.addFramework("Node.js package " + strings.TrimSpace(pkg.Name))
	} else {
		facts.addFramework("Node.js package")
	}
	if _, ok := pkg.DevDependencies["typescript"]; ok {
		facts.addLanguage("TypeScript")
	}
	commands := []string{}
	if _, ok := pkg.Scripts["build"]; ok {
		commands = append(commands, "npm run build")
	}
	if _, ok := pkg.Scripts["test"]; ok {
		commands = append(commands, "npm test")
	}
	if len(commands) > 0 {
		facts.addBuildTestTooling(strings.Join(commands, "; "))
	}
	for dep := range pkg.Dependencies {
		facts.addDependency(dep)
	}
	for dep := range pkg.DevDependencies {
		facts.addDependency(dep)
	}
}

func inspectPythonFacts(root string, facts *codebaseMapFacts) {
	pyproject := filepath.Join(root, "pyproject.toml")
	setupPy := filepath.Join(root, "setup.py")
	requirements := filepath.Join(root, "requirements.txt")
	hasPythonManifest := false
	if _, err := os.Stat(pyproject); err == nil {
		hasPythonManifest = true
		facts.addEntryPoint("pyproject.toml")
	}
	if _, err := os.Stat(setupPy); err == nil {
		hasPythonManifest = true
		facts.addEntryPoint("setup.py")
	}
	if data, err := os.ReadFile(requirements); err == nil {
		hasPythonManifest = true
		facts.addEntryPoint("requirements.txt")
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
				continue
			}
			dep := strings.FieldsFunc(line, func(r rune) bool {
				return r == '=' || r == '<' || r == '>' || r == '~' || r == '!' || r == '['
			})
			if len(dep) > 0 {
				facts.addDependency(dep[0])
			}
		}
	}
	if hasPythonManifest {
		facts.addLanguage("Python")
		facts.addFramework("Python project")
		facts.addBuildTestTooling("python -m pytest")
	}
}

func inspectJvmFacts(root string, facts *codebaseMapFacts) {
	if _, err := os.Stat(filepath.Join(root, "pom.xml")); err == nil {
		facts.addLanguage("Java")
		facts.addFramework("Maven project")
		facts.addBuildTestTooling("mvn test")
	}
	if _, err := os.Stat(filepath.Join(root, "build.gradle")); err == nil {
		facts.addLanguage("Java")
		facts.addFramework("Gradle project")
		facts.addBuildTestTooling("./gradlew test")
	}
	if _, err := os.Stat(filepath.Join(root, "build.gradle.kts")); err == nil {
		facts.addLanguage("Kotlin")
		facts.addFramework("Gradle Kotlin project")
		facts.addBuildTestTooling("./gradlew test")
	}
}

func inspectOtherManifestFacts(root string, facts *codebaseMapFacts) {
	if _, err := os.Stat(filepath.Join(root, "Gemfile")); err == nil {
		facts.addLanguage("Ruby")
		facts.addFramework("Bundler project")
		facts.addBuildTestTooling("bundle exec rspec")
	}
	if _, err := os.Stat(filepath.Join(root, "composer.json")); err == nil {
		facts.addLanguage("PHP")
		facts.addFramework("Composer project")
		facts.addBuildTestTooling("composer test")
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "*.csproj")); len(matches) > 0 {
		facts.addLanguage("C#")
		facts.addFramework(".NET project")
		facts.addBuildTestTooling("dotnet test")
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "*.sln")); len(matches) > 0 {
		facts.addLanguage("C#")
		facts.addFramework(".NET solution")
		facts.addBuildTestTooling("dotnet test")
	}
}

func recordRootLayoutFacts(root string, facts *codebaseMapFacts) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if shouldSkipCodebaseMapDir(name) {
			continue
		}
		if entry.IsDir() {
			facts.addTopDir(name + "/")
			continue
		}
		if isCodebaseMapEntryPoint(name) {
			facts.addEntryPoint(name)
		}
	}
}

func inspectSourceExtensionFacts(root string, facts *codebaseMapFacts) {
	extensionLanguages := map[string]string{
		".go":   "Go",
		".rs":   "Rust",
		".js":   "JavaScript",
		".jsx":  "JavaScript",
		".ts":   "TypeScript",
		".tsx":  "TypeScript",
		".py":   "Python",
		".java": "Java",
		".kt":   "Kotlin",
		".kts":  "Kotlin",
		".rb":   "Ruby",
		".php":  "PHP",
		".cs":   "C#",
	}
	scanned := 0
	const scanLimit = 500
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && shouldSkipCodebaseMapDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if scanned >= scanLimit {
			return filepath.SkipAll
		}
		scanned++
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = entry.Name()
		}
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if language := extensionLanguages[ext]; language != "" {
			facts.addLanguage(language)
			if isSourceEntryPoint(rel) {
				facts.addEntryPoint(rel)
			}
		}
		switch {
		case strings.HasSuffix(entry.Name(), "_test.go"):
			facts.addTestLayout("Go *_test.go files")
		case strings.HasSuffix(entry.Name(), "_test.rs") || strings.HasSuffix(entry.Name(), "_spec.rs") || strings.HasPrefix(rel, "tests/"):
			facts.addTestLayout("Rust tests")
		case strings.HasSuffix(entry.Name(), ".test.js") || strings.HasSuffix(entry.Name(), ".test.ts") || strings.HasPrefix(rel, "test/") || strings.HasPrefix(rel, "tests/"):
			facts.addTestLayout("JavaScript/TypeScript tests")
		case strings.HasPrefix(entry.Name(), "test_") && strings.HasSuffix(entry.Name(), ".py"):
			facts.addTestLayout("Python tests")
		}
		return nil
	})
}

func codebaseMapBaselines(root string) map[string]string {
	facts := inspectCodebaseMapFacts(root)
	baselines := make(map[string]string, len(codebaseMapDocNames))
	for _, name := range codebaseMapDocNames {
		baselines[name] = renderCodebaseMapBaselineDoc(facts, name)
	}
	return baselines
}

func renderCodebaseMapBaselineDoc(facts codebaseMapFacts, name string) string {
	if !facts.hasDetectedFacts() {
		return codebaseMapDocTemplates[name]
	}
	switch name {
	case "STACK.md":
		return fmt.Sprintf(`# Stack

- Languages: %s
- Frameworks and runtimes: %s
- Build and test tooling: %s
- Key dependencies: %s
- Notes: %s
`, joinFacts(facts.Languages, 0), joinFacts(facts.Frameworks, 0), joinFacts(facts.BuildTestTooling, 0), joinFacts(facts.KeyDependencies, 6), joinFacts(facts.Notes, 0))
	case "STRUCTURE.md":
		return fmt.Sprintf(`# Structure

- Directory layout: %s
- Entry points: %s
- Generated versus handwritten boundaries:
- Ownership hints:
- Notes:
`, joinFacts(facts.TopDirs, 0), joinFacts(facts.EntryPoints, 0))
	case "TESTING.md":
		return fmt.Sprintf(`# Testing

- Test layout: %s
- Coverage hotspots:
- Coverage gaps:
- Verification commands: %s
- Fixture patterns:
- Notes:
`, joinFacts(facts.TestLayout, 0), joinFacts(facts.BuildTestTooling, 0))
	default:
		return codebaseMapDocTemplates[name]
	}
}

func (facts codebaseMapFacts) hasDetectedFacts() bool {
	return len(facts.Languages) > 0 ||
		len(facts.Frameworks) > 0 ||
		len(facts.BuildTestTooling) > 0 ||
		len(facts.KeyDependencies) > 0 ||
		len(facts.Notes) > 0 ||
		len(facts.TestLayout) > 0
}

func (facts *codebaseMapFacts) addLanguage(value string) {
	facts.Languages = appendFact(facts.Languages, value)
}

func (facts *codebaseMapFacts) addFramework(value string) {
	facts.Frameworks = appendFact(facts.Frameworks, value)
}

func (facts *codebaseMapFacts) addBuildTestTooling(value string) {
	facts.BuildTestTooling = appendFact(facts.BuildTestTooling, value)
}

func (facts *codebaseMapFacts) addDependency(value string) {
	facts.KeyDependencies = appendFact(facts.KeyDependencies, value)
}

func (facts *codebaseMapFacts) addNote(value string) {
	facts.Notes = appendFact(facts.Notes, value)
}

func (facts *codebaseMapFacts) addTopDir(value string) {
	facts.TopDirs = appendFact(facts.TopDirs, value)
}

func (facts *codebaseMapFacts) addEntryPoint(value string) {
	facts.EntryPoints = appendFact(facts.EntryPoints, value)
}

func (facts *codebaseMapFacts) addTestLayout(value string) {
	facts.TestLayout = appendFact(facts.TestLayout, value)
}

func (facts *codebaseMapFacts) sortAndCompact() {
	sortCompact := func(values []string) []string {
		slices.Sort(values)
		return slices.Compact(values)
	}
	facts.Languages = sortCompact(facts.Languages)
	facts.Frameworks = sortCompact(facts.Frameworks)
	facts.BuildTestTooling = sortCompact(facts.BuildTestTooling)
	facts.KeyDependencies = sortCompact(facts.KeyDependencies)
	facts.Notes = sortCompact(facts.Notes)
	facts.TopDirs = sortCompact(facts.TopDirs)
	facts.EntryPoints = sortCompact(facts.EntryPoints)
	facts.TestLayout = sortCompact(facts.TestLayout)
}

func appendFact(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	return append(values, value)
}

func joinFacts(values []string, max int) string {
	if len(values) == 0 {
		return ""
	}
	if max > 0 && len(values) > max {
		values = values[:max]
	}
	return strings.Join(values, ", ")
}

func firstTOMLStringValue(content, key string) string {
	prefix := key + " = "
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		return strings.TrimSpace(strings.Trim(value, `"'`))
	}
	return ""
}

func firstTOMLSectionKeys(content, section string, max int) []string {
	keys := []string{}
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.Trim(line, "[]") == section
			continue
		}
		if !inSection {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.Trim(key, `"'`))
		if key != "" {
			keys = append(keys, key)
		}
		if max > 0 && len(keys) >= max {
			break
		}
	}
	return keys
}

func shouldSkipCodebaseMapDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "artifacts", "node_modules", "target", "dist", "build", "vendor", "__pycache__":
		return true
	default:
		return false
	}
}

func isCodebaseMapEntryPoint(name string) bool {
	switch name {
	case "go.mod", "Cargo.toml", "package.json", "tsconfig.json", "pyproject.toml", "setup.py",
		"requirements.txt", "pom.xml", "build.gradle", "build.gradle.kts", "Gemfile", "composer.json",
		"main.go", "README.md":
		return true
	default:
		return strings.HasSuffix(name, ".csproj") || strings.HasSuffix(name, ".sln")
	}
}

func isSourceEntryPoint(path string) bool {
	switch path {
	case "main.go", "src/main.rs", "src/lib.rs", "src/index.js", "src/index.ts", "index.js", "index.ts", "main.py":
		return true
	default:
		return false
	}
}

func AssessCodebaseMapDocs(root string) (CodebaseMapAssessment, error) {
	dir := state.CodebaseMapDir(root)
	assessment := CodebaseMapAssessment{
		Status:    CodebaseMapStatusMissing,
		DocStates: map[string]string{},
	}
	baselines := codebaseMapBaselines(root)

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
		if normalizeCodebaseMapDoc(string(data)) == normalizeCodebaseMapDoc(baselines[name]) {
			assessment.DocStates[key] = CodebaseMapStatusBaseline
			assessment.BaselineDocs = append(assessment.BaselineDocs, name)
			continue
		}
		assessment.DocStates[key] = CodebaseMapStatusPopulated
		assessment.PopulatedDocs = append(assessment.PopulatedDocs, name)
	}

	slices.Sort(assessment.MissingDocs)
	slices.Sort(assessment.ScaffoldOnlyDocs)
	slices.Sort(assessment.BaselineDocs)
	slices.Sort(assessment.PopulatedDocs)

	switch {
	case len(assessment.PopulatedDocs) == len(codebaseMapDocNames):
		assessment.Status = CodebaseMapStatusPopulated
	case len(assessment.PopulatedDocs) == 0 && len(assessment.BaselineDocs) > 0 && len(assessment.MissingDocs) == 0:
		assessment.Status = CodebaseMapStatusBaseline
	case len(assessment.PopulatedDocs) == 0 && len(assessment.BaselineDocs) == 0 && len(assessment.ScaffoldOnlyDocs) > 0 && len(assessment.MissingDocs) == 0:
		assessment.Status = CodebaseMapStatusScaffoldOnly
	case len(assessment.PopulatedDocs) == 0 && len(assessment.BaselineDocs) == 0 && len(assessment.ScaffoldOnlyDocs) == 0:
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

func codebaseMapDocIsLegacyGenerated(name, content string) bool {
	lines := substantiveCodebaseMapLines(content)
	switch name {
	case "STACK.md":
		return len(lines) == 6 &&
			lines[0] == "# Stack" &&
			lines[1] == "- Languages: Go" &&
			strings.HasPrefix(lines[2], "- Frameworks and runtimes: Cobra-based CLI module ") &&
			lines[3] == "- Build and test tooling: go build ./...; go test ./..." &&
			strings.HasPrefix(lines[4], "- Key dependencies: ") &&
			strings.HasPrefix(lines[5], "- Notes: go.mod declares Go ")
	case "INTEGRATIONS.md":
		return slices.Equal(lines, []string{
			"# Integrations",
			"- External APIs: Git CLI is used for repository and worktree inspection.",
			"- Infrastructure bindings: Local filesystem state under artifacts/, .slipway.yaml, and git-local runtime directories.",
			"- Datastores and queues: No service datastore detected by baseline scan; Slipway stores YAML, JSON, JSONL, and Markdown artifacts on disk.",
			"- File formats and protocols: YAML change authority, JSON CLI output, JSONL lifecycle events, Markdown governed artifacts.",
			"- Notes: Integration inventory is deterministic baseline context; refine with project-specific external services when present.",
		})
	case "ARCHITECTURE.md":
		return slices.Equal(lines, []string{
			"# Architecture",
			"- Module responsibilities: cmd/ owns CLI surfaces; internal/state owns change authority and filesystem layout; internal/engine owns progression, governance, artifact, and gate logic.",
			"- Dependency flow: CLI commands assemble model state and delegate durable state changes to internal/state and workflow decisions to internal/engine.",
			"- Coupling hotspots: lifecycle progression, artifact readiness, worktree binding, and archive migration share change.yaml path authority.",
			"- Current change blast radius: governed workflow creation, codebase-map context, and done/archive reporting.",
			"- Notes: Baseline was generated from repository layout and known Slipway package boundaries.",
		})
	case "STRUCTURE.md":
		return len(lines) == 6 &&
			lines[0] == "# Structure" &&
			strings.HasPrefix(lines[1], "- Directory layout: ") &&
			strings.HasPrefix(lines[2], "- Entry points: ") &&
			lines[3] == "- Generated versus handwritten boundaries: internal/tmpl contains generated prompt/skill surfaces; cmd/ and internal/ contain handwritten Go runtime code." &&
			lines[4] == "- Ownership hints: Tests are colocated as *_test.go files under cmd/ and internal/." &&
			(lines[5] == "- Notes: Go tests not detected by baseline scan." ||
				lines[5] == "- Notes: Go *_test.go files are present.")
	case "CONVENTIONS.md":
		return slices.Equal(lines, []string{
			"# Conventions",
			"- Naming: CLI commands live in cmd/ with make<Command>Cmd constructors; workflow states and durable schemas live in internal/model.",
			"- File organization: Runtime state helpers belong under internal/state; progression decisions belong under internal/engine/progression.",
			"- Error handling: CLI-facing failures use structured reason codes and typed CLI errors where user remediation matters.",
			"- Configuration: .slipway.yaml is the project-local governance configuration authority.",
			"- State management: change.yaml is current-state authority; lifecycle.jsonl is append-only audit evidence.",
			"- Notes: Generated host-skill templates should stay synchronized with runtime contracts.",
		})
	case "TESTING.md":
		return slices.Equal(lines, []string{
			"# Testing",
			"- Test layout: cmd/*_test.go covers CLI contracts; internal/**/*_test.go covers state, artifact, progression, governance, and template behavior.",
			"- Coverage hotspots: next/run/status JSON contracts, governed lifecycle gates, archive migration, worktree binding, and generated skill/template drift.",
			"- Coverage gaps: End-to-end governed workflow tests are intentionally heavier and should use explicit timeouts.",
			"- Verification commands: go test -timeout=20m ./... -count=1; go build ./...",
			"- Fixture patterns: Tests commonly create temp workspaces, seed governed bundles, write verification YAML, and assert JSON command output.",
			"- Notes: Prefer focused regression tests before full-suite verification.",
		})
	case "CONCERNS.md":
		return slices.Equal(lines, []string{
			"# Concerns",
			"- Architectural pressure points: Worktree binding must happen before canonical governed artifacts are treated as reviewed execution inputs.",
			"- Brittle areas: Scaffold-only codebase maps, planning-vs-execution evidence freshness, and archive relocation can create misleading authority signals.",
			"- Migration traps: Changing artifact roots must preserve repairability, archive discoverability, and change.yaml as the single current-state authority.",
			"- Recheck routing: Planning artifacts invalidate planning evidence; task execution drift invalidates execution evidence; assurance-only edits stay in verification/closeout checks.",
			"- Notes: Treat placeholder files as advisory until populated with concrete repository facts.",
		})
	default:
		return false
	}
}

func substantiveCodebaseMapLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := []string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
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
