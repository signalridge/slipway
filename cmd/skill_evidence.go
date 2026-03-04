package cmd

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/speclane/internal/engine/gate"
	"github.com/signalridge/speclane/internal/engine/skill"
	"github.com/signalridge/speclane/internal/model"
	"gopkg.in/yaml.v3"
)

type skillEvidenceFile struct {
	Path   string
	Record model.EvidenceRecord
}

type changeManifest struct {
	RequestID      string `yaml:"request_id"`
	Slug           string `yaml:"slug"`
	CreatedAtLevel string `yaml:"created_at_level"`
}

func evaluateRequiredSkills(
	root string,
	requestID string,
	level model.Level,
	workflowState model.WorkflowState,
	latestRunSummaryVersion int,
	closeoutRequired bool,
) (map[string]model.EvidenceRecord, []string, error) {
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return nil, nil, err
	}
	required := skill.RequiredSkillsForStateWithRegistry(
		registry,
		level,
		workflowState,
		false,
		closeoutRequired,
	)
	if len(required) == 0 {
		return map[string]model.EvidenceRecord{}, nil, nil
	}

	records, scanIssues, err := loadSkillEvidenceFiles(root, requestID)
	if err != nil {
		return nil, nil, err
	}

	bySkill := map[string][]skillEvidenceFile{}
	for _, record := range records {
		bySkill[record.Record.SkillName] = append(bySkill[record.Record.SkillName], record)
	}
	for key := range bySkill {
		slices.SortFunc(bySkill[key], func(a, b skillEvidenceFile) int {
			at := normalizedEvidenceTimestamp(a.Record)
			bt := normalizedEvidenceTimestamp(b.Record)
			if at.After(bt) {
				return -1
			}
			if bt.After(at) {
				return 1
			}
			if a.Path < b.Path {
				return -1
			}
			if a.Path > b.Path {
				return 1
			}
			return 0
		})
	}

	passing := map[string]model.EvidenceRecord{}
	blockers := append([]string{}, scanIssues...)
	baselineSessionID := latestImplementerBaselineSession(records, latestRunSummaryVersion)

	for _, skillName := range required {
		candidates := bySkill[skillName]
		if len(candidates) == 0 {
			blockers = append(blockers, "required_skill_missing:"+skillName)
			continue
		}

		var candidateIssue string
		for _, candidate := range candidates {
			record := candidate.Record
			if err := skill.ValidateGovernanceEvidenceReadinessWithRegistry(registry, skill.EvidenceReadinessInput{
				Level:                         level,
				Record:                        record,
				LatestFrozenRunSummaryVersion: latestRunSummaryVersion,
				ImplementerBaselineSessionID:  baselineSessionID,
			}); err != nil {
				candidateIssue = "required_skill_not_ready:" + skillName + ":" + sanitizeReason(err.Error())
				continue
			}
			if record.Verdict != model.EvidenceVerdictPass {
				candidateIssue = "required_skill_not_passed:" + skillName
				continue
			}
			if len(record.Blockers) > 0 {
				candidateIssue = "required_skill_blockers_present:" + skillName
				continue
			}
			passing[skillName] = record
			candidateIssue = ""
			break
		}

		if _, ok := passing[skillName]; !ok {
			if candidateIssue == "" {
				candidateIssue = "required_skill_not_ready:" + skillName
			}
			blockers = append(blockers, candidateIssue)
		}
	}

	return passing, uniqueSorted(blockers), nil
}

func loadSkillEvidenceFiles(root, requestID string) ([]skillEvidenceFile, []string, error) {
	skillRoot := filepath.Join(root, ".spln", "evidence", "skills")
	paths := []string{}
	issues := []string{}
	seen := map[string]struct{}{}

	addPath := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	requestDir := filepath.Join(skillRoot, requestID)
	_ = filepath.WalkDir(requestDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		addPath(path)
		return nil
	})

	rootEntries, err := os.ReadDir(skillRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	for _, entry := range rootEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		addPath(filepath.Join(skillRoot, entry.Name()))
	}

	slices.Sort(paths)

	records := make([]skillEvidenceFile, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, "skill_evidence_read_failed:"+filepath.Base(path))
			continue
		}
		record := model.EvidenceRecord{}
		if err := json.Unmarshal(raw, &record); err != nil {
			issues = append(issues, "skill_evidence_parse_failed:"+filepath.Base(path))
			continue
		}
		records = append(records, skillEvidenceFile{Path: path, Record: record})
	}

	return records, uniqueSorted(issues), nil
}

