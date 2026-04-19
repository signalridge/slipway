package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRequirementsContractReturnsValidResult(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "example-change"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))

	result, err := EvaluateRequirementsContract(bundleDir, slug)
	require.NoError(t, err)
	assert.Equal(t, RequirementsContractStatusValid, result.Status)
	assert.Equal(t, ResolveArtifactPath(bundleDir, slug, "requirements.md"), result.Source)
	assert.Contains(t, result.Message, "validated")
}

func TestEvaluateRequirementsContractReturnsMissingResult(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "example-change"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	result, err := EvaluateRequirementsContract(bundleDir, slug)
	require.NoError(t, err)
	assert.Equal(t, RequirementsContractStatusMissing, result.Status)
	assert.Equal(t, ResolveArtifactPath(bundleDir, slug, "requirements.md"), result.Source)
	assert.Contains(t, result.Message, "missing")
}

func TestEvaluateRequirementsContractReturnsInvalidResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantMessage string
	}{
		{
			name: "no requirement blocks",
			content: `# Requirements

This file has no requirement blocks.
`,
			wantMessage: "no Requirement blocks found",
		},
		{
			name: "missing stable ids",
			content: `# Requirements

### Requirement: Auth
The system must authenticate requests.
`,
			wantMessage: "missing stable REQ-* IDs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			slug := "example-change"
			bundleDir := filepath.Join(root, "artifacts", "changes", slug)
			require.NoError(t, os.MkdirAll(bundleDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(tt.content), 0o644))

			result, err := EvaluateRequirementsContract(bundleDir, slug)
			require.NoError(t, err)
			assert.Equal(t, RequirementsContractStatusInvalid, result.Status)
			assert.Equal(t, ResolveArtifactPath(bundleDir, slug, "requirements.md"), result.Source)
			assert.Contains(t, result.Message, tt.wantMessage)
		})
	}
}

func TestEvaluateRequirementsContractReturnsErrorForUnreadableFile(t *testing.T) {
	root := t.TempDir()
	slug := "example-change"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	reqPath := filepath.Join(bundleDir, "requirements.md")
	require.NoError(t, os.WriteFile(reqPath, []byte(`# Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))
	require.NoError(t, os.Chmod(reqPath, 0))
	t.Cleanup(func() {
		_ = os.Chmod(reqPath, 0o644)
	})

	_, err := EvaluateRequirementsContract(bundleDir, slug)
	require.Error(t, err)
}
