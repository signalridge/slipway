package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/autopilot"
)

const (
	exitCodeUsage   = 2
	exitCodeRuntime = 3
)

// CLIError is the stable machine-readable error shape. Next contains typed
// argv variants; rendered shell text is never machine authority.
type storageMutationError interface {
	error
	StorageMutationPhase() string
	StorageMutationCommitted() bool
	StorageMutationProjectionStale() bool
	StorageMutationNamespaceDetached() bool
	StorageMutationAmbiguous() bool
}

type CLIError struct {
	ContractVersion int            `json:"contract_version"`
	Code            string         `json:"code"`
	Message         string         `json:"message"`
	Next            autopilot.Next `json:"next"`
	ExitCode        int            `json:"exit_code"`
	Details         map[string]any `json:"details,omitempty"`
}

func (err *CLIError) Error() string {
	if err == nil {
		return ""
	}
	return err.Message
}

func newUsageError(code, message string, next autopilot.Next) *CLIError {
	return &CLIError{
		ContractVersion: autopilot.ContractVersion,
		Code:            strings.TrimSpace(code),
		Message:         strings.TrimSpace(message),
		Next:            normalizeErrorNext(next),
		ExitCode:        exitCodeUsage,
	}
}

func newRuntimeError(code, message string, next autopilot.Next, details map[string]any) *CLIError {
	return &CLIError{
		ContractVersion: autopilot.ContractVersion,
		Code:            strings.TrimSpace(code),
		Message:         strings.TrimSpace(message),
		Next:            normalizeErrorNext(next),
		ExitCode:        exitCodeRuntime,
		Details:         details,
	}
}

func asCLIError(err error) *CLIError {
	if err == nil {
		return nil
	}
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr
	}
	var protocolErr *autopilot.ProtocolError
	if errors.As(err, &protocolErr) {
		return newRuntimeError(protocolErr.Code, protocolErr.Message, protocolErr.Next, protocolErr.Details)
	}
	var mutationErr storageMutationError
	if errors.As(err, &mutationErr) {
		committed := mutationErr.StorageMutationCommitted()
		projectionStale := mutationErr.StorageMutationProjectionStale()
		namespaceDetached := mutationErr.StorageMutationNamespaceDetached()
		ambiguous := mutationErr.StorageMutationAmbiguous()
		code := "mutation_not_committed"
		switch {
		case namespaceDetached || ambiguous:
			code = "mutation_outcome_ambiguous"
		case committed && projectionStale:
			code = "mutation_committed_projection_stale"
		case committed:
			code = "mutation_committed_verification_failed"
		}
		return newRuntimeError(code, mutationErr.Error(), defaultErrorNext(), map[string]any{
			"phase":              mutationErr.StorageMutationPhase(),
			"committed":          committed,
			"projection_stale":   projectionStale,
			"namespace_detached": namespaceDetached,
			"ambiguous":          ambiguous,
		})
	}
	message := strings.TrimSpace(err.Error())
	next := defaultErrorNext()
	lower := strings.ToLower(message)
	if strings.Contains(lower, "unknown command") ||
		strings.Contains(lower, "unknown flag") ||
		strings.Contains(lower, "requires") ||
		strings.Contains(lower, "accepts") ||
		strings.Contains(lower, "required flag") {
		return newUsageError("invalid_usage", message, next)
	}
	return newRuntimeError("runtime_error", message, next, nil)
}

func normalizeErrorNext(next autopilot.Next) autopilot.Next {
	if err := next.Validate(); err == nil {
		return next
	}
	return defaultErrorNext()
}

func defaultErrorNext() autopilot.Next {
	workspace, err := os.Getwd()
	if err != nil {
		workspace = string(filepath.Separator)
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		workspace = string(filepath.Separator)
	}
	return autopilot.NoneNext(workspace)
}
