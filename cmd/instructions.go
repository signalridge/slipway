package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"github.com/spf13/cobra"
)

// instructionsArtifacts maps a public artifact name to its embedded template
// file name. The engine owns structure (these templates); the authoring skill
// owns substance. `slipway instructions <artifact>` serves a rendered exemplar
// plus the quality bar so a skill reads it before writing (issue #91), and —
// inside a governed change — the resolved output path, dependency/unlock graph,
// and tagged background it must honor while authoring (issue #119).
var instructionsArtifacts = map[string]string{
	"intent":       "intent.md",
	"requirements": "requirements.md",
	"tasks":        "tasks.md",
	"decision":     "decision.md",
	"research":     "research.md",
	"assurance":    "assurance.md",
}

// instructionsDependency names an upstream artifact this one depends on, with a
// path the authoring skill reads lazily and a done flag (the upstream artifact
// satisfies the engine-owned contract). Mirrors OpenSpec's dependency-as-path
// model: upstream inputs are referenced by path, never inlined into the produced
// artifact.
type instructionsDependency struct {
	Artifact string `json:"artifact"`
	Path     string `json:"path"`
	Done     bool   `json:"done"`
}

// instructionsView is the authoring payload returned by `slipway instructions`.
// Outside a governed change only Artifact/Guidance/Template are populated (a
// static exemplar a skill can read before any change exists). Inside an active
// change it is enriched with the resolved output path, the dependency/unlock
// graph, and tagged background (Context/Rules) the skill must respect but must
// NOT copy into the authored file.
type instructionsView struct {
	Artifact           string                   `json:"artifact"`
	Guidance           string                   `json:"guidance"`
	Template           string                   `json:"template"`
	ResolvedOutputPath string                   `json:"resolved_output_path,omitempty"`
	Dependencies       []instructionsDependency `json:"dependencies,omitempty"`
	Unlocks            []string                 `json:"unlocks,omitempty"`
	Context            string                   `json:"context,omitempty"`
	// ContextIsBaseline distinguishes the two opposite copy policies that share
	// the Context field: codebase-map baseline facts are meant to be preserved and
	// extended INTO the authored doc, whereas a governed artifact's project
	// context is background the skill must NOT copy. Text output uses it to pick a
	// non-contradictory heading (issue #119).
	ContextIsBaseline bool     `json:"context_is_baseline,omitempty"`
	Rules             []string `json:"rules,omitempty"`
}

