package model

import "fmt"

// Scores stores raw N/A/I/R/V dimensions. Derived values are computed on read.
type Scores struct {
	Novelty           int `yaml:"novelty" json:"novelty"`
	Ambiguity         int `yaml:"ambiguity" json:"ambiguity"`
	Impact            int `yaml:"impact" json:"impact"`
	Risk              int `yaml:"risk" json:"risk"`
	ReversibilityCost int `yaml:"reversibility_cost" json:"reversibility_cost"`
}

func (s Scores) Validate() error {
	if err := validateScoreDimension("novelty", s.Novelty); err != nil {
		return err
	}
	if err := validateScoreDimension("ambiguity", s.Ambiguity); err != nil {
		return err
	}
	if err := validateScoreDimension("impact", s.Impact); err != nil {
		return err
	}
	if err := validateScoreDimension("risk", s.Risk); err != nil {
		return err
	}
	if err := validateScoreDimension("reversibility_cost", s.ReversibilityCost); err != nil {
		return err
	}
	return nil
}

func (s Scores) DiscoveryScore() int {
	return s.Novelty + s.Ambiguity
}

func (s Scores) ControlScore() int {
	return s.Impact + s.Risk + s.ReversibilityCost
}

func validateScoreDimension(name string, value int) error {
	if value < 0 || value > 4 {
		return fmt.Errorf("%s must be in range 0..4: %d", name, value)
	}
	return nil
}
