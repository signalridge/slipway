package progression

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"github.com/signalridge/slipway/internal/wave"
)

var proseRequirementReferencePattern = regexp.MustCompile(`\bREQ-([A-Z0-9_-]+)\b`)
var proseHTMLCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)

// ChangeSchemaResolution captures the resolved artifact schema plus
// diagnostics/blockers produced during strict schema resolution.
type ChangeSchemaResolution struct {
	Schema   []artifact.ArtifactSpec
	Warnings []string
	Blockers []string
}

type TaskChecklistValidationResult struct {
	Blockers []string
	Warnings []string
}

// ValidateTasksChecklistDetailed validates the tasks.md checklist for a
// governed change and returns both blockers and non-blocking advisories.
func ValidateTasksChecklistDetailed(root string, change model.Change) TaskChecklistValidationResult {
	result := TaskChecklistValidationResult{}

	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		result.Blockers = []string{"tasks_checklist_path_invalid:" + err.Error()}
		return result
	}
	path := filepath.Join(bundleDir, "tasks.md")
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			result.Blockers = []string{"tasks_checklist_missing"}
			return result
		}
		result.Blockers = []string{"tasks_checklist_unreadable"}
		return result
	}
	plan, err := wave.ParseTaskPlan(string(raw))
	if err != nil {
		result.Blockers = []string{"tasks_checklist_invalid_format:" + err.Error()}
		return result
	}
	if len(plan.Tasks) == 0 {
		result.Blockers = []string{"tasks_checklist_empty"}
		return result
	}

	// target_files enforcement is deferred to S1_PLAN/audit and later.
	// Pre-audit drafts may begin with empty target_files.
	enforceTargetFiles := isAtOrPastPlanAudit(change)

	seen := map[string]struct{}{}
	idSet := map[string]struct{}{}
	dependencies := map[string][]string{}
	dependencyGraphValid := true
	coverageAsWarning := false
	if presetPolicy, presetErr := governance.ResolvePresetPolicy(root, change); presetErr == nil {
		coverageAsWarning = presetPolicy.EffectivePreset == model.WorkflowPresetLight
	}

	for i, task := range plan.Tasks {
		id := strings.TrimSpace(task.TaskID)
		if id == "" {
			result.Blockers = append(result.Blockers, fmt.Sprintf("tasks_checklist_task_id_missing:index_%d", i))
			id = fmt.Sprintf("index_%d", i)
		}
		if _, exists := seen[id]; exists {
			result.Blockers = append(result.Blockers, fmt.Sprintf("tasks_checklist_duplicate_task_id:%s", id))
		}
		seen[id] = struct{}{}
		idSet[id] = struct{}{}

		objective := strings.TrimSpace(task.Objective)
		// A placeholder objective ("Pending task objective") is, for governance
		// purposes, a missing concrete objective — the engine seeds it and the
		// authoring skill must replace it (issue #91). Enforced from plan-audit
		// onward, mirroring target_files, so pre-audit drafts stay lenient.
		if objective == "" || (enforceTargetFiles && artifact.LooksLikeTemplatePlaceholder(objective)) {
			result.Blockers = append(result.Blockers, fmt.Sprintf("plan_dimension_completeness_missing_objective:%s", id))
		}
		if enforceTargetFiles {
			if len(task.TargetFiles) == 0 {
				result.Blockers = append(result.Blockers, fmt.Sprintf("plan_dimension_key_links_missing_target_files:%s", id))
			} else {
				hasPlaceholderTarget := false
				for _, file := range task.TargetFiles {
					target := model.NormalizePublicPath(file)
					if target == "" {
						result.Blockers = append(result.Blockers, fmt.Sprintf("plan_dimension_scope_invalid_target:%s", id))
						continue
					}
					if artifact.LooksLikeInstructionPlaceholder(target) {
						hasPlaceholderTarget = true
						continue
					}
					if model.PublicPathIsAbs(file) || model.PublicPathHasParentTraversal(file) {
						result.Blockers = append(result.Blockers, fmt.Sprintf("plan_dimension_scope_out_of_bounds_target:%s:%s", id, target))
					}
				}
				if hasPlaceholderTarget {
					result.Blockers = append(result.Blockers, fmt.Sprintf("plan_dimension_key_links_missing_target_files:%s", id))
				}
			}
		}

		if !task.HasDeclaredTaskKind() {
			result.Warnings = append(result.Warnings, fmt.Sprintf("plan_dimension_context_missing_task_kind_warning:%s", id))
		}

		deps := make([]string, 0, len(task.DependsOn))
		for _, dep := range task.DependsOn {
			depID := strings.TrimSpace(dep)
			if depID == "" {
				continue
			}
			deps = append(deps, depID)
		}
		dependencies[id] = deps
	}

	for id, deps := range dependencies {
		for _, dep := range deps {
			if dep == id {
				result.Blockers = append(result.Blockers, fmt.Sprintf("plan_dimension_dependency_self_reference:%s", id))
				dependencyGraphValid = false
				continue
			}
			if _, exists := idSet[dep]; !exists {
				result.Blockers = append(result.Blockers, fmt.Sprintf("plan_dimension_dependency_unknown:%s->%s", id, dep))
				dependencyGraphValid = false
			}
		}
	}

	if HasDependencyCycle(dependencies) {
		result.Blockers = append(result.Blockers, "plan_dimension_dependency_cycle_detected")
		dependencyGraphValid = false
	}
	if dependencyGraphValid {
		if _, err := wave.PlanWaves(plan.Nodes()); err != nil {
			result.Blockers = append(result.Blockers, "plan_dimension_execution_invalid_wave_plan:"+err.Error())
		}
	}
	coverageIssues, requirementBlockers := validateTaskCoverageAgainstSpec(root, change, plan)
	// Requirements-validity failures are hard regardless of preset: a mechanical
	// or non-substantive requirements.md cannot reach done (issue #91). Only the
	// requirement-to-task coverage advisories follow the light-preset downgrade.
	result.Blockers = append(result.Blockers, requirementBlockers...)
	result.Blockers = append(result.Blockers, validateProseRequirementReferencesAgainstSpec(root, change)...)
	if coverageAsWarning {
		for _, issue := range coverageIssues {
			result.Warnings = append(result.Warnings, checklistWarningToken(issue))
		}
	} else {
		result.Blockers = append(result.Blockers, coverageIssues...)
	}
	result.Blockers = stringutil.UniqueSorted(result.Blockers)
	result.Warnings = stringutil.UniqueSorted(result.Warnings)
	return result
}