func instructionsArtifactNames() []string {
	names := make([]string, 0, len(instructionsArtifacts))
	for name := range instructionsArtifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func instructionsGuidance(name string) string {
	switch name {
	case "requirements":
		return "Author each requirement as `### Requirement: <title>` followed by a `REQ-` " +
			"body that states what the system MUST, SHALL, or is REQUIRED to do (an RFC-2119 " +
			"strong-obligation keyword), and at least one concrete `#### Scenario:` with real " +
			"GIVEN/WHEN/THEN lines. The engine does not seed a body — this file is unwritten " +
			"until you author it; an empty or structure-only requirements file is rejected by " +
			"the substance gate and cannot reach done."
	case "tasks":
		return "Author each task as a checklist line (- [ ] t-NN <objective>) with wave, " +
			"depends_on, target_files, task_kind, and covers metadata. Every task names concrete target_files " +
			"that bound the files or evidence targets it changes or verifies. The engine does not seed a body; an " +
			"empty or placeholder tasks list is rejected by the substance gate."
	case "decision":
		return "Author a concrete decision with alternatives, selected approach, interfaces and data flow, " +
			"rollout/rollback, and risk. The engine does not seed this body; a missing or template-only decision " +
			"is rejected on expanded-schema plan readiness."
	case "research":
		return "Author evidence-backed research with alternatives, unknowns, assumptions, and canonical references. " +
			"The engine does not seed this body; write the real file from these instructions before advancing."
	case "intent":
		return "Use the intent structure to preserve confirmed intake facts and scope boundaries. The engine may " +
			"already have created this file during intake; keep user-confirmed substance and replace placeholder " +
			"sections instead of treating the template as final content."
	case "assurance":
		return "Author final closeout assurance from actual verification evidence: scope summary, verdict, " +
			"evidence index, requirement coverage, residual risks, rollback readiness, and archive decision. " +
			"The engine does not seed this body; it is deferred until you author it at S3_REVIEW from this " +
			"template. A missing, empty, or scaffold-only assurance is rejected at S3_REVIEW and later and cannot reach done."
	default:
		return "Author concrete, substantive content directly. The engine owns structure (the " +
			"template) and you own substance."
	}
}

// codebaseMapGuidance is the authoring quality bar for the repo-scoped
// codebase-map docs. Unlike a governed bundle artifact, the engine's baseline
// scan writes real machine-extracted facts (languages, tooling, layout) the
// author must preserve and extend — not a placeholder seed to delete.
func codebaseMapGuidance() string {
	return "Author this repo-scoped codebase-map doc with concrete, file:line-cited " +
		"findings: module boundaries, dependency direction, conventions, and " +
		"change-relevant risks. `slipway codebase-map` writes a baseline of real " +
		"machine-extracted facts (languages, build/test tooling, directory layout) " +
		"as a trustworthy starting point — preserve and extend it; do not delete it " +
		"as if it were placeholder seed. This map is advisory: it does not gate any " +
		"stage, but research, plan-audit, and wave-orchestration consume it by path."
}

// codebaseMapInstructionsView builds the authoring payload for a codebase-map
// doc when name matches one (by key or file name). It resolves the output path
// against the workspace root and offers the baseline facts as context, but never
// requires an active change. ok is false when name is not a codebase-map doc, so
// the caller falls through to the bundle-artifact path.
func codebaseMapInstructionsView(cmd *cobra.Command, name string) (instructionsView, bool) {
	file, template, ok := artifact.CodebaseMapDocInstruction(name)
	if !ok {
		return instructionsView{}, false
	}
	view := instructionsView{
		Artifact: artifact.CodebaseMapDocKey(file),
		Guidance: codebaseMapGuidance(),
		Template: template,
	}
	// Resolve the repo-scoped output path and baseline context against the
	// workspace root. A workspace that is not initialized leaves the payload as
	// the static template+guidance — instructions never fails just because the
	// root cannot be resolved.
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return view, true
	}
	workspaceRoot := invocationWorkspaceRootFromCommand(cmd, root)
	docPath := filepath.Join(state.CodebaseMapDir(workspaceRoot), file)
	view.ResolvedOutputPath = state.DisplayPath(workspaceRoot, docPath)
	// Offer the baseline only when the scan actually detected facts. When it
	// matches the bare template there is nothing to preserve, so attaching it as
	// "real detected facts" would be misleading noise.
	if baseline, ok := artifact.CodebaseMapBaselineDoc(workspaceRoot, file); ok &&
		strings.TrimSpace(baseline) != strings.TrimSpace(template) {
		view.Context = renderCodebaseMapBaselineContext(baseline)
		// The codebase-map baseline is real detected facts the author preserves
		// and extends into the doc — the opposite of a governed artifact's
		// do-not-copy project context. Flag it so text output labels it honestly.
		view.ContextIsBaseline = true
	}
	return view, true
}

// renderCodebaseMapBaselineContext frames the baseline doc as background the
// author preserves and extends. It is tagged like other instructions context so
// the skill respects it, but unlike bundle context the baseline IS meant to seed
// the authored file (it is real detected facts, not data to keep out of the body).
func renderCodebaseMapBaselineContext(baseline string) string {
	baseline = strings.TrimSpace(baseline)
	if baseline == "" {
		return ""
	}
	return "Baseline facts from `slipway codebase-map` (real detected facts — " +
		"preserve and extend, do not delete):\n" + baseline
}

