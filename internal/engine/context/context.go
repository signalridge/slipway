package enginecontext

import (
	"time"
)

type ExecutionMode string

const (
	ExecutionModeGoverned ExecutionMode = "governed"
)

type EvidenceFreshness string

const (
	EvidenceFreshnessFresh   EvidenceFreshness = "fresh"
	EvidenceFreshnessStale   EvidenceFreshness = "stale"
	EvidenceFreshnessUnknown EvidenceFreshness = "unknown"
)

type EvidenceFreshnessInput struct {
	EvidenceInputHash      string    `json:"evidence_input_hash"`
	CurrentInputHash       string    `json:"current_input_hash"`
	EvidenceTimestamp      time.Time `json:"evidence_timestamp"`
	LatestRelevantUpdateAt time.Time `json:"latest_relevant_update_at"`
}

func EvaluateEvidenceFreshness(
	hasActiveContext bool,
	inputs []EvidenceFreshnessInput,
) EvidenceFreshness {
	if !hasActiveContext {
		return EvidenceFreshnessUnknown
	}
	if len(inputs) == 0 {
		return EvidenceFreshnessUnknown
	}

	evaluated := false
	for _, item := range inputs {
		if item.EvidenceInputHash != "" && item.CurrentInputHash != "" {
			evaluated = true
			if item.EvidenceInputHash != item.CurrentInputHash {
				return EvidenceFreshnessStale
			}
		}
		if !item.EvidenceTimestamp.IsZero() && !item.LatestRelevantUpdateAt.IsZero() {
			evaluated = true
			if item.EvidenceTimestamp.Before(item.LatestRelevantUpdateAt) {
				return EvidenceFreshnessStale
			}
		}
	}
	if !evaluated {
		return EvidenceFreshnessUnknown
	}
	return EvidenceFreshnessFresh
}
