package model

import (
	"fmt"
	"strings"
	"time"
)

// Change is the single, unified lifecycle object for governed work.
// The Slug is the primary key and user-visible identifier.
type Change struct {
	// Identity
	Slug        string `yaml:"slug" json:"slug"`                                   // Primary key, directory name
	Description string `yaml:"description,omitempty" json:"description,omitempty"` // User intent description

	// Lifecycle
	Status          ChangeStatus  `yaml:"status" json:"status"`                                         // active|done|cancelled
	CurrentState    WorkflowState `yaml:"current_state" json:"current_state"`                           // S0_INTAKE / S1_PLAN / S2_EXECUTE / S3_REVIEW / S4_VERIFY / DONE
	IntakeSubStep   IntakeSubStep `yaml:"intake_substep,omitempty" json:"intake_substep,omitempty"`     // Progress within S0_INTAKE
	PlanSubStep     PlanSubStep   `yaml:"plan_substep,omitempty" json:"plan_substep,omitempty"`         // Planning progress within S1_PLAN
	NeedsDiscovery  bool          `yaml:"needs_discovery,omitempty" json:"needs_discovery,omitempty"`   // Whether discovery path is included in planning
	ComplexityLevel string        `yaml:"complexity_level,omitempty" json:"complexity_level,omitempty"` // trivial/simple/complex/critical
	BaseRef         string        `yaml:"base_ref,omitempty" json:"base_ref,omitempty"`                 // Git ref at change creation (default "HEAD")
	CreatedAt       time.Time     `yaml:"created_at,omitempty" json:"created_at,omitempty"`             // Creation timestamp

	// Direct model fields
	GuardrailDomain string `yaml:"guardrail_domain,omitempty" json:"guardrail_domain,omitempty"` // Authoritative guardrail domain

	// Profile
	QualityMode             QualityMode     `yaml:"quality_mode,omitempty" json:"quality_mode,omitempty"`
	WorkflowProfile         WorkflowProfile `yaml:"workflow_profile,omitempty" json:"workflow_profile,omitempty"`
	WorkflowPreset          WorkflowPreset  `yaml:"workflow_preset,omitempty" json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset WorkflowPreset  `yaml:"suggested_workflow_preset,omitempty" json:"suggested_workflow_preset,omitempty"`

	// Worktree
	WorktreePath   string `yaml:"worktree_path,omitempty" json:"worktree_path,omitempty"`
	WorktreeBranch string `yaml:"worktree_branch,omitempty" json:"worktree_branch,omitempty"`

	// Governance
	ArtifactSchema                     ArtifactSchemaName        `yaml:"artifact_schema,omitempty" json:"artifact_schema,omitempty"`
	CustomArtifacts                    []ArtifactDefinition      `yaml:"custom_artifacts,omitempty" json:"custom_artifacts,omitempty"`
	ContextDependencies                ContextDependencies       `yaml:"context_dependencies,omitempty" json:"context_dependencies,omitempty"` // Execution-context metadata: drives prior-context selection and next input assembly, not consumed by progression/governance/gate logic.
	ProjectContext                     ProjectContext            `yaml:"project_context,omitempty" json:"project_context,omitempty"`
	CallerDisabledCtrls                []ControlID               `yaml:"caller_disabled_controls,omitempty" json:"caller_disabled_controls,omitempty"`
	CallerControlModes                 map[ControlID]ControlMode `yaml:"caller_control_modes,omitempty" json:"caller_control_modes,omitempty"`
	CallerIndependentReviewBlastRadius SignalLevel               `yaml:"caller_independent_review_blast_radius,omitempty" json:"caller_independent_review_blast_radius,omitempty"`
	CallerWorktreeBlastRadius          SignalLevel               `yaml:"caller_worktree_blast_radius,omitempty" json:"caller_worktree_blast_radius,omitempty"`
	PlanAuditIterations                int                       `yaml:"plan_audit_iterations,omitempty" json:"plan_audit_iterations,omitempty"`

	// Execution
	ActiveCheckpoint *ActiveCheckpoint `yaml:"active_checkpoint,omitempty" json:"active_checkpoint,omitempty"`

	// Runtime fields (formerly in a separate runtime-state.yaml sidecar, now
	// unified into change.yaml as part of the single-authority model).
	Artifacts                 map[string]ArtifactState `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	LastAutoPassedStates      []AutoPassedState        `yaml:"last_auto_passed_states,omitempty" json:"last_auto_passed_states,omitempty"`
	EvidenceRefs              map[string]string        `yaml:"evidence_refs,omitempty" json:"evidence_refs,omitempty"`
	ReviewIntentDriftFailures int                      `yaml:"review_intent_drift_failures,omitempty" json:"review_intent_drift_failures,omitempty"`
	InterruptedExecutionAt    time.Time                `yaml:"interrupted_execution_at,omitempty" json:"interrupted_execution_at,omitempty"`
}

