package progression

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"gopkg.in/yaml.v3"
)

var errDigestInputsUnavailable = errors.New("evidence digest inputs unavailable")

const suiteResultFileName = "suite-result.yaml"

type skillDigestStampResult struct {
	Blockers []string
}

func reviewDigestSkill(skillName string) bool {
	switch strings.TrimSpace(skillName) {
	case SkillSpecComplianceReview, SkillCodeQualityReview, SkillSecurityReview, SkillIndependentReview:
		return true
	default:
		return false
	}
}

func certifiedSkillInputDigest(
	root string,
	change model.Change,
	skillName string,
	summary *model.ExecutionSummary,
) (model.SkillDigest, error) {
	inputs := map[string]string{}
	switch {
	case strings.TrimSpace(skillName) == SkillPlanAudit:
		if err := addPlanningArtifactInputs(root, change, inputs); err != nil {
			return model.SkillDigest{}, err
		}
	case strings.TrimSpace(skillName) == SkillIntakeClarification:
		if err := addIntakeClarificationIntentInput(root, change, inputs); err != nil {
			return model.SkillDigest{}, err
		}
	case strings.TrimSpace(skillName) == SkillResearchOrchestration:
		if err := addGovernedFileInput(root, change, "intent.md", inputs); err != nil {
			return model.SkillDigest{}, err
		}
		if err := addGovernedFileInput(root, change, "research.md", inputs); err != nil {
			return model.SkillDigest{}, err
		}
	case strings.TrimSpace(skillName) == SkillWaveOrchestration:
		if err := addWaveOrchestrationInputs(root, change, summary, inputs); err != nil {
			return model.SkillDigest{}, err
		}
	case reviewDigestSkill(skillName):
		if err := addReviewSkillInputs(root, change, summary, inputs); err != nil {
			return model.SkillDigest{}, err
		}
	case strings.TrimSpace(skillName) == SkillGoalVerification:
		if err := addGoalVerificationInputs(root, change, summary, inputs); err != nil {
			return model.SkillDigest{}, err
		}
	case strings.TrimSpace(skillName) == SkillFinalCloseout:
		if err := addFinalCloseoutInputs(root, change, summary, inputs); err != nil {
			return model.SkillDigest{}, err
		}
	default:
		if err := addExecutionAndContentInputs(root, change, summary, inputs); err != nil {
			return model.SkillDigest{}, err
		}
	}
	digest := model.SkillDigest{
		Inputs: inputs,
	}
	digest.Normalize()
	return digest, nil
}

// StampEvidenceDigestForSkill records the current engine-owned input digest for an
// accepted passing skill verdict.
func StampEvidenceDigestForSkill(
	root string,
	change model.Change,
	skillName string,
	record model.VerificationRecord,
	summary *model.ExecutionSummary,
) error {
	current, err := certifiedSkillInputDigest(root, change, skillName, summary)
	if err != nil {
		return err
	}
	current.VerdictTimestamp = record.Timestamp

	digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	if err != nil {
		return err
	}
	next := model.EvidenceDigests{
		Version: model.EvidenceDigestsVersion,
		Skills:  map[string]model.SkillDigest{},
	}
	if digests != nil {
		next = *digests
		next.Normalize()
	}
	next.Skills[strings.TrimSpace(skillName)] = current
	return state.SaveEvidenceDigests(root, change.Slug, next)
}

// CheckEvidenceDigestInputsForSkill validates that the current engine-owned
// digest inputs for a skill are available before writing a verification record.
func CheckEvidenceDigestInputsForSkill(
	root string,
	change model.Change,
	skillName string,
	summary *model.ExecutionSummary,
) error {
	_, err := certifiedSkillInputDigest(root, change, skillName, summary)
	return err
}

// PruneEvidenceDigestForSkill removes a skill's digest entry when the
// verification record no longer represents an accepted passing verdict.
func PruneEvidenceDigestForSkill(root string, change model.Change, skillName string) error {
	return pruneEvidenceDigestForSkill(root, change, skillName)
}

