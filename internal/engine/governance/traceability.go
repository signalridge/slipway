package governance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/stringutil"
)

var (
	// ID patterns for traceability extraction.
	intentIDPattern      = regexp.MustCompile(`(?i)\bINT-(\w+)\b`)
	requirementIDPattern = regexp.MustCompile(`(?i)\bREQ-(\w+)\b`)
	decisionIDPattern    = regexp.MustCompile(`(?i)\bDEC-(\w+)\b`)
	coversPattern        = regexp.MustCompile(`(?i)covers:\s*\[([^\]]*)\]`)
	taskHeaderPattern    = regexp.MustCompile(`^\s*[-*]\s*\[[ xX]\]\s*(.+?)\s*$`)
	taskIDPattern        = regexp.MustCompile("`([^`]+)`")
)

// TraceabilityInput holds the data needed for traceability evaluation.
type TraceabilityInput struct {
	BundleDir  string
	Slug       string
	SchemaName model.ArtifactSchemaName
	// LifecycleState is the change's current workflow state. It makes
	// closeout-time checks (per-requirement assurance coverage verdicts)
	// stage-aware: before S3_REVIEW those gaps are advisory because the
	// assurance verdicts are authored during review/verify, while at and after
	// S3_REVIEW they remain blocking. An empty/unknown state stays fail-closed
	// (blocking).
	LifecycleState model.WorkflowState
	// AssuranceOptional is true when the effective workflow preset does not
	// require assurance.md. The zero value preserves standard/strict behavior.
	AssuranceOptional bool
	// ArtifactNames maps logical names (e.g., "intent.md") to their resolved paths.
	// When nil, default artifact resolution is used.
	ArtifactResolver func(artifactName string) string
}

// assuranceVerdictsExpectedLater reports whether the change is still before the
// review phase, where per-requirement assurance coverage verdicts have not yet
// been authored. For those states, a missing assurance verdict is expected and
// must not be a blocking traceability incident. Any other state — including an
// empty/unknown state — is treated as at/after review, keeping the assurance
// gaps fail-closed.
func assuranceVerdictsExpectedLater(state model.WorkflowState) bool {
	switch state {
	case model.StateS0Intake, model.StateS1Plan, model.StateS2Execute:
		return true
	default:
		return false
	}
}

func assuranceArtifactRequiredNow(state model.WorkflowState) bool {
	switch state {
	case model.StateS3Review, model.StateS4Verify, model.StateDone:
		return true
	case "":
		// Empty state means the caller did not provide lifecycle context, which is
		// still used by standalone traceability scans. Preserve that mode: if an
		// assurance file exists, its contents are checked below, but mere absence is
		// not a lifecycle blocker without an explicit review/verify/done state.
		return false
	default:
		return !assuranceVerdictsExpectedLater(state)
	}
}

