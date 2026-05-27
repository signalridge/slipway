package bootstrap

import "strings"

// InitUsageError carries a structured invalid-usage contract from bootstrap to
// the CLI layer without introducing a package cycle on cmd.CLIError.
type InitUsageError struct {
	ErrorCode   string
	Message     string
	Remediation string
	Details     map[string]any
}

func (e *InitUsageError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func newInitUsageError(errorCode, message, remediation string, details map[string]any) *InitUsageError {
	return &InitUsageError{
		ErrorCode:   strings.TrimSpace(errorCode),
		Message:     strings.TrimSpace(message),
		Remediation: strings.TrimSpace(remediation),
		Details:     details,
	}
}