// pruneEvidenceDigestForSkill removes a skill's entry from evidence-digests.yaml
// when present. It is the digest half of removing a verification record, so a
// digest entry never outlives the record it certifies.
func pruneEvidenceDigestForSkill(root string, change model.Change, skillName string) error {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return nil
	}
	digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	if err != nil {
		return err
	}
	if digests == nil {
		return nil
	}
	digests.Normalize()
	if _, ok := digests.Skills[skillName]; !ok {
		return nil
	}
	delete(digests.Skills, skillName)
	return state.SaveEvidenceDigests(root, change.Slug, *digests)
}

func skillDigestFreshnessBlockers(
	root string,
	change model.Change,
	skillName string,
) ([]string, error) {
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return nil, err
	}
	return skillDigestFreshnessBlockersWithSummary(root, change, skillName, summary)
}

func skillDigestFreshnessBlockersWithSummary(
	root string,
	change model.Change,
	skillName string,
	summary *model.ExecutionSummary,
) ([]string, error) {
	digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	if err != nil {
		return nil, err
	}
	stored, hasStored := model.SkillDigest{}, false
	if digests != nil {
		stored, hasStored = digests.Skills[strings.TrimSpace(skillName)]
	}

	current, currentErr := certifiedSkillInputDigest(root, change, skillName, summary)
	if currentErr != nil {
		return []string{skillDigestInputUnavailableBlocker(skillName)}, nil
	}

	if !hasStored {
		return nil, nil
	}
	fresh, changed := model.EvidenceFreshness(stored, current.Inputs)
	if fresh {
		return nil, nil
	}
	return staleSkillDigestBlockers(skillName, changed), nil
}

func reviewerDigestFreshAtCycle(
	root string,
	change model.Change,
	skillName string,
	cycleRunVersion int,
	summary *model.ExecutionSummary,
) ([]string, error) {
	if cycleRunVersion < 1 {
		return []string{"input_digest_unavailable"}, nil
	}
	current, err := certifiedSkillInputDigest(root, change, skillName, summary)
	if err != nil {
		if digestStampUnavailable(err) {
			return []string{"input_digest_unavailable"}, nil
		}
		return nil, err
	}
	digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	if err != nil {
		return nil, err
	}
	if digests == nil {
		return nil, nil
	}
	digests.Normalize()
	stored, ok := digests.Skills[strings.TrimSpace(skillName)]
	if !ok {
		return nil, nil
	}
	fresh, changed := model.EvidenceFreshness(stored, current.Inputs)
	if fresh {
		return nil, nil
	}
	return changed, nil
}

func reviewerDigestCycleRunVersion(root string, change model.Change, summary *model.ExecutionSummary) (int, error) {
	result, ok, err := loadSuiteResultForChange(root, change)
	if err != nil {
		return 0, err
	}
	if ok {
		return result.RunSummaryVersion, nil
	}
	if requiresSuiteResultForSharedReviewerInputs(change) {
		return 0, nil
	}
	if state.ExecutionSummaryReady(summary) {
		return summary.RunSummaryVersion, nil
	}
	return 0, nil
}

func loadSuiteResultForChange(root string, change model.Change) (model.SuiteResult, bool, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return model.SuiteResult{}, false, err
	}
	path := filepath.Join(paths.GovernedBundleDir, "verification", suiteResultFileName)
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from governed bundle authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return model.SuiteResult{}, false, nil
		}
		return model.SuiteResult{}, false, err
	}
	var result model.SuiteResult
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return model.SuiteResult{}, false, fmt.Errorf("parse %s: %w", suiteResultFileName, err)
	}
	result.Normalize()
	if err := result.Validate(); err != nil {
		return model.SuiteResult{}, false, fmt.Errorf("invalid %s: %w", suiteResultFileName, err)
	}
	return result, true, nil
}