func makeInstructionsCmd() *cobra.Command {
	var jsonOutput bool
	var changeSlug string

	cmd := &cobra.Command{
		Use:   "instructions <artifact>",
		Short: desc("instructions"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Accept both the public name ("decision") and the artifact file
			// name ("decision.md") so the missing_required_artifact remediation
			// command (`slipway instructions <subject>`, subject = file name) runs.
			name, artifactFile := normalizeInstructionsArtifactArg(args[0])

			// Codebase-map docs share the instructions->author contract but are
			// repo-scoped and advisory: they resolve against the workspace root
			// and never require an active change.
			if view, ok := codebaseMapInstructionsView(cmd, name); ok {
				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}
				return writeInstructionsText(cmd, view)
			}

			templateName, ok := instructionsArtifacts[name]
			if !ok {
				view, customOK, err := customArtifactInstructionsView(cmd, name, artifactFile, changeSlug)
				if err != nil {
					return err
				}
				if customOK {
					if jsonOutput {
						return encodeJSONResponse(cmd, view)
					}
					return writeInstructionsText(cmd, view)
				}
				return newInvalidUsageError(
					"unknown_artifact",
					fmt.Sprintf("unknown artifact %q", name),
					"Choose a governed artifact: "+strings.Join(instructionsArtifactNames(), ", ")+
						"; or a codebase-map doc: "+strings.Join(artifact.CodebaseMapInstructionKeys(), ", "),
					nil,
				)
			}
			content, err := artifact.RenderArtifactExample(templateName)
			if err != nil {
				return err
			}
			view := instructionsView{
				Artifact: name,
				Guidance: instructionsGuidance(name),
				Template: content,
			}
			// Enrich with change-aware authoring context when invoked inside a
			// governed change. With no --change selector the command still serves
			// the static exemplar on any resolution failure, but an explicit
			// --change fails closed (issue #119).
			if err := enrichInstructionsView(cmd, &view, changeSlug, templateName); err != nil {
				return err
			}

			if jsonOutput {
				return encodeJSONResponse(cmd, view)
			}
			return writeInstructionsText(cmd, view)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().StringVar(&changeSlug, "change", "", "Governed change slug (defaults to the active change)")
	return cmd
}

func normalizeInstructionsArtifactArg(arg string) (name, artifactFile string) {
	raw := strings.TrimSpace(arg)
	lower := strings.ToLower(raw)
	name = strings.TrimSuffix(lower, ".md")
	artifactFile = raw
	if filepath.Ext(artifactFile) == "" {
		artifactFile += ".md"
	}
	return name, artifactFile
}

func customArtifactInstructionsView(
	cmd *cobra.Command,
	name string,
	artifactFile string,
	changeSlug string,
) (instructionsView, bool, error) {
	explicit := strings.TrimSpace(changeSlug) != ""
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		if explicit {
			return instructionsView{}, false, err
		}
		return instructionsView{}, false, nil
	}
	ref, err := resolveActiveChangeRef(root, changeSlug)
	if err != nil {
		if explicit {
			return instructionsView{}, false, err
		}
		return instructionsView{}, false, nil
	}
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return instructionsView{}, false, err
	}
	spec, ok := findInstructionsArtifactSpec(
		artifact.ResolveSchema(change.ArtifactSchema, change.CustomArtifacts),
		name,
		artifactFile,
	)
	if !ok {
		return instructionsView{}, false, nil
	}
	content, err := artifact.RenderArtifactExampleWithTemplate(root, spec.Name, spec.Template)
	if err != nil {
		return instructionsView{}, false, err
	}
	viewName := strings.TrimSuffix(spec.Name, filepath.Ext(spec.Name))
	if strings.TrimSpace(viewName) == "" {
		viewName = spec.Name
	}
	view := instructionsView{
		Artifact: viewName,
		Guidance: instructionsGuidance(name),
		Template: content,
	}
	if err := enrichInstructionsViewForChange(&view, root, change, spec.Name); err != nil {
		if explicit {
			return instructionsView{}, false, err
		}
		warnInstructionsStaticFallback(cmd, err)
	}
	return view, true, nil
}