func checklistWarningToken(token string) string {
	parts := strings.SplitN(strings.TrimSpace(token), ":", 2)
	if len(parts) == 0 || parts[0] == "" {
		return token
	}
	if len(parts) == 1 {
		return parts[0] + "_warning"
	}
	return parts[0] + "_warning:" + parts[1]
}

// validateTaskCoverageAgainstSpec evaluates requirements.md against the task
// plan and returns two distinct blocker sets:
//
//   - coverageIssues: requirement-to-task coverage advisories (missing/unknown
//     coverage, missing stable id, unreadable spec). The light preset downgrades
//     these to warnings.
//   - requirementBlockers: hard requirements-validity failures that block in any
//     preset — a structurally invalid requirements.md, or (from plan-audit
//     onward) a non-substantive one: placeholder/seed content, a requirement body
//     with no RFC-2119 MUST/SHALL keyword, or a requirement with no concrete
//     scenario. A mechanical or vacuous requirements.md cannot reach done
//     (issue #91). The umbrella blocker keeps a stable token; the detailed
//     substance reasons are surfaced by the requirements contract in
//     `slipway validate`.
func validateTaskCoverageAgainstSpec(root string, change model.Change, plan wave.TaskPlan) (coverageIssues, requirementBlockers []string) {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return nil, nil
	}
	specPath := artifact.ResolveArtifactPath(bundleDir, "requirements.md")
	raw, err := os.ReadFile(specPath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return []string{"plan_dimension_coverage_spec_unreadable"}, nil
	}

	content := string(raw)
	requirementBlocks := artifact.ParseRequirementBlocks(content)
	if strings.TrimSpace(content) != "" && len(requirementBlocks) == 0 {
		return nil, []string{"plan_dimension_coverage_requirements_invalid"}
	}

	missingStableIDs := artifact.RequirementBlocksMissingStableIDs(content)
	for _, name := range missingStableIDs {
		coverageIssues = append(coverageIssues, "plan_dimension_coverage_requirement_id_missing:"+name)
	}

	// From plan-audit onward, requirements.md must carry real substance, not the
	// engine's placeholder scaffold and not a structurally-present-but-vacuous
	// requirement (no MUST/SHALL body, no concrete scenario). This is a hard
	// validity gate independent of preset (issue #91).
	if isAtOrPastPlanAudit(change) && len(artifact.RequirementSubstanceBlockers(content)) > 0 {
		requirementBlockers = append(requirementBlockers, "plan_dimension_coverage_requirements_invalid")
	}

	requirementIDs := artifact.ExtractRequirementStableIDs(content)
	if len(requirementIDs) == 0 {
		return stringutil.UniqueSorted(coverageIssues), stringutil.UniqueSorted(requirementBlockers)
	}

	requirementByID := make(map[string]string, len(requirementIDs))
	covered := make(map[string]bool, len(requirementIDs))
	for _, requirementID := range requirementIDs {
		requirementByID[requirementID] = requirementID
	}

	for _, task := range plan.Tasks {
		for _, cover := range task.Covers {
			requirementID := artifact.NormalizeRequirementID(cover)
			if requirementID == "" {
				coverageIssues = append(coverageIssues, fmt.Sprintf("plan_dimension_coverage_unknown_requirement:%s->%s", task.TaskID, strings.TrimSpace(cover)))
				continue
			}
			if _, ok := requirementByID[requirementID]; !ok {
				coverageIssues = append(coverageIssues, fmt.Sprintf("plan_dimension_coverage_unknown_requirement:%s->%s", task.TaskID, requirementID))
				continue
			}
			covered[requirementID] = true
		}
	}

	for requirementID := range requirementByID {
		if covered[requirementID] {
			continue
		}
		coverageIssues = append(coverageIssues, "plan_dimension_coverage_missing_requirement:"+requirementID)
	}
	return stringutil.UniqueSorted(coverageIssues), stringutil.UniqueSorted(requirementBlockers)
}