func stampPassingSkillDigests(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
) (skillDigestStampResult, error) {
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return skillDigestStampResult{}, err
	}
	existingDigests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	if err != nil {
		return skillDigestStampResult{}, err
	}
	stampSkills, err := passingAndPreviouslyAcceptedSkillRecords(root, change, passingSkills)
	if err != nil {
		return skillDigestStampResult{}, err
	}
	directPassing := map[string]bool{}
	for skillName, record := range passingSkills {
		skillName = strings.TrimSpace(skillName)
		if skillName != "" && record.IsPassing() {
			directPassing[skillName] = true
		}
	}
	if len(stampSkills) == 0 {
		return skillDigestStampResult{}, nil
	}
	var result skillDigestStampResult
	for skillName, record := range stampSkills {
		skillName = strings.TrimSpace(skillName)
		if !record.IsPassing() {
			continue
		}
		if !directPassing[skillName] {
			if blockers, err := previouslyAcceptedSkillDigestBlockers(root, change, skillName, summary, existingDigests); err != nil {
				return skillDigestStampResult{}, err
			} else if len(blockers) > 0 {
				result.Blockers = append(result.Blockers, blockers...)
				continue
			}
		}
		if err := StampEvidenceDigestForSkill(root, change, skillName, record, summary); err != nil {
			if digestStampUnavailable(err) {
				if directPassing[skillName] {
					result.Blockers = append(result.Blockers, skillDigestInputUnavailableBlocker(skillName))
				}
				continue
			}
			return skillDigestStampResult{}, err
		}
	}
	result.Blockers = stringutil.UniqueSorted(result.Blockers)
	return result, nil
}

func previouslyAcceptedSkillDigestBlockers(
	root string,
	change model.Change,
	skillName string,
	summary *model.ExecutionSummary,
	existingDigests *model.EvidenceDigests,
) ([]string, error) {
	if existingDigests == nil {
		return nil, nil
	}
	existingDigests.Normalize()
	stored, ok := existingDigests.Skills[strings.TrimSpace(skillName)]
	if !ok {
		return nil, nil
	}
	current, err := certifiedSkillInputDigest(root, change, skillName, summary)
	if err != nil {
		if digestStampUnavailable(err) {
			return nil, nil
		}
		return nil, err
	}
	fresh, changed := model.EvidenceFreshness(stored, current.Inputs)
	if fresh {
		return nil, nil
	}
	return staleSkillDigestBlockers(skillName, changed), nil
}

func passingAndPreviouslyAcceptedSkillRecords(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
) (map[string]model.VerificationRecord, error) {
	out := map[string]model.VerificationRecord{}
	for skillName, record := range passingSkills {
		skillName = strings.TrimSpace(skillName)
		if skillName == "" || !record.IsPassing() {
			continue
		}
		out[skillName] = record
	}

	recorded, err := recordedSkillEvidenceSet(root, change)
	if err != nil {
		return nil, err
	}
	if len(recorded) == 0 {
		return out, nil
	}
	verifications, err := state.ListVerificationsForChange(root, change)
	if err != nil {
		return nil, err
	}
	currentPosition := currentStaleEvidencePosition(change)
	recordedPositions, err := recordedSkillEvidencePositions(root)
	if err != nil {
		return nil, err
	}
	for skillName, record := range verifications {
		skillName = strings.TrimSpace(skillName)
		if skillName == SkillIntakeClarification {
			// Intake consumes intent.md while that file is expected to evolve during
			// clarification; later planning/research evidence owns the durable
			// intent freshness boundary.
			continue
		}
		if skillName == "" || !recorded[skillName] || !record.IsPassing() {
			continue
		}
		position, ok := recordedPositions[skillName]
		if ok && staleEvidenceRepairDeferredToReview(change, EvidenceRepairTarget{
			SkillName:   skillName,
			State:       position.workflowState(),
			PlanSubStep: position.planSubStep(),
		}) {
			continue
		}
		if ok && compareStaleEvidencePosition(position, currentPosition) > 0 {
			continue
		}
		if _, exists := out[skillName]; !exists {
			out[skillName] = record
		}
	}
	return out, nil
}

func recordedSkillEvidencePositions(root string) (map[string]staleEvidencePosition, error) {
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return nil, err
	}
	positions := map[string]staleEvidencePosition{}
	for _, def := range registry {
		position, ok := staleEvidencePositionForDefinition(def)
		if !ok {
			continue
		}
		positions[strings.TrimSpace(def.Name)] = position
	}
	return positions, nil
}

func recordedSkillEvidenceSet(root string, change model.Change) (map[string]bool, error) {
	events, err := state.ReadLifecycleEvents(root, change)
	if err != nil {
		return nil, err
	}
	recorded := map[string]bool{}
	for _, event := range events {
		if event.EventType != "skill.evidence_recorded" || event.Result != "recorded" {
			continue
		}
		skillName := strings.TrimSpace(event.SkillID)
		if skillName != "" {
			recorded[skillName] = true
		}
	}
	return recorded, nil
}

