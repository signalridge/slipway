package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const ExecutionSummaryVersion = 1

type ExecutionVerdict string

const (
	ExecutionVerdictPass ExecutionVerdict = "pass"
	ExecutionVerdictFail ExecutionVerdict = "fail"
)

func (v ExecutionVerdict) IsValid() bool {
	switch v {
	case ExecutionVerdictPass, ExecutionVerdictFail:
		return true
	default:
		return false
	}
}

type ExecutionTaskSummary struct {
	TaskID       string      `yaml:"task_id" json:"task_id"`
	Verdict      TaskVerdict `yaml:"verdict" json:"verdict"`
	TaskKind     TaskKind    `yaml:"task_kind,omitempty" json:"task_kind,omitempty"`
	ChangedFiles []string    `yaml:"changed_files,omitempty" json:"changed_files,omitempty"`
	TargetFiles  []string    `yaml:"target_files,omitempty" json:"target_files,omitempty"`
	EvidenceRef  string      `yaml:"evidence_ref,omitempty" json:"evidence_ref,omitempty"`
	// NoOpJustification legitimizes a pass code task that changed zero files
	// because honest investigation concluded no safe behavior-preserving change
	// exists. It lives on evidence (never on the tasks-plan hash), so recording
	// it introduces no wave-execution staleness cascade.
	NoOpJustification string                       `yaml:"no_op_justification,omitempty" json:"no_op_justification,omitempty"`
	FreshnessInputs   ExecutionTaskFreshnessInputs `yaml:"freshness_inputs,omitempty" json:"freshness_inputs,omitempty"`
	// EvidenceInputHash is retained only so legacy hash-only summaries can be
	// diagnosed as stale instead of failing to parse. It is no longer a
	// freshness authority and new generated summaries should leave it empty.
	EvidenceInputHash string       `yaml:"evidence_input_hash,omitempty" json:"evidence_input_hash,omitempty"`
	Blockers          []ReasonCode `yaml:"blockers,omitempty" json:"blockers,omitempty"`
	CapturedAt        time.Time    `yaml:"captured_at,omitempty" json:"captured_at,omitempty"`
}

type ExecutionTaskFreshnessInputs struct {
	ChangeID          string `yaml:"change_id,omitempty" json:"change_id,omitempty"`
	RunSummaryVersion int    `yaml:"run_summary_version,omitempty" json:"run_summary_version,omitempty"`
	TaskID            string `yaml:"task_id,omitempty" json:"task_id,omitempty"`
	GuardrailDomain   string `yaml:"guardrail_domain,omitempty" json:"guardrail_domain,omitempty"`
	TasksPlanHash     string `yaml:"tasks_plan_hash,omitempty" json:"tasks_plan_hash,omitempty"`
}

func (i *ExecutionTaskFreshnessInputs) Normalize() {
	i.ChangeID = strings.TrimSpace(i.ChangeID)
	i.TaskID = strings.TrimSpace(i.TaskID)
	i.GuardrailDomain = strings.TrimSpace(i.GuardrailDomain)
	i.TasksPlanHash = strings.TrimSpace(i.TasksPlanHash)
}

func (i ExecutionTaskFreshnessInputs) Normalized() ExecutionTaskFreshnessInputs {
	out := i
	out.Normalize()
	return out
}

func (i ExecutionTaskFreshnessInputs) IsZero() bool {
	i = i.Normalized()
	return i.ChangeID == "" &&
		i.RunSummaryVersion == 0 &&
		i.TaskID == "" &&
		i.GuardrailDomain == "" &&
		i.TasksPlanHash == ""
}

func (i ExecutionTaskFreshnessInputs) Equal(other ExecutionTaskFreshnessInputs) bool {
	return i.Normalized() == other.Normalized()
}

func (i ExecutionTaskFreshnessInputs) FieldMap() map[string]string {
	i = i.Normalized()
	if i.IsZero() {
		return map[string]string{}
	}
	fields := map[string]string{
		"change_id":           i.ChangeID,
		"run_summary_version": fmt.Sprintf("%d", i.RunSummaryVersion),
		"task_id":             i.TaskID,
		"guardrail_domain":    i.GuardrailDomain,
	}
	if i.TasksPlanHash != "" {
		fields["tasks_plan_hash"] = i.TasksPlanHash
	}
	return fields
}

