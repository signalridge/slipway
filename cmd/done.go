package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/action"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type doneView struct {
	Slug                    string   `json:"slug"`
	ExecutionMode           string   `json:"execution_mode"`
	QualityMode             string   `json:"quality_mode,omitempty"`
	WorkflowPreset          string   `json:"workflow_preset,omitempty"`
	EffectiveWorkflowPreset string   `json:"effective_workflow_preset,omitempty"`
	PresetUpgradeReasons    []string `json:"preset_upgrade_reasons,omitempty"`
	NeedsDiscovery          bool     `json:"needs_discovery,omitempty"`
	Status                  string   `json:"status"`
	Archived                bool     `json:"archived"`
}

type doneBulkItem struct {
	Slug        string             `json:"slug,omitempty"`
	Status      string             `json:"status,omitempty"`
	Reason      string             `json:"reason,omitempty"`
	ErrorDetail string             `json:"error_detail,omitempty"`
	ReasonCodes []model.ReasonCode `json:"reason_codes,omitempty"`
}

type doneBulkView struct {
	Archived []doneBulkItem `json:"archived"`
	Skipped  []doneBulkItem `json:"skipped,omitempty"`
	Failed   []doneBulkItem `json:"failed,omitempty"`
}

func newDoneBulkArchived(slug string) doneBulkItem {
	return doneBulkItem{
		Slug:   slug,
		Status: string(model.ChangeStatusDone),
	}
}

func newDoneBulkSkipped(slug, status, reason string) doneBulkItem {
	return newDoneBulkSkippedWithReasonCodes(slug, status, reason, nil)
}

func newDoneBulkSkippedWithReasonCodes(slug, status, reason string, reasonCodes []model.ReasonCode) doneBulkItem {
	if len(reasonCodes) == 0 {
		reasonCodes = []model.ReasonCode{model.NewReasonCode(reason, "")}
	}
	return doneBulkItem{
		Slug:        slug,
		Status:      status,
		Reason:      reason,
		ReasonCodes: model.NormalizeReasonCodes(reasonCodes),
	}
}

func newDoneBulkFailed(slug, reason, errorDetail string) doneBulkItem {
	return doneBulkItem{
		Slug:        slug,
		Reason:      reason,
		ErrorDetail: errorDetail,
		ReasonCodes: []model.ReasonCode{model.NewReasonCode(reason, "")},
	}
}

func markChangeDone(change *model.Change) {
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
}

func sortDoneBulkItems(items []doneBulkItem) {
	slices.SortFunc(items, func(a, b doneBulkItem) int {
		if a.Slug != b.Slug {
			if a.Slug < b.Slug {
				return -1
			}
			return 1
		}
		if a.Reason != b.Reason {
			if a.Reason < b.Reason {
				return -1
			}
			return 1
		}
		if a.Status != b.Status {
			if a.Status < b.Status {
				return -1
			}
			return 1
		}
		if a.ErrorDetail < b.ErrorDetail {
			return -1
		}
		if a.ErrorDetail > b.ErrorDetail {
			return 1
		}
		return 0
	})
}

func sortDoneBulkView(view *doneBulkView) {
	sortDoneBulkItems(view.Archived)
	sortDoneBulkItems(view.Skipped)
	sortDoneBulkItems(view.Failed)
}

