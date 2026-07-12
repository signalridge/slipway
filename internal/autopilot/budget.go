package autopilot

import "fmt"

const DefaultBudget = 8

const minimumResumeBudget = 3

func ValidateBudget(budget int) error {
	if budget < 1 {
		return fmt.Errorf("budget must be at least 1")
	}
	if budget > 1000 {
		return fmt.Errorf("budget cannot exceed 1000")
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