// Phase returns the user-facing phase for this change.
func (c Change) Phase() UserPhase {
	return PhaseFor(c.CurrentState)
}

// NewChange creates a new Change with the given slug, initialized at S0_INTAKE.
func NewChange(slug string) Change {
	return Change{
		Slug:           slug,
		Status:         ChangeStatusActive,
		CurrentState:   StateS0Intake,
		IntakeSubStep:  IntakeEntrySubStep(),
		PlanSubStep:    PlanSubStepNone,
		ArtifactSchema: ArtifactSchemaExpanded,
		BaseRef:        "HEAD",
		CreatedAt:      time.Now().UTC(),
		Artifacts:      map[string]ArtifactState{},
		EvidenceRefs:   map[string]string{},
	}
}

func (c *Change) normalizeCollections() {
	if c.EvidenceRefs == nil {
		c.EvidenceRefs = map[string]string{}
	}
	if c.Artifacts == nil {
		c.Artifacts = map[string]ArtifactState{}
	}
}

func (c *Change) Normalize() {
	c.normalizeCollections()
	// Enforce substep consistency: clear substeps that do not belong to the
	// current state and seed entry defaults for states that require them.
	if c.CurrentState != StateS0Intake {
		c.IntakeSubStep = IntakeSubStepNone
	} else if c.IntakeSubStep == IntakeSubStepNone {
		c.IntakeSubStep = IntakeEntrySubStep()
	}
	if c.CurrentState != StateS1Plan && c.CurrentState != StateDone {
		c.PlanSubStep = PlanSubStepNone
	} else if c.CurrentState == StateS1Plan && c.PlanSubStep == PlanSubStepNone {
		c.PlanSubStep = PlanEntrySubStep(c.NeedsDiscovery)
	}
	c.ContextDependencies.Normalize()
	if !c.InterruptedExecutionAt.IsZero() {
		c.InterruptedExecutionAt = c.InterruptedExecutionAt.Round(0).UTC()
	}
}

func (c Change) Validate() error {
	if c.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if !c.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", c.Status)
	}
	if c.CurrentState != "" && !c.CurrentState.IsValid() {
		return fmt.Errorf("invalid current_state: %q", c.CurrentState)
	}
	if c.IntakeSubStep != "" && !c.IntakeSubStep.IsValid() {
		return fmt.Errorf("invalid intake_substep: %q", c.IntakeSubStep)
	}
	if c.PlanSubStep != "" && !c.PlanSubStep.IsValid() {
		return fmt.Errorf("invalid plan_substep: %q", c.PlanSubStep)
	}
	if c.CurrentState == StateS0Intake && c.IntakeSubStep == IntakeSubStepNone {
		return fmt.Errorf("intake_substep is required when current_state is S0_INTAKE")
	}
	if c.CurrentState != StateS0Intake && c.IntakeSubStep != IntakeSubStepNone {
		return fmt.Errorf("intake_substep must be empty when current_state is %q", c.CurrentState)
	}
	if c.CurrentState == StateS1Plan && c.PlanSubStep == PlanSubStepNone {
		return fmt.Errorf("plan_substep is required when current_state is S1_PLAN")
	}
	if c.CurrentState != StateS1Plan && c.CurrentState != StateDone && c.PlanSubStep != PlanSubStepNone {
		return fmt.Errorf("plan_substep must be empty when current_state is %q", c.CurrentState)
	}
	if c.QualityMode != "" && !c.QualityMode.IsValid() {
		return fmt.Errorf("invalid quality_mode: %q", c.QualityMode)
	}
	if !c.WorkflowProfile.IsValid() {
		return fmt.Errorf("invalid workflow_profile: %q", c.WorkflowProfile)
	}
	if c.WorkflowPreset != "" && !c.WorkflowPreset.IsValid() {
		return fmt.Errorf("invalid workflow_preset: %q", c.WorkflowPreset)
	}
	if c.SuggestedWorkflowPreset != "" && !c.SuggestedWorkflowPreset.IsValid() {
		return fmt.Errorf("invalid suggested_workflow_preset: %q", c.SuggestedWorkflowPreset)
	}
	if err := c.ContextDependencies.Validate(); err != nil {
		return fmt.Errorf("context_dependencies invalid: %w", err)
	}
	for i, id := range c.CallerDisabledCtrls {
		if !id.IsValid() {
			return fmt.Errorf("caller_disabled_controls[%d]: invalid control_id %q", i, id)
		}
	}
	for id, mode := range c.CallerControlModes {
		if !id.IsValid() {
			return fmt.Errorf("caller_control_modes[%q]: invalid control_id", id)
		}
		if !mode.IsValid() {
			return fmt.Errorf("caller_control_modes[%q]: invalid mode %q", id, mode)
		}
	}
	if c.CallerIndependentReviewBlastRadius != "" && !c.CallerIndependentReviewBlastRadius.IsValid() {
		return fmt.Errorf("caller_independent_review_blast_radius: invalid signal level %q", c.CallerIndependentReviewBlastRadius)
	}
	if c.CallerWorktreeBlastRadius != "" && !c.CallerWorktreeBlastRadius.IsValid() {
		return fmt.Errorf("caller_worktree_blast_radius: invalid signal level %q", c.CallerWorktreeBlastRadius)
	}
	for key, artifact := range c.Artifacts {
		if artifact.ID == "" {
			return fmt.Errorf("artifacts[%q] is missing id", key)
		}
		if !artifact.State.IsValid() {
			return fmt.Errorf("artifacts[%q] has invalid state: %q", key, artifact.State)
		}
	}
	if c.ActiveCheckpoint != nil {
		if !c.ActiveCheckpoint.PausedAt.IsZero() {
			c.ActiveCheckpoint.PausedAt = c.ActiveCheckpoint.PausedAt.Round(0).UTC()
		}
		if err := c.ActiveCheckpoint.Validate(); err != nil {
			return fmt.Errorf("active_checkpoint: %w", err)
		}
	}
	// Runtime fields are now part of change.yaml. Validation ensures in-memory
	// Change consistency for all callers.
	for i, autoPassed := range c.LastAutoPassedStates {
		if err := autoPassed.Validate(); err != nil {
			return fmt.Errorf("last_auto_passed_states[%d]: %w", i, err)
		}
	}
	if c.ReviewIntentDriftFailures < 0 {
		return fmt.Errorf("review_intent_drift_failures must be >= 0")
	}
	return nil
}

