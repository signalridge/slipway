package artifact

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/engine/wave"
)

// TasksContractStatus mirrors RequirementsContractStatus for tasks.md.
type TasksContractStatus string

const (
	TasksContractStatusValid   TasksContractStatus = "valid"
	TasksContractStatusInvalid TasksContractStatus = "invalid"
	TasksContractStatusMissing TasksContractStatus = "missing"
)

// TasksContractResult is the result of evaluating tasks.md substance.
type TasksContractResult struct {
	Status  TasksContractStatus
	Source  string
	Message string
}

// EvaluateTasksContract checks tasks.md for substance, not just presence: a
// tasks list that declares no task, fails to parse, or still carries the
// engine's placeholder objectives ("Pending task objective" / "Pending
// verification objective") is the mechanical scaffold the authoring skill must
// replace (issue #91). The engine owns structure; the skill owns substance.
func EvaluateTasksContract(bundleDir, slug string) (TasksContractResult, error) {
	sourcePath := ResolveArtifactPath(bundleDir, "tasks.md")
	if _, err := os.Stat(sourcePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return TasksContractResult{
				Status:  TasksContractStatusMissing,
				Source:  sourcePath,
				Message: "tasks.md is missing",
			}, nil
		}
		return TasksContractResult{}, err
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return TasksContractResult{}, err
	}

	if blockers := TaskSubstanceBlockers(string(raw)); len(blockers) > 0 {
		return TasksContractResult{
			Status:  TasksContractStatusInvalid,
			Source:  sourcePath,
			Message: fmt.Sprintf("tasks.md is not substantive: %s", strings.Join(blockers, "; ")),
		}, nil
	}

	return TasksContractResult{
		Status:  TasksContractStatusValid,
		Source:  sourcePath,
		Message: "tasks.md validated",
	}, nil
}

// TaskSubstanceBlockers returns substance problems in tasks.md. An empty slice
// means the tasks carry real objectives.
//
// It parses the checkbox-native task plan rather than scanning the whole file,
// so authoring-guidance comments and surrounding prose can neither trip nor
// satisfy the gate (issue #91 blocker): only real checklist task objectives are
// judged. A tasks.md that is empty, fails to parse, declares no task, or carries
// an empty/placeholder objective is rejected.
func TaskSubstanceBlockers(content string) []string {
	if strings.TrimSpace(content) == "" {
		return []string{"tasks.md is empty"}
	}
	plan, err := wave.ParseTaskPlan(content)
	if err != nil {
		return []string{fmt.Sprintf("tasks.md is not well-formed: %v", err)}
	}
	if len(plan.Tasks) == 0 {
		return []string{"tasks.md declares no checklist tasks; author concrete tasks"}
	}

	var blockers []string
	for _, task := range plan.Tasks {
		id := strings.TrimSpace(task.TaskID)
		if id == "" {
			id = "(unidentified task)"
		}
		objective := strings.TrimSpace(task.Objective)
		if objective == "" {
			blockers = append(blockers, fmt.Sprintf("task %s has no objective", id))
			continue
		}
		if LooksLikeTemplatePlaceholder(objective) {
			blockers = append(blockers, fmt.Sprintf(
				"task %s has a placeholder objective (%q); author a concrete objective", id, objective))
		}
	}
	return blockers
}
