package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReleaseWorkflowValidatesTagBeforeSecretExposure(t *testing.T) {
	workflow := readWorkflowYAML(t, ".github/workflows/release.yaml")
	jobs := workflowMap(t, workflow, "jobs")
	concurrency := workflowMap(t, workflow, "concurrency")
	assert.Equal(t, "release-${{ inputs.tag || github.ref_name }}", workflowString(t, concurrency, "group"))
	assert.Equal(t, false, concurrency["cancel-in-progress"])

	validateJob := workflowMap(t, jobs, "validate-tag")
	assert.Equal(t, "ubuntu-latest", workflowString(t, validateJob, "runs-on"))
	assert.Equal(t, "read", workflowString(t, workflowMap(t, validateJob, "permissions"), "contents"))
	assertNotContainsWorkflowValues(t, validateJob, "secrets.", "validate-tag must not expose secrets")

	validateRun := firstStepRun(t, validateJob, "Validate release tag syntax")
	assert.Contains(t, validateRun, "core='(0|[1-9][0-9]*)'")
	assert.Contains(t, validateRun, "prerelease_id=")
	assert.Contains(t, validateRun, "[[:cntrl:]]")
	assert.NotContains(t, validateRun, "grep")
	assert.Contains(t, validateRun, "printf 'tag_name=%s\\n'")
	assert.Contains(t, validateRun, ">> \"$GITHUB_OUTPUT\"")

	testJob := workflowMap(t, jobs, "test")
	assertWorkflowNeeds(t, testJob, "validate-tag")
	assert.Equal(t, "read", workflowString(t, workflowMap(t, testJob, "permissions"), "contents"))
	assert.Equal(
		t,
		"${{ needs.validate-tag.outputs.tag_name }}",
		firstStepWithUses(t, testJob, "actions/checkout@")["with"].(map[string]any)["ref"],
	)
	assertNotContainsWorkflowValues(t, testJob, "secrets.", "test must not expose release secrets")
	assertNotContainsWorkflowValues(t, testJob, "inputs.tag", "test must consume the validated tag output")

	releaseJob := workflowMap(t, jobs, "release")
	assertWorkflowNeeds(t, releaseJob, "validate-tag", "test")
	assert.NotContains(t, releaseJob, "environment", "release publishing must not require manual environment approval")
	releasePerms := workflowMap(t, releaseJob, "permissions")
	assert.Equal(t, "write", workflowString(t, releasePerms, "contents"))
	assert.Equal(t, "write", workflowString(t, releasePerms, "packages"))
	assert.Equal(t, "write", workflowString(t, releasePerms, "id-token"))
	assert.Equal(t, "write", workflowString(t, releasePerms, "attestations"))
	assert.Equal(
		t,
		"${{ needs.validate-tag.outputs.tag_name }}",
		firstStepWithUses(t, releaseJob, "actions/checkout@")["with"].(map[string]any)["ref"],
	)
	assert.Contains(t, workflowString(t, workflowMap(t, releaseJob, "outputs"), "tag_name"), "needs.validate-tag.outputs.tag_name")
	assertNotContainsWorkflowValues(t, releaseJob, "inputs.tag", "release must consume the validated tag output")
}

func TestReleasePleaseFailsClosedAndReconcilesPublishedState(t *testing.T) {
	workflow := readWorkflowYAML(t, ".github/workflows/release-please.yaml")
	concurrency := workflowMap(t, workflow, "concurrency")
	assert.Equal(t, "release-please", workflowString(t, concurrency, "group"))
	assert.Equal(t, false, concurrency["cancel-in-progress"])

	job := workflowMap(t, workflowMap(t, workflow, "jobs"), "release-please")
	requireTokenRun := firstStepRun(t, job, "Require release automation token")
	assert.Contains(t, requireTokenRun, "GH_PAT is required")

	action := firstNamedStep(t, job, "Release Please")
	with := workflowMap(t, action, "with")
	assert.Equal(t, "${{ secrets.GH_PAT }}", workflowString(t, with, "token"))
	assertNotContainsWorkflowValues(t, action, "secrets.GITHUB_TOKEN", "Release Please must not fall back to a token that suppresses tag workflows")

	checkout := firstNamedStep(t, job, "Check out triggering revision")
	checkoutWith := workflowMap(t, checkout, "with")
	assert.Equal(t, "${{ github.sha }}", workflowString(t, checkoutWith, "ref"))
	assert.Equal(t, 0, checkoutWith["fetch-depth"])

	reconcileRun := firstStepRun(t, job, "Reconcile manifest, tag, and GitHub Release")
	for _, contract := range []string{
		".release-please-manifest.json",
		"git ls-remote --exit-code --tags",
		"releases/tags/${expected_tag}",
	} {
		assert.Contains(t, reconcileRun, contract)
	}
	assert.NotContains(t, reconcileRun, "releases/latest", "an older serialized run must not compare its manifest with a moving latest release")
}

func TestReleasePleaseReconciliationStepFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Release Please shell contract runs on an Ubuntu runner")
	}
	workflow := readWorkflowYAML(t, ".github/workflows/release-please.yaml")
	job := workflowMap(t, workflowMap(t, workflow, "jobs"), "release-please")
	reconcileRun := firstStepRun(t, job, "Reconcile manifest, tag, and GitHub Release")

	tests := []struct {
		name        string
		gitExit     string
		releaseJSON string
		wantError   bool
	}{
		{
			name:        "reconciled",
			gitExit:     "0",
			releaseJSON: `{"tag_name":"v1.2.3","draft":false,"prerelease":false}`,
		},
		{
			name:        "missing tag",
			gitExit:     "1",
			releaseJSON: `{"tag_name":"v1.2.3","draft":false,"prerelease":false}`,
			wantError:   true,
		},
		{
			name:        "draft release",
			gitExit:     "0",
			releaseJSON: `{"tag_name":"v1.2.3","draft":true,"prerelease":false}`,
			wantError:   true,
		},
		{
			name:        "prerelease",
			gitExit:     "0",
			releaseJSON: `{"tag_name":"v1.2.3","draft":false,"prerelease":true}`,
			wantError:   true,
		},
		{
			name:        "release tag mismatch",
			gitExit:     "0",
			releaseJSON: `{"tag_name":"v9.9.9","draft":false,"prerelease":false}`,
			wantError:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory := t.TempDir()
			require.NoError(t, os.WriteFile(
				filepath.Join(directory, ".release-please-manifest.json"),
				[]byte(`{".":"1.2.3"}`),
				0o600,
			))
			bin := filepath.Join(directory, "bin")
			require.NoError(t, os.Mkdir(bin, 0o700))
			require.NoError(t, os.WriteFile(
				filepath.Join(bin, "git"),
				[]byte("#!/bin/sh\nexit \"${FAKE_GIT_EXIT}\"\n"),
				0o700,
			))
			require.NoError(t, os.WriteFile(
				filepath.Join(bin, "gh"),
				[]byte("#!/bin/sh\ncase \"$*\" in\n  *releases/tags/*) printf '%s\\n' \"$FAKE_RELEASE_JSON\" ;;\n  *) exit 64 ;;\nesac\n"),
				0o700,
			))

			command := exec.Command("bash", "-euo", "pipefail", "-c", reconcileRun)
			command.Dir = directory
			command.Env = append(os.Environ(),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"GITHUB_REPOSITORY=signalridge/slipway",
				"FAKE_GIT_EXIT="+tt.gitExit,
				"FAKE_RELEASE_JSON="+tt.releaseJSON,
			)
			output, err := command.CombinedOutput()
			if tt.wantError {
				require.Error(t, err, string(output))
				return
			}
			require.NoError(t, err, string(output))
			assert.Contains(t, string(output), "Reconciled manifest 1.2.3")
		})
	}
}

func TestReleaseWorkflowRejectsOutputInjectionTags(t *testing.T) {
	workflow := readWorkflowYAML(t, ".github/workflows/release.yaml")
	validateRun := firstStepRun(t, workflowMap(t, workflowMap(t, workflow, "jobs"), "validate-tag"), "Validate release tag syntax")

	output, stderr, err := runReleaseTagValidationStep(t, validateRun, "v1.2.3", "ignored-ref")
	require.NoError(t, err, stderr)
	assert.Equal(t, "tag_name=v1.2.3\n", output)

	tests := []struct {
		name string
		tag  string
	}{
		{name: "valid line after invalid prefix", tag: "bad-ref\nv1.2.3"},
		{name: "valid line before injected output", tag: "v1.2.3\ntag_name=v9.9.9"},
		{name: "carriage return", tag: "v1.2.3\rtag_name=v9.9.9"},
		{name: "tab control", tag: "v1.2.3\t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, stderr, err := runReleaseTagValidationStep(t, validateRun, tt.tag, "v1.2.3")
			require.Error(t, err, stderr)
			assert.Empty(t, output, "invalid tags must fail before writing GITHUB_OUTPUT")
		})
	}
}

