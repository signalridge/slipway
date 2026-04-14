package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/tmpl"
	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/spf13/cobra"
)

type healthView struct {
	ExecutionMode string                             `json:"execution_mode"`
	View          string                             `json:"view,omitempty"`
	Findings      []state.HealthFinding              `json:"findings,omitempty"`
	Governance    *governance.GovernanceHealthReport `json:"governance,omitempty"`
	Observations  []model.SignalObservation          `json:"observations,omitempty"`
	Diagnostics   []string                           `json:"diagnostics,omitempty"`
	Doctor        *doctorView                        `json:"doctor,omitempty"`
	ShowRepo      bool                               `json:"-"`
}

type doctorView struct {
	Actions []doctorAction `json:"actions,omitempty"`
}

type doctorAction struct {
	Priority   int    `json:"priority"`
	Category   string `json:"category"`
	Slug       string `json:"slug,omitempty"`
	Summary    string `json:"summary"`
	Command    string `json:"command,omitempty"`
	Repairable bool   `json:"repairable"`
}

func makeHealthCmd() *cobra.Command {
	var jsonOutput bool
	var governanceFlag bool
	var allFlag bool
	var observationsFlag bool
	var doctorFlag bool
	var changeSlug string
	var routeView string

	cmd := &cobra.Command{
		Use:   "health",
		Short: desc("health"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateRouteView("health", routeView); err != nil {
				return err
			}
			explicitView := strings.TrimSpace(routeView)
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			showRepo := doctorFlag || !governanceFlag || allFlag
			showGov := doctorFlag || governanceFlag || allFlag

			view := healthView{
				ExecutionMode: "diagnostics",
				ShowRepo:      showRepo,
				View:          explicitView,
			}
			governanceNotApplicable := false
			if doctorFlag {
				view.ExecutionMode = "doctor"
			}

			// Repo health (default behavior preserved).
			if showRepo {
				report, err := state.CollectHealthReport(root)
				if err != nil {
					return err
				}
				view.Findings = normalizeHealthFindings(root, report.Findings)
				view.Findings = append(view.Findings, agentContractHealthFindings(root)...)
				slices.SortFunc(view.Findings, func(a, b state.HealthFinding) int {
					if a.Category != b.Category {
						return strings.Compare(a.Category, b.Category)
					}
					if a.Slug != b.Slug {
						return strings.Compare(a.Slug, b.Slug)
					}
					return strings.Compare(a.Message, b.Message)
				})
			}

			// Governance health.
			if showGov {
				change, err := resolveHealthChangeTarget(root, changeSlug)
				switch {
				case err == nil && change != nil:
					if view.View == "" {
						// Auto view routing is applied only when health is
						// evaluating a concrete active/selected change.
						view.View = resolveEffectiveRouteView("health", "")
					}
					var govReport governance.GovernanceHealthReport
					var persistedSnap model.GovernanceSnapshot
					var snap model.GovernanceSnapshot
					err := withChangeStateLock(root, change.Slug, "health", func() error {
						latest, err := state.LoadChangeForDiagnostics(root, change.Slug)
						if err != nil {
							return err
						}
						persistedSnap, err = governance.LoadSnapshot(root, latest.Slug)
						if err != nil {
							govReport = governance.CollectGovernanceHealth(root, latest)
							skipRecompute, validationErr := shouldSkipGovernanceSnapshotRecompute(root, latest)
							if validationErr != nil {
								return validationErr
							}
							if observationsFlag && !skipRecompute {
								paths, pathErr := state.ResolveChangePaths(root, latest)
								if pathErr == nil {
									preview, previewErr := governance.PreviewGovernanceSnapshot(root, latest, paths.GovernedBundleDir)
									if previewErr == nil {
										view.Observations = preview.Observations
									}
								}
							}
							return nil
						}
						govReport = governance.CollectGovernanceHealthWithSnapshot(root, latest, persistedSnap)
						if governanceHealthHasCheckStatus(govReport, "controls_config", "FAIL") {
							return nil
						}
						skipRecompute, validationErr := shouldSkipGovernanceSnapshotRecompute(root, latest)
						if validationErr != nil {
							return validationErr
						}
						if skipRecompute {
							return nil
						}
						paths, err := state.ResolveChangePaths(root, latest)
						if err != nil {
							return err
						}
						snap, err = governance.RecomputeGovernanceSnapshot(root, latest, paths.GovernedBundleDir)
						if err != nil {
							// Snapshot recompute failed: degrade gracefully with diagnostic.
							govReport = governance.CollectGovernanceHealth(root, latest)
							view.Diagnostics = append(view.Diagnostics, "governance_snapshot_unavailable: "+err.Error())
							return nil
						}
						govReport = governance.CollectGovernanceHealthWithSnapshot(root, latest, snap)
						freshnessSnap := selectGovernanceHealthFreshnessSnapshot(persistedSnap, snap)
						overrideGovernanceHealthCheck(&govReport, governance.SignalFreshnessCheck(freshnessSnap))
						if observationsFlag {
							view.Observations = snap.Observations
						}
						return nil
					})
					if err != nil {
						return fmt.Errorf("governance health: %w", err)
					}
					view.Governance = &govReport
				case err == nil:
					view.Diagnostics = append(view.Diagnostics, "No active change; governance health not applicable.")
					governanceNotApplicable = true
				case doctorFlag && errors.Is(err, state.ErrMultipleActiveChanges):
					view.Diagnostics = append(view.Diagnostics, "Multiple active changes; governance health requires an explicit `--change` target.")
				default:
					return fmt.Errorf("governance health: %w", err)
				}
			}
			if doctorFlag {
				doctor, err := buildDoctorView(root, changeSlug, view.Findings, view.Governance)
				if err != nil {
					return err
				}
				if governanceNotApplicable {
					doctor.Actions = normalizeDoctorActions(append(doctor.Actions, doctorAction{
						Priority:   50,
						Category:   "governance",
						Summary:    "No active change; governance health not applicable.",
						Repairable: false,
					}))
				}
				view.Doctor = doctor
			}

			if jsonOutput {
				return encodeJSONResponse(cmd, view)
			}
			return writeHealthText(cmd.OutOrStdout(), view)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().BoolVar(&governanceFlag, "governance", false, "Show governance diagnostics for the active or selected change")
	cmd.Flags().BoolVar(&allFlag, "all", false, "Show both repo health and governance health")
	cmd.Flags().BoolVar(&observationsFlag, "observations", false, "Include detailed governance signal provenance")
	cmd.Flags().BoolVar(&doctorFlag, "doctor", false, "Synthesize prioritized repair and recovery actions without mutating state")
	cmd.Flags().StringVar(&routeView, "view", "", "Health view override (e.g. incident-response, observability-query)")
	addChangeSelectorFlags(cmd, &changeSlug, "Target a specific change for governance health")
	return cmd
}

func overrideGovernanceHealthCheck(report *governance.GovernanceHealthReport, replacement governance.GovernanceHealthCheck) {
	for i := range report.Checks {
		if report.Checks[i].Name == replacement.Name {
			report.Checks[i] = replacement
			report.Healthy = true
			for _, check := range report.Checks {
				if check.Status == "FAIL" {
					report.Healthy = false
					break
				}
			}
			return
		}
	}
	report.Checks = append(report.Checks, replacement)
	if replacement.Status == "FAIL" {
		report.Healthy = false
	}
}

func selectGovernanceHealthFreshnessSnapshot(
	persisted model.GovernanceSnapshot,
	recomputed model.GovernanceSnapshot,
) model.GovernanceSnapshot {
	if persisted.Version > 0 && persisted.PersistedEqual(recomputed) {
		return persisted
	}
	return recomputed
}

// resolveHealthChangeTarget finds the change to use for governance health checks.
func resolveHealthChangeTarget(root, slug string) (*model.Change, error) {
	if slug != "" {
		change, err := state.LoadChangeForDiagnostics(root, slug)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, newPreconditionError(
					"change_not_found",
					fmt.Sprintf("no change found for slug %q", slug),
					"Check the slug with `slipway status`.",
					slug,
					nil,
				)
			}
			return nil, newStateIntegrityError(
				"change_state_load_failed",
				fmt.Sprintf("failed to load change state for %q: %v", slug, err),
				"Run `slipway repair` to inspect or repair change state files.",
				slug,
				map[string]any{
					"path": filepath.Join("artifacts", "changes", slug, "change.yaml"),
				},
			)
		}
		return &change, nil
	}
	changes, issues, err := state.ListChangesBestEffortWithIssues(root)
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			var runtimeErr *state.ChangeRuntimeStateLoadError
			if errors.As(issue.Err, &runtimeErr) {
				continue
			}
			return nil, issue.Err
		}
	}
	active := make([]model.Change, 0, len(changes))
	for _, change := range changes {
		if change.Status == model.ChangeStatusActive {
			active = append(active, change)
		}
	}
	switch len(active) {
	case 0:
		return nil, nil
	case 1:
		return &active[0], nil
	default:
		return nil, state.ErrMultipleActiveChanges
	}
}

