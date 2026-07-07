package scopecontract

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/stringutil"
	"github.com/signalridge/slipway/internal/wave"
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
	Status                  Status   `json:"status"`
	PlannedTargets          []string `json:"planned_targets,omitempty"`
	ChangedFiles            []string `json:"changed_files,omitempty"`
	OutOfScopeFiles         []string `json:"out_of_scope_files,omitempty"`
	MissingContractTasks    []string `json:"missing_contract_tasks,omitempty"`
	MissingChangedFileTasks []string `json:"missing_changed_file_tasks,omitempty"`
	// ExemptContextFiles discloses the dirty codebase-map (artifacts/codebase/**)
	// files the scope-contract filter intentionally drops from ChangedFiles. The
	// exemption is preserved (these files stay out of ChangedFiles /
	// OutOfScopeFiles and never affect Status); this field only makes the
	// otherwise-silent exemption observable.
	ExemptContextFiles []string `json:"exempt_context_files,omitempty"`
	// NoOpJustifiedTasks discloses the pass code tasks that the Scope Contract
	// exempted from the changed-files requirement because they carry a
	// no_op_justification and changed zero files (an honest behavior-preserving
	// no-op). Like ExemptContextFiles, this exemption is otherwise silent — the
	// task simply never appears in MissingChangedFileTasks — so this field makes
	// the justification observable to a reviewer without reading raw evidence. It
	// is observational only and never affects Status.
	NoOpJustifiedTasks []NoOpJustifiedTask `json:"no_op_justified_tasks,omitempty"`
	Blockers           []model.ReasonCode  `json:"blockers,omitempty"`
}

// NoOpJustifiedTask pairs a task with the no_op_justification that earned its
// changed-files exemption, for observable disclosure in the scope-contract
// Report.
type NoOpJustifiedTask struct {
	TaskID            string `json:"task_id"`
	NoOpJustification string `json:"no_op_justification"`
}

// EvaluateWithChangedFiles scores a plan/summary pair against the Scope Contract.
//
// Precondition: summary must already be Load-validated (each task has passed
// ExecutionTaskSummary.Validate). Evaluate trusts the no_op_justification
// envelope enforced at that load boundary and does not re-validate it here — the
// only production callers reach this through LoadExecutionSummary, which fails
// closed on an out-of-envelope justification before a summary can arrive. A new
// caller that constructs a summary in memory must Validate it first rather than
// rely on Evaluate to catch a contradictory or inert justification.
func EvaluateWithChangedFiles(plan wave.TaskPlan, summary *model.ExecutionSummary, extraChangedFiles []string) Report {
	report := Report{Status: StatusNotApplicable}
	if summary == nil || summary.RunSummaryVersion < 1 {
		return report
	}

	report.PlannedTargets = plannedTargets(plan)
	report.ChangedFiles = mergeChangedFiles(changedFiles(summary), extraChangedFiles)

	if len(plan.Tasks) == 0 {
		report.Status = StatusFail
		report.Blockers = []model.ReasonCode{model.NewReasonCode(ReasonScopeContractMissing, "tasks.md")}
		return report
	}

	report.MissingContractTasks = tasksMissingContract(plan)
	report.MissingChangedFileTasks = tasksMissingChangedFiles(plan, summary)
	report.OutOfScopeFiles = outOfScopeFiles(report.PlannedTargets, report.ChangedFiles)
	report.NoOpJustifiedTasks = noOpJustifiedTasks(summary)

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

func EvaluateBundleWithChangedFiles(bundleDir string, summary *model.ExecutionSummary, extraChangedFiles []string) (Report, error) {
	report := Report{Status: StatusNotApplicable}
	if summary == nil || summary.RunSummaryVersion < 1 {
		return report, nil
	}

	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md")) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return Report{}, err
	}
	plan, err := wave.ParseTaskPlan(string(raw))
	if err != nil {
		return Report{}, err
	}
	return EvaluateWithChangedFiles(plan, summary, extraChangedFiles), nil
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

func mergeChangedFiles(left, right []string) []string {
	files := make([]string, 0, len(left)+len(right))
	files = append(files, left...)
	for _, file := range right {
		if normalized := normalizePathPattern(file); normalized != "" {
			files = append(files, normalized)
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
	// Normalize the changed-file count exactly as tasksMissingChangedFiles does
	// (drop empty/duplicate entries) so an empty-but-justified task is not
	// flagged, then defer the verdict/kind/justification decision to the shared
	// model authority that the evidence record-time gates also use.
	hasFiles := len(changedFiles(&model.ExecutionSummary{Tasks: []model.ExecutionTaskSummary{task}})) > 0
	return task.RequiresChangedFiles(hasFiles)
}

// noOpJustifiedTasks returns, sorted by task ID, the tasks whose
// no_op_justification is load-bearing: a pass code task that changed zero files,
// which RequiresChangedFiles would otherwise require files from. It mirrors the
// ValidateNoOpJustification envelope so a contradictory or out-of-envelope
// justification (which the load boundary already rejects) is never disclosed as
// a valid exemption.
func noOpJustifiedTasks(summary *model.ExecutionSummary) []NoOpJustifiedTask {
	if summary == nil {
		return nil
	}
	var out []NoOpJustifiedTask
	for _, task := range summary.Tasks {
		justification := strings.TrimSpace(task.NoOpJustification)
		if justification == "" {
			continue
		}
		hasFiles := len(changedFiles(&model.ExecutionSummary{Tasks: []model.ExecutionTaskSummary{task}})) > 0
		if hasFiles || task.Verdict != model.TaskVerdictPass || task.TaskKind != model.TaskKindCode {
			continue
		}
		out = append(out, NoOpJustifiedTask{
			TaskID:            strings.TrimSpace(task.TaskID),
			NoOpJustification: justification,
		})
	}
	slices.SortFunc(out, func(a, b NoOpJustifiedTask) int {
		return strings.Compare(a.TaskID, b.TaskID)
	})
	return out
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
	value := model.NormalizePublicPath(input)
	if value == "" {
		return ""
	}
	cleaned := value
	if strings.HasSuffix(strings.ReplaceAll(input, "\\", "/"), "/") && !strings.HasSuffix(cleaned, "/") {
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
		ExemptContextFiles:      append([]string(nil), r.ExemptContextFiles...),
		Blockers:                append([]model.ReasonCode(nil), r.Blockers...),
	}
}
