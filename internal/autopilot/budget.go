package autopilot

import "fmt"

const DefaultBudget = 8

const maxBudget = 1000

const minimumResumeBudget = 3

func ValidateBudget(budget int) error {
	if budget < 1 {
		return fmt.Errorf("budget must be at least 1")
	}
	if budget > maxBudget {
		return fmt.Errorf("budget cannot exceed %d", maxBudget)
	}
	return nil
}

// ConsumeBudget accounts for one newly issued Action.
func ConsumeBudget(remaining int) (int, error) {
	if remaining < 1 {
		return remaining, fmt.Errorf("action budget exhausted")
	}
	return remaining - 1, nil
}
