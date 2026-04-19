package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/intake"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingIntentClassifier struct {
	classification progression.IntentClassification
	err            error
	inputs         []string
}

func (r *recordingIntentClassifier) Classify(_ context.Context, inferenceText string) (progression.IntentClassification, error) {
	r.inputs = append(r.inputs, inferenceText)
	if r.err != nil {
		return progression.IntentClassification{}, r.err
	}
	return r.classification, nil
}

func TestNewCommandRequiresDescription(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		cmd := makeNewCmd()
		cmd.SetArgs([]string{})
		require.Error(t, cmd.Execute())
	})
}

func TestGenerateUniqueChangeSlugFailsWhenSlugLookupErrors(t *testing.T) {
	t.Parallel()

	_, err := generateUniqueChangeSlug("fix login timeout", func(string) (bool, error) {
		return false, assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)
}

func TestGenerateUniqueChangeSlugUsesNextAvailableSuffix(t *testing.T) {
	t.Parallel()

	var seen []string
	slug, err := generateUniqueChangeSlug("fix login timeout", func(candidate string) (bool, error) {
		seen = append(seen, candidate)
		switch candidate {
		case "fix-login-timeout", "fix-login-timeout-2":
			return true, nil
		case "fix-login-timeout-3":
			return false, nil
		default:
			return false, fmt.Errorf("unexpected candidate %q", candidate)
		}
	})
	require.NoError(t, err)
	assert.Equal(t, "fix-login-timeout-3", slug)
	assert.Equal(t, []string{
		"fix-login-timeout",
		"fix-login-timeout-2",
		"fix-login-timeout-3",
	}, seen)
}

func TestGenerateUniqueChangeSlugFailsWhenAttemptsExhausted(t *testing.T) {
	t.Parallel()

	_, err := generateUniqueChangeSlug("fix login timeout", func(string) (bool, error) {
		return true, nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to generate unique change slug")
}

func TestNewCommandGuardrailAutoCreatesDiscoveryChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		classifier := &recordingIntentClassifier{
			classification: progression.IntentClassification{
				GuardrailDomain: "auth_authz",
				NeedsDiscovery:  true,
				Complexity:      "critical",
			},
		}

		cmd := makeNewCmd()
		cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
		cmd.SetArgs([]string{"update auth middleware timeout strategy"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.NeedsDiscovery)
		assert.Equal(t, model.GuardrailDomainAuthAuthZ, change.GuardrailDomain)
		assert.Equal(t, model.StateS0Intake, change.CurrentState)
	})
}

func TestNewCommandPassesDescriptionAndDocContentToIntentClassifier(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		docBody := "# Session timeout\n\n## Constraints\n- keep middleware contract\n"
		require.NoError(t, os.WriteFile(docPath, []byte(docBody), 0o644))

		classifier := &recordingIntentClassifier{
			classification: progression.IntentClassification{
				GuardrailDomain: "",
				NeedsDiscovery:  false,
				Complexity:      "simple",
			},
		}

		cmd := makeNewCmd()
		cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
		cmd.SetArgs([]string{"--preset", "standard", "--from-doc", docPath, "session timeout"})
		require.NoError(t, cmd.Execute())

		require.Len(t, classifier.inputs, 1)
		assert.Equal(t, "session timeout\n"+strings.TrimSpace(docBody), classifier.inputs[0])
	})
}

func TestNewCommandFromDocSeedsRequirementsAndTasks(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		require.NoError(t, os.WriteFile(docPath, []byte(`# Session timeout

## In Scope
- expire idle sessions after 15 minutes

## Constraints
- keep existing middleware contract
`), 0o644))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--preset", "standard", "--from-doc", docPath, "session timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		requirementsRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "requirements.md"))
		require.NoError(t, err)
		assert.Contains(t, string(requirementsRaw), "15 minutes")

		tasksRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "tasks.md"))
		require.NoError(t, err)
		assert.Contains(t, string(tasksRaw), "t-01")
		assert.NotContains(t, string(tasksRaw), "Define implementation tasks")
	})
}

func TestNewCommandFromDocAcceptanceOnlySeedsTasks(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		require.NoError(t, os.WriteFile(docPath, []byte(`# Session timeout

## Acceptance Criteria
- verify idle sessions expire after 15 minutes
- keep the current middleware contract intact
`), 0o644))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--preset", "standard", "--from-doc", docPath, "session timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		tasksRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "tasks.md"))
		require.NoError(t, err)
		assert.Contains(t, string(tasksRaw), "15 minutes")
		assert.Contains(t, string(tasksRaw), "middleware contract")
	})
}

