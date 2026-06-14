package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// REQ-001: the shared scope-contract view must disclose the dirty
// artifacts/codebase/** files the scope-contract filter exempts from
// changed_files, so validate/status/review --json all surface the otherwise
// silent exemption. The exempted file must appear under exempt_context_files,
// must NOT leak into changed_files, and must not flip the pass status.
func TestBuildScopeContractViewSurfacesExemptContextFiles(t *testing.T) {
	t.Parallel()

	report := &scopecontract.Report{
		Status:             scopecontract.StatusPass,
		PlannedTargets:     []string{"cmd/validate.go"},
		ChangedFiles:       []string{"cmd/validate.go"},
		ExemptContextFiles: []string{"artifacts/codebase/ARCHITECTURE.md"},
	}

	view := buildScopeContractView(report)
	require.NotNil(t, view)

	assert.Equal(t, "pass", view.Status)
	assert.Contains(t, view.ExemptContextFiles, "artifacts/codebase/ARCHITECTURE.md",
		"exempt_context_files must disclose the dirty codebase-map file the filter exempts")
	assert.NotContains(t, view.ChangedFiles, "artifacts/codebase/ARCHITECTURE.md",
		"the exempted file must stay out of changed_files")
}
