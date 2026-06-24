package model

import (
	"sort"
	"strconv"
	"strings"
)

// This file defines the pure context-origin / fresh-context attestation grammar
// consumed by the independence gates of the context-origin attestation change.
// Every token rides VerificationRecord.References — the single inbound channel
// the engine consumes — and every parser here stays free of any
// cmd/tmpl/toolgen import, a sibling of the wave_execution.go token grammar.

// Canonical context-origin stage identifiers. A context handle self-describes
// the lifecycle stage it was produced under; these are the stage keys the
// cross-stage independence lattice compares. The review stage is deliberately
// shared by the selected S3 review set; the authority feeder supplies
// per-review-skill participant keys outside the model vocabulary.
const (
	StageContextExecutor    = "executor"
	StageContextPlanOrigin  = "plan_origin"
	StageContextAuditOrigin = "audit_origin"
	StageContextReview      = "review"
	StageContextFix         = "fix"

	// Deprecated: the folded S3 lifecycle no longer records or accepts
	// goal-stage context-origin evidence.
	StageContextGoal = "goal"
	// Deprecated: the folded S3 lifecycle no longer records or accepts
	// closeout-stage context-origin evidence.
	StageContextCloseout = "closeout"
)

// ContextOriginReferencePrefix is the literal prefix of a verification reference
// that records the per-stage context handle a stage verdict was produced under:
// context_origin:stage=<stage>=<handle>. The handle is the per-stage fresh
// context identifier the cross-stage independence gate compares across stages.
// It is named context_origin to avoid colliding with the unrelated
// review_context JSON object on the next/handoff surface.
const ContextOriginReferencePrefix = "context_origin:stage="

// PlanOriginReferencePrefix is the literal prefix of a plan-audit verification
// reference recording the context handle the plan author produced the bundle
// under: plan_origin:<handle>.
const PlanOriginReferencePrefix = "plan_origin:"

// AuditOriginReferencePrefix is the literal prefix of a plan-audit verification
// reference recording the context handle the plan auditor reviewed the bundle
// under: audit_origin:<handle>.
const AuditOriginReferencePrefix = "audit_origin:"

// ContextOriginHandle is the parsed decomposition of a context-origin handle
// reference token. Stage is the lifecycle stage the handle self-describes;
// Handle is the per-stage fresh-context identifier compared for distinctness.
type ContextOriginHandle struct {
	Stage  string
	Handle string
}

// isMultiValuedContextOriginStage reports whether a context-origin stage may
// legitimately record more than one distinct handle on a single record. Only
// the fix stage is multi-valued: a reviewer's record accumulates one
// context_origin:stage=fix handle per fresh-context repair subagent / batch, so
// multiple distinct fix handles are expected rather than ambiguous. Every other
// stage (review, plan_origin, audit_origin, executor) is single-valued and
// stays fail-closed on conflicting handles.
func isMultiValuedContextOriginStage(stage string) bool {
	return stage == StageContextFix
}

// ContextOriginHandlesFromVerification extracts the per-stage context handles a
// record attests via context_origin:stage=<stage>=<handle>, restricted to the
// single-valued stages. The result is keyed by stage. ok is false when no
// well-formed single-valued handle is present, or when the record carries
// conflicting handles for the same single-valued stage — ambiguous evidence
// fails closed (mirroring ExecutorAgentHandlesFromVerification) rather than
// letting the last reference win. A repeated identical handle for a stage is
// idempotent.
//
// Multi-valued stages (fix) are intentionally excluded: their references neither
// poison the whole-record parse on multiplicity nor land in this single-valued
// map. Use FixContextOriginHandleSetFromVerification to read the fix handle set.
func ContextOriginHandlesFromVerification(record VerificationRecord) (map[string]ContextOriginHandle, bool) {
	handles := map[string]ContextOriginHandle{}
	for _, ref := range record.References {
		stage, handle, ok := parseContextOriginReference(ref)
		if !ok {
			continue
		}
		if isMultiValuedContextOriginStage(stage) {
			// Multi-valued stage handles are read as a set elsewhere; they do
			// not participate in the single-valued fail-closed guard and are not
			// stored here.
			continue
		}
		if existing, seen := handles[stage]; seen && existing.Handle != handle {
			// Two references naming the same single-valued stage with different
			// handles are ambiguous; the gate fails closed rather than letting
			// the last reference win.
			return nil, false
		}
		handles[stage] = ContextOriginHandle{Stage: stage, Handle: handle}
	}
	if len(handles) == 0 {
		return nil, false
	}
	return handles, true
}