func TestNewCommandFromDocSeedsIntentSectionsAndSourceDocument(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		require.NoError(t, os.WriteFile(docPath, []byte(`# Session timeout

## In Scope
- expire idle sessions after 15 minutes

## Out of Scope
- redesign the login screen

## Constraints
- keep existing middleware contract

## Acceptance Criteria
- verify idle sessions expire after 15 minutes
`), 0o644))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--preset", "standard", "--from-doc", docPath})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		intentRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "intent.md"))
		require.NoError(t, err)
		intent := string(intentRaw)

		assert.Contains(t, intent, "## In Scope\n- expire idle sessions after 15 minutes")
		assert.Contains(t, intent, "## Out of Scope\n- redesign the login screen")
		assert.Contains(t, intent, "## Constraints\n- keep existing middleware contract")
		assert.Contains(t, intent, "## Acceptance Signals\n- verify idle sessions expire after 15 minutes")
		assert.Contains(t, intent, "### Source Document")
		assert.Contains(t, intent, "# Session timeout")
	})
}

func TestNewCommandFromDocIgnoresInlineHeadingMentionsBeforeRealSections(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "idea.md")
		require.NoError(t, os.WriteFile(docPath, []byte(`# Session timeout

This draft mentions ## In Scope in prose before the actual section.

## In Scope
- expire idle sessions after 15 minutes
- preserve MFA enforcement for admin sessions
`), 0o644))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--preset", "standard", "--from-doc", docPath, "session timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		requirementsRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "requirements.md"))
		require.NoError(t, err)
		assert.Contains(t, string(requirementsRaw), "15 minutes")
		assert.Contains(t, strings.ToLower(string(requirementsRaw)), "preserve mfa enforcement")
		assert.NotContains(t, string(requirementsRaw), "The system MUST session timeout.")
	})
}

func TestExtractDocSectionsSupportsColonSuffixedHeadings(t *testing.T) {
	doc := `# Session timeout

## In Scope:
- expire idle sessions after 15 minutes

## Constraints:
- keep existing middleware contract
`

	sections := intake.ParseDoc(doc)
	assert.Contains(t, sections.Scope, "15 minutes")
	assert.Contains(t, sections.Constraints, "middleware contract")
}

func TestNewCommandFromDocRejectsUnreadableFile(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--from-doc", filepath.Join(root, "missing.md")})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read document")
	})
}

func TestNewCommandFromDocRejectsEmptyDocument(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		docPath := filepath.Join(root, "empty.md")
		require.NoError(t, os.WriteFile(docPath, []byte(" \n\t\n"), 0o644))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--from-doc", docPath, "example change"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "document is empty")
	})
}

func TestNewCommandInteractivePromptShowsProjectContext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/interactive\n\ngo 1.25.5\n"), 0o644))
		runGit(t, root, "add", "go.mod")
		runGit(t, root, "commit", "-m", "seed project context")

		oldStdin := newCommandStdin
		oldIsTerminal := newCommandIsTerminal
		defer func() {
			newCommandStdin = oldStdin
			newCommandIsTerminal = oldIsTerminal
		}()

		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		defer reader.Close()

		_, err = writer.WriteString("fix login timeout\n")
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		newCommandStdin = reader
		newCommandIsTerminal = func(fd int) bool { return true }

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{})
		require.NoError(t, cmd.Execute())

		output := buf.String()
		assert.Contains(t, output, "Project context (auto-detected):")
		assert.Contains(t, output, "Tech Stack: Go")
		assert.Contains(t, output, "Languages:  Go")
		assert.Contains(t, output, "Recent work:")
		assert.Contains(t, output, "What change do you want to make?")
	})
}

func TestNewCommandInteractiveSafeDegradeStillRequiresPresetConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		oldStdin := newCommandStdin
		oldIsTerminal := newCommandIsTerminal
		defer func() {
			newCommandStdin = oldStdin
			newCommandIsTerminal = oldIsTerminal
		}()

		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		defer reader.Close()

		_, err = writer.WriteString("fix login timeout\n")
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		newCommandStdin = reader
		newCommandIsTerminal = func(fd int) bool { return true }

		classifier := &recordingIntentClassifier{err: assert.AnError}

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
		cmd.SetArgs([]string{})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.WorkflowPresetConfirmationPending())
		assert.Equal(t, model.WorkflowPresetStandard, change.SuggestedWorkflowPreset)
		assert.Contains(t, buf.String(), "intent inference degraded; safe fallback applied")
	})
}

func TestNewCommandInteractiveWithoutClassifierSafeDegradesAndRequiresPresetConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		oldStdin := newCommandStdin
		oldIsTerminal := newCommandIsTerminal
		defer func() {
			newCommandStdin = oldStdin
			newCommandIsTerminal = oldIsTerminal
		}()

		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		defer reader.Close()

		_, err = writer.WriteString("fix login timeout\n")
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		newCommandStdin = reader
		newCommandIsTerminal = func(fd int) bool { return true }

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.WorkflowPresetConfirmationPending())
		assert.Equal(t, model.WorkflowPresetStandard, change.SuggestedWorkflowPreset)
		assert.True(t, change.NeedsDiscovery)
		assert.Equal(t, "complex", change.ComplexityLevel)
		assert.Empty(t, change.GuardrailDomain)
		assert.Contains(t, buf.String(), "intent inference degraded; safe fallback applied")
	})
}

