package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalReasonCodeTaxonomySnapshot(t *testing.T) {
	t.Parallel()

	got := make([]string, 0, len(canonicalReasonDefinitions))
	for code := range canonicalReasonDefinitions {
		got = append(got, code)
	}
	sort.Strings(got)
	wantCodes := canonicalReasonCodeSnapshot()
	wantSeverities := canonicalReasonSeveritySnapshot(wantCodes)
	require.Len(t, wantSeverities, len(wantCodes))
	for code, definition := range canonicalReasonDefinitions {
		assert.Truef(t, definition.Severity.IsValid(), "%s must define a valid severity", code)
		assert.NotEmptyf(t, strings.TrimSpace(definition.Message), "%s must define canonical message prose", code)
		wantSeverity, ok := wantSeverities[code]
		require.Truef(t, ok, "%s must have frozen severity", code)
		assert.Equalf(t, wantSeverity, definition.Severity, "%s severity changed", code)
	}

	assert.Equal(t, wantCodes, got)
}

func canonicalReasonCodeSnapshot() []string {
	return []string{
		"archive_failed",
		"archived_lifecycle_event_scan_failed",
		"artifact_not_ready",
		"artifact_reconcile_failed",
		"artifact_schema_missing",
		"artifact_validation_failed",
		"assurance_contract_missing",
		"assurance_contract_path_invalid",
		"assurance_contract_unreadable",
		"assurance_section_placeholder",
		"assurance_structure_invalid",
		"change_bundle_unreadable",
		"change_is_done",
		"change_not_active",
		"checkpoint_stale",
		"checkpoint_task_missing_from_wave_plan",
		"checkpoint_wave_index_drift",
		"closeout_assurance_attestation_missing",
		"closeout_chain_order_invalid",
		"closeout_goal_verification_reuse_invalid",
		"closeout_reviewer_independence_missing",
		"codebase_map_freshness_missing",
		"codebase_map_freshness_partial",
		"codebase_map_freshness_scaffold_only",
		"codebase_map_freshness_stale",
		"codebase_map_freshness_unknown",
		"config_parse_failure",
		"context_origin_handle_invalid",
		"cross_stage_context_not_distinct",
		"decision_contract_path_invalid",
		"decision_contract_unreadable",
		"decision_section_placeholder",
		"decision_status_rejected",
		"decision_status_unknown",
		"decision_structure_invalid",
		"dedicated_worktree_branch_mismatch",
		"dedicated_worktree_metadata_required",
		"dedicated_worktree_path_invalid",
		"dedicated_worktree_required",
		"degraded_dispatch_justification_missing",
		"dispatch_mode_absent_on_started_parallel_wave",
		"execution_interrupted",
		"execution_summary_unreadable",
		"execution_verdict_fail",
		"executor_agent_missing",
		"governance_action_required",
		"governed_bundle_path_invalid",
		"high_risk_check_failed",
		"high_risk_check_missing",
		"incomplete_execution_task",
		"intake_clarification_incomplete",
		"intake_confirmation_incomplete",
		"intake_substep_invalid",
		"intent_drift",
		"invalid_blocker",
		"invalid_pivot_kind",
		"lifecycle_event_log_unreadable",
		"lifecycle_event_scan_failed",
		"lifecycle_event_scan_skipped",
		"lifecycle_event_write_failed",
		"list_changes_failed",
		"load_change_failed",
		"manifest_r0_invalid",
		"missing_discovery_evidence",
		"missing_required_artifact",
		"missing_run_summary",
		"missing_task_evidence_for_run_summary",
		"missing_worktree_branch",
		"missing_worktree_path",
		"multiple_active_changes",
		"no_skill_required",
		"non_pass_task",
		"non_pass_wave",
		"not_done_ready",
		"orphan_bundle_directory",
		"orphan_task_evidence",
		"orphaned_change_bundle",
		"parallel_wave_changed_file_overlap",
		"pivot_not_approved",
		"pivot_required",
		"pivot_state_invalid",
		"plan_audit_budget_exhausted",
		"plan_audit_evidence_missing",
		"plan_audit_failed",
		"plan_audit_iteration",
		"plan_audit_origin_invalid",
		"plan_audit_stalled",
		"plan_checker_feedback_required",
		"plan_checker_loop_terminated",
		"plan_dimension_completeness_missing_objective",
		"plan_dimension_coverage_missing_requirement",
		"plan_dimension_coverage_requirement_id_missing",
		"plan_dimension_coverage_requirements_invalid",
		"plan_dimension_coverage_spec_unreadable",
		"plan_dimension_coverage_unknown_requirement",
		"plan_dimension_dependency_cycle_detected",
		"plan_dimension_dependency_self_reference",
		"plan_dimension_dependency_unknown",
		"plan_dimension_execution_invalid_wave_plan",
		"plan_dimension_key_links_missing_target_files",
		"plan_dimension_scope_invalid_target",
		"plan_dimension_scope_out_of_bounds_target",
		"preset_confirmation_required",
		"required_artifact_dependency_missing",
		"required_artifact_schema_missing",
		"required_artifact_unreadable",
		"required_skill_blockers_present",
		"required_skill_missing",
		"required_skill_not_passed",
		"required_skill_not_ready",
		"required_skill_stale",
		"rescope_state_invalid",
		"research_section_placeholder",
		"research_structure_invalid",
		"review_layer_failed",
		"review_layer_missing",
		"run_slipway_done_to_finalize",
		"run_slipway_run_to_advance",
		"scope_contract_changed_files_missing",
		"scope_contract_drift",
		"scope_contract_evaluation_failed",
		"scope_contract_missing",
		"sensitive_evidence_missing",
		"session_isolation_warning",
		"ship_gate_blocked",
		"skill_prompt_surface_missing",
		"skill_prompt_surface_unreadable",
		"skill_registry_invalid",
		"stale_checkpoint_state",
		"stale_evidence_recovery_available",
		"stale_execution_evidence",
		"stale_planning_evidence",
		"stale_runtime_binding",
		"task",
		"task_blocker",
		"task_blockers",
		"task_blockers_invalid_key",
		"task_changed_file_scope_escape",
		"task_evidence_invalid",
		"task_evidence_unreadable",
		"tasks_checklist_duplicate_task_id",
		"tasks_checklist_empty",
		"tasks_checklist_invalid_format",
		"tasks_checklist_missing",
		"tasks_checklist_path_invalid",
		"tasks_checklist_task_id_missing",
		"tasks_checklist_unreadable",
		"tasks_plan_changed_since_task_evidence",
		"unknown_reason_code",
		"verification_evidence_missing",
		"wave_execution_blocked",
		"wave_execution_unavailable",
		"wave_orchestration_run_summary_version_invalid",
		"wave_orchestration_stale_task_evidence",
		"wave_plan_drift",
		"wave_plan_load_failed",
		"wave_plan_missing",
		"wave_plan_repair_blocked",
		"wave_plan_unreadable",
		"wave_run_missing",
		"wave_run_version_mismatch",
		"wave_runs_incomplete",
		"wave_runs_invalid_count",
		"wave_runs_load_failed",
		"wave_runs_missing",
		"wave_runs_unreadable",
		"wave_task_linkage_mismatch",
		"wave_test_impl_not_distinct",
		"workspace_scope_config_missing",
		"workspace_scope_marker_missing",
		"worktree_metadata_persist_failed",
		"worktree_validation_error",
	}
}