func parseContextOriginReference(raw string) (stage, handle string, ok bool) {
	raw = strings.Trim(strings.TrimSpace(raw), "\"'`.,;()[]{}")
	if !strings.HasPrefix(raw, ContextOriginReferencePrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(raw, ContextOriginReferencePrefix)
	// stage names carry no '=', so the first '=' separates the stage from the
	// handle; any '=' inside the handle itself is preserved.
	stage, handle, found := strings.Cut(rest, "=")
	if !found {
		return "", "", false
	}
	stage = strings.TrimSpace(stage)
	handle = strings.TrimSpace(handle)
	if stage == "" || handle == "" {
		return "", "", false
	}
	if isRetiredContextOriginStage(stage) {
		return "", "", false
	}
	return stage, handle, true
}

func isRetiredContextOriginStage(stage string) bool {
	switch stage {
	case StageContextGoal, StageContextCloseout:
		return true
	default:
		return false
	}
}

// PlanOriginHandleFromVerification extracts the single plan-author context handle
// a record attests via plan_origin:<handle>, stamped onto the StageContextPlanOrigin
// stage. ok is false when no well-formed handle is present, or when the record
// carries conflicting handles — ambiguous evidence fails closed. A repeated
// identical handle is idempotent.
func PlanOriginHandleFromVerification(record VerificationRecord) (ContextOriginHandle, bool) {
	return singleStageHandleFromVerification(record, PlanOriginReferencePrefix, StageContextPlanOrigin)
}

// AuditOriginHandleFromVerification extracts the single plan-auditor context handle
// a record attests via audit_origin:<handle>, stamped onto the StageContextAuditOrigin
// stage. ok is false when no well-formed handle is present, or when the record
// carries conflicting handles — ambiguous evidence fails closed. A repeated
// identical handle is idempotent.
func AuditOriginHandleFromVerification(record VerificationRecord) (ContextOriginHandle, bool) {
	return singleStageHandleFromVerification(record, AuditOriginReferencePrefix, StageContextAuditOrigin)
}

// ReviewContextOriginHandleFromVerification extracts the reviewer context handle
// a review-skill record attests via context_origin:stage=review=<handle>.
func ReviewContextOriginHandleFromVerification(record VerificationRecord) (ContextOriginHandle, bool) {
	handles, ok := ContextOriginHandlesFromVerification(record)
	if !ok {
		return ContextOriginHandle{}, false
	}
	handle, ok := handles[StageContextReview]
	if !ok || strings.TrimSpace(handle.Handle) == "" {
		return ContextOriginHandle{}, false
	}
	return handle, true
}

// FixContextOriginHandleSetFromVerification flattens every
// context_origin:stage=fix=<handle> reference a record attests into a deduped
// set of fix context handles. The fix stage is multi-valued — a reviewer's
// record accumulates one handle per fresh-context repair subagent / batch — so
// this NEVER fails closed on multiplicity; it simply collects the distinct
// handles. Blank handles are skipped by the parser. The result is never nil so
// callers can range over it safely; an absence of fix handles yields an empty
// set. This mirrors ExecutorParticipantHandleSetFromVerification.
func FixContextOriginHandleSetFromVerification(record VerificationRecord) map[string]struct{} {
	set := map[string]struct{}{}
	for _, ref := range record.References {
		stage, handle, ok := parseContextOriginReference(ref)
		if !ok || stage != StageContextFix {
			continue
		}
		set[handle] = struct{}{}
	}
	return set
}

func singleStageHandleFromVerification(record VerificationRecord, prefix, stage string) (ContextOriginHandle, bool) {
	var found ContextOriginHandle
	seen := false
	for _, ref := range record.References {
		handle, ok := parseStageOriginReference(ref, prefix)
		if !ok {
			continue
		}
		candidate := ContextOriginHandle{Stage: stage, Handle: handle}
		if seen && candidate != found {
			return ContextOriginHandle{}, false
		}
		found = candidate
		seen = true
	}
	if !seen {
		return ContextOriginHandle{}, false
	}
	return found, true
}

func parseStageOriginReference(raw, prefix string) (handle string, ok bool) {
	raw = strings.Trim(strings.TrimSpace(raw), "\"'`.,;()[]{}")
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	handle = strings.TrimSpace(strings.TrimPrefix(raw, prefix))
	if handle == "" {
		return "", false
	}
	return handle, true
}

// ExecutorParticipantHandleSetFromVerification flattens the per-wave, per-task
// executor agent handles (ExecutorAgentHandlesFromVerification) into a deduped
// set of executor context handles. The blank collapse value that
// ExecutorAgentHandlesFromVerification emits for conflicting same-wave/task
// handles is dropped — it is not a real handle. The result is never nil so
// callers can range over it safely; an absence of executor handles yields an
// empty set.
func ExecutorParticipantHandleSetFromVerification(record VerificationRecord) map[string]struct{} {
	set := map[string]struct{}{}
	for _, byTask := range ExecutorAgentHandlesFromVerification(record) {
		for _, handle := range byTask {
			if handle == "" {
				continue
			}
			set[handle] = struct{}{}
		}
	}
	return set
}

// ContextParticipant is a stage's contribution to the cross-stage context
// independence lattice. A stage contributes either a single handle (most
// stages) or a set of handles (the executor stage, whose participants are the
// per-task executor agents). Exactly one of Handle / HandleSet is populated.
type ContextParticipant struct {
	Handle    string
	HandleSet map[string]struct{}
}

// CrossStageContextCollisions returns the colliding stage pairs in the
// cross-stage context independence lattice, restricted to edges with at least
// one endpoint in ownedStages. Two participants collide when either both are
// single-handle participants sharing the same handle, or one is a single handle
// that is a member of the other's executor handle set. Each colliding pair is
// returned exactly once as a lexically ordered [stageA, stageB] tuple, and the
// returned slice is sorted for deterministic output. nil is returned when there
// are no collisions on an owned edge.
func CrossStageContextCollisions(participants map[string]ContextParticipant, ownedStages map[string]struct{}) [][2]string {
	stages := make([]string, 0, len(participants))
	for stage := range participants {
		stages = append(stages, stage)
	}
	sort.Strings(stages)

	var collisions [][2]string
	for i := 0; i < len(stages); i++ {
		for j := i + 1; j < len(stages); j++ {
			stageA, stageB := stages[i], stages[j]
			if _, ownedA := ownedStages[stageA]; !ownedA {
				if _, ownedB := ownedStages[stageB]; !ownedB {
					continue
				}
			}
			if participantsCollide(participants[stageA], participants[stageB]) {
				collisions = append(collisions, [2]string{stageA, stageB})
			}
		}
	}
	return collisions
}

func participantsCollide(a, b ContextParticipant) bool {
	switch {
	case a.Handle != "" && b.Handle != "":
		return a.Handle == b.Handle
	case a.Handle != "" && len(b.HandleSet) > 0:
		_, in := b.HandleSet[a.Handle]
		return in
	case b.Handle != "" && len(a.HandleSet) > 0:
		_, in := a.HandleSet[b.Handle]
		return in
	default:
		return false
	}
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
