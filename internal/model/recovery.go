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
	RecoveryClassConfirmPreset   RecoveryClass = "confirm_preset"
	RecoveryClassPreserveWork    RecoveryClass = "preserve_work"
	RecoveryClassDiscardChange   RecoveryClass = "discard_change"
	RecoveryClassNewChange       RecoveryClass = "new_change"
	RecoveryClassSatisfyControl  RecoveryClass = "satisfy_control"
	RecoveryClassReviewAlignment RecoveryClass = "review_alignment"
	RecoveryClassRerunSkill      RecoveryClass = "rerun_skill"
	RecoveryClassFixScope        RecoveryClass = "fix_scope"
	RecoveryClassRefreshWave     RecoveryClass = "refresh_execution"
	RecoveryClassAdvance         RecoveryClass = "advance"
)

// recoveryClassPriority orders recovery classes from root-most (earliest
// lifecycle authority a stuck operator should address first) to latest. It is a
// STATIC ordering, deliberately NOT a per-change dependency graph — the
// dependency-ordered recovery planner is the P2 scope (#85). Lower index =
// higher priority for selecting the single primary command.
var recoveryClassPriority = []RecoveryClass{
	RecoveryClassConfirmPreset,
	// PreserveWork outranks DiscardChange so that when a no-target recovery
	// surfaces both an unmanaged-worktree orphan and a plain discardable orphan,
	// the non-destructive preserve-first action is chosen as primary (#285).
	RecoveryClassPreserveWork,
	RecoveryClassNewChange,
	RecoveryClassDiscardChange,
	RecoveryClassSatisfyControl,
	RecoveryClassReviewAlignment,
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
	"archive_failed": {
		Remediation:     "Inspect the archive failure detail, repair the governed bundle or filesystem issue, then re-run bulk finalization.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"artifact_not_ready": {
		Remediation:     "Complete the governed artifact readiness issue named in the blocker detail, then re-run the workflow.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassSatisfyControl,
	},
	"artifact_reconcile_failed": {
		Remediation:     "Repair artifact reconciliation for the governed bundle, then re-run finalization.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"artifact_schema_missing": {
		Remediation:     "Repair or regenerate the governed artifact schema before continuing.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"artifact_validation_failed": {
		Remediation:     "Fix the invalid done artifact named in the failure detail, then re-run finalization.",
		CommandTemplate: "slipway validate",
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
	"decision_status_rejected": {
		Remediation:     "Revise or replace the superseded, deprecated, or rejected decision.md status before continuing.",
		CommandTemplate: "slipway instructions decision",
		Class:           RecoveryClassSatisfyControl,
	},
	"decision_status_unknown": {
		Remediation:     "Change decision.md to a supported status or remove the status section before continuing.",
		CommandTemplate: "slipway instructions decision",
		Class:           RecoveryClassSatisfyControl,
	},
	"change_not_active": {
		Remediation:     "Inspect active changes and ignore or remove skipped non-active records before re-running bulk finalization.",
		CommandTemplate: "slipway status",
		Class:           RecoveryClassSatisfyControl,
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
	"new_change_required": {
		Remediation:     "The requested work is no longer the same governed intent; open a new governed change and carry over only the relevant reviewed context.",
		CommandTemplate: "slipway new",
		Class:           RecoveryClassNewChange,
	},
	"review_required": {
		Remediation:     "Run review convergence before finalization; review owns plan/code/evidence alignment gates.",
		CommandTemplate: "slipway review",
		Class:           RecoveryClassReviewAlignment,
	},
	"review_alignment_required": {
		Remediation:     "Run `slipway fix` to dispatch a fresh-context repair subagent for the stale authority, update code/artifacts/evidence inside the current change, record affected reviewer evidence with `context_origin:stage=fix=<handle>`, then rerun `slipway review`.",
		CommandTemplate: "slipway fix",
		Class:           RecoveryClassReviewAlignment,
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
		Remediation:     "The high-risk safety check {subject} failed in ship-verification; fix the SAST findings, re-run ship-verification, and record `high_risk_check:{subject}=pass` in its References before continuing.",
		CommandTemplate: "slipway validate --focus sast",
		Class:           RecoveryClassSatisfyControl,
	},
	"high_risk_check_missing": {
		Remediation:     "Record the required high-risk safety check {subject} from ship-verification: run SAST (e.g. `slipway validate --focus sast`), triage findings, then add `high_risk_check:{subject}=pass` to the ship-verification References (use `=fail` to block ship); then continue.",
		CommandTemplate: "slipway validate --focus sast",
		Class:           RecoveryClassSatisfyControl,
	},
	"host_capability_unavailable": {
		Remediation:     "Run the selected governance skill in a host that provides {detail}, or explicitly select the documented fallback for {subject} and record fresh evidence.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassSatisfyControl,
		SplitDetail:     true,
	},
	"subagent_dispatch_authorization_required": {
		Remediation:     "The host has not declared {detail} dispatch available for {subject}: authorize host {detail} delegation, or explicitly select a named degraded fallback for {subject} and record fresh evidence. Do not review or audit inline in the host context without recording the chosen fallback.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassSatisfyControl,
		SplitDetail:     true,
	},
	"incomplete_execution_task": {
		Remediation:     "Record task evidence for {subject}: the wave host must decide the outcome, then run `slipway evidence task --task-id {subject} --verdict <verdict> --evidence-ref <ref> [--changed-file <path>]... --json` before re-running wave-orchestration. For delegated work, use the subagent's factual report as evidence_ref material; for inline work, use the host's command/transcript proof.",
		CommandTemplate: "slipway evidence task --task-id {subject} --verdict <verdict> --evidence-ref <ref> [--changed-file <path>]...",
		Class:           RecoveryClassSatisfyControl,
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
	"lifecycle_event_write_failed": {
		Remediation:     "Repair lifecycle event logging or filesystem permissions, then re-run finalization.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"list_changes_failed": {
		Remediation:     "Repair change state so active changes can be listed, then re-run bulk finalization.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"load_change_failed": {
		Remediation:     "Repair the governed change state named in the failure detail, then re-run bulk finalization.",
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
	"non_pass_task": {
		Remediation:     "Resolve the failing task evidence for task {subject}, then re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"not_done_ready": {
		Remediation:     "Complete the governed workflow gates before finalizing this change.",
		CommandTemplate: "slipway next",
		Class:           RecoveryClassRerunSkill,
	},
	"orphaned_bundle_ownership_unknown": {
		// Emitted as orphaned_bundle_ownership_unknown:<slug>. The bundle lost its
		// change.yaml and the git worktree/branch cross-check that proves ownership
		// FAILED, so we cannot show it is safe to discard. Fail closed: this is a
		// PRESERVE-first recovery, never a discard. CommandTemplate is empty so no
		// primary_command routes to `slipway delete`; the prose carries the action and
		// never recommends --worktree.
		Remediation:     "Governed bundle {subject} lost its change.yaml, and Slipway could not verify whether a live git worktree holds its work (the worktree cross-check failed). Do not discard it yet: inspect for a live worktree or branch named after {subject} and preserve any unmerged work first. Only after confirming no unmerged work remains, discard the stale residue with `slipway delete --change {subject}` (never pass --worktree).",
		CommandTemplate: "",
		Class:           RecoveryClassPreserveWork,
	},
	"orphaned_bundle_unmanaged_worktree": {
		// Emitted as orphaned_bundle_unmanaged_worktree:<slug>. The bundle lost its
		// change.yaml, but a live git worktree Slipway never provisioned still holds
		// work for the slug. This is a PRESERVE-first recovery, not a discard: the
		// structured surface must NOT route to `slipway delete` as the primary
		// command and must NOT carry the discard_change class (#285). The CommandTemplate
		// is deliberately empty — preservation is a manual, multi-step judgment with no
		// single safe automated command — so primary_command is omitted and the prose
		// carries the action. `slipway delete --change <slug>` survives only in prose as
		// the FINAL residue cleanup after the work is saved, and never with --worktree.
		Remediation:     "Governed bundle {subject} lost its change.yaml, but a live git worktree Slipway does not manage still holds work for this slug. Inspect and preserve that worktree and its branch first — Slipway never removes a worktree it did not provision. Once its work is merged or saved, discard only the stale bundle residue with `slipway delete --change {subject}` (never pass --worktree).",
		CommandTemplate: "",
		Class:           RecoveryClassPreserveWork,
	},
	"orphaned_change_bundle": {
		// Emitted as orphaned_change_bundle:<slug>. The governed bundle directory
		// survived without its change.yaml authority (a partially-deleted change),
		// which would otherwise dead-end resolution; route the operator to discard
		// the abandoned local state with the public delete surface.
		Remediation:     "Governed bundle {subject} is missing its change.yaml authority; discard the abandoned change with `slipway delete --change {subject}` (add --worktree to also remove its worktree).",
		CommandTemplate: "slipway delete --change {subject}",
		Class:           RecoveryClassDiscardChange,
	},
	"stale_runtime_binding": {
		// Emitted as stale_runtime_binding:<slug>. The active bundle directory was
		// removed entirely but git-local runtime state still records the abandoned
		// change, so route to the same public discard surface.
		Remediation:     "Runtime binding for {subject} remains after its governed bundle was removed; discard the abandoned local state with `slipway delete --change {subject}` (add --worktree to also remove its worktree).",
		CommandTemplate: "slipway delete --change {subject}",
		Class:           RecoveryClassDiscardChange,
	},
	"plan_dimension_completeness_missing_objective": {
		Remediation:     "Fix tasks.md so every task has a concrete objective.",
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
	"plan_dimension_decision_soundness_unattested": {
		Remediation:     "Re-run the owning plan-dimension audit skill and record a passing decision_soundness attestation reference.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_consistency_unattested": {
		Remediation:     "Re-run the owning plan-dimension audit skill and record a passing consistency attestation reference.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_attestation_invalid": {
		Remediation:     "Fix malformed dim:<name>=<verdict>:<evidence-ref> evidence references and re-record the owning skill evidence.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_attestation_conflict": {
		Remediation:     "Remove conflicting plan-dimension verdict references and re-record a single owning skill verdict for the dimension.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_attestation_evidence_unresolvable": {
		Remediation:     "Change the plan-dimension evidence reference to a resolvable workspace-relative path and re-record the owning skill evidence.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_decision_soundness_evidence_invalid": {
		Remediation:     "Re-run the decision-soundness audit with codebase-grounded evidence outside artifacts/ and re-record the owning skill evidence.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_decision_unsound": {
		Remediation:     "Repair the current plan artifacts in place to address the unsound decision, then rerun the owning plan-dimension audit skill.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_consistency_failed": {
		Remediation:     "Repair inconsistent plan artifacts in place, then rerun the owning plan-dimension audit skill.",
		CommandTemplate: "slipway evidence skill --skill plan-audit --verdict pass",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_dimension_consistency_unknown_requirement_ref": {
		Remediation:     "Fix governed artifact prose so every REQ-* reference names a declared requirement, then re-run validation.",
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
		Remediation:     "Run the {subject} governance skill and record its skill evidence, then advance. In S3 this means the profile-filtered selected peer skills reported by selected_review_skills: spec-compliance-review and independent-review, plus code-quality-review when selected by the workflow profile and security-review when selected by policy, together with the always-required ship-verification terminal gate.",
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
		Remediation:     "Inputs {subject} certified changed after its verdict; re-run the {subject} skill against the current inputs and record the fresh result with `slipway evidence skill` — do not restamp the stale verdict.",
		CommandTemplate: "slipway evidence skill --skill {subject} --verdict pass",
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
	"orphan_task_evidence": {
		Remediation:     "Task {subject} has recorded evidence at the current run version but is no longer in the current tasks.md wave plan; run `slipway repair` to prune readable current-run orphan task evidence after the wave plan is current. If repair reports non-repairable/malformed evidence or historical preservation applies, inspect the repair findings or intentionally reopen with `slipway fix --start-reexecution --discard-prior-evidence`.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassRefreshWave,
	},
	"s3_task_plan_drift_requires_inplace_convergence": {
		Remediation:     "tasks.md has S3 task-plan amendments ({subject}) that the materialized wave plan has not absorbed yet. Run S3 in-place convergence to re-materialize the wave plan at the same run_summary_version and preserve prior task evidence when the preserved changed_files still fit the amended target_files. Record task evidence only for newly added tasks surfaced as incomplete; edited already-evidenced tasks are re-certified through review evidence before wave-orchestration is re-recorded. If convergence reports preserved changed_files outside the amended target_files, restore honest target coverage or explicitly reopen with prior-evidence discard.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassReviewAlignment,
		Priority:        5,
		SplitDetail:     true,
	},
	"s3_task_plan_drift_requires_reexecution": {
		Remediation:     "Task {subject}'s preserved S3 task evidence cannot honestly certify the amended task-plan detail ({detail}); restore target_files coverage and the preserved task semantics, then run `slipway run`, or intentionally reopen execution with prior task evidence discard.",
		CommandTemplate: "slipway fix --start-reexecution --discard-prior-evidence",
		Class:           RecoveryClassReviewAlignment,
		Priority:        4,
		SplitDetail:     true,
	},
	"stale_planning_evidence": {
		Remediation:     "Planning artifacts changed after execution evidence; repair plan/code alignment, then refresh affected S2+ execution evidence in order.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"stale_execution_evidence": {
		Remediation:     "Execution evidence is stale; re-run wave-orchestration for the affected tasks.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
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
		Remediation:     "Plan audit exhausted its checker iteration budget; revise the plan bundle to resolve the outstanding checker feedback and re-run plan-audit.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_checker_feedback_required": {
		Remediation:     "Apply the plan checker feedback to the bundle before re-running plan-audit.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_checker_loop_terminated": {
		Remediation:     "The plan checker loop terminated before plan audit could pass; revise the plan bundle and re-run plan-audit.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"ship_verification_evidence_missing": {
		Remediation:     "Required ship-verification evidence is missing; re-run ship-verification before done.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        10,
	},
	"ship_verification_evidence_stale": {
		Remediation:     "Ship-verification evidence exists but is invalidated by stale execution or review evidence; refresh the stale upstream evidence (re-run the stale reviewers and execution evidence), then re-run ship-verification before done. Do not restamp the stale ship-verification record.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        10,
	},
	"ship_verification_ordering_invalid": {
		// Detail carries a colon-bearing sentence (the out-of-order peer name), so
		// the remediation/command are static and recovery treats the detail as
		// opaque to avoid a misleading subject split.
		Remediation:     "Ship-verification is stamped before a selected S3 review peer; re-stamp the stale reviewer, then re-run ship-verification so it observes the final review evidence.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        20,
	},
	"ship_verification_assurance_attestation_missing": {
		// Detail carries a colon-bearing sentence, so this remediation/command are
		// static and recovery treats the detail as opaque to avoid a misleading
		// subject split.
		Remediation:     "Ship-verification did not record the assurance attestation (closeout:assurance_complete=pass) required on standard/strict workflows; re-run ship-verification and record it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        20,
	},
	"ship_verification_reviewer_independence_missing": {
		Remediation:     "Ship-verification did not record the reviewer-independence attestation (closeout:reviewer_independence=pass) required on standard/strict workflows; re-run ship-verification and record it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
		Priority:        20,
	},
	"context_origin_handle_invalid": {
		Remediation:     "A governed stage recorded a missing or invalid context-origin handle; re-run the owning stage through its configured fresh delegated session, defaulting to native host dispatch, so it records a valid context-origin handle.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"cross_stage_context_not_distinct": {
		// Detail names the colliding stage pair (earlier|later); recovery routes to
		// the later/owning stage, which must re-run through a fresh delegated session
		// so it records a context-origin handle distinct from the earlier stage.
		Remediation:     "Two governed stages share a context-origin handle; re-run the later (owning) stage of the colliding pair through its configured fresh delegated session, defaulting to native host dispatch, so it records a distinct context-origin handle.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"plan_audit_origin_invalid": {
		Remediation:     "Plan audit recorded the same author and auditor context-origin handle (self-audit); re-run plan-audit through its configured fresh delegated session, defaulting to native host dispatch, so its auditor handle is distinct from the plan author.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRerunSkill,
	},
	"degraded_dispatch_justification_missing": {
		Remediation:     "A degraded_sequential dispatch needs a genuine tool-unavailable justification; record degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail> and re-run wave-orchestration (the blocker detail is wave=<n>).",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"review_layer_missing": {
		Remediation:     "Re-run review and record passing evidence for review layer {subject}.",
		CommandTemplate: "slipway review",
		Class:           RecoveryClassRerunSkill,
	},
	"review_layer_failed": {
		Remediation:     "Run `slipway fix` to dispatch a fresh-context repair subagent for review layer {subject}, then rerun the affected reviewer and `slipway review`.",
		CommandTemplate: "slipway fix",
		Class:           RecoveryClassRerunSkill,
	},
	"ship_gate_blocked": {
		Remediation:     "Refresh verification evidence, resolve the ship gate blockers, and re-run finalization.",
		CommandTemplate: "slipway done",
		Class:           RecoveryClassRerunSkill,
		Priority:        90,
	},
	"scope_contract_drift": {
		Remediation:     "Changed files fall outside the planned Scope Contract; recorded wave evidence is preserved. If it is legitimate same-intent work, update the owning task `target_files` in `tasks.md` to cover them and re-run the current stage with `slipway run`. If S3 review has frozen the affected task evidence and the amended target_files intentionally exclude the preserved changed file, reopen with `slipway fix --start-reexecution --discard-prior-evidence` instead of trying to restamp that task evidence. If the objective changed, open a new governed change.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassReviewAlignment,
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
	"sensitive_evidence_missing": {
		Remediation:     "Sensitive changed file {detail} is missing owning evidence for {subject}. Keep or return to S2_IMPLEMENT through the lifecycle, then record task evidence with `slipway evidence task` using the required marker: schema_migration uses `migration-applied:<command>`, auth_authz uses `auth-review:<review-ref>`, and api_contract uses `contract-test:<test-command>`.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
		SplitDetail:     true,
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
	"unknown_reason_code": {
		Remediation:     "A blocker used an unrecognized reason code. Inspect the original token in the blocker detail, update the producer to emit a canonical reason code, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_execution_unavailable": {
		Remediation:     "Repair the wave execution context before re-running wave-orchestration.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_plan_drift": {
		Remediation:     "Rebuild the derived wave plan from the current tasks.md, then refresh affected execution evidence.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_plan_load_failed": {
		Remediation:     "Update tasks.md so its task IDs, dependencies, and target files form a schedulable wave plan, then run `slipway repair` to rebuild wave-plan.yaml from it.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_plan_missing": {
		Remediation:     "Rebuild wave-plan.yaml from the current tasks.md before wave execution.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_plan_unreadable": {
		Remediation:     "The engine-owned wave-plan.yaml cache is corrupt or carries unsupported fields; do not hand-edit it. Regenerate it with `slipway repair`, then refresh affected execution evidence with `slipway run`.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_run_missing": {
		Remediation:     "Record passing wave run evidence by re-running wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"wave_run_version_mismatch": {
		Remediation:     "Repair or regenerate wave run evidence so its run_summary_version matches the requested execution summary.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_runs_incomplete": {
		Remediation:     "Complete missing wave run evidence by re-running wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"wave_runs_invalid_count": {
		Remediation:     "Repair wave run evidence so the recorded run count matches the execution summary.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_runs_load_failed": {
		Remediation:     "Repair wave run evidence so status can load the authoritative run records.",
		CommandTemplate: "slipway repair",
		Class:           RecoveryClassSatisfyControl,
	},
	"wave_runs_missing": {
		Remediation:     "Re-run wave-orchestration to record wave run evidence for the latest execution summary.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"wave_runs_unreadable": {
		Remediation:     "Fix unreadable wave run evidence, then re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"wave_task_linkage_mismatch": {
		Remediation:     "Re-run wave-orchestration so wave run task linkage matches wave-plan.yaml.",
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

	// --- Wave/review blocker codes (completeness pass) ---
	// These surface as gate blockers in the lists passed to BuildRecovery by
	// next/status/validate/review, so each must carry a CLI-valid next action.
	// Commands are sourced from the producing stage, not guessed.
	"execution_verdict_fail": {
		Remediation:     "The recorded execution verdict is fail; resolve the failing wave/task evidence and re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"missing_run_summary": {
		Remediation:     "No execution run summary is recorded; run wave-orchestration to produce it.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"non_pass_wave": {
		Remediation:     "A governed execution wave did not pass; resolve the failing wave's task evidence and re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"task_blockers": {
		Remediation:     "A governed task reported blockers; resolve the task's blockers and re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"task_blockers_invalid_key": {
		Remediation:     "A governed task blocker key is invalid; fix the malformed task ID in tasks.md, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
	},
	"task": {
		// Wrapper code: detail is taskID:<innerCode>:<innerDetail>. Surface the
		// task ID and route to wave-orchestration, which owns task execution.
		Remediation:     "Task {subject} reported an execution blocker ({detail}); resolve it and re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
		SplitDetail:     true,
	},
	"task_blocker": {
		// Wrapper code: detail is taskID:<innerCode>:<innerDetail>.
		Remediation:     "Task {subject} reported a wave blocker ({detail}); resolve it and re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
		SplitDetail:     true,
	},
	"task_evidence_invalid": {
		Remediation:     "Task evidence is malformed; re-run wave-orchestration to re-record valid task evidence.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"task_changed_file_scope_escape": {
		Remediation:     "Task {subject} recorded changed file {detail} outside its planned target_files; fix the task's target_files in tasks.md to cover it (or revert the out-of-scope change), then re-run validation/current lifecycle. If this is S3 review and the task evidence is already frozen, do not try to restamp it; either restore honest target coverage and run S3 convergence, or intentionally reopen with `slipway fix --start-reexecution --discard-prior-evidence`.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassFixScope,
		SplitDetail:     true,
	},
	"parallel_wave_changed_file_overlap": {
		Remediation:     "Two tasks in the same plan-parallel wave recorded the same changed file and can clobber each other in the shared worktree; this is a plan-quality defect. Split the overlapping target_files, add an ordering dependency so the tasks no longer share a parallel wave, or stop for explicit operator replanning before re-running wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"dispatch_mode_absent_on_started_parallel_wave": {
		Remediation:     "A started parallel wave recorded no valid dispatch_mode; record dispatch_mode:wave=<n>:parallel_subagents evidence, or record dispatch_mode:wave=<n>:degraded_sequential plus degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>, then re-run wave-orchestration (the blocker detail is wave=<n>).",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"executor_agent_missing": {
		Remediation:     "A parallel_subagents wave is missing the executor_agent handle for a planned task; record executor_agent:wave=<n>:task=<id>:<handle> for every task in the wave and re-run wave-orchestration.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassRefreshWave,
	},
	"intent_drift": {
		Remediation:     "Implementation intent drift was detected; re-align the implementation with the approved intent and refresh affected evidence. If the intent itself changed, open a new governed change instead.",
		CommandTemplate: "slipway run",
		Class:           RecoveryClassReviewAlignment,
	},
	"invalid_blocker": {
		Remediation:     "A blocker token is invalid; inspect the original token in the blocker detail, fix the producer to emit a canonical reason code, then re-run validation.",
		CommandTemplate: "slipway validate",
		Class:           RecoveryClassSatisfyControl,
	},
}

// diagnosticOnlyReasonCodes are canonical reason codes that deliberately carry
// NO BuildRecovery command. Their omission from blockerRemediations is
// intentional, not a discoverability gap: each one's operator next-action is
// carried by a different public surface (named per entry), or the code is a
// terminal/informational/advisory state with no corrective command. This set is
// the explicit, test-enforced companion to blockerRemediations — every canonical
// reason code must appear in exactly one of the two (see TestRecoveryCoverageIsExhaustive),
// so a newly added code cannot silently surface with no discoverable action.
var diagnosticOnlyReasonCodes = map[string]struct{}{
	// Surfaced only as `slipway health` findings; the operator next-action is
	// carried by that finding's RepairHint, not by BuildRecovery (health never
	// calls BuildRecovery). The RepairHint command is noted for traceability.
	"multiple_active_changes":              {}, // RepairHint: slipway status
	"execution_interrupted":                {}, // RepairHint: slipway run --resume
	"orphan_bundle_directory":              {}, // RepairHint: inspect/remove the orphan bundle manually
	"task_evidence_unreadable":             {}, // RepairHint: regenerate execution evidence (slipway run)
	"workspace_scope_config_missing":       {}, // RepairHint: slipway repair
	"workspace_scope_marker_missing":       {}, // RepairHint: slipway repair
	"skill_prompt_surface_missing":         {}, // RepairHint: slipway init --tools <id> --refresh
	"skill_prompt_surface_unreadable":      {}, // RepairHint: slipway init --tools <id> --refresh
	"codebase_map_freshness_missing":       {}, // RepairHint: slipway codebase-map
	"codebase_map_freshness_partial":       {}, // RepairHint: slipway codebase-map
	"codebase_map_freshness_scaffold_only": {}, // RepairHint: slipway codebase-map
	"codebase_map_freshness_stale":         {}, // RepairHint: slipway codebase-map
	"codebase_map_freshness_unknown":       {}, // RepairHint: slipway codebase-map
	"lifecycle_event_log_unreadable":       {}, // RepairHint: inspect events/lifecycle.jsonl
	"lifecycle_event_scan_failed":          {}, // RepairHint: inspect repo state, rerun health
	"lifecycle_event_scan_skipped":         {}, // RepairHint: fix change.yaml (real issue surfaces as change_bundle_unreadable)
	"archived_lifecycle_event_scan_failed": {}, // RepairHint: inspect archived change directory

	// Surfaced as a state-integrity CLIError whose next-action is carried by the
	// error's structured Remediation field (newStateIntegrityError), not by
	// BuildRecovery (which receives no reasons on those error paths).
	"config_parse_failure":         {}, // CLIError.Remediation: slipway repair
	"change_bundle_unreadable":     {}, // CLIError.Remediation: fix change.yaml, then slipway repair
	"execution_summary_unreadable": {}, // CLIError.Remediation: fix execution-summary.yaml, then slipway repair
	"skill_registry_invalid":       {}, // CLIError.Remediation: slipway repair (restore generated skills)

	// Terminal / informational / advisory states. These can reach BuildRecovery in
	// a blocker list, but they need no corrective command: `next` renders their
	// guidance directly, or they are non-actionable advisories.
	"change_is_done":            {}, // terminal success; `next` renders done guidance
	"no_skill_required":         {}, // informational advance posture; `next` renders advance guidance
	"session_isolation_warning": {}, // advisory evidence warning; no single corrective command
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
		remediation, ok := recoveryRemediationForReason(rc)
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
	sort.SliceStable(steps, func(i, j int) bool {
		return lessRecoveryStep(steps[i], steps[j])
	})
	primary := steps[0]
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
	base, _ := recoveryRemediationForReason(rep)
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

func recoveryRemediationForReason(rc ReasonCode) (blockerRemediation, bool) {
	remediation, ok := blockerRemediations[rc.Code]
	if !ok {
		return blockerRemediation{}, false
	}
	if ownerAware, ok := planDimensionOwnerAwareRemediation(rc, remediation); ok {
		return ownerAware, true
	}
	if isArchivedActiveResidueReason(rc) {
		return blockerRemediation{
			Remediation:     "Active-state residue for archived change {subject} remains under artifacts/changes/{subject}. Remove only that stale active-state residue with `slipway delete --change {subject}`. The archived record and source commits are not deletion targets.",
			CommandTemplate: "slipway delete --change {subject}",
			Class:           RecoveryClassDiscardChange,
		}, true
	}
	return remediation, true
}

func planDimensionOwnerAwareRemediation(rc ReasonCode, base blockerRemediation) (blockerRemediation, bool) {
	owner, _ := splitSubjectDetail(rc.Detail)
	if owner != "plan-audit" && owner != "spec-compliance-review" {
		return blockerRemediation{}, false
	}

	owned := base
	owned.SplitDetail = true
	switch rc.Code {
	case "plan_dimension_decision_soundness_unattested":
		owned.Remediation = "Re-run {subject} and record a passing decision_soundness attestation reference."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	case "plan_dimension_consistency_unattested":
		owned.Remediation = "Re-run {subject} and record a passing consistency attestation reference."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	case "plan_dimension_attestation_invalid":
		owned.Remediation = "Fix malformed dim:<name>=<verdict>:<evidence-ref> evidence references and re-record {subject} evidence."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	case "plan_dimension_attestation_conflict":
		owned.Remediation = "Remove conflicting plan-dimension verdict references and re-record a single {subject} verdict for the dimension."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	case "plan_dimension_attestation_evidence_unresolvable":
		owned.Remediation = "Change the plan-dimension evidence reference to a resolvable workspace-relative path and re-record {subject} evidence."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	case "plan_dimension_decision_soundness_evidence_invalid":
		owned.Remediation = "Re-run the decision-soundness audit with codebase-grounded evidence outside artifacts/ and re-record {subject} evidence."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	case "plan_dimension_decision_unsound":
		owned.Remediation = "Repair the current plan artifacts in place to address the unsound decision, then rerun {subject}."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	case "plan_dimension_consistency_failed":
		owned.Remediation = "Repair inconsistent plan artifacts in place, then rerun {subject}."
		owned.CommandTemplate = "slipway evidence skill --skill {subject} --verdict pass"
	default:
		return blockerRemediation{}, false
	}
	return owned, true
}

// ArchivedActiveResidueMessagePrefix is the shared sentinel that marks an
// orphaned_change_bundle reason as the archived-same-slug active-residue variant
// (a stale active bundle whose authority moved to the archive). The message
// producer in cmd/common.go builds its Message with this exact prefix, and
// isArchivedActiveResidueReason matches on it, so the two prose strings cannot
// silently drift across the package boundary.
const ArchivedActiveResidueMessagePrefix = "Active-state residue for archived change "

func isArchivedActiveResidueReason(rc ReasonCode) bool {
	return rc.Code == "orphaned_change_bundle" &&
		strings.Contains(rc.Message, ArchivedActiveResidueMessagePrefix)
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
// a repair command is never emitted with an empty placeholder.
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

func lessRecoveryStep(a, b RecoveryStep) bool {
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
