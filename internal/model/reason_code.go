package model

import (
	"fmt"
	"slices"
	"strings"
)

type ReasonSeverity string

const (
	ReasonSeverityInfo    ReasonSeverity = "info"
	ReasonSeverityWarning ReasonSeverity = "warning"
	ReasonSeverityError   ReasonSeverity = "error"
)

func (s ReasonSeverity) IsValid() bool {
	switch s {
	case ReasonSeverityInfo, ReasonSeverityWarning, ReasonSeverityError:
		return true
	default:
		return false
	}
}

type ReasonCode struct {
	Code     string         `yaml:"code" json:"code"`
	Severity ReasonSeverity `yaml:"severity" json:"severity"`
	Message  string         `yaml:"message" json:"message"`
	Detail   string         `yaml:"detail,omitempty" json:"detail,omitempty"`
}

type ReasonDefinition struct {
	Severity ReasonSeverity `json:"severity"`
	Message  string         `json:"message"`
}

var canonicalReasonDefinitions = map[string]ReasonDefinition{
	"artifact_not_ready": {
		Severity: ReasonSeverityError,
		Message:  "Required governed artifacts are not ready",
	},
	"artifact_schema_missing": {
		Severity: ReasonSeverityError,
		Message:  "The governed change is missing a frozen artifact schema",
	},
	"assurance_structure_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The assurance artifact structure is invalid",
	},
	"closeout_assurance_attestation_missing": {
		Severity: ReasonSeverityError,
		Message:  "The final-closeout assurance attestation is missing",
	},
	"closeout_goal_verification_reuse_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Final-closeout cannot reuse the recorded goal-verification evidence",
	},
	"dedicated_worktree_branch_mismatch": {
		Severity: ReasonSeverityError,
		Message:  "The bound worktree branch does not match the recorded change branch",
	},
	"dedicated_worktree_metadata_required": {
		Severity: ReasonSeverityError,
		Message:  "Dedicated worktree metadata is missing",
	},
	"dedicated_worktree_path_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The recorded dedicated worktree path is invalid",
	},
	"dedicated_worktree_required": {
		Severity: ReasonSeverityError,
		Message:  "A dedicated worktree is required for this governed change",
	},
	"execution_interrupted": {
		Severity: ReasonSeverityWarning,
		Message:  "Governed execution was interrupted",
	},
	"governance_action_required": {
		Severity: ReasonSeverityError,
		Message:  "A required governance control must be satisfied before continuing",
	},
	"governed_bundle_path_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The governed bundle path is invalid",
	},
	"high_risk_check_failed": {
		Severity: ReasonSeverityError,
		Message:  "A required high-risk safety check failed",
	},
	"high_risk_check_missing": {
		Severity: ReasonSeverityError,
		Message:  "A required high-risk safety check is missing",
	},
	"incomplete_execution_task": {
		Severity: ReasonSeverityError,
		Message:  "A planned execution task has no recorded passing evidence",
	},
	"invalid_pivot_kind": {
		Severity: ReasonSeverityError,
		Message:  "The requested pivot kind is invalid",
	},
	"intake_clarification_incomplete": {
		Severity: ReasonSeverityError,
		Message:  "Intake clarification is incomplete",
	},
	"intake_confirmation_incomplete": {
		Severity: ReasonSeverityError,
		Message:  "Intake confirmation is incomplete",
	},
	"intake_substep_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The intake substep is invalid",
	},
	"manifest_r0_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The governed change manifest failed R0 validation",
	},
	"missing_discovery_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Required discovery evidence is missing",
	},
	"missing_required_artifact": {
		Severity: ReasonSeverityError,
		Message:  "A required governed artifact is missing",
	},
	"missing_task_evidence_for_run_summary": {
		Severity: ReasonSeverityError,
		Message:  "Task evidence is missing for the recorded run summary; rerun wave-orchestration to capture task evidence",
	},
	"missing_worktree_branch": {
		Severity: ReasonSeverityError,
		Message:  "The change is missing a bound worktree branch",
	},
	"missing_worktree_path": {
		Severity: ReasonSeverityError,
		Message:  "The change is missing a bound worktree path",
	},
	"no_skill_required": {
		Severity: ReasonSeverityInfo,
		Message:  "No skill is required for the current workflow state",
	},
	"non_pass_task": {
		Severity: ReasonSeverityError,
		Message:  "A governed task did not pass",
	},
	"pivot_not_approved": {
		Severity: ReasonSeverityError,
		Message:  "The requested pivot is not approved",
	},
	"plan_audit_budget_exhausted": {
		Severity: ReasonSeverityError,
		Message:  "Plan audit exhausted its checker iteration budget",
	},
	"plan_audit_evidence_missing": {
		Severity: ReasonSeverityError,
		Message:  "Plan audit evidence is missing",
	},
	"plan_audit_failed": {
		Severity: ReasonSeverityError,
		Message:  "Plan audit did not pass",
	},
	"plan_audit_iteration": {
		Severity: ReasonSeverityError,
		Message:  "Plan audit checker iteration is in progress",
	},
	"plan_audit_stalled": {
		Severity: ReasonSeverityError,
		Message:  "Plan audit feedback did not improve",
	},
	"plan_checker_feedback_required": {
		Severity: ReasonSeverityError,
		Message:  "Plan checker feedback must be incorporated before continuing",
	},
	"plan_checker_loop_terminated": {
		Severity: ReasonSeverityError,
		Message:  "Plan checker loop terminated before plan audit could pass",
	},
	"plan_dimension_completeness_missing_objective": {
		Severity: ReasonSeverityError,
		Message:  "A task is missing a concrete objective",
	},
	"plan_dimension_execution_missing_wave": {
		Severity: ReasonSeverityError,
		Message:  "A task is missing an execution wave",
	},
	"plan_dimension_key_links_missing_target_files": {
		Severity: ReasonSeverityError,
		Message:  "A code task is missing target files",
	},
	"plan_dimension_scope_invalid_target": {
		Severity: ReasonSeverityError,
		Message:  "A task target file entry is invalid",
	},
	"plan_dimension_scope_out_of_bounds_target": {
		Severity: ReasonSeverityError,
		Message:  "A task target file is outside the repository",
	},
	"plan_dimension_dependency_self_reference": {
		Severity: ReasonSeverityError,
		Message:  "A task dependency references itself",
	},
	"plan_dimension_dependency_unknown": {
		Severity: ReasonSeverityError,
		Message:  "A task dependency references an unknown task",
	},
	"plan_dimension_dependency_cycle_detected": {
		Severity: ReasonSeverityError,
		Message:  "Task dependencies contain a cycle",
	},
	"plan_dimension_execution_invalid_wave_plan": {
		Severity: ReasonSeverityError,
		Message:  "The task wave plan is invalid",
	},
	"plan_dimension_coverage_spec_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Requirement coverage input is unreadable",
	},
	"plan_dimension_coverage_requirements_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Requirement coverage input is invalid",
	},
	"plan_dimension_coverage_requirement_id_missing": {
		Severity: ReasonSeverityError,
		Message:  "A requirement is missing an explicit ID",
	},
	"plan_dimension_coverage_unknown_requirement": {
		Severity: ReasonSeverityError,
		Message:  "A task covers an unknown requirement",
	},
	"plan_dimension_coverage_missing_requirement": {
		Severity: ReasonSeverityError,
		Message:  "A requirement has no task coverage",
	},
	"preset_confirmation_required": {
		Severity: ReasonSeverityError,
		Message:  "Workflow preset confirmation is required before continuing",
	},
	"required_artifact_schema_missing": {
		Severity: ReasonSeverityError,
		Message:  "A required governed artifact is missing from the artifact schema",
	},
	"required_artifact_dependency_missing": {
		Severity: ReasonSeverityError,
		Message:  "A required governed artifact dependency is missing",
	},
	"required_artifact_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "A required governed artifact is unreadable",
	},
	"required_skill_blockers_present": {
		Severity: ReasonSeverityError,
		Message:  "A required governance skill still reports blockers",
	},
	"required_skill_missing": {
		Severity: ReasonSeverityError,
		Message:  "Required governance skill evidence is missing",
	},
	"required_skill_not_passed": {
		Severity: ReasonSeverityError,
		Message:  "A required governance skill did not pass",
	},
	"required_skill_not_ready": {
		Severity: ReasonSeverityError,
		Message:  "A required governance skill is present but not ready",
	},
	"required_skill_stale": {
		Severity: ReasonSeverityError,
		Message:  "A required governance skill certified inputs that changed; rerun the skill to re-certify the named artifact",
	},
	"research_structure_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The research artifact structure is invalid",
	},
	"run_slipway_done_to_finalize": {
		Severity: ReasonSeverityWarning,
		Message:  "Run `slipway done` to finalize the governed change",
	},
	"run_slipway_run_to_advance": {
		Severity: ReasonSeverityWarning,
		Message:  "Run `slipway run` to advance the workflow",
	},
	"pivot_state_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Pivot is not allowed from the current workflow state",
	},
	"rescope_state_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Rescope pivots are only allowed from S2_EXECUTE",
	},
	"review_layer_missing": {
		Severity: ReasonSeverityError,
		Message:  "Required review layer evidence is missing",
	},
	"review_layer_failed": {
		Severity: ReasonSeverityError,
		Message:  "Required review layer evidence did not pass",
	},
	"scope_contract_changed_files_missing": {
		Severity: ReasonSeverityError,
		Message:  "Scope Contract changed-files evidence is missing",
	},
	"scope_contract_drift": {
		Severity: ReasonSeverityError,
		Message:  "Changed files are outside the planned Scope Contract",
	},
	"scope_contract_evaluation_failed": {
		Severity: ReasonSeverityError,
		Message:  "Scope Contract evaluation failed",
	},
	"scope_contract_missing": {
		Severity: ReasonSeverityError,
		Message:  "Scope Contract is missing required target files",
	},
	"ship_gate_blocked": {
		Severity: ReasonSeverityError,
		Message:  "The ship gate blocked finalization",
	},
	"stale_execution_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Execution evidence is stale; rerun wave-orchestration for affected tasks",
	},
	"stale_planning_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Planning artifacts changed after execution evidence; rerun affected planning gates before refreshing execution evidence",
	},
	"stale_evidence_recovery_available": {
		Severity: ReasonSeverityWarning,
		Message:  "Stale evidence can be recovered by reopening the earliest affected authority",
	},
	"tasks_checklist_invalid_format": {
		Severity: ReasonSeverityError,
		Message:  "The governed tasks checklist format is invalid",
	},
	"tasks_checklist_missing": {
		Severity: ReasonSeverityError,
		Message:  "The governed tasks checklist is missing",
	},
	"tasks_checklist_path_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The governed tasks checklist path is invalid",
	},
	"tasks_checklist_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "The governed tasks checklist is unreadable",
	},
	"tasks_checklist_empty": {
		Severity: ReasonSeverityError,
		Message:  "The governed tasks checklist has no tasks",
	},
	"tasks_checklist_task_id_missing": {
		Severity: ReasonSeverityError,
		Message:  "A governed task checklist entry is missing a task ID",
	},
	"tasks_checklist_duplicate_task_id": {
		Severity: ReasonSeverityError,
		Message:  "The governed tasks checklist contains a duplicate task ID",
	},
	"tasks_plan_changed_since_task_evidence": {
		Severity: ReasonSeverityError,
		Message:  "The tasks plan changed after this task's evidence was captured; rerun wave-orchestration for the affected task",
	},
	"verification_evidence_missing": {
		Severity: ReasonSeverityError,
		Message:  "Required verification evidence is missing; in S4_VERIFY recovery, rerun goal-verification, then rerun final-closeout",
	},
	"wave_orchestration_run_summary_version_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The wave run-summary version is invalid; rerun wave-orchestration to produce a versioned run summary",
	},
	"wave_orchestration_stale_task_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Task evidence was captured after the wave verification record; rerun wave-orchestration to refresh the wave record",
	},
	"wave_plan_missing": {
		Severity: ReasonSeverityError,
		Message:  "The wave plan is missing; materialize the wave plan from tasks.md before wave execution",
	},
	"worktree_validation_error": {
		Severity: ReasonSeverityError,
		Message:  "Worktree validation failed",
	},
	"worktree_metadata_persist_failed": {
		Severity: ReasonSeverityError,
		Message:  "Worktree metadata could not be persisted",
	},
}

