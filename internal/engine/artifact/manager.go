package artifact

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/tmpl"
	"gopkg.in/yaml.v3"
)

// ErrUnknownArtifact is returned when an artifact name is not found in the stale graph.
var ErrUnknownArtifact = errors.New("unknown artifact")

// ArtifactSpec defines a single artifact in a schema, including its
// dependency relationships and discovery requirements.
type ArtifactSpec struct {
	Name              string   `json:"name" yaml:"name"`
	Template          string   `json:"template,omitempty" yaml:"template,omitempty"`
	DependsOn         []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	RequiredSections  []string `json:"required_sections,omitempty" yaml:"required_sections,omitempty"`
	RequiresDiscovery bool     `json:"requires_discovery,omitempty" yaml:"requires_discovery,omitempty"`
}

//go:embed schemas.yaml
var schemasYAML []byte

type schemasFile struct {
	Schemas map[string][]ArtifactSpec `yaml:"schemas"`
}

var loadedSchemas = func() schemasFile {
	var sf schemasFile
	if err := yaml.Unmarshal(schemasYAML, &sf); err != nil {
		panic(fmt.Sprintf("artifact: failed to parse schemas.yaml: %v", err))
	}
	return sf
}()

var coreSchema = loadedSchemas.Schemas["core"]
var expandedSchema = loadedSchemas.Schemas["expanded"]

// ResolveSchema returns the artifact specs for the given schema name.
// For "custom", the provided customDefs are converted to ArtifactSpec entries.
func ResolveSchema(name model.ArtifactSchemaName, customDefs []model.ArtifactDefinition) []ArtifactSpec {
	switch name {
	case model.ArtifactSchemaCore:
		return append([]ArtifactSpec(nil), coreSchema...)
	case model.ArtifactSchemaCustom:
		specs := make([]ArtifactSpec, 0, len(customDefs))
		for _, def := range customDefs {
			specs = append(specs, ArtifactSpec{
				Name:              def.Name,
				Template:          def.Template,
				DependsOn:         append([]string(nil), def.DependsOn...),
				RequiredSections:  nil,
				RequiresDiscovery: def.RequiresDiscovery,
			})
		}
		return specs
	default:
		// expanded is the default
		return append([]ArtifactSpec(nil), expandedSchema...)
	}
}

// RequiredArtifactsForChange returns the artifact names required for a change.
// Discovery-only artifacts are included when needsDiscovery is true.
// Light preset excludes assurance.md.
func RequiredArtifactsForChange(schema []ArtifactSpec, needsDiscovery bool, preset ...model.WorkflowPreset) []string {
	resolvedPreset := model.WorkflowPreset("")
	for _, candidate := range preset {
		if candidate.IsValid() {
			resolvedPreset = candidate
		}
	}
	isLight := resolvedPreset == model.WorkflowPresetLight
	names := make([]string, 0, len(schema))
	for _, spec := range schema {
		if spec.RequiresDiscovery && !needsDiscovery {
			continue
		}
		if isLight && spec.Name == "assurance.md" {
			continue
		}
		names = append(names, spec.Name)
	}
	return names
}

// ResolveArtifactPath maps a logical artifact name to its on-disk path inside
// a governed bundle directory (artifacts/changes/<slug>). All artifacts are
// stored flat in the bundle root.
func ResolveArtifactPath(baseDir, slug, artifactName string) string {
	return filepath.Join(baseDir, strings.TrimSpace(artifactName))
}

// BuildStaleGraph builds a stale propagation graph from a schema.
// When artifact A depends on artifact B, modifying B should mark A as stale.
// The returned graph maps each artifact to the list of artifacts that become
// stale when it is modified (i.e., its downstream dependents).
func BuildStaleGraph(schema []ArtifactSpec) map[string][]string {
	// First, collect all artifact names and build a "depended-on-by" map.
	graph := make(map[string][]string)
	for _, spec := range schema {
		if _, ok := graph[spec.Name]; !ok {
			graph[spec.Name] = nil
		}
	}

	for _, spec := range schema {
		for _, dep := range spec.DependsOn {
			graph[dep] = append(graph[dep], spec.Name)
		}
	}

	// Sort each entry for deterministic output.
	for k := range graph {
		slices.Sort(graph[k])
	}
	return graph
}

// DefaultStaleGraph returns the stale graph for the expanded schema.
func DefaultStaleGraph() map[string][]string {
	return BuildStaleGraph(expandedSchema)
}

func requiredSectionsForArtifact(name string) []string {
	for _, schema := range [][]ArtifactSpec{expandedSchema, coreSchema} {
		for _, spec := range schema {
			if spec.Name != name || len(spec.RequiredSections) == 0 {
				continue
			}
			return append([]string(nil), spec.RequiredSections...)
		}
	}
	return nil
}

func TemplateContent(name string) (string, error) {
	return tmpl.Content(filepath.Join("artifacts", name))
}

type templateData struct {
	Slug               string
	InitialRequest     string
	QualityMode        string
	BundleRoot         string
	CodebaseMapRoot    string
	BundleArchiveRoot  string
	ProjectTechStack   string
	ProjectConventions string
	ProjectTestCmd     string
	ProjectBuildCmd    string
	ProjectLanguages   string
	ComplexityLevel    string
	ComplexityRank     int    // 0=trivial, 1=simple, 2=complex, 3=critical
	GuardrailDomain    string // Inferred guardrail domain (e.g. auth_authz, security_credentials)

	// Seeded draft content — computed from InitialRequest, project context,
	// and optionally from --from-doc extraction.
	SeededRequirements        string
	SeededDecision            string
	SeededDecisionApproach    string
	SeededDecisionInterfaces  string
	SeededDecisionRollback    string
	SeededDecisionRisk        string
	SeededResearch            string
	SeededResearchUnknowns    string
	SeededResearchAssumptions string
	SeededResearchReferences  string
	SeededTasks               string
}

