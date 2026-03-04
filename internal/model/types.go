package model

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Level string

const (
	LevelL1 Level = "L1"
	LevelL2 Level = "L2"
	LevelL3 Level = "L3"
)

func (l Level) IsValid() bool {
	switch l {
	case LevelL1, LevelL2, LevelL3:
		return true
	default:
		return false
	}
}

type LevelSource string

const (
	LevelSourceAuto         LevelSource = "auto"
	LevelSourceUserSelected LevelSource = "user_selected"
)

func (s LevelSource) IsValid() bool {
	switch s {
	case LevelSourceAuto, LevelSourceUserSelected:
		return true
	default:
		return false
	}
}

type AdmissionStatus string

const (
	AdmissionStatusActive        AdmissionStatus = "active"
	AdmissionStatusDone          AdmissionStatus = "done"
	AdmissionStatusCancelled     AdmissionStatus = "cancelled"
	AdmissionStatusSealedHandoff AdmissionStatus = "sealed_handoff"
)

func (s AdmissionStatus) IsValid() bool {
	switch s {
	case AdmissionStatusActive, AdmissionStatusDone, AdmissionStatusCancelled, AdmissionStatusSealedHandoff:
		return true
	default:
		return false
	}
}

type ChangeStatus string

const (
	ChangeStatusActive    ChangeStatus = "active"
	ChangeStatusDone      ChangeStatus = "done"
	ChangeStatusCancelled ChangeStatus = "cancelled"
)

func (s ChangeStatus) IsValid() bool {
	switch s {
	case ChangeStatusActive, ChangeStatusDone, ChangeStatusCancelled:
		return true
	default:
		return false
	}
}

type WorkflowState string

const (
	StateS0Intake            WorkflowState = "S0_INTAKE"
	StateS1Analyze           WorkflowState = "S1_ANALYZE"
	StateS2Discover          WorkflowState = "S2_DISCOVER"
	StateS3ScopeConfirmation WorkflowState = "S3_SCOPE_CONFIRMATION"
	StateS4SpecBundle        WorkflowState = "S4_SPEC_BUNDLE"
	StateS5PlanAudit         WorkflowState = "S5_PLAN_AUDIT"
	StateS6RunWaves          WorkflowState = "S6_RUN_WAVES"
	StateS7Review            WorkflowState = "S7_REVIEW"
	StateS8Verify            WorkflowState = "S8_VERIFY"
	StateDone                WorkflowState = "DONE"
)

type LevelHistoryEvent struct {
	Level       Level       `yaml:"level" json:"level"`
	LevelSource LevelSource `yaml:"level_source" json:"level_source"`
	Reason      string      `yaml:"reason,omitempty" json:"reason,omitempty"`
	At          time.Time   `yaml:"at" json:"at"`
}

type IntakeAssessment struct {
	IntentType       string   `yaml:"intent_type" json:"intent_type"`
	IsExecutable     bool     `yaml:"is_executable" json:"is_executable"`
	Confidence       float64  `yaml:"confidence" json:"confidence"`
	ChangeTargets    []string `yaml:"change_targets" json:"change_targets"`
	IntendedDelta    string   `yaml:"intended_delta" json:"intended_delta"`
	AcceptanceAnchor string   `yaml:"acceptance_anchor" json:"acceptance_anchor"`
	BlockingUnknowns []string `yaml:"blocking_unknowns" json:"blocking_unknowns"`
	AuxiliarySignals []string `yaml:"auxiliary_signals,omitempty" json:"auxiliary_signals,omitempty"`
}

type RouteSnapshot struct {
	Scores            Scores   `yaml:"scores" json:"scores"`
	GuardrailDomain   string   `yaml:"guardrail_domain,omitempty" json:"guardrail_domain,omitempty"`
	RoutingRationale  []string `yaml:"routing_rationale,omitempty" json:"routing_rationale,omitempty"`
	BlockingConflicts []string `yaml:"blocking_conflicts,omitempty" json:"blocking_conflicts,omitempty"`
}

type ArtifactLifecycle string

const (
	ArtifactLifecycleDraft    ArtifactLifecycle = "draft"
	ArtifactLifecycleInReview ArtifactLifecycle = "in_review"
	ArtifactLifecycleApproved ArtifactLifecycle = "approved"
	ArtifactLifecycleFrozen   ArtifactLifecycle = "frozen"
	ArtifactLifecycleStale    ArtifactLifecycle = "stale"
)

func (s ArtifactLifecycle) IsValid() bool {
	switch s {
	case ArtifactLifecycleDraft, ArtifactLifecycleInReview, ArtifactLifecycleApproved, ArtifactLifecycleFrozen, ArtifactLifecycleStale:
		return true
	default:
		return false
	}
}