func (c Change) EffectiveWorkflowProfile() WorkflowProfile {
	return c.WorkflowProfile.Effective()
}

func (c Change) MarshalYAML() (interface{}, error) {
	normalized := c
	normalized.Normalize()
	type alias Change
	return alias(normalized), nil
}

func (c *Change) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type alias Change
	var parsed alias
	if err := unmarshal(&parsed); err != nil {
		return err
	}
	*c = Change(parsed)
	c.Normalize()
	return nil
}

// CheckpointKind defines the typed checkpoint semantics for wave execution pauses.
type CheckpointKind string

const (
	CheckpointHumanVerify CheckpointKind = "human_verify"
	CheckpointDecision    CheckpointKind = "decision"
	CheckpointHumanAction CheckpointKind = "human_action"
)

func (k CheckpointKind) IsValid() bool {
	switch k {
	case CheckpointHumanVerify, CheckpointDecision, CheckpointHumanAction:
		return true
	default:
		return false
	}
}

// ActiveCheckpoint tracks a paused checkpoint contract that requires user
// input before wave execution can resume.
type ActiveCheckpoint struct {
	PausedTaskID     string    `yaml:"paused_task_id" json:"paused_task_id"`
	PausedWaveIndex  int       `yaml:"paused_wave_index,omitempty" json:"paused_wave_index,omitempty"`
	PausedAt         time.Time `yaml:"paused_at,omitempty" json:"paused_at,omitempty"`
	CheckpointType   string    `yaml:"checkpoint_type" json:"checkpoint_type"`
	AllowedResponses []string  `yaml:"allowed_responses,omitempty" json:"allowed_responses,omitempty"`
}

type AutoPassedState struct {
	State  WorkflowState `yaml:"state" json:"state"`
	Reason string        `yaml:"reason" json:"reason"`
}

func (s AutoPassedState) Validate() error {
	if !s.State.IsValid() {
		return fmt.Errorf("invalid auto-passed state: %q", s.State)
	}
	if strings.TrimSpace(s.Reason) == "" {
		return fmt.Errorf("auto-passed reason is required")
	}
	return nil
}

func (cp ActiveCheckpoint) Validate() error {
	if strings.TrimSpace(cp.PausedTaskID) == "" {
		return fmt.Errorf("paused_task_id is required")
	}
	if cp.PausedWaveIndex < 0 {
		return fmt.Errorf("paused_wave_index must be >= 0")
	}
	kind := CheckpointKind(cp.CheckpointType)
	if !kind.IsValid() {
		return fmt.Errorf("invalid checkpoint_type %q; must be one of: human_verify, decision, human_action", cp.CheckpointType)
	}
	if kind == CheckpointDecision && len(cp.AllowedResponses) == 0 {
		return fmt.Errorf("checkpoint_type=decision requires non-empty allowed_responses")
	}
	return nil
}
