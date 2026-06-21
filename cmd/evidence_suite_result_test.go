package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEvidenceSuiteResultRecordsCurrentRunSummaryVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence suite result records")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 3, "t-01")
		proofPath := filepath.Join(root, "verification", "logs", "suite-result-run2.txt")
		require.NoError(t, os.MkdirAll(filepath.Dir(proofPath), 0o755))
		require.NoError(t, os.WriteFile(proofPath, []byte("fresh full suite proof\n"), 0o644))
		expectedDigest := sha256DigestForTest([]byte("fresh full suite proof\n"))
		suitePath := filepath.Join(state.VerificationDir(root, slug), "suite-result.yaml")
		require.NoError(t, os.Remove(suitePath))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetOut(&out)
		cmd.SetArgs([]string{
			"suite-result",
			"--change", slug,
			"--full-suite-proof", "verification/logs/suite-result-run2.txt",
			"--sast-digest", "credentials.safety_baseline=sha256:sast",
			"--json",
		})
		require.NoError(t, cmd.Execute())

		var view evidenceSuiteResultView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, 3, view.RunSummaryVersion)
		assert.Equal(t, expectedDigest, view.FullSuiteDigest)
		assert.Equal(t, "sha256:sast", view.SASTDigests["credentials.safety_baseline"])
		assert.Equal(t, "artifacts/changes/"+slug+"/verification/suite-result.yaml", view.Path)
		assert.True(t, view.Recorded)

		raw, err := os.ReadFile(suitePath)
		require.NoError(t, err)
		var record model.SuiteResult
		require.NoError(t, yaml.Unmarshal(raw, &record))
		assert.Equal(t, model.SuiteResultVersion, record.Version)
		assert.Equal(t, 3, record.RunSummaryVersion)
		assert.Equal(t, expectedDigest, record.FullSuiteDigest)
		assert.Equal(t, "sha256:sast", record.SASTDigests["credentials.safety_baseline"])
		assert.False(t, record.CapturedAt.IsZero())
	})
}

func TestEvidenceSuiteResultRejectsMissingExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence suite result needs summary")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"suite-result",
			"--change", slug,
			"--full-suite-digest", "sha256:full-suite",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_suite_result_run_summary_missing", cliErr.ErrorCode)
		assert.Equal(t, slug, cliErr.Slug)
	})
}

func TestEvidenceSuiteResultRejectsInvalidSASTDigest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence suite result invalid sast")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"suite-result",
			"--change", slug,
			"--full-suite-digest", "sha256:full-suite",
			"--sast-digest", "credentials.safety_baseline",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_suite_result_named_value_invalid", cliErr.ErrorCode)
	})
}

func TestEvidenceSuiteResultRejectsFullSuiteDigestAndProofConflict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence suite result conflict")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"suite-result",
			"--change", slug,
			"--full-suite-digest", "sha256:full-suite",
			"--full-suite-proof", "verification/logs/suite-result-run2.txt",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_suite_result_full_suite_conflict", cliErr.ErrorCode)
	})
}

func TestEvidenceSuiteResultRejectsUnsafeProofPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence suite result unsafe proof")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"suite-result",
			"--change", slug,
			"--full-suite-proof", "../suite-result-run2.txt",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_suite_result_proof_path_invalid", cliErr.ErrorCode)
	})
}

func TestEvidenceSuiteResultHashesSASTProof(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence suite result hashes sast")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		proofPath := filepath.Join(root, "verification", "logs", "sast.txt")
		require.NoError(t, os.MkdirAll(filepath.Dir(proofPath), 0o755))
		require.NoError(t, os.WriteFile(proofPath, []byte("sast proof\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"suite-result",
			"--change", slug,
			"--full-suite-digest", "sha256:full-suite",
			"--sast-proof", "credentials.safety_baseline=verification/logs/sast.txt",
		})
		require.NoError(t, cmd.Execute())

		raw, err := os.ReadFile(filepath.Join(state.VerificationDir(root, slug), "suite-result.yaml"))
		require.NoError(t, err)
		var record model.SuiteResult
		require.NoError(t, yaml.Unmarshal(raw, &record))
		assert.Equal(t, sha256DigestForTest([]byte("sast proof\n")), record.SASTDigests["credentials.safety_baseline"])
	})
}

func sha256DigestForTest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
