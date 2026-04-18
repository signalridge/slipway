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

func TestEmitHydrateBlocksUsesInvocationWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initGitRepoForWorktreeTests(t, root)

		worktreeRoot := filepath.Join(t.TempDir(), "linked-worktree")
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", "feat/hydrate-worktree", "HEAD")

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		require.NoError(t, bootstrap.InitWorkspace(worktreeRoot, []string{"codex"}, false))

		_, err = os.Stat(filepath.Join(root, ".codex"))
		assert.True(t, os.IsNotExist(err), "main scope should not need codex adapters for hydrate rendering")
		_, err = os.Stat(filepath.Join(worktreeRoot, ".codex", "skills", "slipway", "security-review", "references", "authentication.md"))
		require.NoError(t, err)

		var buf bytes.Buffer
		err = emitHydrateBlocks(root, &buf, []string{"security-review/authentication.md"})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "===== SLIPWAY HYDRATE: security-review/authentication.md =====")
	})
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
			"--focus", "incident",
			"--hydrate-ref", "incident-response/incident-severity-matrix.md",
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

		// Temporarily lower the size-warning threshold so incident-response's
		// six references (~30 KB total) comfortably exceed it. The canonical
		// 32 KB threshold remains exercised by the defaults elsewhere.
		orig := hydrateWarnBytes
		hydrateWarnBytes = 4 * 1024
		defer func() { hydrateWarnBytes = orig }()

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--focus", "incident", "--hydrate"})
		require.NoError(t, cmd.Execute())

		rendered := out.String()
		assert.Contains(t, rendered, "WARN hydrate_output_large:")
		assert.Contains(t, rendered, "===== SLIPWAY HYDRATE: incident-response/incident-severity-matrix.md =====")
		assert.Contains(t, rendered, "===== SLIPWAY HYDRATE: incident-response/incident-response-framework.md =====")
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
			"--focus", "incident",
			"--hydrate",
			"--hydrate-ref", "incident-response/incident-severity-matrix.md",
		})
		require.NoError(t, cmd.Execute())

		rendered := out.String()
		assert.NotContains(t, rendered, "WARN hydrate_output_large:")
		assert.Contains(t, rendered, "Hydrate: incident-response/incident-severity-matrix.md")
		assert.Contains(t, rendered, "===== SLIPWAY HYDRATE: incident-response/incident-severity-matrix.md =====")
		assert.NotContains(t, rendered, "incident-response-framework.md =====")
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
			"--focus", "incident",
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

func TestValidateDiagnosticsFocusStillAdvertisesHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--focus", "sast"})
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Equal(t, "sast", view.Mode)
		assert.Contains(t, view.HydrateReferences, "sast-orchestration/codeql-ruleset-catalog.md")
		assert.Contains(t, view.HydrateReferences, "sast-orchestration/sarif-merge.md")
	})
}

// TestValidateFocusPropertyAdvertisesHydrateReferences locks the Wave-2 PR-3
// hydrate contract for `validate --focus property`: the focus alias resolves
// through surface policy to property-testing and its declared hydrate
// references land on the view. Suggested-only skills (variant-analysis,
// performance-profiling) must not appear on this path.
func TestValidateFocusPropertyAdvertisesHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--focus", "property"})
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Equal(t, "property", view.Mode)

		for _, ref := range []string{
			"property-testing/design.md",
			"property-testing/generating.md",
			"property-testing/strategies.md",
			"property-testing/libraries.md",
			"property-testing/interpreting-failures.md",
		} {
			assert.Contains(t, view.HydrateReferences, ref,
				"--focus property must carry registry-declared hydrate ref %s", ref)
		}

		for _, ref := range view.HydrateReferences {
			assert.NotContains(t, ref, "variant-analysis/",
				"suggested-only variant-analysis must not leak into --focus property hydrate")
			assert.NotContains(t, ref, "performance-profiling/",
				"suggested-only performance-profiling must not leak into --focus property hydrate")
		}
	})
}

// TestValidateFocusMutationAdvertisesHydrateReferences locks the hydrate
// contract for `validate --focus mutation`: mutation-testing's two registry
// refs appear on the view; suggested-only skills do not.
func TestValidateFocusMutationAdvertisesHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--focus", "mutation"})
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Equal(t, "mutation", view.Mode)

		for _, ref := range []string{
			"mutation-testing/optimization-strategies.md",
			"mutation-testing/configuration.md",
		} {
			assert.Contains(t, view.HydrateReferences, ref,
				"--focus mutation must carry registry-declared hydrate ref %s", ref)
		}
		for _, ref := range view.HydrateReferences {
			assert.NotContains(t, ref, "variant-analysis/",
				"suggested-only variant-analysis must not leak into --focus mutation hydrate")
			assert.NotContains(t, ref, "performance-profiling/",
				"suggested-only performance-profiling must not leak into --focus mutation hydrate")
		}
	})
}

// TestReviewFocusCalibrationAdvertisesHydrateReferences locks the Wave-3 PR-3
// hydrate contract for `review --focus calibration`: the focus alias resolves
// through surface policy to multi-reviewer-calibration and its declared
// hydrate reference lands on the focus-resolver helper the review command
// feeds into its view. Script-only suggested skills (ci-triage,
// review-comment-triage) must not appear on this path. The full review
// command requires an active change and full governance; this test pins the
// cmd-layer hydrate surface itself, which is the narrower contract Wave-3
// PR-3 cares about.
func TestReviewFocusCalibrationAdvertisesHydrateReferences(t *testing.T) {
	t.Parallel()

	keys := resolveEffectiveFocusHydrate("review", "calibration")
	require.NotEmpty(t, keys, "--focus calibration must expose hydrate keys")

	assert.Contains(t, keys, "multi-reviewer-calibration/review-dimensions.md",
		"--focus calibration must carry the registry-declared hydrate ref")

	for _, ref := range keys {
		assert.True(t,
			strings.HasPrefix(ref, "multi-reviewer-calibration/"),
			"only the explicit-focus backing skill's keys may surface on this alias, got %q", ref)
		assert.NotContains(t, ref, "ci-triage/",
			"suggested-only ci-triage must not leak into --focus calibration hydrate")
		assert.NotContains(t, ref, "review-comment-triage/",
			"suggested-only review-comment-triage must not leak into --focus calibration hydrate")
	}
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