// DocSections carries extracted document sections across the cmd → artifact
// package boundary when --from-doc is used.
type DocSections struct {
	Scope       string
	Constraints string
	Acceptance  string
}

func loadProjectContext(root string) model.ProjectContext {
	projectCtx := model.ProjectContext{}
	if cfg, err := model.LoadConfig(state.ConfigPath(root)); err == nil {
		projectCtx = cfg.Context
	}
	return projectCtx
}

func buildTemplateData(root string, change model.Change, docs *DocSections, overrideCtx ...model.ProjectContext) templateData {
	var projectCtx model.ProjectContext
	if len(overrideCtx) > 0 {
		projectCtx = overrideCtx[0]
	} else {
		projectCtx = loadProjectContext(root)
	}
	projectLanguages := strings.Join(projectCtx.Languages, ", ")
	slug := strings.TrimSpace(change.Slug)
	data := templateData{
		Slug:               slug,
		InitialRequest:     strings.TrimSpace(change.Description),
		QualityMode:        string(change.EffectiveQualityMode()),
		BundleRoot:         filepath.ToSlash(filepath.Join("artifacts", "changes", slug)),
		CodebaseMapRoot:    filepath.ToSlash(filepath.Join("artifacts", "codebase")),
		BundleArchiveRoot:  filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug)),
		ProjectTechStack:   strings.TrimSpace(projectCtx.TechStack),
		ProjectConventions: strings.TrimSpace(projectCtx.Conventions),
		ProjectTestCmd:     strings.TrimSpace(projectCtx.TestCmd),
		ProjectBuildCmd:    strings.TrimSpace(projectCtx.BuildCmd),
		ProjectLanguages:   strings.TrimSpace(projectLanguages),
		ComplexityLevel:    change.ComplexityLevel,
		ComplexityRank:     complexityRank(change.ComplexityLevel),
		GuardrailDomain:    change.GuardrailDomain,
	}
	data.SeededRequirements = seedRequirements(data)
	data.SeededDecision = seedDecision(data)
	data.SeededDecisionApproach = seedDecisionApproach(data)
	data.SeededDecisionInterfaces = seedDecisionInterfaces(data)
	data.SeededDecisionRollback = seedDecisionRollback(data)
	data.SeededDecisionRisk = seedDecisionRisk(data)
	data.SeededResearch = seedResearch(data)
	data.SeededResearchUnknowns = seedResearchUnknowns(data)
	data.SeededResearchAssumptions = seedResearchAssumptions(data)
	data.SeededResearchReferences = seedResearchReferences(data)
	data.SeededTasks = seedTasks(data)

	if docs == nil {
		return data
	}
	if docs.Scope != "" {
		data.SeededRequirements = seededRequirementsContent(data, docSectionItems(docs.Scope))
	}
	if docs.Scope != "" || docs.Acceptance != "" {
		data.SeededTasks = seedTasksFromDoc(data, *docs)
	}
	if docs.Constraints != "" {
		data.SeededDecision += "\n### Constraints (from source document)\n" + docs.Constraints + "\n"
		constraintItems := constraintSeedItems(docs.Constraints)
		data.SeededDecisionApproach = appendDocConstraints(
			data.SeededDecisionApproach,
			constraintItems,
			"This direction must continue honoring the documented constraints:",
		)
		data.SeededDecisionInterfaces = appendDocConstraints(
			data.SeededDecisionInterfaces,
			constraintItems,
			"Interface and data-flow changes must respect these documented constraints:",
		)
		data.SeededDecisionRollback = appendDocConstraints(
			data.SeededDecisionRollback,
			constraintItems,
			"Rollback planning must preserve these documented constraints:",
		)
		data.SeededDecisionRisk = appendDocConstraints(
			data.SeededDecisionRisk,
			constraintItems,
			"Constraint-driven risks to keep explicit during implementation:",
		)
		data.SeededResearch += "\n### Constraints (from source document)\n" + docs.Constraints + "\n"
		if len(constraintItems) > 0 {
			var unknowns strings.Builder
			unknowns.WriteString(data.SeededResearchUnknowns)
			for _, item := range constraintItems {
				fmt.Fprintf(&unknowns, "- How does the constraint %q limit the implementation of %s?\n", item, strings.ToLower(data.InitialRequest))
			}
			data.SeededResearchUnknowns = unknowns.String()

			var assumptions strings.Builder
			assumptions.WriteString(data.SeededResearchAssumptions)
			for _, item := range constraintItems {
				fmt.Fprintf(&assumptions, "- Assume the constraint %q remains binding until code inspection or stakeholder input proves otherwise.\n", item)
			}
			data.SeededResearchAssumptions = assumptions.String()
			data.SeededResearchReferences += "- The `--from-doc` source document, especially its Constraints section.\n"
		}
	}
	return data
}

// seedRequirements generates a first-pass requirements section from the change description.
func seedRequirements(data templateData) string {
	requirementItems := []string{strings.TrimSpace(data.InitialRequest)}
	return seededRequirementsContent(data, requirementItems)
}

// seedDecision generates a first-pass decision section from available context.
func seedDecision(data templateData) string {
	return "Pending investigation. Replace with concrete alternatives, tradeoffs, and the selected direction after research or code inspection.\n"
}

func seedDecisionApproach(data templateData) string {
	return "Pending investigation. Record the selected approach only after the alternatives have concrete evidence."
}

