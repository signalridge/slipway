package status

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/signalridge/slipway/internal/engine/action"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

type Projection struct {
	SummaryBlockers   []model.ReasonCode
	Progress          *Progress
	EvidenceInventory EvidenceInventory
	GateStatus        map[string]model.GateRecord
	ArtifactDAG       []ArtifactNode
	Diagnostics       []string
}

type Progress struct {
	Percentage        int
	StageIndex        int
	StageTotal        int
	StageName         string
	CurrentWaveIndex  int
	CompletedWaves    int
	TotalWaves        int
	WavesByVerdict    map[string]int
	TasksCompleted    int
	TasksTotal        int
	TasksByVerdict    map[string]int
	RunSummaryVersion int
}

type EvidenceInventory struct {
	TaskEvidence    []EvidenceRef
	NonTaskEvidence []EvidenceRef
}

type EvidenceRef struct {
	Key  string
	Path string
}

type ArtifactNode struct {
	Name      string
	State     string
	DependsOn []string
	Ready     bool
}

func BuildProjection(
	root string,
	change model.Change,
	executionSummary *model.ExecutionSummary,
	evidenceRefs map[string]string,
	readiness progression.GovernanceReadiness,
	stageName func(model.WorkflowState, model.IntakeSubStep, model.PlanSubStep) string,
) (Projection, error) {
	projection := Projection{
		SummaryBlockers:   summaryBlockers(executionSummary),
		Progress:          executionProgress(root, change, executionSummary, stageName),
		EvidenceInventory: buildEvidenceInventory(executionSummary, evidenceRefs),
		GateStatus:        GateStatusFromEvaluations(readiness.GateEvaluations),
		ArtifactDAG:       artifactNodesFromProjection(readiness.ArtifactProjection),
		Diagnostics:       stringutil.UniqueSorted(readiness.Diagnostics),
	}

	if bundleProgress, ok := bundleProgress(root, change, stageName); ok && !state.ExecutionSummaryRelevantState(change.CurrentState) {
		projection.Progress = bundleProgress
	}

	return projection, nil
}

func summaryBlockers(summary *model.ExecutionSummary) []model.ReasonCode {
	if summary == nil || len(summary.OpenBlockers) == 0 {
		return nil
	}
	return append([]model.ReasonCode(nil), summary.OpenBlockers...)
}

func executionProgress(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	stageName func(model.WorkflowState, model.IntakeSubStep, model.PlanSubStep) string,
) *Progress {
	path := action.WorkflowPath(change.NeedsDiscovery)
	if len(path) == 0 {
		return nil
	}

	stageIndex := 0
	foundState := false
	for i, stateName := range path {
		if stateName == change.CurrentState {
			stageIndex = i
			foundState = true
			break
		}
	}
	if !foundState && change.CurrentState == model.StateDone && len(path) > 0 {
		stageIndex = len(path) - 1
	}

	byVerdict := map[string]int{}
	wavesByVerdict := map[string]int{}
	completed := 0
	total := 0
	latestRunVersion := 0
	completedWaves := 0
	totalWaves := 0
	currentWaveIndex := 0
	if summary != nil && summary.RunSummaryVersion >= 1 {
		latestRunVersion = summary.RunSummaryVersion
		for _, task := range summary.Tasks {
			total++
			byVerdict[string(task.Verdict)]++
			if task.Verdict == model.TaskVerdictPass && len(task.Blockers) == 0 {
				completed++
			}
		}
		if plan, err := state.LoadOptionalWavePlanForChange(root, change); err == nil && plan != nil {
			totalWaves = len(plan.Waves)
			if waveRuns, err := state.LoadOptionalWaveRuns(root, change.Slug, summary.RunSummaryVersion); err == nil {
				if len(waveRuns) == len(plan.Waves) && len(state.WaveTaskLinkageIssues(*plan, waveRuns)) == 0 {
					currentWaveIndex = state.ResumeWaveIndex(*plan, waveRuns)
					runByWave := make(map[int]model.WaveRun, len(waveRuns))
					for _, run := range waveRuns {
						runByWave[run.WaveIndex] = run
					}
					for _, plannedWave := range plan.Waves {
						verdict := model.WaveVerdictPending
						if run, ok := runByWave[plannedWave.WaveIndex]; ok {
							verdict = run.Verdict
						}
						wavesByVerdict[string(verdict)]++
						if verdict == model.WaveVerdictPass {
							completedWaves++
						}
					}
				}
			}
		}
	}

	stageTotal := len(path)
	stagePct := 0
	if stageTotal > 1 {
		stagePct = (stageIndex * 100) / (stageTotal - 1)
	}

	taskPct := 0
	if total > 0 {
		taskPct = (completed * 100) / total
	}

	percentage := stagePct
	if total > 0 {
		percentage = (stagePct*70 + taskPct*30) / 100
	}
	if change.CurrentState == model.StateDone {
		percentage = 100
	}

	progress := &Progress{
		Percentage:        percentage,
		StageIndex:        stageIndex,
		StageTotal:        stageTotal,
		StageName:         stageName(change.CurrentState, change.IntakeSubStep, change.PlanSubStep),
		CurrentWaveIndex:  currentWaveIndex,
		CompletedWaves:    completedWaves,
		TotalWaves:        totalWaves,
		TasksCompleted:    completed,
		TasksTotal:        total,
		RunSummaryVersion: latestRunVersion,
	}
	if len(byVerdict) > 0 {
		progress.TasksByVerdict = byVerdict
	}
	if len(wavesByVerdict) > 0 {
		progress.WavesByVerdict = wavesByVerdict
	}
	return progress
}

