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

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
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
func ResolveArtifactPath(baseDir, artifactName string) string {
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

// RenderArtifactExample renders the named artifact template (e.g.
// "requirements.md") with representative example data so callers such as
// `slipway instructions` can present a concrete, directive-free exemplar —
// headings and authoring-guidance comments — instead of the raw Go-template
// source with unresolved `{{ … }}` actions. The engine owns this structure; the
// authoring skill writes the substance into the skeleton (issues #91, #119).
func RenderArtifactExample(name string) (string, error) {
	example := model.Change{
		Slug:        "example-change",
		Description: "describe the change here",
	}
	return renderTemplateWithFallback("", name, "", buildTemplateData(example))
}

// RenderArtifactExampleWithTemplate renders a governed artifact exemplar using
// an optional external template path. It is the custom-schema companion to
// RenderArtifactExample for command surfaces such as `slipway instructions`.
func RenderArtifactExampleWithTemplate(root, name, externalPath string) (string, error) {
	example := model.Change{
		Slug:        "example-change",
		Description: "describe the change here",
	}
	return renderTemplateWithFallback(root, name, externalPath, buildTemplateData(example))
}

type templateData struct {
	Slug              string
	InitialRequest    string
	QualityMode       string
	BundleRoot        string
	CodebaseMapRoot   string
	BundleArchiveRoot string
	ComplexityLevel   string
	ComplexityRank    int    // 0=trivial, 1=simple, 2=complex, 3=critical
	GuardrailDomain   string // Inferred guardrail domain (e.g. auth_authz, security_credentials)
}

func buildTemplateData(change model.Change) templateData {
	slug := strings.TrimSpace(change.Slug)
	data := templateData{
		Slug:              slug,
		InitialRequest:    strings.TrimSpace(change.Description),
		QualityMode:       string(change.EffectiveQualityMode()),
		BundleRoot:        filepath.ToSlash(filepath.Join("artifacts", "changes", slug)),
		CodebaseMapRoot:   filepath.ToSlash(filepath.Join("artifacts", "codebase")),
		BundleArchiveRoot: filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug)),
		ComplexityLevel:   change.ComplexityLevel,
		ComplexityRank:    complexityRank(change.ComplexityLevel),
		GuardrailDomain:   change.GuardrailDomain,
	}
	return data
}

// ScaffoldIntentForChange creates only intent.md in the governed bundle
// directory. This is used when preset is pending to ensure the intake artifact
// exists without creating the full governed bundle.
func ScaffoldIntentForChange(root string, change model.Change) error {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return fmt.Errorf("slug is required")
	}
	base, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	data := buildTemplateData(change)
	intentPath := ResolveArtifactPath(base, "intent.md")
	if _, err := os.Stat(intentPath); err == nil {
		return nil // already exists
	}
	rendered, err := renderTemplateWithFallback(root, "intent.md", "", data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(intentPath), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	return os.WriteFile(intentPath, []byte(rendered), 0o644) // #nosec G306 -- file is a user-facing project or governance artifact where operator-readable mode is intentional.
}

// ScaffoldGovernedBundleForChange creates the artifact files for a governed
// change. The engine owns structure only; project context is persisted on the
// change and surfaced through `slipway instructions`, not copied into bodies.
func ScaffoldGovernedBundleForChange(root string, change model.Change, preset model.WorkflowPreset, schema ...[]ArtifactSpec) error {
	ops, err := ScaffoldGovernedBundleTransactionOpsForChange(root, change, preset, schema...)
	if err != nil {
		return err
	}
	return fsutil.ApplyFileTransaction(ops)
}

// ScaffoldGovernedBundleTransactionOpsForChange returns the file operations
// needed to create scaffold-owned governed artifacts without applying them.
func ScaffoldGovernedBundleTransactionOpsForChange(root string, change model.Change, preset model.WorkflowPreset, schema ...[]ArtifactSpec) ([]fsutil.FileTransactionOp, error) {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
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
		return nil, err
	}
	if err := os.MkdirAll(base, 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return nil, err
	}
	data := buildTemplateData(change)
	ops := make([]fsutil.FileTransactionOp, 0, len(files))

	// Build a lookup from artifact name to template source path for custom schemas.
	templatePaths := map[string]string{}
	for _, spec := range specs {
		if spec.Template != "" {
			templatePaths[spec.Name] = spec.Template
		}
	}

	for _, file := range files {
		// The engine defers creation of skill-authored artifacts so an
		// un-authored required artifact is missing (fail-closed), not a
		// placeholder body the skill must overwrite (issue #119).
		if deferredToSkillAuthoring(file) {
			continue
		}
		path := ResolveArtifactPath(base, file)
		if shouldWrite, err := shouldScaffoldArtifactPath(path); err != nil {
			return nil, err
		} else if !shouldWrite {
			continue
		}

		rendered, err := renderTemplateWithFallback(root, file, templatePaths[file], data)
		if err != nil {
			return nil, err
		}
		ops = append(ops, fsutil.WriteFileTransactionOp(path, []byte(rendered), 0o644))
	}

	return ops, nil
}