func governanceHealthHasCheckStatus(report governance.GovernanceHealthReport, name, status string) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func shouldSkipGovernanceSnapshotRecompute(root string, change model.Change) (bool, error) {
	validation, err := state.ValidateChangeWorktree(root, change)
	if err != nil {
		return false, err
	}
	return len(validation.Blockers) > 0, nil
}

func writeHealthText(w io.Writer, view healthView) error {
	writer := newFormatWriter(w)
	if strings.TrimSpace(view.View) != "" {
		writer.Writef("View: %s\n\n", view.View)
	}

	if view.Doctor != nil {
		writer.Writef("Doctor:\n")
		if len(view.Doctor.Actions) == 0 {
			writer.Writef("  Actions: none\n")
		} else {
			for _, action := range view.Doctor.Actions {
				line := fmt.Sprintf("  %d. %s", action.Priority, action.Summary)
				if action.Slug != "" {
					line += " (" + action.Slug + ")"
				}
				writer.Writef("%s\n", line)
				if strings.TrimSpace(action.Command) != "" {
					writer.Writef("      command: %s\n", action.Command)
				}
			}
		}
		if view.ShowRepo || view.Governance != nil || len(view.Observations) > 0 || len(view.Diagnostics) > 0 {
			writer.Writef("\n")
		}
	}

	// Repo health.
	if view.ShowRepo {
		writer.Writef("Repo Health:\n")
		if len(view.Findings) == 0 {
			writer.Writef("  Findings: none\n")
		} else {
			writer.Writef("  Findings:\n")
			for _, finding := range view.Findings {
				line := "  - [" + string(finding.Severity) + "] " + finding.Category + ": " + finding.Message
				if finding.Slug != "" {
					line += " (" + finding.Slug + ")"
				}
				writer.Writef("  %s\n", line)
				if hint := strings.TrimSpace(finding.RepairHint); hint != "" {
					label := "repair"
					if !finding.Repairable {
						label = "action"
					}
					writer.Writef("        %s: %s\n", label, hint)
				}
			}
		}
	}

	// Governance health.
	if view.Governance != nil {
		writer.Writef("\nGovernance Health (active change: %s):\n", view.Governance.Slug)
		for _, check := range view.Governance.Checks {
			writer.Writef("  %-28s %s - %s\n", check.Name+":", check.Status, check.Message)
		}
		if view.Governance.Healthy {
			writer.Writef("\nOverall: HEALTHY\n")
		} else {
			writer.Writef("\nOverall: UNHEALTHY\n")
		}
	}
	if len(view.Observations) > 0 {
		writer.Writef("\nObservations:\n")
		for _, obs := range view.Observations {
			writer.Writef("  - %s: %s=%s (%s)\n", obs.ID, obs.Signal, obs.Level, obs.Source)
			if len(obs.EvidenceRefs) > 0 {
				writer.Writef("      evidence: %s\n", strings.Join(obs.EvidenceRefs, ", "))
			}
			writer.Writef("      reason: %s\n", obs.Reason)
		}
	}
	if len(view.Diagnostics) > 0 {
		if view.ShowRepo || view.Governance != nil || len(view.Observations) > 0 {
			writer.Writef("\n")
		}
		for _, diagnostic := range view.Diagnostics {
			writer.Writef("%s\n", diagnostic)
		}
	}

	return writer.Err()
}

