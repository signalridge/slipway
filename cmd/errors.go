package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/model"
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
	exitCodeInvalidUsage      = 2
	exitCodePrecondition      = 3
	exitCodeStateIntegrity    = 4
	exitCodeGovernanceBlocked = 5
	exitCodeRuntimeFailure    = 6
)

// Execution limits.
const (
	// maxAutoNextIterations is the safety cap for governed run loops.
	maxAutoNextIterations = 20
	// defaultMaxRetriesPerSkill is the default retry budget per skill execution.
	defaultMaxRetriesPerSkill = 2
)

type CLIError struct {
	ErrorCode   string             `json:"error_code"`
	Category    FailureCategory    `json:"category"`
	Message     string             `json:"message"`
	Remediation string             `json:"remediation"`
	ExitCode    int                `json:"exit_code"`
	Slug        string             `json:"slug,omitempty"`
	Details     map[string]any     `json:"details,omitempty"`
	Reasons     []model.ReasonCode `json:"reasons,omitempty"`
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
	slug string,
	details map[string]any,
) *CLIError {
	return newCLIErrorWithReasons(category, errorCode, message, remediation, slug, nil, details)
}

func newCLIErrorWithReasons(
	category FailureCategory,
	errorCode string,
	message string,
	remediation string,
	slug string,
	reasons []model.ReasonCode,
	details map[string]any,
) *CLIError {
	return &CLIError{
		ErrorCode:   strings.TrimSpace(errorCode),
		Category:    category,
		Message:     strings.TrimSpace(message),
		Remediation: strings.TrimSpace(remediation),
		ExitCode:    exitCodeForCategory(category),
		Slug:        strings.TrimSpace(slug),
		Details:     details,
		Reasons:     model.NormalizeReasonCodes(reasons),
	}
}

func newInvalidUsageError(errorCode, message, remediation string, details map[string]any) *CLIError {
	return newCLIError(categoryInvalidUsage, errorCode, message, remediation, "", details)
}

func newPreconditionError(errorCode, message, remediation, slug string, details map[string]any) *CLIError {
	return newCLIError(categoryPrecondition, errorCode, message, remediation, slug, details)
}

func newStateIntegrityError(errorCode, message, remediation, slug string, details map[string]any) *CLIError {
	return newCLIError(categoryStateIntegrity, errorCode, message, remediation, slug, details)
}

func newGovernanceBlockedError(errorCode, message, remediation, slug string, details map[string]any) *CLIError {
	return newCLIError(categoryGovernanceBlocked, errorCode, message, remediation, slug, details)
}

func newGovernanceBlockedErrorWithReasons(
	errorCode,
	message,
	remediation,
	slug string,
	reasons []model.ReasonCode,
	details map[string]any,
) *CLIError {
	return newCLIErrorWithReasons(categoryGovernanceBlocked, errorCode, message, remediation, slug, reasons, details)
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
	var typed *CLIError
	if errors.As(err, &typed) {
		return typed
	}
	var initUsage *bootstrap.InitUsageError
	if errors.As(err, &initUsage) {
		return newInvalidUsageError(
			initUsage.ErrorCode,
			initUsage.Message,
			initUsage.Remediation,
			initUsage.Details,
		)
	}

	msg := strings.TrimSpace(err.Error())
	lower := strings.ToLower(msg)

	// Cobra-generated flag/arg errors that cannot be wrapped at source.
	if strings.HasPrefix(lower, "unknown flag") ||
		strings.HasPrefix(lower, "invalid argument") ||
		strings.HasPrefix(lower, "required flag") ||
		strings.HasPrefix(lower, "unknown command") ||
		strings.HasPrefix(lower, "unknown shorthand flag") {
		return newInvalidUsageError(
			"invalid_usage",
			msg,
			"Review command help and invoke with supported flags.",
			nil,
		)
	}

	return newCLIError(
		categoryRuntimeFailure,
		"runtime_failure",
		msg,
		"Retry the command. If the issue persists, inspect runtime state and logs.",
		"",
		nil,
	)
}