func digestStampUnavailable(err error) bool {
	return errors.Is(err, errDigestInputsUnavailable) || errors.Is(err, fs.ErrNotExist)
}

func staleSkillDigestBlockers(skillName string, changed []string) []string {
	changed = stringutil.UniqueSorted(changed)
	blockers := make([]string, 0, len(changed))
	for _, name := range changed {
		blockers = append(blockers, "required_skill_stale:"+strings.TrimSpace(skillName)+":"+skillDigestBlockerInputName(name))
	}
	return stringutil.UniqueSorted(blockers)
}

func skillDigestBlockerInputName(name string) string {
	name = strings.TrimSpace(name)
	if name == "tasks.md:scope" {
		return "tasks.md"
	}
	return name
}

func skillDigestInputUnavailableBlocker(skillName string) string {
	return "required_skill_stale:" + strings.TrimSpace(skillName) + ":input_digest_unavailable"
}

func addPlanningArtifactInputs(root string, change model.Change, inputs map[string]string) error {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	seenAny := false
	// assurance.md is not a plan-audit input: it is deferred to S3_REVIEW authoring
	// (issue #141) and does not exist at plan-audit time. It remains an input to the
	// final-closeout digest.
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md"} {
		path := filepath.Join(bundleDir, rel)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		seenAny = true
		if err := addGovernedFileInput(root, change, rel, inputs); err != nil {
			return err
		}
	}
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	raw, err := os.ReadFile(tasksPath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !seenAny {
			return errDigestInputsUnavailable
		}
		return err
	}
	hash, err := wave.TaskPlanStructuralHash(string(raw))
	if err != nil {
		return fmt.Errorf("hash tasks.md structurally: %w", err)
	}
	inputs["tasks.md"] = hash
	return nil
}

func addTaskPlanScopeInput(root string, change model.Change, inputs map[string]string) error {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md")) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return err
	}
	hash, err := wave.TaskPlanScopeHash(string(raw))
	if err != nil {
		return fmt.Errorf("hash tasks.md scope: %w", err)
	}
	inputs["tasks.md:scope"] = hash
	return nil
}

func addGovernedFileInput(root string, change model.Change, rel string, inputs map[string]string) error {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	path := filepath.Join(bundleDir, rel)
	hash, err := computeProseFileInputHash(path)
	if err != nil {
		return err
	}
	inputs[filepath.ToSlash(rel)] = hash
	return nil
}

func addIntakeClarificationIntentInput(root string, change model.Change, inputs map[string]string) error {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	path := filepath.Join(bundleDir, "intent.md")
	hash, err := computeFilteredProseFileInputHash(path, func(heading string) bool {
		// Open Questions are research-routing state. Intake evidence owns the
		// clarified scope text, so resolving research questions must not request
		// an S0 repair.
		return !strings.EqualFold(strings.TrimSpace(heading), "Open Questions")
	})
	if err != nil {
		return err
	}
	inputs["intent.md"] = hash
	return nil
}

func computeProseFileInputHash(path string) (string, error) {
	return computeFilteredProseFileInputHash(path, nil)
}

func computeFilteredProseFileInputHash(path string, includeSection func(heading string) bool) (string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return "", err
	}
	sections := proseArtifactMaterialSections(filepath.Base(path), string(raw))
	if includeSection != nil {
		filtered := make([]map[string]string, 0, len(sections))
		for _, section := range sections {
			if includeSection(section["heading"]) {
				filtered = append(filtered, section)
			}
		}
		sections = filtered
	}
	return model.ComputeInputHash(map[string]any{
		"artifact":          filepath.Base(path),
		"material_sections": sections,
	})
}

func proseArtifactMaterialSections(name, content string) []map[string]string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = stringutil.StripHTMLComments(content)

	type proseSection struct {
		heading string
		lines   []string
	}

	sections := []proseSection{{}}
	for _, line := range strings.Split(content, "\n") {
		if heading, ok := proseArtifactSectionHeading(line); ok {
			sections = append(sections, proseSection{heading: heading})
			continue
		}
		last := len(sections) - 1
		sections[last].lines = append(sections[last].lines, line)
	}

	material := make([]map[string]string, 0, len(sections))
	for _, section := range sections {
		body := normalizeProseArtifactSectionBody(section.lines)
		if body == "" || proseArtifactKnownDefault(name, section.heading, body) {
			continue
		}
		entry := map[string]string{"body": body}
		if section.heading != "" {
			entry["heading"] = section.heading
		}
		material = append(material, entry)
	}
	return material
}

func proseArtifactSectionHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "## ") {
		return "", false
	}
	heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
	return heading, heading != ""
}

func normalizeProseArtifactSectionBody(lines []string) string {
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized = append(normalized, strings.TrimRight(line, " \t"))
	}
	for len(normalized) > 0 && strings.TrimSpace(normalized[0]) == "" {
		normalized = normalized[1:]
	}
	for len(normalized) > 0 && strings.TrimSpace(normalized[len(normalized)-1]) == "" {
		normalized = normalized[:len(normalized)-1]
	}
	if len(normalized) == 1 && strings.HasPrefix(strings.TrimSpace(normalized[0]), "# ") {
		return ""
	}
	return strings.Join(normalized, "\n")
}

func proseArtifactKnownDefault(name, heading, body string) bool {
	defaultsByHeading := proseArtifactKnownDefaults[strings.TrimSpace(name)]
	if len(defaultsByHeading) == 0 {
		return false
	}
	defaults := defaultsByHeading[strings.ToLower(strings.TrimSpace(heading))]
	if len(defaults) == 0 {
		return false
	}
	normalizedBody := normalizeKnownProseDefault(body)
	for _, known := range defaults {
		if normalizedBody == normalizeKnownProseDefault(known) {
			return true
		}
	}
	return false
}

func normalizeKnownProseDefault(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

var proseArtifactKnownDefaults = map[string]map[string][]string{
	"intent.md": {
		"summary": {
			"Describe the change objective.",
		},
	},
}

func addExecutionAndContentInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
) error {
	if summary == nil {
		loaded, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil {
			return err
		}
		summary = loaded
	}
	if err := addExecutionSummaryInputs(summary, inputs); err != nil {
		return err
	}
	if err := addContentPathInputs(root, change, summary, inputs, false); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errDigestInputsUnavailable
	}
	return nil
}

func addExecutionSummaryInputs(summary *model.ExecutionSummary, inputs map[string]string) error {
	if state.ExecutionSummaryReady(summary) {
		semanticSummary := *summary
		semanticSummary.CapturedAt = time.Time{}
		for i := range semanticSummary.Tasks {
			semanticSummary.Tasks[i].CapturedAt = time.Time{}
		}
		hash, err := model.ComputeInputHash(map[string]any{
			"execution_summary": semanticSummary,
		})
		if err != nil {
			return err
		}
		inputs["execution-summary.yaml"] = hash
		inputs["run_summary_version"], err = model.ComputeInputHash(map[string]any{
			"run_summary_version": summary.RunSummaryVersion,
		})
		if err != nil {
			return err
		}
		if strings.TrimSpace(summary.TasksPlanHash) != "" {
			inputs["tasks_plan_hash"] = strings.TrimSpace(summary.TasksPlanHash)
		}
	}
	return nil
}

func addContentPathInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
	normalizeCurrentChangeAuthority bool,
) error {
	if summary == nil {
		return nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}
	for _, rel := range proofReuseContentPaths(change, summary) {
		candidates, ok, err := proofReuseWorkspacePaths(paths.WorkspaceRoot, rel)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				hash, hashErr := deletedFileInputHash(rel)
				if hashErr != nil {
					return hashErr
				}
				inputs[filepath.ToSlash(strings.TrimSpace(rel))] = hash
				continue
			}
			return err
		}
		if !ok {
			return fmt.Errorf("content path must be workspace-relative: %s", rel)
		}
		for _, path := range candidates {
			key := filepath.ToSlash(rel)
			if strings.ContainsAny(rel, "*?[") {
				display, displayErr := filepath.Rel(paths.WorkspaceRoot, path)
				if displayErr == nil {
					key = filepath.ToSlash(display)
				}
			}
			if proofReuseSkipsContentPath(change, key) {
				continue
			}
			var hash string
			var err error
			if normalizeCurrentChangeAuthority {
				hash, err = goalVerificationContentPathInputHash(change, path, key)
			} else {
				hash, err = workspacePathInputHash(path, key)
			}
			if err != nil {
				return err
			}
			inputs[key] = hash
		}
	}
	return nil
}

func addGoalVerificationInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
) error {
	if summary == nil {
		loaded, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil {
			return err
		}
		summary = loaded
	}
	if !state.ExecutionSummaryReady(summary) {
		return errDigestInputsUnavailable
	}
	if err := addSharedReviewerInputs(
		root,
		change,
		summary,
		inputs,
		requiresSuiteResultForSharedReviewerInputs(change),
	); err != nil {
		return err
	}
	if err := addPlanningArtifactInputs(root, change, inputs); err != nil {
		return err
	}
	if err := addTaskPlanScopeInput(root, change, inputs); err != nil {
		return err
	}
	if err := addChangedTargetFileSetInput(change, summary, inputs); err != nil {
		return err
	}
	if err := addContentPathInputs(root, change, summary, inputs, true); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errDigestInputsUnavailable
	}
	return nil
}

func addChangedTargetFileSetInput(change model.Change, summary *model.ExecutionSummary, inputs map[string]string) error {
	contentPaths := proofReuseContentPaths(change, summary)
	hash, err := model.ComputeInputHash(map[string]any{
		"changed_target_files": contentPaths,
	})
	if err != nil {
		return err
	}
	inputs["changed_target_files"] = hash
	return nil
}

func addFinalCloseoutInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
) error {
	if summary == nil {
		loaded, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil {
			return err
		}
		summary = loaded
	}
	if !state.ExecutionSummaryReady(summary) {
		return errDigestInputsUnavailable
	}
	if err := addSharedReviewerInputs(root, change, summary, inputs, false); err != nil {
		return err
	}
	if err := addChangedTargetFileSetInput(change, summary, inputs); err != nil {
		return err
	}
	if err := addContentPathInputs(root, change, summary, inputs, true); err != nil {
		return err
	}
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return err
	}
	assurancePath := filepath.Join(bundleDir, "assurance.md")
	if _, err := os.Stat(assurancePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := addGovernedFileInput(root, change, "assurance.md", inputs); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errDigestInputsUnavailable
	}
	return nil
}

func addReviewSkillInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
) error {
	if summary == nil {
		loaded, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil {
			return err
		}
		summary = loaded
	}
	if err := addSharedReviewerInputs(
		root,
		change,
		summary,
		inputs,
		requiresSuiteResultForSharedReviewerInputs(change),
	); err != nil {
		return err
	}
	if err := addPlanningArtifactInputs(root, change, inputs); err != nil {
		return err
	}
	if err := addTaskPlanScopeInput(root, change, inputs); err != nil {
		return err
	}
	if err := addReviewerSpecificInputs(root, change, summary, inputs); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errDigestInputsUnavailable
	}
	return nil
}

func addSharedReviewerInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
	requireSuiteResult bool,
) error {
	result, ok, err := loadSuiteResultForChange(root, change)
	if err != nil {
		return err
	}
	if ok {
		if state.ExecutionSummaryReady(summary) && result.RunSummaryVersion != summary.RunSummaryVersion {
			return fmt.Errorf(
				"%s run_summary_version mismatch: got=%d want=%d",
				suiteResultFileName,
				result.RunSummaryVersion,
				summary.RunSummaryVersion,
			)
		}
		if err := requireGuardrailSuiteResultSASTDigest(change, result); err != nil {
			return err
		}
		shared, err := result.SharedReviewerInputDigests()
		if err != nil {
			return err
		}
		for name, digest := range shared {
			inputs[name] = digest
		}
		return nil
	}
	if requireSuiteResult {
		return errDigestInputsUnavailable
	}
	return addExecutionSummaryInputs(summary, inputs)
}

func requireGuardrailSuiteResultSASTDigest(change model.Change, result model.SuiteResult) error {
	required := requiredGuardrailSuiteResultSASTDigestName(change)
	if required == "" {
		return nil
	}
	result.Normalize()
	if strings.TrimSpace(result.SASTDigests[required]) == "" {
		return errDigestInputsUnavailable
	}
	return nil
}