// EvaluateTraceability derives a traceability summary by scanning artifact content.
// It uses ResolveArtifactPath through the provided resolver, never hardcoding file names.
func EvaluateTraceability(input TraceabilityInput) model.TraceabilitySummary {
	resolve := input.ArtifactResolver
	if resolve == nil {
		resolve = func(name string) string {
			return artifact.ResolveArtifactPath(input.BundleDir, name)
		}
	}

	summary := model.TraceabilitySummary{
		Status: model.TraceabilityStatusOK,
	}
	var gaps []model.TraceabilityGap

	// Scan intent artifact for INT-* IDs.
	intentContent := readArtifactContent(resolve("intent.md"))
	intentIDs := extractIDs(intentContent, intentIDPattern)

	// Scan requirements artifact for REQ-* IDs.
	reqContent := readArtifactContent(resolve("requirements.md"))
	requireIDs := extractIDs(reqContent, requirementIDPattern)
	parsedReqBlocks := artifact.ParseRequirementBlocks(reqContent)
	if strings.TrimSpace(reqContent) != "" && len(parsedReqBlocks) == 0 {
		gaps = append(gaps, model.TraceabilityGap{
			ID:       "requirements-no-blocks",
			Type:     "requirement",
			Issue:    "requirements artifact has no Requirement blocks (expected ### Requirement: <Name>)",
			Blocking: true,
		})
	}
	if missingREQIDs := artifact.RequirementBlocksMissingStableIDs(reqContent); len(missingREQIDs) > 0 {
		issue := "requirements artifact has no stable REQ-* IDs"
		if len(missingREQIDs) < len(parsedReqBlocks) {
			issue = fmt.Sprintf("requirement blocks missing stable REQ-* IDs: %s", strings.Join(missingREQIDs, ", "))
		}
		gaps = append(gaps, model.TraceabilityGap{
			ID:       "requirements-stable-ids",
			Type:     "requirement",
			Issue:    issue,
			Blocking: true,
		})
	}
	gaps = append(gaps, evaluateRequirementDeltaStructure(reqContent)...)

	// Scan decision artifact for DEC-* IDs.
	decContent := readArtifactContent(resolve("decision.md"))
	decisionIDs := extractIDs(decContent, decisionIDPattern)

	// Scan tasks artifact for covers: [REQ-*] references.
	tasksContent := readArtifactContent(resolve("tasks.md"))
	taskCovers := extractCoversRefs(tasksContent)

	// Scan assurance artifact for per-requirement coverage.
	assurancePath := resolve("assurance.md")
	assuranceContent, assuranceReadErr := readArtifactContentWithError(assurancePath)
	assuranceCoverage := extractMarkdownSectionBody(assuranceContent, "## Requirement Coverage")
	assuranceREQs := extractIDs(assuranceCoverage, requirementIDPattern)

	// Build links and detect gaps.
	var links []model.TraceabilityLink
	coreSchema := input.SchemaName == model.ArtifactSchemaCore

	// Check: each REQ must reference at least one INT.
	// Extract per-requirement blocks to check scope-local references.
	reqBlocks := extractRequirementBlocks(reqContent, requireIDs)
	validIntentIDs := make(map[string]struct{}, len(intentIDs))
	for _, intID := range intentIDs {
		validIntentIDs["INT-"+intID] = struct{}{}
	}
	for _, reqID := range requireIDs {
		hasUpstream := false
		block := reqBlocks[reqID]
		referencedIntentIDs := extractIDs(block, intentIDPattern)
		var unknownIntentRefs []string
		for _, intID := range referencedIntentIDs {
			fullRef := "INT-" + intID
			if _, ok := validIntentIDs[fullRef]; ok {
				links = append(links, model.TraceabilityLink{
					FromID: "REQ-" + reqID, FromType: "requirement",
					ToID: fullRef, ToType: "intent",
				})
				hasUpstream = true
				continue
			}
			unknownIntentRefs = append(unknownIntentRefs, fullRef)
		}
		if len(unknownIntentRefs) > 0 {
			gaps = append(gaps, model.TraceabilityGap{
				ID:       "REQ-" + reqID,
				Type:     "requirement",
				Issue:    "requirement references unknown intent ID(s): " + strings.Join(unknownIntentRefs, ", "),
				Blocking: true,
			})
			continue
		}
		if !hasUpstream && len(intentIDs) > 0 {
			gaps = append(gaps, model.TraceabilityGap{
				ID:       "REQ-" + reqID,
				Type:     "requirement",
				Issue:    "requirement has no upstream intent reference",
				Blocking: !coreSchema,
			})
		}
	}

	// Check: each DEC must reference at least one REQ in its local block.
	if len(requireIDs) > 0 {
		validRequirements := map[string]struct{}{}
		for _, reqID := range requireIDs {
			validRequirements[requirementRef(reqID)] = struct{}{}
		}
		decisionBlocks := extractBlocksByID(decContent, decisionIDs, "DEC-")
		for _, decID := range decisionIDs {
			block := decisionBlocks[decID]
			reqRefs := extractIDs(block, requirementIDPattern)
			hasRequirementLink := false
			for _, reqID := range reqRefs {
				fullReqID := requirementRef(reqID)
				if _, ok := validRequirements[fullReqID]; !ok {
					continue
				}
				links = append(links, model.TraceabilityLink{
					FromID: "DEC-" + decID, FromType: "decision",
					ToID: fullReqID, ToType: "requirement",
				})
				hasRequirementLink = true
			}
			if !hasRequirementLink {
				gaps = append(gaps, model.TraceabilityGap{
					ID:       "DEC-" + decID,
					Type:     "decision",
					Issue:    "decision has no linked requirement reference",
					Blocking: true,
				})
			}
		}
	}

	// Check: each task covers at least one REQ.
	coveredREQs := map[string]bool{}
	for taskID, refs := range taskCovers {
		if len(refs) == 0 {
			gaps = append(gaps, model.TraceabilityGap{
				ID:       taskID,
				Type:     "task",
				Issue:    "task covers no requirement",
				Blocking: true,
			})
		}
		for _, ref := range refs {
			fullRef := normalizeRequirementRef(ref)
			if fullRef == "" {
				continue
			}
			coveredREQs[fullRef] = true
			links = append(links, model.TraceabilityLink{
				FromID: taskID, FromType: "task",
				ToID: fullRef, ToType: "requirement",
			})
		}
	}
	if strings.TrimSpace(tasksContent) != "" {
		for _, reqID := range requireIDs {
			fullReqID := requirementRef(reqID)
			if coveredREQs[fullReqID] {
				continue
			}
			gaps = append(gaps, model.TraceabilityGap{
				ID:       fullReqID,
				Type:     "requirement",
				Issue:    "requirement has no covering task",
				Blocking: true,
			})
		}
	}

	// Check: assurance must include per-requirement coverage verdicts.
	// These verdicts are authored during review/verify, so before S3_REVIEW a
	// missing verdict is expected and reported as a non-blocking warning; at and
	// after review (and for an unknown state) it fails closed.
	assuranceBlocking := !assuranceVerdictsExpectedLater(input.LifecycleState)
	assuranceRequiredNow := assuranceBlocking &&
		!input.AssuranceOptional &&
		assuranceArtifactRequiredNow(input.LifecycleState)
	switch {
	case assuranceRequiredNow && assuranceReadErr != nil:
		gapID := "assurance-unreadable"
		issue := "assurance.md unreadable at review/verify phase"
		if os.IsNotExist(assuranceReadErr) {
			gapID = "assurance-missing"
			issue = "assurance.md missing at review/verify phase"
		}
		gaps = append(gaps, model.TraceabilityGap{
			ID:       gapID,
			Type:     "assurance",
			Issue:    issue,
			Blocking: true,
		})
	case assuranceRequiredNow && strings.TrimSpace(assuranceContent) == "":
		gaps = append(gaps, model.TraceabilityGap{
			ID:       "assurance-empty",
			Type:     "assurance",
			Issue:    "assurance.md empty at review/verify phase",
			Blocking: true,
		})
	}
	if strings.TrimSpace(assuranceContent) != "" && len(requireIDs) > 0 {
		if len(assuranceREQs) == 0 {
			gaps = append(gaps, model.TraceabilityGap{
				ID:       "assurance",
				Type:     "assurance",
				Issue:    "assurance verifies no requirement IDs",
				Blocking: assuranceBlocking,
			})
		} else {
			assuredREQs := map[string]bool{}
			for _, reqID := range assuranceREQs {
				fullReqID := requirementRef(reqID)
				assuredREQs[fullReqID] = true
				links = append(links, model.TraceabilityLink{
					FromID: "assurance", FromType: "assurance",
					ToID: fullReqID, ToType: "requirement",
				})
			}
			for _, reqID := range requireIDs {
				fullReqID := requirementRef(reqID)
				if assuredREQs[fullReqID] {
					continue
				}
				gaps = append(gaps, model.TraceabilityGap{
					ID:       fullReqID,
					Type:     "assurance",
					Issue:    "requirement missing assurance coverage verdict",
					Blocking: assuranceBlocking,
				})
			}
		}
	}

	// Check: blocking open questions in intent.
	if hasBlockingOpenQuestions(intentContent) {
		// Only blocking if downstream artifacts exist.
		if len(decContent) > 0 || len(tasksContent) > 0 {
			gaps = append(gaps, model.TraceabilityGap{
				ID:       "intent-open-questions",
				Type:     "intent",
				Issue:    "blocking open questions remain unresolved while downstream artifacts are ready",
				Blocking: true,
			})
		}
	}

	summary.Links = links
	summary.Gaps = gaps

	// Determine overall status.
	hasBlocking := false
	for _, gap := range gaps {
		if gap.Blocking {
			hasBlocking = true
			break
		}
	}

	switch {
	case hasBlocking:
		summary.Status = model.TraceabilityStatusFail
		summary.Message = fmt.Sprintf("%d blocking traceability gaps", countBlockingGaps(gaps))
	case len(gaps) > 0:
		summary.Status = model.TraceabilityStatusWarning
		summary.Message = fmt.Sprintf("%d traceability warnings", len(gaps))
	default:
		summary.Message = buildTraceabilitySuccessMessage(intentContent, reqContent, tasksContent, decContent, assuranceContent)
	}

	return summary
}

