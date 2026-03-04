package cmd

import (
	"encoding/json"
	"io"
	"strings"
)

type FailureCategory string

const (
	categoryInvalidUsage      FailureCategory = "invalid_usage"
	categoryPrecondition      FailureCategory = "precondition_blocked"
	categoryStateIntegrity    FailureCategory = "state_integrity"
	categoryGovernanceBlocked FailureCategory = "governance_blocked"
	categoryRuntimeFailure    FailureCategory = "runtime_failure"
)

const (
	exitCodeSuccess           = 0
	exitCodeInvalidUsage      = 2
	exitCodePrecondition      = 3
	exitCodeStateIntegrity    = 4
	exitCodeGovernanceBlocked = 5
	exitCodeRuntimeFailure    = 6
)

type CLIError struct {
	ErrorCode   string          `json:"error_code"`
	Category    FailureCategory `json:"category"`
	Message     string          `json:"message"`
	Remediation string          `json:"remediation"`
	ExitCode    int             `json:"exit_code"`
	RequestID   string          `json:"request_id,omitempty"`
	Details     map[string]any  `json:"details,omitempty"`
}

func (e *CLIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func newCLIError(
	category FailureCategory,
	errorCode string,
	message string,
	remediation string,
	requestID string,
	details map[string]any,
) *CLIError {
	return &CLIError{
		ErrorCode:   strings.TrimSpace(errorCode),
		Category:    category,
		Message:     strings.TrimSpace(message),
		Remediation: strings.TrimSpace(remediation),
		ExitCode:    exitCodeForCategory(category),
		RequestID:   strings.TrimSpace(requestID),
		Details:     details,
	}
}

func emitCLIError(w io.Writer, err *CLIError) error {
	if err == nil {
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(err)
}

func exitCodeForCategory(category FailureCategory) int {
	switch category {
	case categoryInvalidUsage:
		return exitCodeInvalidUsage
	case categoryPrecondition:
		return exitCodePrecondition
	case categoryStateIntegrity:
		return exitCodeStateIntegrity
	case categoryGovernanceBlocked:
		return exitCodeGovernanceBlocked
	default:
		return exitCodeRuntimeFailure
	}
}

func asCLIError(err error) *CLIError {
	if err == nil {
		return nil
	}
	if typed, ok := err.(*CLIError); ok {
		return typed
	}

	msg := strings.TrimSpace(err.Error())
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "non_spln_intent"):
		return newCLIError(
			categoryPrecondition,
			"non_spln_intent",
			msg,
			"Provide an executable change request, or use normal chat flow for advisory questions.",
			"",
			nil,
		)
	case strings.Contains(lower, "no active request"):
		return newCLIError(
			categoryPrecondition,
			"no_active_request",
			msg,
			"Create an executable request with `spln new`.",
			"",
			nil,
		)
	case strings.Contains(lower, "ambiguous"):
		return newCLIError(
			categoryPrecondition,
			"active_context_ambiguous",
			msg,
			"Run `spln status` for diagnostics, then `spln repair` for repairable faults.",
			"",
			nil,
		)
	case strings.Contains(lower, "requires approved g_ship"),
		strings.Contains(lower, "pivot blocked"),
		strings.Contains(lower, "frozen run summary"),
		strings.Contains(lower, "not done-ready"),
		strings.Contains(lower, "requires governed s6_run_waves"):
		return newCLIError(
			categoryGovernanceBlocked,
			"governance_blocked",
			msg,
			"Resolve blockers and rerun the command.",
			"",
			nil,
		)
	case strings.Contains(lower, "invalid --"),
		strings.Contains(lower, "unknown flag"),
		strings.Contains(lower, "unsupported"),
		strings.Contains(lower, "not supported"),
		strings.Contains(lower, "expected reroute|rescope"),
		strings.Contains(lower, "is allowed only"):
		return newCLIError(
			categoryInvalidUsage,
			"invalid_usage",
			msg,
			"Review command help and invoke with supported flags and state.",
			"",
			nil,
		)
	case strings.Contains(lower, "config"),
		strings.Contains(lower, "yaml"),
		strings.Contains(lower, "state integrity"):
		return newCLIError(
			categoryStateIntegrity,
			"state_integrity",
			msg,
			"Run `spln repair` to repair local state/config integrity.",
			"",
			nil,
		)
	default:
		return newCLIError(
			categoryRuntimeFailure,
			"runtime_failure",
			msg,
			"Retry the command. If the issue persists, inspect runtime state and logs.",
			"",
			nil,
		)
	}
}
