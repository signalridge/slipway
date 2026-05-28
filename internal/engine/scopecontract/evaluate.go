package scopecontract

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/stringutil"
)

type Status string

const (
	StatusNotApplicable Status = "not_applicable"
	StatusPass          Status = "pass"
	StatusFail          Status = "fail"
)

const (
	ReasonScopeContractMissing             = "scope_contract_missing"
	ReasonScopeContractChangedFilesMissing = "scope_contract_changed_files_missing"
	ReasonScopeContractDrift               = "scope_contract_drift"
	ReasonScopeContractEvaluationFailed    = "scope_contract_evaluation_failed"
)

type Report struct {
	Status                  Status             `json:"status"`
	PlannedTargets          []string           `json:"planned_targets,omitempty"`
	ChangedFiles            []string           `json:"changed_files,omitempty"`
	OutOfScopeFiles         []string           `json:"out_of_scope_files,omitempty"`
	MissingContractTasks    []string           `json:"missing_contract_tasks,omitempty"`
	MissingChangedFileTasks []string           `json:"missing_changed_file_tasks,omitempty"`
	Blockers                []model.ReasonCode `json:"blockers,omitempty"`
	Diagnostics             []string           `json:"diagnostics,omitempty"`
}

func Evaluate(plan wave.TaskPlan, summary *model.ExecutionSummary) Report {
	report := Report{Status: StatusNotApplicable}
	if summary == nil || summary.RunSummaryVersion < 1 {
		return report
	}

	report.PlannedTargets = plannedTargets(plan)
	report.ChangedFiles = changedFiles(summary)

	if len(plan.Tasks) == 0 {
		report.Status = StatusFail
		report.Blockers = []model.ReasonCode{model.NewReasonCode(ReasonScopeContractMissing, "tasks.md")}
		return report
	}

	report.MissingContractTasks = tasksMissingContract(plan)
	report.MissingChangedFileTasks = tasksMissingChangedFiles(plan, summary)
	report.OutOfScopeFiles = outOfScopeFiles(report.PlannedTargets, report.ChangedFiles)

	if len(report.MissingContractTasks) > 0 {
		report.Blockers = append(report.Blockers, model.NewReasonCode(ReasonScopeContractMissing, strings.Join(report.MissingContractTasks, ",")))
	}
	if len(report.MissingChangedFileTasks) > 0 {
		report.Blockers = append(report.Blockers, model.NewReasonCode(ReasonScopeContractChangedFilesMissing, strings.Join(report.MissingChangedFileTasks, ",")))
	}
	if len(report.OutOfScopeFiles) > 0 {
		report.Blockers = append(report.Blockers, model.NewReasonCode(ReasonScopeContractDrift, strings.Join(report.OutOfScopeFiles, ",")))
	}

	report.Blockers = model.NormalizeReasonCodes(report.Blockers)
	if len(report.Blockers) > 0 {
		report.Status = StatusFail
		return report
	}
	report.Status = StatusPass
	return report
}

func EvaluateBundle(bundleDir string, summary *model.ExecutionSummary) (Report, error) {
	report := Report{Status: StatusNotApplicable}
	if summary == nil || summary.RunSummaryVersion < 1 {
		return report, nil
	}

	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md"))
	if err != nil {
		return Report{}, err
	}
	plan, err := wave.ParseTaskPlan(string(raw))
	if err != nil {
		return Report{}, err
	}
	return Evaluate(plan, summary), nil
}

func plannedTargets(plan wave.TaskPlan) []string {
	var targets []string
	for _, task := range plan.Tasks {
		for _, target := range task.TargetFiles {
			if normalized := normalizePathPattern(target); normalized != "" {
				targets = append(targets, normalized)
			}
		}
	}
	return stringutil.UniqueSorted(targets)
}

func changedFiles(summary *model.ExecutionSummary) []string {
	if summary == nil {
		return nil
	}
	var files []string
	for _, task := range summary.Tasks {
		for _, file := range task.ChangedFiles {
			if normalized := normalizePathPattern(file); normalized != "" {
				files = append(files, normalized)
			}
		}
	}
	return stringutil.UniqueSorted(files)
}

func tasksMissingContract(plan wave.TaskPlan) []string {
	var missing []string
	for _, task := range plan.Tasks {
		if len(plannedTargets(wave.TaskPlan{Tasks: []wave.TaskNode{task}})) == 0 {
			missing = append(missing, strings.TrimSpace(task.TaskID))
		}
	}
	return stringutil.UniqueSorted(missing)
}

