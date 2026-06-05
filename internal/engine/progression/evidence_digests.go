package progression

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

var errDigestInputsUnavailable = errors.New("evidence digest inputs unavailable")

const digestBackfilledFromLegacyVerdictEvent = "digest_backfilled_from_legacy_verdict"

type skillDigestStampResult struct {
	BackfilledSkills []string
	Blockers         []string
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
		if err := addGovernedFileInput(root, change, "intent.md", inputs); err != nil {
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
	return stampEvidenceDigestForSkill(root, change, skillName, record, summary)
}

func stampEvidenceDigestForSkill(
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
	current.RunVersion = record.RunVersion
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

// Tier-0 evidence-restamp refusal reasons.
const (
	EvidenceRestampReasonNoEvidence        = "no_passing_evidence"
	EvidenceRestampReasonVerdictNotPassing = "verdict_not_passing"
	EvidenceRestampReasonInputsChanged     = "inputs_changed_after_verdict"
	EvidenceRestampReasonInputsUnavailable = "input_digest_unavailable"
)

// EvidenceRestampOutcome reports the result of a Tier-0 evidence-restamp attempt.
type EvidenceRestampOutcome struct {
	Skill         string   // skill whose digest was assessed
	Eligible      bool     // Tier-0 safe: verdict passing + inputs unchanged after verdict
	Stamped       bool     // digest was written (always false in dry-run)
	DryRun        bool     // assessment only; no state mutated
	Reason        string   // refusal reason, set when !Eligible
	ChangedInputs []string // inputs that changed after the verdict, when refused for drift
	RerunSkill    string   // host skill to re-run when not eligible
}

// RestampEvidenceDigestTier0 records the engine-owned input digest for a skill
// when (and only when) the recorded verdict is still passing and the certified
// inputs did not change after the verdict — Tier 0, provably safe. It never
// fabricates a pass: a missing or non-passing verdict, or any input changed
// after the verdict, is refused, and the caller is told which host skill to
// re-run. With dryRun the eligibility is assessed without writing the digest.
func RestampEvidenceDigestTier0(root string, change model.Change, skillName string, dryRun bool) (EvidenceRestampOutcome, error) {
	skillName = strings.TrimSpace(skillName)
	outcome := EvidenceRestampOutcome{Skill: skillName, DryRun: dryRun, RerunSkill: skillName}
	if skillName == "" {
		return outcome, fmt.Errorf("skill name is required")
	}
	record, err := state.LoadVerification(root, change.Slug, skillName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			outcome.Reason = EvidenceRestampReasonNoEvidence
			return outcome, nil
		}
		return outcome, err
	}
	if !record.IsPassing() {
		outcome.Reason = EvidenceRestampReasonVerdictNotPassing
		return outcome, nil
	}
	if err := ensureDigestInputsAvailableForRestamp(root, change, skillName); err != nil {
		if digestStampUnavailable(err) {
			outcome.Reason = EvidenceRestampReasonInputsUnavailable
			return outcome, nil
		}
		return outcome, err
	}
	// Tier-0 safety: refuse if any certified input changed after the verdict.
	changedAfterVerdict, err := digestInputsChangedAfterVerdict(root, change, skillName, record.Timestamp)
	if err != nil {
		if digestStampUnavailable(err) {
			outcome.Reason = EvidenceRestampReasonInputsUnavailable
			return outcome, nil
		}
		return outcome, err
	}
	if len(changedAfterVerdict) > 0 {
		outcome.Reason = EvidenceRestampReasonInputsChanged
		outcome.ChangedInputs = changedAfterVerdict
		return outcome, nil
	}
	outcome.Eligible = true
	if dryRun {
		return outcome, nil
	}
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return outcome, err
	}
	if err := stampEvidenceDigestForSkill(root, change, skillName, record, summary); err != nil {
		if digestStampUnavailable(err) {
			outcome.Eligible = false
			outcome.Reason = EvidenceRestampReasonInputsUnavailable
			return outcome, nil
		}
		return outcome, err
	}
	outcome.Stamped = true
	return outcome, nil
}

func ensureDigestInputsAvailableForRestamp(root string, change model.Change, skillName string) error {
	_, err := digestInputArtifactPaths(root, change, skillName)
	return err
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
	record model.VerificationRecord,
) ([]string, error) {
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return nil, err
	}
	return skillDigestFreshnessBlockersWithSummary(root, change, skillName, record, summary)
}

