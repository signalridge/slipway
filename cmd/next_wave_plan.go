package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// buildWavePlan returns the authoritative persisted wave plan during governed
// execution, and falls back to a derived preview from tasks.md before the
// authoritative plan exists.
func buildWavePlan(root string, change *model.Change, artifactBundle string) *wavePlanView {
	if change != nil && change.CurrentState == model.StateS2Execute {
		return authoritativeWavePlanView(root, *change)
	}
	return derivedWavePlanView(root, artifactBundle)
}

func authoritativeWavePlanView(root string, change model.Change) *wavePlanView {
	plan, err := state.LoadOptionalWavePlanForChange(root, change)
	switch {
	case err == nil && plan != nil:
		return wavePlanViewFromModel(*plan)
	case err == nil:
		return &wavePlanView{ParseError: "authoritative wave-plan.yaml is missing; run `slipway repair`"}
	case errors.Is(err, fs.ErrNotExist):
		return &wavePlanView{ParseError: "authoritative wave-plan.yaml is missing; run `slipway repair`"}
	default:
		return &wavePlanView{ParseError: fmt.Sprintf("failed to load authoritative wave-plan.yaml: %v", err)}
	}
}

// derivedWavePlanView parses tasks.md and computes dependency-ordered waves.
// Returns nil if tasks.md is missing or empty (not an error — plan may not exist yet).
func derivedWavePlanView(root, artifactBundle string) *wavePlanView {
	if artifactBundle == "" {
		return nil
	}
	tasksPath := filepath.Join(resolveInputContextPath(root, root, artifactBundle), "tasks.md")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		return nil
	}

	plan, err := wave.ParseTaskPlan(string(content))
	if err != nil {
		return &wavePlanView{ParseError: err.Error()}
	}
	nodes := plan.Nodes()
	if len(nodes) == 0 {
		return nil
	}

	waves, err := wave.PlanWaves(nodes)
	if err != nil {
		return &wavePlanView{
			TotalTasks: len(nodes),
			ParseError: err.Error(),
		}
	}

	planView := &wavePlanView{
		WaveCount: len(waves),
		Waves:     make([]waveView, len(waves)),
	}

	for i, w := range waves {
		tasks := make([]waveTaskView, len(w.Nodes))
		for j, n := range w.Nodes {
			tasks[j] = waveTaskView{
				TaskID:      n.TaskID,
				Objective:   n.Objective,
				DependsOn:   n.DependsOn,
				TargetFiles: n.TargetFiles,
				TaskKind:    string(n.TaskKind),
			}
		}
		planView.Waves[i] = waveView{
			WaveIndex: i + 1,
			Tasks:     tasks,
		}
		planView.TotalTasks += len(w.Nodes)
	}

	return planView
}

func wavePlanViewFromModel(plan model.WavePlan) *wavePlanView {
	plan.Normalize()
	if len(plan.Waves) == 0 {
		return nil
	}

	view := &wavePlanView{
		TotalTasks: plan.TotalTasks,
		WaveCount:  len(plan.Waves),
		Waves:      make([]waveView, len(plan.Waves)),
	}
	for i, plannedWave := range plan.Waves {
		tasks := make([]waveTaskView, len(plannedWave.Tasks))
		for j, task := range plannedWave.Tasks {
			tasks[j] = waveTaskView{
				TaskID:      task.TaskID,
				Objective:   task.Objective,
				DependsOn:   append([]string(nil), task.DependsOn...),
				TargetFiles: append([]string(nil), task.TargetFiles...),
				TaskKind:    string(task.TaskKind),
			}
		}
		view.Waves[i] = waveView{
			WaveIndex: plannedWave.WaveIndex,
			Tasks:     tasks,
		}
	}
	return view
}
