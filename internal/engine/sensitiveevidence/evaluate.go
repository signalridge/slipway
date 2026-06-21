package sensitiveevidence

import (
	"path"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

type Status string

const (
	StatusNotApplicable Status = "not_applicable"
	StatusPass          Status = "pass"
	StatusFail          Status = "fail"
)

const ReasonSensitiveEvidenceMissing = "sensitive_evidence_missing"

type SensitiveFile struct {
	File     string `json:"file" yaml:"file"`
	Category string `json:"category" yaml:"category"`
	Marker   string `json:"marker" yaml:"marker"`
}

type MissingEvidence struct {
	File     string `json:"file" yaml:"file"`
	Category string `json:"category" yaml:"category"`
	Marker   string `json:"marker" yaml:"marker"`
}

type Report struct {
	Status          Status             `json:"status" yaml:"status"`
	SensitiveFiles  []SensitiveFile    `json:"sensitive_files,omitempty" yaml:"sensitive_files,omitempty"`
	MissingEvidence []MissingEvidence  `json:"missing_evidence,omitempty" yaml:"missing_evidence,omitempty"`
	Blockers        []model.ReasonCode `json:"blockers,omitempty" yaml:"blockers,omitempty"`
}

type categoryRule struct {
	category string
	marker   string
	match    func(string) bool
}

var categoryRules = []categoryRule{
	{
		category: "schema_migration",
		marker:   "migration-applied",
		match:    isSchemaFile,
	},
	{
		category: "auth_authz",
		marker:   "auth-review",
		match:    isAuthFile,
	},
	{
		category: "api_contract",
		marker:   "contract-test",
		match:    isAPIContractFile,
	},
}

// Evaluate enforces that sensitive changed files have owning task evidence.
// It intentionally reads only execution evidence and explicit changed-file
// inputs; environment variables cannot disable the gate.
func Evaluate(summary *model.ExecutionSummary, extraChangedFiles []string) Report {
	if summary == nil || summary.RunSummaryVersion <= 0 {
		return Report{Status: StatusNotApplicable}
	}

	changedFiles := passedChangedFiles(summary, extraChangedFiles)
	sensitiveFiles := classifySensitiveFiles(changedFiles)
	if len(sensitiveFiles) == 0 {
		return Report{Status: StatusNotApplicable}
	}

	markers := evidenceMarkers(summary)
	report := Report{
		Status:         StatusPass,
		SensitiveFiles: sensitiveFiles,
	}

	for _, file := range sensitiveFiles {
		if markers[file.Marker] {
			continue
		}
		report.Status = StatusFail
		report.MissingEvidence = append(report.MissingEvidence, MissingEvidence(file))
		report.Blockers = append(report.Blockers, model.NewReasonCode(
			ReasonSensitiveEvidenceMissing,
			file.Category+":"+file.File,
		))
	}

	normalizeReport(&report)
	return report
}

func passedChangedFiles(summary *model.ExecutionSummary, extraChangedFiles []string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(file string) {
		file = normalizePath(file)
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		out = append(out, file)
	}

	for _, task := range summary.Tasks {
		if task.Verdict != model.TaskVerdictPass || len(task.Blockers) > 0 {
			continue
		}
		for _, file := range task.ChangedFiles {
			add(file)
		}
	}
	for _, file := range extraChangedFiles {
		add(file)
	}
	slices.Sort(out)
	return out
}

func classifySensitiveFiles(changedFiles []string) []SensitiveFile {
	var out []SensitiveFile
	for _, file := range changedFiles {
		for _, rule := range categoryRules {
			if !rule.match(file) {
				continue
			}
			out = append(out, SensitiveFile{
				File:     file,
				Category: rule.category,
				Marker:   rule.marker,
			})
			break
		}
	}
	slices.SortFunc(out, func(left, right SensitiveFile) int {
		if left.Category != right.Category {
			return strings.Compare(left.Category, right.Category)
		}
		return strings.Compare(left.File, right.File)
	})
	return out
}

func evidenceMarkers(summary *model.ExecutionSummary) map[string]bool {
	out := make(map[string]bool)
	for _, task := range summary.Tasks {
		if task.Verdict != model.TaskVerdictPass || len(task.Blockers) > 0 {
			continue
		}
		for _, token := range markerTokens(task.EvidenceRef) {
			for _, rule := range categoryRules {
				if token == rule.marker || strings.HasPrefix(token, rule.marker+":") || strings.HasPrefix(token, rule.marker+"=") {
					out[rule.marker] = true
				}
			}
		}
	}
	return out
}

func markerTokens(ref string) []string {
	fields := strings.FieldsFunc(ref, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ',', ';':
			return true
		default:
			return false
		}
	})
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return slices.DeleteFunc(fields, func(field string) bool { return field == "" })
}