func requiredGuardrailSuiteResultSASTDigestName(change model.Change) string {
	domain := strings.TrimSpace(change.GuardrailDomain)
	if domain == "" {
		return ""
	}
	return domain + ".safety_baseline"
}

func requiresSuiteResultForSharedReviewerInputs(change model.Change) bool {
	return change.CurrentState == model.StateS3Review
}

func addReviewerSpecificInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
) error {
	if err := addReviewSummaryContentInputs(root, change, summary, inputs); err != nil {
		return err
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}
	for _, rel := range reviewWorkspaceInputPaths(paths) {
		if err := addWorkspaceFileInput(paths.WorkspaceRoot, rel, inputs); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				hash, hashErr := deletedFileInputHash(rel)
				if hashErr != nil {
					return hashErr
				}
				inputs[rel] = hash
				continue
			}
			return err
		}
	}
	return nil
}

func addReviewSummaryContentInputs(root string, change model.Change, summary *model.ExecutionSummary, inputs map[string]string) error {
	if summary == nil {
		return nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}
	for _, rel := range proofReuseContentPaths(change, summary) {
		rel = filepath.ToSlash(strings.TrimSpace(rel))
		if rel == "" || reviewInputPathExcluded(rel) {
			continue
		}
		if !strings.ContainsAny(rel, "*?[") {
			if err := addWorkspaceFileInput(paths.WorkspaceRoot, rel, inputs); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					hash, hashErr := deletedFileInputHash(rel)
					if hashErr != nil {
						return hashErr
					}
					inputs[rel] = hash
					continue
				}
				return err
			}
			continue
		}
		candidates, ok, err := proofReuseWorkspacePaths(paths.WorkspaceRoot, rel)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("content path must be workspace-relative: %s", rel)
		}
		for _, path := range candidates {
			key := rel
			if strings.ContainsAny(rel, "*?[") {
				display, displayErr := filepath.Rel(paths.WorkspaceRoot, path)
				if displayErr == nil {
					key = filepath.ToSlash(display)
				}
			}
			hash, err := workspacePathInputHash(path, key)
			if err != nil {
				return err
			}
			inputs[key] = hash
		}
	}
	return nil
}

func addWaveOrchestrationInputs(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	inputs map[string]string,
) error {
	plan, err := state.LoadWavePlanForChange(root, change)
	if err != nil {
		return err
	}
	plan.GeneratedAt = time.Time{}
	plan.Normalize()
	planHash, err := model.ComputeInputHash(map[string]any{
		"wave_plan": plan,
	})
	if err != nil {
		return err
	}
	inputs["wave-plan.yaml"] = planHash

	if summary == nil {
		loaded, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil {
			return err
		}
		summary = loaded
	}
	runVersion := 0
	if state.ExecutionSummaryReady(summary) {
		runVersion = summary.RunSummaryVersion
	} else {
		runVersion, err = LatestTaskEvidenceRunVersion(root, change.Slug)
		if err != nil {
			return err
		}
	}
	if runVersion < 1 {
		return errDigestInputsUnavailable
	}
	tasks, issues, err := LoadExecutionTasksFromEvidence(root, change.Slug, runVersion)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return fmt.Errorf("load runtime task evidence for wave digest: %s", strings.Join(issues, "; "))
	}
	if len(tasks) == 0 {
		return errDigestInputsUnavailable
	}
	taskHash, err := model.ComputeInputHash(map[string]any{
		"run_summary_version": runVersion,
		"tasks":               tasks,
	})
	if err != nil {
		return err
	}
	inputs["runtime_task_evidence"] = taskHash
	return nil
}

// LatestTaskEvidenceRunVersion returns the highest run_summary_version present
// in runtime task evidence. Wave-orchestration uses this before it can produce
// execution-summary.yaml.
func LatestTaskEvidenceRunVersion(root, slug string) (int, error) {
	dir := state.EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	latest := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		runVersion, err := taskEvidenceRunVersion(path)
		if err != nil {
			return 0, fmt.Errorf("load task evidence run version from %s: %w", state.DisplayPath(root, path), err)
		}
		if runVersion > latest {
			latest = runVersion
		}
	}
	return latest, nil
}

