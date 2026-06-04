package enginecontext

import (
	"reflect"
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
	ExpectedStructuralInput map[string]string `json:"expected_structural_input,omitempty"`
	CurrentStructuralInput  map[string]string `json:"current_structural_input,omitempty"`
	// EvidenceTimestamp and LatestRelevantUpdateAt remain serialized legacy
	// metadata, but freshness is evaluated only from structural inputs. Ordering
	// checks that still need timestamps must live at their domain-specific gate.
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
		if len(item.ExpectedStructuralInput) > 0 || len(item.CurrentStructuralInput) > 0 {
			evaluated = true
			if !reflect.DeepEqual(item.ExpectedStructuralInput, item.CurrentStructuralInput) {
				return EvidenceFreshnessStale
			}
		}
	}
	if !evaluated {
		return EvidenceFreshnessUnknown
	}
	return EvidenceFreshnessFresh
}