func skillDigestFreshnessBlockersWithSummary(
	root string,
	change model.Change,
	skillName string,
	record model.VerificationRecord,
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
		// No stored digest entry yet. If the skill was never recorded and a digest
		// file already exists for other skills, there is nothing to be stale about.
		if digests != nil {
			recorded, err := lifecycleHasRecordedSkillEvidence(root, change, skillName)
			if err != nil {
				return nil, err
			}
			if !recorded {
				return nil, nil
			}
		}
		// Tier-0: a recorded (or legacy file-absent) orphan is healable when its
		// certified inputs did not change after the verdict; only report stale, with
		// the specific changed inputs, when they genuinely did. Unifying the
		// digests==nil and digests!=nil branches here means a missing digest entry no
		// longer deadlocks on the generic input_digest_missing token.
		changedAfterVerdict, err := digestInputsChangedAfterVerdict(root, change, skillName, record.Timestamp)
		if err != nil {
			return nil, err
		}
		if len(changedAfterVerdict) > 0 {
			return staleSkillDigestBlockers(skillName, changedAfterVerdict), nil
		}
		return nil, nil
	}

	if stored.RunVersion != record.RunVersion {
		return []string{"required_skill_stale:" + skillName + ":run_version"}, nil
	}
	fresh, changed := model.EvidenceFreshness(stored, current.Inputs)
	if fresh {
		return nil, nil
	}
	if skillDigestRecordRefreshedAfterStored(record, stored) {
		changedAfterVerdict, err := digestSelectedInputsChangedAfterVerdict(root, change, skillName, record.Timestamp, changed)
		if err != nil {
			return nil, err
		}
		if len(changedAfterVerdict) == 0 {
			return nil, nil
		}
		return staleSkillDigestBlockers(skillName, changedAfterVerdict), nil
	}
	return staleSkillDigestBlockers(skillName, changed), nil
}

func skillDigestRecordRefreshedAfterStored(record model.VerificationRecord, stored model.SkillDigest) bool {
	if record.Timestamp.IsZero() {
		return false
	}
	if stored.VerdictTimestamp.IsZero() {
		return true
	}
	return record.Timestamp.UTC().After(stored.VerdictTimestamp.UTC())
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
	legacyDigestFileAbsent := existingDigests == nil
	var result skillDigestStampResult
	for skillName, record := range stampSkills {
		skillName = strings.TrimSpace(skillName)
		if !record.IsPassing() {
			continue
		}
		backfill, err := legacyDigestBackfillEventRequired(root, change, skillName, record, legacyDigestFileAbsent)
		if err != nil {
			return skillDigestStampResult{}, err
		}
		if !directPassing[skillName] {
			digestChecked := hasStoredSkillDigest(existingDigests, skillName)
			if blockers, err := previouslyAcceptedSkillDigestBlockers(root, change, skillName, record, summary, existingDigests); err != nil {
				return skillDigestStampResult{}, err
			} else if len(blockers) > 0 {
				result.Blockers = append(result.Blockers, blockers...)
				continue
			}
			if recorded, err := lifecycleHasRecordedSkillEvidence(root, change, skillName); err != nil {
				return skillDigestStampResult{}, err
			} else if recorded && !digestChecked {
				// Tier-0 backfill: a recorded orphan with no digest entry is healable
				// when its certified inputs did not change after the verdict; only block,
				// with the specific changed inputs, when they genuinely did. This applies
				// whether or not a digest file already exists for other skills, so a
				// missing entry no longer deadlocks on input_digest_missing.
				changedAfterVerdict, err := digestInputsChangedAfterVerdict(root, change, skillName, record.Timestamp)
				if err != nil {
					if digestStampUnavailable(err) {
						continue
					}
					return skillDigestStampResult{}, err
				}
				if len(changedAfterVerdict) > 0 {
					result.Blockers = append(result.Blockers, staleSkillDigestBlockers(skillName, changedAfterVerdict)...)
					continue
				}
			}
		}
		if err := stampEvidenceDigestForSkill(root, change, skillName, record, summary); err != nil {
			if digestStampUnavailable(err) {
				if directPassing[skillName] {
					result.Blockers = append(result.Blockers, skillDigestInputUnavailableBlocker(skillName))
				}
				continue
			}
			return skillDigestStampResult{}, err
		}
		if backfill {
			result.BackfilledSkills = append(result.BackfilledSkills, strings.TrimSpace(skillName))
		}
	}
	result.BackfilledSkills = stringutil.UniqueSorted(result.BackfilledSkills)
	result.Blockers = stringutil.UniqueSorted(result.Blockers)
	return result, nil
}

