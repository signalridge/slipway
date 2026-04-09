package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type ChangeStatus string

const (
	ChangeStatusActive    ChangeStatus = "active"
	ChangeStatusDone      ChangeStatus = "done"
	ChangeStatusCancelled ChangeStatus = "cancelled"
)

func (s ChangeStatus) String() string { return string(s) }

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
	StateS0Intake  WorkflowState = "S0_INTAKE"
	StateS1Plan    WorkflowState = "S1_PLAN"
	StateS2Execute WorkflowState = "S2_EXECUTE"
	StateS3Review  WorkflowState = "S3_REVIEW"
	StateS4Verify  WorkflowState = "S4_VERIFY"
	StateDone      WorkflowState = "DONE"
)

func (s WorkflowState) String() string { return string(s) }

func (s WorkflowState) IsValid() bool {
	switch s {
	case StateS0Intake, StateS1Plan, StateS2Execute, StateS3Review, StateS4Verify, StateDone:
		return true
	default:
		return false
	}
}

// IntakeSubStep tracks progress within S0_INTAKE.
type IntakeSubStep string

const (
	IntakeSubStepNone     IntakeSubStep = ""
	IntakeSubStepClarify  IntakeSubStep = "clarify"
	IntakeSubStepResearch IntakeSubStep = "research"
	IntakeSubStepConfirm  IntakeSubStep = "confirm"
)

func (i IntakeSubStep) String() string { return string(i) }

func (i IntakeSubStep) IsValid() bool {
	switch i {
	case IntakeSubStepNone, IntakeSubStepClarify, IntakeSubStepResearch, IntakeSubStepConfirm:
		return true
	default:
		return false
	}
}

// IntakeEntrySubStep returns the initial intake sub-step (always clarify).
func IntakeEntrySubStep() IntakeSubStep {
	return IntakeSubStepClarify
}

// PlanSubStep tracks planning progress within S1_PLAN.
type PlanSubStep string

const (
	PlanSubStepNone     PlanSubStep = ""
	PlanSubStepResearch PlanSubStep = "research"
	PlanSubStepBundle   PlanSubStep = "bundle"
	PlanSubStepAudit    PlanSubStep = "audit"
	PlanSubStepValidate PlanSubStep = "validate"
)

func (p PlanSubStep) String() string { return string(p) }

func (p PlanSubStep) IsValid() bool {
	switch p {
	case PlanSubStepNone, PlanSubStepResearch,
		PlanSubStepBundle, PlanSubStepAudit, PlanSubStepValidate:
		return true
	default:
		return false
	}
}

// PlanEntrySubStep returns the initial planning sub-step based on discovery need.
func PlanEntrySubStep(needsDiscovery bool) PlanSubStep {
	if needsDiscovery {
		return PlanSubStepResearch
	}
	return PlanSubStepBundle
}

type ArtifactLifecycle string

const (
	ArtifactLifecycleDraft    ArtifactLifecycle = "draft"
	ArtifactLifecycleApproved ArtifactLifecycle = "approved"
	ArtifactLifecycleFrozen   ArtifactLifecycle = "frozen"
	ArtifactLifecycleStale    ArtifactLifecycle = "stale"
)

func (s ArtifactLifecycle) String() string { return string(s) }

func (s ArtifactLifecycle) IsValid() bool {
	switch s {
	case ArtifactLifecycleDraft, ArtifactLifecycleApproved, ArtifactLifecycleFrozen, ArtifactLifecycleStale:
		return true
	default:
		return false
	}
}