func makeDoneCmd() *cobra.Command {
	var allReady bool
	var changeSlug string
	cmd := &cobra.Command{
		Use:   "done",
		Short: "Finalize a done-ready change and archive it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			if allReady && changeSlug != "" {
				return newInvalidUsageError(
					"flag_conflict",
					"--all-ready cannot be used with --change",
					"Use --all-ready for bulk archive or --change <slug> for a single change, not both.",
					nil,
				)
			}
			if allReady {
				view := archiveAllDoneReady(root)
				return encodeJSONResponse(cmd, view)
			}

			active, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, active.Slug, "done", func() error {
				change, err := loadActiveChange(
					root,
					active.Slug,
					"cannot finalize non-active change status %q",
					"Only active changes can be finalized.",
				)
				if err != nil {
					return err
				}

				if !action.CanFinalizeDone(change.CurrentState) {
					return newGovernanceBlockedError(
						"not_done_ready",
						"governed change is not done-ready",
						"Complete all governance stages before finalizing.",
						active.Slug,
						nil,
					)
				}
				if err := reconcileDoneFilesystemState(root, &change); err != nil {
					return err
				}
				if err := validateGovernedDoneArtifacts(root, change); err != nil {
					return err
				}
				shipEval, shipBlocked, err := refreshDoneShipGate(root, &change)
				if err != nil {
					return err
				}
				if shipBlocked {
					return shipGateBlockedError(change, shipEval)
				}
				markChangeDone(&change)

				if _, err := state.ArchiveChange(root, change, model.ChangeStatusDone); err != nil {
					return err
				}
				profile := buildChangeProfileView(change)
				presetFields, err := buildWorkflowPresetView(root, change)
				if err != nil {
					return err
				}
				view := doneView{
					Slug:                    active.Slug,
					ExecutionMode:           governedExecutionMode,
					QualityMode:             profile.QualityMode,
					WorkflowPreset:          presetFields.WorkflowPreset,
					EffectiveWorkflowPreset: presetFields.EffectiveWorkflowPreset,
					PresetUpgradeReasons:    presetFields.PresetUpgradeReasons,
					NeedsDiscovery:          profile.NeedsDiscovery,
					Status:                  string(model.ChangeStatusDone),
					Archived:                true,
				}

				return encodeJSONResponse(cmd, view)
			})
		},
	}
	cmd.Flags().BoolVar(&allReady, "all-ready", false, "Archive every active change that is done-ready")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func archiveAllDoneReady(root string) doneBulkView {
	view := doneBulkView{
		Archived: []doneBulkItem{},
		Skipped:  []doneBulkItem{},
		Failed:   []doneBulkItem{},
	}

	changes, err := state.ListChanges(root)
	if err != nil {
		view.Failed = append(view.Failed, newDoneBulkFailed("", "list_changes_failed", err.Error()))
		return view
	}

	for _, change := range changes {
		if change.Status != model.ChangeStatusActive {
			continue
		}
		item, ok := archiveSingleDoneReadyUnderLock(root, change.Slug)
		if !ok {
			continue
		}
		appendDoneBulkResult(&view, item)
	}

	sortDoneBulkView(&view)
	return view
}

func appendDoneBulkResult(view *doneBulkView, item doneBulkItem) {
	switch {
	case item.Reason == "":
		view.Archived = append(view.Archived, item)
	case item.Status != "":
		view.Skipped = append(view.Skipped, item)
	default:
		view.Failed = append(view.Failed, item)
	}
}

func archiveSingleDoneReadyUnderLock(root, slug string) (doneBulkItem, bool) {
	item := doneBulkItem{}
	ok := false

	err := withChangeStateLock(root, slug, "done", func() error {
		change, err := state.LoadChange(root, slug)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			item = newDoneBulkFailed(slug, "load_change_failed", err.Error())
			ok = true
			return nil
		}
		if change.Status != model.ChangeStatusActive {
			item = newDoneBulkSkipped(slug, string(change.Status), "change_not_active")
			ok = true
			return nil
		}

		item = archiveSingleDoneReady(root, slug, change)
		ok = true
		return nil
	})
	if err != nil {
		return doneBulkFailedFromError(slug, err), true
	}
	return item, ok
}

func doneBulkFailedFromError(slug string, err error) doneBulkItem {
	cliErr := asCLIError(err)
	if cliErr == nil {
		return newDoneBulkFailed(slug, "archive_failed", err.Error())
	}

	detail := cliErr.Message
	if detail == "" {
		detail = err.Error()
	}
	reason := cliErr.ErrorCode
	if reason == "" {
		reason = "archive_failed"
	}
	return newDoneBulkFailed(slug, reason, detail)
}