func bundleProgress(
	root string,
	change model.Change,
	stageName func(model.WorkflowState, model.IntakeSubStep, model.PlanSubStep) string,
) (*Progress, bool) {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return nil, false
	}

	taskPlan, err := readTaskPlanFromBundle(bundleDir)
	if err != nil || len(taskPlan.Tasks) == 0 {
		return nil, false
	}

	completed, total := checklistProgress(taskPlan)
	path := action.WorkflowPath(change.NeedsDiscovery)
	stageIndex := 0
	for i, stateName := range path {
		if stateName == change.CurrentState {
			stageIndex = i
			break
		}
	}

	stageTotal := len(path)
	stagePct := 0
	if stageTotal > 1 {
		stagePct = (stageIndex * 100) / (stageTotal - 1)
	}
	taskPct := 0
	if total > 0 {
		taskPct = (completed * 100) / total
	}
	percentage := stagePct
	if total > 0 {
		percentage = (stagePct*70 + taskPct*30) / 100
	}
	if change.CurrentState == model.StateDone {
		percentage = 100
	}

	return &Progress{
		Percentage:     percentage,
		StageIndex:     stageIndex,
		StageTotal:     stageTotal,
		StageName:      stageName(change.CurrentState, change.IntakeSubStep, change.PlanSubStep),
		TasksCompleted: completed,
		TasksTotal:     total,
	}, true
}

func checklistProgress(plan wave.TaskPlan) (completed, total int) {
	for _, task := range plan.Tasks {
		total++
		if task.Completed {
			completed++
		}
	}
	return
}

func readTaskPlanFromBundle(bundleDir string) (wave.TaskPlan, error) {
	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md")) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return wave.TaskPlan{}, err
	}
	return wave.ParseTaskPlan(string(raw))
}

func buildEvidenceInventory(summary *model.ExecutionSummary, nonTask map[string]string) EvidenceInventory {
	taskEvidence := make([]EvidenceRef, 0)
	if summary != nil && summary.RunSummaryVersion >= 1 {
		for _, task := range summary.Tasks {
			if task.EvidenceRef == "" {
				continue
			}
			key, err := model.BuildTaskRunKey(task.TaskID)
			if err != nil {
				continue
			}
			taskEvidence = append(taskEvidence, EvidenceRef{Key: key, Path: task.EvidenceRef})
		}
	}

	keys := make([]string, 0, len(nonTask))
	for key := range nonTask {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	nonTaskEvidence := make([]EvidenceRef, 0, len(keys))
	for _, key := range keys {
		nonTaskEvidence = append(nonTaskEvidence, EvidenceRef{Key: key, Path: nonTask[key]})
	}

	return EvidenceInventory{
		TaskEvidence:    taskEvidence,
		NonTaskEvidence: nonTaskEvidence,
	}
}

func GateStatusFromEvaluations(evaluations map[gate.GateID]gate.GateEvaluation) map[string]model.GateRecord {
	if len(evaluations) == 0 {
		return nil
	}
	keys := make([]string, 0, len(evaluations))
	for gateID := range evaluations {
		keys = append(keys, string(gateID))
	}
	slices.Sort(keys)

	status := make(map[string]model.GateRecord, len(keys))
	for _, key := range keys {
		eval := evaluations[gate.GateID(key)]
		status[key] = model.GateRecord{
			GateID:      string(eval.GateID),
			Status:      eval.Status,
			ReasonCodes: append([]model.ReasonCode(nil), eval.ReasonCodes...),
		}
	}
	return status
}

func artifactNodesFromProjection(projection *progression.ArtifactProjection) []ArtifactNode {
	if projection == nil || len(projection.Nodes) == 0 {
		return nil
	}

	nodes := make([]ArtifactNode, 0, len(projection.Nodes))
	for _, node := range projection.Nodes {
		if !node.Required {
			continue
		}
		nodes = append(nodes, ArtifactNode{
			Name:      node.Name,
			State:     node.State,
			DependsOn: append([]string(nil), node.DependsOn...),
			Ready:     node.Ready,
		})
	}
	return nodes
}
