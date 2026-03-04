package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type EvidenceVerdict string

const (
	EvidenceVerdictPass EvidenceVerdict = "pass"
	EvidenceVerdictFail EvidenceVerdict = "fail"
)

func (v EvidenceVerdict) IsValid() bool {
	switch v {
	case EvidenceVerdictPass, EvidenceVerdictFail:
		return true
	default:
		return false
	}
}

type EvidenceRole string

const (
	EvidenceRoleImplementer EvidenceRole = "implementer"
	EvidenceRoleReviewer    EvidenceRole = "reviewer"
	EvidenceRoleOperator    EvidenceRole = "operator"
)

func (r EvidenceRole) IsValid() bool {
	switch r {
	case EvidenceRoleImplementer, EvidenceRoleReviewer, EvidenceRoleOperator:
		return true
	default:
		return false
	}
}

type EvidenceRecord struct {
	RunSummaryVersion int             `json:"run_summary_version"`
	SessionID         string          `json:"session_id"`
	SkillName         string          `json:"skill_name"`
	Version           string          `json:"version"`
	State             WorkflowState   `json:"state"`
	Verdict           EvidenceVerdict `json:"verdict"`
	Blockers          []string        `json:"blockers"`
	References        []string        `json:"references"`
	Timestamp         time.Time       `json:"timestamp"`
	InputHash         string          `json:"input_hash,omitempty"`
	InputScope        []string        `json:"input_scope,omitempty"`
	MitigationTarget  string          `json:"mitigation_target,omitempty"`
	ActorID           string          `json:"actor_id,omitempty"`
	Role              EvidenceRole    `json:"role,omitempty"`
}

var mitigationBySkill = map[string]string{
	"intake-analysis":    "unclear intent and hidden guardrail risk",
	"scope-confirmation": "L3 discovery/scope drift",
	"plan-audit":         "stale or incomplete plan bundle",
	"wave-orchestration": "uncontrolled parallel execution drift",
	"artifact-review":    "cross-artifact inconsistency",
	"goal-verification":  "false completion claims",
	"final-closeout":     "stale final evidence before governed ship decision",
}

var (
	stateRequiresRunSummary = map[WorkflowState]struct{}{
		StateS6RunWaves: {},
		StateS7Review:   {},
		StateS8Verify:   {},
	}
	statePreRunSummary = map[WorkflowState]struct{}{
		StateS1Analyze:           {},
		StateS3ScopeConfirmation: {},
		StateS5PlanAudit:         {},
	}
	evidenceHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	skillNameSanitizer  = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
)

func (e EvidenceRecord) Validate() error {
	if e.RunSummaryVersion < 0 {
		return fmt.Errorf("run_summary_version must be >= 0: %d", e.RunSummaryVersion)
	}
	if !IsUUIDv7(e.SessionID) {
		return fmt.Errorf("session_id must be UUIDv7: %q", e.SessionID)
	}
	if strings.TrimSpace(e.SkillName) == "" {
		return errors.New("skill_name is required")
	}
	if strings.TrimSpace(e.Version) == "" {
		return errors.New("version is required")
	}
	if strings.TrimSpace(string(e.State)) == "" {
		return errors.New("state is required")
	}
	if !e.Verdict.IsValid() {
		return fmt.Errorf("invalid verdict: %q", e.Verdict)
	}
	if e.Blockers == nil {
		return errors.New("blockers field is required")
	}
	if e.References == nil {
		return errors.New("references field is required")
	}
	if e.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if e.Role != "" && !e.Role.IsValid() {
		return fmt.Errorf("invalid role: %q", e.Role)
	}

	if _, requires := stateRequiresRunSummary[e.State]; requires {
		if e.RunSummaryVersion < 1 {
			return fmt.Errorf("state %q requires run_summary_version >= 1", e.State)
		}
		if strings.TrimSpace(e.InputHash) == "" {
			return fmt.Errorf("state %q requires input_hash", e.State)
		}
	}
	if _, preRun := statePreRunSummary[e.State]; preRun && e.RunSummaryVersion != 0 {
		return fmt.Errorf("state %q requires run_summary_version = 0", e.State)
	}

	if e.InputHash != "" && !evidenceHashPattern.MatchString(e.InputHash) {
		return fmt.Errorf("input_hash must be lowercase hex SHA-256: %q", e.InputHash)
	}

	expectedMitigation, err := ResolveMitigationTarget(e.SkillName)
	if err != nil {
		return err
	}
	if e.MitigationTarget != "" && e.MitigationTarget != expectedMitigation {
		return fmt.Errorf(
			"mitigation_target mismatch for skill_name=%q: expected %q got %q",
			e.SkillName,
			expectedMitigation,
			e.MitigationTarget,
		)
	}

	return nil
}

func ResolveMitigationTarget(skillName string) (string, error) {
	target, ok := mitigationBySkill[skillName]
	if !ok {
		return "", fmt.Errorf("unknown skill_name for mitigation mapping: %q", skillName)
	}
	return target, nil
}

// ComputeInputHash returns canonical SHA-256 hash over normalized JSON payload.
func ComputeInputHash(payload map[string]any) (string, error) {
	normalized := normalizeCanonical(payload)
	b, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeCanonical(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out := make(map[string]any, len(x))
		for _, k := range keys {
			out[k] = normalizeCanonical(x[k])
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			out = append(out, normalizeCanonical(item))
		}
		return out
	case string:
		return strings.ReplaceAll(x, "\r\n", "\n")
	default:
		return v
	}
}

func WriteEvidenceFile(dir string, evidence EvidenceRecord) (string, error) {
	if err := evidence.Validate(); err != nil {
		return "", err
	}
	if evidence.MitigationTarget == "" {
		target, err := ResolveMitigationTarget(evidence.SkillName)
		if err != nil {
			return "", err
		}
		evidence.MitigationTarget = target
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	session := evidence.SessionID
	skill := sanitizeSkillName(evidence.SkillName)
	baseName := fmt.Sprintf("%s--%s", session, skill)

	for idx := 0; ; idx++ {
		fileName := baseName
		if idx > 0 {
			fileName = fmt.Sprintf("%s--%d", baseName, idx)
		}
		fileName += ".json"
		path := filepath.Join(dir, fileName)

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", err
		}

		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "  ")
		encodeErr := encoder.Encode(evidence)
		closeErr := f.Close()
		if encodeErr != nil {
			_ = os.Remove(path)
			return "", encodeErr
		}
		if closeErr != nil {
			_ = os.Remove(path)
			return "", closeErr
		}

		return path, nil
	}
}

func sanitizeSkillName(skillName string) string {
	s := skillNameSanitizer.ReplaceAllString(skillName, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "unknown-skill"
	}
	return s
}