func evaluateRequirementDeltaStructure(reqContent string) []model.TraceabilityGap {
	if strings.TrimSpace(reqContent) == "" {
		return nil
	}
	deltaHeadings := []struct {
		kind    string
		heading string
	}{
		{kind: "added", heading: "## ADDED Requirements"},
		{kind: "modified", heading: "## MODIFIED Requirements"},
		{kind: "removed", heading: "## REMOVED Requirements"},
	}
	var gaps []model.TraceabilityGap
	for _, section := range deltaHeadings {
		if !markdownSectionExists(reqContent, section.heading) {
			continue
		}
		body := extractMarkdownSectionBody(reqContent, section.heading)
		if strings.TrimSpace(body) == "" {
			gaps = append(gaps, model.TraceabilityGap{
				ID:       "requirements-delta-" + section.kind + "-empty",
				Type:     "requirement",
				Issue:    fmt.Sprintf("%s section is empty", section.heading),
				Blocking: true,
			})
			continue
		}
		blocks := artifact.ParseRequirementBlocks(body)
		if len(blocks) > 0 {
			continue
		}
		gaps = append(gaps, model.TraceabilityGap{
			ID:       "requirements-delta-" + section.kind + "-no-blocks",
			Type:     "requirement",
			Issue:    fmt.Sprintf("%s section has no Requirement blocks (expected ### Requirement: <Name>)", section.heading),
			Blocking: true,
		})
	}
	gaps = append(gaps, evaluateStructuredSupportSection(reqContent, "## NON-GOALS", "non-goals", nil)...)
	gaps = append(gaps, evaluateStructuredSupportSection(reqContent, "## DECISIONS", "decisions", decisionIDPattern)...)
	gaps = append(gaps, evaluateStructuredSupportSection(reqContent, "## ROLLBACK", "rollback", nil)...)
	return gaps
}

