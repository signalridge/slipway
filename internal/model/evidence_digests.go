package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const EvidenceDigestsVersion = 1
const SuiteResultVersion = 1

type EvidenceDigests struct {
	Version int                    `yaml:"version" json:"version"`
	Skills  map[string]SkillDigest `yaml:"skills" json:"skills"`
}

type SkillDigest struct {
	VerdictTimestamp time.Time         `yaml:"verdict_timestamp,omitempty" json:"verdict_timestamp,omitempty"`
	Inputs           map[string]string `yaml:"inputs" json:"inputs"`
	LegacyRunVersion int               `yaml:"run_version,omitempty" json:"-"`
}

type SuiteResult struct {
	Version           int               `yaml:"version" json:"version"`
	RunSummaryVersion int               `yaml:"run_summary_version" json:"run_summary_version"`
	FullSuiteDigest   string            `yaml:"full_suite_digest" json:"full_suite_digest"`
	SASTDigests       map[string]string `yaml:"sast_digests,omitempty" json:"sast_digests,omitempty"`
	CapturedAt        time.Time         `yaml:"captured_at,omitempty" json:"captured_at,omitempty"`
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

func (r *SuiteResult) Normalize() {
	if r.Version == 0 {
		r.Version = SuiteResultVersion
	}
	r.FullSuiteDigest = strings.TrimSpace(r.FullSuiteDigest)
	if !r.CapturedAt.IsZero() {
		r.CapturedAt = r.CapturedAt.Round(0).UTC()
	}
	if r.SASTDigests == nil {
		r.SASTDigests = map[string]string{}
	}
	keys := make([]string, 0, len(r.SASTDigests))
	for name := range r.SASTDigests {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	normalized := make(map[string]string, len(r.SASTDigests))
	for _, name := range keys {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			trimmedName = name
		}
		normalized[trimmedName] = strings.TrimSpace(r.SASTDigests[name])
	}
	r.SASTDigests = normalized
}

func (r SuiteResult) Validate() error {
	r.Normalize()
	var errs []string
	if r.Version != SuiteResultVersion {
		errs = append(errs, fmt.Sprintf("version must be %d", SuiteResultVersion))
	}
	if r.RunSummaryVersion < 1 {
		errs = append(errs, "run_summary_version must be >= 1")
	}
	if strings.TrimSpace(r.FullSuiteDigest) == "" {
		errs = append(errs, "full_suite_digest is required")
	}
	for name, digest := range r.SASTDigests {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, "sast digest name is required")
		}
		if strings.TrimSpace(digest) == "" {
			errs = append(errs, fmt.Sprintf("sast digest is required for %s", name))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (r SuiteResult) SharedReviewerInputDigests() (map[string]string, error) {
	r.Normalize()
	if err := r.Validate(); err != nil {
		return nil, err
	}
	runSummaryDigest, err := ComputeInputHash(map[string]any{
		"run_summary_version": r.RunSummaryVersion,
	})
	if err != nil {
		return nil, err
	}
	inputs := map[string]string{
		"suite-result:run_summary_version": runSummaryDigest,
		"suite-result:full_suite":          r.FullSuiteDigest,
	}
	for name, digest := range r.SASTDigests {
		inputs["suite-result:sast:"+name] = digest
	}
	return inputs, nil
}

func (d *SkillDigest) Normalize() {
	d.LegacyRunVersion = 0
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
