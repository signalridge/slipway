package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmitHydrateBlocksRendersDelimiter locks the `===== SLIPWAY HYDRATE:
// <skill-id>/<name> =====` delimiter contract and ensures each selected
// reference's body is rendered verbatim.
func TestEmitHydrateBlocksRendersDelimiter(t *testing.T) {
	root := generatedHydrateWorkspace(t)
	keys := []string{
		"security-review/authentication.md",
		"security-review/injection.md",
	}
	var buf bytes.Buffer
	if err := emitHydrateBlocks(root, &buf, keys); err != nil {
		t.Fatalf("emitHydrateBlocks: %v", err)
	}
	out := buf.String()
	for _, key := range keys {
		delim := fmt.Sprintf("===== SLIPWAY HYDRATE: %s =====", key)
		if !strings.Contains(out, delim) {
			t.Fatalf("output missing delimiter %q; got:\n%s", delim, out)
		}
	}
	// Each reference authored in Wave-1 begins with an H1. Assert it lands in
	// the emitted body so consumer agents see file-start grounding.
	if !strings.Contains(out, "\n# ") && !strings.HasPrefix(strings.TrimSpace(out), "#") {
		if !strings.Contains(out, "# ") {
			t.Fatalf("expected an H1 heading in emitted bodies; got:\n%s", out)
		}
	}
}

// TestEmitHydrateBlocksEmptyIsNoOp ensures the helper stays silent when no
// hydrate keys are selected, so commands without active references do not
// emit spurious delimiter lines.
func TestEmitHydrateBlocksEmptyIsNoOp(t *testing.T) {
	var buf bytes.Buffer
	if err := emitHydrateBlocks("", &buf, nil); err != nil {
		t.Fatalf("emitHydrateBlocks(nil): %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty output for nil keys, got %q", buf.String())
	}
}

// TestEmitHydrateBlocksRejectsUnsafeKey verifies that keys carrying `=` or
// newlines trip the delimiter-safety guard before any body is emitted.
func TestEmitHydrateBlocksRejectsUnsafeKey(t *testing.T) {
	for _, key := range []string{"security-review/foo=bar.md", "security-review/a\nb.md"} {
		var buf bytes.Buffer
		err := emitHydrateBlocks("", &buf, []string{key})
		if err == nil {
			t.Fatalf("expected error for unsafe key %q", key)
		}
		cliErr := asCLIError(err)
		if cliErr == nil {
			t.Fatalf("%q: expected CLI error, got %T", key, err)
		}
		if cliErr.ErrorCode != "hydrate_key_unsafe" {
			t.Fatalf("%q: unexpected error code %q", key, cliErr.ErrorCode)
		}
		if buf.Len() != 0 {
			t.Fatalf("%q: expected no output on unsafe key, got %q", key, buf.String())
		}
	}
}

// TestEmitHydrateBlocksMissingReferenceFailsDeterministically ensures a
// registry entry pointing at a non-existent file surfaces a structured
// `hydrate_reference_missing` error rather than a vague FS error.
func TestEmitHydrateBlocksMissingReferenceFailsDeterministically(t *testing.T) {
	root := generatedHydrateWorkspace(t)
	var buf bytes.Buffer
	err := emitHydrateBlocks(root, &buf, []string{"security-review/this-file-does-not-exist.md"})
	if err == nil {
		t.Fatal("expected error for missing reference file")
	}
	cliErr := asCLIError(err)
	if cliErr == nil {
		t.Fatalf("expected CLI error, got %T", err)
	}
	if cliErr.ErrorCode != "hydrate_reference_missing" {
		t.Fatalf("unexpected error code %q", cliErr.ErrorCode)
	}
}

// TestEmitHydrateBlocksOverWarnThresholdContinuesRendering keeps the advisory
// warning contract honest: large hydrate selections should still render, but
// they must surface a stable warning line ahead of the delimiter blocks.
func TestEmitHydrateBlocksOverWarnThresholdContinuesRendering(t *testing.T) {
	root := generatedHydrateWorkspace(t)
	prev := hydrateWarnBytes
	hydrateWarnBytes = 32
	t.Cleanup(func() { hydrateWarnBytes = prev })

	// At a 32-byte threshold, any real reference body will overflow the warning
	// band. Use two keys so the output proves rendering still continues.
	keys := []string{"security-review/authentication.md", "security-review/injection.md"}
	var buf bytes.Buffer
	if err := emitHydrateBlocks(root, &buf, keys); err != nil {
		t.Fatalf("emitHydrateBlocks: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "WARN hydrate_output_large:") {
		t.Fatalf("expected advisory warning in output, got:\n%s", out)
	}
	for _, key := range keys {
		delim := fmt.Sprintf("===== SLIPWAY HYDRATE: %s =====", key)
		if !strings.Contains(out, delim) {
			t.Fatalf("expected delimiter %q after warning, got:\n%s", delim, out)
		}
	}
}

func TestSelectHydrateKeysRejectsUnknownRequestedKey(t *testing.T) {
	_, err := selectHydrateKeys(
		[]string{"security-review/authentication.md", "security-review/injection.md"},
		[]string{"security-review/does-not-exist.md"},
	)
	require.Error(t, err)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "hydrate_ref_unknown", cliErr.ErrorCode)
}