func seedDecisionInterfaces(data templateData) string {
	return "Pending investigation. Name changed interfaces and data flows, or write \"none\" after inspection."
}

func seedDecisionRollback(data templateData) string {
	return "Pending investigation. Write the concrete rollback path and verification command after implementation scope is known."
}

func seedDecisionRisk(data templateData) string {
	return "Pending investigation. List concrete risks only after inspecting the affected code and contracts."
}

// seedResearch generates a first-pass research section.
func seedResearch(data templateData) string {
	return "Pending investigation. Replace with concrete alternatives, supporting evidence, and the selected direction.\n"
}

func seedResearchUnknowns(data templateData) string {
	return "- Pending investigation. List unknowns that must be resolved before planning.\n"
}

func seedResearchAssumptions(data templateData) string {
	return "- Pending investigation. List assumptions only after identifying the evidence that supports them.\n"
}

func seedResearchReferences(data templateData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- `artifacts/changes/%s/intent.md` for the original request and intake context.\n", data.Slug)
	b.WriteString("- `requirements.md` and `decision.md` in the same bundle once planning artifacts are refined.\n")
	if strings.TrimSpace(data.ProjectTestCmd) != "" {
		fmt.Fprintf(&b, "- Existing verification flows referenced by the scaffold context, especially `%s`.\n", strings.TrimSpace(data.ProjectTestCmd))
	} else {
		b.WriteString("- Existing code paths and tests related to the affected behavior in the repository.\n")
	}
	return b.String()
}

// seedTasks generates a first-pass task list from the change description.
func seedTasks(data templateData) string {
	var b strings.Builder
	reqRefs := strings.Join(seededRequirementRefs(data, 0), ", ")
	b.WriteString("- [ ] `t-01` Pending task objective\n")
	b.WriteString("  - wave: 1\n")
	b.WriteString("  - depends_on: []\n")
	b.WriteString("  - target_files: []\n")
	b.WriteString("  - task_kind: investigation\n")
	fmt.Fprintf(&b, "  - covers: [%s]\n\n", reqRefs)
	return b.String()
}

// seedTasksFromDoc enriches tasks with --from-doc extracted scope items.
func seedTasksFromDoc(data templateData, docs DocSections) string {
	scopeItems := docSectionItems(docs.Scope)
	acceptanceItems := docSectionItems(docs.Acceptance)
	if len(scopeItems) == 0 && len(acceptanceItems) == 0 {
		return seedTasks(data)
	}

	var b strings.Builder
	taskNum := 1
	codeTaskIDs := make([]string, 0, len(scopeItems))
	reqRefs := seededRequirementRefs(data, len(scopeItems))
	for idx, line := range scopeItems {
		fmt.Fprintf(&b, "- [ ] `t-%02d` %s\n", taskNum, capitalizeFirst(line))
		b.WriteString("  - wave: 1\n")
		b.WriteString("  - depends_on: []\n")
		b.WriteString("  - target_files: []\n")
		b.WriteString("  - task_kind: code\n")
		fmt.Fprintf(&b, "  - covers: [%s]\n\n", reqRefs[idx])
		codeTaskIDs = append(codeTaskIDs, fmt.Sprintf("t-%02d", taskNum))
		taskNum++
	}

	if len(codeTaskIDs) == 0 {
		fmt.Fprintf(&b, "- [ ] `t-%02d` Implement %s\n", taskNum, strings.ToLower(data.InitialRequest))
		b.WriteString("  - wave: 1\n")
		b.WriteString("  - depends_on: []\n")
		b.WriteString("  - target_files: []\n")
		b.WriteString("  - task_kind: code\n")
		b.WriteString("  - covers: [REQ-001]\n\n")
		codeTaskIDs = append(codeTaskIDs, fmt.Sprintf("t-%02d", taskNum))
		taskNum++
	}

	if len(acceptanceItems) > 0 {
		deps := strings.Join(codeTaskIDs, ", ")
		for _, line := range acceptanceItems {
			fmt.Fprintf(&b, "- [ ] `t-%02d` %s\n", taskNum, capitalizeFirst(line))
			b.WriteString("  - wave: 2\n")
			fmt.Fprintf(&b, "  - depends_on: [%s]\n", deps)
			b.WriteString("  - target_files: []\n")
			b.WriteString("  - task_kind: verification\n")
			fmt.Fprintf(&b, "  - covers: [%s]\n\n", strings.Join(reqRefs, ", "))
			taskNum++
		}
		return b.String()
	}

	fmt.Fprintf(&b, "- [ ] `t-%02d` Pending verification objective\n", taskNum)
	b.WriteString("  - wave: 2\n")
	fmt.Fprintf(&b, "  - depends_on: [%s]\n", strings.Join(codeTaskIDs, ", "))
	b.WriteString("  - target_files: []\n")
	b.WriteString("  - task_kind: verification\n")
	fmt.Fprintf(&b, "  - covers: [%s]\n", strings.Join(reqRefs, ", "))
	return b.String()
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size == 1 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

func seededRequirementsContent(data templateData, requirementItems []string) string {
	items := make([]string, 0, len(requirementItems))
	for _, item := range requirementItems {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) == 0 {
		fallback := strings.TrimSpace(data.InitialRequest)
		if fallback == "" {
			fallback = "define requirements based on the initial request"
		}
		items = append(items, fallback)
	}

	var b strings.Builder
	reqNum := 1
	for _, item := range items {
		appendRequirementBlock(&b, reqNum, item)
		reqNum++
	}
	if data.GuardrailDomain != "" {
		appendGuardrailRequirementBlock(&b, reqNum, data.GuardrailDomain)
	}
	return b.String()
}

func appendRequirementBlock(b *strings.Builder, reqNum int, objective string) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	cleanedObjective := strings.Join(strings.Fields(strings.TrimSpace(objective)), " ")
	fmt.Fprintf(b, "### Requirement: %s\n", cleanedObjective)
	fmt.Fprintf(b, "REQ-%03d: %s\n", reqNum, requirementNormativeSentence(cleanedObjective))
	b.WriteString("\n#### Scenario: Primary flow\n")
	b.WriteString("GIVEN the relevant workflow is exercised\n")
	b.WriteString("WHEN the requirement is implemented in the target flow\n")
	fmt.Fprintf(b, "THEN the expected behavior for %s is observed.\n", strings.ToLower(cleanedObjective))
}

