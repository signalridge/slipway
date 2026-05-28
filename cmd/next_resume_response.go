package cmd

import (
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

// validateResumeResponse checks a --resume-response value against the active checkpoint contract.
// If the checkpoint has allowed_responses, the response must match one of them.
// If the response is empty, it is rejected — non-interactive mode requires explicit text.
func validateResumeResponse(cp *model.ActiveCheckpoint, response string) error {
	response = strings.TrimSpace(response)
	if response == "" {
		return newInvalidUsageError(
			"resume_response_required",
			fmt.Sprintf(
				"active checkpoint requires --resume-response: paused_task=%s checkpoint_type=%s",
				cp.PausedTaskID,
				cp.CheckpointType,
			),
			"Provide --resume-response with the required value.",
			nil,
		)
	}

	invalidResponseError := func() error {
		allowedResponses := strings.Join(cp.AllowedResponses, ", ")
		return newInvalidUsageError(
			"resume_response_invalid",
			fmt.Sprintf(
				"--resume-response %q is not valid for checkpoint %s; allowed: %s",
				response,
				cp.PausedTaskID,
				allowedResponses,
			),
			fmt.Sprintf("Use one of: %s", allowedResponses),
			nil,
		)
	}

	kind := model.CheckpointKind(cp.CheckpointType)
	switch kind {
	case model.CheckpointDecision:
		if len(cp.AllowedResponses) == 0 {
			return newStateIntegrityError(
				"checkpoint_config_invalid",
				"checkpoint_type=decision requires allowed_responses but none configured",
				"Run `slipway repair` to repair local state integrity.",
				"",
				nil,
			)
		}
		for _, allowed := range cp.AllowedResponses {
			if strings.EqualFold(response, allowed) {
				return nil
			}
		}
		return invalidResponseError()
	case model.CheckpointHumanVerify, model.CheckpointHumanAction:
		return nil
	default:
		if len(cp.AllowedResponses) > 0 {
			for _, allowed := range cp.AllowedResponses {
				if strings.EqualFold(response, allowed) {
					return nil
				}
			}
			return invalidResponseError()
		}
		return nil
	}
}
