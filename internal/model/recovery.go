package model

import (
	"sort"
	"strings"
)

// ParsedBlocker is the structured decomposition of a blocker. It is the single
// decomposition point for prefix tokens such as
// "required_skill_stale:<skill>:<artifact>" or
// "tasks_plan_changed_since_task_evidence:<taskID>". Code is the reason code
// (the first ':'-segment of any prefix token, already isolated by
// ReasonCodeFromSpec); Subject is the first segment of the reason Detail (a
// skill name, task ID, layer, ...); Detail is the remainder.
type ParsedBlocker struct {
	Code    string `json:"code"`
	Subject string `json:"subject,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Raw     string `json:"raw"`
}

// ParseBlocker decomposes a ReasonCode into Code/Subject/Detail/Raw. It is the
// only place prefix-token detail is split into Subject + Detail, so views, the
// recovery builder, and CLIError all share one parse.
func ParseBlocker(rc ReasonCode) ParsedBlocker {
	rc.Normalize()
	subject, detail := splitSubjectDetail(rc.Detail)
	raw := rc.Code
	if trimmed := strings.TrimSpace(rc.Detail); trimmed != "" {
		raw = rc.Code + ":" + trimmed
	}
	return ParsedBlocker{
		Code:    rc.Code,
		Subject: subject,
		Detail:  detail,
		Raw:     raw,
	}
}

func splitSubjectDetail(detail string) (string, string) {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return "", ""
	}
	if before, after, ok := strings.Cut(detail, ":"); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	return detail, ""
}

// RecoveryClass categorizes how a blocker is recovered and drives primary-step
// selection. These are stable presentation-layer labels, not gate codes.
type RecoveryClass string

const (
	RecoveryClassConfirmPreset  RecoveryClass = "confirm_preset"
	RecoveryClassSatisfyControl RecoveryClass = "satisfy_control"
	RecoveryClassReopenEvidence RecoveryClass = "reopen_evidence"
	RecoveryClassRerunSkill     RecoveryClass = "rerun_skill"
	RecoveryClassFixScope       RecoveryClass = "fix_scope"
	RecoveryClassRefreshWave    RecoveryClass = "refresh_execution"
	RecoveryClassAdvance        RecoveryClass = "advance"
)

// recoveryClassPriority orders recovery classes from root-most (earliest
// lifecycle authority a stuck operator should address first) to latest. It is a
// STATIC ordering, deliberately NOT a per-change dependency graph — the
// dependency-ordered recovery planner is the P2 scope (#85). Lower index =
// higher priority for selecting the single primary command.
var recoveryClassPriority = []RecoveryClass{
	RecoveryClassConfirmPreset,
	RecoveryClassSatisfyControl,
	RecoveryClassReopenEvidence,
	RecoveryClassRerunSkill,
	RecoveryClassFixScope,
	RecoveryClassRefreshWave,
	RecoveryClassAdvance,
}

type blockerRemediation struct {
	// Remediation and CommandTemplate may contain {subject}/{detail}
	// placeholders filled from the ParsedBlocker.
	Remediation     string
	CommandTemplate string
	Class           RecoveryClass
	// SplitDetail forces recovery to expose the first detail segment as Subject
	// even when templates are static. Templates with {subject}/{detail} split
	// automatically; opaque prose details stay in Details as a whole.
	SplitDetail bool
	// Priority breaks ties inside one RecoveryClass. Lower wins; zero means the
	// default class-local priority.
	Priority int
}

const defaultRecoveryPriority = 100

// blockerRemediations maps a blocker Code to its remediation. Because every
// prefix family's Code is its first ':'-segment, Code-keying covers both exact
// codes and prefix families. It is the recovery-facing companion to
// canonicalReasonDefinitions, scoped to recovery-relevant tokens.
var blockerRemediations = map[string]blockerRemediation{
	"artifact_not_ready": {
		Remediation:     "Complete the governed artifact readiness issue named in the blocker detail, then re-run the workflow.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassSatisfyControl,
	},
	"artifact_schema_missing": {
		Remediation:     "Repair or regenerate the governed artifact schema before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"assurance_structure_invalid": {
		Remediation:     "Author assurance.md from the current artifact instructions, replace placeholder scaffold with real verification substance, then re-run validation.",
		CommandTemplate: "slipway instructions assurance",
		Class:           RecoveryClassSatisfyControl,
	},
	"assurance_contract_missing": {
		Remediation:     "Author assurance.md from the current artifact instructions, write the real file, then re-run validation.",
		CommandTemplate: "slipway instructions assurance",
		Class:           RecoveryClassSatisfyControl,
	},
	"assurance_contract_path_invalid": {
		Remediation:     "Repair the governed bundle path for assurance.md before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"assurance_contract_unreadable": {
		Remediation:     "Fix assurance.md so it is readable before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassSatisfyControl,
	},
	"assurance_section_placeholder": {
		Remediation:     "Replace the placeholder section in assurance.md with real closeout evidence, then re-run validation.",
		CommandTemplate: "slipway instructions assurance",
		Class:           RecoveryClassSatisfyControl,
	},
	"decision_contract_path_invalid": {
		Remediation:     "Repair the governed bundle path for decision.md before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"decision_contract_unreadable": {
		Remediation:     "Fix decision.md so it is readable before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"decision_structure_invalid": {
		Remediation:     "Author decision.md from the current artifact instructions, fix the required decision sections, then re-run validation.",
		CommandTemplate: "slipway instructions decision",
		Class:           RecoveryClassSatisfyControl,
	},
	"decision_section_placeholder": {
		Remediation:     "Replace the placeholder section in decision.md with a concrete decision, then re-run validation.",
		CommandTemplate: "slipway instructions decision",
		Class:           RecoveryClassSatisfyControl,
	},
	"closeout_goal_verification_reuse_invalid": {
		Remediation:     "Final-closeout cannot safely reuse goal-verification evidence; re-run goal-verification, then re-run final-closeout.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        15,
	},
	"dedicated_worktree_branch_mismatch": {
		Remediation:     "Run `slipway run` to reconcile the recorded branch to the bound worktree's actual branch and continue.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"dedicated_worktree_metadata_required": {
		Remediation:     "Record or repair dedicated worktree metadata before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"dedicated_worktree_path_invalid": {
		Remediation:     "Repair the recorded dedicated worktree path before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"dedicated_worktree_required": {
		Remediation:     "Bind the change to a dedicated worktree before continuing.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassSatisfyControl,
	},
	"preset_confirmation_required": {
		Remediation:     "Confirm the workflow preset before continuing.",
		CommandTemplate: "slipway preset <light|standard|strict>",
		Class:           RecoveryClassConfirmPreset,
	},
	"governance_action_required": {
		Remediation:     "Satisfy the required {subject} governance control before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassSatisfyControl,
	},
	"governed_bundle_path_invalid": {
		Remediation:     "Repair the governed bundle path before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"high_risk_check_failed": {
		Remediation:     "The high-risk safety check {subject} failed in goal-verification; fix the SAST findings, re-run goal-verification, and record `high_risk_check:{subject}=pass` in its References before continuing.",
		CommandTemplate: "slipway validate --focus sast",
		Class:           RecoveryClassSatisfyControl,
	},
	"high_risk_check_missing": {
		Remediation:     "Record the required high-risk safety check {subject} from goal-verification: run SAST (e.g. `slipway validate --focus sast`), triage findings, then add `high_risk_check:{subject}=pass` to the goal-verification References (use `=fail` to block ship); then continue.",
		CommandTemplate: "slipway validate --focus sast",
		Class:           RecoveryClassSatisfyControl,
	},
	"incomplete_execution_task": {
		Remediation:     "Execute task {subject} and record its evidence with `slipway evidence task`, or rescope tasks.md to drop it, then re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"intake_confirmation_incomplete": {
		Remediation:     "Complete the Approved Summary in intent.md before continuing.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"intake_substep_invalid": {
		Remediation:     "Repair the intake substep state before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"manifest_r0_invalid": {
		Remediation:     "Fix the R0 review manifest evidence and re-run review.",
		CommandTemplate: "slipway review",
		Class:           RecoveryClassRerunSkill,
	},
	"missing_task_evidence_for_run_summary": {
		Remediation:     "Task evidence is missing for the recorded run summary; re-run wave-orchestration to capture it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"missing_worktree_branch": {
		Remediation:     "Repair the change worktree binding so it records a branch.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"missing_worktree_path": {
		Remediation:     "Repair the change worktree binding so it records a path.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"non_pass_task": {
		Remediation:     "Resolve the failing task evidence for task {subject}, then re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"plan_dimension_completeness_missing_objective": {
		Remediation:     "Fix tasks.md so every task has a concrete objective.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_execution_missing_wave": {
		Remediation:     "Fix tasks.md so every task declares an execution wave.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_key_links_missing_target_files": {
		Remediation:     "Fix tasks.md so every task declares target_files.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_scope_invalid_target": {
		Remediation:     "Fix invalid task target_files entries in tasks.md.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_scope_out_of_bounds_target": {
		Remediation:     "Move task target_files back inside the repository or remove the out-of-bounds target.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_dependency_self_reference": {
		Remediation:     "Fix tasks.md so tasks do not depend on themselves.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_dependency_unknown": {
		Remediation:     "Fix tasks.md dependencies so every dependency references a declared task.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_dependency_cycle_detected": {
		Remediation:     "Break the dependency cycle in tasks.md before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_execution_invalid_wave_plan": {
		Remediation:     "Fix the wave/dependency plan in tasks.md before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_coverage_spec_unreadable": {
		Remediation:     "Restore readable requirements.md coverage input, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_coverage_requirements_invalid": {
		Remediation:     "Author requirements.md substance: each requirement needs a stable REQ-* id, a normative MUST/SHALL body, and at least one concrete scenario. Run `slipway instructions requirements` for the template and bar, then re-run validation.",
		CommandTemplate: "slipway instructions requirements",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_coverage_requirement_id_missing": {
		Remediation:     "Add explicit requirement IDs in requirements.md, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_coverage_unknown_requirement": {
		Remediation:     "Fix tasks.md covers entries so they reference declared requirements.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"plan_dimension_coverage_missing_requirement": {
		Remediation:     "Add task coverage for every requirement or update requirements.md.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"required_artifact_schema_missing": {
		Remediation:     "Fix the governed artifact schema so required artifact {subject} is defined.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"required_artifact_dependency_missing": {
		Remediation:     "Fix the governed artifact schema dependency {subject}, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassSatisfyControl,
	},
	"required_artifact_unreadable": {
		Remediation:     "Fix the required governed artifact {subject} so it is readable, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassSatisfyControl,
	},
	"required_skill_missing": {
		Remediation:     "Run the {subject} governance skill and record its verification evidence, then advance.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"required_skill_not_ready": {
		Remediation:     "The {subject} skill evidence is present but not ready; re-run it and record fresh evidence.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"required_skill_not_passed": {
		Remediation:     "The {subject} skill did not pass; resolve its findings and re-run it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"required_skill_blockers_present": {
		Remediation:     "The {subject} skill still reports blockers; resolve them and re-run it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"required_skill_stale": {
		Remediation:     "Inputs certified by {subject} changed; run Slipway to reopen the earliest affected authority and re-run the owning stage.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"research_structure_invalid": {
		Remediation:     "Author research.md from the current artifact instructions, fix the required research sections, then re-run validation.",
		CommandTemplate: "slipway instructions research",
		Class:           RecoveryClassSatisfyControl,
	},
	"research_section_placeholder": {
		Remediation:     "Replace the placeholder section in research.md with evidence-backed research, then re-run validation.",
		CommandTemplate: "slipway instructions research",
		Class:           RecoveryClassSatisfyControl,
	},
	"tasks_plan_changed_since_task_evidence": {
		Remediation:     "Task {subject}'s plan changed after its evidence was captured; re-run wave-orchestration for the affected task.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"stale_planning_evidence": {
		Remediation:     "Planning artifacts changed after execution evidence; reopen planning audit, then refresh execution evidence in order.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassReopenEvidence,
	},
	"stale_evidence_recovery_available": {
		Remediation:     "Stale evidence can be recovered by reopening the earliest affected authority.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassReopenEvidence,
	},
	"stale_execution_evidence": {
		Remediation:     "Execution evidence is stale; re-run wave-orchestration for the affected tasks.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"plan_audit_failed": {
		Remediation:     "Plan audit did not pass; fix the bundle blockers and re-run plan-audit.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_audit_evidence_missing": {
		Remediation:     "Plan audit evidence is missing; run plan-audit and record passing evidence.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_audit_iteration": {
		Remediation:     "Plan audit is iterating; incorporate checker feedback before continuing.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_audit_stalled": {
		Remediation:     "Plan audit feedback did not improve; revise the bundle and re-run plan-audit.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_audit_budget_exhausted": {
		// rescope is S2_EXECUTE-only (gate.EvaluateGPivot); from S1 plan-audit the
		// runnable escape is reroute (valid S1-S4) or manual bundle revision.
		Remediation:     "Plan audit exhausted its checker iteration budget; revise the plan bundle to resolve the outstanding checker feedback and re-run plan-audit, or reroute the change to a different approach.",
		CommandTemplate: "slipway pivot --reroute",
		Class:           RecoveryClassSatisfyControl,
	},
	"plan_checker_feedback_required": {
		Remediation:     "Apply the plan checker feedback to the bundle before re-running plan-audit.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_checker_loop_terminated": {
		// Same S1 constraint as plan_audit_budget_exhausted: reroute, not rescope.
		Remediation:     "The plan checker loop terminated before plan audit could pass; revise the plan bundle and re-run plan-audit, or reroute the change to a different approach.",
		CommandTemplate: "slipway pivot --reroute",
		Class:           RecoveryClassSatisfyControl,
	},
	"verification_evidence_missing": {
		Remediation:     "Required verification evidence is missing; in S4 recovery re-run goal-verification, then final-closeout.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        10,
	},
	"closeout_assurance_attestation_missing": {
		// Detail carries a colon-bearing sentence, so this remediation/command are
		// static and recovery treats the detail as opaque to avoid a misleading
		// subject split.
		Remediation:     "Final-closeout did not record the closeout assurance attestation (closeout:assurance_complete=pass) required on standard/strict workflows; re-run final-closeout and record it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        20,
	},
	"review_layer_missing": {
		Remediation:     "Re-run review and record passing evidence for review layer {subject}.",
		CommandTemplate: "slipway review",
		Class:           RecoveryClassRerunSkill,
	},
	"review_layer_failed": {
		Remediation:     "Resolve review findings for layer {subject}, then re-run review.",
		CommandTemplate: "slipway review",
		Class:           RecoveryClassRerunSkill,
	},
	"ship_gate_blocked": {
		Remediation:     "Refresh verification evidence, resolve the ship gate blockers, and re-run finalization.",
		CommandTemplate: "slipway done",
		Class:           RecoveryClassRerunSkill,
		Priority:        90,
	},
	"scope_contract_drift": {
		Remediation:     "Changed files fall outside the planned Scope Contract; add the targets to tasks.md or revert the out-of-scope change.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"scope_contract_missing": {
		Remediation:     "The Scope Contract is missing required target files; add them to the tasks.md plan.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"scope_contract_changed_files_missing": {
		Remediation:     "Task changed_files evidence is missing for the Scope Contract; re-run wave-orchestration to record it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"scope_contract_evaluation_failed": {
		Remediation:     "Fix the Scope Contract evaluation error, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"tasks_checklist_invalid_format": {
		Remediation:     "Fix the tasks.md checklist format before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"tasks_checklist_missing": {
		Remediation:     "Author tasks.md from the current artifact instructions, write the real task checklist, then re-run validation.",
		CommandTemplate: "slipway instructions tasks.md",
		Class:           RecoveryClassFixScope,
	},
	"tasks_checklist_path_invalid": {
		Remediation:     "Fix the tasks.md path or governed bundle path before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"tasks_checklist_unreadable": {
		Remediation:     "Fix tasks.md so it is readable before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"tasks_checklist_empty": {
		Remediation:     "Add at least one governed task entry to tasks.md before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"tasks_checklist_task_id_missing": {
		Remediation:     "Add a task ID to each tasks.md checklist item before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"tasks_checklist_duplicate_task_id": {
		Remediation:     "Make tasks.md task IDs unique before continuing.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"wave_plan_missing": {
		Remediation:     "Materialize the wave plan from tasks.md before wave execution.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"wave_orchestration_run_summary_version_invalid": {
		Remediation:     "Re-run wave-orchestration to produce a versioned run summary.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"wave_orchestration_stale_task_evidence": {
		Remediation:     "Task evidence post-dates the wave record; re-run wave-orchestration to refresh it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"worktree_validation_error": {
		Remediation:     "Repair the worktree validation failure before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"worktree_metadata_persist_failed": {
		Remediation:     "Repair the worktree metadata persistence failure before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"missing_discovery_evidence": {
		Remediation:     "Provide discovery (research-orchestration) evidence before planning.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"missing_required_artifact": {
		Remediation:     "Author the required governed artifact {subject}: run `slipway instructions {subject}` for its template, resolved output path, and upstream inputs, write the real file, then re-run validation.",
		CommandTemplate: "slipway instructions {subject}",
		Class:           RecoveryClassSatisfyControl,
	},
	"intake_clarification_incomplete": {
		Remediation:     "Complete intake clarification before planning.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"run_slipway_run_to_advance": {
		Remediation:     "The current step is ready; advance the workflow.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassAdvance,
	},
	"run_slipway_done_to_finalize": {
		Remediation:     "All governance gates passed; finalize the governed change.",
		CommandTemplate: "slipway done",
		Class:           RecoveryClassAdvance,
	},
}

// RecoveryStep is one actionable (code, subject) group rendered with its parsed
// segments and remediation. Blockers that share a code and subject but differ
// only in their detail (e.g. one stale skill spanning many artifacts) collapse
// into a single step whose Details list the distinct remainders.
type RecoveryStep struct {
	Code          string         `json:"code"`
	Subject       string         `json:"subject,omitempty"`
	Details       []string       `json:"details,omitempty"`
	Severity      ReasonSeverity `json:"severity"`
	Message       string         `json:"message"`
	Remediation   string         `json:"remediation"`
	Command       string         `json:"command,omitempty"`
	RecoveryClass RecoveryClass  `json:"recovery_class,omitempty"`
	priority      int
}

// RecoverySummary is the top-level read-only recovery object surfaced on
// next/run/validate and CLIError. It carries one primary command/action chosen
// by a static stage-priority rule plus a step per actionable blocker.
type RecoverySummary struct {
	PrimaryCommand string         `json:"primary_command,omitempty"`
	PrimaryAction  string         `json:"primary_action,omitempty"`
	RecoveryClass  RecoveryClass  `json:"recovery_class,omitempty"`
	Steps          []RecoveryStep `json:"steps,omitempty"`
}

// BuildRecovery projects actionable blockers into a RecoverySummary. Blockers
// that share a (code, subject) collapse into one step whose Details list the
// distinct remainders — neither the command nor the remediation varies by detail
// within a group — so a multi-artifact drift surfaces one actionable step instead
// of N near-identical ones. It returns nil when no blocker has a recovery-relevant
// remediation, so the field stays absent (omitempty). It never re-judges gates —
// it only renders the blockers it is given.
func BuildRecovery(blockers []ReasonCode) *RecoverySummary {
	type groupKey struct{ code, subject string }
	groups := map[groupKey][]ReasonCode{}
	var order []groupKey
	for _, rc := range NormalizeReasonCodes(blockers) {
		remediation, ok := blockerRemediations[rc.Code]
		if !ok {
			continue
		}
		parsed := parseBlockerForRecovery(rc, remediation)
		key := groupKey{parsed.Code, parsed.Subject}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], rc)
	}
	if len(order) == 0 {
		return nil
	}
	steps := make([]RecoveryStep, 0, len(order))
	for _, key := range order {
		steps = append(steps, recoveryStepForGroup(groups[key]))
	}
	primary := selectPrimaryStep(steps)
	return &RecoverySummary{
		PrimaryCommand: primary.Command,
		PrimaryAction:  primary.Remediation,
		RecoveryClass:  primary.RecoveryClass,
		Steps:          steps,
	}
}

// recoveryStepForGroup renders one step for a group of blockers sharing a
// (code, subject). The representative drives the remediation/command; Details
// collects the distinct, sorted remainders the group spans.
func recoveryStepForGroup(group []ReasonCode) RecoveryStep {
	rep := group[0]
	rep.Normalize()
	base := blockerRemediations[rep.Code]
	parsed := parseBlockerForRecovery(rep, base)
	priority := recoveryPriority(base)
	if parsed.Code == "missing_required_artifact" {
		priority = missingRequiredArtifactRecoveryPriority(parsed.Subject)
	}
	return RecoveryStep{
		Code:          parsed.Code,
		Subject:       parsed.Subject,
		Details:       groupDetails(group, base),
		Severity:      rep.Severity,
		Message:       groupMessage(parsed.Code, rep, len(group)),
		Remediation:   fillTemplate(base.Remediation, parsed),
		Command:       resolveCommandTemplate(base.CommandTemplate, parsed),
		RecoveryClass: base.Class,
		priority:      priority,
	}
}

func missingRequiredArtifactRecoveryPriority(subject string) int {
	switch strings.TrimSpace(subject) {
	case "intent.md":
		return 10
	case "research.md":
		return 20
	case "requirements.md":
		return 30
	case "decision.md":
		return 40
	case "tasks.md":
		return 50
	case "assurance.md":
		return 60
	default:
		return defaultRecoveryPriority
	}
}

// groupDetails returns the sorted, de-duplicated non-empty details across a group.
func groupDetails(group []ReasonCode, remediation blockerRemediation) []string {
	seen := map[string]struct{}{}
	var details []string
	for _, rc := range group {
		detail := parseBlockerForRecovery(rc, remediation).Detail
		if detail == "" {
			continue
		}
		if _, ok := seen[detail]; ok {
			continue
		}
		seen[detail] = struct{}{}
		details = append(details, detail)
	}
	sort.Strings(details)
	return details
}

func parseBlockerForRecovery(rc ReasonCode, remediation blockerRemediation) ParsedBlocker {
	rc.Normalize()
	if remediationShouldSplitDetail(remediation) {
		return ParseBlocker(rc)
	}
	detail := strings.TrimSpace(rc.Detail)
	raw := rc.Code
	if detail != "" {
		raw = rc.Code + ":" + detail
	}
	return ParsedBlocker{
		Code:   rc.Code,
		Detail: detail,
		Raw:    raw,
	}
}

func remediationShouldSplitDetail(remediation blockerRemediation) bool {
	if remediation.SplitDetail {
		return true
	}
	return strings.Contains(remediation.Remediation, "{subject}") ||
		strings.Contains(remediation.Remediation, "{detail}") ||
		strings.Contains(remediation.CommandTemplate, "{subject}") ||
		strings.Contains(remediation.CommandTemplate, "{detail}")
}

// groupMessage keeps the representative's specific message for a singleton group
// (it still names the one detail) and falls back to the code-level canonical
// message when several details were collapsed, since the specifics now live in
// Details.
func groupMessage(code string, rep ReasonCode, size int) string {
	if size <= 1 {
		return rep.Message
	}
	return NewReasonCode(code, "").Message
}

// resolveCommandTemplate fills a command template and falls back to a generic
// advance command if the template needs a subject the blocker did not carry, so
// a recovery command is never emitted with an empty placeholder.
func resolveCommandTemplate(template string, parsed ParsedBlocker) string {
	if template == "" {
		return ""
	}
	if strings.Contains(template, "{subject}") && strings.TrimSpace(parsed.Subject) == "" {
		return "slipway run"
	}
	return fillTemplate(template, parsed)
}

func fillTemplate(template string, parsed ParsedBlocker) string {
	out := strings.ReplaceAll(template, "{subject}", parsed.Subject)
	out = strings.ReplaceAll(out, "{detail}", parsed.Detail)
	return tidyTemplate(out)
}

// tidyTemplate removes the artifacts an absent {subject}/{detail} would leave —
// the double space where the placeholder was, and a space orphaned in front of
// punctuation — so a remediation never surfaces "  " or " .".
func tidyTemplate(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for _, p := range []string{" .", " ,", " :", " ;"} {
		s = strings.ReplaceAll(s, p, p[1:])
	}
	return strings.TrimSpace(s)
}

// selectPrimaryStep picks the single primary recovery step by static class
// priority, breaking ties by severity (error before warning) then code, so the
// selection is deterministic and independent of blocker ordering.
func selectPrimaryStep(steps []RecoveryStep) RecoveryStep {
	best := steps[0]
	for _, step := range steps[1:] {
		if lessRecoveryStep(step, best) {
			best = step
		}
	}
	return best
}

func lessRecoveryStep(a, b RecoveryStep) bool {
	oa, ob := recoveryPrimaryOverrideRank(a), recoveryPrimaryOverrideRank(b)
	if oa != ob {
		return oa < ob
	}
	pa, pb := recoveryClassRank(a.RecoveryClass), recoveryClassRank(b.RecoveryClass)
	if pa != pb {
		return pa < pb
	}
	ra, rb := recoveryStepPriority(a), recoveryStepPriority(b)
	if ra != rb {
		return ra < rb
	}
	sa, sb := severityRank(a.Severity), severityRank(b.Severity)
	if sa != sb {
		return sa < sb
	}
	return a.Code < b.Code
}

func recoveryPrimaryOverrideRank(step RecoveryStep) int {
	switch step.Code {
	case "stale_evidence_recovery_available":
		return 0
	default:
		return 1
	}
}

func recoveryClassRank(class RecoveryClass) int {
	for i, c := range recoveryClassPriority {
		if c == class {
			return i
		}
	}
	return len(recoveryClassPriority)
}

func recoveryPriority(remediation blockerRemediation) int {
	if remediation.Priority > 0 {
		return remediation.Priority
	}
	return defaultRecoveryPriority
}

func recoveryStepPriority(step RecoveryStep) int {
	if step.priority > 0 {
		return step.priority
	}
	return defaultRecoveryPriority
}

func severityRank(s ReasonSeverity) int {
	switch s {
	case ReasonSeverityError:
		return 0
	case ReasonSeverityWarning:
		return 1
	default:
		return 2
	}
}
