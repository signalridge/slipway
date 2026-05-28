package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// VerificationRecord is the simplified governance verification format.
// It replaces the 15-field EvidenceRecord with 4+2 fields:
// verdict, blockers, timestamp, run_version, plus optional References and Notes.
// Skill metadata (state, plan_substep,
// mitigation_target) is resolved from the skill registry at read time,
// not written by agents.
type VerificationRecord struct {
	Verdict    string       `yaml:"verdict" json:"verdict"`
	Blockers   []ReasonCode `yaml:"blockers" json:"blockers"`
	Timestamp  time.Time    `yaml:"timestamp" json:"timestamp"`
	RunVersion int          `yaml:"run_version" json:"run_version"`
	References []string     `yaml:"references,omitempty" json:"references,omitempty"`
	Notes      string       `yaml:"notes,omitempty" json:"notes,omitempty"`
}

const (
	VerificationVerdictPass = "pass"
	VerificationVerdictFail = "fail"
)

// IsPassing returns true if the verdict is "pass" and there are no blockers.
func (v VerificationRecord) IsPassing() bool {
	return v.Verdict == VerificationVerdictPass && len(v.Blockers) == 0
}

// Validate checks that all required fields are present and well-formed.
func (v VerificationRecord) Validate() error {
	var errs []string

	switch v.Verdict {
	case VerificationVerdictPass, VerificationVerdictFail:
		// ok
	case "":
		errs = append(errs, "verdict is required")
	default:
		errs = append(errs, fmt.Sprintf("verdict must be %q or %q, got %q",
			VerificationVerdictPass, VerificationVerdictFail, v.Verdict))
	}

	if v.Blockers == nil {
		errs = append(errs, "blockers must be present (use empty array [])")
	}

	if v.Timestamp.IsZero() {
		errs = append(errs, "timestamp is required")
	}

	if v.RunVersion < 0 {
		errs = append(errs, "run_version must be >= 0")
	}
	for i, blocker := range v.Blockers {
		if err := blocker.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("blockers[%d]: %v", i, err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Normalize ensures Blockers and References are non-nil slices.
func (v *VerificationRecord) Normalize() {
	if v.Blockers == nil {
		v.Blockers = []ReasonCode{}
	}
	if len(v.Blockers) == 0 {
		v.Blockers = []ReasonCode{}
	} else {
		v.Blockers = NormalizeReasonCodes(v.Blockers)
	}
	if v.References == nil {
		v.References = []string{}
	}
}
