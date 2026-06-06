package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/control"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func executeGovernedPivot(root, slug, kind string) (pivotView, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return pivotView{}, err
	}
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return pivotView{}, err
	}
	changeBeforePivot := change
	if err := validatePivotPreconditions(kind, change.CurrentState); err != nil {
		return pivotView{}, err
	}

	kindEnum := gate.PivotKind(kind)
	eval := gate.EvaluateGPivot(kindEnum, true, change.CurrentState)
	if eval.Status != model.GateStatusApproved {
		return pivotView{}, newGovernanceBlockedErrorWithReasons(
			"pivot_blocked",
			fmt.Sprintf("pivot blocked: %s", strings.Join(model.ReasonSpecs(eval.ReasonCodes), ", ")),
			"Resolve blockers and rerun the command.",
			"",
			eval.ReasonCodes,
			nil,
		)
	}

	// Reroute and rescope both preserve GuardrailDomain while forcing discovery
	// re-entry for conservative re-planning.
	change.NeedsDiscovery = true
	change.ArtifactSchema = progression.ResolveFrozenArtifactSchema(change.ArtifactSchema, cfg.Defaults.ArtifactSchema, change.NeedsDiscovery)

	// Pivot reset: clear execution residue.
	change.ResetPivotExecutionResidue()
	if err := state.RelocateGovernedBundle(root, changeBeforePivot, change); err != nil {
		return pivotView{}, err
	}

	if kindEnum == gate.PivotKindRescope {
		// Rescope returns to S0_INTAKE/clarify for intent re-evaluation.
		// Clear Approved Summary so the user must re-confirm after amendment.
		if err := clearApprovedSummaryForRescope(root, change); err != nil {
			return pivotView{}, err
		}
		change.EnterIntake()
	} else {
		// Reroute returns to S1_PLAN.
		change.EnterPlanning(change.NeedsDiscovery)
	}
	if err := state.SaveChange(root, change); err != nil {
		return pivotView{}, err
	}
	if err := state.RemoveExecutionSummary(root, change.Slug); err != nil {
		return pivotView{}, err
	}
	if err := state.ResetWaveExecution(root, change.Slug); err != nil {
		return pivotView{}, err
	}
	if err := clearPivotRuntimeResidue(root, change.Slug); err != nil {
		return pivotView{}, err
	}
	if err := os.RemoveAll(filepath.Dir(state.TaskPIDFilePath(root, change.Slug))); err != nil {
		return pivotView{}, err
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return pivotView{}, err
	}
	if err := deactivateGovernanceControlsForPivot(root, changeBeforePivot, paths.GovernedBundleDir); err != nil {
		return pivotView{}, err
	}
	// Snapshot recompute is best-effort during pivot; failure is non-blocking.
	_, _ = governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)

	return pivotView{
		Slug:          slug,
		Kind:          kind,
		ExecutionMode: governedExecutionMode,
		CurrentState:  string(change.CurrentState),
	}, nil
}

func clearPivotRuntimeResidue(root, slug string) error {
	runtimeDir := state.ChangeDir(root, slug)
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(runtimeDir, entry.Name())
		if entry.Name() == "evidence" {
			if err := clearPivotEvidenceResidue(path); err != nil {
				return err
			}
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func clearPivotEvidenceResidue(evidenceDir string) error {
	entries, err := os.ReadDir(evidenceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "tasks" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(evidenceDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// clearApprovedSummaryForRescope implements the rescope artifact semantics from
// the design doc: preserve existing intent.md content but clear the Approved
// Summary section so the user must re-confirm after amendment.
func clearApprovedSummaryForRescope(root string, change model.Change) error {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}
	intentPath := filepath.Join(paths.GovernedBundleDir, "intent.md")
	data, err := os.ReadFile(intentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no intent.md to clear
		}
		return err
	}
	content := string(data)

	// Find and clear the Approved Summary section content while keeping the heading.
	const heading = "## Approved Summary"
	idx := strings.Index(content, heading)
	if idx < 0 {
		return nil // no Approved Summary section
	}

	afterHeading := idx + len(heading)
	rest := content[afterHeading:]
	nextHeading := strings.Index(rest, "\n## ")
	var sectionEnd int
	if nextHeading >= 0 {
		sectionEnd = afterHeading + nextHeading
	} else {
		sectionEnd = len(content)
	}

	// Replace section body with a reopened marker.
	cleared := content[:afterHeading] +
		"\n<!-- Cleared by rescope pivot — re-confirm after amendment -->\n" +
		content[sectionEnd:]

	return os.WriteFile(intentPath, []byte(cleared), 0o644)
}

func deactivateGovernanceControlsForPivot(root string, change model.Change, bundleDir string) error {
	snap, err := governance.LoadSnapshot(root, change.Slug)
	if err != nil {
		if _, backupErr := governance.BackupUnreadableSnapshot(root, change.Slug, time.Now().UTC()); backupErr != nil {
			return backupErr
		}
		snap, err = governance.PreviewGovernanceSnapshot(root, change, bundleDir)
		if err != nil {
			return err
		}
	}
	if snap.Version == 0 || len(snap.ActiveControls) == 0 {
		return nil
	}

	active := append([]model.ControlActivation(nil), snap.ActiveControls...)

	for _, ctrl := range snap.ActiveControls {
		active = control.DeactivateControl(active, ctrl.ControlID)
	}

	snap.ActiveControls = active
	snap.ComputedAt = time.Now().UTC()
	return governance.SaveSnapshot(root, change.Slug, snap)
}