func validateProseRequirementReferencesAgainstSpec(root string, change model.Change) []string {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return nil
	}
	requirementsPath := artifact.ResolveArtifactPath(bundleDir, "requirements.md")
	rawRequirements, err := os.ReadFile(requirementsPath) // #nosec G304 -- path is resolved from governed artifact authority.
	if err != nil {
		return nil
	}

	knownRequirements := map[string]struct{}{}
	for _, block := range artifact.ParseRequirementBlocks(string(rawRequirements)) {
		id := artifact.NormalizeRequirementID(block.StableID)
		if id == "" {
			continue
		}
		knownRequirements[id] = struct{}{}
	}
	if len(knownRequirements) == 0 {
		return nil
	}

	artifactNames := proseRequirementReferenceArtifactNames(root, change)
	var blockers []string
	for _, artifactName := range artifactNames {
		path := artifact.ResolveArtifactPath(bundleDir, artifactName)
		raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from governed artifact authority.
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			continue
		}
		refs := proseRequirementReferences(artifactName, string(raw))
		for _, ref := range refs {
			if _, ok := knownRequirements[ref]; ok {
				continue
			}
			blockers = append(blockers, "plan_dimension_consistency_unknown_requirement_ref:"+artifactName+":"+ref)
		}
	}
	return stringutil.UniqueSorted(blockers)
}

func proseRequirementReferenceArtifactNames(root string, change model.Change) []string {
	names := RequiredArtifactNames(root, change)
	if len(names) == 0 {
		return nil
	}

	scannable := make([]string, 0, len(names))
	for _, name := range names {
		if existenceOwnedByDedicatedGate(name) {
			continue
		}
		scannable = append(scannable, name)
	}
	return stringutil.UniqueSorted(scannable)
}