func agentContractHealthFindings(root string) []state.HealthFinding {
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return []state.HealthFinding{{
			Severity:   model.ReasonSeverityError,
			Category:   "agent_contract",
			Message:    "Governance agent mapping is invalid",
			Repairable: false,
			RepairHint: "Edit `.slipway.yaml` so each governance skill points to a governance-mapped Slipway agent.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("agent_mapping_invalid", err.Error())},
		}}
	}

	workspaceRoot := invocationWorkspaceRoot(root)
	activeTool := toolgen.ResolveWorkspaceTool(workspaceRoot)
	detectedTools := toolgen.DetectExistingTools(workspaceRoot)
	validateGeneratedSurface := slices.Contains(detectedTools, activeTool.ID) &&
		strings.TrimSpace(activeTool.AgentStyle) != "" &&
		strings.TrimSpace(activeTool.AgentsDir) != ""

	validAgents := map[string]struct{}{}
	missingTemplates := map[string]error{}
	for _, name := range tmpl.AgentNames() {
		validAgents[name] = struct{}{}
		if _, err := tmpl.Content("agents/" + name + ".md"); err != nil {
			missingTemplates[name] = err
		}
	}

	findings := []state.HealthFinding{}
	for _, def := range registry {
		agentName := strings.TrimSpace(def.AgentHint)
		if agentName == "" {
			continue
		}

		if _, ok := validAgents[agentName]; !ok {
			findings = append(findings, state.HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "agent_contract",
				Slug:       def.Name,
				Message:    fmt.Sprintf("Governance skill %q points to unknown agent %q", def.Name, agentName),
				Repairable: false,
				RepairHint: "Edit `.slipway.yaml` so each governance skill points to a generated Slipway agent.",
				Reasons:    []model.ReasonCode{model.NewReasonCode("agent_mapping_unknown_agent", fmt.Sprintf("%s=%s", def.Name, agentName))},
			})
			continue
		}
		if _, missing := missingTemplates[agentName]; missing {
			findings = append(findings, state.HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "agent_contract",
				Slug:       def.Name,
				Message:    fmt.Sprintf("Governance skill %q points to missing agent template %q", def.Name, agentName),
				Repairable: false,
				RepairHint: "Restore the missing agent template in `internal/tmpl/templates/agents/` and regenerate tool adapters.",
				Reasons:    []model.ReasonCode{model.NewReasonCode("agent_template_missing", fmt.Sprintf("%s=%s", def.Name, agentName))},
			})
			continue
		}
	}
	if !validateGeneratedSurface {
		return findings
	}

	for _, def := range registry {
		agentName := strings.TrimSpace(def.AgentHint)
		if agentName == "" {
			continue
		}

		agentPath := toolgen.AgentPath(activeTool, agentName)
		if strings.TrimSpace(agentPath) == "" {
			continue
		}
		fullPath := filepath.Join(workspaceRoot, agentPath)
		if _, err := os.Stat(fullPath); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			findings = append(findings, state.HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "agent_contract",
				Slug:       def.Name,
				Message:    fmt.Sprintf("Governance skill %q points to unreadable generated agent file for %s", def.Name, activeTool.ID),
				Repairable: false,
				RepairHint: fmt.Sprintf("Run `slipway init --tools %s --refresh` to regenerate tool surfaces in the current workspace, then retry.", activeTool.ID),
				Reasons: []model.ReasonCode{
					model.NewReasonCode("agent_generated_surface_unreadable", state.DisplayPath(workspaceRoot, fullPath)),
				},
			})
			continue
		}

		findings = append(findings, state.HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "agent_contract",
			Slug:       def.Name,
			Message:    fmt.Sprintf("Governance skill %q points to missing generated agent file for %s", def.Name, activeTool.ID),
			Repairable: false,
			RepairHint: fmt.Sprintf("Run `slipway init --tools %s --refresh` to regenerate tool surfaces in the current workspace.", activeTool.ID),
			Reasons: []model.ReasonCode{
				model.NewReasonCode("agent_generated_surface_missing", state.DisplayPath(workspaceRoot, fullPath)),
			},
		})
	}
	return findings
}