func evaluateStructuredSupportSection(content, heading, kind string, requiredPattern *regexp.Regexp) []model.TraceabilityGap {
	if !markdownSectionExists(content, heading) {
		return nil
	}
	body := extractMarkdownSectionBody(content, heading)
	if strings.TrimSpace(body) == "" {
		return []model.TraceabilityGap{{
			ID:       "requirements-delta-" + kind + "-empty",
			Type:     "requirement",
			Issue:    fmt.Sprintf("%s section is empty", heading),
			Blocking: true,
		}}
	}
	if requiredPattern != nil && len(extractIDs(body, requiredPattern)) == 0 {
		return []model.TraceabilityGap{{
			ID:       "requirements-delta-" + kind + "-missing-ids",
			Type:     "requirement",
			Issue:    fmt.Sprintf("%s section has no stable decision IDs", heading),
			Blocking: true,
		}}
	}
	return nil
}

// extractRequirementBlocks splits the requirements file content into per-REQ-ID blocks.
func extractRequirementBlocks(content string, reqIDs []string) map[string]string {
	return extractBlocksByID(content, reqIDs, "REQ-")
}

// extractBlocksByID splits content into per-ID blocks using the given prefix.
// Each block spans from the matching marker to the next marker with the same prefix or end of content.
func extractBlocksByID(content string, ids []string, prefix string) map[string]string {
	blocks := make(map[string]string, len(ids))
	for _, id := range ids {
		marker := prefix + id
		idx := strings.Index(content, marker)
		if idx < 0 {
			blocks[id] = ""
			continue
		}
		// Find the extent: from this marker to the next marker with the same prefix or end.
		rest := content[idx:]
		endIdx := len(rest)
		for _, otherID := range ids {
			if otherID == id {
				continue
			}
			otherMarker := prefix + otherID
			if pos := strings.Index(rest[len(marker):], otherMarker); pos >= 0 {
				candidate := len(marker) + pos
				if candidate < endIdx {
					endIdx = candidate
				}
			}
		}
		blocks[id] = rest[:endIdx]
	}
	return blocks
}

func readArtifactContent(path string) string {
	content, _ := readArtifactContentWithError(path)
	return content
}