func appendGuardrailRequirementBlock(b *strings.Builder, reqNum int, guardrailDomain string) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "### Requirement: %s guardrail compliance\n", guardrailDomain)
	fmt.Fprintf(b, "REQ-%03d: The implementation MUST comply with %s guardrail requirements.\n", reqNum, guardrailDomain)
	b.WriteString("\n#### Scenario: Guardrail compliance\n")
	b.WriteString("GIVEN the change touches a guarded domain\n")
	b.WriteString("WHEN the implementation is updated\n")
	fmt.Fprintf(b, "THEN %s guardrail requirements remain satisfied.\n", guardrailDomain)
}

func requirementNormativeSentence(objective string) string {
	lowerObjective := strings.ToLower(strings.TrimSpace(objective))
	if lowerObjective == "" {
		return "The system MUST support the approved change intent."
	}
	if looksLikeActionPhrase(lowerObjective) {
		return "The system MUST " + lowerObjective + "."
	}
	return "The system MUST support the requested change described as: " + lowerObjective + "."
}

func looksLikeActionPhrase(objective string) bool {
	firstWord := objective
	if fields := strings.Fields(objective); len(fields) > 0 {
		firstWord = fields[0]
	}
	switch firstWord {
	case "add", "allow", "block", "create", "deny", "detect", "disable", "document",
		"enable", "enforce", "expire", "expose", "fix", "generate", "improve", "infer",
		"keep", "limit", "migrate", "parse", "preserve", "prevent", "reduce", "remove",
		"replace", "require", "reuse", "seed", "support", "track", "update", "use",
		"validate", "verify", "write":
		return true
	default:
		return false
	}
}

func docSectionItems(section string) []string {
	lines := strings.Split(section, "\n")
	if items := markdownListItems(lines); len(items) > 0 {
		return items
	}
	return proseParagraphItems(lines)
}

func markdownListItems(lines []string) []string {
	items := make([]string, 0)
	current := ""
	sawListMarker := false
	currentHasNestedItems := false

	flush := func() {
		if strings.TrimSpace(current) == "" {
			current = ""
			currentHasNestedItems = false
			return
		}
		items = append(items, strings.Join(strings.Fields(current), " "))
		current = ""
		currentHasNestedItems = false
	}

	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			if sawListMarker {
				flush()
			}
			continue
		}
		if item, ok := listItemText(trimmed); ok {
			if leadingIndentWidth(raw) == 0 {
				sawListMarker = true
				flush()
				current = item
				continue
			}
			if sawListMarker {
				if current == "" {
					current = item
				} else if !currentHasNestedItems {
					current += ": " + item
					currentHasNestedItems = true
				} else {
					current += "; " + item
				}
				continue
			}
		}
		if sawListMarker {
			if current == "" {
				current = trimmed
			} else {
				current += " " + trimmed
			}
		}
	}

	if !sawListMarker {
		return nil
	}
	flush()
	return items
}

func leadingIndentWidth(s string) int {
	width := 0
	for _, r := range s {
		if r != ' ' && r != '\t' {
			break
		}
		width++
	}
	return width
}

func proseParagraphItems(lines []string) []string {
	parts := make([]string, 0, len(lines))
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parts = append(parts, trimmed)
	}
	if len(parts) == 0 {
		return nil
	}
	return []string{strings.Join(parts, " ")}
}

func listItemText(line string) (string, bool) {
	for _, prefix := range []string{"- ", "* "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}

	idx := 0
	for idx < len(line) && line[idx] >= '0' && line[idx] <= '9' {
		idx++
	}
	if idx == 0 || idx+1 >= len(line) {
		return "", false
	}
	if (line[idx] != '.' && line[idx] != ')') || line[idx+1] != ' ' {
		return "", false
	}
	return strings.TrimSpace(line[idx+2:]), true
}

func seededRequirementRefs(data templateData, scopeItemCount int) []string {
	count := scopeItemCount
	if count == 0 {
		count = 1
	}
	if data.GuardrailDomain != "" {
		count++
	}
	return reqReferenceList(count)
}

func reqReferenceList(count int) []string {
	if count <= 0 {
		return []string{"REQ-001"}
	}
	refs := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		refs = append(refs, fmt.Sprintf("REQ-%03d", i))
	}
	return refs
}

func appendDocConstraints(base string, items []string, heading string) string {
	if len(items) == 0 {
		return base
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(base))
	b.WriteString("\n\n")
	b.WriteString(heading)
	b.WriteString("\n")
	for _, item := range items {
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func constraintSeedItems(section string) []string {
	items := docSectionItems(section)
	if strings.TrimSpace(section) == "" {
		return items
	}

	lines := strings.Split(section, "\n")
	preamble := make([]string, 0, len(lines))
	sawTopLevelList := false
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			if sawTopLevelList {
				break
			}
			continue
		}
		if _, ok := listItemText(trimmed); ok && leadingIndentWidth(raw) == 0 {
			sawTopLevelList = true
			break
		}
		if sawTopLevelList {
			break
		}
		preamble = append(preamble, trimmed)
	}
	if !sawTopLevelList {
		return items
	}

	preambleText := strings.Join(preamble, " ")
	if strings.TrimSpace(preambleText) == "" {
		return items
	}
	return append([]string{preambleText}, items...)
}

