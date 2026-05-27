package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/intake"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type createOutput struct {
	Mode                             string                `json:"mode"`
	Slug                             string                `json:"slug"`
	Description                      string                `json:"description"`
	QualityMode                      string                `json:"quality_mode,omitempty"`
	WorkflowProfile                  string                `json:"workflow_profile,omitempty"`
	WorkflowPreset                   string                `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset          string                `json:"suggested_workflow_preset,omitempty"`
	PresetConfirmationPending        bool                  `json:"preset_confirmation_pending,omitempty"`
	CurrentState                     string                `json:"current_state"`
	IntakeSubStep                    string                `json:"intake_substep,omitempty"`
	PlanSubStep                      string                `json:"plan_substep,omitempty"`
	Phase                            model.UserPhase       `json:"phase"`
	NeedsDiscovery                   bool                  `json:"needs_discovery"`
	ComplexityLevel                  string                `json:"complexity_level,omitempty"`
	GuardrailDomain                  string                `json:"guardrail_domain,omitempty"`
	ArtifactSchema                   string                `json:"artifact_schema"`
	WorkflowCreated                  bool                  `json:"workflow_created"`
	WorktreePath                     string                `json:"worktree_path,omitempty"`
	WorktreeBranch                   string                `json:"worktree_branch,omitempty"`
	WorktreeCreated                  bool                  `json:"worktree_created,omitempty"`
	WorktreeSkippedReason            string                `json:"worktree_skipped_reason,omitempty"`
	ProjectContext                   *projectContextOutput `json:"project_context,omitempty"`
	IntentInferenceDegraded          bool                  `json:"intent_inference_degraded,omitempty"`
	IntentInferenceDegradationReason string                `json:"intent_inference_degradation_reason,omitempty"`
}

type projectContextOutput struct {
	TechStack   string `json:"tech_stack,omitempty"`
	Conventions string `json:"conventions,omitempty"`
	Languages   string `json:"languages,omitempty"`
	TestCmd     string `json:"test_cmd,omitempty"`
	BuildCmd    string `json:"build_cmd,omitempty"`
	RecentWork  string `json:"recent_work,omitempty"`
}

const generateUniqueChangeSlugMaxAttempts = 10000

var (
	newCommandStdin      = os.Stdin
	newCommandIsTerminal = term.IsTerminal
)

type stdinClassificationInput struct {
	Description     string  `json:"description"`
	GuardrailDomain *string `json:"guardrail_domain,omitempty"`
	NeedsDiscovery  *bool   `json:"needs_discovery,omitempty"`
	Complexity      *string `json:"complexity,omitempty"`
	WorkflowProfile *string `json:"workflow_profile,omitempty"`

	TechStack   *string  `json:"tech_stack,omitempty"`
	Conventions *string  `json:"conventions,omitempty"`
	TestCmd     *string  `json:"test_cmd,omitempty"`
	BuildCmd    *string  `json:"build_cmd,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	RecentWork  *string  `json:"recent_work,omitempty"`

	DisabledControls             []string          `json:"disabled_controls,omitempty"`
	ControlModes                 map[string]string `json:"control_modes,omitempty"`
	IndependentReviewBlastRadius *string           `json:"independent_review_blast_radius,omitempty"`
	WorktreeBlastRadius          *string           `json:"worktree_blast_radius,omitempty"`
}

type stdinIntentClassifier struct {
	classification progression.IntentClassification
}

func (s stdinIntentClassifier) Classify(_ context.Context, _ string) (progression.IntentClassification, error) {
	return s.classification, nil
}

type intentClassifierContextKey struct{}
type stdinInputContextKey struct{}

func withIntentClassifierContext(ctx context.Context, classifier progression.IntentClassifier) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, intentClassifierContextKey{}, classifier)
}

func withStdinInputContext(ctx context.Context, input *stdinClassificationInput) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, stdinInputContextKey{}, input)
}