func (t *ExecutionTaskSummary) Normalize() {
	if t.ChangedFiles == nil {
		t.ChangedFiles = []string{}
	}
	if t.TargetFiles == nil {
		t.TargetFiles = []string{}
	}
	if t.Blockers == nil {
		t.Blockers = []ReasonCode{}
	}
	if !t.CapturedAt.IsZero() {
		t.CapturedAt = t.CapturedAt.Round(0).UTC()
	}
	t.NoOpJustification = strings.TrimSpace(t.NoOpJustification)
	t.FreshnessInputs.Normalize()
	slices.Sort(t.ChangedFiles)
	slices.Sort(t.TargetFiles)
	if len(t.Blockers) == 0 {
		t.Blockers = []ReasonCode{}
	} else {
		t.Blockers = NormalizeReasonCodes(t.Blockers)
	}
}

func (t ExecutionTaskSummary) Validate() error {
	if err := ValidateTaskID(t.TaskID); err != nil {
		return err
	}
	if !t.Verdict.IsValid() {
		return fmt.Errorf("invalid verdict %q", t.Verdict)
	}
	if t.TaskKind != "" && !t.TaskKind.IsValid() {
		return fmt.Errorf("invalid task_kind %q", t.TaskKind)
	}
	// Enforce the no_op_justification validity envelope on every task that reaches
	// Validate, not only at the evidence write gates. This closes the summary
	// read/save boundary: a hand-edited execution-summary.yaml cannot smuggle a
	// contradictory or out-of-envelope justification into durable state that
	// scope-contract then trusts for its changed-files exemption. It routes
	// through the same ValidateNoOpJustification authority as the record gates and
	// the task-evidence read boundary, so every boundary agrees.
	if err := t.ValidateNoOpJustification(len(t.ChangedFiles) > 0); err != nil {
		return err
	}
	for i, blocker := range t.Blockers {
		if err := blocker.Validate(); err != nil {
			return fmt.Errorf("blockers[%d]: %w", i, err)
		}
	}
	return nil
}

// RequiresChangedFiles reports whether this task must record at least one
// changed file to be a valid, scope-contract-consistent pass. It is the single
// authority shared by the scope-contract evaluation boundary and the evidence
// record-time gates, so every boundary agrees on which tasks may honestly
// change zero files.
//
// hasChangedFiles is the caller's normalized view of whether the task recorded
// any changed file. Callers that normalize paths (dropping empty or duplicate
// entries) pass their normalized count so an entry that normalizes away is not
// mistaken for a real change.
//
// Non-pass verdicts and verification/investigation kinds never require changed
// files. A code task is additionally exempt when it carries a no_op_justification
// and changed zero files (an honest behavior-preserving no-op); an unjustified
// empty code task stays required (fail-closed).
func (t ExecutionTaskSummary) RequiresChangedFiles(hasChangedFiles bool) bool {
	if t.Verdict != TaskVerdictPass {
		return false
	}
	switch t.TaskKind {
	case TaskKindVerification, TaskKindInvestigation:
		return false
	default:
		if t.TaskKind == TaskKindCode &&
			strings.TrimSpace(t.NoOpJustification) != "" && !hasChangedFiles {
			return false
		}
		return true
	}
}

// Sentinel errors returned by ValidateNoOpJustification so callers can map each
// failure mode to their own error surface (CLI error code, plain parser error).
var (
	// ErrNoOpJustificationWithChangedFiles reports the contradiction of a
	// no_op_justification riding alongside recorded changed files.
	ErrNoOpJustificationWithChangedFiles = errors.New("no_op_justification must not be combined with changed_files")
	// ErrNoOpJustificationInvalidTask reports a no_op_justification on a task
	// outside its only legitimate shape: a pass code task that changed zero files.
	ErrNoOpJustificationInvalidTask = errors.New("no_op_justification is valid only for a pass code task that changed zero files")
)

