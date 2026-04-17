package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type statusView struct {
	ExecutionMode             string                      `json:"execution_mode"`
	View                      string                      `json:"view,omitempty"`
	HydrateReferences         []string                    `json:"hydrate_references,omitempty"`
	Slug                      string                      `json:"slug,omitempty"`
	QualityMode               string                      `json:"quality_mode,omitempty"`
	WorkflowPreset            string                      `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset   string                      `json:"suggested_workflow_preset,omitempty"`
	EffectiveWorkflowPreset   string                      `json:"effective_workflow_preset,omitempty"`
	PresetConfirmationPending bool                        `json:"preset_confirmation_pending,omitempty"`
	PresetUpgradeReasons      []string                    `json:"preset_upgrade_reasons,omitempty"`
	GovernanceForecast        *governanceForecastView     `json:"governance_forecast,omitempty"`
	AutoPassedStates          []model.AutoPassedState     `json:"auto_passed_states,omitempty"`
	NeedsDiscovery            bool                        `json:"needs_discovery,omitempty"`
	Phase                     model.UserPhase             `json:"phase,omitempty"`
	LifecycleStatus           string                      `json:"lifecycle_status,omitempty"`
	CurrentState              model.WorkflowState         `json:"current_state,omitempty"`
	IntakeSubStep             model.IntakeSubStep         `json:"intake_substep,omitempty"`
	PlanSubStep               model.PlanSubStep           `json:"plan_substep,omitempty"`
	PlanningNote              string                      `json:"planning_note,omitempty"`
	InterruptedExecutionAt    string                      `json:"interrupted_execution_at,omitempty"`
	Narrative                 string                      `json:"narrative,omitempty"`
	NextReadyActions          []string                    `json:"next_ready_actions,omitempty"`
	SummaryBlockers           []model.ReasonCode          `json:"summary_blockers,omitempty"`
	Blockers                  []model.ReasonCode          `json:"blockers,omitempty"`
	GateStatus                map[string]model.GateRecord `json:"gate_status,omitempty"`
	ContextDependencies       *model.ContextDependencies  `json:"context_dependencies,omitempty"`
	SelectedPriorContext      []selectedPriorContextView  `json:"selected_prior_context,omitempty"`
	UnresolvedDependencies    []unresolvedDependencyView  `json:"unresolved_dependencies,omitempty"`
	Progress                  *statusProgress             `json:"progress,omitempty"`
	ArtifactDAG               []artifactDAGNode           `json:"artifact_dag,omitempty"`
	ArtifactAmendments        []artifact.AmendmentEvent   `json:"artifact_amendments,omitempty"`
	EvidencePointers          statusEvidencePointers      `json:"evidence_pointers,omitempty"`
	EvidenceFreshness         string                      `json:"evidence_freshness"`
	SourceStateFile           string                      `json:"source_state_file,omitempty"`
	Diagnostics               []string                    `json:"diagnostics,omitempty"`
	// Governance (derived from governance_snapshot.yaml)
	GovernanceSignals *governanceSignalView   `json:"governance_signals,omitempty"`
	ActiveControls    []governanceControlView `json:"active_controls,omitempty"`
	RequiredActions   []governanceActionView  `json:"required_actions,omitempty"`
}

type governanceSignalView struct {
	Domains     []string `json:"domains,omitempty"`
	BlastRadius string   `json:"blast_radius"`
}

type governanceControlView struct {
	ControlID    string   `json:"control_id"`
	Mode         string   `json:"mode"`
	Scope        string   `json:"scope"`
	TriggeredBy  []string `json:"triggered_by,omitempty"`
	PolicySource string   `json:"policy_source,omitempty"`
}

type governanceActionView struct {
	ControlID   string `json:"control_id"`
	Mode        string `json:"mode"`
	Description string `json:"description"`
	Satisfied   bool   `json:"satisfied"`
}

type artifactDAGNode struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	DependsOn []string `json:"depends_on,omitempty"`
	Ready     bool     `json:"ready"`
}

type statusProgress struct {
	Percentage        int            `json:"percentage"`
	StageIndex        int            `json:"stage_index"`
	StageTotal        int            `json:"stage_total"`
	StageName         string         `json:"stage_name"`
	CurrentWaveIndex  int            `json:"current_wave_index,omitempty"`
	CompletedWaves    int            `json:"completed_waves,omitempty"`
	TotalWaves        int            `json:"total_waves,omitempty"`
	WavesByVerdict    map[string]int `json:"waves_by_verdict,omitempty"`
	TasksCompleted    int            `json:"tasks_completed"`
	TasksTotal        int            `json:"tasks_total"`
	TasksByVerdict    map[string]int `json:"tasks_by_verdict,omitempty"`
	RunSummaryVersion int            `json:"run_summary_version"`
}

type statusEvidencePointers struct {
	TaskEvidence    map[string]string `json:"task_evidence,omitempty"`
	NonTaskEvidence map[string]string `json:"non_task_evidence,omitempty"`
}

type statusRoute struct {
	change      *model.Change
	multiChange bool
	diagnostics *statusView
}

func makeStatusCmd() *cobra.Command {
	var format string
	var changeSlug string
	var jsonFlag bool
	var view string
	var hydrate bool
	var hydrateRefs []string
	var listViews bool
	var statsMode bool
	var rootMode bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: desc("status"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if rootMode {
				root, err := projectRootFromWD()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), root)
				return err
			}
			if statsMode {
				root, err := projectRootFromWD()
				if err != nil {
					return err
				}
				sv, err := buildStatsView(root, time.Now().UTC())
				if err != nil {
					return err
				}
				if jsonFlag {
					return encodeJSONResponse(cmd, sv)
				}
				return writeStatsText(cmd.OutOrStdout(), sv)
			}
			if listViews {
				return emitViewDiscovery(cmd, "status", format)
			}
			if err := validateViewAlias("status", view); err != nil {
				return err
			}
			if len(hydrateRefs) > 0 && !hydrate {
				return newInvalidUsageError(
					"hydrate_ref_requires_hydrate",
					"`--hydrate-ref` requires `--hydrate`",
					"Add `--hydrate` to emit hydrate bodies, or remove `--hydrate-ref`.",
					map[string]any{"hydrate_refs": normalizeHydrateKeys(hydrateRefs)},
				)
			}
			explicitView := strings.TrimSpace(view)
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			if _, err := os.Stat(state.ConfigPath(root)); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("workspace is not initialized; run `slipway init`")
				}
				return err
			}
			if _, err := loadConfigAtRoot(root); err != nil {
				return err
			}

			// Resolve output format.
			outputFormat, err := resolveStatusFormat(format, jsonFlag)
			if err != nil {
				return err
			}

			// When --change is provided, show detail view for that specific change.
			if changeSlug != "" {
				effectiveView := resolveEffectiveView("status", explicitView)
				hydrateKeys := normalizeHydrateKeys(resolveEffectiveViewHydrate("status", explicitView))
				if hydrate {
					hydrateKeys, err = selectHydrateKeys(hydrateKeys, hydrateRefs)
					if err != nil {
						return err
					}
				}
				change, err := loadChangeBySlug(root, changeSlug)
				if err != nil {
					return err
				}
				return showStatusForChange(cmd, root, change, outputFormat, effectiveView, hydrateKeys, hydrate)
			}

			changes, err := state.ListChanges(root)
			if err != nil {
				return err
			}
			var active []model.Change
			for _, c := range changes {
				if c.Status == model.ChangeStatusActive {
					active = append(active, c)
				}
			}
			route, err := resolveStatusRouteForRoot(root, active)
			if err != nil {
				return err
			}
			if route.multiChange {
				return printMultiChangeSummary(cmd, active, outputFormat)
			}
			if route.diagnostics != nil {
				// Diagnostics mode is not a routed command context. When no
				// active change exists, only preserve an explicit --view value.
				route.diagnostics.View = explicitView
				if explicitView != "" {
					route.diagnostics.HydrateReferences = normalizeHydrateKeys(resolveEffectiveViewHydrate("status", explicitView))
					if hydrate {
						route.diagnostics.HydrateReferences, err = selectHydrateKeys(route.diagnostics.HydrateReferences, hydrateRefs)
						if err != nil {
							return err
						}
					}
				}
				return printStatusView(cmd, root, *route.diagnostics, outputFormat, hydrate)
			}

			effectiveView := resolveEffectiveView("status", explicitView)
			hydrateKeys := normalizeHydrateKeys(resolveEffectiveViewHydrate("status", explicitView))
			if hydrate {
				hydrateKeys, err = selectHydrateKeys(hydrateKeys, hydrateRefs)
				if err != nil {
					return err
				}
			}
			return showStatusForChange(cmd, root, *route.change, outputFormat, effectiveView, hydrateKeys, hydrate)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "JSON output (shorthand for --format json)")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text|yaml|json (also used by --list-views: text|json)")
	cmd.Flags().StringVar(&view, "view", "", "Status view override (e.g. incident)")
	cmd.Flags().BoolVar(&listViews, "list-views", false, "List public --view aliases for this command and exit")
	cmd.Flags().BoolVar(&hydrate, "hydrate", false, "Append selected hydrate reference bodies (text output only)")
	cmd.Flags().StringArrayVar(&hydrateRefs, "hydrate-ref", nil, "Restrict `--hydrate` output to the selected `<skill-id>/<name>` reference (repeatable)")
	cmd.Flags().BoolVar(&statsMode, "stats", false, "Show workspace diagnostics (active count, stale summaries, integrity issues)")
	cmd.Flags().BoolVar(&rootMode, "root", false, "Print the canonical slipway scope root")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	return cmd
}

func resolveStatusRoute(active []model.Change) statusRoute {
	switch {
	case len(active) == 0:
		return statusRoute{
			diagnostics: diagnosticStatusView("no active change; start one with `slipway new`"),
		}
	case len(active) > 1:
		return statusRoute{multiChange: true}
	default:
		c := active[0]
		return statusRoute{change: &c}
	}
}

func resolveStatusRouteForRoot(root string, active []model.Change) (statusRoute, error) {
	route := resolveStatusRoute(active)
	if !route.multiChange {
		return route, nil
	}

	ref, err := resolveActiveChangeRef(root, "")
	if err != nil {
		if shouldFallbackStatusMultiSummary(err) {
			return route, nil
		}
		return statusRoute{}, err
	}

	change, err := loadChangeBySlug(root, ref.Slug)
	if err != nil {
		return statusRoute{}, err
	}
	return statusRoute{change: &change}, nil
}

func shouldFallbackStatusMultiSummary(err error) bool {
	cliErr := asCLIError(err)
	if cliErr == nil {
		return false
	}
	return cliErr.ErrorCode == "no_active_change" || cliErr.ErrorCode == "active_context_ambiguous"
}

func diagnosticStatusView(message string) *statusView {
	return &statusView{
		ExecutionMode:     "diagnostics",
		EvidenceFreshness: "unknown",
		Diagnostics:       []string{message},
	}
}

func showStatusForChange(cmd *cobra.Command, root string, change model.Change, outputFormat string, requestedView string, hydrateKeys []string, hydrate bool) error {
	return withChangeStateLock(root, change.Slug, "status", func() error {
		latest, err := state.LoadChange(root, change.Slug)
		if err != nil {
			return err
		}
		view, err := buildStatusViewFromChange(root, latest)
		if err != nil {
			return err
		}
		view.View = requestedView
		view.HydrateReferences = hydrateKeys
		return printStatusView(cmd, root, view, outputFormat, hydrate)
	})
}

func resolveStatusFormat(format string, jsonFlag bool) (string, error) {
	if jsonFlag {
		return "json", nil
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json", nil
	case "yaml":
		return "yaml", nil
	case "text":
		return "text", nil
	default:
		return "", newInvalidUsageError(
			"invalid_format",
			fmt.Sprintf("invalid --format %q; expected text|yaml|json", format),
			"Use --format text, --format yaml, or --format json.",
			nil,
		)
	}
}

// multiChangeSummaryEntry represents one active change in the multi-change summary.
type multiChangeSummaryEntry struct {
	Slug         string `json:"slug,omitempty"`
	Description  string `json:"description,omitempty"`
	ExecMode     string `json:"execution_mode"`
	CurrentState string `json:"current_state"`
	WorktreePath string `json:"worktree_path,omitempty"`
}

// multiChangeSummaryView is the top-level output for multi-active status display.
type multiChangeSummaryView struct {
	ExecutionMode string                    `json:"execution_mode"`
	ActiveCount   int                       `json:"active_count"`
	ActiveChanges []multiChangeSummaryEntry `json:"active_changes"`
	Hint          string                    `json:"hint"`
}