type ArtifactState struct {
	ID          string            `yaml:"id" json:"id"`
	Path        string            `yaml:"path,omitempty" json:"path,omitempty"`
	Version     int               `yaml:"version,omitempty" json:"version,omitempty"`
	State       ArtifactLifecycle `yaml:"state" json:"state"`
	ContentHash string            `yaml:"content_hash,omitempty" json:"content_hash,omitempty"`
	UpdatedAt   time.Time         `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type GateStatus string

const (
	GateStatusApproved GateStatus = "approved"
	GateStatusBlocked  GateStatus = "blocked"
	GateStatusPending  GateStatus = "pending"
)

func (s GateStatus) String() string { return string(s) }

func (s GateStatus) IsValid() bool {
	switch s {
	case GateStatusApproved, GateStatusBlocked, GateStatusPending:
		return true
	default:
		return false
	}
}

// GuardrailDomain identifies a higher-control governance domain.
type GuardrailDomain = string

const (
	GuardrailDomainAuthAuthZ            GuardrailDomain = "auth_authz"
	GuardrailDomainSecurityCredentials  GuardrailDomain = "security_credentials"
	GuardrailDomainPrivacyPII           GuardrailDomain = "privacy_pii"
	GuardrailDomainFinancialFlows       GuardrailDomain = "financial_flows"
	GuardrailDomainSchemaDataMigration  GuardrailDomain = "schema_data_migration"
	GuardrailDomainIrreversibleOps      GuardrailDomain = "irreversible_operations"
	GuardrailDomainExternalAPIContracts GuardrailDomain = "external_api_contracts"
)

type GateRecord struct {
	GateID      string       `yaml:"gate_id" json:"gate_id"`
	Status      GateStatus   `yaml:"status" json:"status"`
	ReasonCodes []ReasonCode `yaml:"reason_codes,omitempty" json:"reason_codes,omitempty"`
	UpdatedAt   time.Time    `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type TaskVerdict string

const (
	TaskVerdictPass       TaskVerdict = "pass"
	TaskVerdictFail       TaskVerdict = "fail"
	TaskVerdictBlocked    TaskVerdict = "blocked"
	TaskVerdictIncomplete TaskVerdict = "incomplete"
	TaskVerdictTimeout    TaskVerdict = "timeout"
)

func (v TaskVerdict) String() string { return string(v) }

func (v TaskVerdict) IsValid() bool {
	switch v {
	case TaskVerdictPass, TaskVerdictFail, TaskVerdictBlocked, TaskVerdictIncomplete, TaskVerdictTimeout:
		return true
	default:
		return false
	}
}

type TaskRun struct {
	TaskID            string       `yaml:"task_id" json:"task_id"`
	RunSummaryVersion int          `yaml:"run_summary_version" json:"run_summary_version"`
	TaskKind          TaskKind     `yaml:"task_kind,omitempty" json:"task_kind,omitempty"`
	Verdict           TaskVerdict  `yaml:"verdict" json:"verdict"`
	ChangedFiles      []string     `yaml:"changed_files,omitempty" json:"changed_files,omitempty"`
	TargetFiles       []string     `yaml:"target_files,omitempty" json:"target_files,omitempty"`
	EvidenceRef       string       `yaml:"evidence_ref,omitempty" json:"evidence_ref,omitempty"`
	Blockers          []ReasonCode `yaml:"blockers,omitempty" json:"blockers,omitempty"`
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
	for i, blocker := range r.Blockers {
		if err := blocker.Validate(); err != nil {
			return fmt.Errorf("blockers[%d]: %w", i, err)
		}
	}
	return nil
}

func ValidateTaskID(taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return errors.New("task_id is required")
	}
	if strings.Contains(taskID, "__rv") {
		return fmt.Errorf("task_id must not contain delimiter %q: %q", "__rv", taskID)
	}
	return nil
}

func BuildTaskRunKey(taskID string, runSummaryVersion int) (string, error) {
	if err := ValidateTaskID(taskID); err != nil {
		return "", err
	}
	if runSummaryVersion < 1 {
		return "", fmt.Errorf("run_summary_version must be >= 1: %d", runSummaryVersion)
	}
	return fmt.Sprintf("%s__rv%d", taskID, runSummaryVersion), nil
}
