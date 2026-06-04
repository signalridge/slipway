package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const EvidenceDigestsVersion = 1

type EvidenceDigests struct {
	Version int                    `yaml:"version" json:"version"`
	Skills  map[string]SkillDigest `yaml:"skills" json:"skills"`
}

type SkillDigest struct {
	RunVersion       int               `yaml:"run_version" json:"run_version"`
	VerdictTimestamp time.Time         `yaml:"verdict_timestamp,omitempty" json:"verdict_timestamp,omitempty"`
	Inputs           map[string]string `yaml:"inputs" json:"inputs"`
}

func (d *EvidenceDigests) Normalize() {
	if d.Version == 0 {
		d.Version = EvidenceDigestsVersion
	}
	if d.Skills == nil {
		d.Skills = map[string]SkillDigest{}
	}
	normalized := make(map[string]SkillDigest, len(d.Skills))
	for skillName, digest := range d.Skills {
		trimmed := strings.TrimSpace(skillName)
		if trimmed == "" {
			trimmed = skillName
		}
		digest.Normalize()
		normalized[trimmed] = digest
	}
	d.Skills = normalized
}

func (d EvidenceDigests) Validate() error {
	var errs []string
	if d.Version != EvidenceDigestsVersion {
		errs = append(errs, fmt.Sprintf("version must be %d", EvidenceDigestsVersion))
	}
	for skillName, digest := range d.Skills {
		if strings.TrimSpace(skillName) == "" {
			errs = append(errs, "skill name is required")
			continue
		}
		if err := digest.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("skills[%s]: %v", skillName, err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (d *SkillDigest) Normalize() {
	if !d.VerdictTimestamp.IsZero() {
		d.VerdictTimestamp = d.VerdictTimestamp.UTC()
	}
	if d.Inputs == nil {
		d.Inputs = map[string]string{}
	}
	normalized := make(map[string]string, len(d.Inputs))
	for name, digest := range d.Inputs {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			trimmedName = name
		}
		normalized[trimmedName] = strings.TrimSpace(digest)
	}
	d.Inputs = normalized
}

func (d SkillDigest) Validate() error {
	var errs []string
	if d.RunVersion < 0 {
		errs = append(errs, "run_version must be >= 0")
	}
	for name, digest := range d.Inputs {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, "input name is required")
		}
		if strings.TrimSpace(digest) == "" {
			errs = append(errs, fmt.Sprintf("input digest is required for %s", name))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func EvidenceFreshness(stored SkillDigest, currentInputs map[string]string) (bool, []string) {
	stored.Normalize()
	current := SkillDigest{Inputs: currentInputs}
	current.Normalize()

	changedSet := map[string]struct{}{}
	for name, storedDigest := range stored.Inputs {
		if current.Inputs[name] != storedDigest {
			changedSet[name] = struct{}{}
		}
	}
	for name := range current.Inputs {
		if _, ok := stored.Inputs[name]; !ok {
			changedSet[name] = struct{}{}
		}
	}
	if len(changedSet) == 0 {
		return true, nil
	}
	changed := make([]string, 0, len(changedSet))
	for name := range changedSet {
		changed = append(changed, name)
	}
	slices.Sort(changed)
	return false, changed
}