func intentClassifierFromContext(cmd *cobra.Command) progression.IntentClassifier {
	if cmd != nil {
		if classifier, ok := cmd.Context().Value(intentClassifierContextKey{}).(progression.IntentClassifier); ok {
			return classifier
		}
	}
	return nil
}

func stdinInputFromContext(cmd *cobra.Command) *stdinClassificationInput {
	if cmd != nil {
		if input, ok := cmd.Context().Value(stdinInputContextKey{}).(*stdinClassificationInput); ok {
			return input
		}
	}
	return nil
}

func makeNewCmd() *cobra.Command {
	var preset string
	var discuss bool
	var full bool
	var trivial bool
	var fromDoc string
	var workflowProfile string

	cmd := &cobra.Command{
		Use:   "new [description]",
		Short: desc("new"),
		Long: `Create a governed change starting at S0_INTAKE.

This command creates a governed change and begins the intake-first workflow.
Project context is auto-detected from lockfiles, scripts, and recent work for
human-oriented CLI prompts. Lifecycle classification defaults to conservative
safe-degrade values unless JSON stdin supplies explicit classification metadata.

If no description is provided and stdin is a terminal, an interactive prompt
displays inferred project context and asks for the description.

Use --trivial to force complexity=trivial and minimize intake depth.
Use --from-doc to seed intake from an existing document (PRD, RFC, etc.).
Use --discuss to persist unresolved gray areas into context before execution.
Use --full to require refreshed final-closeout evidence before ship.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qualityMode := model.QualityModeStandard
			if discuss {
				qualityMode = model.QualityModeDiscuss
			}
			if full {
				qualityMode = model.QualityModeFull
			}
			workflowPreset, err := parseWorkflowPreset(preset)
			if err != nil {
				return err
			}

			var description string
			if len(args) > 0 {
				description = strings.TrimSpace(args[0])
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")

			// JSON mode: read structured input from stdin for description
			// and optional intent classification.
			if jsonFlag && !newCommandIsTerminal(int(newCommandStdin.Fd())) {
				stdinInput, stdinErr := readStdinClassificationInput()
				if stdinErr != nil && stdinErr.Error() != "stdin is empty" {
					return newInvalidUsageError(
						"stdin_parse_error",
						fmt.Sprintf("failed to parse JSON stdin: %s", stdinErr),
						"Provide valid JSON on stdin or omit stdin entirely.",
						nil,
					)
				}
				if stdinErr == nil {
					if description == "" && strings.TrimSpace(stdinInput.Description) != "" {
						description = strings.TrimSpace(stdinInput.Description)
					}
					if classifier := buildStdinClassifier(stdinInput); classifier != nil {
						cmd.SetContext(withIntentClassifierContext(cmd.Context(), classifier))
					}
					cmd.SetContext(withStdinInputContext(cmd.Context(), &stdinInput))
				}
			}

			if description == "" && fromDoc != "" {
				// --from-doc without description: extract summary from document
				data, err := os.ReadFile(fromDoc)
				if err != nil {
					return newInvalidUsageError(
						"from_doc_read_failed",
						fmt.Sprintf("cannot read document: %s", err),
						"Provide a valid file path with --from-doc.",
						nil,
					)
				}
				description = intake.ExtractSummary(string(data))
				if description == "" {
					return newInvalidUsageError(
						"from_doc_no_summary",
						"could not extract a summary from the document",
						"Provide a description as the first argument or ensure the document has a clear title/summary.",
						nil,
					)
				}
			}

			if description == "" {
				if jsonFlag {
					return newInvalidUsageError(
						"description_required",
						"description required in JSON mode",
						"Provide description as a positional argument or in JSON stdin: {\"description\":\"...\"}",
						nil,
					)
				}
				// Non-TTY without description: error
				if !newCommandIsTerminal(int(newCommandStdin.Fd())) {
					return newInvalidUsageError(
						"description_required",
						"description required in non-interactive mode",
						"Provide a description: slipway new \"your change description\"",
						nil,
					)
				}
				// TTY interactive mode: infer context, prompt for description
				root, err := projectRootFromCommand(cmd)
				if err != nil {
					return err
				}
				projectCtx := progression.InferProjectContext(root)
				prompt := intake.BuildInteractivePromptPayload(root, projectCtx)
				fmt.Fprintln(cmd.OutOrStdout(), prompt.Header)
				for _, line := range prompt.Lines {
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprint(cmd.OutOrStdout(), prompt.Question)
				scanner := bufio.NewScanner(newCommandStdin)
				if scanner.Scan() {
					description = strings.TrimSpace(scanner.Text())
				}
				if description == "" {
					return newInvalidUsageError(
						"description_required",
						"description required",
						"Provide a description: slipway new \"your change description\"",
						nil,
					)
				}
			}

			profileValue := workflowProfile
			if strings.TrimSpace(profileValue) == "" {
				if si := stdinInputFromContext(cmd); si != nil && si.WorkflowProfile != nil {
					profileValue = *si.WorkflowProfile
				}
			}
			parsedWorkflowProfile, err := parseWorkflowProfile(profileValue)
			if err != nil {
				return err
			}

			return runNewCommand(cmd, description, qualityMode, parsedWorkflowProfile, workflowPreset, trivial, fromDoc)
		},
	}

	cmd.Flags().StringVar(&preset, "preset", "", "Governance preset: light|standard|strict")
	cmd.Flags().BoolVar(&discuss, "discuss", false, "Persist unresolved gray areas into context before execution")
	cmd.Flags().BoolVar(&full, "full", false, "Require refreshed final-closeout evidence before ship")
	cmd.Flags().BoolVar(&trivial, "trivial", false, "Force complexity=trivial, minimize intake depth")
	cmd.Flags().StringVar(&fromDoc, "from-doc", "", "Seed intake from a document path (PRD, RFC, etc.)")
	cmd.Flags().StringVar(&workflowProfile, "profile", "", "Workflow profile: code|docs|research|config|meta")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func runNewCommand(
	cmd *cobra.Command,
	description string,
	qualityMode model.QualityMode,
	workflowProfile model.WorkflowProfile,
	workflowPreset model.WorkflowPreset,
	trivial bool,
	fromDoc string,
) error {
	return createDirectGovernedChange(cmd, description, qualityMode, workflowProfile, workflowPreset, trivial, fromDoc)
}

// createDirectGovernedChange is the governed change creation path for `new`.
func createDirectGovernedChange(
	cmd *cobra.Command,
	description string,
	qualityMode model.QualityMode,
	workflowProfile model.WorkflowProfile,
	workflowPreset model.WorkflowPreset,
	trivial bool,
	fromDoc string,
) error {
	if description == "" {
		return newInvalidUsageError(
			"empty_description",
			"description cannot be empty",
			"Provide a non-empty description as the first argument.",
			nil,
		)
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")

	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return err
	}
	if _, err := os.Stat(state.ConfigPath(root)); errorsIsNotExist(err) {
		return newPreconditionError(
			"workspace_not_initialized",
			"workspace is not initialized; run `slipway init`",
			"Run `slipway init` to initialize the workspace.",
			"",
			nil,
		)
	} else if err != nil {
		return err
	}

	// Read --from-doc content for inference and intent.md seeding
	var fromDocContent string
	var extractedFromDoc intake.DocSeed
	if fromDoc != "" {
		data, err := os.ReadFile(fromDoc)
		if err != nil {
			return newInvalidUsageError(
				"from_doc_read_failed",
				fmt.Sprintf("cannot read document: %s", err),
				"Provide a valid file path with --from-doc.",
				nil,
			)
		}
		fromDocContent = strings.TrimSpace(string(data))
		if fromDocContent == "" {
			return newInvalidUsageError(
				"from_doc_empty",
				"document is empty",
				"Provide a non-empty document with --from-doc.",
				nil,
			)
		}
		extractedFromDoc = intake.ParseDoc(fromDocContent)
	}

	// Inference uses description + doc content combined for richer signal
	inferenceText := description
	if fromDocContent != "" {
		inferenceText = description + "\n" + fromDocContent
	}
	setup, inferenceResult := resolveChangeSetup(
		cmd.Context(),
		inferenceText,
		intentClassifierFromContext(cmd),
	)

	// Override complexity if --trivial flag is set
	if trivial {
		setup.ComplexityLevel = "trivial"
	}
	setup.ArtifactSchema = model.DefaultArtifactSchemaForWorkflowProfile(workflowProfile, setup.NeedsDiscovery, setup.ArtifactSchema)

	// Hard cut: JSON callers own project context explicitly. Do not auto-infer
	// repo context on machine-facing surfaces. Auto-detection remains only for
	// human-oriented CLI flows outside --json.
	projectCtx := model.ProjectContext{}
	if jsonFlag {
		if si := stdinInputFromContext(cmd); si != nil {
			mergeStdinProjectContext(&projectCtx, si)
		}
	} else {
		projectCtx = progression.InferProjectContext(root)
	}

	return withChangeCreateLock(root, func() error {
		if _, err := state.EnsureLocalStateGitIgnore(root); err != nil {
			return err
		}
		if err := rejectIfConflictingChange(root); err != nil {
			return err
		}

		slug, err := generateUniqueChangeSlug(description, func(s string) (bool, error) {
			return state.ChangeSlugExists(root, s)
		})
		if err != nil {
			return err
		}

		change := model.NewChange(slug)
		change.Description = description
		change.NeedsDiscovery = setup.NeedsDiscovery
		change.ComplexityLevel = setup.ComplexityLevel
		change.GuardrailDomain = setup.GuardrailDomain
		change.ArtifactSchema = setup.ArtifactSchema
		change.QualityMode = qualityMode
		change.WorkflowProfile = workflowProfile
		if si := stdinInputFromContext(cmd); si != nil {
			change.ProjectContext = projectCtx
			change.CallerDisabledCtrls, err = parseCallerDisabledControls(si.DisabledControls)
			if err != nil {
				return newInvalidUsageError(
					"invalid_disabled_controls",
					err.Error(),
					"Use canonical control IDs for disabled_controls.",
					nil,
				)
			}
			change.CallerControlModes, err = parseCallerControlModes(si.ControlModes)
			if err != nil {
				return newInvalidUsageError(
					"invalid_control_modes",
					err.Error(),
					"Use canonical control IDs and blocking|advisory values for control_modes.",
					nil,
				)
			}
			change.CallerIndependentReviewBlastRadius, err = parseSignalLevelOverride(si.IndependentReviewBlastRadius, "independent_review_blast_radius")
			if err != nil {
				return newInvalidUsageError(
					"invalid_independent_review_blast_radius",
					err.Error(),
					"Use one of: low, medium, high.",
					nil,
				)
			}
			change.CallerWorktreeBlastRadius, err = parseSignalLevelOverride(si.WorktreeBlastRadius, "worktree_blast_radius")
			if err != nil {
				return newInvalidUsageError(
					"invalid_worktree_blast_radius",
					err.Error(),
					"Use one of: low, medium, high.",
					nil,
				)
			}
		}
		if workflowPreset.IsValid() {
			change.WorkflowPreset = workflowPreset
		} else {
			suggested, err := suggestWorkflowPreset(root, setup)
			if err != nil {
				return err
			}
			minPresetConfigured, err := governanceMinPresetConfigured(root)
			if err != nil {
				return err
			}
			if suggested == model.WorkflowPresetLight &&
				setup.GuardrailDomain == "" &&
				!setup.NeedsDiscovery &&
				!minPresetConfigured {
				change.WorkflowPreset = suggested
			} else {
				change.SuggestedWorkflowPreset = suggested
			}
		}

		worktreeBinding, err := state.EnsureDefaultWorktreeForChange(root, &change)
		if err != nil {
			return err
		}
		if strings.TrimSpace(change.WorktreePath) != "" {
			workspaceRoot, err := state.WorkspaceRootForChange(root, change)
			if err != nil {
				return err
			}
			if _, err := state.EnsureLocalStateGitIgnore(workspaceRoot); err != nil {
				return err
			}
		}
		if err := state.SaveChange(root, change); err != nil {
			return err
		}
		// S0_INTAKE owns intent clarification. Defer downstream governed
		// artifacts until S1_PLAN/bundle so they can be seeded from the
		// confirmed intent instead of from empty template placeholders.
		if err := artifact.ScaffoldIntentForChangeWithContext(root, change, projectCtx); err != nil {
			if change.WorkflowPreset.IsValid() {
				return restoreNewPresetAfterScaffoldFailure(root, &change, err)
			}
			return err
		}

		// Post-scaffold: apply --from-doc content to intent.md
		if fromDocContent != "" {
			paths, err := state.ResolveChangePaths(root, change)
			if err != nil {
				return err
			}
			intentPath := filepath.Join(paths.GovernedBundleDir, "intent.md")
			if err := intake.SeedIntentFile(intentPath, fromDocContent, extractedFromDoc); err != nil {
				return err
			}
		}
		if _, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
			Command:      "new",
			EventType:    "change.created",
			Action:       "created",
			Result:       "ok",
			AfterState:   change.CurrentState,
			AfterSubStep: string(change.IntakeSubStep),
			SideEffects: []state.LifecycleSideEffect{
				{Kind: "change_authority_written", Detail: "change.yaml"},
				{Kind: "artifact_scaffolded", Detail: string(change.ArtifactSchema)},
			},
		}); err != nil {
			return err
		}

		var ctxOut *projectContextOutput
		if !projectCtx.IsZero() {
			ctxOut = &projectContextOutput{
				TechStack:   projectCtx.TechStack,
				Conventions: projectCtx.Conventions,
				Languages:   strings.Join(projectCtx.Languages, ", "),
				TestCmd:     projectCtx.TestCmd,
				BuildCmd:    projectCtx.BuildCmd,
				RecentWork:  projectCtx.RecentWork,
			}
		}

		if jsonFlag {
			out := createOutput{
				Mode:                             "governed",
				Slug:                             slug,
				Description:                      description,
				QualityMode:                      string(qualityMode),
				WorkflowProfile:                  string(change.EffectiveWorkflowProfile()),
				WorkflowPreset:                   string(change.WorkflowPreset),
				SuggestedWorkflowPreset:          string(change.SuggestedWorkflowPreset),
				PresetConfirmationPending:        change.WorkflowPresetConfirmationPending(),
				CurrentState:                     string(model.StateS0Intake),
				IntakeSubStep:                    string(change.IntakeSubStep),
				Phase:                            change.Phase(),
				NeedsDiscovery:                   change.NeedsDiscovery,
				ComplexityLevel:                  change.ComplexityLevel,
				GuardrailDomain:                  change.GuardrailDomain,
				ArtifactSchema:                   string(change.ArtifactSchema),
				WorkflowCreated:                  true,
				WorktreePath:                     change.WorktreePath,
				WorktreeBranch:                   change.WorktreeBranch,
				WorktreeCreated:                  worktreeBinding.Created,
				WorktreeSkippedReason:            worktreeBinding.SkippedReason,
				ProjectContext:                   ctxOut,
				IntentInferenceDegraded:          inferenceResult.Degraded,
				IntentInferenceDegradationReason: inferenceResult.DegradeReason,
			}
			return encodeJSONResponse(cmd, out)
		}

		writer := newFormatWriter(cmd.OutOrStdout())
		writer.Writef(
			"change %s created  state=%s  substep=%s  complexity=%s\n",
			slug, model.StateS0Intake, change.IntakeSubStep, change.ComplexityLevel,
		)
		if inferenceResult.Degraded {
			writer.Writef("intent inference degraded; safe fallback applied\n")
		}
		if change.WorkflowPresetConfirmationPending() {
			writer.Writef("preset confirmation required: run `slipway preset <light|standard|strict>` before continuing\n")
			writer.Writef("AI suggestion: %s\n", change.SuggestedWorkflowPreset)
		} else {
			writer.Writeln("next: slipway next")
		}
		if change.WorktreePath != "" {
			writer.Writef("worktree: %s  branch=%s\n", state.DisplayPath(root, change.WorktreePath), change.WorktreeBranch)
		}
		return writer.Err()
	})
}

func restoreNewPresetAfterScaffoldFailure(root string, change *model.Change, scaffoldErr error) error {
	if restoreErr := restorePresetOnScaffoldFailure(root, change, "", ""); restoreErr != nil {
		return errors.Join(scaffoldErr, restoreErr)
	}
	return scaffoldErr
}

func generateUniqueChangeSlug(description string, slugExists func(string) (bool, error)) (string, error) {
	baseSlug := model.SlugifyTitle(description)
	if slugExists == nil {
		return baseSlug, nil
	}

	exists, err := slugExists(baseSlug)
	if err != nil {
		return "", err
	}
	if !exists {
		return baseSlug, nil
	}

	for n := 2; n <= generateUniqueChangeSlugMaxAttempts; n++ {
		suffix := fmt.Sprintf("-%d", n)
		candidateBase := baseSlug
		if len(candidateBase)+len(suffix) > model.MaxSlugLength {
			candidateBase = strings.Trim(candidateBase[:model.MaxSlugLength-len(suffix)], "-")
			if candidateBase == "" {
				candidateBase = "change"
			}
		}
		candidate := candidateBase + suffix
		exists, err := slugExists(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"unable to generate unique change slug for %q after %d attempts",
		baseSlug,
		generateUniqueChangeSlugMaxAttempts,
	)
}

// resolveChangeSetup determines guardrail domain, discovery need, artifact
// schema, complexity, and initial state from the description.
func resolveChangeSetup(
	ctx context.Context,
	inferenceText string,
	classifier progression.IntentClassifier,
) (model.ChangeSetup, progression.InferenceResult) {
	result := progression.ResolveIntentClassification(ctx, inferenceText, classifier)
	return classificationToChangeSetup(result.Classification), result
}

func classificationToChangeSetup(classification progression.IntentClassification) model.ChangeSetup {
	artifactSchema := model.ArtifactSchemaCore
	if classification.NeedsDiscovery {
		artifactSchema = model.ArtifactSchemaExpanded
	}

	return model.ChangeSetup{
		GuardrailDomain: classification.GuardrailDomain,
		NeedsDiscovery:  classification.NeedsDiscovery,
		ArtifactSchema:  artifactSchema,
		InitialSubStep:  model.PlanEntrySubStep(classification.NeedsDiscovery),
		ComplexityLevel: classification.Complexity,
	}
}

func suggestWorkflowPreset(root string, setup model.ChangeSetup) (model.WorkflowPreset, error) {
	suggestion := model.WorkflowPresetLight
	if setup.GuardrailDomain != "" || setup.NeedsDiscovery {
		suggestion = model.WorkflowPresetStandard
	}

	cfgPath := state.ConfigPath(root)
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("governance config parse error (fail-closed): %w", err)
		}
	} else {
		if cfg.Governance.DefaultPreset.IsValid() && cfg.Governance.DefaultPreset.Rank() > suggestion.Rank() {
			suggestion = cfg.Governance.DefaultPreset
		}
		if cfg.Governance.MinPreset.IsValid() && cfg.Governance.MinPreset.Rank() > suggestion.Rank() {
			suggestion = cfg.Governance.MinPreset
		}
	}

	return suggestion, nil
}

func governanceMinPresetConfigured(root string) (bool, error) {
	cfgPath := state.ConfigPath(root)
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("governance config parse error (fail-closed): %w", err)
	}
	return cfg.Governance.MinPreset.IsValid(), nil
}

func readStdinClassificationInput() (stdinClassificationInput, error) {
	data, err := io.ReadAll(io.LimitReader(newCommandStdin, 1<<20))
	if err != nil {
		return stdinClassificationInput{}, err
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return stdinClassificationInput{}, fmt.Errorf("stdin is empty")
	}
	var input stdinClassificationInput
	if err := json.Unmarshal(data, &input); err != nil {
		return stdinClassificationInput{}, fmt.Errorf("parse stdin json: %w", err)
	}
	return input, nil
}

func buildStdinClassifier(input stdinClassificationInput) progression.IntentClassifier {
	if input.GuardrailDomain == nil && input.NeedsDiscovery == nil && input.Complexity == nil {
		return nil
	}
	c := progression.IntentClassification{
		GuardrailDomain: "",
		NeedsDiscovery:  true,
		Complexity:      "complex",
	}
	if input.GuardrailDomain != nil {
		c.GuardrailDomain = *input.GuardrailDomain
	}
	if input.NeedsDiscovery != nil {
		c.NeedsDiscovery = *input.NeedsDiscovery
	}
	if input.Complexity != nil {
		c.Complexity = *input.Complexity
	}
	return stdinIntentClassifier{classification: c}
}

func mergeStdinProjectContext(ctx *model.ProjectContext, si *stdinClassificationInput) {
	if si.TechStack != nil {
		ctx.TechStack = *si.TechStack
	}
	if si.Conventions != nil {
		ctx.Conventions = *si.Conventions
	}
	if si.TestCmd != nil {
		ctx.TestCmd = *si.TestCmd
	}
	if si.BuildCmd != nil {
		ctx.BuildCmd = *si.BuildCmd
	}
	if len(si.Languages) > 0 {
		ctx.Languages = si.Languages
	}
	if si.RecentWork != nil {
		ctx.RecentWork = *si.RecentWork
	}
}

func parseCallerDisabledControls(raw []string) ([]model.ControlID, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	ids := make([]model.ControlID, 0, len(raw))
	for _, s := range raw {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			id := model.ControlID(trimmed)
			if !id.IsValid() {
				return nil, fmt.Errorf("unknown control_id %q", trimmed)
			}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return ids, nil
}

func parseCallerControlModes(raw map[string]string) (map[model.ControlID]model.ControlMode, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	modes := make(map[model.ControlID]model.ControlMode, len(raw))
	for rawID, rawMode := range raw {
		id := model.ControlID(strings.TrimSpace(rawID))
		if !id.IsValid() {
			return nil, fmt.Errorf("unknown control_id %q", rawID)
		}
		mode := model.ControlMode(strings.TrimSpace(rawMode))
		if !mode.IsValid() {
			return nil, fmt.Errorf("control %q has invalid mode %q", rawID, rawMode)
		}
		modes[id] = mode
	}
	return modes, nil
}

func parseWorkflowProfile(raw string) (model.WorkflowProfile, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	profile := model.WorkflowProfile(trimmed)
	if !profile.IsValid() {
		return "", newInvalidUsageError(
			"invalid_workflow_profile",
			fmt.Sprintf("invalid workflow profile %q", trimmed),
			"Use one of: code, docs, research, config, meta.",
			nil,
		)
	}
	return profile, nil
}

func parseSignalLevelOverride(raw *string, field string) (model.SignalLevel, error) {
	if raw == nil {
		return "", nil
	}
	level := model.SignalLevel(strings.TrimSpace(*raw))
	if level == "" {
		return "", nil
	}
	if !level.IsValid() {
		return "", fmt.Errorf("%s must be one of low|medium|high, got %q", field, *raw)
	}
	return level, nil
}