func NewReasonCode(code, detail string) ReasonCode {
	code = normalizeReasonCode(code)
	detail = strings.TrimSpace(detail)
	definition, ok := canonicalReasonDefinitions[code]
	reason := ReasonCode{
		Code:     code,
		Severity: ReasonSeverityError,
		Message:  humanizeReasonCode(code),
		Detail:   detail,
	}
	if ok {
		reason.Severity = definition.Severity
		reason.Message = definition.Message
	}
	if detail != "" {
		reason.Message = reason.Message + ": " + detail
	}
	reason.Normalize()
	return reason
}

func ReasonCodeFromSpec(spec string) ReasonCode {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return NewReasonCode("invalid_blocker", "")
	}
	if code, detail, ok := strings.Cut(trimmed, ":"); ok {
		return NewReasonCode(code, detail)
	}
	if code, detail, ok := strings.Cut(trimmed, "="); ok {
		return NewReasonCode(code, detail)
	}
	return NewReasonCode(trimmed, "")
}

func ReasonCodesFromSpecs(specs []string) []ReasonCode {
	if len(specs) == 0 {
		return nil
	}
	reasons := make([]ReasonCode, 0, len(specs))
	for _, spec := range specs {
		if strings.TrimSpace(spec) == "" {
			continue
		}
		reasons = append(reasons, ReasonCodeFromSpec(spec))
	}
	return NormalizeReasonCodes(reasons)
}

