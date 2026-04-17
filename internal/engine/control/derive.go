package control

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

// DeriveControlsInput provides direct model inputs for control derivation,
// replacing the old signal.Detect() + control.Evaluate() chain.
type DeriveControlsInput struct {
	GuardrailDomain     string
	NeedsDiscovery      bool
	ExecutionRunVersion int
	TaskResults         map[string]model.TaskRun
	PlannedTargetFiles  []string
	Traceability        model.TraceabilitySummary
	ExistingControls    []model.ControlActivation
	PolicySource        string
	Overrides           *ControlOverrides
}

// DeriveControlsResult contains the computed signals and controls.
type DeriveControlsResult struct {
	Summary        model.SignalSummary
	Observations   []model.SignalObservation
	ActiveControls []model.ControlActivation
	NewActivations []model.ControlActivation
}

// DeriveControls computes governance signals and controls directly from Change
// fields, without going through the old signal.Detect() scoring pipeline.
func DeriveControls(input DeriveControlsInput) DeriveControlsResult {
	policySource := input.PolicySource
	if policySource == "" {
		policySource = model.BuiltinPolicySource
	}

	// Compute domains.
	domains := deriveDomains(input)

	// Compute blast radius.
	blastRadius, brObs := deriveBlastRadius(input)

	// Build reduced SignalSummary: only domain + blast-radius provenance.
	summary := model.SignalSummary{
		Domains:     domains,
		BlastRadius: blastRadius,
	}

	// Only domain and blast_radius observations.
	var observations []model.SignalObservation
	observations = append(observations, brObs)
	if len(domains) > 0 {
		observations = append(observations, model.SignalObservation{
			ID:           "sig-domain",
			Signal:       model.SignalDomain,
			Level:        model.SignalLevelHigh,
			Source:       "change.guardrail_domain",
			Reason:       fmt.Sprintf("guardrail domains: %s", strings.Join(domains, ", ")),
			EvidenceRefs: []string{"change.guardrail_domain"},
		})
	}

	// Derive control candidates.
	thresholds := defaultThresholds()
	if input.Overrides != nil {
		thresholds = applyOverrides(thresholds, *input.Overrides)
	}

	var candidates []model.ControlActivation

	// clarification: blocking traceability gaps.
	if input.Traceability.Status == model.TraceabilityStatusFail {
		if gap, found := input.Traceability.FirstBlockingIntentGap(); found {
			candidates = append(candidates, model.ControlActivation{
				ControlID:    model.ControlClarification,
				Mode:         ResolveControlMode(model.ControlClarification, input.Overrides),
				Scope:        model.ControlScopeDiscovery,
				Active:       true,
				TriggeredBy:  []string{"traceability: " + gap.Issue},
				PolicySource: policySource,
			})
		}
	}

	// exploration: derives from NeedsDiscovery, NOT from novelty/confidence scores.
	if input.NeedsDiscovery {
		candidates = append(candidates, model.ControlActivation{
			ControlID:    model.ControlResearch,
			Mode:         ResolveControlMode(model.ControlResearch, input.Overrides),
			Scope:        model.ControlScopeDiscovery,
			Active:       true,
			TriggeredBy:  []string{"needs_discovery=true"},
			PolicySource: policySource,
		})
	}

	// domain-review: derives from flat GuardrailDomain being non-empty.
	if len(domains) > 0 {
		candidates = append(candidates, model.ControlActivation{
			ControlID:    model.ControlDomainReview,
			Mode:         ResolveControlMode(model.ControlDomainReview, input.Overrides),
			Scope:        model.ControlScopeReview,
			Active:       true,
			TriggeredBy:  domains,
			PolicySource: policySource,
		})
	}

	// independent-review: medium+ blast radius OR any domain.
	if blastRadius.Order() >= thresholds.independentReviewBlastRadius.Order() ||
		len(domains) > 0 {
		triggers := []string{}
		if blastRadius.Order() >= thresholds.independentReviewBlastRadius.Order() {
			triggers = append(triggers, "blast_radius="+string(blastRadius))
		}
		if len(domains) > 0 {
			triggers = append(triggers, "domain_present")
		}
		candidates = append(candidates, model.ControlActivation{
			ControlID:    model.ControlIndependentReview,
			Mode:         ResolveControlMode(model.ControlIndependentReview, input.Overrides),
			Scope:        model.ControlScopeReview,
			Active:       true,
			TriggeredBy:  triggers,
			PolicySource: policySource,
		})
	}

	// worktree-isolation: medium+ blast radius OR any domain.
	if blastRadius.Order() >= thresholds.worktreeBlastRadius.Order() ||
		len(domains) > 0 {
		triggers := []string{}
		if blastRadius.Order() >= thresholds.worktreeBlastRadius.Order() {
			triggers = append(triggers, "blast_radius="+string(blastRadius))
		}
		if len(domains) > 0 {
			triggers = append(triggers, "domain_present")
		}
		candidates = append(candidates, model.ControlActivation{
			ControlID:    model.ControlWorktreeIsolation,
			Mode:         ResolveControlMode(model.ControlWorktreeIsolation, input.Overrides),
			Scope:        model.ControlScopeExecution,
			Active:       true,
			TriggeredBy:  triggers,
			PolicySource: policySource,
		})
	}

	// rollback-required: deterministic domain mapping (schema_data_migration, irreversible_operations).
	rollbackDomains := []string{
		model.GuardrailDomainSchemaDataMigration,
		model.GuardrailDomainIrreversibleOps,
	}
	for _, d := range domains {
		if slices.Contains(rollbackDomains, d) {
			candidates = append(candidates, model.ControlActivation{
				ControlID:    model.ControlRollbackRequired,
				Mode:         ResolveControlMode(model.ControlRollbackRequired, input.Overrides),
				Scope:        model.ControlScopeRelease,
				Active:       true,
				TriggeredBy:  []string{"domain=" + d},
				PolicySource: policySource,
			})
			break
		}
	}

	// Filter out disabled controls.
	if input.Overrides != nil && len(input.Overrides.DisabledControls) > 0 {
		candidates = filterDisabledControls(candidates, input.Overrides.DisabledControls)
	}

	// Monotonic merge: keep existing active controls, add new ones.
	merged := mergeMonotonic(input.ExistingControls, candidates)

	return DeriveControlsResult{
		Summary:        summary,
		Observations:   observations,
		ActiveControls: merged.ActiveControls,
		NewActivations: merged.NewActivations,
	}
}

