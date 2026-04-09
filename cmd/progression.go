package cmd

import (
	"errors"

	"github.com/signalridge/slipway/internal/engine/progression"
)

func tryAdvance(root string, change changeRef) (progression.AdvanceSummary, error) {
	summary, err := progression.Advance(root, change.Slug)
	if err != nil {
		return progression.AdvanceSummary{}, mapProgressionError(err, change.Slug)
	}
	return summary, nil
}

// mapProgressionError maps progression engine errors to CLIError.
func mapProgressionError(err error, slug string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, progression.ErrGateBlocked):
		return newGovernanceBlockedError(
			"gate_blocked",
			err.Error(),
			"Resolve blockers and rerun the command.",
			slug,
			nil,
		)
	case errors.Is(err, progression.ErrNoNextState):
		return newStateIntegrityError(
			"no_next_state",
			err.Error(),
			"Run `slipway repair` to repair local state integrity.",
			slug,
			nil,
		)
	default:
		return err
	}
}