func ScaffoldGovernedBundleForChangeWithPreset(root string, change model.Change, preset model.WorkflowPreset, schema ...[]ArtifactSpec) error {
	return scaffoldGovernedBundleForChange(root, change, preset, nil, nil, schema...)
}

// ScaffoldIntentForChangeWithContext creates only intent.md in the governed
// bundle directory using an externally-provided ProjectContext. This is used
// when preset is pending to ensure the intake artifact exists without creating
// the full governed bundle.
func ScaffoldIntentForChangeWithContext(root string, change model.Change, projectCtx model.ProjectContext) error {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return fmt.Errorf("slug is required")
	}
	base, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	data := buildTemplateData(root, change, nil, projectCtx)
	intentPath := ResolveArtifactPath(base, slug, "intent.md")
	if _, err := os.Stat(intentPath); err == nil {
		return nil // already exists
	}
	rendered, err := renderTemplateWithFallback(root, "intent.md", "", data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(intentPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(intentPath, []byte(rendered), 0o644)
}

// ScaffoldGovernedBundleForChangeWithContext creates the artifact files using
// an externally-provided ProjectContext (e.g. from InferProjectContext) instead
// of loading from .slipway.yaml.
func ScaffoldGovernedBundleForChangeWithContext(root string, change model.Change, preset model.WorkflowPreset, projectCtx model.ProjectContext, schema ...[]ArtifactSpec) error {
	return scaffoldGovernedBundleForChange(root, change, preset, &projectCtx, nil, schema...)
}

// ScaffoldGovernedBundleForChangeWithContextAndDocs creates the artifact files
// using an externally-provided ProjectContext and doc sections extracted from
// --from-doc. The doc sections enrich seeded content in templates.
func ScaffoldGovernedBundleForChangeWithContextAndDocs(root string, change model.Change, preset model.WorkflowPreset, projectCtx model.ProjectContext, docs DocSections, schema ...[]ArtifactSpec) error {
	return scaffoldGovernedBundleForChange(root, change, preset, &projectCtx, &docs, schema...)
}

func scaffoldGovernedBundleForChange(root string, change model.Change, preset model.WorkflowPreset, projectCtx *model.ProjectContext, docs *DocSections, schema ...[]ArtifactSpec) error {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return fmt.Errorf("slug is required")
	}

	// Use provided schema or default to expanded.
	var specs []ArtifactSpec
	if len(schema) > 0 && schema[0] != nil {
		specs = schema[0]
	} else {
		specs = expandedSchema
	}

	presetArgs := []model.WorkflowPreset{change.WorkflowPreset}
	if preset.IsValid() {
		presetArgs = append(presetArgs, preset)
	}
	files := RequiredArtifactsForChange(specs, change.NeedsDiscovery, presetArgs...)
	slices.Sort(files)

	base, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	var data templateData
	if projectCtx != nil {
		data = buildTemplateData(root, change, docs, *projectCtx)
	} else {
		data = buildTemplateData(root, change, docs)
	}

	// Build a lookup from artifact name to template source path for custom schemas.
	templatePaths := map[string]string{}
	for _, spec := range specs {
		if spec.Template != "" {
			templatePaths[spec.Name] = spec.Template
		}
	}

	for _, file := range files {
		path := ResolveArtifactPath(base, slug, file)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		rendered, err := renderTemplateWithFallback(root, file, templatePaths[file], data)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func complexityRank(level string) int {
	switch level {
	case "trivial":
		return 0
	case "simple":
		return 1
	case "complex":
		return 2
	case "critical":
		return 3
	default:
		return 1 // default to simple
	}
}

// EnsureResearchArtifactForChange ensures research.md exists in the governed bundle.
func EnsureResearchArtifactForChange(root string, change model.Change) error {
	base, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}

	path := filepath.Join(base, "research.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	rendered, err := renderTemplateWithFallback(root, "research.md", "", buildTemplateData(root, change, nil))
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(rendered), 0o644)
}

// validateSectionStructure validates that content contains the given headings
// in order, each with at least one non-empty line of content beneath it.
func validateSectionStructure(content string, headings []string) (lines []string, indices []int, err error) {
	lines = strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	indices = make([]int, 0, len(headings))
	searchFrom := 0

	for _, heading := range headings {
		idx := -1
		for i := searchFrom; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == heading {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, nil, fmt.Errorf("missing required heading %q", heading)
		}
		indices = append(indices, idx)
		searchFrom = idx + 1
	}

	for i, heading := range headings {
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
			return nil, nil, fmt.Errorf("section %q must have non-empty content", heading)
		}
	}
	return lines, indices, nil
}

// ResearchStructureBlockers validates the research.md artifact structure.
func ResearchStructureBlockers(content string) []model.ReasonCode {
	headings := requiredSectionsForArtifact("research.md")
	if len(headings) == 0 {
		return nil // research.md may not be present for non-discovery changes
	}

	_, _, err := validateSectionStructure(content, headings)
	if err != nil {
		return []model.ReasonCode{model.NewReasonCode("research_structure_invalid", err.Error())}
	}
	return nil
}