func canonicalReasonSeveritySnapshot(codes []string) map[string]ReasonSeverity {
	severities := make(map[string]ReasonSeverity, len(codes))
	for _, code := range codes {
		severities[code] = ReasonSeverityError
	}
	for code, severity := range map[string]ReasonSeverity{
		"change_is_done":                       ReasonSeverityInfo,
		"checkpoint_stale":                     ReasonSeverityWarning,
		"codebase_map_freshness_partial":       ReasonSeverityWarning,
		"codebase_map_freshness_scaffold_only": ReasonSeverityWarning,
		"codebase_map_freshness_stale":         ReasonSeverityWarning,
		"codebase_map_freshness_unknown":       ReasonSeverityWarning,
		"execution_interrupted":                ReasonSeverityWarning,
		"lifecycle_event_scan_skipped":         ReasonSeverityWarning,
		"no_skill_required":                    ReasonSeverityInfo,
		"run_slipway_done_to_finalize":         ReasonSeverityWarning,
		"run_slipway_run_to_advance":           ReasonSeverityWarning,
		"session_isolation_warning":            ReasonSeverityWarning,
		"stale_checkpoint_state":               ReasonSeverityWarning,
		"stale_evidence_recovery_available":    ReasonSeverityWarning,
	} {
		severities[code] = severity
	}
	return severities
}