func (r *ReasonCode) Normalize() {
	if r == nil {
		return
	}
	r.Code = normalizeReasonCode(r.Code)
	r.Detail = strings.TrimSpace(r.Detail)
	r.Message = strings.TrimSpace(r.Message)
	if definition, ok := canonicalReasonDefinitions[r.Code]; ok {
		if !r.Severity.IsValid() {
			r.Severity = definition.Severity
		}
		if r.Message == "" {
			r.Message = definition.Message
			if r.Detail != "" {
				r.Message = r.Message + ": " + r.Detail
			}
		}
	}
	if !r.Severity.IsValid() {
		r.Severity = ReasonSeverityError
	}
	if r.Message == "" {
		r.Message = humanizeReasonCode(r.Code)
		if r.Detail != "" {
			r.Message = r.Message + ": " + r.Detail
		}
	}
}

func (r ReasonCode) Validate() error {
	if normalizeReasonCode(r.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if !r.Severity.IsValid() {
		return fmt.Errorf("invalid severity: %q", r.Severity)
	}
	if strings.TrimSpace(r.Message) == "" {
		return fmt.Errorf("message is required")
	}
	return nil
}

func (r ReasonCode) Key() string {
	code := normalizeReasonCode(r.Code)
	if code == "" {
		return "invalid"
	}
	detail := strings.TrimSpace(r.Detail)
	if detail == "" {
		return code
	}
	return code + "\x00" + detail
}

func NormalizeReasonCodes(reasons []ReasonCode) []ReasonCode {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]ReasonCode, 0, len(reasons))
	seen := map[string]struct{}{}
	for _, reason := range reasons {
		reason.Normalize()
		key := reason.Key()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, reason)
	}
	slices.SortFunc(out, func(a, b ReasonCode) int {
		return strings.Compare(a.Key(), b.Key())
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func ReasonMessages(reasons []ReasonCode) []string {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]string, 0, len(reasons))
	for _, reason := range NormalizeReasonCodes(reasons) {
		if strings.TrimSpace(reason.Message) == "" {
			continue
		}
		out = append(out, reason.Message)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func ReasonSpecs(reasons []ReasonCode) []string {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]string, 0, len(reasons))
	for _, reason := range NormalizeReasonCodes(reasons) {
		spec := reason.Code
		if strings.TrimSpace(reason.Detail) != "" {
			spec += ":" + reason.Detail
		}
		if strings.TrimSpace(spec) == "" {
			continue
		}
		out = append(out, spec)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeReasonCode(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func humanizeReasonCode(code string) string {
	code = normalizeReasonCode(code)
	if code == "" {
		return "Unspecified workflow blocker"
	}
	parts := strings.Fields(strings.ReplaceAll(code, "_", " "))
	if len(parts) == 0 {
		return "Workflow blocker"
	}
	parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
	return strings.Join(parts, " ")
}