func TestNewJSONStdinPersistsProjectContextAndCallerControlOverrides(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		oldStdin := newCommandStdin
		oldIsTerminal := newCommandIsTerminal
		defer func() {
			newCommandStdin = oldStdin
			newCommandIsTerminal = oldIsTerminal
		}()

		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		defer reader.Close()

		_, err = writer.WriteString(`{
  "description":"fix login timeout",
  "tech_stack":"TypeScript, React",
  "conventions":"Keep CLI responses deterministic",
  "test_cmd":"pnpm test",
  "build_cmd":"pnpm build",
  "languages":["ts","tsx"],
  "recent_work":"migrated auth screens to app router",
  "disabled_controls":["research"],
  "control_modes":{"independent-review":"advisory"},
  "independent_review_blast_radius":"medium",
  "worktree_blast_radius":"low"
}`)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		newCommandStdin = reader
		newCommandIsTerminal = func(fd int) bool { return false }

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		var payload createOutput
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
		require.NotNil(t, payload.ProjectContext)
		assert.Equal(t, "TypeScript, React", payload.ProjectContext.TechStack)
		assert.Equal(t, "Keep CLI responses deterministic", payload.ProjectContext.Conventions)
		assert.Equal(t, "pnpm test", payload.ProjectContext.TestCmd)
		assert.Equal(t, "pnpm build", payload.ProjectContext.BuildCmd)
		assert.Equal(t, "migrated auth screens to app router", payload.ProjectContext.RecentWork)

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, "TypeScript, React", change.ProjectContext.TechStack)
		assert.Equal(t, "Keep CLI responses deterministic", change.ProjectContext.Conventions)
		assert.Equal(t, "pnpm test", change.ProjectContext.TestCmd)
		assert.Equal(t, "pnpm build", change.ProjectContext.BuildCmd)
		assert.Equal(t, []string{"ts", "tsx"}, change.ProjectContext.Languages)
		assert.Equal(t, "migrated auth screens to app router", change.ProjectContext.RecentWork)
		assert.Contains(t, change.CallerDisabledCtrls, model.ControlResearch)
		assert.Equal(t, model.ControlModeAdvisory, change.CallerControlModes[model.ControlIndependentReview])
		assert.Equal(t, model.SignalLevelMedium, change.CallerIndependentReviewBlastRadius)
		assert.Equal(t, model.SignalLevelLow, change.CallerWorktreeBlastRadius)

		policy, err := governance.ResolvePresetPolicy(root, change)
		require.NoError(t, err)
		require.NotNil(t, policy.Overrides)
		assert.Contains(t, policy.Overrides.DisabledControls, model.ControlResearch)
		assert.Equal(t, model.ControlModeAdvisory, policy.Overrides.ModeOverrides[model.ControlIndependentReview])
		assert.Equal(t, model.SignalLevelMedium, policy.Overrides.IndependentReviewBlastRadius)
		assert.Equal(t, model.SignalLevelLow, policy.Overrides.WorktreeBlastRadius)
	})
}

func TestNewJSONModeDoesNotInferProjectContextFromRepo(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/noinfer\n\ngo 1.25.5\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte("test:\n\tgo test ./...\n\nbuild:\n\tgo build ./...\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("Prefer Go table tests.\n"), 0o644))
		runGit(t, root, "add", "go.mod", "Makefile", "CLAUDE.md")
		runGit(t, root, "commit", "-m", "seed inferred context candidates")

		oldStdin := newCommandStdin
		oldIsTerminal := newCommandIsTerminal
		defer func() {
			newCommandStdin = oldStdin
			newCommandIsTerminal = oldIsTerminal
		}()

		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		defer reader.Close()

		_, err = writer.WriteString(`{"description":"fix login timeout"}`)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		newCommandStdin = reader
		newCommandIsTerminal = func(fd int) bool { return false }

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		var payload createOutput
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
		assert.Nil(t, payload.ProjectContext)

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.ProjectContext.IsZero())
	})
}