// The hand-declared `wave:` task metadata is retired — the engine computes
// execution waves from depends_on + target_files — so validation no longer
// demands a declared wave and the blocker vocabulary for it must be fully
// removed: no canonical reason definition, no remediation vocabulary entry,
// and no public recognition as a canonical code. The retired code is kept as
// a string literal on purpose: any Go identifier for it is deleted together
// with the vocabulary, and this contract must keep compiling after that.
func TestDeclaredWaveBlockerVocabularyRetired(t *testing.T) {
	t.Parallel()

	const retired = "plan_dimension_execution_missing_wave"

	_, inRegistry := canonicalReasonDefinitions[retired]
	assert.Falsef(t, inRegistry,
		"the canonical reason registry must not define the retired declared-wave blocker %q", retired)

	_, inRemediations := blockerRemediations[retired]
	assert.Falsef(t, inRemediations,
		"the remediation vocabulary must not define the retired declared-wave blocker %q", retired)

	assert.Falsef(t, IsCanonicalReasonCode(retired),
		"%q must no longer be recognized as a canonical reason code", retired)
}

// The single-stage `review_origin_handle_invalid` blocker is retired in favor
// of the lattice-wide `context_origin_handle_invalid` code (which covers every
// stage's context-origin attestation, not just the review pair). The old code
// must be fully removed: no canonical reason definition, no remediation
// vocabulary entry, and no public recognition as a canonical code, so nothing
// downgrades it to `unknown_reason_code` by silently treating it as live.
func TestReviewOriginHandleVocabularyRetired(t *testing.T) {
	t.Parallel()

	const retired = "review_origin_handle_invalid"

	_, inRegistry := canonicalReasonDefinitions[retired]
	assert.Falsef(t, inRegistry,
		"the canonical reason registry must not define the retired review-origin blocker %q", retired)

	_, inRemediations := blockerRemediations[retired]
	assert.Falsef(t, inRemediations,
		"the remediation vocabulary must not define the retired review-origin blocker %q", retired)

	assert.Falsef(t, IsCanonicalReasonCode(retired),
		"%q must no longer be recognized as a canonical reason code", retired)
}

func TestNewReasonCodeMakesUnknownCodeExplicit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		code       string
		detail     string
		wantDetail string
	}{
		{
			name:       "normalized token",
			code:       "new_unreviewed_code",
			detail:     "source-path",
			wantDetail: "new_unreviewed_code: source-path",
		},
		{
			name:       "raw token with noncanonical separator",
			code:       "foo/bar",
			detail:     "source-path",
			wantDetail: "foo/bar: source-path",
		},
		{
			name:       "empty token",
			code:       "",
			detail:     "",
			wantDetail: "empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reason := NewReasonCode(tt.code, tt.detail)

			assert.Equal(t, "unknown_reason_code", reason.Code)
			assert.Equal(t, tt.wantDetail, reason.Detail)
			assert.Equal(t, ReasonSeverityError, reason.Severity)
		})
	}
}

func TestReasonCodeNormalizeMakesUnknownCodeExplicit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		code       string
		wantDetail string
	}{
		{
			name:       "normalized token",
			code:       "new_unreviewed_code",
			wantDetail: "new_unreviewed_code: source-path",
		},
		{
			name:       "raw token with noncanonical separator",
			code:       "foo/bar",
			wantDetail: "foo/bar: source-path",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reason := ReasonCode{
				Code:     tt.code,
				Severity: ReasonSeverityWarning,
				Message:  "looks valid",
				Detail:   "source-path",
			}
			reason.Normalize()

			assert.Equal(t, "unknown_reason_code", reason.Code)
			assert.Equal(t, tt.wantDetail, reason.Detail)
			assert.Equal(t, ReasonSeverityError, reason.Severity)
		})
	}
}