func proseRequirementReferences(artifactName, content string) []string {
	content = proseHTMLCommentPattern.ReplaceAllString(strings.ReplaceAll(content, "\r\n", "\n"), "")
	seen := map[string]struct{}{}
	var refs []string
	for _, line := range strings.Split(content, "\n") {
		if artifactName == "tasks.md" && strings.Contains(strings.ToLower(line), "covers:") {
			continue
		}
		for _, match := range proseRequirementReferencePattern.FindAllStringSubmatch(line, -1) {
			if len(match) != 2 {
				continue
			}
			ref := artifact.NormalizeRequirementID("REQ-" + match[1])
			if ref == "" {
				continue
			}
			if _, ok := seen[ref]; ok {
				continue
			}
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	slices.Sort(refs)
	return refs
}

// HasDependencyCycle returns true if the dependency graph contains a cycle.
func HasDependencyCycle(dependencies map[string][]string) bool {
	const (
		notVisited = 0
		visiting   = 1
		visited    = 2
	)
	stateByNode := map[string]int{}

	var visit func(string) bool
	visit = func(node string) bool {
		switch stateByNode[node] {
		case visiting:
			return true
		case visited:
			return false
		}
		stateByNode[node] = visiting
		for _, dep := range dependencies[node] {
			if _, exists := dependencies[dep]; !exists {
				continue
			}
			if visit(dep) {
				return true
			}
		}
		stateByNode[node] = visited
		return false
	}

	for node := range dependencies {
		if stateByNode[node] == notVisited && visit(node) {
			return true
		}
	}
	return false
}

// isAtOrPastPlanAudit returns true when the change is at S1_PLAN/audit or
// any later lifecycle position. Pre-audit states (S0_INTAKE, S1_PLAN/research,
// S1_PLAN/bundle) return false.
func isAtOrPastPlanAudit(change model.Change) bool {
	switch change.CurrentState {
	case model.StateS0Intake:
		return false
	case model.StateS1Plan:
		switch change.PlanSubStep {
		case model.PlanSubStepAudit, model.PlanSubStepValidate:
			return true
		default:
			return false
		}
	default:
		return true
	}
}

// ShouldCheckGovernedBundle reports whether bundle blockers should be surfaced
// as the standalone checker for the current workflow position.
func ShouldCheckGovernedBundle(change model.Change) bool {
	switch change.CurrentState {
	case model.StateS1Plan:
		switch change.PlanSubStep {
		case model.PlanSubStepBundle, model.PlanSubStepAudit:
			return true
		default:
			return false
		}
	case model.StateS2Implement, model.StateS3Review:
		return true
	default:
		return false
	}
}

// WorktreeDerivation holds the pure result of worktree metadata extraction and
// validation. It contains blockers (if any) and the derived metadata, but does
// not mutate the input change. The caller decides whether to apply the metadata.
type WorktreeDerivation struct {
	Blockers       []string
	WorktreePath   string
	WorktreeBranch string
}

// DeriveWorktreeBlockers is a pure function that extracts worktree metadata
// from skill evidence, validates it, and returns blockers and derived metadata
// without mutating the change. The caller is responsible for applying the
// derived metadata via ApplyWorktreeMetadata if there are no blockers.
func DeriveWorktreeBlockers(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
) (WorktreeDerivation, error) {
	isWorktreeState := change.CurrentState == model.StateS2Implement
	if !change.NeedsDiscovery || !isWorktreeState {
		return WorktreeDerivation{}, nil
	}

	record, ok := passingSkills[SkillWorktreePreflight]
	if !ok {
		diskRecord, err := state.LoadVerification(root, change.Slug, SkillWorktreePreflight)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return WorktreeDerivation{Blockers: []string{state.WorktreeReasonMetadataRequired}}, nil
			}
			return WorktreeDerivation{}, fmt.Errorf("load worktree-preflight verification: %w", err)
		}
		if !diskRecord.IsPassing() {
			return WorktreeDerivation{Blockers: []string{state.WorktreeReasonMetadataRequired}}, nil
		}
		record = diskRecord
	}

	worktreePath, worktreeBranch, _, reasons := state.ParseWorktreePreflightReferences(record.References)
	if len(reasons) > 0 {
		blockers := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			blockers = append(blockers, state.WorktreeReasonMetadataRequired+":worktree-preflight:"+reason)
		}
		return WorktreeDerivation{Blockers: stringutil.UniqueSorted(blockers)}, nil
	}

	candidate := change
	if err := state.PersistScopeWorktreeMetadata(&candidate, worktreePath, worktreeBranch); err != nil {
		return WorktreeDerivation{Blockers: []string{"worktree_metadata_persist_failed:" + err.Error()}}, nil
	}

	validation, err := state.ValidateChangeWorktree(root, candidate)
	if err != nil {
		return WorktreeDerivation{}, err
	}
	if len(validation.Blockers) > 0 {
		return WorktreeDerivation{Blockers: model.ReasonSpecs(validation.Blockers)}, nil
	}
	return WorktreeDerivation{
		WorktreePath:   candidate.WorktreePath,
		WorktreeBranch: candidate.WorktreeBranch,
	}, nil
}