func archiveSingleDoneReady(root, slug string, change model.Change) doneBulkItem {
	if !action.CanFinalizeDone(change.CurrentState) {
		return newDoneBulkSkipped(slug, string(change.CurrentState), "not_done_ready")
	}
	if err := reconcileDoneFilesystemState(root, &change); err != nil {
		return newDoneBulkFailed(slug, "artifact_reconcile_failed", err.Error())
	}
	if err := validateGovernedDoneArtifacts(root, change); err != nil {
		return newDoneBulkFailed(slug, "artifact_validation_failed", err.Error())
	}
	shipEval, shipBlocked, err := refreshDoneShipGate(root, &change)
	if err != nil {
		return doneBulkFailedFromError(slug, err)
	}
	if shipBlocked {
		return newDoneBulkSkippedWithReasonCodes(
			slug,
			string(change.CurrentState),
			"ship_gate_blocked",
			append([]model.ReasonCode(nil), shipEval.ReasonCodes...),
		)
	}
	markChangeDone(&change)

	if _, err := state.ArchiveChange(root, change, model.ChangeStatusDone); err != nil {
		return newDoneBulkFailed(slug, "archive_failed", err.Error())
	}
	return newDoneBulkArchived(slug)
}

func reconcileDoneFilesystemState(root string, change *model.Change) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	policy, err := governance.ResolvePresetPolicy(root, *change)
	if err != nil {
		return err
	}
	if err := artifact.ReconcileFromFilesystem(root, change, policy.EffectivePreset); err != nil {
		return err
	}
	return state.SaveChange(root, *change)
}

func refreshDoneShipGate(root string, change *model.Change) (gate.GateEvaluation, bool, error) {
	if change == nil {
		return gate.GateEvaluation{}, false, fmt.Errorf("change is required")
	}
	eval, err := progression.EvaluateShipGate(root, *change)
	if err != nil {
		return gate.GateEvaluation{}, false, err
	}
	if eval.Status != model.GateStatusBlocked {
		return eval, false, nil
	}
	return eval, true, nil
}

func shipGateBlockedError(change model.Change, eval gate.GateEvaluation) error {
	reasonCodes := append([]model.ReasonCode(nil), eval.ReasonCodes...)
	reasons := model.ReasonSpecs(reasonCodes)
	message := "fresh G_ship check blocked finalization"
	if len(reasons) > 0 {
		message = fmt.Sprintf("%s: %s", message, strings.Join(reasons, ", "))
	}
	details := map[string]any{
		"gate_id": string(gate.GateShip),
	}
	if len(reasons) > 0 {
		details["reasons"] = reasons
	}
	if len(reasonCodes) == 0 {
		reasonCodes = []model.ReasonCode{model.NewReasonCode("ship_gate_blocked", "")}
	}
	return newGovernanceBlockedErrorWithReasons(
		"ship_gate_blocked",
		message,
		"Refresh verification evidence, resolve ship gate blockers, and rerun `slipway done`.",
		change.Slug,
		reasonCodes,
		details,
	)
}

func validateGovernedDoneArtifacts(root string, change model.Change) error {
	// Light effective preset: assurance.md is optional.
	// Uses EffectivePreset so min_preset and guardrail-domain upgrades are respected.
	if policy, err := governance.ResolvePresetPolicy(root, change); err == nil && policy.EffectivePreset == model.WorkflowPresetLight {
		return nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return err
	}
	assurancePath := artifact.ResolveArtifactPath(paths.GovernedBundleDir, change.Slug, "assurance.md")
	content, err := os.ReadFile(assurancePath)
	if err != nil {
		return newGovernanceBlockedError(
			"assurance_missing",
			"assurance.md not found or unreadable",
			"Create and fill assurance.md before finalizing.",
			change.Slug,
			nil,
		)
	}
	if err := artifact.ValidateAssuranceStructure(string(content)); err != nil {
		return newGovernanceBlockedError(
			"assurance_invalid",
			fmt.Sprintf("assurance.md validation failed: %v", err),
			"Fill all required sections in assurance.md.",
			change.Slug,
			nil,
		)
	}

	return nil
}