func isSchemaFile(file string) bool {
	segments, base := pathParts(file)
	for _, segment := range segments {
		switch segment {
		case "migration", "migrations":
			return true
		}
	}
	if base == "schema.prisma" || base == "schema.sql" {
		return true
	}
	if len(segments) >= 3 && segments[len(segments)-2] == "schema" {
		switch segments[len(segments)-3] {
		case "ent":
			return true
		}
	}
	if len(segments) >= 3 && segments[len(segments)-3] == "prisma" && segments[len(segments)-2] == "schema" {
		return strings.HasSuffix(base, ".prisma")
	}
	return strings.HasSuffix(file, "/prisma/schema.prisma")
}

func isAuthFile(file string) bool {
	segments, base := pathParts(file)
	for _, segment := range segments {
		switch segment {
		case "auth", "authn", "authz", "authentication", "authorization", "rbac", "permission", "permissions":
			return true
		}
	}

	stem := strings.TrimSuffix(base, path.Ext(base))
	switch {
	case stem == "auth", stem == "authn", stem == "authz", stem == "permission", stem == "permissions", stem == "rbac":
		return true
	case strings.Contains(stem, "authz"), strings.Contains(stem, "authorization"):
		return true
	case strings.Contains(stem, "authn"), strings.Contains(stem, "authentication"):
		return true
	case hasAuthzFilenameToken(stem):
		return true
	}
	return false
}

func hasAuthzFilenameToken(stem string) bool {
	for _, token := range strings.FieldsFunc(stem, func(r rune) bool {
		switch r {
		case '_', '-', '.':
			return true
		default:
			return false
		}
	}) {
		switch token {
		case "permission", "permissions", "rbac":
			return true
		}
	}
	return false
}

func isAPIContractFile(file string) bool {
	segments, base := pathParts(file)
	for _, segment := range segments {
		switch segment {
		case "proto", "protos", "openapi", "contracts", "api-contracts":
			return true
		}
	}
	if strings.HasSuffix(base, ".proto") {
		return true
	}
	switch base {
	case "openapi.yaml", "openapi.yml", "openapi.json", "swagger.yaml", "swagger.yml", "swagger.json":
		return true
	}
	return false
}

func pathParts(file string) ([]string, string) {
	file = normalizePath(file)
	if file == "" {
		return nil, ""
	}
	segments := strings.Split(file, "/")
	return segments, segments[len(segments)-1]
}

func normalizePath(file string) string {
	file = strings.TrimSpace(strings.ReplaceAll(file, "\\", "/"))
	file = path.Clean(file)
	if file == "." {
		return ""
	}
	return strings.TrimPrefix(file, "./")
}

func normalizeReport(report *Report) {
	slices.SortFunc(report.MissingEvidence, func(left, right MissingEvidence) int {
		if left.Category != right.Category {
			return strings.Compare(left.Category, right.Category)
		}
		return strings.Compare(left.File, right.File)
	})
	for i := range report.Blockers {
		report.Blockers[i].Normalize()
	}
	report.Blockers = model.NormalizeReasonCodes(report.Blockers)
}
