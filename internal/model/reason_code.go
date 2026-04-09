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
	"high_risk_check_failed": {
		Severity: ReasonSeverityError,
		Message:  "A required high-risk safety check failed",
	},
	"high_risk_check_missing": {
		Severity: ReasonSeverityError,
		Message:  "A required high-risk safety check is missing",
	},
	"invalid_pivot_kind": {
		Severity: ReasonSeverityError,
		Message:  "The requested pivot kind is invalid",
	},
	"manifest_r0_invalid": {
		Severity: ReasonSeverityError,
		Message:  "The governed change manifest failed R0 validation",
	},
	"missing_discovery_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Required discovery evidence is missing",
	},
	"missing_worktree_branch": {
		Severity: ReasonSeverityError,
		Message:  "The change is missing a bound worktree branch",
	},
	"missing_worktree_path": {
		Severity: ReasonSeverityError,
		Message:  "The change is missing a bound worktree path",
	},
	"pivot_not_approved": {
		Severity: ReasonSeverityError,
		Message:  "The requested pivot is not approved",
	},
	"plan_audit_failed": {
		Severity: ReasonSeverityError,
		Message:  "Plan audit did not pass",
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
	"rescope_requires_s2_state": {
		Severity: ReasonSeverityError,
		Message:  "Rescope pivots are only allowed from S2_EXECUTE",
	},
	"stale_execution_evidence": {
		Severity: ReasonSeverityError,
		Message:  "Execution evidence is stale; rerun wave-orchestration to refresh execution-summary.yaml",
	},
	"tasks_checklist_invalid_format": {
		Severity: ReasonSeverityError,
		Message:  "The governed tasks checklist format is invalid",
	},
	"verification_evidence_missing": {
		Severity: ReasonSeverityError,
		Message:  "Required verification evidence is missing",
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
