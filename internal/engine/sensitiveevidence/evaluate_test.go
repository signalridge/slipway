package sensitiveevidence

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateBlocksSensitiveChangedFilesWithoutOwningEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		file     string
		category string
	}{
		{
			name:     "schema migration",
			file:     "db/migrations/001_create_users.sql",
			category: "schema_migration",
		},
		{
			name:     "authz code",
			file:     "internal/authz/policy.go",
			category: "auth_authz",
		},
		{
			name:     "permissions filename",
			file:     "internal/security/permissions.go",
			category: "auth_authz",
		},
		{
			name:     "rbac filename token",
			file:     "internal/security/user-rbac.go",
			category: "auth_authz",
		},
		{
			name:     "api contract",
			file:     "api/openapi.yaml",
			category: "api_contract",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			report := Evaluate(summary(taskRun("t-01", model.TaskKindCode, "go-test:./...", tc.file)), nil)

			require.Equal(t, StatusFail, report.Status)
			assert.Contains(t, model.ReasonSpecs(report.Blockers), "sensitive_evidence_missing:"+tc.category+":"+tc.file)
			require.Len(t, report.MissingEvidence, 1)
			assert.Equal(t, tc.category, report.MissingEvidence[0].Category)
			assert.Equal(t, tc.file, report.MissingEvidence[0].File)
		})
	}
}

func TestEvaluatePassesWhenMatchingOwningEvidenceExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		file      string
		evidence  string
		taskKind  model.TaskKind
		expectHit string
	}{
		{
			name:      "schema migration applied",
			file:      "supabase/migrations/20260610_add_users.sql",
			evidence:  "migration-applied:supabase db push",
			taskKind:  model.TaskKindOps,
			expectHit: "schema_migration",
		},
		{
			name:      "auth review recorded",
			file:      "internal/auth/token.go",
			evidence:  "auth-review:manual-authz-checklist",
			taskKind:  model.TaskKindVerification,
			expectHit: "auth_authz",
		},
		{
			name:      "permissions filename auth review recorded",
			file:      "internal/security/permissions.go",
			evidence:  "auth-review:manual-authz-checklist",
			taskKind:  model.TaskKindVerification,
			expectHit: "auth_authz",
		},
		{
			name:      "api contract test recorded",
			file:      "proto/public/v1/service.proto",
			evidence:  "contract-test:go test ./internal/contracts",
			taskKind:  model.TaskKindTest,
			expectHit: "api_contract",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			report := Evaluate(summary(taskRun("t-01", tc.taskKind, tc.evidence, tc.file)), nil)

			assert.Equal(t, StatusPass, report.Status)
			assert.Empty(t, report.Blockers)
			require.Len(t, report.SensitiveFiles, 1)
			assert.Equal(t, tc.expectHit, report.SensitiveFiles[0].Category)
		})
	}
}

func TestEvaluateAllowsSeparateVerificationEvidenceForCategory(t *testing.T) {
	t.Parallel()

	report := Evaluate(summary(
		taskRun("t-01", model.TaskKindCode, "implementation:api-change", "api/openapi.yaml"),
		taskRun("t-verify", model.TaskKindTest, "contract-test:go test ./api", "internal/api/client_test.go"),
	), nil)

	assert.Equal(t, StatusPass, report.Status)
	assert.Empty(t, report.Blockers)
}

func TestEvaluateIgnoresNonSensitiveFiles(t *testing.T) {
	t.Parallel()

	tests := []string{
		"cmd/status.go",
		"internal/cache/policy.go",
		"internal/cache/eviction_policy.go",
	}

	for _, file := range tests {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			report := Evaluate(summary(taskRun("t-01", model.TaskKindCode, "go-test:./...", file)), nil)

			assert.Equal(t, StatusNotApplicable, report.Status)
			assert.Empty(t, report.Blockers)
			assert.Empty(t, report.SensitiveFiles)
		})
	}
}

func TestEvaluateHasNoEnvironmentBypass(t *testing.T) {
	t.Setenv("GSD_SKIP_SCHEMA_CHECK", "true")

	const file = "prisma/schema.prisma"
	report := Evaluate(summary(taskRun("t-01", model.TaskKindCode, "go-test:./...", file)), nil)

	require.Equal(t, StatusFail, report.Status)
	assert.Contains(t, model.ReasonSpecs(report.Blockers), "sensitive_evidence_missing:schema_migration:"+file)
}

func taskRun(taskID string, kind model.TaskKind, evidenceRef string, changedFiles ...string) model.ExecutionTaskSummary {
	return model.ExecutionTaskSummary{
		TaskID:       taskID,
		Verdict:      model.TaskVerdictPass,
		TaskKind:     kind,
		ChangedFiles: changedFiles,
		TargetFiles:  changedFiles,
		EvidenceRef:  evidenceRef,
	}
}

func summary(tasks ...model.ExecutionTaskSummary) *model.ExecutionSummary {
	out := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks:             tasks,
	}
	out.Normalize()
	return out
}
