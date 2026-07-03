package state

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
)

const LifecycleEventLogFileName = "lifecycle.jsonl"

const lifecyclePredecessorContextMultiplier = 4

// LifecycleSideEffect records a runtime-owned mutation associated with a
// lifecycle event. It deliberately mirrors progression side effects without
// importing the progression package.
type LifecycleSideEffect struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// LifecycleEvent is an append-only audit record for governed lifecycle
// activity. It is a trace surface, not a second state authority; change.yaml
// remains the current lifecycle authority.
type LifecycleEvent struct {
	Version       int                   `json:"version"`
	EventID       string                `json:"event_id"`
	CorrelationID string                `json:"correlation_id,omitempty"`
	ChangeSlug    string                `json:"change_slug"`
	OccurredAt    time.Time             `json:"occurred_at"`
	Command       string                `json:"command,omitempty"`
	ActorKind     string                `json:"actor_kind,omitempty"`
	EventType     string                `json:"event_type"`
	Action        string                `json:"action,omitempty"`
	Reason        string                `json:"reason,omitempty"`
	Result        string                `json:"result,omitempty"`
	GateID        string                `json:"gate_id,omitempty"`
	ControlID     string                `json:"control_id,omitempty"`
	SkillID       string                `json:"skill_id,omitempty"`
	BeforeState   model.WorkflowState   `json:"before_state,omitempty"`
	AfterState    model.WorkflowState   `json:"after_state,omitempty"`
	BeforeSubStep string                `json:"before_substep,omitempty"`
	AfterSubStep  string                `json:"after_substep,omitempty"`
	Blockers      []model.ReasonCode    `json:"blockers,omitempty"`
	EvidenceRefs  map[string]string     `json:"evidence_refs,omitempty"`
	SideEffects   []LifecycleSideEffect `json:"side_effects,omitempty"`
	ClearedFields []string              `json:"cleared_fields,omitempty"`
	Diagnostics   []string              `json:"diagnostics,omitempty"`
}

// LifecycleEventLogPath resolves the governed bundle lifecycle event log path.
func LifecycleEventLogPath(root string, change model.Change) (string, error) {
	paths, err := ResolveChangePaths(root, change)
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.GovernedBundleDir, "events", LifecycleEventLogFileName), nil
}

func lifecycleEventLogPathForRead(root string, change model.Change) (string, error) {
	activePath, err := LifecycleEventLogPath(root, change)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(activePath); statErr == nil {
		return activePath, nil
	}
	paths, err := ResolveChangePaths(root, change)
	if err != nil {
		return "", err
	}
	archivedPath := filepath.Join(paths.GovernedBundleArchive, "events", LifecycleEventLogFileName)
	if _, statErr := os.Stat(archivedPath); statErr == nil {
		return archivedPath, nil
	}
	return activePath, nil
}

// AppendLifecycleEvent appends one lifecycle event and verifies it can be read
// back. The append is implemented as an atomic rewrite of the JSONL file so a
// crash leaves either the previous log or the previous log plus the new event.
func AppendLifecycleEvent(root string, change model.Change, event LifecycleEvent) (LifecycleEvent, error) {
	normalized, err := normalizeLifecycleEvent(change, event)
	if err != nil {
		return LifecycleEvent{}, err
	}
	path, err := LifecycleEventLogPath(root, change)
	if err != nil {
		return LifecycleEvent{}, err
	}

	var existing []byte
	if raw, readErr := os.ReadFile(path); readErr == nil { // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
		existing = raw
	} else if !os.IsNotExist(readErr) {
		return LifecycleEvent{}, readErr
	}
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		existing = append(existing, '\n')
	}

	line, err := json.Marshal(normalized)
	if err != nil {
		return LifecycleEvent{}, err
	}
	payload := append(append(existing, line...), '\n')
	if err := fsutil.WriteFileAtomic(path, payload, 0o644); err != nil {
		return LifecycleEvent{}, err
	}
	if err := verifyLifecycleEventReadback(path, normalized); err != nil {
		return LifecycleEvent{}, err
	}
	return normalized, nil
}

// ReadLifecycleEvents reads the governed lifecycle event log. Missing logs are
// treated as empty so older changes remain compatible.
func ReadLifecycleEvents(root string, change model.Change) ([]LifecycleEvent, error) {
	path, err := lifecycleEventLogPathForRead(root, change)
	if err != nil {
		return nil, err
	}
	return readLifecycleEventsFromPath(path)
}

// ReadLifecycleEventTailWithPredecessorTransitionFromPath reads the bounded
// tail plus the nearest earlier state.transitioned event when that predecessor
// is outside the retained tail. Display surfaces use the predecessor only as
// replay-classification context before trimming their final view.
func ReadLifecycleEventTailWithPredecessorTransitionFromPath(path string, limit int) ([]LifecycleEvent, error) {
	if limit <= 0 {
		return readLifecycleEventsFromPath(path)
	}
	return readLifecycleEventTailWithPredecessorTransitionFromPath(path, limit)
}