func TestNewJSONWithoutClassifierReportsSafeDegradeDefaults(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		oldStdin := newCommandStdin
		oldIsTerminal := newCommandIsTerminal
		defer func() {
			newCommandStdin = oldStdin
			newCommandIsTerminal = oldIsTerminal
		}()

		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		defer reader.Close()
		require.NoError(t, writer.Close())

		newCommandStdin = reader
		newCommandIsTerminal = func(fd int) bool { return false }

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		var payloadMap map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payloadMap))
		assert.Equal(t, true, payloadMap["intent_inference_degraded"])
		assert.Equal(t, "no_classifier", payloadMap["intent_inference_degradation_reason"])
		_, hasLegacyDegradeReason := payloadMap["intent_degrade_reason"]
		assert.False(t, hasLegacyDegradeReason)

		var payload createOutput
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
		assert.True(t, payload.IntentInferenceDegraded)
		assert.Equal(t, "no_classifier", payload.IntentInferenceDegradationReason)
		assert.True(t, payload.NeedsDiscovery)
		assert.Equal(t, "complex", payload.ComplexityLevel)
		assert.Empty(t, payload.GuardrailDomain)
		assert.Nil(t, payload.ProjectContext)

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.NeedsDiscovery)
		assert.Equal(t, "complex", change.ComplexityLevel)
		assert.Empty(t, change.GuardrailDomain)
	})
}

func TestPresetUsesPersistedProjectContextFromNewJSONInput(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/presetctx\n\ngo 1.25.5\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte("test:\n\tgo test ./...\n\nbuild:\n\tgo build ./...\n"), 0o644))

		oldStdin := newCommandStdin
		oldIsTerminal := newCommandIsTerminal
		defer func() {
			newCommandStdin = oldStdin
			newCommandIsTerminal = oldIsTerminal
		}()

		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		defer reader.Close()

		_, err = writer.WriteString(`{
  "description":"seed preset with caller project context",
  "tech_stack":"TypeScript, Next.js",
  "conventions":"Prefer route handlers over bespoke API wrappers",
  "test_cmd":"pnpm test",
  "build_cmd":"pnpm build",
  "languages":["ts","tsx"]
}`)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		newCommandStdin = reader
		newCommandIsTerminal = func(fd int) bool { return false }

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		presetCmd := makePresetCmd()
		presetCmd.SetArgs([]string{"light"})
		require.NoError(t, presetCmd.Execute())

		requirementsPath := filepath.Join(root, "artifacts", "changes", slug, "requirements.md")
		raw, err := os.ReadFile(requirementsPath)
		require.NoError(t, err)
		content := string(raw)
		assert.Contains(t, content, "TypeScript, Next.js")
		assert.Contains(t, content, "Prefer route handlers over bespoke API wrappers")
		assert.Contains(t, content, "pnpm test")
		assert.Contains(t, content, "pnpm build")
		assert.NotContains(t, content, "go test ./...")
	})
}

func TestRestoreNewPresetAfterScaffoldFailureReturnsCombinedError(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	badRoot := filepath.Join(tmp, "root-file")
	require.NoError(t, os.WriteFile(badRoot, []byte("not-a-directory"), 0o644))

	change := model.NewChange("new-restore-error")
	change.WorkflowPreset = model.WorkflowPresetLight

	scaffoldErr := fmt.Errorf("scaffold failed")
	err := restoreNewPresetAfterScaffoldFailure(badRoot, &change, scaffoldErr)
	require.Error(t, err)
	assert.ErrorIs(t, err, scaffoldErr)
	assert.Contains(t, err.Error(), "restore preset after scaffold failure")
	assert.Empty(t, change.WorkflowPreset)
	assert.Equal(t, model.WorkflowPresetLight, change.SuggestedWorkflowPreset)
}

func TestNewCommandUsesCoreSchemaForSimpleChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		classifier := &recordingIntentClassifier{
			classification: progression.IntentClassification{
				GuardrailDomain: "",
				NeedsDiscovery:  false,
				Complexity:      "simple",
			},
		}

		cmd := makeNewCmd()
		cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
		cmd.SetArgs([]string{"fix login timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.False(t, change.NeedsDiscovery)
		assert.Equal(t, model.ArtifactSchemaCore, change.ArtifactSchema)
	})
}

func TestNewCommandSafeDegradeKeepsPendingPresetInJSONMode(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		classifier := &recordingIntentClassifier{err: assert.AnError}

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
		cmd.SetArgs([]string{"--json", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
		assert.Equal(t, true, payload["preset_confirmation_pending"])
		assert.Equal(t, "standard", payload["suggested_workflow_preset"])
		assert.Equal(t, "complex", payload["complexity_level"])
		assert.Equal(t, true, payload["needs_discovery"])

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.WorkflowPresetConfirmationPending())
		assert.Equal(t, "complex", change.ComplexityLevel)
		assert.True(t, change.NeedsDiscovery)
		assert.Empty(t, change.GuardrailDomain)
	})
}

func TestNewCommandAutoPromotesDiscoveryForGuardrails(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"refactor auth policy enforcement"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.NeedsDiscovery)
		assert.Equal(t, model.ArtifactSchemaExpanded, change.ArtifactSchema)
	})
}

func TestNewCommandDiscussPersistsQualityMode(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--discuss", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.Equal(t, model.QualityModeDiscuss, change.QualityMode)

		changeRaw, err := os.ReadFile(state.BundleChangeFilePath(root, slug))
		require.NoError(t, err)
		assert.NotContains(t, string(changeRaw), "entry_surface:")
		assert.Contains(t, string(changeRaw), "quality_mode: discuss")
	})
}