func testHumanizeReasonCode(code string) string {
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

func TestReasonAndErrorContractTestsDoNotTextMatchMessageProse(t *testing.T) {
	t.Parallel()

	for _, rel := range reasonAndErrorContractLintFiles(t) {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			t.Parallel()

			root := repositoryRootForReasonCodeContractTest(t)
			path := filepath.Join(root, filepath.FromSlash(rel))
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			require.NoError(t, err)

			violations := messageProseAssertionViolations(fset, file)

			assert.Empty(t, violations,
				"assert stable Code/Detail/ErrorCode/Category/ExitCode/Details fields instead of Message prose")
		})
	}
}

func TestMessageProseAssertionLintDetectsBypassShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "seeded roots and inline receivers",
			src: `package model

func TestLintSample(t *testing.T) {
	got := NewReasonCode("required_skill_missing", "goal-verification")
	assert.Equal(t, "prose", got.Message)
	assert.Contains(t, NewReasonCode("required_skill_missing", "goal-verification").Message, "prose")
	assert.Equal(t, "prose", testReason().Message)

	var finding HealthFinding
	for _, reason := range finding.Reasons {
		_ = strings.Contains(reason.Message, "prose")
	}
}
`,
			want: 4,
		},
		{
			name: "inline receivers without seeded roots",
			src: `package model

func TestLintSample(t *testing.T) {
	assert.Contains(t, NewReasonCode("required_skill_missing", "goal-verification").Message, "prose")
	assert.Equal(t, "prose", testReason().Message)
}
`,
			want: 2,
		},
		{
			name: "json error payload message map index",
			src: `package model

func TestLintSample(t *testing.T) {
	payload := decodeJSONMap(t, stderr)
	assert.Contains(t, payload["message"], "prose")
	assert.Equal(t, "prose", payload["message"])
	_ = strings.Contains(payload["message"].(string), "prose")
	assert.Contains(t, payload["mess\u0061ge"], "prose")
}
`,
			want: 4,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "lint_sample_test.go", tt.src, 0)
			require.NoError(t, err)

			violations := messageProseAssertionViolations(fset, file)

			require.Len(t, violations, tt.want)
			for _, violation := range violations {
				assert.Contains(t, violation, "lint_sample_test.go")
			}
		})
	}
}