func findInstructionsArtifactSpec(schema []artifact.ArtifactSpec, name, artifactFile string) (artifact.ArtifactSpec, bool) {
	for _, spec := range schema {
		specName := strings.TrimSpace(spec.Name)
		if specName == "" {
			continue
		}
		if strings.EqualFold(specName, artifactFile) {
			return spec, true
		}
		specKey := strings.TrimSuffix(specName, filepath.Ext(specName))
		if strings.EqualFold(specKey, name) {
			return spec, true
		}
	}
	return artifact.ArtifactSpec{}, false
}

// enrichInstructionsView adds active-change authoring context to view when a
// governed change can be resolved. With no change selector, a missing active
// change leaves view as the static exemplar; other resolution failures still
// fall back but emit a warning so operators do not mistake static output for a
// resolved authoring payload. But an explicit --change must fail closed:
// silently downgrading a missing or typo'd slug to static output makes it look
// successful and defeats the recovery command (issue #119).
func enrichInstructionsView(cmd *cobra.Command, view *instructionsView, changeSlug, artifactFile string) error {
	explicit := strings.TrimSpace(changeSlug) != ""
	fail := func(err error) error {
		if explicit {
			return err
		}
		warnInstructionsStaticFallback(cmd, err)
		return nil
	}
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return fail(err)
	}
	ref, err := resolveActiveChangeRef(root, changeSlug)
	if err != nil {
		return fail(err)
	}
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return fail(err)
	}
	if err := enrichInstructionsViewForChange(view, root, change, artifactFile); err != nil {
		return fail(err)
	}
	return nil
}

func enrichInstructionsViewForChange(
	view *instructionsView,
	root string,
	change model.Change,
	artifactFile string,
) error {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}

	view.ResolvedOutputPath = state.DisplayPath(root, artifact.ResolveArtifactPath(paths.GovernedBundleDir, artifactFile))

	schema := artifact.ResolveSchema(change.ArtifactSchema, change.CustomArtifacts)
	required := make(map[string]bool)
	for _, name := range progression.RequiredArtifactNames(root, change) {
		required[name] = true
	}
	for _, spec := range schema {
		if spec.Name == artifactFile {
			for _, dep := range spec.DependsOn {
				depPath := artifact.ResolveArtifactPath(paths.GovernedBundleDir, dep)
				_, statErr := os.Stat(depPath)
				if !required[dep] && statErr != nil {
					continue
				}
				view.Dependencies = append(view.Dependencies, instructionsDependency{
					Artifact: dep,
					Path:     state.DisplayPath(root, depPath),
					Done:     instructionsDependencyDone(paths.GovernedBundleDir, dep, statErr),
				})
			}
			continue
		}
		// Unlocks: required artifacts that depend on this one.
		if required[spec.Name] {
			for _, dep := range spec.DependsOn {
				if dep == artifactFile {
					view.Unlocks = append(view.Unlocks, spec.Name)
				}
			}
		}
	}
	view.Unlocks = stringutil.UniqueSorted(view.Unlocks)

	view.Context = renderInstructionsContext(change.ProjectContext)
	view.Rules = instructionsRules(change)
	return nil
}

func instructionsDependencyDone(bundleDir string, artifactName string, statErr error) bool {
	if statErr != nil {
		return false
	}

	switch artifactName {
	case "requirements.md":
		result, err := artifact.EvaluateRequirementsContract(bundleDir)
		return err == nil && result.Status == artifact.RequirementsContractStatusValid
	case "decision.md":
		result, err := artifact.EvaluateDecisionContract(bundleDir)
		return err == nil && result.Status == artifact.DecisionContractStatusValid
	case "tasks.md":
		result, err := artifact.EvaluateTasksContract(bundleDir)
		return err == nil && result.Status == artifact.TasksContractStatusValid
	case "research.md":
		data, err := os.ReadFile(artifact.ResolveArtifactPath(bundleDir, artifactName))
		return err == nil && len(artifact.ResearchStructureBlockers(string(data))) == 0
	case "assurance.md":
		data, err := os.ReadFile(artifact.ResolveArtifactPath(bundleDir, artifactName))
		return err == nil && len(artifact.AssuranceStructureBlockers(string(data))) == 0
	default:
		return true
	}
}