func TestReleaseWorkflowSmokeInputsComeFromGeneratedManifest(t *testing.T) {
	workflow := readWorkflowYAML(t, ".github/workflows/release.yaml")
	jobs := workflowMap(t, workflow, "jobs")
	releaseJob := workflowMap(t, jobs, "release")

	outputs := workflowMap(t, releaseJob, "outputs")
	for _, output := range []string{"binary_matrix", "deb_asset", "rpm_asset", "apk_asset"} {
		assert.Contains(t, workflowString(t, outputs, output), "steps.smoke.outputs."+output)
	}

	smokeRun := firstStepRun(t, releaseJob, "Generate release smoke manifest")
	assert.Contains(t, smokeRun, "cd dist")
	assert.Contains(t, smokeRun, "find . -maxdepth 1 -type f")
	assert.Contains(t, smokeRun, "binary_matrix=")
	assert.Contains(t, smokeRun, "deb_asset=")
	assert.Contains(t, smokeRun, "rpm_asset=")
	assert.Contains(t, smokeRun, "apk_asset=")

	assert.Equal(
		t,
		"${{ needs.release.outputs.deb_asset }}",
		firstNamedStep(t, workflowMap(t, jobs, "verify-deb"), "Download and install deb")["env"].(map[string]any)["ASSET"],
	)
	assert.Equal(
		t,
		"${{ needs.release.outputs.rpm_asset }}",
		firstNamedStep(t, workflowMap(t, jobs, "verify-rpm"), "Download and install rpm")["env"].(map[string]any)["ASSET"],
	)
	assert.Equal(
		t,
		"${{ needs.release.outputs.apk_asset }}",
		firstNamedStep(t, workflowMap(t, jobs, "verify-apk"), "Download and install apk")["env"].(map[string]any)["ASSET"],
	)

	verifyBinary := workflowMap(t, jobs, "verify-binary")
	strategy := workflowMap(t, verifyBinary, "strategy")
	matrix := workflowMap(t, strategy, "matrix")
	assert.Equal(t, "${{ fromJSON(needs.release.outputs.binary_matrix) }}", workflowString(t, matrix, "include"))
}

func TestReleaseWorkflowPostPublishChecksHandleRunnerPolicyDrift(t *testing.T) {
	workflow := readWorkflowYAML(t, ".github/workflows/release.yaml")
	jobs := workflowMap(t, workflow, "jobs")

	provenance := workflowMap(t, jobs, "provenance")
	with := workflowMap(t, provenance, "with")
	assert.Equal(t, true, with["upload-assets"])
	assert.Equal(t, true, with["compile-generator"])
	assert.Contains(t, workflowString(t, with, "base64-subjects"), "needs.release.outputs.hashes")

	homebrewRun := firstStepRun(t, workflowMap(t, jobs, "verify-homebrew"), "Install via Homebrew")
	tapIndex := strings.Index(homebrewRun, "brew tap signalridge/tap")
	trustIndex := strings.Index(homebrewRun, "brew trust signalridge/tap")
	installIndex := strings.Index(homebrewRun, "brew install --cask slipway")
	require.NotEqual(t, -1, tapIndex, "Homebrew verification must tap signalridge/tap")
	require.NotEqual(t, -1, trustIndex, "Homebrew verification must trust the cask tap before install")
	require.NotEqual(t, -1, installIndex, "Homebrew verification must install the slipway cask")
	assert.Less(t, tapIndex, trustIndex)
	assert.Less(t, trustIndex, installIndex)
}

func TestGoReleaserUsesCosignBundleSigning(t *testing.T) {
	config := readWorkflowYAML(t, ".goreleaser.yaml")
	signs, ok := config["signs"].([]any)
	require.True(t, ok, "expected signs list")
	require.Len(t, signs, 1)

	sign, ok := signs[0].(map[string]any)
	require.True(t, ok, "expected sign entry")
	assert.Equal(t, "cosign", sign["cmd"])
	assert.Equal(t, "${artifact}.sigstore.json", sign["signature"])
	assert.NotContains(t, sign, "certificate")

	args, ok := sign["args"].([]any)
	require.True(t, ok, "expected sign args")
	argSet := workflowStringSet(args)
	assert.Contains(t, argSet, "--bundle=${signature}")
	assert.Contains(t, argSet, "sign-blob")
	assert.Contains(t, argSet, "${artifact}")
	assert.NotContains(t, argSet, "--output-certificate=${certificate}")
	assert.NotContains(t, argSet, "--output-signature=${signature}")
}

func TestCIWorkflowEnforcesKernelCoverageGate(t *testing.T) {
	workflow := readWorkflowYAML(t, ".github/workflows/ci.yml")
	coverageJob := workflowMap(t, workflowMap(t, workflow, "jobs"), "coverage")
	assert.Equal(t, "Kernel Coverage Gate", workflowString(t, coverageJob, "name"))

	measureRun := firstStepRun(t, coverageJob, "Measure soft-autopilot coverage")
	for _, pkg := range []string{
		"./cmd",
		"./internal/adapter",
		"./internal/autopilot",
		"./internal/fsutil",
		"./internal/jsonstrict",
		"./internal/recoverycmd",
		"./internal/runstore",
		"./internal/testlint",
		"./internal/tmpl",
	} {
		assert.Contains(t, measureRun, pkg)
	}
	assert.Contains(t, measureRun, "-coverprofile=tmp/coverage-gated.out")

	gateRun := firstStepRun(t, coverageJob, "Enforce coverage baseline")
	assert.Contains(t, gateRun, "./internal/covergate/cmd/covergate")
	assert.Contains(t, gateRun, "-check")
	assert.Contains(t, gateRun, "-profile tmp/coverage-gated.out")
	assert.Contains(t, gateRun, "-baseline coverage-baseline.json")
}