func TestNewCommandFullPersistsFullQualityMode(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--full", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.QualityModeFull, change.QualityMode)
	})
}

func TestNewCommandDiscussAndFullAreComposable(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--discuss", "--full", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		// --full wins over --discuss when both are set
		assert.Equal(t, model.QualityModeFull, change.QualityMode)
		assert.Equal(t, "full", payload["quality_mode"])
	})
}

func TestNewCommandPresetPersistsConfirmedWorkflowPreset(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var buf bytes.Buffer
		cmd := makeNewCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--preset", "light", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.Equal(t, model.WorkflowPresetLight, change.WorkflowPreset)
		assert.Empty(t, change.SuggestedWorkflowPreset)
		assert.Equal(t, "light", payload["workflow_preset"])
	})
}

func TestNewCommandWithoutPresetAutoConfirmsLowRiskChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		classifier := &recordingIntentClassifier{
			classification: progression.IntentClassification{
				GuardrailDomain: "",
				NeedsDiscovery:  false,
				Complexity:      "simple",
			},
		}

		cmd := makeNewCmd()
		cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
		cmd.SetArgs([]string{"fix login timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Low-risk changes (no guardrail domain, no discovery, suggestion=light)
		// are now auto-confirmed by Track 1A.
		assert.False(t, change.WorkflowPresetConfirmationPending())
		assert.Equal(t, model.WorkflowPresetLight, change.WorkflowPreset)
		assert.Empty(t, change.SuggestedWorkflowPreset)
	})
}

func TestNewCommandWithoutPresetDoesNotAutoConfirmWhenMinPresetConfigured(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfgPath := state.ConfigPath(root)
		cfg, err := model.LoadConfig(cfgPath)
		require.NoError(t, err)
		cfg.Governance.MinPreset = model.WorkflowPresetLight
		require.NoError(t, model.SaveConfig(cfgPath, cfg))

		classifier := &recordingIntentClassifier{
			classification: progression.IntentClassification{
				GuardrailDomain: "",
				NeedsDiscovery:  false,
				Complexity:      "simple",
			},
		}

		cmd := makeNewCmd()
		cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
		cmd.SetArgs([]string{"fix login timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.True(t, change.WorkflowPresetConfirmationPending(),
			"presence of min_preset must keep preset confirmation explicit even when suggestion stays light")
		assert.Equal(t, model.WorkflowPresetLight, change.SuggestedWorkflowPreset)
		assert.Empty(t, change.WorkflowPreset)
	})
}

func TestNewCommandExplicitLightScaffoldsAssuranceWhenMinPresetUpgradesEffectivePreset(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfgPath := state.ConfigPath(root)
		cfg, err := model.LoadConfig(cfgPath)
		require.NoError(t, err)
		cfg.Governance.MinPreset = model.WorkflowPresetStandard
		require.NoError(t, model.SaveConfig(cfgPath, cfg))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"--preset", "light", "fix login timeout"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		assert.FileExists(t, filepath.Join(root, "artifacts", "changes", slug, "assurance.md"),
			"scaffold must honor effective preset, not just confirmed light preset")
	})
}

func TestNewCommandWithoutPresetCreatesPendingConfirmationForGuardrailDomain(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"update auth middleware timeout strategy"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.True(t, change.WorkflowPresetConfirmationPending())
		assert.True(t, change.SuggestedWorkflowPreset.IsValid())
		assert.Empty(t, change.WorkflowPreset)
		assert.NoFileExists(t, filepath.Join(root, "artifacts", "changes", slug, "decision.md"))
		assert.NoFileExists(t, filepath.Join(root, "artifacts", "changes", slug, "tasks.md"))
		assert.NoFileExists(t, filepath.Join(root, "artifacts", "changes", slug, "assurance.md"))
	})
}

func TestNewCommandPendingPresetStillScaffoldsIntentMD(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"update auth middleware timeout strategy"})
		require.NoError(t, cmd.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.True(t, change.WorkflowPresetConfirmationPending(),
			"preset should be pending for guardrail domain change")
		// intent.md must exist even when preset is pending — it is the
		// primary S0_INTAKE artifact.
		intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		assert.FileExists(t, intentPath, "intent.md must be scaffolded for S0_INTAKE even with pending preset")

		// Verify intent.md contains the description and complexity assessment
		data, err := os.ReadFile(intentPath)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "update auth middleware timeout strategy",
			"intent.md should contain the change description")
		assert.Contains(t, content, "## Complexity Assessment",
			"intent.md should have Complexity Assessment section")
	})
}

func TestNewCommandRejectsWhenActiveChangeAlreadyExists(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		existing := model.NewChange("existing-change")
		require.NoError(t, state.SaveChange(root, existing))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"follow-up change"})
		err := cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "active_change_exists", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "creating a new change")
	})
}