func warnInstructionsStaticFallback(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}
	var cliErr *CLIError
	if errors.As(err, &cliErr) && cliErr.ErrorCode == "no_active_change" {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"warning: serving static instructions because active change context could not be resolved: %v\n",
		err)
}

// renderInstructionsContext renders project context as tagged background. Per
// the OpenSpec model this is information the authoring skill must respect but
// must NOT copy into the artifact body (it previously leaked in as a
// "## Project Context" block written straight into the file).
func renderInstructionsContext(pc model.ProjectContext) string {
	if pc.IsZero() {
		return ""
	}
	var b strings.Builder
	b.WriteString("Background only — do NOT copy this into the authored artifact.\n")
	if v := strings.TrimSpace(pc.TechStack); v != "" {
		fmt.Fprintf(&b, "- Tech Stack: %s\n", v)
	}
	if len(pc.Languages) > 0 {
		fmt.Fprintf(&b, "- Languages: %s\n", strings.Join(pc.Languages, ", "))
	}
	if v := strings.TrimSpace(pc.TestCmd); v != "" {
		fmt.Fprintf(&b, "- Test Command: %s\n", v)
	}
	if v := strings.TrimSpace(pc.BuildCmd); v != "" {
		fmt.Fprintf(&b, "- Build Command: %s\n", v)
	}
	if v := strings.TrimSpace(pc.Conventions); v != "" {
		fmt.Fprintf(&b, "- Conventions: %s\n", v)
	}
	return strings.TrimRight(b.String(), "\n")
}

// instructionsRules returns artifact-authoring constraints the skill must honor
// but must not copy into the artifact (tagged background, like context).
func instructionsRules(change model.Change) []string {
	var rules []string
	if gd := strings.TrimSpace(change.GuardrailDomain); gd != "" {
		rules = append(rules, fmt.Sprintf(
			"This change touches the %s guarded domain: keep the artifact's guardrail "+
				"obligations explicit and fail closed to review and evidence (no bypass, "+
				"force-pass, or private attestation).", gd))
	}
	return rules
}

func writeInstructionsText(cmd *cobra.Command, view instructionsView) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "# Authoring instructions: %s\n\n", view.Artifact)
	fmt.Fprintln(out, view.Guidance)
	if view.ResolvedOutputPath != "" {
		fmt.Fprintf(out, "\nWrite the authored artifact to: %s\n", view.ResolvedOutputPath)
	}
	if len(view.Dependencies) > 0 {
		fmt.Fprintln(out, "\n## Dependencies (read these by path; do not inline them)")
		for _, dep := range view.Dependencies {
			status := "missing"
			if dep.Done {
				status = "done"
			}
			fmt.Fprintf(out, "- %s [%s] — %s\n", dep.Artifact, status, dep.Path)
		}
	}
	if len(view.Unlocks) > 0 {
		fmt.Fprintf(out, "\nCompleting this unlocks: %s\n", strings.Join(view.Unlocks, ", "))
	}
	if view.Context != "" {
		// The Context field carries two opposite copy policies; label it honestly
		// so the heading never contradicts the body (issue #119).
		if view.ContextIsBaseline {
			fmt.Fprintln(out, "\n## Baseline facts (preserve and extend — this is your starting content)")
		} else {
			fmt.Fprintln(out, "\n## Project context (background — do NOT copy into the artifact)")
		}
		fmt.Fprintln(out, view.Context)
	}
	if len(view.Rules) > 0 {
		fmt.Fprintln(out, "\n## Rules (constraints — do NOT copy into the artifact)")
		for _, rule := range view.Rules {
			fmt.Fprintf(out, "- %s\n", rule)
		}
	}
	fmt.Fprintln(out, "\n## Template")
	fmt.Fprintln(out)
	fmt.Fprintln(out, view.Template)
	return nil
}
