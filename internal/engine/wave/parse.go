package wave

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

var taskCheckboxPattern = regexp.MustCompile(`^(\s*[-*]\s*\[)([ xX])(\]\s*)(.*)$`)

type TaskPlanFormat string

const (
	TaskPlanFormatCheckboxMarkdown TaskPlanFormat = "checkbox_markdown"
)

type TaskNode struct {
	Node
	Completed        bool     `json:"completed,omitempty"`
	Covers           []string `json:"covers,omitempty"`
	taskKindDeclared bool
}

type TaskPlan struct {
	Tasks  []TaskNode     `json:"tasks,omitempty"`
	Format TaskPlanFormat `json:"format,omitempty"`
}

func (p TaskPlan) Nodes() []Node {
	nodes := make([]Node, 0, len(p.Tasks))
	for _, task := range p.Tasks {
		nodes = append(nodes, task.Node)
	}
	return nodes
}

func (t TaskNode) HasDeclaredTaskKind() bool {
	return t.taskKindDeclared
}

func (t TaskNode) HasDeclaredWave() bool {
	return t.WaveIndex > 0
}

// ParseTaskPlan parses the steady-state checkbox-native tasks.md contract.
func ParseTaskPlan(content string) (TaskPlan, error) {
	return parseCheckboxTaskPlan(content)
}

func ApplyCompletedTaskCheckboxes(content string, completed map[string]bool) (string, bool, error) {
	if len(completed) == 0 || strings.TrimSpace(content) == "" {
		return content, false, nil
	}

	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		taskID, _, isCompleted, ok := parseTaskCheckboxLine(line)
		if !ok || taskID == "" {
			continue
		}
		if !completed[taskID] || isCompleted {
			continue
		}
		lines[i] = setTaskCheckboxState(line, true)
		changed = true
	}
	if !changed {
		return content, false, nil
	}
	return strings.Join(lines, "\n"), true, nil
}

func TaskPlanSemanticHash(content string) (string, error) {
	plan, err := ParseTaskPlan(content)
	if err != nil {
		return "", err
	}

	tasks := make([]any, 0, len(plan.Tasks))
	for _, task := range plan.Tasks {
		tasks = append(tasks, map[string]any{
			"task_id":         task.TaskID,
			"objective":       strings.TrimSpace(task.Objective),
			"wave":            task.WaveIndex,
			"depends_on":      append([]string(nil), task.DependsOn...),
			"target_files":    append([]string(nil), task.TargetFiles...),
			"task_kind":       task.TaskKind.String(),
			"covers":          append([]string(nil), task.Covers...),
			"checkpoint_type": strings.TrimSpace(task.CheckpointType),
		})
	}
	return model.ComputeInputHash(map[string]any{
		"format": string(plan.Format),
		"tasks":  tasks,
	})
}

// allowedMetadataKeys is the union of required + optional keys accepted by
// the checkbox-native contract.
var allowedMetadataKeys = map[string]struct{}{
	"wave":            {},
	"depends_on":      {},
	"target_files":    {},
	"task_kind":       {},
	"covers":          {},
	"checkpoint_type": {},
}

func parseCheckboxTaskPlan(content string) (TaskPlan, error) {
	var tasks []TaskNode
	var current *taskNodeBuilder

	seenTaskIDs := map[string]struct{}{}
	inCodeBlock := false

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		if taskID, objective, completed, ok := parseTaskCheckboxLine(line); ok {
			if current != nil {
				task, err := current.build()
				if err != nil {
					return TaskPlan{}, err
				}
				tasks = append(tasks, task)
			}
			if taskID != "" {
				if _, dup := seenTaskIDs[taskID]; dup {
					return TaskPlan{}, fmt.Errorf("duplicate task_id %q", taskID)
				}
				seenTaskIDs[taskID] = struct{}{}
			}
			current = &taskNodeBuilder{
				taskID:    taskID,
				objective: objective,
				completed: completed,
				seenKeys:  map[string]struct{}{},
			}
			continue
		}

		if current == nil {
			continue
		}

		key, value := parseMetadataLine(trimmed)
		if key == "" {
			continue
		}
		if err := current.applyStrictMetadata(key, value); err != nil {
			return TaskPlan{}, err
		}
	}

	if current != nil && current.taskID != "" {
		task, err := current.build()
		if err != nil {
			return TaskPlan{}, err
		}
		tasks = append(tasks, task)
	}

	if err := scanner.Err(); err != nil {
		return TaskPlan{}, fmt.Errorf("scanning tasks.md: %w", err)
	}
	return TaskPlan{Tasks: tasks, Format: TaskPlanFormatCheckboxMarkdown}, nil
}

type taskNodeBuilder struct {
	taskID         string
	objective      string
	waveIndex      int
	dependsOn      []string
	targetFiles    []string
	taskKind       string
	checkpointType string
	covers         []string
	completed      bool
	seenKeys       map[string]struct{}
}

// applyStrictMetadata enforces the checkbox-native contract: rejects unknown
// keys and fails on duplicate metadata keys.
func (b *taskNodeBuilder) applyStrictMetadata(key, value string) error {
	if _, allowed := allowedMetadataKeys[key]; !allowed {
		return fmt.Errorf("task %q uses unknown metadata key %q", b.taskID, key)
	}
	if _, dup := b.seenKeys[key]; dup {
		return fmt.Errorf("task %q has duplicate metadata key %q", b.taskID, key)
	}
	b.seenKeys[key] = struct{}{}
	return b.applyMetadata(key, value)
}