func TestNewCommandRejectsWhenHiddenBoundWorktreeActiveChangeExists(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		runGit(t, root, "config", "user.email", "test@example.com")
		runGit(t, root, "config", "user.name", "Test User")
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")

		slug := createGovernedRequest(t, root, "L3", "existing hidden bound worktree change")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		require.NoError(t, os.Remove(state.ConfigPath(worktreeRoot)))
		require.NoError(t, os.Remove(state.WorkspaceScopeMarkerPath(worktreeRoot)))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"follow-up change"})
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "active_change_exists", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "creating a new change")
	})
}

func TestNewHelpDoesNotMentionLevelOrPlanOnly(t *testing.T) {
	t.Parallel()

	cmd := makeNewCmd()
	assert.NotContains(t, cmd.Long, "--level")
	assert.NotContains(t, cmd.Long, "--plan-only")
	assert.Nil(t, cmd.Flags().Lookup("level"))
	assert.Nil(t, cmd.Flags().Lookup("plan-only"))
}

func TestNextAfterNewUsesDirectSetupState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetContext(withIntentClassifierContext(create.Context(), &recordingIntentClassifier{
			classification: progression.IntentClassification{
				GuardrailDomain: "",
				NeedsDiscovery:  false,
				Complexity:      "simple",
			},
		}))
		create.SetArgs([]string{"--preset", "standard", "fix login timeout"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		var buf bytes.Buffer
		next := makeNextCmd()
		next.SetOut(&buf)
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, model.StateS0Intake, view.CurrentState)
		assert.Equal(t, "fix login timeout", view.InputContext.Description)
		// At S0_INTAKE, next is blocked by intake clarification requirements.
		// Verify no stale intake-classify or routing-era blockers appear.
		for _, b := range view.Blockers {
			assert.NotContains(t, b.Code, "intake-classify", "no intake-classify blocker should appear after cutover")
			assert.NotContains(t, b.Detail, "intake-classify", "no intake-classify blocker should appear after cutover")
			assert.NotContains(t, b.Code, "route_snapshot", "no route_snapshot blocker should appear after cutover")
			assert.NotContains(t, b.Detail, "route_snapshot", "no route_snapshot blocker should appear after cutover")
		}

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS0Intake, change.CurrentState)
		assert.False(t, change.NeedsDiscovery)
	})
}

func TestNextAfterNewWithoutPresetBlocksOnPresetConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth session validation"})
		require.NoError(t, create.Execute())

		var buf bytes.Buffer
		next := makeNextCmd()
		next.SetOut(&buf)
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.ReasonCodesFromSpecs([]string{"preset_confirmation_required"}), view.Blockers,
			"pending preset must produce exactly one blocker, no downstream leakage")
		assert.Nil(t, view.NextSkill,
			"next_skill must be nil when preset is pending")
	})
}

func TestNextPreviewAfterNewWithoutPresetBlocksOnPresetConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth session validation"})
		require.NoError(t, create.Execute())

		var buf bytes.Buffer
		next := makeNextCmd()
		next.SetOut(&buf)
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.ReasonCodesFromSpecs([]string{"preset_confirmation_required"}), view.Blockers,
			"next must surface exactly preset_confirmation_required, no downstream leakage")
		assert.Nil(t, view.NextSkill,
			"next_skill must be nil when preset is pending")
	})
}

func TestValidateAfterNewWithoutPresetShowsPendingConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth session validation"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		assert.True(t, view.PresetConfirmationPending)
		assert.NotEmpty(t, view.SuggestedWorkflowPreset)
		assert.Equal(t, model.ReasonCodesFromSpecs([]string{"preset_confirmation_required"}), view.Blockers,
			"pending preset must produce exactly one blocker, no downstream leakage")
	})
}

func TestNextPendingPresetDoesNotLeakArtifactStatusOrMutateChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth session validation"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		require.NoError(t, os.WriteFile(intentPath, []byte(`# Intent

## Summary
update auth session validation

## Complexity Assessment
critical

## Guardrail Domains
auth_authz

## In Scope
- tighten auth session validation rules

## Out of Scope
- unrelated login UI changes

## Constraints
- no external API contract changes

## Acceptance Signals
- session validation failures are rejected deterministically

## Open Questions
<!-- none -->

## Deferred Ideas
<!-- none -->