// ApplyWorktreeMetadata applies derived worktree metadata to a change.
// This is the explicit mutation step that must be called by the mutating caller
// after DeriveWorktreeBlockers returns no blockers.
func ApplyWorktreeMetadata(change *model.Change, d WorktreeDerivation) error {
	return state.PersistScopeWorktreeMetadata(change, d.WorktreePath, d.WorktreeBranch)
}

// PresetConfirmationBlockers returns a preset_confirmation_required blocker
// when the change has a pending preset suggestion that has not been confirmed.
// This is a universal early S1 entry blocker — it must be checked before any
// artifact side effects (including discovery research.md authoring), not only
// when the governed bundle sub-step is active.
func PresetConfirmationBlockers(change model.Change) []string {
	if change.WorkflowPresetConfirmationPending() {
		return []string{"preset_confirmation_required"}
	}
	return nil
}

// CheckGovernedBundleReady returns true if all required governed bundle artifacts exist.
func CheckGovernedBundleReady(root string, change model.Change) bool {
	return len(GovernedBundleBlockers(root, change)) == 0
}

// RequiredArtifactNames returns the change's required governed artifact set
// after schema resolution and effective-preset policy are applied.
func RequiredArtifactNames(root string, change model.Change) []string {
	resolution := ResolveChangeSchemaDiagnostics(change)
	requiredPreset := change.WorkflowPreset
	if policy, err := governance.ResolvePresetPolicy(root, change); err == nil {
		requiredPreset = policy.EffectivePreset
	}
	return artifact.RequiredArtifactsForChange(resolution.Schema, change.NeedsDiscovery, change.WorkflowPreset, requiredPreset)
}

// DecisionArtifactRequired reports whether decision.md is in the change's
// required artifact set (expanded/custom schemas may require it). Keep callers
// on this shared helper so validate, instructions, and progression gates do not
// drift on preset/schema policy.
func DecisionArtifactRequired(root string, change model.Change) bool {
	return slices.Contains(RequiredArtifactNames(root, change), "decision.md")
}