// ParseDecisionLockedDecisions extracts decision items from decision.md.
// It reads "Selected Approach" as the primary decision, and "Alternatives Considered"
// for the selected direction marker. Ignores template placeholder text,
// including scaffolded seeded-draft defaults that have not been confirmed by
// research or explicit user selection yet.
func ParseDecisionLockedDecisions(content string) []string {
	var decisions []string

	// Extract selected direction from Alternatives Considered table/section
	_, selected := parseResearchAlternatives(markdownSectionLines(content, "Alternatives Considered"))
	if selected != "" && !LooksLikeTemplatePlaceholder(selected) {
		decisions = append(decisions, "Selected Direction: "+selected)
	}

	// Extract content from Selected Approach section as a locked decision
	approachLines := markdownSectionLines(content, "Selected Approach")
	approach := strings.TrimSpace(strings.Join(approachLines, "\n"))
	if approach != "" && !LooksLikeTemplatePlaceholder(approach) {
		decisions = append(decisions, "Selected Approach: "+approach)
	}

	if len(decisions) == 0 {
		return nil
	}
	return decisions
}

// LooksLikeTemplatePlaceholder returns true if the text looks like unedited
// scaffold content, including seeded draft prose that still needs explicit
// confirmation before it should satisfy governance/runtime checks.
func LooksLikeTemplatePlaceholder(text string) bool {
	lower := strings.ToLower(text)
	placeholderPhrases := []string{
		"describe the chosen approach",
		"list 2-3 implementation",
		"describe risk analysis",
		"describe rollout sequencing",
		"describe the key interfaces",
		"confirm or replace this after research and user selection",
		"confirm or replace this after interface review",
		"confirm or replace this after rollout planning",
		"confirm or replace this after risk review",
		"pending — detail after",
		"pending — define after",
		"pending — assess after",
		"pending investigation",
		"pending task objective",
		"replace with concrete",
		"record the selected approach only after",
		"name changed interfaces and data flows",
		"write the concrete rollback path",
		"list concrete risks only",
		"list assumptions only after",
	}
	for _, phrase := range placeholderPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func ValidateAssuranceStructure(content string) error {
	blockers := AssuranceStructureBlockers(content)
	if len(blockers) == 0 {
		return nil
	}
	return errors.New(strings.Join(blockers, "; "))
}

// AssuranceStructureBlockers returns a list of blocker strings for an invalid
// assurance.md body structure. Empty slice means the document is valid.
// This is the blocker-oriented counterpart of ValidateAssuranceStructure,
// consistent with ResearchStructureBlockers.
func AssuranceStructureBlockers(content string) []string {
	headings := requiredSectionsForArtifact("assurance.md")
	if len(headings) == 0 {
		return []string{"assurance_structure_invalid:no required sections configured for assurance.md"}
	}

	_, _, err := validateSectionStructure(content, headings)
	if err != nil {
		return []string{"assurance_structure_invalid:" + err.Error()}
	}

	// Deterministic placeholder floor (issue #47): a section can be structurally
	// non-empty yet still hold only the generated scaffold prose. Such content
	// is semantically blank and must not satisfy closeout. This blocker is the
	// fail-closed counterpart to the AI-driven assurance attestation; it cannot
	// be rubber-stamped because detection derives from the embedded template.
	var blockers []string
	for _, heading := range headings {
		body := strings.Join(markdownSectionLines(content, heading), "\n")
		if assuranceSectionLooksScaffold(heading, body) {
			blockers = append(blockers, "assurance_section_placeholder:"+heading)
		}
	}
	return blockers
}

// assuranceSectionScaffold lazily derives, from the embedded assurance.md
// template, the normalized scaffold body for each required section. The
// template is the single source of truth for what "unedited scaffold" looks
// like, so detection cannot drift from the template wording the way a
// hand-maintained phrase list does. Required-section bodies in the template
// contain no template directives, so the raw content is sufficient.
var assuranceSectionScaffold = sync.OnceValue(func() map[string]string {
	scaffold := map[string]string{}
	content, err := TemplateContent("assurance.md")
	if err != nil {
		return scaffold
	}
	for _, heading := range requiredSectionsForArtifact("assurance.md") {
		body := normalizeAssuranceBody(strings.Join(markdownSectionLines(content, heading), "\n"))
		if body != "" {
			scaffold[heading] = body
		}
	}
	return scaffold
})

// normalizeAssuranceBody collapses whitespace so detection is insensitive to
// reflow, indentation, and trailing space.
func normalizeAssuranceBody(body string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(body)), " ")
}

// assuranceSectionLooksScaffold reports whether a required assurance section
// body is still template scaffold. Hybrid detection: an exact template-derived
// match catches the verbatim scaffold, and containment checks catch cases where
// the author appended prose without replacing the scaffold body or one of its
// seed sentences. Empty bodies are not flagged here — the structure check owns
// the empty case.
//
// The containment checks accept a false-positive tradeoff: authored prose that
// embeds a scaffold seed sentence verbatim is rejected. Short section seeds are
// more collision-prone, but the floor favors catching retained scaffold over
// admitting boilerplate-only assurance.
func assuranceSectionLooksScaffold(heading, body string) bool {
	scaffold := assuranceSectionScaffold()[heading]
	if scaffold == "" {
		return false
	}
	norm := normalizeAssuranceBody(body)
	if norm == "" {
		return false
	}
	if norm == scaffold || strings.Contains(norm, scaffold) {
		return true
	}
	for _, seed := range assuranceScaffoldSentences(scaffold) {
		if seed == scaffold {
			continue
		}
		if norm == seed || strings.Contains(norm, seed) {
			return true
		}
	}
	return false
}

