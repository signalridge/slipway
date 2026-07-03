package model

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const PlanDimensionReferencePrefix = "dim:"

type PlanDimensionName string

const (
	PlanDimensionDecisionSoundness PlanDimensionName = "decision_soundness"
	PlanDimensionConsistency       PlanDimensionName = "consistency"
)

const (
	PlanDimensionVerdictPass = "pass"
	PlanDimensionVerdictFail = "fail"
)

type PlanDimensionAttestation struct {
	Name         PlanDimensionName `json:"name"`
	Verdict      string            `json:"verdict"`
	EvidenceRef  string            `json:"evidence_ref"`
	EvidencePath string            `json:"evidence_path"`
}

type PlanDimensionAttestationSet struct {
	Attestations map[PlanDimensionName]PlanDimensionAttestation `json:"attestations"`
}

var lineSuffixPattern = regexp.MustCompile(`:\d+(?::\d+)?(?:-\d+(?::\d+)?)?$`)

func RequiredPlanDimensionAttestationBlockers(root string, record VerificationRecord) (PlanDimensionAttestationSet, []ReasonCode) {
	return RequiredPlanDimensionAttestationBlockersForSkill(root, record, "")
}

func RequiredPlanDimensionAttestationBlockersForSkill(
	root string,
	record VerificationRecord,
	owningSkill string,
) (PlanDimensionAttestationSet, []ReasonCode) {
	return PlanDimensionAttestationBlockers(
		root,
		record,
		[]PlanDimensionName{
			PlanDimensionDecisionSoundness,
			PlanDimensionConsistency,
		},
		owningSkill,
	)
}

func PlanDimensionAttestationBlockers(
	root string,
	record VerificationRecord,
	required []PlanDimensionName,
	owningSkill string,
) (PlanDimensionAttestationSet, []ReasonCode) {
	attestations := PlanDimensionAttestationSet{
		Attestations: map[PlanDimensionName]PlanDimensionAttestation{},
	}
	var blockers []ReasonCode
	conflicted := map[PlanDimensionName]struct{}{}
	seenDimensions := map[PlanDimensionName]struct{}{}

	for _, ref := range record.References {
		if name, ok := planDimensionNameFromReference(ref); ok {
			seenDimensions[name] = struct{}{}
		}
		attestation, ok, reason := parsePlanDimensionReference(root, ref)
		if !ok {
			if reason != nil {
				blockers = append(blockers, *reason)
			}
			continue
		}
		if existing, seen := attestations.Attestations[attestation.Name]; seen {
			if existing.Verdict == attestation.Verdict {
				continue
			}
			delete(attestations.Attestations, attestation.Name)
			conflicted[attestation.Name] = struct{}{}
			blockers = append(blockers, NewReasonCode("plan_dimension_attestation_conflict", string(attestation.Name)))
			continue
		}
		if _, seen := conflicted[attestation.Name]; seen {
			continue
		}
		attestations.Attestations[attestation.Name] = attestation
	}

	for _, name := range required {
		attestation, ok := attestations.Attestations[name]
		if !ok {
			if _, seen := seenDimensions[name]; !seen {
				blockers = append(blockers, missingPlanDimensionReason(name))
			}
			continue
		}
		switch name {
		case PlanDimensionDecisionSoundness:
			if attestation.Verdict == PlanDimensionVerdictFail {
				blockers = append(blockers, NewReasonCode("plan_dimension_decision_unsound", attestation.EvidenceRef))
			}
		case PlanDimensionConsistency:
			if attestation.Verdict == PlanDimensionVerdictFail {
				blockers = append(blockers, NewReasonCode("plan_dimension_consistency_failed", attestation.EvidenceRef))
			}
		}
	}

	return attestations, attachPlanDimensionOwningSkill(NormalizeReasonCodes(blockers), owningSkill)
}

func attachPlanDimensionOwningSkill(blockers []ReasonCode, owningSkill string) []ReasonCode {
	owningSkill = strings.TrimSpace(owningSkill)
	if owningSkill == "" || len(blockers) == 0 {
		return blockers
	}
	owned := make([]ReasonCode, 0, len(blockers))
	for _, blocker := range blockers {
		detail := strings.TrimSpace(blocker.Detail)
		if detail == "" {
			blocker.Detail = owningSkill
		} else {
			blocker.Detail = owningSkill + ":" + detail
		}
		owned = append(owned, blocker)
	}
	return NormalizeReasonCodes(owned)
}