func tasksMissingChangedFiles(plan wave.TaskPlan, summary *model.ExecutionSummary) []string {
	if summary == nil {
		return nil
	}
	summaryByTaskID := map[string]model.ExecutionTaskSummary{}
	for _, task := range summary.Tasks {
		summaryByTaskID[strings.TrimSpace(task.TaskID)] = task
	}
	var missing []string
	for _, task := range plan.Tasks {
		taskID := strings.TrimSpace(task.TaskID)
		if !plannedTaskRequiresChangedFiles(task) {
			continue
		}
		if _, ok := summaryByTaskID[taskID]; !ok {
			missing = append(missing, taskID)
		}
	}
	for _, task := range summary.Tasks {
		if !requiresChangedFiles(task) {
			continue
		}
		if len(changedFiles(&model.ExecutionSummary{Tasks: []model.ExecutionTaskSummary{task}})) == 0 {
			missing = append(missing, strings.TrimSpace(task.TaskID))
		}
	}
	return stringutil.UniqueSorted(missing)
}

func plannedTaskRequiresChangedFiles(task wave.TaskNode) bool {
	switch task.TaskKind {
	case model.TaskKindVerification, model.TaskKindInvestigation:
		return false
	default:
		return true
	}
}

func requiresChangedFiles(task model.ExecutionTaskSummary) bool {
	if task.Verdict != model.TaskVerdictPass {
		return false
	}
	switch task.TaskKind {
	case model.TaskKindVerification, model.TaskKindInvestigation:
		return false
	default:
		return true
	}
}

func outOfScopeFiles(targets, files []string) []string {
	if len(files) == 0 {
		return nil
	}
	var out []string
	for _, file := range files {
		if !matchesAnyTarget(targets, file) {
			out = append(out, file)
		}
	}
	return stringutil.UniqueSorted(out)
}

func matchesAnyTarget(targets []string, file string) bool {
	file = normalizePathPattern(file)
	for _, target := range targets {
		if matchesTarget(target, file) {
			return true
		}
	}
	return false
}

func matchesTarget(target, file string) bool {
	target = normalizePathPattern(target)
	file = normalizePathPattern(file)
	if target == "" || file == "" {
		return false
	}
	if target == file {
		return true
	}
	if strings.HasSuffix(target, "/**") {
		prefix := strings.TrimSuffix(target, "/**")
		return file == prefix || strings.HasPrefix(file, prefix+"/")
	}
	if strings.HasSuffix(target, "/") {
		return strings.HasPrefix(file, target)
	}
	if strings.Contains(target, "**") {
		matched, err := doubleStarMatch(target, file)
		return err == nil && matched
	}
	matched, err := path.Match(target, file)
	return err == nil && matched
}

func normalizePathPattern(input string) string {
	value := strings.TrimSpace(input)
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(value)
	value = strings.TrimPrefix(value, "./")
	for strings.Contains(value, "//") {
		value = strings.ReplaceAll(value, "//", "/")
	}
	cleaned := path.Clean(value)
	if cleaned == "." {
		return ""
	}
	if strings.HasSuffix(value, "/") && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned
}

func doubleStarMatch(pattern, name string) (bool, error) {
	var expr strings.Builder
	expr.WriteString("^")
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				expr.WriteString(".*")
				i += 2
				continue
			}
			expr.WriteString(`[^/]*`)
		case '?':
			expr.WriteString(`[^/]`)
		default:
			expr.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
		i++
	}
	expr.WriteString("$")
	return regexp.MatchString(expr.String(), name)
}

func (r Report) Clone() Report {
	return Report{
		Status:                  r.Status,
		PlannedTargets:          append([]string(nil), r.PlannedTargets...),
		ChangedFiles:            append([]string(nil), r.ChangedFiles...),
		OutOfScopeFiles:         append([]string(nil), r.OutOfScopeFiles...),
		MissingContractTasks:    append([]string(nil), r.MissingContractTasks...),
		MissingChangedFileTasks: append([]string(nil), r.MissingChangedFileTasks...),
		Blockers:                append([]model.ReasonCode(nil), r.Blockers...),
		Diagnostics:             append([]string(nil), r.Diagnostics...),
	}
}

func (r Report) String() string {
	return fmt.Sprintf("%s planned=%d changed=%d drift=%d", r.Status, len(r.PlannedTargets), len(r.ChangedFiles), len(r.OutOfScopeFiles))
}