// assuranceScaffoldSentences derives sentence-level scaffold seeds from a
// normalized template section body. This catches older one-sentence scaffold
// sections when the template later grows extra instructions.
func assuranceScaffoldSentences(scaffold string) []string {
	var sentences []string
	start := 0
	for i, r := range scaffold {
		switch r {
		case '.', '!', '?':
			sentence := strings.TrimSpace(scaffold[start : i+1])
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			start = i + 1
		}
	}
	if tail := strings.TrimSpace(scaffold[start:]); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

// renderTemplateWithFallback resolves a template using the following priority:
//  1. External template file (externalPath relative to root) if non-empty
//  2. Embedded template from internal/tmpl/templates/artifacts/
//  3. Minimal stub with artifact name as heading
func renderTemplateWithFallback(root, name, externalPath string, data templateData) (string, error) {
	var content string

	if externalPath != "" {
		absPath := externalPath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(root, absPath)
		}
		raw, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("custom template %q for artifact %q: %w", externalPath, name, err)
		}
		content = string(raw)
	} else {
		embedded, err := TemplateContent(name)
		if err != nil {
			// No embedded template available; use minimal stub.
			content = "# " + strings.TrimSuffix(name, filepath.Ext(name)) + "\n"
		} else {
			content = embedded
		}
	}

	t, err := template.New(name).Option("missingkey=error").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", name, err)
	}
	return b.String(), nil
}

func parseResearchAlternatives(lines []string) (int, string) {
	optionCount := 0
	selectedDirection := ""
	currentHeading := ""
	currentContent := []string{}

	flush := func() {
		if currentHeading == "" {
			return
		}
		if strings.EqualFold(currentHeading, "Selected Direction") {
			selectedDirection = strings.TrimSpace(strings.Join(currentContent, "\n"))
			return
		}
		optionCount++
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			flush()
			currentHeading = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			currentContent = nil
			continue
		}
		if currentHeading != "" && trimmed != "" {
			currentContent = append(currentContent, trimmed)
		}
	}
	flush()
	return optionCount, selectedDirection
}

func markdownSectionLines(content string, heading string) []string {
	normalizedHeading := heading
	if !strings.HasPrefix(normalizedHeading, "## ") {
		normalizedHeading = "## " + normalizedHeading
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	inSection := false
	section := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if trimmed == normalizedHeading {
				inSection = true
				continue
			}
			if inSection {
				break
			}
		}
		if inSection {
			section = append(section, line)
		}
	}
	return section
}

// PropagateStale marks downstream artifacts as stale when a source artifact is modified.
// Frozen artifacts are skipped (immutable). If graph is nil, the default expanded
// stale graph is used.
//
// Content-hash optimization: if the modified artifact's ArtifactState has a ContentHash
// and the file on disk matches that hash, no propagation occurs (no real change).
func PropagateStale(change *model.Change, modifiedArtifact string, graph ...map[string][]string) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}

	var g map[string][]string
	if len(graph) > 0 && graph[0] != nil {
		g = graph[0]
	} else {
		g = DefaultStaleGraph()
	}

	// Content-hash shortcut: if the modified artifact's stored hash matches the
	// current file content, the edit was a no-op and propagation can be skipped.
	artifactID := strings.TrimSuffix(modifiedArtifact, filepath.Ext(modifiedArtifact))
	if current, ok := change.Artifacts[artifactID]; ok && current.ContentHash != "" && current.Path != "" {
		if diskHash, err := model.ComputeFileContentHash(current.Path); err == nil && diskHash == current.ContentHash {
			return nil // no real change; skip propagation
		}
	}

	downstream, err := stalePropagationOrderFromGraph(modifiedArtifact, g)
	if err != nil {
		return err
	}

	for _, name := range downstream {
		downID := strings.TrimSuffix(name, filepath.Ext(name))
		current, ok := change.Artifacts[downID]
		if !ok {
			continue
		}
		// Skip frozen artifacts — they are immutable.
		if current.State == model.ArtifactLifecycleFrozen {
			continue
		}
		current.State = model.ArtifactLifecycleStale
		change.Artifacts[downID] = current
	}
	return nil
}

// stalePropagationOrderFromGraph is the internal BFS traversal used by
// PropagateStale and artifact package tests.
func stalePropagationOrderFromGraph(start string, g map[string][]string) ([]string, error) {
	if _, ok := g[start]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownArtifact, start)
	}

	order := make([]string, 0)
	visited := map[string]struct{}{}
	queue := append([]string(nil), g[start]...)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if _, seen := visited[node]; seen {
			continue
		}
		visited[node] = struct{}{}
		order = append(order, node)

		next := append([]string(nil), g[node]...)
		slices.Sort(next)
		queue = append(queue, next...)
	}

	return order, nil
}

// AmendmentEvent records a frozen artifact that was auto-amended during reconciliation.
type AmendmentEvent struct {
	ArtifactID   string `json:"artifact_id"`
	FromState    string `json:"from_state"`
	ToState      string `json:"to_state"`
	PreviousHash string `json:"previous_hash,omitempty"`
	NewHash      string `json:"new_hash,omitempty"`
}

// ReconcileResult captures the facts produced by ReconcileFromFilesystem.
// Callers can inspect Amendments to surface auto-amendment events.
type ReconcileResult struct {
	Amendments []AmendmentEvent `json:"amendments,omitempty"`
}

