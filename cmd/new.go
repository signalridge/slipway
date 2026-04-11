package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/intake"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type createOutput struct {
	Mode                      string                `json:"mode"`
	Slug                      string                `json:"slug"`
	Description               string                `json:"description"`
	QualityMode               string                `json:"quality_mode,omitempty"`
	WorkflowPreset            string                `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset   string                `json:"suggested_workflow_preset,omitempty"`
	PresetConfirmationPending bool                  `json:"preset_confirmation_pending,omitempty"`
	CurrentState              string                `json:"current_state"`
	IntakeSubStep             string                `json:"intake_substep,omitempty"`
	PlanSubStep               string                `json:"plan_substep,omitempty"`
	Phase                     model.UserPhase       `json:"phase"`
	NeedsDiscovery            bool                  `json:"needs_discovery"`
	ComplexityLevel           string                `json:"complexity_level,omitempty"`
	GuardrailDomain           string                `json:"guardrail_domain,omitempty"`
	ArtifactSchema            string                `json:"artifact_schema"`
	WorkflowCreated           bool                  `json:"workflow_created"`
	ProjectContext            *projectContextOutput `json:"project_context,omitempty"`
}

type projectContextOutput struct {
	TechStack  string `json:"tech_stack,omitempty"`
	Languages  string `json:"languages,omitempty"`
	TestCmd    string `json:"test_cmd,omitempty"`
	BuildCmd   string `json:"build_cmd,omitempty"`
	RecentWork string `json:"recent_work,omitempty"`
}

const generateUniqueChangeSlugMaxAttempts = 10000

var (
	newCommandStdin      = os.Stdin
	newCommandIsTerminal = term.IsTerminal
)

func makeNewCmd() *cobra.Command {
	var preset string
	var discuss bool
	var full bool
	var trivial bool
	var fromDoc string

	cmd := &cobra.Command{
		Use:   "new [description]",
		Short: desc("new"),
		Long: `Create a governed change starting at S0_INTAKE.

This command creates a governed change and begins the intake-first workflow.
Complexity is inferred from the description and guardrail domains.
Project context is auto-detected from lockfiles, scripts, and recent work.

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
				// --json without description: always error
				if jsonFlag {
					return newInvalidUsageError(
						"description_required",
						"description required in JSON mode",
						"Provide a description as the first argument, or use --from-doc to seed from a document.",
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
				root, err := projectRootFromWD()
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

			return runNewCommand(cmd, description, qualityMode, workflowPreset, trivial, fromDoc)
		},
	}

	cmd.Flags().StringVar(&preset, "preset", "", "Governance preset: light|standard|strict")
	cmd.Flags().BoolVar(&discuss, "discuss", false, "Persist unresolved gray areas into context before execution")
	cmd.Flags().BoolVar(&full, "full", false, "Require refreshed final-closeout evidence before ship")
	cmd.Flags().BoolVar(&trivial, "trivial", false, "Force complexity=trivial, minimize intake depth")
	cmd.Flags().StringVar(&fromDoc, "from-doc", "", "Seed intake from a document path (PRD, RFC, etc.)")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func runNewCommand(
	cmd *cobra.Command,
	description string,
	qualityMode model.QualityMode,
	workflowPreset model.WorkflowPreset,
	trivial bool,
	fromDoc string,
) error {
	return createDirectGovernedChange(cmd, description, qualityMode, workflowPreset, trivial, fromDoc)
}