func readWorkflowYAML(t *testing.T, rel string) map[string]any {
	t.Helper()
	root := findRepoRootForWorkflowTest(t)
	raw, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &out))
	return out
}

func findRepoRootForWorkflowTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "could not find repo root from %s", dir)
		dir = parent
	}
}

func workflowMap(t *testing.T, raw any, key string) map[string]any {
	t.Helper()
	m, ok := raw.(map[string]any)
	require.Truef(t, ok, "expected map for %q, got %T", key, raw)
	value, ok := m[key]
	require.Truef(t, ok, "missing key %q", key)
	out, ok := value.(map[string]any)
	require.Truef(t, ok, "expected map at %q, got %T", key, value)
	return out
}

func runReleaseTagValidationStep(t *testing.T, run, inputTag, refName string) (string, string, error) {
	t.Helper()
	outputPath := filepath.Join(t.TempDir(), "github_output")
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", run)
	cmd.Env = append(os.Environ(),
		"INPUT_TAG="+inputTag,
		"REF_NAME="+refName,
		"GITHUB_OUTPUT="+outputPath,
	)
	combined, err := cmd.CombinedOutput()
	rawOutput, readErr := os.ReadFile(outputPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		require.NoError(t, readErr)
	}
	return string(rawOutput), string(combined), err
}

func workflowString(t *testing.T, raw any, key string) string {
	t.Helper()
	m, ok := raw.(map[string]any)
	require.Truef(t, ok, "expected map for %q, got %T", key, raw)
	value, ok := m[key]
	require.Truef(t, ok, "missing key %q", key)
	out, ok := value.(string)
	require.Truef(t, ok, "expected string at %q, got %T", key, value)
	return out
}

func firstStepRun(t *testing.T, job map[string]any, name string) string {
	t.Helper()
	step := firstNamedStep(t, job, name)
	run, ok := step["run"].(string)
	require.Truef(t, ok, "step %q has no run body", name)
	return run
}

func firstStepWithUses(t *testing.T, job map[string]any, usesPrefix string) map[string]any {
	t.Helper()
	steps, ok := job["steps"].([]any)
	require.True(t, ok, "job has no steps")
	for _, rawStep := range steps {
		step, ok := rawStep.(map[string]any)
		require.Truef(t, ok, "expected step map, got %T", rawStep)
		uses, _ := step["uses"].(string)
		if strings.HasPrefix(uses, usesPrefix) {
			return step
		}
	}
	t.Fatalf("missing step with uses prefix %q", usesPrefix)
	return nil
}

func firstNamedStep(t *testing.T, job map[string]any, name string) map[string]any {
	t.Helper()
	steps, ok := job["steps"].([]any)
	require.True(t, ok, "job has no steps")
	for _, rawStep := range steps {
		step, ok := rawStep.(map[string]any)
		require.Truef(t, ok, "expected step map, got %T", rawStep)
		if step["name"] == name {
			return step
		}
	}
	t.Fatalf("missing step %q", name)
	return nil
}

func assertWorkflowNeeds(t *testing.T, job map[string]any, want ...string) {
	t.Helper()
	rawNeeds, ok := job["needs"]
	require.True(t, ok, "job has no needs")
	got := workflowStringSet(rawNeeds)
	for _, need := range want {
		assert.Contains(t, got, need)
	}
}

func workflowStringSet(raw any) map[string]struct{} {
	out := map[string]struct{}{}
	switch v := raw.(type) {
	case string:
		out[v] = struct{}{}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				out[s] = struct{}{}
			}
		}
	}
	return out
}

func assertNotContainsWorkflowValues(t *testing.T, raw any, needle, msg string) {
	t.Helper()
	assert.NotContains(t, workflowScalarDump(raw), needle, msg)
}

func workflowScalarDump(raw any) string {
	switch v := raw.(type) {
	case map[string]any:
		var b strings.Builder
		for key, value := range v {
			b.WriteString(key)
			b.WriteByte('\n')
			b.WriteString(workflowScalarDump(value))
			b.WriteByte('\n')
		}
		return b.String()
	case []any:
		var b strings.Builder
		for _, item := range v {
			b.WriteString(workflowScalarDump(item))
			b.WriteByte('\n')
		}
		return b.String()
	case string:
		return v
	default:
		return ""
	}
}