// GovernedBundleBlockers returns a list of blocker strings for missing governed
// bundle artifacts. Empty slice means the bundle is ready.
func GovernedBundleBlockers(root string, change model.Change) []string {
	resolution := ResolveChangeSchemaDiagnostics(change)

	// Worktree validation is deferred to the dedicated worktree gate (step 5 in
	// AdvanceGoverned) for S2_IMPLEMENT when the worktree is not yet bound. Checking
	// it here would return dedicated_worktree_metadata_required before
	// DeriveWorktreeBlockers has a chance to extract worktree-preflight evidence
	// — creating a deadlock.
	skipWorktreeCheck := change.CurrentState == model.StateS2Implement &&
		change.NeedsDiscovery &&
		change.WorktreePath == ""
	if !skipWorktreeCheck {
		worktreeValidation, err := state.ValidateChangeWorktree(root, change)
		if err != nil {
			return stringutil.UniqueSorted(append(resolution.Blockers, "worktree_validation_error"))
		}
		if len(worktreeValidation.Blockers) > 0 {
			return stringutil.UniqueSorted(append(resolution.Blockers, model.ReasonSpecs(worktreeValidation.Blockers)...))
		}
	}

	requiredPreset := change.WorkflowPreset
	if policy, err := governance.ResolvePresetPolicy(root, change); err == nil {
		requiredPreset = policy.EffectivePreset
	}
	required := artifact.RequiredArtifactsForChange(resolution.Schema, change.NeedsDiscovery, change.WorkflowPreset, requiredPreset)
	base, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return stringutil.UniqueSorted(append(resolution.Blockers, "governed_bundle_path_invalid:"+err.Error()))
	}

	specByName := map[string]artifact.ArtifactSpec{}
	for _, spec := range resolution.Schema {
		specByName[spec.Name] = spec
	}
	for _, name := range required {
		if _, ok := specByName[name]; !ok {
			return []string{"required_artifact_schema_missing:" + name}
		}
	}

	blockers := append([]string{}, resolution.Blockers...)
	for _, name := range required {
		if existenceOwnedByDedicatedGate(name) {
			continue
		}
		path := artifact.ResolveArtifactPath(base, name)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				blockers = append(blockers, "missing_required_artifact:"+name)
				continue
			}
			blockers = append(blockers, "required_artifact_unreadable:"+name)
		}
	}
	return stringutil.UniqueSorted(blockers)
}

// existenceOwnedByDedicatedGate reports whether a required artifact's existence is
// enforced by a dedicated lifecycle gate rather than the generic bundle/readiness
// existence checks. assurance.md is a review/verify-phase deliverable deferred to
// S3_REVIEW authoring (issue #141); its existence and structure are owned solely by
// AssuranceContractBlockers, which fails closed at S3_REVIEW and later. The generic
// gates run before S3 too, so they must skip it — otherwise a deferred (and thus
// absent) assurance.md would surface as missing_required_artifact and strand the
// change at S1_PLAN/S2_IMPLEMENT. Skipping it here also avoids double-reporting the
// same gap at S3+ (assurance_contract_missing is the specific, owning blocker).
func existenceOwnedByDedicatedGate(name string) bool {
	return name == "assurance.md"
}

// AssuranceContractBlockers validates the assurance.md body-first contract:
// all required headings present, in order, with non-empty content, and — via
// the shared artifact.AssuranceStructureBlockers floor — not still template
// scaffold (issue #47). Returns nil before enforcement begins: light effective
// preset (assurance optional), or states earlier than S3_REVIEW. Once enforcing
// at S3_REVIEW and later, a missing file yields assurance_contract_missing and a
// template-only/scaffold body is rejected per-section rather than passing. Unknown
// states fail closed.
func AssuranceContractBlockers(root string, change model.Change) []string {
	// Light effective preset: assurance.md is optional.
	// Uses EffectivePreset so min_preset and guardrail-domain upgrades are respected.
	if policy, err := governance.ResolvePresetPolicy(root, change); err == nil && policy.EffectivePreset == model.WorkflowPresetLight {
		return nil
	}
	switch change.CurrentState {
	case model.StateS0Intake, model.StateS1Plan, model.StateS2Implement:
		return nil
	default:
		// enforce
	}

	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return []string{"assurance_contract_path_invalid:" + err.Error()}
	}
	path := filepath.Join(bundleDir, "assurance.md")
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{"assurance_contract_missing"}
		}
		return []string{"assurance_contract_unreadable"}
	}
	return artifact.AssuranceStructureBlockers(string(raw))
}