// deriveDomains computes domains from the flat GuardrailDomain field.
// GuardrailDomain is populated at change creation time via caller-provided
// classification; no runtime inference is needed.
func deriveDomains(input DeriveControlsInput) []string {
	if d := strings.TrimSpace(input.GuardrailDomain); d != "" {
		return []string{d}
	}
	return nil
}

// deriveBlastRadius computes blast radius from the contract source for the
// current lifecycle phase.
// Pre-execution (ExecutionRunVersion == 0): count tasks.md target_files.
// Post-execution (ExecutionRunVersion >= 1): count unique ChangedFiles.
func deriveBlastRadius(input DeriveControlsInput) (model.SignalLevel, model.SignalObservation) {
	var fileCount int
	var source string

	if input.ExecutionRunVersion == 0 {
		// Pre-execution: use the planned scope from tasks.md rather than any
		// stale or speculative task-run metadata.
		fileCount = countUniqueFiles(input.PlannedTargetFiles)
		source = "tasks_checklist.target_files"
	} else {
		// Post-execution: count unique ChangedFiles across summarized task results.
		fileCount = countUniqueChangedFiles(input.TaskResults, input.ExecutionRunVersion)
		source = "execution_summary.tasks.changed_files"
		if fileCount == 0 {
			// Task results empty or ChangedFiles unavailable despite execution
			// having occurred; fall back to planned target files so blast
			// radius does not falsely report "low".
			fileCount = countUniqueFiles(input.PlannedTargetFiles)
			source = "tasks_checklist.target_files(fallback)"
			if fileCount == 0 {
				// Neither task results nor planned target files available;
				// degrade to medium as safe conservative default per Plan B.
				return model.SignalLevelMedium, model.SignalObservation{
					ID:           "sig-blast-radius",
					Signal:       model.SignalBlastRadius,
					Level:        model.SignalLevelMedium,
					Source:       "degrade_medium(no_data)",
					Reason:       "post-execution blast radius unavailable — degrading to medium",
					EvidenceRefs: []string{"degrade_medium(no_data)"},
				}
			}
		}
	}

	level := blastRadiusLevel(fileCount)

	return level, model.SignalObservation{
		ID:           "sig-blast-radius",
		Signal:       model.SignalBlastRadius,
		Level:        level,
		Source:       source,
		Reason:       fmt.Sprintf("%d files (%s threshold: <=3 low, 4-10 medium, >10 high)", fileCount, source),
		EvidenceRefs: []string{source},
	}
}

// blastRadiusLevel implements the built-in baseline thresholds.
func blastRadiusLevel(fileCount int) model.SignalLevel {
	switch {
	case fileCount <= 3:
		return model.SignalLevelLow
	case fileCount <= 10:
		return model.SignalLevelMedium
	default:
		return model.SignalLevelHigh
	}
}

func countUniqueFiles(files []string) int {
	seen := map[string]struct{}{}
	for _, f := range files {
		normalized := normalizeFilePath(f)
		if normalized != "" {
			seen[normalized] = struct{}{}
		}
	}
	return len(seen)
}

func countUniqueChangedFiles(taskRuns map[string]model.TaskRun, latestVersion int) int {
	seen := map[string]struct{}{}
	for _, run := range taskRuns {
		if run.RunSummaryVersion != latestVersion {
			continue
		}
		for _, f := range run.ChangedFiles {
			normalized := normalizeFilePath(f)
			if normalized != "" {
				seen[normalized] = struct{}{}
			}
		}
	}
	return len(seen)
}

func normalizeFilePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}