func planDimensionNameFromReference(ref string) (PlanDimensionName, bool) {
	raw := strings.Trim(strings.TrimSpace(ref), "\"'`.,;()[]{}")
	if !strings.HasPrefix(raw, PlanDimensionReferencePrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(raw, PlanDimensionReferencePrefix)
	nameRaw, _, ok := strings.Cut(rest, "=")
	if !ok {
		return "", false
	}
	name := PlanDimensionName(strings.TrimSpace(nameRaw))
	if !planDimensionNameValid(name) {
		return "", false
	}
	return name, true
}

func missingPlanDimensionReason(name PlanDimensionName) ReasonCode {
	switch name {
	case PlanDimensionDecisionSoundness:
		return NewReasonCode("plan_dimension_decision_soundness_unattested", "")
	case PlanDimensionConsistency:
		return NewReasonCode("plan_dimension_consistency_unattested", "")
	default:
		return NewReasonCode("plan_dimension_attestation_invalid", string(name))
	}
}

func parsePlanDimensionReference(root, ref string) (PlanDimensionAttestation, bool, *ReasonCode) {
	raw := strings.Trim(strings.TrimSpace(ref), "\"'`.,;()[]{}")
	if !strings.HasPrefix(raw, PlanDimensionReferencePrefix) {
		return PlanDimensionAttestation{}, false, nil
	}

	rest := strings.TrimPrefix(raw, PlanDimensionReferencePrefix)
	nameRaw, verdictAndEvidence, ok := strings.Cut(rest, "=")
	if !ok {
		return PlanDimensionAttestation{}, false, reasonPtr("plan_dimension_attestation_invalid", raw)
	}
	verdictRaw, evidenceRef, ok := strings.Cut(verdictAndEvidence, ":")
	if !ok {
		return PlanDimensionAttestation{}, false, reasonPtr("plan_dimension_attestation_invalid", raw)
	}

	name := PlanDimensionName(strings.TrimSpace(nameRaw))
	verdict := strings.TrimSpace(verdictRaw)
	evidenceRef = strings.TrimSpace(evidenceRef)
	if !planDimensionNameValid(name) || !planDimensionVerdictValid(verdict) {
		return PlanDimensionAttestation{}, false, reasonPtr("plan_dimension_attestation_invalid", raw)
	}

	evidencePath, reason := normalizePlanDimensionEvidencePath(evidenceRef)
	if reason != nil {
		return PlanDimensionAttestation{}, false, reason
	}
	if name == PlanDimensionDecisionSoundness && planDimensionEvidenceInArtifacts(evidencePath) {
		return PlanDimensionAttestation{}, false, reasonPtr("plan_dimension_decision_soundness_evidence_invalid", evidenceRef)
	}
	if reason := requirePlanDimensionEvidenceFile(root, evidencePath, evidenceRef); reason != nil {
		return PlanDimensionAttestation{}, false, reason
	}

	return PlanDimensionAttestation{
		Name:         name,
		Verdict:      verdict,
		EvidenceRef:  evidenceRef,
		EvidencePath: evidencePath,
	}, true, nil
}

func planDimensionNameValid(name PlanDimensionName) bool {
	switch name {
	case PlanDimensionDecisionSoundness, PlanDimensionConsistency:
		return true
	default:
		return false
	}
}

func planDimensionVerdictValid(verdict string) bool {
	switch verdict {
	case PlanDimensionVerdictPass, PlanDimensionVerdictFail:
		return true
	default:
		return false
	}
}

func normalizePlanDimensionEvidencePath(evidenceRef string) (string, *ReasonCode) {
	if planDimensionEvidenceRefInvalid(evidenceRef) {
		return "", reasonPtr("plan_dimension_attestation_invalid", evidenceRef)
	}
	pathRef := stripPlanDimensionEvidenceSuffix(evidenceRef)
	normalized := NormalizePublicPath(pathRef)
	if normalized == "" ||
		PublicPathIsAbs(pathRef) ||
		PublicPathHasParentTraversal(pathRef) ||
		planDimensionEvidenceRefInvalid(normalized) {
		return "", reasonPtr("plan_dimension_attestation_invalid", evidenceRef)
	}
	return normalized, nil
}

func requirePlanDimensionEvidenceFile(root, normalized, evidenceRef string) *ReasonCode {
	fullPath := filepath.Join(root, filepath.FromSlash(normalized))
	info, err := os.Stat(fullPath)
	if err != nil || !info.Mode().IsRegular() {
		return reasonPtr("plan_dimension_attestation_evidence_unresolvable", evidenceRef)
	}
	return nil
}

func planDimensionEvidenceInArtifacts(evidencePath string) bool {
	return evidencePath == "artifacts" || strings.HasPrefix(evidencePath, "artifacts/")
}

func stripPlanDimensionEvidenceSuffix(evidenceRef string) string {
	beforeAnchor, _, _ := strings.Cut(evidenceRef, "#")
	return lineSuffixPattern.ReplaceAllString(strings.TrimSpace(beforeAnchor), "")
}

func planDimensionEvidenceRefInvalid(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(trimmed, "<") || strings.Contains(trimmed, ">") {
		return true
	}
	return lower == "placeholder" ||
		lower == "todo" ||
		lower == "path/to" ||
		strings.HasPrefix(lower, "path/to/")
}

func reasonPtr(code, detail string) *ReasonCode {
	reason := NewReasonCode(code, detail)
	return &reason
}