func reviewWorkspaceInputPaths(paths state.ResolvedChangePaths) []string {
	workspaceRoot := strings.TrimSpace(paths.WorkspaceRoot)
	if workspaceRoot == "" {
		return nil
	}
	files, gitMarkers := reviewWorkspaceGitNameOnly(workspaceRoot)
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		rel := filepath.ToSlash(strings.TrimSpace(file))
		rel = strings.TrimPrefix(rel, "./")
		if rel == "" || reviewInputPathExcluded(rel) {
			continue
		}
		filtered = append(filtered, rel)
	}
	filtered = append(filtered, gitMarkers...)
	return stringutil.UniqueSorted(filtered)
}

func reviewWorkspaceGitNameOnly(workspaceRoot string) ([]string, []string) {
	var files []string
	var markers []string
	if diffFiles, err := gitNameOnlyResult(workspaceRoot, "diff", "--name-only", "HEAD", "--"); err == nil {
		files = append(files, diffFiles...)
	} else {
		markers = append(markers, "__git_error__/diff_name_only")
	}
	if otherFiles, err := gitNameOnlyResult(workspaceRoot, "ls-files", "--others", "--exclude-standard"); err == nil {
		files = append(files, otherFiles...)
	} else {
		markers = append(markers, "__git_error__/ls_files_others")
	}
	return files, markers
}

func reviewInputPathExcluded(rel string) bool {
	rel = strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	if rel == "" || strings.HasPrefix(rel, ".git/") {
		return true
	}
	if rel == "artifacts/changes" || strings.HasPrefix(rel, "artifacts/changes/") {
		return true
	}
	if strings.HasPrefix(rel, ".slipway/") {
		return true
	}
	return false
}

func addWorkspaceFileInput(workspaceRoot, rel string, inputs map[string]string) error {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" {
		return nil
	}
	path := filepath.Join(workspaceRoot, filepath.FromSlash(rel))
	hash, err := workspacePathInputHash(path, rel)
	if err != nil {
		return err
	}
	inputs[rel] = hash
	return nil
}

func workspacePathInputHash(path, rel string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return deletedFileInputHash(rel)
		}
		return "", err
	}
	if !info.IsDir() {
		return model.ComputeFileContentHash(path)
	}
	files := map[string]string{}
	if err := filepath.WalkDir(path, func(child string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		display, err := filepath.Rel(path, child)
		if err != nil {
			return err
		}
		hash, err := model.ComputeFileContentHash(child)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(display)] = hash
		return nil
	}); err != nil {
		return "", err
	}
	return model.ComputeInputHash(map[string]any{
		"path":  filepath.ToSlash(strings.TrimSpace(rel)),
		"type":  "directory",
		"files": files,
	})
}

func goalVerificationContentPathInputHash(change model.Change, path, rel string) (string, error) {
	if currentChangeAuthorityInput(change, rel) {
		return changeAuthorityInputHash(path, rel)
	}
	return workspacePathInputHash(path, rel)
}

func currentChangeAuthorityInput(change model.Change, rel string) bool {
	rel = strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	if rel == "" || strings.TrimSpace(change.Slug) == "" {
		return false
	}
	want := filepath.ToSlash(filepath.Join("artifacts", "changes", change.Slug, "change.yaml"))
	return rel == want
}

func changeAuthorityInputHash(path, rel string) (string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return "", err
	}
	var authority model.Change
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&authority); err != nil {
		return "", err
	}
	authority.Normalize()
	if err := authority.Validate(); err != nil {
		return "", err
	}
	authority = normalizeChangeAuthorityDigestInput(authority)
	return model.ComputeInputHash(map[string]any{
		"path":             filepath.ToSlash(strings.TrimSpace(rel)),
		"change_authority": authority,
	})
}

func normalizeChangeAuthorityDigestInput(authority model.Change) model.Change {
	authority.EvidenceRefs = nil
	authority.Artifacts = nil
	authority.LastAutoPassedStates = nil
	authority.ReviewIntentDriftFailures = 0
	authority.InterruptedExecutionAt = time.Time{}
	return authority
}

func deletedFileInputHash(rel string) (string, error) {
	return model.ComputeInputHash(map[string]any{
		"path":    filepath.ToSlash(strings.TrimSpace(rel)),
		"deleted": true,
	})
}