func shouldScaffoldArtifactPath(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refuse scaffold artifact symlink %s", path)
	}
	return false, nil
}

// deferredToSkillAuthoring reports whether an artifact's body is authored
// directly by the host skill (via `slipway instructions <artifact>`) rather than
// scaffolded by the engine. The engine defers creation so an un-authored required
// artifact surfaces as missing (fail-closed), not a passing placeholder the skill
// must overwrite (issue #119).
//
// assurance.md is deferred too (issue #141): it is a review/verify-phase
// deliverable with nothing to write before execution completes, so the engine no
// longer seeds an early scaffold at S1_PLAN/bundle. The owning host authors it at
// S3_REVIEW from `slipway instructions assurance`, and its existence is enforced
// solely by AssuranceContractBlockers at S3_REVIEW and later — see
// existenceOwnedByDedicatedGate in the progression package, which keeps the
// pre-S3 bundle/readiness existence gates from stranding a change on its absence.
func deferredToSkillAuthoring(name string) bool {
	switch name {
	case "requirements.md", "decision.md", "research.md", "tasks.md", "assurance.md":
		return true
	default:
		return false
	}
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

// EnsureResearchArtifactForChange ensures the governed bundle directory exists
// for research authoring. research.md itself stays absent until the
// research-orchestration host writes the real file from `slipway instructions
// research`, so an un-authored research artifact fails closed as missing.
func EnsureResearchArtifactForChange(root string, change model.Change) error {
	base, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	return os.MkdirAll(base, 0o755) // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
}

// validateSectionStructure validates that content contains the given headings
// in order, each with at least one non-comment content line beneath it.
func validateSectionStructure(content string, headings []string) error {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	indices := make([]int, 0, len(headings))
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
			return fmt.Errorf("missing required heading %q", heading)
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
		body := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(stringutil.StripHTMLComments(body)) == "" {
			return fmt.Errorf("section %q must have non-empty content", heading)
		}
	}
	return nil
}

// ResearchStructureBlockers validates the research.md artifact structure.
func ResearchStructureBlockers(content string) []model.ReasonCode {
	headings := requiredSectionsForArtifact("research.md")
	if len(headings) == 0 {
		return nil // research.md may not be present for non-discovery changes
	}

	if err := validateSectionStructure(content, headings); err != nil {
		return []model.ReasonCode{model.NewReasonCode("research_structure_invalid", err.Error())}
	}

	var blockers []model.ReasonCode
	for _, heading := range headings {
		body := strings.Join(markdownSectionLines(content, heading), "\n")
		if artifactSectionBodyLooksPlaceholder(body) {
			blockers = append(blockers, model.NewReasonCode("research_section_placeholder", heading))
		}
	}
	return blockers
}

var legacyArtifactSectionPlaceholderPhrases = []string{
	"pending investigation. replace with concrete alternatives, tradeoffs, and the selected direction after research or code inspection.",
	"pending investigation. record the selected approach only after the alternatives have concrete evidence.",
	"pending investigation. name changed interfaces and data flows, or write \"none\" after inspection.",
	"pending investigation. write the concrete rollback path and verification command after implementation scope is known.",
	"pending investigation. list concrete risks only after inspecting the affected code and contracts.",
	"pending investigation. replace with concrete alternatives, supporting evidence, and the selected direction.",
	"pending investigation. list unknowns that must be resolved before planning.",
	"pending investigation. list assumptions only after identifying the evidence that supports them.",
	"requirements.md` and `decision.md` in the same bundle once planning artifacts are refined",
	"existing code paths and tests related to the affected behavior in the repository",
	"existing verification flows referenced by the scaffold context",
}

func artifactSectionBodyLooksPlaceholder(body string) bool {
	stripped := strings.TrimSpace(stringutil.StripHTMLComments(body))
	if stripped == "" {
		return true
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(stripped), " "))
	for _, phrase := range legacyArtifactSectionPlaceholderPhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
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
// confirmation before it should satisfy governance/runtime checks. It is the
// broad matcher used by the decision/runtime and task-objective paths: a
// superset of LooksLikeRequirementsPlaceholder (the requirements-scaffold seed
// markers + legacy tautology lines) plus the generic decision/research/task
// sentinels below. The requirements substance gate uses the narrower
// LooksLikeRequirementsPlaceholder so legitimately-authored requirement prose
// that shares a generic phrase is not false-flagged (issue #91).
func LooksLikeTemplatePlaceholder(text string) bool {
	if LooksLikeRequirementsPlaceholder(text) {
		return true
	}
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
		"pending verification objective",
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

	if err := validateSectionStructure(content, headings); err != nil {
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
		raw, err := os.ReadFile(absPath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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
				filePath = ResolveArtifactPath(bundleDir, fileName)
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
			filePath = ResolveArtifactPath(bundleDir, fileName)
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
			current.Path = ResolveArtifactPath(base, name)
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