func readArtifactContentWithError(path string) (string, error) {
	if path == "" {
		return "", os.ErrNotExist
	}
	b, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func extractIDs(content string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	var ids []string
	for _, m := range matches {
		id := m[1]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func extractCoversRefs(tasksContent string) map[string][]string {
	result := map[string][]string{}
	lines := strings.Split(tasksContent, "\n")

	currentTask := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if taskID := extractTaskIdentifier(trimmed); taskID != "" {
			currentTask = taskID
			if _, ok := result[currentTask]; !ok {
				result[currentTask] = nil
			}
		}

		// Extract covers: [REQ-*] references.
		if m := coversPattern.FindStringSubmatch(trimmed); len(m) > 1 {
			refs := parseRefList(m[1])
			if currentTask != "" {
				result[currentTask] = append(result[currentTask], refs...)
			}
		}
	}

	return result
}

func extractTaskIdentifier(line string) string {
	matches := taskHeaderPattern.FindStringSubmatch(line)
	if len(matches) != 2 {
		return ""
	}
	taskText := strings.TrimSpace(matches[1])
	if taskText == "" {
		return ""
	}
	if id := taskIDPattern.FindStringSubmatch(taskText); len(id) == 2 {
		return strings.TrimSpace(id[1])
	}
	return taskText
}

func parseRefList(raw string) []string {
	var refs []string
	for _, part := range strings.Split(raw, ",") {
		ref := strings.TrimSpace(part)
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func hasBlockingOpenQuestions(intentContent string) bool {
	return stringutil.HasBlockingOpenQuestions(intentContent)
}

func extractMarkdownSectionBody(content, heading string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	target := strings.ToLower(strings.TrimSpace(heading))
	found := false
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !found {
			if strings.ToLower(trimmed) == target {
				found = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			break
		}
		body = append(body, line)
	}
	return strings.TrimSpace(strings.Join(body, "\n"))
}

func markdownSectionExists(content, heading string) bool {
	if strings.TrimSpace(content) == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(heading))
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.ToLower(strings.TrimSpace(line)) == target {
			return true
		}
	}
	return false
}

func requirementRef(id string) string {
	return normalizeRequirementRef("REQ-" + strings.TrimSpace(id))
}

func normalizeRequirementRef(ref string) string {
	ref = filepath.ToSlash(strings.TrimSpace(strings.Trim(ref, "`")))
	if requirementID := artifact.NormalizeRequirementID(ref); requirementID != "" {
		return requirementID
	}
	if ref == "" {
		return ""
	}
	return strings.ToUpper(ref)
}

func countBlockingGaps(gaps []model.TraceabilityGap) int {
	count := 0
	for _, g := range gaps {
		if g.Blocking {
			count++
		}
	}
	return count
}

func buildTraceabilitySuccessMessage(intentContent, reqContent, tasksContent, decContent, assuranceContent string) string {
	stages := make([]string, 0, 5)
	if strings.TrimSpace(intentContent) != "" {
		stages = append(stages, "intent")
	}
	if strings.TrimSpace(reqContent) != "" {
		stages = append(stages, "requirements")
	}
	if strings.TrimSpace(tasksContent) != "" {
		stages = append(stages, "tasks")
	}
	if strings.TrimSpace(decContent) != "" {
		stages = append(stages, "decisions")
	}
	if strings.TrimSpace(assuranceContent) != "" {
		stages = append(stages, "assurance")
	}
	if len(stages) == 0 {
		return "all traceability checks passed"
	}
	return fmt.Sprintf("all traceability checks passed across %s", joinTraceabilityStages(stages))
}

func joinTraceabilityStages(stages []string) string {
	switch len(stages) {
	case 0:
		return ""
	case 1:
		return stages[0]
	case 2:
		return stages[0] + " and " + stages[1]
	default:
		return strings.Join(stages[:len(stages)-1], ", ") + ", and " + stages[len(stages)-1]
	}
}

func isAuditGapEligibleForLightPreset(gap model.TraceabilityGap) bool {
	// Keep governance/structure gaps blocking even on the light preset.
	if gap.Type == "intent" {
		return false
	}
	switch gap.ID {
	case "requirements-no-blocks", "requirements-stable-ids":
		return false
	default:
		if strings.HasPrefix(gap.ID, "requirements-delta-") {
			return false
		}
		return true
	}
}

// downgradeAuditGapsForLightPreset converts audit-style blocking gaps to
// non-blocking (advisory). Governance/structure gaps stay blocking so light
// preset simplifies authoring without accepting broken artifact contracts.
// After downgrade, the overall status is recomputed.
func downgradeAuditGapsForLightPreset(summary model.TraceabilitySummary) model.TraceabilitySummary {
	for i := range summary.Gaps {
		if summary.Gaps[i].Blocking && isAuditGapEligibleForLightPreset(summary.Gaps[i]) {
			summary.Gaps[i].Blocking = false
		}
	}
	// Recompute status after downgrade.
	hasBlocking := false
	for _, gap := range summary.Gaps {
		if gap.Blocking {
			hasBlocking = true
			break
		}
	}
	switch {
	case hasBlocking:
		summary.Status = model.TraceabilityStatusFail
		summary.Message = fmt.Sprintf("%d blocking traceability gaps", countBlockingGaps(summary.Gaps))
	case len(summary.Gaps) > 0:
		summary.Status = model.TraceabilityStatusWarning
		summary.Message = fmt.Sprintf("%d traceability warnings", len(summary.Gaps))
	default:
		summary.Status = model.TraceabilityStatusOK
		if strings.TrimSpace(summary.Message) == "" {
			summary.Message = "all traceability checks passed"
		}
	}
	return summary
}