## Approved Summary
<!-- pending preset confirmation -->
`), 0o644))
		writeSkillVerification(t, root, slug, progression.SkillIntakeClarification, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 0,
		})

		// Snapshot change.yaml before next.
		before, err := os.ReadFile(state.BundleChangeFilePath(root, slug))
		require.NoError(t, err)

		// Run next (query-only) — should NOT mutate change.yaml.
		var buf bytes.Buffer
		next := makeNextCmd()
		next.SetOut(&buf)
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		after, err := os.ReadFile(state.BundleChangeFilePath(root, slug))
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after),
			"change.yaml must not be modified when next is blocked by pending preset")

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
		if adv, ok := payload["advanced"].(map[string]any); ok {
			assert.Equal(t, "query", adv["action"],
				"pending preset must keep next read-only while surfacing the confirmation blocker")
		}
		assert.Equal(t, string(model.StateS0Intake), payload["current_state"])
		assert.Equal(t, string(model.IntakeSubStepClarify), payload["intake_substep"])
		input, ok := payload["input_context"].(map[string]any)
		require.True(t, ok)
		_, hasArtifactStatus := input["artifact_status"]
		assert.False(t, hasArtifactStatus,
			"artifact_status must not appear in next JSON when preset is pending")
		_, hasArtifactBundle := input["artifact_bundle"]
		assert.False(t, hasArtifactBundle,
			"artifact_bundle must not appear in next JSON when preset is pending")

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS0Intake, reloaded.CurrentState)
		assert.Equal(t, model.IntakeSubStepClarify, reloaded.IntakeSubStep)
	})
}

func TestNextPreviewPendingPresetDoesNotLeakArtifactStatus(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth session validation"})
		require.NoError(t, create.Execute())

		var buf bytes.Buffer
		next := makeNextCmd()
		next.SetOut(&buf)
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
		input, ok := payload["input_context"].(map[string]any)
		require.True(t, ok)
		_, hasArtifactStatus := input["artifact_status"]
		assert.False(t, hasArtifactStatus,
			"artifact_status must not appear in next JSON when preset is pending")
	})
}

func TestStatusPendingPresetMinimalView(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth session validation"})
		require.NoError(t, create.Execute())

		var buf bytes.Buffer
		status := makeStatusCmd()
		status.SetOut(&buf)
		status.SetArgs([]string{"--json"})
		require.NoError(t, status.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		// Must NOT contain downstream bundle fields.
		_, hasProgress := payload["progress"]
		assert.False(t, hasProgress,
			"progress must not appear in status JSON when preset is pending")
		_, hasArtifactDAG := payload["artifact_dag"]
		assert.False(t, hasArtifactDAG,
			"artifact_dag must not appear in status JSON when preset is pending")
		_, hasSourceState := payload["source_state_file"]
		assert.False(t, hasSourceState,
			"source_state_file must not appear in status JSON when preset is pending")

		// next_ready_actions must contain only preset, not next/cancel.
		actions, ok := payload["next_ready_actions"].([]any)
		require.True(t, ok, "next_ready_actions must be present")
		for _, a := range actions {
			s, _ := a.(string)
			assert.NotEqual(t, "next", s,
				"next must not appear in next_ready_actions when preset is pending")
			assert.NotEqual(t, "cancel", s,
				"cancel must not appear in next_ready_actions when preset is pending")
		}
	})
}

func TestNextPromotesCoreConfigDefaultToExpandedWhenDiscoveryIsRequired(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfgPath := state.ConfigPath(root)
		cfg, err := model.LoadConfig(cfgPath)
		require.NoError(t, err)
		cfg.Defaults.ArtifactSchema = model.ArtifactSchemaCore
		require.NoError(t, model.SaveConfig(cfgPath, cfg))

		create := makeNewCmd()
		create.SetArgs([]string{"update auth middleware timeout strategy"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		var buf bytes.Buffer
		next := makeNextCmd()
		next.SetOut(&buf)
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, change.NeedsDiscovery)
		assert.Equal(t, model.ArtifactSchemaExpanded, change.ArtifactSchema)
	})
}

func TestDiscoveryPathNextBlocksOnPresetConfirmationBeforeArtifacts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Create a discovery-path change (guardrail domain inferred from description).
		create := makeNewCmd()
		create.SetArgs([]string{"update auth middleware timeout strategy"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))

		// Run next without confirming preset.
		var buf bytes.Buffer
		next := makeNextCmd()
		next.SetOut(&buf)
		next.SetArgs([]string{"--json"})
		require.NoError(t, next.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.ReasonCodesFromSpecs([]string{"preset_confirmation_required"}), view.Blockers,
			"discovery path must show only preset_confirmation_required, no worktree or skill leakage")
		assert.Nil(t, view.NextSkill,
			"next_skill must be nil when preset is pending on discovery path")

		// Verify no research.md was authored while preset is pending.
		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		_, err := os.Stat(filepath.Join(bundleDir, "research.md"))
		assert.True(t, os.IsNotExist(err),
			"research.md must not be authored before preset confirmation")
	})
}

func TestDiscoveryPathStatusBlocksOnPresetConfirmation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth middleware timeout strategy"})
		require.NoError(t, create.Execute())

		var buf bytes.Buffer
		status := makeStatusCmd()
		status.SetOut(&buf)
		status.SetArgs([]string{"--json"})
		require.NoError(t, status.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.ReasonCodesFromSpecs([]string{"preset_confirmation_required"}), view.Blockers,
			"status must show only preset_confirmation_required, no worktree or skill leakage")
	})
}

func withWorkspace(t *testing.T, root string, fn func()) {
	t.Helper()
	ensureTestGitRepo(t, root)
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	defer func() {
		_ = os.Chdir(previousWD)
	}()
	fn()
}

// initTestWorkspace wraps bootstrap.InitWorkspace and seeds a default claude
// tool adapter so that ResolveWorkspaceTool succeeds in tests.
func initTestWorkspace(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
	seedTestToolAdapter(t, root)
}

// seedTestToolAdapter writes a minimal claude adapter sentinel so that
// toolgen.ResolveWorkspaceTool resolves successfully in test workspaces.
func seedTestToolAdapter(t *testing.T, root string) {
	t.Helper()
	markerPath := filepath.Join(root, ".claude", "slipway", ".adapter-generated")
	require.NoError(t, os.MkdirAll(filepath.Dir(markerPath), 0o755))
	require.NoError(t, os.WriteFile(markerPath, []byte("test marker"), 0o644))
}

// ensureTestGitRepo initializes a bare-minimum git repo if one doesn't exist.
// Idempotent — safe to call multiple times on the same root.
func ensureTestGitRepo(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return
	}
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
}

func singleChangeSlug(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			if e.Name() == "archived" {
				continue
			}
			dirs = append(dirs, e)
		}
	}
	require.Len(t, dirs, 1)
	return dirs[0].Name()
}

func TestSuggestWorkflowPreset_RespectsMinPreset(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Set min_preset to strict in config.
		cfgPath := state.ConfigPath(root)
		cfg, err := model.LoadConfig(cfgPath)
		require.NoError(t, err)
		cfg.Governance.MinPreset = model.WorkflowPresetStrict
		require.NoError(t, model.SaveConfig(cfgPath, cfg))

		// Create a simple change (no guardrail domain, no discovery).
		create := makeNewCmd()
		create.SetArgs([]string{"fix typo"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// The suggestion must be at least strict (the project minimum).
		assert.Equal(t, model.WorkflowPresetStrict, change.SuggestedWorkflowPreset,
			"AI suggestion must respect project min_preset")
	})
}

func TestNewCommandWithMalformedConfigFailsClosed(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Write a corrupt .slipway.yaml.
		cfgPath := state.ConfigPath(root)
		require.NoError(t, os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0o644))

		create := makeNewCmd()
		create.SetArgs([]string{"fix typo"})
		err := create.Execute()
		require.Error(t, err, "malformed .slipway.yaml must cause a fail-closed error, not silent default fallback")
		assert.Contains(t, err.Error(), ".slipway.yaml")
	})
}

func TestNewCommandFailsClosedWhenSlugNamespaceIsCorrupt(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := model.SlugifyTitle("fix login timeout")
		corruptBundleDir := filepath.Join(state.ActiveBundlesDir(root), slug)
		require.NoError(t, os.MkdirAll(corruptBundleDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(corruptBundleDir, "change.yaml"), []byte("version: ["), 0o644))

		cmd := makeNewCmd()
		cmd.SetArgs([]string{"fix login timeout"})
		err := cmd.Execute()
		require.Error(t, err, "corrupt slug namespace must fail closed instead of silently reusing the slug")
		assert.Contains(t, err.Error(), slug)
	})
}

func TestSuggestWorkflowPreset_DefaultPresetDoesNotBypassGuardrailFloor(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Set default_preset to light.
		cfgPath := state.ConfigPath(root)
		cfg, err := model.LoadConfig(cfgPath)
		require.NoError(t, err)
		cfg.Governance.DefaultPreset = model.WorkflowPresetLight
		require.NoError(t, model.SaveConfig(cfgPath, cfg))

		// Create a change that infers a guardrail domain.
		create := makeNewCmd()
		create.SetArgs([]string{"refactor auth middleware credential handling"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// The suggestion must be at least standard due to guardrail domain.
		assert.True(t, change.SuggestedWorkflowPreset.Rank() >= model.WorkflowPresetStandard.Rank(),
			"AI suggestion must not suggest light for guardrail-bound change even when default_preset=light, got %s",
			change.SuggestedWorkflowPreset)
	})
}

func TestStatusPendingPresetShowsPresetHintNotNext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"update auth session validation"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.True(t, change.WorkflowPresetConfirmationPending())

		view, err := buildStatusViewFromChange(root, change)
		require.NoError(t, err)

		hint := primaryActionHint(view)
		assert.Contains(t, hint, "slipway preset",
			"pending confirmation should steer user to preset command, not next")
		assert.NotEqual(t, "slipway next  (planning phase)", hint,
			"should not show generic next hint when preset is pending")
	})
}