func TestMessageProseAssertionLintScopesRootsPerFunction(t *testing.T) {
	t.Parallel()

	source := `package model

func TestReasonMessage(t *testing.T) {
	reason := NewReasonCode("required_skill_missing", "goal-verification")
	assert.Contains(t, reason.Message, "prose")
}

func TestUnrelatedMessage(t *testing.T) {
	reason := buildSummary()
	assert.Contains(t, reason.Message, "prose")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "lint_sample_test.go", source, 0)
	require.NoError(t, err)

	violations := messageProseAssertionViolations(fset, file)

	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "lint_sample_test.go")
}

func reasonAndErrorContractLintFiles(t *testing.T) []string {
	t.Helper()

	root := repositoryRootForReasonCodeContractTest(t)
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipReasonContractLintDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(files)
	return files
}

func shouldSkipReasonContractLintDir(name string) bool {
	switch name {
	case ".git", ".worktrees", "vendor":
		return true
	default:
		return false
	}
}

func messageProseAssertionViolations(fset *token.FileSet, file *ast.File) []string {
	var violations []string
	ast.Inspect(file, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		contractRoots := contractMessageRootNames(fn.Body)
		violations = append(violations, messageProseAssertionViolationsInNode(fset, fn.Body, contractRoots)...)
		return false
	})
	return violations
}

// messageProseAssertionViolationsInNode is a lightweight AST lint for
// syntactically recognizable reason/error payloads. It is not a typed whole-repo
// ban on every field named Message; extend the isContractMessage* helpers when
// new reason/error payload surfaces need enforcement.
func messageProseAssertionViolationsInNode(fset *token.FileSet, node ast.Node, contractRoots map[string]struct{}) []string {
	var violations []string
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !isMessageTextAssertion(call) && !isMessageTextMatchCall(call) {
			return true
		}
		for _, arg := range call.Args {
			if containsContractMessageSelector(arg, contractRoots) {
				pos := fset.Position(arg.Pos())
				violations = append(violations, pos.String())
				break
			}
		}
		return true
	})
	return violations
}

func contractMessageRootNames(node ast.Node) map[string]struct{} {
	roots := map[string]struct{}{}
	ast.Inspect(node, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			if !isContractMessageType(typed.Type) {
				return true
			}
			for _, name := range typed.Names {
				roots[name.Name] = struct{}{}
			}
		case *ast.AssignStmt:
			for i, expr := range typed.Rhs {
				if i >= len(typed.Lhs) || !isContractMessageExpr(expr) {
					continue
				}
				if ident, ok := typed.Lhs[i].(*ast.Ident); ok {
					roots[ident.Name] = struct{}{}
				}
			}
		case *ast.RangeStmt:
			ident, ok := typed.Value.(*ast.Ident)
			if ok && isContractMessageCollection(typed.X) {
				roots[ident.Name] = struct{}{}
			}
		}
		return true
	})
	return roots
}

func repositoryRootForReasonCodeContractTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return root
}

func isMessageTextAssertion(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || (pkg.Name != "assert" && pkg.Name != "require") {
		return false
	}
	switch selector.Sel.Name {
	case "Contains", "Containsf", "NotContains", "NotContainsf",
		"Regexp", "Regexpf", "NotRegexp", "NotRegexpf",
		"ErrorContains", "Equal", "Equalf", "NotEqual", "NotEqualf":
		return true
	default:
		return false
	}
}

func isMessageTextMatchCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "strings" {
		return false
	}
	switch selector.Sel.Name {
	case "Contains", "HasPrefix", "HasSuffix", "EqualFold":
		return true
	default:
		return false
	}
}

func containsContractMessageSelector(expr ast.Expr, roots map[string]struct{}) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if found {
			return false
		}
		switch typed := node.(type) {
		case *ast.SelectorExpr:
			if typed.Sel.Name != "Message" {
				return true
			}
			if isContractMessageReceiver(typed.X, roots) {
				found = true
				return false
			}
		case *ast.IndexExpr:
			if isContractMessageMapIndex(typed, roots) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func isContractMessageMapIndex(expr *ast.IndexExpr, roots map[string]struct{}) bool {
	if expr == nil || !isMessageMapKey(expr.Index) {
		return false
	}
	return isContractMessageReceiver(expr.X, roots)
}

func isMessageMapKey(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(lit.Value)
	return err == nil && value == "message"
}

func isContractMessageReceiver(expr ast.Expr, roots map[string]struct{}) bool {
	root := rootIdentName(expr)
	if _, ok := roots[root]; ok {
		return true
	}
	return isContractMessageExpr(expr)
}

func rootIdentName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return rootIdentName(typed.X)
	case *ast.CallExpr:
		return rootIdentName(typed.Fun)
	case *ast.IndexExpr:
		return rootIdentName(typed.X)
	default:
		return ""
	}
}

func isContractMessageExpr(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.CallExpr:
		return isContractMessageConstructor(typed.Fun)
	case *ast.CompositeLit:
		return isContractMessageType(typed.Type)
	default:
		return false
	}
}

func isContractMessageConstructor(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.Ident:
		switch typed.Name {
		case "asCLIError", "decodeJSONMap", "NewReasonCode", "ReasonCodeFromSpec", "findGateReasonCode", "testReason":
			return true
		default:
			return false
		}
	case *ast.SelectorExpr:
		switch typed.Sel.Name {
		case "NewReasonCode", "ReasonCodeFromSpec":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isContractMessageType(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.Ident:
		switch typed.Name {
		case "CLIError", "ReasonCode", "HealthFinding":
			return true
		default:
			return false
		}
	case *ast.SelectorExpr:
		switch typed.Sel.Name {
		case "ReasonCode", "HealthFinding":
			return true
		default:
			return false
		}
	case *ast.StarExpr:
		return isContractMessageType(typed.X)
	default:
		return false
	}
}

func isContractMessageCollection(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch selector.Sel.Name {
	case "Blockers", "OpenBlockers", "ReasonCodes", "Reasons", "Findings":
		return true
	default:
		return false
	}
}