func buildDoctorView(
	root, changeSlug string,
	findings []state.HealthFinding,
	report *governance.GovernanceHealthReport,
) (*doctorView, error) {
	actions := make([]doctorAction, 0, len(findings)+len(governanceDoctorActions(report))+1)
	for _, finding := range findings {
		action := doctorAction{
			Priority:   doctorPriority(finding),
			Category:   finding.Category,
			Slug:       finding.Slug,
			Summary:    finding.Message,
			Repairable: finding.Repairable,
		}
		if finding.Repairable {
			action.Command = extractCommandHint(finding.RepairHint)
			if action.Command == "" {
				action.Command = "slipway repair"
			}
			action.Repairable = true
		} else if healthFindingRepairable(root, finding) {
			action.Command = "slipway repair"
			action.Repairable = true
		} else {
			action.Command = extractCommandHint(finding.RepairHint)
		}
		actions = append(actions, action)
	}
	actions = append(actions, governanceDoctorActions(report)...)

	resumeAction, err := doctorResumeAction(root, changeSlug)
	if err != nil {
		return nil, err
	}
	if resumeAction != nil {
		actions = append(actions, *resumeAction)
	}

	return &doctorView{Actions: normalizeDoctorActions(actions)}, nil
}

func governanceDoctorActions(report *governance.GovernanceHealthReport) []doctorAction {
	if report == nil {
		return nil
	}

	actions := make([]doctorAction, 0, len(report.Checks))
	for _, check := range report.Checks {
		if check.Status == "OK" {
			continue
		}

		action := doctorAction{
			Priority:   governanceDoctorPriority(check.Status),
			Category:   "governance_" + check.Name,
			Slug:       report.Slug,
			Summary:    check.Message,
			Repairable: false,
		}
		if command, repairable := governanceDoctorCommand(check); command != "" {
			action.Command = command
			action.Repairable = repairable
		}
		actions = append(actions, action)
	}
	return actions
}