func (b *taskNodeBuilder) applyMetadata(key, value string) error {
	switch key {
	case "task_id":
		b.taskID = cleanValue(value)
	case "depends_on":
		b.dependsOn = parseStringList(value)
	case "target_files":
		b.targetFiles = parseStringList(value)
	case "task_kind":
		b.taskKind = cleanValue(value)
	case "covers":
		b.covers = parseStringList(value)
	case "objective":
		b.objective = cleanValue(value)
	case "checkpoint_type":
		b.checkpointType = cleanValue(value)
	case "wave":
		waveIndex, err := parsePositiveInt(value)
		if err != nil {
			return fmt.Errorf("task %q has invalid wave %q: %w", b.taskID, cleanValue(value), err)
		}
		b.waveIndex = waveIndex
	}
	return nil
}

func (b *taskNodeBuilder) build() (TaskNode, error) {
	var kind model.TaskKind
	switch strings.ToLower(b.taskKind) {
	case "code":
		kind = model.TaskKindCode
	case "test":
		kind = model.TaskKindTest
	case "doc":
		kind = model.TaskKindDoc
	case "ops":
		kind = model.TaskKindOps
	case "verification":
		kind = model.TaskKindVerification
	case "investigation":
		kind = model.TaskKindInvestigation
	case "other":
		kind = model.TaskKindOther
	case "":
		// Empty task_kind is normalized to Other for safe wave isolation and
		// default resume behavior.
		kind = model.TaskKindOther
	default:
		return TaskNode{}, fmt.Errorf("task %q has unknown task_kind %q", b.taskID, b.taskKind)
	}

	return TaskNode{
		Node: Node{
			TaskID:         b.taskID,
			Objective:      b.objective,
			WaveIndex:      b.waveIndex,
			DependsOn:      append([]string(nil), b.dependsOn...),
			TargetFiles:    append([]string(nil), b.targetFiles...),
			TaskKind:       kind,
			CheckpointType: b.checkpointType,
		},
		Completed:        b.completed,
		Covers:           append([]string(nil), b.covers...),
		taskKindDeclared: strings.TrimSpace(b.taskKind) != "",
	}, nil
}

func parseTaskCheckboxLine(line string) (string, string, bool, bool) {
	matches := taskCheckboxPattern.FindStringSubmatch(line)
	if len(matches) != 5 {
		return "", "", false, false
	}

	completed := strings.EqualFold(matches[2], "x")
	rest := strings.TrimSpace(matches[4])
	if rest == "" {
		return "", "", completed, true
	}

	if strings.HasPrefix(rest, "`") {
		if idx := strings.Index(rest[1:], "`"); idx >= 0 {
			taskID := strings.TrimSpace(rest[1 : idx+1])
			objective := trimTaskObjectiveSeparator(rest[idx+2:])
			return taskID, objective, completed, true
		}
	}

	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", "", completed, true
	}

	taskID := strings.Trim(parts[0], "`")
	if !looksLikeTaskID(taskID) {
		return "", "", false, false
	}
	objective := trimTaskObjectiveSeparator(strings.TrimPrefix(rest, parts[0]))
	return taskID, objective, completed, true
}

func trimTaskObjectiveSeparator(raw string) string {
	objective := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(objective, "-: "):
		return strings.TrimSpace(strings.TrimPrefix(objective, "-: "))
	case strings.HasPrefix(objective, "- "):
		return strings.TrimSpace(strings.TrimPrefix(objective, "- "))
	case strings.HasPrefix(objective, ": "):
		return strings.TrimSpace(strings.TrimPrefix(objective, ": "))
	default:
		return objective
	}
}

func looksLikeTaskID(candidate string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	return strings.HasPrefix(candidate, "t-") || strings.HasPrefix(candidate, "task-")
}

func setTaskCheckboxState(line string, completed bool) string {
	matches := taskCheckboxPattern.FindStringSubmatch(line)
	if len(matches) != 5 {
		return line
	}
	state := " "
	if completed {
		state = "x"
	}
	return matches[1] + state + matches[3] + matches[4]
}

// parseMetadataLine parses "- **key**: value" or "- key: value" patterns.
func parseMetadataLine(line string) (string, string) {
	switch {
	case strings.HasPrefix(line, "- "):
		line = strings.TrimPrefix(line, "- ")
	case strings.HasPrefix(line, "* "):
		line = strings.TrimPrefix(line, "* ")
	default:
		return "", ""
	}

	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "**") {
		line = strings.TrimPrefix(line, "**")
		if idx := strings.Index(line, "**"); idx > 0 {
			key := line[:idx]
			rest := strings.TrimPrefix(line[idx:], "**")
			line = key + rest
		}
	}

	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", ""
	}

	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	key = strings.Trim(key, "`")
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, " ", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return key, value
}

func cleanValue(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"'")
	return s
}

// parseStringList parses a JSON-like list or comma-separated values.
func parseStringList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" || s == "none" || s == "None" {
		return nil
	}

	if strings.HasPrefix(s, "[") {
		var items []string
		if err := json.Unmarshal([]byte(s), &items); err == nil {
			return items
		}
		s = strings.Trim(s, "[]")
	}

	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = cleanValue(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parsePositiveInt(s string) (int, error) {
	value, err := strconv.Atoi(cleanValue(s))
	if err != nil {
		return 0, err
	}
	if value < 1 {
		return 0, fmt.Errorf("must be >= 1")
	}
	return value, nil
}