// createDirectGovernedChange is the governed change creation path for `new`.
func createDirectGovernedChange(
	cmd *cobra.Command,
	description string,
	qualityMode model.QualityMode,
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

	root, err := projectRootFromWD()
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
	setup := resolveChangeSetup(inferenceText)

	// Override complexity if --trivial flag is set
	if trivial {
		setup.ComplexityLevel = "trivial"
	}

	// Infer project context (auto-detect from repo, merge with config)
	projectCtx := progression.InferProjectContext(root)

	return withChangeCreateLock(root, func() error {
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

		if err := state.SaveChange(root, change); err != nil {
			return err
		}
		if change.WorkflowPreset.IsValid() {
			// Preset confirmed (auto or explicit): scaffold the full bundle.
			resolution := progression.ResolveChangeSchemaDiagnostics(change)
			if len(resolution.Blockers) > 0 {
				return fmt.Errorf("resolve artifact schema: %s", strings.Join(resolution.Blockers, ","))
			}
			policy, err := governance.ResolvePresetPolicy(root, change)
			if err != nil {
				return err
			}
			// If --from-doc content is available, extract sections and pass
			// them into scaffolding so seeded artifacts are enriched.
			var scaffoldErr error
			if fromDocContent != "" {
				docs := artifact.DocSections{
					Scope:       extractedFromDoc.Scope,
					Constraints: extractedFromDoc.Constraints,
					Acceptance:  extractedFromDoc.Acceptance,
				}
				scaffoldErr = artifact.ScaffoldGovernedBundleForChangeWithContextAndDocs(root, change, policy.EffectivePreset, projectCtx, docs, resolution.Schema)
			} else {
				scaffoldErr = artifact.ScaffoldGovernedBundleForChangeWithContext(root, change, policy.EffectivePreset, projectCtx, resolution.Schema)
			}
			if scaffoldErr != nil {
				return restoreNewPresetAfterScaffoldFailure(root, &change, scaffoldErr)
			}
		} else {
			// Preset pending: scaffold only intent.md so S0_INTAKE has
			// its primary artifact. The full bundle will be created when
			// preset is confirmed via `slipway preset`.
			if err := artifact.ScaffoldIntentForChangeWithContext(root, change, projectCtx); err != nil {
				return err
			}
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

		ctxOut := &projectContextOutput{
			TechStack:  projectCtx.TechStack,
			Languages:  strings.Join(projectCtx.Languages, ", "),
			TestCmd:    projectCtx.TestCmd,
			BuildCmd:   projectCtx.BuildCmd,
			RecentWork: projectCtx.RecentWork,
		}

		if jsonFlag {
			out := createOutput{
				Mode:                      "governed",
				Slug:                      slug,
				Description:               description,
				QualityMode:               string(qualityMode),
				WorkflowPreset:            string(change.WorkflowPreset),
				SuggestedWorkflowPreset:   string(change.SuggestedWorkflowPreset),
				PresetConfirmationPending: change.WorkflowPresetConfirmationPending(),
				CurrentState:              string(model.StateS0Intake),
				IntakeSubStep:             string(change.IntakeSubStep),
				Phase:                     change.Phase(),
				NeedsDiscovery:            change.NeedsDiscovery,
				ComplexityLevel:           change.ComplexityLevel,
				GuardrailDomain:           change.GuardrailDomain,
				ArtifactSchema:            string(change.ArtifactSchema),
				WorkflowCreated:           true,
				ProjectContext:            ctxOut,
			}
			return encodeJSONResponse(cmd, out)
		}

		writer := newFormatWriter(cmd.OutOrStdout())
		writer.Writef(
			"change %s created  state=%s  substep=%s  complexity=%s\n",
			slug, model.StateS0Intake, change.IntakeSubStep, change.ComplexityLevel,
		)
		if change.WorkflowPresetConfirmationPending() {
			writer.Writef("preset confirmation required: run `slipway preset <light|standard|strict>` before continuing\n")
			writer.Writef("AI suggestion: %s\n", change.SuggestedWorkflowPreset)
		} else {
			writer.Writeln("next: slipway next")
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
		candidate := fmt.Sprintf("%s-%d", baseSlug, n)
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
func resolveChangeSetup(description string) model.ChangeSetup {
	guardrailDomain := progression.InferGuardrailDomain(description, true)
	needsDiscovery := progression.InferDiscovery(description, guardrailDomain)
	complexityLevel := progression.InferComplexity(description, guardrailDomain)

	artifactSchema := model.ArtifactSchemaCore
	if needsDiscovery {
		artifactSchema = model.ArtifactSchemaExpanded
	}

	return model.ChangeSetup{
		GuardrailDomain: guardrailDomain,
		NeedsDiscovery:  needsDiscovery,
		ArtifactSchema:  artifactSchema,
		InitialSubStep:  model.PlanEntrySubStep(needsDiscovery),
		ComplexityLevel: complexityLevel,
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
