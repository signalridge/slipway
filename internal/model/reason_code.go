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

const unknownReasonCode = "unknown_reason_code"

var canonicalReasonDefinitions = map[string]ReasonDefinition{
	"archive_failed": {
		Severity: ReasonSeverityError,
		Message:  "Bulk finalization could not archive a governed change",
	},
	"artifact_not_ready": {
		Severity: ReasonSeverityError,
		Message:  "Required governed artifacts are not ready",
	},
	"artifact_reconcile_failed": {
		Severity: ReasonSeverityError,
		Message:  "Done artifact reconciliation failed",
	},
	"artifact_schema_missing": {
		Severity: ReasonSeverityError,
		Message:  "The governed change is missing a frozen artifact schema",
	},
	"artifact_validation_failed": {
		Severity: ReasonSeverityError,
		Message:  "Done artifact validation failed",
	},
	"archived_lifecycle_event_scan_failed": {
		Severity: ReasonSeverityError,
		Message:  "Unable to list archived changes for lifecycle event health",
	},
	"assurance_structure_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The assurance artifact structure is invalid",
	},
	"assurance_contract_missing": {
		Severity: ReasonSeverityError,
		Message:  "The assurance artifact is missing",
	},
	"assurance_contract_path_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The assurance artifact path is invalid",
	},
	"assurance_contract_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "The assurance artifact is unreadable",
	},
	"assurance_section_placeholder": {
		Severity: ReasonSeverityError,
		Message:  "The assurance artifact still contains a placeholder section",
	},
	"closeout_assurance_attestation_missing": {
		Severity: ReasonSeverityError,
		Message:  "The final-closeout assurance attestation is missing",
	},
	"closeout_chain_order_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Final-closeout must be stamped after every selected S3 peer; goal-verification is an unordered peer, not a serialized post-review step",
	},
	"closeout_reviewer_independence_missing": {
		Severity: ReasonSeverityError,
		Message:  "The final-closeout reviewer-independence attestation is missing",
	},
	"closeout_goal_verification_reuse_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Final-closeout cannot reuse the recorded goal-verification evidence",
	},
	"change_bundle_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Change bundle authority is unreadable",
	},
	"change_is_done": {
		Severity: ReasonSeverityInfo,
		Message:  "The governed change is already done",
	},
	"change_not_active": {
		Severity: ReasonSeverityError,
		Message:  "Bulk finalization skipped a non-active governed change",
	},
	"checkpoint_stale": {
		Severity: ReasonSeverityWarning,
		Message:  "Active checkpoint has exceeded the stale threshold",
	},
	"checkpoint_task_missing_from_wave_plan": {
		Severity: ReasonSeverityError,
		Message:  "Checkpoint task is not present in the current wave plan",
	},
	"checkpoint_wave_index_drift": {
		Severity: ReasonSeverityError,
		Message:  "Checkpoint wave index does not match the current wave plan",
	},
	"codebase_map_freshness_missing": {
		Severity: ReasonSeverityError,
		Message:  "Repo-scoped codebase map is missing",
	},
	"codebase_map_freshness_partial": {
		Severity: ReasonSeverityWarning,
		Message:  "Repo-scoped codebase map is partially populated",
	},
	"codebase_map_freshness_scaffold_only": {
		Severity: ReasonSeverityWarning,
		Message:  "Repo-scoped codebase map is scaffold-only",
	},
	"codebase_map_freshness_stale": {
		Severity: ReasonSeverityWarning,
		Message:  "Repo-scoped codebase map is stale",
	},
	"codebase_map_freshness_unknown": {
		Severity: ReasonSeverityWarning,
		Message:  "Repo-scoped codebase map freshness is unknown",
	},
	"config_parse_failure": {
		Severity: ReasonSeverityError,
		Message:  "Config parsing failed",
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
	"decision_contract_path_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The decision artifact path is invalid",
	},
	"decision_contract_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "The decision artifact is unreadable",
	},
	"decision_section_placeholder": {
		Severity: ReasonSeverityError,
		Message:  "The decision artifact still contains a placeholder section",
	},
	"decision_status_rejected": {
		Severity: ReasonSeverityError,
		Message:  "The decision artifact status is rejected for planning",
	},
	"decision_status_unknown": {
		Severity: ReasonSeverityError,
		Message:  "The decision artifact status is unknown",
	},
	"decision_structure_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The decision artifact structure is invalid",
	},
	"degraded_dispatch_justification_missing": {
		Severity: ReasonSeverityError,
		Message:  "A degraded_sequential dispatch is missing its tool-unavailable justification",
	},
	"dispatch_mode_absent_on_started_parallel_wave": {
		Severity: ReasonSeverityError,
		Message:  "A started parallel wave recorded no valid dispatch_mode evidence; the engine will not infer parallel dispatch, so record dispatch_mode:wave=<n>:parallel_subagents (or degraded_sequential) and re-run",
	},
	"execution_interrupted": {
		Severity: ReasonSeverityWarning,
		Message:  "Governed execution was interrupted",
	},
	"execution_summary_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Execution summary authority is unreadable",
	},
	"execution_verdict_fail": {
		Severity: ReasonSeverityError,
		Message:  "Execution verdict failed",
	},
	"executor_agent_missing": {
		Severity: ReasonSeverityError,
		Message:  "A parallel_subagents wave is missing the executor_agent handle for a planned task; record executor_agent:wave=<n>:task=<id>:<handle> for every task in the wave and re-run",
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
	"intent_drift": {
		Severity: ReasonSeverityError,
		Message:  "Implementation intent drift was detected",
	},
	"invalid_blocker": {
		Severity: ReasonSeverityError,
		Message:  "A blocker token is invalid",
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
	"lifecycle_event_log_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Lifecycle event log is unreadable",
	},
	"lifecycle_event_scan_failed": {
		Severity: ReasonSeverityError,
		Message:  "Unable to list active changes for lifecycle event health",
	},
	"lifecycle_event_scan_skipped": {
		Severity: ReasonSeverityWarning,
		Message:  "Skipped lifecycle event health because change authority is unreadable",
	},
	"lifecycle_event_write_failed": {
		Severity: ReasonSeverityError,
		Message:  "Bulk finalization could not record a lifecycle event",
	},
	"legacy_runtime_handoff": {
		Severity: ReasonSeverityWarning,
		Message:  "A legacy repo-level runtime handoff file exists and requires manual migration",
	},
	"legacy_runtime_changes_dir": {
		Severity: ReasonSeverityWarning,
		Message:  "A retired repo-level runtime changes directory exists with content and requires manual inspection",
	},
	"legacy_runtime_changes_dir_empty": {
		Severity: ReasonSeverityWarning,
		Message:  "An empty retired repo-level runtime changes directory exists and can be cleaned by repair",
	},
	"list_changes_failed": {
		Severity: ReasonSeverityError,
		Message:  "Bulk finalization could not list active changes",
	},
	"load_change_failed": {
		Severity: ReasonSeverityError,
		Message:  "Bulk finalization could not load a governed change",
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
	"missing_run_summary": {
		Severity: ReasonSeverityError,
		Message:  "The execution run summary is missing",
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
	"multiple_active_changes": {
		Severity: ReasonSeverityError,
		Message:  "Multiple active changes are present",
	},
	"non_pass_wave": {
		Severity: ReasonSeverityError,
		Message:  "A governed execution wave did not pass",
	},
	"new_change_required": {
		Severity: ReasonSeverityError,
		Message:  "The requested work no longer belongs to the current governed change",
	},
	"not_done_ready": {
		Severity: ReasonSeverityError,
		Message:  "The governed change is not ready for finalization",
	},
	"orphaned_bundle_ownership_unknown": {
		Severity: ReasonSeverityError,
		Message:  "A governed bundle lost its change.yaml authority and its live-worktree ownership could not be verified",
	},
	"orphaned_bundle_unmanaged_worktree": {
		Severity: ReasonSeverityError,
		Message:  "A governed bundle lost its change.yaml authority but a live worktree Slipway does not manage holds work for its slug",
	},
	"orphaned_change_bundle": {
		Severity: ReasonSeverityError,
		Message:  "A governed bundle directory is missing its change.yaml authority",
	},
	"orphan_bundle_directory": {
		Severity: ReasonSeverityError,
		Message:  "Bundle directory exists without change.yaml",
	},
	"orphan_task_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Orphan task evidence exists outside the current wave plan",
	},
	"parallel_wave_changed_file_overlap": {
		Severity: ReasonSeverityError,
		Message:  "Two tasks in the same parallel wave recorded the same changed file; same-worktree parallel executors can clobber each other",
	},
	"stale_runtime_binding": {
		Severity: ReasonSeverityError,
		Message:  "A per-change runtime binding remains after its governed bundle was removed",
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
	"plan_dimension_key_links_missing_target_files": {
		Severity: ReasonSeverityError,
		Message:  "A task is missing target files",
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
		Message:  "requirements.md is structurally invalid or not substantive",
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
	"review_alignment_required": {
		Severity: ReasonSeverityError,
		Message:  "A stale authority requires plan/code/evidence alignment review before finalization",
	},
	"review_required": {
		Severity: ReasonSeverityError,
		Message:  "Review convergence is required before finalization",
	},
	"research_structure_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The research artifact structure is invalid",
	},
	"research_section_placeholder": {
		Severity: ReasonSeverityError,
		Message:  "The research artifact still contains a placeholder section",
	},
	"run_slipway_done_to_finalize": {
		Severity: ReasonSeverityWarning,
		Message:  "Run `slipway done` to finalize the governed change",
	},
	"run_slipway_run_to_advance": {
		Severity: ReasonSeverityWarning,
		Message:  "Run `slipway run` to advance the workflow",
	},
	"review_layer_missing": {
		Severity: ReasonSeverityError,
		Message:  "Required review layer evidence is missing",
	},
	"context_origin_handle_invalid": {
		Severity: ReasonSeverityError,
		Message:  "A governed stage recorded a missing or invalid context-origin handle",
	},
	"cross_stage_context_not_distinct": {
		Severity: ReasonSeverityError,
		Message:  "Two governed stages recorded the same context-origin handle and are not distinct",
	},
	"plan_audit_origin_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Plan audit recorded the same author and auditor context-origin handle (self-audit)",
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
	"session_isolation_warning": {
		Severity: ReasonSeverityWarning,
		Message:  "Session isolation warning detected in task evidence",
	},
	"sensitive_evidence_missing": {
		Severity: ReasonSeverityError,
		Message:  "Sensitive changed file is missing owning evidence",
	},
	"ship_gate_blocked": {
		Severity: ReasonSeverityError,
		Message:  "The ship gate blocked finalization",
	},
	"skill_prompt_surface_missing": {
		Severity: ReasonSeverityError,
		Message:  "Governance skill points to a missing host skill surface",
	},
	"skill_prompt_surface_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Governance skill points to an unreadable host skill surface",
	},
	"skill_registry_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Governance skill registry is invalid",
	},
	"stale_checkpoint_state": {
		Severity: ReasonSeverityWarning,
		Message:  "Active checkpoint exists outside S2_IMPLEMENT",
	},
	"stale_execution_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Execution evidence is stale; rerun wave-orchestration for affected tasks",
	},
	"stale_planning_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Planning artifacts changed after execution evidence; rerun affected planning gates before refreshing execution evidence",
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
	"task": {
		Severity: ReasonSeverityError,
		Message:  "A task-scoped execution blocker is present",
	},
	"task_changed_file_scope_escape": {
		Severity: ReasonSeverityError,
		Message:  "A task recorded a changed file outside its planned target_files; fix target_files and re-record evidence, then let review verify plan/code alignment",
	},
	"task_blocker": {
		Severity: ReasonSeverityError,
		Message:  "A task-scoped wave blocker is present",
	},
	"task_blockers": {
		Severity: ReasonSeverityError,
		Message:  "A governed task reported blockers",
	},
	"task_blockers_invalid_key": {
		Severity: ReasonSeverityError,
		Message:  "A governed task blocker key is invalid",
	},
	"task_evidence_invalid": {
		Severity: ReasonSeverityError,
		Message:  "Task evidence is invalid",
	},
	"task_evidence_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Task evidence is unreadable",
	},
	"tasks_plan_changed_since_task_evidence": {
		Severity: ReasonSeverityError,
		Message:  "The tasks plan changed after this task's evidence was captured; rerun wave-orchestration for the affected task",
	},
	unknownReasonCode: {
		Severity: ReasonSeverityError,
		Message:  "An unrecognized reason code was produced",
	},
	"verification_evidence_missing": {
		Severity: ReasonSeverityError,
		Message:  "Required verification evidence is missing; rerun goal-verification, then rerun final-closeout before done",
	},
	"wave_orchestration_run_summary_version_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The wave run-summary version is invalid; rerun wave-orchestration to produce a versioned run summary",
	},
	"wave_orchestration_stale_task_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Task evidence was captured after the wave verification record; rerun wave-orchestration to refresh the wave record",
	},
	"wave_execution_blocked": {
		Severity: ReasonSeverityError,
		Message:  "Wave execution is blocked",
	},
	"wave_execution_unavailable": {
		Severity: ReasonSeverityError,
		Message:  "Wave execution is unavailable",
	},
	"wave_plan_drift": {
		Severity: ReasonSeverityError,
		Message:  "Derived wave plan is stale against tasks.md",
	},
	"wave_plan_load_failed": {
		Severity: ReasonSeverityError,
		Message:  "Wave plan authority could not be loaded",
	},
	"wave_plan_missing": {
		Severity: ReasonSeverityError,
		Message:  "The derived wave plan is missing; rebuild wave-plan.yaml from tasks.md before wave execution",
	},
	"wave_plan_repair_blocked": {
		Severity: ReasonSeverityError,
		Message:  "Wave plan repair is blocked",
	},
	"wave_plan_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Wave plan authority is unreadable",
	},
	"wave_run_missing": {
		Severity: ReasonSeverityError,
		Message:  "A planned wave has no recorded run evidence",
	},
	"wave_run_version_mismatch": {
		Severity: ReasonSeverityError,
		Message:  "Wave run evidence does not match the requested run version",
	},
	"wave_runs_incomplete": {
		Severity: ReasonSeverityError,
		Message:  "Wave run evidence is incomplete for the current wave plan",
	},
	"wave_runs_invalid_count": {
		Severity: ReasonSeverityError,
		Message:  "Wave run evidence count is invalid",
	},
	"wave_runs_load_failed": {
		Severity: ReasonSeverityError,
		Message:  "Wave run evidence could not be loaded",
	},
	"wave_runs_missing": {
		Severity: ReasonSeverityError,
		Message:  "Wave runs are missing for the latest execution summary",
	},
	"wave_runs_unreadable": {
		Severity: ReasonSeverityError,
		Message:  "Wave run evidence is unreadable",
	},
	"wave_task_linkage_mismatch": {
		Severity: ReasonSeverityError,
		Message:  "Wave run task linkage does not match wave-plan.yaml",
	},
	"workspace_scope_config_missing": {
		Severity: ReasonSeverityError,
		Message:  "Bound worktree scope config is missing",
	},
	"workspace_scope_marker_missing": {
		Severity: ReasonSeverityError,
		Message:  "Bound worktree scope marker is missing",
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

func IsCanonicalReasonCode(code string) bool {
	_, ok := canonicalReasonDefinitions[normalizeReasonCode(strings.TrimSpace(code))]
	return ok
}

func NewReasonCode(code, detail string) ReasonCode {
	rawCode := strings.TrimSpace(code)
	code = normalizeReasonCode(rawCode)
	detail = strings.TrimSpace(detail)
	definition, ok := canonicalReasonDefinitions[code]
	if !ok {
		return newUnknownReasonCode(rawCode, detail)
	}
	reason := ReasonCode{
		Code:     code,
		Severity: definition.Severity,
		Message:  definition.Message,
		Detail:   detail,
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
	rawCode := strings.TrimSpace(r.Code)
	r.Code = normalizeReasonCode(rawCode)
	r.Detail = strings.TrimSpace(r.Detail)
	r.Message = strings.TrimSpace(r.Message)
	definition, ok := canonicalReasonDefinitions[r.Code]
	if !ok {
		unknown := newUnknownReasonCode(rawCode, r.Detail)
		*r = unknown
		return
	}
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

func newUnknownReasonCode(code, detail string) ReasonCode {
	code = strings.TrimSpace(code)
	detail = strings.TrimSpace(detail)
	if code == "" {
		code = "empty"
	}
	unknownDetail := code
	if detail != "" {
		unknownDetail += ": " + detail
	}
	definition := canonicalReasonDefinitions[unknownReasonCode]
	reason := ReasonCode{
		Code:     unknownReasonCode,
		Severity: definition.Severity,
		Message:  definition.Message,
		Detail:   unknownDetail,
	}
	if reason.Detail != "" {
		reason.Message += ": " + reason.Detail
	}
	return reason
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