func TestStatusCommandHydrateRefRequiresHydrate(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, true))

		cmd := makeStatusCmd()
		cmd.SetArgs([]string{
			"--view", "supply-chain-audit",
			"--hydrate-ref", "supply-chain-audit/results-template.md",
		})
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "hydrate_ref_requires_hydrate", cliErr.ErrorCode)
	})
}

func TestStatusCommandHydrateWarnsButStillRendersBodies(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, true))

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--view", "supply-chain-audit", "--hydrate"})
		require.NoError(t, cmd.Execute())

		rendered := out.String()
		assert.Contains(t, rendered, "WARN hydrate_output_large:")
		assert.Contains(t, rendered, "===== SLIPWAY HYDRATE: supply-chain-audit/results-template.md =====")
		assert.Contains(t, rendered, "===== SLIPWAY HYDRATE: supply-chain-audit/dependency-management-best-practices.md =====")
	})
}

func TestStatusCommandHydrateRefNarrowsOutput(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, true))

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{
			"--view", "supply-chain-audit",
			"--hydrate",
			"--hydrate-ref", "supply-chain-audit/results-template.md",
		})
		require.NoError(t, cmd.Execute())

		rendered := out.String()
		assert.NotContains(t, rendered, "WARN hydrate_output_large:")
		assert.Contains(t, rendered, "Hydrate: supply-chain-audit/results-template.md")
		assert.Contains(t, rendered, "===== SLIPWAY HYDRATE: supply-chain-audit/results-template.md =====")
		assert.NotContains(t, rendered, "dependency-management-best-practices.md =====")
	})
}

func TestHealthCommandHydrateRefNarrowsOutput(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, true))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{
			"--view", "incident-response",
			"--hydrate",
			"--hydrate-ref", "incident-response/incident-severity-matrix.md",
		})
		require.NoError(t, cmd.Execute())

		rendered := out.String()
		assert.Contains(t, rendered, "Hydrate: incident-response/incident-severity-matrix.md")
		assert.Contains(t, rendered, "===== SLIPWAY HYDRATE: incident-response/incident-severity-matrix.md =====")
		assert.NotContains(t, rendered, "incident-response-framework.md =====")
	})
}

func TestValidateDiagnosticsManualModeStillAdvertisesHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--mode", "sast-orchestration"})
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Equal(t, "sast-orchestration", view.Mode)
		assert.Contains(t, view.HydrateReferences, "sast-orchestration/codeql-ruleset-catalog.md")
		assert.Contains(t, view.HydrateReferences, "sast-orchestration/sarif-merge.md")
	})
}

func TestLoadHydrateBodyRejectsMalformedKey(t *testing.T) {
	_, err := loadHydrateBody("", "not-a-shaped-key")
	if err == nil {
		t.Fatal("expected error for malformed key")
	}
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "hydrate_key_invalid" {
		t.Fatalf("expected hydrate_key_invalid, got %v", err)
	}
}

func TestLoadHydrateBodyReadsGeneratedWorkspaceTree(t *testing.T) {
	root := generatedHydrateWorkspace(t)
	refPath := filepath.Join(root, ".codex", "skills", "slipway", "security-review", "references", "authentication.md")
	want := "# Workspace Override\n\nOnly the generated workspace tree should be read here.\n"
	if err := os.WriteFile(refPath, []byte(want), 0o644); err != nil {
		t.Fatalf("override generated hydrate file: %v", err)
	}

	got, err := loadHydrateBody(root, "security-review/authentication.md")
	if err != nil {
		t.Fatalf("loadHydrateBody: %v", err)
	}
	if got != want {
		t.Fatalf("expected workspace-backed hydrate body %q, got %q", want, got)
	}
}

func TestLoadHydrateBodyFailsWhenGeneratedReferenceDriftsMissing(t *testing.T) {
	root := generatedHydrateWorkspace(t)
	refPath := filepath.Join(root, ".codex", "skills", "slipway", "security-review", "references", "authentication.md")
	if err := os.Remove(refPath); err != nil {
		t.Fatalf("remove generated hydrate file: %v", err)
	}

	_, err := loadHydrateBody(root, "security-review/authentication.md")
	if err == nil {
		t.Fatal("expected error after deleting generated hydrate file")
	}
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "hydrate_reference_missing" {
		t.Fatalf("expected hydrate_reference_missing, got %v", err)
	}
}

func generatedHydrateWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	if err := toolgen.Generate(root, []string{"codex"}, true); err != nil {
		t.Fatalf("generate codex skill tree: %v", err)
	}
	return root
}