func normalizeLifecycleEvent(change model.Change, event LifecycleEvent) (LifecycleEvent, error) {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return LifecycleEvent{}, fmt.Errorf("change slug is required")
	}
	event.ChangeSlug = slug
	if event.Version == 0 {
		event.Version = 1
	}
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.CorrelationID == "" {
		event.CorrelationID = "corr-" + uuid.NewString()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}
	event.EventType = strings.TrimSpace(event.EventType)
	if event.EventType == "" {
		return LifecycleEvent{}, fmt.Errorf("lifecycle event_type is required")
	}
	event.Command = strings.TrimSpace(event.Command)
	event.ActorKind = strings.TrimSpace(event.ActorKind)
	if event.ActorKind == "" {
		event.ActorKind = "cli"
	}
	event.Action = strings.TrimSpace(event.Action)
	event.Reason = strings.TrimSpace(event.Reason)
	event.Result = strings.TrimSpace(event.Result)
	event.GateID = strings.TrimSpace(event.GateID)
	event.ControlID = strings.TrimSpace(event.ControlID)
	event.SkillID = strings.TrimSpace(event.SkillID)
	event.BeforeSubStep = strings.TrimSpace(event.BeforeSubStep)
	event.AfterSubStep = strings.TrimSpace(event.AfterSubStep)
	event.EvidenceRefs = normalizeLifecycleEvidenceRefs(event.EvidenceRefs)
	return event, nil
}

func normalizeLifecycleEvidenceRefs(refs map[string]string) map[string]string {
	if len(refs) == 0 {
		return nil
	}
	out := make(map[string]string, len(refs))
	for key, value := range refs {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func verifyLifecycleEventReadback(path string, expected LifecycleEvent) error {
	events, err := readLifecycleEventsFromPath(path)
	if err != nil {
		return fmt.Errorf("read back lifecycle event log: %w", err)
	}
	for _, event := range events {
		if event.EventID == expected.EventID && event.ChangeSlug == expected.ChangeSlug {
			return nil
		}
	}
	return fmt.Errorf("lifecycle event %q missing after append", expected.EventID)
}

func readLifecycleEventsFromPath(path string) ([]LifecycleEvent, error) {
	file, err := os.Open(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var events []LifecycleEvent
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event LifecycleEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode lifecycle event log line %d: %w", lineNumber, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func readLifecycleEventTailWithPredecessorTransitionFromPath(path string, limit int) ([]LifecycleEvent, error) {
	file, err := os.Open(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return nil, nil
	}

	const chunkSize int64 = 64 * 1024
	contextLimit := lifecyclePredecessorContextLimit(limit)
	offset := info.Size()
	var raw []byte
	var lines []string
	var predecessor *LifecycleEvent
	for offset > 0 {
		readSize := chunkSize
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		chunk := make([]byte, readSize)
		if _, err := file.ReadAt(chunk, offset); err != nil {
			return nil, err
		}
		raw = append(append([]byte(nil), chunk...), raw...)
		lines = nonEmptyJSONLLines(raw)
		if len(lines) <= limit && offset > 0 {
			continue
		}

		tailStart := len(lines) - limit
		if tailStart < 0 {
			tailStart = 0
		}
		contextStart := 0
		if offset > 0 {
			// The first line can be a partial record until the scan reaches the
			// file start. Do not parse it as context before it is complete.
			contextStart = 1
		}
		searchStart := tailStart - contextLimit
		if searchStart < contextStart {
			searchStart = contextStart
		}
		for i := tailStart - 1; i >= searchStart; i-- {
			event, err := decodeLifecycleEventContextLine(lines[i], tailStart-i)
			if err != nil {
				return nil, err
			}
			if event.EventType == "state.transitioned" {
				copy := event
				predecessor = &copy
				break
			}
		}
		if predecessor != nil || offset == 0 || tailStart-contextStart >= contextLimit {
			break
		}
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	events, err := decodeLifecycleEventLines(lines, "tail")
	if err != nil {
		return nil, err
	}
	if predecessor == nil {
		return events, nil
	}
	withContext := make([]LifecycleEvent, 0, len(events)+1)
	withContext = append(withContext, *predecessor)
	withContext = append(withContext, events...)
	return withContext, nil
}

func lifecyclePredecessorContextLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	return limit * lifecyclePredecessorContextMultiplier
}

func decodeLifecycleEventLines(lines []string, context string) ([]LifecycleEvent, error) {
	events := make([]LifecycleEvent, 0, len(lines))
	for i, line := range lines {
		var event LifecycleEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode lifecycle event log %s line %d: %w", context, i+1, err)
		}
		events = append(events, event)
	}
	return events, nil
}

func decodeLifecycleEventContextLine(line string, distanceFromTail int) (LifecycleEvent, error) {
	var event LifecycleEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return LifecycleEvent{}, fmt.Errorf("decode lifecycle event log context line %d before tail: %w", distanceFromTail, err)
	}
	return event, nil
}

func nonEmptyJSONLLines(raw []byte) []string {
	parts := bytes.Split(raw, []byte{'\n'})
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(string(part))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
