package model

import (
	"strconv"
	"strings"
)

// This file defines the pure context-origin / fresh-context attestation grammar
// consumed by the independence gates of the context-origin attestation change.
// Every token rides VerificationRecord.References — the single inbound channel
// the engine consumes — and every parser here stays free of any
// cmd/tmpl/toolgen import, a sibling of the wave_execution.go token grammar.

// ReviewOriginReferencePrefix is the literal prefix of a review verification
// reference that records the per-review context handle a review verdict was
// produced under: review_origin:skill=<skill>=<handle>. The handle is the
// per-review context identifier the distinct-context gate compares across the
// spec-compliance-review / code-quality-review pair. It is named review_origin to
// avoid colliding with the unrelated review_context JSON object on the
// next/handoff surface.
const ReviewOriginReferencePrefix = "review_origin:skill="

// ReviewOriginHandle is the parsed decomposition of a review-context handle
// reference token. Skill is the review skill the handle self-describes; Handle is
// the per-review context identifier compared for distinctness.
type ReviewOriginHandle struct {
	Skill  string
	Handle string
}

// ReviewOriginHandleFromVerification extracts the single review-context handle a
// review record attests via review_origin:skill=<skill>=<handle>. ok is false
// when no well-formed handle is present, or when the record carries conflicting
// handles — ambiguous evidence fails closed (mirroring
// ExecutorAgentHandlesFromVerification) rather than letting the last reference
// win. A repeated identical handle is idempotent.
func ReviewOriginHandleFromVerification(record VerificationRecord) (ReviewOriginHandle, bool) {
	var found ReviewOriginHandle
	seen := false
	for _, ref := range record.References {
		skill, handle, ok := parseReviewOriginReference(ref)
		if !ok {
			continue
		}
		candidate := ReviewOriginHandle{Skill: skill, Handle: handle}
		if seen && candidate != found {
			return ReviewOriginHandle{}, false
		}
		found = candidate
		seen = true
	}
	if !seen {
		return ReviewOriginHandle{}, false
	}
	return found, true
}

func parseReviewOriginReference(raw string) (skill, handle string, ok bool) {
	raw = strings.Trim(strings.TrimSpace(raw), "\"'`.,;()[]{}")
	if !strings.HasPrefix(raw, ReviewOriginReferencePrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(raw, ReviewOriginReferencePrefix)
	// skill names carry no '=', so the first '=' separates the skill from the
	// handle; any '=' inside the handle itself is preserved.
	skill, handle, found := strings.Cut(rest, "=")
	if !found {
		return "", "", false
	}
	skill = strings.TrimSpace(skill)
	handle = strings.TrimSpace(handle)
	if skill == "" || handle == "" {
		return "", "", false
	}
	return skill, handle, true
}

// WaveDegradedJustificationReferencePrefix is the literal prefix of a
// wave-orchestration verification reference that justifies a degraded_sequential
// dispatch with a genuine tool-unavailable signal:
// degraded_dispatch_justification:wave=<wave_index>:tool_unavailable=<detail>.
// A bare degraded_sequential dispatch_mode token is no longer self-sufficient;
// the dispatch gate accepts it only when paired with this justification for the
// same wave.
const WaveDegradedJustificationReferencePrefix = "degraded_dispatch_justification:wave="

const waveDegradedJustificationToolUnavailableSeparator = ":tool_unavailable="

// DegradedDispatchJustificationsFromVerification returns the set of wave indices
// that recorded a genuine tool-unavailable justification for a
// degraded_sequential dispatch. A justification with an empty detail is not a
// genuine signal and is ignored.
func DegradedDispatchJustificationsFromVerification(record VerificationRecord) map[int]struct{} {
	justified := map[int]struct{}{}
	for _, ref := range record.References {
		waveIndex, ok := parseWaveDegradedJustificationReference(ref)
		if !ok {
			continue
		}
		justified[waveIndex] = struct{}{}
	}
	if len(justified) == 0 {
		return nil
	}
	return justified
}

func parseWaveDegradedJustificationReference(raw string) (waveIndex int, ok bool) {
	raw = strings.Trim(strings.TrimSpace(raw), "\"'`.,;()[]{}")
	if !strings.HasPrefix(raw, WaveDegradedJustificationReferencePrefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(raw, WaveDegradedJustificationReferencePrefix)
	waveRaw, detail, found := strings.Cut(rest, waveDegradedJustificationToolUnavailableSeparator)
	if !found {
		return 0, false
	}
	index, err := strconv.Atoi(strings.TrimSpace(waveRaw))
	if err != nil || index < 1 {
		return 0, false
	}
	if strings.TrimSpace(detail) == "" {
		return 0, false
	}
	return index, true
}