func latestImplementerBaselineSession(records []skillEvidenceFile, runSummaryVersion int) string {
	best := skillEvidenceFile{}
	found := false
	for _, record := range records {
		if record.Record.SkillName != "wave-orchestration" {
			continue
		}
		if runSummaryVersion > 0 && record.Record.RunSummaryVersion != runSummaryVersion {
			continue
		}
		if record.Record.Verdict != model.EvidenceVerdictPass {
			continue
		}
		if len(record.Record.Blockers) > 0 {
			continue
		}
		if !found || normalizedEvidenceTimestamp(record.Record).After(normalizedEvidenceTimestamp(best.Record)) {
			best = record
			found = true
		}
	}
	if !found {
		return ""
	}
	return best.Record.SessionID
}

func normalizedEvidenceTimestamp(record model.EvidenceRecord) time.Time {
	if record.Timestamp.IsZero() {
		return time.Time{}
	}
	return record.Timestamp.UTC()
}

func extractHighRiskChecks(
	passingSkills map[string]model.EvidenceRecord,
	nonTaskEvidence map[string]string,
) map[string]bool {
	checks := map[string]bool{}
	for _, record := range passingSkills {
		for _, ref := range record.References {
			checkID, pass, ok := parseHighRiskCheckReference(ref)
			if !ok {
				continue
			}
			checks[checkID] = pass
		}
	}

	for key, value := range nonTaskEvidence {
		checkID := strings.TrimSpace(key)
		checkID = strings.TrimPrefix(checkID, "check_result.")
		if !gate.IsRegisteredCheckID(checkID) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "pass", "passed", "true", "ok":
			checks[checkID] = true
		case "fail", "failed", "false":
			checks[checkID] = false
		}
	}

	return checks
}

func parseHighRiskCheckReference(reference string) (checkID string, pass bool, ok bool) {
	ref := strings.TrimSpace(reference)
	if ref == "" {
		return "", false, false
	}

	normalized := strings.ToLower(ref)
	normalized = strings.TrimPrefix(normalized, "high_risk_check:")
	normalized = strings.TrimPrefix(normalized, "check:")
	if strings.Contains(normalized, "=") {
		parts := strings.SplitN(normalized, "=", 2)
		checkID = strings.TrimSpace(parts[0])
		passToken := strings.TrimSpace(parts[1])
		if !gate.IsRegisteredCheckID(checkID) {
			return "", false, false
		}
		switch passToken {
		case "pass", "passed", "true", "ok":
			return checkID, true, true
		case "fail", "failed", "false":
			return checkID, false, true
		default:
			return "", false, false
		}
	}

	if strings.Count(normalized, ":") >= 1 {
		parts := strings.Split(normalized, ":")
		last := parts[len(parts)-1]
		checkID = strings.Join(parts[:len(parts)-1], ":")
		checkID = strings.TrimSpace(checkID)
		if !gate.IsRegisteredCheckID(checkID) {
			return "", false, false
		}
		switch strings.TrimSpace(last) {
		case "pass", "passed", "true", "ok":
			return checkID, true, true
		case "fail", "failed", "false":
			return checkID, false, true
		}
	}

	if gate.IsRegisteredCheckID(normalized) {
		return normalized, true, true
	}
	return "", false, false
}

func validateChangeManifestR0(
	manifestPath string,
	requestID string,
	slug string,
	level model.Level,
) (bool, []string) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, []string{"manifest_missing"}
	}
	parsed := changeManifest{}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return false, []string{"manifest_parse_invalid"}
	}

	reasons := []string{}
	if strings.TrimSpace(parsed.RequestID) != strings.TrimSpace(requestID) {
		reasons = append(reasons, "manifest_request_id_mismatch")
	}
	if strings.TrimSpace(parsed.Slug) != strings.TrimSpace(slug) {
		reasons = append(reasons, "manifest_slug_mismatch")
	}
	if strings.TrimSpace(parsed.CreatedAtLevel) != string(level) {
		reasons = append(reasons, "manifest_created_at_level_mismatch")
	}
	return len(reasons) == 0, uniqueSorted(reasons)
}

func sanitizeReason(input string) string {
	s := strings.TrimSpace(strings.ToLower(input))
	if s == "" {
		return "invalid"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "invalid"
	}
	return result
}
