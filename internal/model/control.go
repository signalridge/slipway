package model

import "fmt"

// ControlID identifies a built-in or override governance control.
type ControlID string

const (
	ControlClarification     ControlID = "clarification"
	ControlResearch          ControlID = "research"
	ControlDomainReview      ControlID = "domain-review"
	ControlIndependentReview ControlID = "independent-review"
	ControlSecurityReview    ControlID = "security-review"
	ControlWorktreeIsolation ControlID = "worktree-isolation"
	ControlRollbackRequired  ControlID = "rollback-required"
)

func (c ControlID) String() string { return string(c) }

func (c ControlID) IsValid() bool {
	switch c {
	case ControlClarification, ControlResearch, ControlDomainReview,
		ControlIndependentReview, ControlSecurityReview, ControlWorktreeIsolation, ControlRollbackRequired:
		return true
	default:
		return false
	}
}

// ControlMode defines whether a control blocks progression or is advisory.
type ControlMode string

const (
	ControlModeBlocking ControlMode = "blocking"
	ControlModeAdvisory ControlMode = "advisory"
)

func (m ControlMode) String() string { return string(m) }

func (m ControlMode) IsValid() bool {
	switch m {
	case ControlModeBlocking, ControlModeAdvisory:
		return true
	default:
		return false
	}
}

// ControlScope defines the lifecycle phase where a control applies.
type ControlScope string

const (
	ControlScopeDiscovery ControlScope = "discovery"
	ControlScopeReview    ControlScope = "review"
	ControlScopeExecution ControlScope = "execution"
	ControlScopeRelease   ControlScope = "release"
)

func (s ControlScope) String() string { return string(s) }

func (s ControlScope) IsValid() bool {
	switch s {
	case ControlScopeDiscovery, ControlScopeReview, ControlScopeExecution, ControlScopeRelease:
		return true
	default:
		return false
	}
}

// ControlActivation records the activation (or deactivation) of a governance control.
type ControlActivation struct {
	ControlID    ControlID    `yaml:"control_id" json:"control_id"`
	Mode         ControlMode  `yaml:"mode" json:"mode"`
	Scope        ControlScope `yaml:"scope" json:"scope"`
	Active       bool         `yaml:"active" json:"active"`
	TriggeredBy  []string     `yaml:"triggered_by,omitempty" json:"triggered_by,omitempty"`
	PolicySource string       `yaml:"policy_source" json:"policy_source"`
}

// Validate checks control activation invariants.
func (a ControlActivation) Validate() error {
	if !a.ControlID.IsValid() {
		return fmt.Errorf("invalid control_id: %q", a.ControlID)
	}
	if !a.Mode.IsValid() {
		return fmt.Errorf("invalid control mode: %q", a.Mode)
	}
	if !a.Scope.IsValid() {
		return fmt.Errorf("invalid control scope: %q", a.Scope)
	}
	if a.PolicySource == "" {
		return fmt.Errorf("policy_source is required")
	}
	if len(a.TriggeredBy) == 0 {
		return fmt.Errorf("triggered_by is required")
	}
	for i, trigger := range a.TriggeredBy {
		if trigger == "" {
			return fmt.Errorf("triggered_by[%d] is required", i)
		}
	}
	return nil
}

// BuiltinPolicySource is the source identifier for built-in controls.
const BuiltinPolicySource = "builtin"

// OverridePolicySource is the source identifier for project-level overrides.
const OverridePolicySource = "override"