// DecisionContractBlockers enforces decision.md substance for changes whose
// schema requires it (the expanded schema). Parallel to the requirements/tasks
// substance gates, an unwritten or template-only decision.md (only the
// <!-- ... --> guidance comments) must not satisfy planning readiness (issue
// #119). Returns nil when decision.md is not a required artifact, before
// plan-audit (so pre-audit drafts stay lenient, mirroring the tasks target_files
// gate), or when the file is absent — a missing required decision.md is owned by
// GovernedBundleBlockers (missing_required_artifact) rather than double-reported.
func DecisionContractBlockers(root string, change model.Change) []string {
	if !DecisionArtifactRequired(root, change) {
		return nil
	}
	// Defer strict substance enforcement to plan-audit and later, mirroring the
	// tasks checklist gate, so pre-audit planning drafts are not blocked early.
	if !isAtOrPastPlanAudit(change) {
		return nil
	}

	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return []string{"decision_contract_path_invalid:" + err.Error()}
	}
	path := filepath.Join(bundleDir, "decision.md")
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return []string{"decision_contract_unreadable"}
	}
	return artifact.DecisionSubstanceBlockers(string(raw))
}

// ResolveChangeSchemaDiagnostics resolves the artifact schema for a change
// using strict, frozen change metadata only.
func ResolveChangeSchemaDiagnostics(change model.Change) ChangeSchemaResolution {
	if change.ArtifactSchema != "" {
		schemaName := ResolveFrozenArtifactSchema(change.ArtifactSchema, "", change.NeedsDiscovery)
		resolution := ChangeSchemaResolution{
			Schema: artifact.ResolveSchema(schemaName, change.CustomArtifacts),
		}
		if schemaName != change.ArtifactSchema {
			resolution.Warnings = []string{"schema_promotion:discovery_requires_expanded"}
		}
		return resolution
	}
	return ChangeSchemaResolution{
		Blockers: []string{"artifact_schema_missing"},
	}
}

// ComputeVerificationReadiness computes whether the single terminal
// ship-verification skill exists with a pass verdict.
func ComputeVerificationReadiness(passingSkills map[string]model.VerificationRecord) bool {
	record, ok := passingSkills[SkillShipVerification]
	if !ok {
		return false
	}
	return record.IsPassing()
}

// ValidatePlanningReadiness is the standalone checker for S1_PLAN.validate.
// It is the sole validator for post-audit recovery; callers at PlanSubStep=validate
// must not wire bundle checks independently — all bundle probes run internally here.
func ValidatePlanningReadiness(root string, change model.Change) model.PlanningValidationResult {
	result := model.PlanningValidationResult{}

	// 1. Bundle completeness (reuses GovernedBundleBlockers internally).
	bundleBlockers := GovernedBundleBlockers(root, change)
	result.Blockers = append(result.Blockers, model.ReasonCodesFromSpecs(bundleBlockers)...)

	// 2. Tasks checklist validation.
	checklistResult := ValidateTasksChecklistDetailed(root, change)
	result.Blockers = append(result.Blockers, model.ReasonCodesFromSpecs(checklistResult.Blockers)...)
	result.Diagnostics = append(result.Diagnostics, checklistResult.Warnings...)

	// 2b. Decision substance (expanded schema): an unwritten or template-only
	// decision.md must not satisfy planning readiness, parallel to the
	// requirements/tasks substance gates (issue #119).
	decisionBlockers := DecisionContractBlockers(root, change)
	result.Blockers = append(result.Blockers, model.ReasonCodesFromSpecs(decisionBlockers)...)

	// 3. Schema diagnostics.
	schemaDiag := ResolveChangeSchemaDiagnostics(change)
	result.Blockers = append(result.Blockers, model.ReasonCodesFromSpecs(schemaDiag.Blockers)...)
	result.Diagnostics = append(result.Diagnostics, schemaDiag.Warnings...)

	// 4. Worktree authenticity (for any bound worktree and any discovery-required
	// active workflow state).
	worktreeValidation, err := state.ValidateChangeWorktree(root, change)
	if err != nil {
		result.Blockers = append(result.Blockers, model.NewReasonCode("worktree_validation_error", ""))
	} else {
		result.Blockers = append(result.Blockers, worktreeValidation.Blockers...)
	}

	result.Blockers = model.NormalizeReasonCodes(result.Blockers)
	return result
}