type ArtifactState struct {
	ID        string            `yaml:"id" json:"id"`
	Path      string            `yaml:"path,omitempty" json:"path,omitempty"`
	Version   int               `yaml:"version,omitempty" json:"version,omitempty"`
	State     ArtifactLifecycle `yaml:"state" json:"state"`
	UpdatedAt time.Time         `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type GateDecision string

const (
	GateDecisionApprove            GateDecision = "approve"
	GateDecisionReject             GateDecision = "reject"
	GateDecisionConditionalApprove GateDecision = "conditional_approve"
)

func (g GateDecision) IsValid() bool {
	switch g {
	case GateDecisionApprove, GateDecisionReject, GateDecisionConditionalApprove:
		return true
	default:
		return false
	}
}

type GateStatus string

const (
	GateStatusApproved GateStatus = "approved"
	GateStatusBlocked  GateStatus = "blocked"
	GateStatusPending  GateStatus = "pending"
)

func (s GateStatus) IsValid() bool {
	switch s {
	case GateStatusApproved, GateStatusBlocked, GateStatusPending:
		return true
	default:
		return false
	}
}

type GateRecord struct {
	GateID    string       `yaml:"gate_id" json:"gate_id"`
	Status    GateStatus   `yaml:"status" json:"status"`
	Decision  GateDecision `yaml:"decision,omitempty" json:"decision,omitempty"`
	Reasons   []string     `yaml:"reasons,omitempty" json:"reasons,omitempty"`
	UpdatedAt time.Time    `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type TaskVerdict string

const (
	TaskVerdictPass       TaskVerdict = "pass"
	TaskVerdictFail       TaskVerdict = "fail"
	TaskVerdictBlocked    TaskVerdict = "blocked"
	TaskVerdictTimeout    TaskVerdict = "timeout"
	TaskVerdictIncomplete TaskVerdict = "incomplete"
)

func (v TaskVerdict) IsValid() bool {
	switch v {
	case TaskVerdictPass, TaskVerdictFail, TaskVerdictBlocked, TaskVerdictTimeout, TaskVerdictIncomplete:
		return true
	default:
		return false
	}
}

type TaskRun struct {
	TaskID            string      `yaml:"task_id" json:"task_id"`
	RunSummaryVersion int         `yaml:"run_summary_version" json:"run_summary_version"`
	TaskKind          TaskKind    `yaml:"task_kind,omitempty" json:"task_kind,omitempty"`
	Verdict           TaskVerdict `yaml:"verdict" json:"verdict"`
	ChangedFiles      []string    `yaml:"changed_files,omitempty" json:"changed_files,omitempty"`
	TargetFiles       []string    `yaml:"target_files,omitempty" json:"target_files,omitempty"`
	EvidenceRef       string      `yaml:"evidence_ref,omitempty" json:"evidence_ref,omitempty"`
	Blockers          []string    `yaml:"blockers,omitempty" json:"blockers,omitempty"`
}

func (r TaskRun) Validate() error {
	if strings.TrimSpace(r.TaskID) == "" {
		return errors.New("task_id is required")
	}
	if r.RunSummaryVersion < 1 {
		return fmt.Errorf("run_summary_version must be >= 1: %d", r.RunSummaryVersion)
	}
	if !r.Verdict.IsValid() {
		return fmt.Errorf("invalid task verdict: %q", r.Verdict)
	}
	if r.TaskKind != "" && !r.TaskKind.IsValid() {
		return fmt.Errorf("invalid task_kind: %q", r.TaskKind)
	}
	return nil
}

type ActionEvent struct {
	Action    string            `yaml:"action" json:"action"`
	State     WorkflowState     `yaml:"state" json:"state"`
	Timestamp time.Time         `yaml:"timestamp" json:"timestamp"`
	Details   map[string]string `yaml:"details,omitempty" json:"details,omitempty"`
}

func TruncateLevelHistory(history []LevelHistoryEvent, maxEntries int) []LevelHistoryEvent {
	if maxEntries <= 0 || len(history) <= maxEntries {
		return history
	}
	return append([]LevelHistoryEvent(nil), history[len(history)-maxEntries:]...)
}

var taskRunKeyPattern = regexp.MustCompile(`^(.+)__rv([0-9]+)$`)

func BuildTaskRunKey(taskID string, runSummaryVersion int) (string, error) {
	if strings.TrimSpace(taskID) == "" {
		return "", errors.New("task_id is required")
	}
	if runSummaryVersion < 1 {
		return "", fmt.Errorf("run_summary_version must be >= 1: %d", runSummaryVersion)
	}
	return fmt.Sprintf("%s__rv%d", taskID, runSummaryVersion), nil
}

func ParseTaskRunKey(key string) (taskID string, runSummaryVersion int, err error) {
	m := taskRunKeyPattern.FindStringSubmatch(strings.TrimSpace(key))
	if len(m) != 3 {
		return "", 0, fmt.Errorf("invalid task_runs key %q: expected <task_id>__rv<run_summary_version>", key)
	}
	runSummaryVersion, err = strconv.Atoi(m[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid run_summary_version in key %q: %w", key, err)
	}
	if runSummaryVersion < 1 {
		return "", 0, fmt.Errorf("run_summary_version in key %q must be >= 1", key)
	}
	return m[1], runSummaryVersion, nil
}

func ValidateTaskRunMap(taskRuns map[string]TaskRun) error {
	for key, run := range taskRuns {
		if err := run.Validate(); err != nil {
			return fmt.Errorf("task_runs[%q]: %w", key, err)
		}

		taskID, version, err := ParseTaskRunKey(key)
		if err != nil {
			return err
		}
		if taskID != run.TaskID {
			return fmt.Errorf("task_runs key/payload mismatch for %q: key task_id=%q payload task_id=%q", key, taskID, run.TaskID)
		}
		if version != run.RunSummaryVersion {
			return fmt.Errorf(
				"task_runs key/payload mismatch for %q: key run_summary_version=%d payload run_summary_version=%d",
				key,
				version,
				run.RunSummaryVersion,
			)
		}
	}

	return nil
}

func InsertTaskRun(taskRuns map[string]TaskRun, run TaskRun) (map[string]TaskRun, error) {
	if err := run.Validate(); err != nil {
		return nil, err
	}
	key, err := BuildTaskRunKey(run.TaskID, run.RunSummaryVersion)
	if err != nil {
		return nil, err
	}
	if taskRuns == nil {
		taskRuns = map[string]TaskRun{}
	}

	existing, exists := taskRuns[key]
	if exists && !reflect.DeepEqual(existing, run) {
		return nil, fmt.Errorf("task_runs already contains %q with different payload", key)
	}
	taskRuns[key] = run
	return taskRuns, nil
}