func governanceDoctorPriority(status string) int {
	switch status {
	case "FAIL":
		return 15
	case "WARN":
		return 35
	default:
		return 45
	}
}

func governanceDoctorCommand(check governance.GovernanceHealthCheck) (string, bool) {
	switch check.Name {
	case "signal_control_coherence":
		if strings.Contains(check.Message, "execution summary") ||
			strings.Contains(check.Message, "wave") ||
			strings.Contains(check.Message, "bundle path") {
			return "slipway repair", true
		}
	}
	return "", false
}

func doctorPriority(finding state.HealthFinding) int {
	switch {
	case finding.Repairable && finding.Severity == model.ReasonSeverityError:
		return 10
	case !finding.Repairable && finding.Severity == model.ReasonSeverityError:
		return 20
	case finding.Repairable:
		return 30
	default:
		return 40
	}
}

func doctorResumeAction(root, changeSlug string) (*doctorAction, error) {
	change, err := resolveHealthChangeTarget(root, changeSlug)
	if err != nil {
		if errors.Is(err, state.ErrMultipleActiveChanges) {
			return nil, nil
		}
		return nil, err
	}
	if change == nil {
		return nil, nil
	}
	if change.Status != model.ChangeStatusActive || change.CurrentState != model.StateS2Execute {
		return nil, nil
	}
	execCtx, err := loadExecutionContext(root, *change)
	if err != nil {
		return nil, nil
	}
	if change.ActiveCheckpoint != nil {
		if err := validateActiveCheckpointAuthority(root, *change, execCtx, "doctor"); err != nil {
			return nil, nil
		}
		return &doctorAction{
			Priority:   90,
			Category:   "execution_resume",
			Slug:       change.Slug,
			Summary:    "Execution can continue once the active checkpoint receives a response",
			Command:    `slipway run --resume-response "<response>"`,
			Repairable: false,
		}, nil
	}

	if !execCtx.Ready {
		return nil, nil
	}
	_, resumeWaveIndex, err := loadResumableWaveExecution(root, *change, execCtx, "doctor")
	if err != nil {
		return nil, nil
	}
	if resumeWaveIndex == 0 {
		return nil, nil
	}
	return &doctorAction{
		Priority:   90,
		Category:   "execution_resume",
		Slug:       change.Slug,
		Summary:    "Governed execution can resume from the latest incomplete wave",
		Command:    "slipway run --resume",
		Repairable: false,
	}, nil
}

