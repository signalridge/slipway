package cmd

import (
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/engine/wave"
)

// buildWavePlan parses tasks.md and computes dependency-ordered waves.
// Returns nil if tasks.md is missing or empty (not an error — plan may not exist yet).
func buildWavePlan(root, artifactBundle string) *wavePlanView {
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

	totalTasks := 0
	waveViews := make([]waveView, len(waves))
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
		waveViews[i] = waveView{
			WaveIndex: i + 1,
			Tasks:     tasks,
		}
		totalTasks += len(w.Nodes)
	}

	return &wavePlanView{
		TotalTasks: totalTasks,
		WaveCount:  len(waves),
		Waves:      waveViews,
	}
}