// ReconcileFromFilesystem reconciles artifact states with filesystem reality for governed changes.
// Rules:
// - frozen: never overridden by filesystem reconciliation (unless in amendment-eligible state)
// - file missing: state becomes draft
// - file exists and content hash differs from stored: current lifecycle unchanged, downstream stale propagation
// - file exists and content hash matches: no change
//
// Returns a ReconcileResult containing any amendment events that occurred.
func ReconcileFromFilesystem(root string, change *model.Change, preset ...model.WorkflowPreset) (ReconcileResult, error) {
	var result ReconcileResult
	if change == nil {
		return result, fmt.Errorf("change is required")
	}
	schemaName := change.ArtifactSchema
	if schemaName == "" {
		schemaName = model.ArtifactSchemaExpanded
	}
	schema := ResolveSchema(schemaName, change.CustomArtifacts)
	if err := materializeRequiredArtifacts(root, change, schema, preset...); err != nil {
		return result, err
	}
	bundleDir, err := state.GovernedBundleDir(root, *change)
	if err != nil {
		return result, err
	}

	// Amendment-eligible states: S2_EXECUTE and S3_REVIEW allow frozen
	// artifacts to be amended (unfrozen to approved) when their content changes.
	amendable := change.CurrentState == model.StateS2Execute ||
		change.CurrentState == model.StateS3Review

	for id, artifact := range change.Artifacts {
		// Frozen artifacts: immutable unless in an amendment-eligible state.
		if artifact.State == model.ArtifactLifecycleFrozen {
			if !amendable {
				continue
			}
			// In amendment-eligible state, check if content changed on disk.
			filePath := artifact.Path
			if filePath == "" {
				fileName := artifactFileName(id)
				filePath = ResolveArtifactPath(bundleDir, change.Slug, fileName)
			}
			diskHash, err := model.ComputeFileContentHash(filePath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return result, err
			}
			if diskHash == artifact.ContentHash {
				continue // No change: stay frozen.
			}
			// Content changed: treat as amendment (unfreeze to approved).
			previousHash := artifact.ContentHash
			artifact.State = model.ArtifactLifecycleApproved
			artifact.ContentHash = diskHash
			change.Artifacts[id] = artifact
			result.Amendments = append(result.Amendments, AmendmentEvent{
				ArtifactID:   id,
				FromState:    string(model.ArtifactLifecycleFrozen),
				ToState:      string(model.ArtifactLifecycleApproved),
				PreviousHash: previousHash,
				NewHash:      diskHash,
			})
			// Propagate stale to downstream artifacts.
			fileName := artifactFileName(id)
			if err := PropagateStale(change, fileName); err != nil {
				if !errors.Is(err, ErrUnknownArtifact) {
					return result, err
				}
			}
			continue
		}

		// Resolve file path: use Path if set, otherwise construct from convention.
		filePath := artifact.Path
		if filePath == "" {
			fileName := artifactFileName(id)
			filePath = ResolveArtifactPath(bundleDir, change.Slug, fileName)
		}

		_, err := os.Stat(filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// File missing: reset to draft. If the artifact previously had
				// materialized content or a non-draft lifecycle, invalidate
				// downstream artifacts because their upstream input disappeared.
				if artifact.ContentHash != "" || artifact.State != model.ArtifactLifecycleDraft {
					fileName := artifactFileName(id)
					if err := PropagateStale(change, fileName); err != nil {
						if !errors.Is(err, ErrUnknownArtifact) {
							return result, err
						}
					}
				}
				artifact.State = model.ArtifactLifecycleDraft
				artifact.ContentHash = ""
				change.Artifacts[id] = artifact
				continue
			}
			return result, err
		}

		// File exists: compute content hash and compare.
		diskHash, err := model.ComputeFileContentHash(filePath)
		if err != nil {
			return result, err
		}

		if diskHash != artifact.ContentHash {
			// Hash differs: propagate stale to downstream, then update stored hash.
			fileName := artifactFileName(id)
			if err := PropagateStale(change, fileName); err != nil {
				// Unknown artifact in the stale graph is non-fatal; skip propagation.
				if !errors.Is(err, ErrUnknownArtifact) {
					return result, err
				}
			}
			artifact.ContentHash = diskHash
			change.Artifacts[id] = artifact
		}
		// Hash matches: no change needed.
	}

	return result, nil
}

func materializeRequiredArtifacts(root string, change *model.Change, schema []ArtifactSpec, preset ...model.WorkflowPreset) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}

	base, err := state.GovernedBundleDir(root, *change)
	if err != nil {
		return err
	}
	presetArgs := append([]model.WorkflowPreset{change.WorkflowPreset}, preset...)
	for _, name := range RequiredArtifactsForChange(schema, change.NeedsDiscovery, presetArgs...) {
		artifactID := strings.TrimSuffix(name, filepath.Ext(name))
		current := change.Artifacts[artifactID]
		if current.ID == "" {
			current.ID = artifactID
		}
		if current.State == "" {
			current.State = model.ArtifactLifecycleDraft
		}
		if current.Path == "" {
			current.Path = ResolveArtifactPath(base, change.Slug, name)
		}

		info, err := os.Stat(current.Path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			change.Artifacts[artifactID] = current
			continue
		}

		if current.ContentHash == "" {
			hash, err := model.ComputeFileContentHash(current.Path)
			if err != nil {
				return err
			}
			current.ContentHash = hash
		}
		if current.UpdatedAt.IsZero() {
			current.UpdatedAt = info.ModTime().UTC()
		}
		change.Artifacts[artifactID] = current
	}

	return nil
}

// artifactFileName returns the filesystem name for an artifact ID.
// If the ID already contains a dot (extension), it is used as-is.
// Otherwise, "change" and "tasks" get ".yaml" and everything else gets ".md".
func artifactFileName(id string) string {
	if strings.Contains(id, ".") {
		return id
	}
	if id == "change" {
		return id + ".yaml"
	}
	return id + ".md"
}