func normalizeHealthFindings(root string, findings []state.HealthFinding) []state.HealthFinding {
	if len(findings) == 0 {
		return nil
	}

	normalized := append([]state.HealthFinding(nil), findings...)
	for i := range normalized {
		if normalized[i].Repairable || !healthFindingRepairable(root, normalized[i]) {
			continue
		}
		normalized[i].Repairable = true
		if normalized[i].Category == "execution_summary" {
			normalized[i].RepairHint = "Run `slipway repair` to rebuild execution-summary.yaml from wave-backed execution evidence."
		}
	}
	return normalized
}

func healthFindingRepairable(root string, finding state.HealthFinding) bool {
	if finding.Category != "execution_summary" || strings.TrimSpace(finding.Slug) == "" {
		return false
	}
	record, found, err := progression.LatestPassingWaveEvidence(root, finding.Slug)
	return err == nil && found && record.RunVersion >= 1
}

func extractCommandHint(hint string) string {
	start := strings.Index(hint, "`")
	if start == -1 {
		return ""
	}
	end := strings.Index(hint[start+1:], "`")
	if end == -1 {
		return ""
	}
	command := strings.TrimSpace(hint[start+1 : start+1+end])
	if !strings.HasPrefix(command, "slipway ") {
		return ""
	}
	return command
}

func normalizeDoctorActions(actions []doctorAction) []doctorAction {
	slices.SortFunc(actions, func(a, b doctorAction) int {
		if a.Priority != b.Priority {
			return a.Priority - b.Priority
		}
		if a.Category != b.Category {
			return strings.Compare(a.Category, b.Category)
		}
		if a.Slug != b.Slug {
			return strings.Compare(a.Slug, b.Slug)
		}
		if a.Command != b.Command {
			return strings.Compare(a.Command, b.Command)
		}
		return strings.Compare(a.Summary, b.Summary)
	})

	deduped := make([]doctorAction, 0, len(actions))
	seen := map[string]struct{}{}
	for _, action := range actions {
		key := strings.Join([]string{
			fmt.Sprintf("%d", action.Priority),
			action.Category,
			action.Slug,
			action.Summary,
			action.Command,
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, action)
	}
	return deduped
}
