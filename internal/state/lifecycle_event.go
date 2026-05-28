package state

import (
	"bufio"
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
	if raw, readErr := os.ReadFile(path); readErr == nil {
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
	file, err := os.Open(path)
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