func hasStoredSkillDigest(existingDigests *model.EvidenceDigests, skillName string) bool {
	if existingDigests == nil {
		return false
	}
	existingDigests.Normalize()
	_, ok := existingDigests.Skills[strings.TrimSpace(skillName)]
	return ok
}

func previouslyAcceptedSkillDigestBlockers(
	root string,
	change model.Change,
	skillName string,
	record model.VerificationRecord,
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
	if stored.RunVersion != record.RunVersion {
		return []string{"required_skill_stale:" + strings.TrimSpace(skillName) + ":run_version"}, nil
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
	if skillDigestRecordRefreshedAfterStored(record, stored) {
		changedAfterVerdict, err := digestSelectedInputsChangedAfterVerdict(root, change, skillName, record.Timestamp, changed)
		if err != nil {
			return nil, err
		}
		if len(changedAfterVerdict) == 0 {
			return nil, nil
		}
		return staleSkillDigestBlockers(skillName, changedAfterVerdict), nil
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
		if _, exists := out[skillName]; !exists {
			out[skillName] = record
		}
	}
	return out, nil
}

func legacyDigestBackfillEventRequired(root string, change model.Change, skillName string, record model.VerificationRecord, legacyDigestFileAbsent bool) (bool, error) {
	if !legacyDigestFileAbsent || !record.IsPassing() {
		return false, nil
	}
	return lifecycleHasRecordedSkillEvidence(root, change, skillName)
}

func lifecycleHasRecordedSkillEvidence(root string, change model.Change, skillName string) (bool, error) {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return false, nil
	}
	events, err := state.ReadLifecycleEvents(root, change)
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if event.EventType == "skill.evidence_recorded" && strings.TrimSpace(event.SkillID) == skillName {
			return true, nil
		}
	}
	return false, nil
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
		blockers = append(blockers, "required_skill_stale:"+strings.TrimSpace(skillName)+":"+name)
	}
	return blockers
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
	// assurance.md is intentionally excluded: plan-audit (S1) never audits the
	// assurance contract — AssuranceContractBlockers enforces it only at S3_REVIEW
	// and later — so a late assurance.md edit must not retroactively stale the
	// plan-audit digest. assurance.md remains an input to the final-closeout digest.
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
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !seenAny {
			return errDigestInputsUnavailable
		}
		return err
	}
	hash, err := wave.TaskPlanSemanticHash(string(raw))
	if err != nil {
		return fmt.Errorf("hash tasks.md semantically: %w", err)
	}
	inputs["tasks.md"] = hash
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

func computeProseFileInputHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return model.ComputeInputHash(map[string]any{
		"content": string(raw),
	})
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
	if err := addContentPathInputs(root, change, summary, inputs); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errDigestInputsUnavailable
	}
	return nil
}