// ValidateNoOpJustification enforces that a no_op_justification rides only on the
// evidence shape it legitimizes — a pass code task that changed zero files. It is
// the single authority shared by the evidence record-time gates (manual and
// batch) and the persisted-evidence parser, so every write and read boundary
// agrees on the field's validity envelope and no boundary silently stores a
// contradictory or inert justification.
//
// hasChangedFiles is the caller's normalized view of whether the task recorded
// any changed file, matching the RequiresChangedFiles convention.
//
// An empty justification is always valid. A non-empty justification is rejected
// fail-closed when the task recorded changed files (contradiction) or when the
// task is not a pass code task (the field would be meaningless there).
func (t ExecutionTaskSummary) ValidateNoOpJustification(hasChangedFiles bool) error {
	if strings.TrimSpace(t.NoOpJustification) == "" {
		return nil
	}
	if hasChangedFiles {
		return ErrNoOpJustificationWithChangedFiles
	}
	if t.Verdict != TaskVerdictPass || t.TaskKind != TaskKindCode {
		return ErrNoOpJustificationInvalidTask
	}
	return nil
}

func (t ExecutionTaskSummary) ToTaskRun(runSummaryVersion int) TaskRun {
	return TaskRun{
		TaskID:            t.TaskID,
		RunSummaryVersion: runSummaryVersion,
		TaskKind:          t.TaskKind,
		Verdict:           t.Verdict,
		ChangedFiles:      append([]string(nil), t.ChangedFiles...),
		TargetFiles:       append([]string(nil), t.TargetFiles...),
		EvidenceRef:       strings.TrimSpace(t.EvidenceRef),
		Blockers:          append([]ReasonCode(nil), t.Blockers...),
	}
}

func (t ExecutionTaskSummary) Equal(other ExecutionTaskSummary) bool {
	left := t
	right := other
	left.Normalize()
	right.Normalize()

	return left.TaskID == right.TaskID &&
		left.Verdict == right.Verdict &&
		left.TaskKind == right.TaskKind &&
		left.EvidenceRef == right.EvidenceRef &&
		left.NoOpJustification == right.NoOpJustification &&
		left.FreshnessInputs.Equal(right.FreshnessInputs) &&
		left.EvidenceInputHash == right.EvidenceInputHash &&
		left.CapturedAt.Equal(right.CapturedAt) &&
		slices.Equal(left.ChangedFiles, right.ChangedFiles) &&
		slices.Equal(left.TargetFiles, right.TargetFiles) &&
		slices.Equal(left.Blockers, right.Blockers)
}

type ExecutionSummary struct {
	Version           int                    `yaml:"version" json:"version"`
	RunSummaryVersion int                    `yaml:"run_summary_version" json:"run_summary_version"`
	CapturedAt        time.Time              `yaml:"captured_at" json:"captured_at"`
	OverallVerdict    ExecutionVerdict       `yaml:"overall_verdict" json:"overall_verdict"`
	TasksPlanHash     string                 `yaml:"tasks_plan_hash,omitempty" json:"tasks_plan_hash,omitempty"`
	CompletedTasks    []string               `yaml:"completed_tasks,omitempty" json:"completed_tasks,omitempty"`
	NonPassTasks      []string               `yaml:"non_pass_tasks,omitempty" json:"non_pass_tasks,omitempty"`
	OpenBlockers      []ReasonCode           `yaml:"open_blockers,omitempty" json:"open_blockers,omitempty"`
	Tasks             []ExecutionTaskSummary `yaml:"tasks,omitempty" json:"tasks,omitempty"`
}

func (s *ExecutionSummary) Normalize() {
	if s.Version == 0 {
		s.Version = ExecutionSummaryVersion
	}
	if !s.CapturedAt.IsZero() {
		s.CapturedAt = s.CapturedAt.Round(0).UTC()
	}
	s.TasksPlanHash = strings.TrimSpace(s.TasksPlanHash)
	if s.CompletedTasks == nil {
		s.CompletedTasks = []string{}
	}
	if s.NonPassTasks == nil {
		s.NonPassTasks = []string{}
	}
	if s.OpenBlockers == nil {
		s.OpenBlockers = []ReasonCode{}
	}
	if s.Tasks == nil {
		s.Tasks = []ExecutionTaskSummary{}
	}
	slices.Sort(s.CompletedTasks)
	slices.Sort(s.NonPassTasks)
	if len(s.OpenBlockers) == 0 {
		s.OpenBlockers = []ReasonCode{}
	} else {
		s.OpenBlockers = NormalizeReasonCodes(s.OpenBlockers)
	}
	for i := range s.Tasks {
		s.Tasks[i].Normalize()
	}
	slices.SortFunc(s.Tasks, func(a, b ExecutionTaskSummary) int {
		return strings.Compare(a.TaskID, b.TaskID)
	})
}

