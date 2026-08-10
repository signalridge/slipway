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

type journalRecordLimitError interface {
	error
	JournalRecordContext() string
	JournalRecordSize() int
	JournalRecordLimit() int
}

// committedOutputError means the Run mutation returned successfully, but the
// response could not be derived or written. The durable Run is the recovery
// authority; callers must inspect it instead of retrying the mutation blindly.
type committedOutputError struct {
	err error
}

func (err *committedOutputError) Error() string {
	if err == nil || err.err == nil {
		return "mutation committed, but its response could not be emitted; do not retry blindly"
	}
	return "mutation committed, but its response could not be emitted; do not retry blindly: " + err.err.Error()
}

func (err *committedOutputError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.err
}

type CLIError struct {
	ContractVersion int            `json:"contract_version"`
	Code            string         `json:"code"`
	Message         string         `json:"message"`
	Next            autopilot.Next `json:"next"`
	ExitCode        int            `json:"exit_code"`
	Details         map[string]any `json:"details,omitempty"`
}

type cliErrorContext struct {
	WorkspaceRoot string
	RunID         string
}

type cliErrorContextCarrier struct {
	err     error
	context cliErrorContext
}

func (carrier *cliErrorContextCarrier) Error() string { return carrier.err.Error() }
func (carrier *cliErrorContextCarrier) Unwrap() error { return carrier.err }

func withCLIErrorContext(err error, workspaceRoot, runID string) error {
	if err == nil {
		return nil
	}
	return &cliErrorContextCarrier{
		err: err,
		context: cliErrorContext{
			WorkspaceRoot: workspaceRoot,
			RunID:         runID,
		},
	}
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
	return asCLIErrorWithContext(err, cliErrorContext{})
}

func asCLIErrorWithContext(err error, context cliErrorContext) *CLIError {
	if err == nil {
		return nil
	}
	var carrier *cliErrorContextCarrier
	if errors.As(err, &carrier) {
		if strings.TrimSpace(carrier.context.WorkspaceRoot) != "" {
			context.WorkspaceRoot = carrier.context.WorkspaceRoot
		}
		if strings.TrimSpace(carrier.context.RunID) != "" {
			context.RunID = carrier.context.RunID
		}
	}
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr
	}
	var protocolErr *autopilot.ProtocolError
	if errors.As(err, &protocolErr) {
		return newRuntimeError(protocolErr.Code, protocolErr.Message, protocolErr.Next, protocolErr.Details)
	}
	var recordLimitErr journalRecordLimitError
	if errors.As(err, &recordLimitErr) {
		next := storageRecoveryNext(context)
		return newRuntimeError("journal_record_too_large", recordLimitErr.Error(), next, map[string]any{
			"context": recordLimitErr.JournalRecordContext(),
			"size":    recordLimitErr.JournalRecordSize(),
			"limit":   recordLimitErr.JournalRecordLimit(),
		})
	}
	var outputErr *committedOutputError
	if errors.As(err, &outputErr) {
		return newRuntimeError(
			"mutation_committed_output_failed",
			outputErr.Error(),
			storageRecoveryNext(context),
			map[string]any{
				"phase":              "response_output",
				"committed":          true,
				"projection_stale":   false,
				"namespace_detached": false,
				"ambiguous":          false,
			},
		)
	}
	if errors.Is(err, autopilot.ErrRunBusy) {
		details := map[string]any(nil)
		var mutationErr storageMutationError
		if errors.As(err, &mutationErr) {
			details = map[string]any{
				"phase":              mutationErr.StorageMutationPhase(),
				"committed":          mutationErr.StorageMutationCommitted(),
				"projection_stale":   mutationErr.StorageMutationProjectionStale(),
				"namespace_detached": mutationErr.StorageMutationNamespaceDetached(),
				"ambiguous":          mutationErr.StorageMutationAmbiguous(),
			}
		}
		return newRuntimeError("run_busy", err.Error(), storageRecoveryNext(context), details)
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
		next := defaultErrorNext()
		if committed || projectionStale || namespaceDetached || ambiguous {
			next = storageRecoveryNext(context)
		}
		return newRuntimeError(code, mutationErr.Error(), next, map[string]any{
			"phase":              mutationErr.StorageMutationPhase(),
			"committed":          committed,
			"projection_stale":   projectionStale,
			"namespace_detached": namespaceDetached,
			"ambiguous":          ambiguous,
		})
	}
	message := strings.TrimSpace(err.Error())
	next := defaultErrorNext()
	if strings.TrimSpace(context.RunID) != "" {
		next = storageRecoveryNext(context)
	}
	return newRuntimeError("runtime_error", message, next, nil)
}

func storageRecoveryNext(context cliErrorContext) autopilot.Next {
	workspaceRoot := context.WorkspaceRoot
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = defaultErrorNext().WorkspaceRoot()
	}
	return statusInspectionNext(workspaceRoot, context.RunID)
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

func validateRunIDArgument(rawRoot, runID string) *CLIError {
	if err := autopilot.ValidateRunID(runID); err != nil {
		return newUsageError(
			"invalid_run_id",
			err.Error(),
			statusInspectionNextForRawRoot(rawRoot, ""),
		)
	}
	return nil
}

func statusInspectionNext(workspaceRoot, runID string) autopilot.Next {
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = defaultErrorNext().WorkspaceRoot()
	}
	if resolved, err := resolveRoot(workspaceRoot); err == nil {
		workspaceRoot = resolved
	} else if absolute, absoluteErr := filepath.Abs(workspaceRoot); absoluteErr == nil {
		workspaceRoot = absolute
	}
	argv := []string{"slipway", "status"}
	variantID := "inspect-runs"
	if strings.TrimSpace(runID) != "" {
		argv = append(argv, runID)
		variantID = "inspect-run"
	}
	argv = append(argv, "--root", workspaceRoot)
	next, err := autopilot.NewCommandNext(
		autopilot.NextOperationCommand,
		workspaceRoot,
		variantID,
		argv,
		nil,
	)
	if err != nil {
		return autopilot.NoneNext(workspaceRoot)
	}
	return next
}

func statusInspectionNextForRawRoot(rawRoot, runID string) autopilot.Next {
	workspaceRoot := rawRoot
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = defaultErrorNext().WorkspaceRoot()
	}
	if resolved, err := resolveRoot(workspaceRoot); err == nil {
		workspaceRoot = resolved
	} else if absolute, absoluteErr := filepath.Abs(workspaceRoot); absoluteErr == nil {
		workspaceRoot = absolute
	}
	return statusInspectionNext(workspaceRoot, runID)
}