func addExecutionSummaryInputs(summary *model.ExecutionSummary, inputs map[string]string) error {
	if state.ExecutionSummaryReady(summary) {
		hash, err := model.ComputeInputHash(map[string]any{
			"execution_summary": summary,
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

func addContentPathInputs(root string, change model.Change, summary *model.ExecutionSummary, inputs map[string]string) error {
	if summary == nil {
		return nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}
	for _, rel := range closeoutGoalVerificationReuseContentPaths(change, summary) {
		candidates, ok, err := closeoutGoalVerificationReuseWorkspacePaths(paths.WorkspaceRoot, rel)
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
			hash, err := model.ComputeFileContentHash(path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					hash, err = deletedFileInputHash(key)
					if err != nil {
						return err
					}
					inputs[key] = hash
					continue
				}
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
	if err := addChangedTargetFileSetInput(change, summary, inputs); err != nil {
		return err
	}
	if err := addContentPathInputs(root, change, summary, inputs); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errDigestInputsUnavailable
	}
	return nil
}

func addChangedTargetFileSetInput(change model.Change, summary *model.ExecutionSummary, inputs map[string]string) error {
	contentPaths := closeoutGoalVerificationReuseContentPaths(change, summary)
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
	if err := addGoalVerificationInputs(root, change, summary, inputs); err != nil {
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
	return addGovernedFileInput(root, change, "assurance.md", inputs)
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
	if err := addReviewSummaryContentInputs(root, change, summary, inputs); err != nil {
		return err
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}
	for _, rel := range reviewWorkspaceInputPaths(paths, change) {
		if err := addWorkspaceFileInput(paths.WorkspaceRoot, rel, inputs); err != nil {
			return err
		}
	}
	if len(inputs) == 0 {
		return errDigestInputsUnavailable
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
	for _, rel := range closeoutGoalVerificationReuseContentPaths(change, summary) {
		rel = filepath.ToSlash(strings.TrimSpace(rel))
		if rel == "" || reviewInputPathExcluded(change, rel) {
			continue
		}
		if !strings.ContainsAny(rel, "*?[") {
			if err := addWorkspaceFileInput(paths.WorkspaceRoot, rel, inputs); err != nil {
				return err
			}
			continue
		}
		candidates, ok, err := closeoutGoalVerificationReuseWorkspacePaths(paths.WorkspaceRoot, rel)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("content path must be workspace-relative: %s", rel)
		}
		for _, path := range candidates {
			hash, err := model.ComputeFileContentHash(path)
			if err != nil {
				return err
			}
			key := rel
			if strings.ContainsAny(rel, "*?[") {
				display, displayErr := filepath.Rel(paths.WorkspaceRoot, path)
				if displayErr == nil {
					key = filepath.ToSlash(display)
				}
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
	if summary == nil || summary.RunSummaryVersion < 1 {
		return errDigestInputsUnavailable
	}
	tasks, issues, err := LoadExecutionTasksFromEvidence(root, change.Slug, summary.RunSummaryVersion)
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
		"run_summary_version": summary.RunSummaryVersion,
		"tasks":               tasks,
	})
	if err != nil {
		return err
	}
	inputs["runtime_task_evidence"] = taskHash
	return nil
}

func reviewWorkspaceInputPaths(paths state.ResolvedChangePaths, change model.Change) []string {
	workspaceRoot := strings.TrimSpace(paths.WorkspaceRoot)
	if workspaceRoot == "" {
		return nil
	}
	files := append([]string{}, gitNameOnly(workspaceRoot, "diff", "--name-only", "HEAD", "--")...)
	files = append(files, gitNameOnly(workspaceRoot, "ls-files", "--others", "--exclude-standard")...)
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		rel := filepath.ToSlash(strings.TrimSpace(file))
		rel = strings.TrimPrefix(rel, "./")
		if rel == "" || reviewInputPathExcluded(change, rel) {
			continue
		}
		filtered = append(filtered, rel)
	}
	return stringutil.UniqueSorted(filtered)
}

func reviewInputPathExcluded(change model.Change, rel string) bool {
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
	hash, err := model.ComputeFileContentHash(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			hash, err = deletedFileInputHash(rel)
			if err != nil {
				return err
			}
			inputs[rel] = hash
			return nil
		}
		return err
	}
	inputs[rel] = hash
	return nil
}

func deletedFileInputHash(rel string) (string, error) {
	return model.ComputeInputHash(map[string]any{
		"path":    filepath.ToSlash(strings.TrimSpace(rel)),
		"deleted": true,
	})
}

func digestInputsChangedAfterVerdict(
	root string,
	change model.Change,
	skillName string,
	verdictAt time.Time,
) ([]string, error) {
	return digestSelectedInputsChangedAfterVerdict(root, change, skillName, verdictAt, nil)
}

func digestSelectedInputsChangedAfterVerdict(
	root string,
	change model.Change,
	skillName string,
	verdictAt time.Time,
	onlyInputs []string,
) ([]string, error) {
	if verdictAt.IsZero() {
		return nil, nil
	}
	paths, err := digestInputArtifactPaths(root, change, skillName)
	if err != nil {
		if errors.Is(err, errDigestInputsUnavailable) {
			return nil, nil
		}
		return nil, err
	}
	only := map[string]struct{}{}
	for _, input := range onlyInputs {
		input = strings.TrimSpace(filepath.ToSlash(input))
		if input != "" {
			only[input] = struct{}{}
		}
	}
	changed := []string{}
	var summary *model.ExecutionSummary
	if strings.TrimSpace(skillName) == SkillWaveOrchestration {
		loaded, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil {
			return nil, err
		}
		summary = loaded
	}
	current, err := certifiedSkillInputDigest(root, change, skillName, summary)
	if err != nil {
		if errors.Is(err, errDigestInputsUnavailable) {
			return nil, nil
		}
		return nil, err
	}
	for rel, inputPaths := range paths {
		if len(only) > 0 {
			if _, ok := only[rel]; !ok {
				continue
			}
		}
		if len(inputPaths) == 0 {
			if deletedInputDigest(current.Inputs[rel], rel) {
				changed = append(changed, rel)
			}
			continue
		}
		inputChanged, err := digestInputChangedAfterVerdict(rel, inputPaths, verdictAt)
		if err != nil {
			return nil, err
		}
		if inputChanged {
			changed = append(changed, rel)
		}
	}
	for rel, digest := range current.Inputs {
		if len(only) > 0 {
			if _, ok := only[rel]; !ok {
				continue
			}
		}
		if _, ok := paths[rel]; ok {
			continue
		}
		if deletedInputDigest(digest, rel) {
			changed = append(changed, rel)
		}
	}
	return stringutil.UniqueSorted(changed), nil
}

func deletedInputDigest(digest, rel string) bool {
	if strings.TrimSpace(digest) == "" {
		return false
	}
	expected, err := model.ComputeInputHash(map[string]any{
		"path":    filepath.ToSlash(strings.TrimSpace(rel)),
		"deleted": true,
	})
	return err == nil && digest == expected
}

func digestInputChangedAfterVerdict(rel string, paths []string, verdictAt time.Time) (bool, error) {
	for _, path := range paths {
		changed, err := digestInputPathChangedAfterVerdict(rel, path, verdictAt)
		if err != nil {
			return false, err
		}
		if changed {
			return true, nil
		}
	}
	return false, nil
}

func digestInputPathChangedAfterVerdict(rel, path string, verdictAt time.Time) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return info.ModTime().UTC().After(verdictAt.UTC()), nil
	}
	changed := false
	err = filepath.WalkDir(path, func(child string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().UTC().After(verdictAt.UTC()) {
			changed = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("inspect digest input %s: %w", rel, err)
	}
	return changed, nil
}

func digestInputArtifactPaths(root string, change model.Change, skillName string) (map[string][]string, error) {
	var summary *model.ExecutionSummary
	if strings.TrimSpace(skillName) == SkillWaveOrchestration {
		loaded, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil {
			return nil, err
		}
		summary = loaded
	}
	current, err := certifiedSkillInputDigest(root, change, skillName, summary)
	if err != nil {
		return nil, err
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for rel := range current.Inputs {
		if err := addDigestInputArtifactPath(out, paths, change.Slug, rel, summary); err != nil {
			return nil, err
		}
	}
	if len(out) == 0 {
		return nil, errDigestInputsUnavailable
	}
	return out, nil
}

func addDigestInputArtifactPath(
	out map[string][]string,
	paths state.ResolvedChangePaths,
	slug string,
	rel string,
	summary *model.ExecutionSummary,
) error {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	if rel == "" {
		return nil
	}
	switch rel {
	case "changed_target_files":
		out[rel] = append(out[rel], state.ExecutionSummaryPathForRead(paths.WorkspaceRoot, slug))
		return nil
	case "run_summary_version":
		rel = "execution-summary.yaml"
	case "tasks_plan_hash":
		rel = "tasks.md"
	case "runtime_task_evidence":
		if summary == nil || summary.RunSummaryVersion < 1 {
			return errDigestInputsUnavailable
		}
		taskPaths, err := runtimeTaskEvidenceInputPaths(paths.WorkspaceRoot, slug, summary.RunSummaryVersion)
		if err != nil {
			return err
		}
		out[rel] = append(out[rel], taskPaths...)
		return nil
	}
	candidates := []string{}
	if rel == "wave-plan.yaml" {
		candidates = append(candidates, state.WavePlanPathForRead(paths.WorkspaceRoot, slug))
	}
	if !strings.Contains(rel, "/") {
		candidates = append(candidates, filepath.Join(paths.GovernedBundleDir, filepath.FromSlash(rel)))
	}
	candidates = append(candidates, filepath.Join(paths.WorkspaceRoot, filepath.FromSlash(rel)))
	if rel == "execution-summary.yaml" {
		candidates = append(candidates, state.ExecutionSummaryPathForRead(paths.WorkspaceRoot, slug))
	}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		if info.IsDir() {
			continue
		}
		out[rel] = append(out[rel], path)
		return nil
	}
	out[rel] = nil
	return nil
}

func runtimeTaskEvidenceInputPaths(root, slug string, runSummaryVersion int) ([]string, error) {
	if runSummaryVersion < 1 {
		return nil, errDigestInputsUnavailable
	}
	dir := state.EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errDigestInputsUnavailable
		}
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		version, err := taskEvidenceRunVersion(path)
		if err != nil {
			return nil, err
		}
		if version == runSummaryVersion {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil, errDigestInputsUnavailable
	}
	return paths, nil
}
