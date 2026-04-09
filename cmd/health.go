package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type healthView struct {
	ExecutionMode string                             `json:"execution_mode"`
	Findings      []state.HealthFinding              `json:"findings,omitempty"`
	Governance    *governance.GovernanceHealthReport `json:"governance,omitempty"`
	Observations  []model.SignalObservation          `json:"observations,omitempty"`
	Diagnostics   []string                           `json:"diagnostics,omitempty"`
	ShowRepo      bool                               `json:"-"`
}

func makeHealthCmd() *cobra.Command {
	var jsonOutput bool
	var governanceFlag bool
	var allFlag bool
	var observationsFlag bool
	var changeSlug string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show repo-local integrity and repairability findings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			showRepo := !governanceFlag || allFlag
			showGov := governanceFlag || allFlag

			view := healthView{
				ExecutionMode: "diagnostics",
				ShowRepo:      showRepo,
			}

			// Repo health (default behavior preserved).
			if showRepo {
				report, err := state.CollectHealthReport(root)
				if err != nil {
					return err
				}
				view.Findings = report.Findings
			}

			// Governance health.
			if showGov {
				change, err := resolveHealthChangeTarget(root, changeSlug)
				if err != nil {
					return fmt.Errorf("governance health: %w", err)
				}
				if change != nil {
					var govReport governance.GovernanceHealthReport
					var persistedSnap model.GovernanceSnapshot
					var snap model.GovernanceSnapshot
					err := withChangeStateLock(root, change.Slug, "health", func() error {
						latest, err := state.LoadChange(root, change.Slug)
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
				} else {
					view.Diagnostics = append(view.Diagnostics, "No active change; governance health not applicable.")
				}
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
		change, err := loadChangeBySlug(root, slug)
		if err != nil {
			return nil, err
		}
		return &change, nil
	}
	change, err := state.FindActiveChange(root)
	if err != nil {
		if errors.Is(err, state.ErrNoActiveChange) {
			return nil, nil
		}
		// Surface real errors (multiple active changes, parse failures, etc.)
		// instead of silently treating them as "no active change".
		return nil, err
	}
	return &change, nil
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