func (s *ExecutionSummary) SyncDerivedFields() {
	if s == nil {
		return
	}

	completed := make([]string, 0, len(s.Tasks))
	nonPass := make([]string, 0, len(s.Tasks))
	for _, task := range s.Tasks {
		if task.Verdict == TaskVerdictPass && len(task.Blockers) == 0 {
			completed = append(completed, task.TaskID)
		} else {
			nonPass = append(nonPass, task.TaskID)
		}
	}

	s.CompletedTasks = sortedStringCopy(completed)
	s.NonPassTasks = sortedStringCopy(nonPass)
	if len(s.NonPassTasks) > 0 || len(s.OpenBlockers) > 0 {
		s.OverallVerdict = ExecutionVerdictFail
	} else {
		s.OverallVerdict = ExecutionVerdictPass
	}
}

func (s ExecutionSummary) Validate() error {
	if s.Version != ExecutionSummaryVersion {
		return fmt.Errorf("version must be %d", ExecutionSummaryVersion)
	}
	if s.RunSummaryVersion < 1 {
		return fmt.Errorf("run_summary_version must be >= 1")
	}
	if s.CapturedAt.IsZero() {
		return fmt.Errorf("captured_at is required")
	}
	if !s.OverallVerdict.IsValid() {
		return fmt.Errorf("invalid overall_verdict %q", s.OverallVerdict)
	}
	completed := make([]string, 0, len(s.Tasks))
	nonPass := make([]string, 0, len(s.Tasks))
	seenTaskIDs := make(map[string]struct{}, len(s.Tasks))
	for i, task := range s.Tasks {
		if err := task.Validate(); err != nil {
			return fmt.Errorf("tasks[%d]: %w", i, err)
		}
		if _, exists := seenTaskIDs[task.TaskID]; exists {
			return fmt.Errorf("duplicate task_id %q", task.TaskID)
		}
		seenTaskIDs[task.TaskID] = struct{}{}
		if task.Verdict == TaskVerdictPass && len(task.Blockers) == 0 {
			completed = append(completed, task.TaskID)
		} else {
			nonPass = append(nonPass, task.TaskID)
		}
	}
	for i, blocker := range s.OpenBlockers {
		if err := blocker.Validate(); err != nil {
			return fmt.Errorf("open_blockers[%d]: %w", i, err)
		}
	}
	if !slices.Equal(sortedStringCopy(s.CompletedTasks), sortedStringCopy(completed)) {
		return fmt.Errorf("completed_tasks must match pass-without-blockers tasks")
	}
	if !slices.Equal(sortedStringCopy(s.NonPassTasks), sortedStringCopy(nonPass)) {
		return fmt.Errorf("non_pass_tasks must match tasks with failing verdicts or open blockers")
	}
	expectedVerdict := ExecutionVerdictPass
	if len(nonPass) > 0 || len(s.OpenBlockers) > 0 {
		expectedVerdict = ExecutionVerdictFail
	}
	if s.OverallVerdict != expectedVerdict {
		return fmt.Errorf("overall_verdict must match derived task results and open blockers")
	}
	return nil
}

func (s ExecutionSummary) TaskRunMap() map[string]TaskRun {
	runs := make(map[string]TaskRun, len(s.Tasks))
	for _, task := range s.Tasks {
		runs[task.TaskID] = task.ToTaskRun(s.RunSummaryVersion)
	}
	return runs
}

func (s ExecutionSummary) Equal(other ExecutionSummary) bool {
	left := s
	right := other
	left.Normalize()
	right.Normalize()

	if left.Version != right.Version ||
		left.RunSummaryVersion != right.RunSummaryVersion ||
		!left.CapturedAt.Equal(right.CapturedAt) ||
		left.OverallVerdict != right.OverallVerdict ||
		left.TasksPlanHash != right.TasksPlanHash ||
		!slices.Equal(left.CompletedTasks, right.CompletedTasks) ||
		!slices.Equal(left.NonPassTasks, right.NonPassTasks) ||
		!slices.Equal(left.OpenBlockers, right.OpenBlockers) ||
		len(left.Tasks) != len(right.Tasks) {
		return false
	}

	for i := range left.Tasks {
		if !left.Tasks[i].Equal(right.Tasks[i]) {
			return false
		}
	}
	return true
}

func sortedStringCopy(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := append([]string(nil), values...)
	slices.Sort(out)
	return out
}
