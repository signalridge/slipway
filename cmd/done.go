package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	Slug                    string                   `json:"slug"`
	ExecutionMode           string                   `json:"execution_mode"`
	QualityMode             string                   `json:"quality_mode,omitempty"`
	WorkflowPreset          string                   `json:"workflow_preset,omitempty"`
	EffectiveWorkflowPreset string                   `json:"effective_workflow_preset,omitempty"`
	PresetUpgradeReasons    []string                 `json:"preset_upgrade_reasons,omitempty"`
	NeedsDiscovery          bool                     `json:"needs_discovery,omitempty"`
	Status                  string                   `json:"status"`
	Archived                bool                     `json:"archived"`
	ArchivePath             string                   `json:"archive_path,omitempty"`
	ArchiveKind             string                   `json:"archive_kind,omitempty"`
	ArchiveCommitRequired   bool                     `json:"archive_commit_required,omitempty"`
	WorktreeDirtyWarning    string                   `json:"worktree_dirty_warning,omitempty"`
	WorktreeDirtyFiles      []string                 `json:"worktree_dirty_files,omitempty"`
	RemediationSources      []model.ArchiveReference `json:"remediation_sources,omitempty"`
}

type doneBulkItem struct {
	Slug                 string             `json:"slug,omitempty"`
	Status               string             `json:"status,omitempty"`
	Reason               string             `json:"reason,omitempty"`
	ErrorDetail          string             `json:"error_detail,omitempty"`
	ReasonCodes          []model.ReasonCode `json:"reason_codes,omitempty"`
	WorktreeDirtyWarning string             `json:"worktree_dirty_warning,omitempty"`
	WorktreeDirtyFiles   []string           `json:"worktree_dirty_files,omitempty"`
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

const doneWorktreeDirtyWarning = "worktree has uncommitted non-bundle changes; commit them together with the archived bundle before removing the worktree"

// doneWorktreeDirtyState reports uncommitted non-bundle worktree changes as a
// non-blocking advisory. `done` proceeds and archives — premature worktree
// deletion is already refused by `git worktree remove` on a dirty tree — but the
// returned warning and file list tell the operator what to commit alongside the
// archived bundle. The active governed bundle is exempt because `done` rewrites
// it; sibling or archived bundles are reported (see WorkspaceChangedFilesForDoneArchive).
func doneWorktreeDirtyState(root string, change model.Change) (string, []string) {
	if strings.TrimSpace(change.WorktreePath) == "" {
		return "", nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return "", nil
	}
	files := progression.WorkspaceChangedFilesForDoneArchive(paths)
	if len(files) == 0 {
		return "", nil
	}
	return doneWorktreeDirtyWarning, files
}

func detectRemediationSources(root string, change model.Change) []model.ArchiveReference {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return nil
	}
	refs := []model.ArchiveReference{}
	_ = filepath.WalkDir(paths.GovernedBundleDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !(strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		refs = append(refs, archiveReferencesFromText(string(data))...)
		return nil
	})
	return mergeArchiveReferences(nil, existingArchiveReferences(root, refs))
}

func archiveReferencesFromText(text string) []model.ArchiveReference {
	const marker = "artifacts/changes/archived/"
	refs := []model.ArchiveReference{}
	for {
		idx := strings.Index(text, marker)
		if idx < 0 {
			break
		}
		rest := text[idx+len(marker):]
		slug := leadingArchiveSlug(rest)
		if slug != "" {
			refs = append(refs, model.ArchiveReference{
				Slug:     slug,
				Path:     marker + slug,
				Relation: "remediates",
			})
		}
		if len(rest) == 0 {
			break
		}
		text = rest
	}
	return refs
}

func leadingArchiveSlug(text string) string {
	text = strings.TrimLeft(text, "`'\"<([")
	end := len(text)
	for i, r := range text {
		if r == '/' || r == '\\' || r == '`' || r == '\'' || r == '"' || r == '<' || r == ')' || r == ']' || r == ' ' || r == '\n' || r == '\t' {
			end = i
			break
		}
	}
	return strings.TrimSpace(text[:end])
}

func mergeArchiveReferences(existing, detected []model.ArchiveReference) []model.ArchiveReference {
	if len(existing) == 0 && len(detected) == 0 {
		return nil
	}
	combined := append(append([]model.ArchiveReference{}, existing...), detected...)
	change := model.Change{RemediationSources: combined}
	change.Normalize()
	return change.RemediationSources
}

func existingArchiveReferences(root string, refs []model.ArchiveReference) []model.ArchiveReference {
	if len(refs) == 0 {
		return nil
	}
	filtered := make([]model.ArchiveReference, 0, len(refs))
	for _, ref := range refs {
		if ref.Path == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil || !info.IsDir() {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered
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
		Short: desc("done"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
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
				worktreeDirtyWarning, worktreeDirtyFiles := doneWorktreeDirtyState(root, change)
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
				beforeChange := change
				markChangeDone(&change)
				change.RemediationSources = mergeArchiveReferences(
					change.RemediationSources,
					detectRemediationSources(root, change),
				)

				if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
					Command:     "done",
					EventType:   "done.marked",
					Action:      "archived",
					Reason:      "operator_finalized_done_ready",
					Result:      string(model.ChangeStatusDone),
					GateID:      string(gate.GateShip),
					BeforeState: beforeChange.CurrentState,
					AfterState:  change.CurrentState,
				}); err != nil {
					return err
				}
				archivePaths, err := state.ResolveChangePaths(root, change)
				if err != nil {
					return err
				}
				archived, err := state.ArchiveChange(root, change, model.ChangeStatusDone)
				if err != nil {
					return err
				}
				profile := buildChangeProfileView(change)
				presetFields, err := buildWorkflowPresetView(root, change)
				if err != nil {
					return err
				}
				archiveKind := "terminal"
				if len(archived.RemediationSources) > 0 {
					archiveKind = "remediation"
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
					ArchivePath:             state.DisplayPath(root, archivePaths.GovernedBundleArchive),
					ArchiveKind:             archiveKind,
					ArchiveCommitRequired:   strings.TrimSpace(change.WorktreePath) != "",
					WorktreeDirtyWarning:    worktreeDirtyWarning,
					WorktreeDirtyFiles:      worktreeDirtyFiles,
					RemediationSources:      archived.RemediationSources,
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
	worktreeDirtyWarning, worktreeDirtyFiles := doneWorktreeDirtyState(root, change)
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
	beforeChange := change
	markChangeDone(&change)

	if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
		Command:     "done",
		EventType:   "done.marked",
		Action:      "archived",
		Reason:      "operator_finalized_done_ready",
		Result:      string(model.ChangeStatusDone),
		GateID:      string(gate.GateShip),
		BeforeState: beforeChange.CurrentState,
		AfterState:  change.CurrentState,
	}); err != nil {
		return newDoneBulkFailed(slug, "lifecycle_event_write_failed", err.Error())
	}
	if _, err := state.ArchiveChange(root, change, model.ChangeStatusDone); err != nil {
		return newDoneBulkFailed(slug, "archive_failed", err.Error())
	}
	item := newDoneBulkArchived(slug)
	item.WorktreeDirtyWarning = worktreeDirtyWarning
	item.WorktreeDirtyFiles = worktreeDirtyFiles
	return item
}

func reconcileDoneFilesystemState(root string, change *model.Change) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	policy, err := governance.ResolvePresetPolicy(root, *change)
	if err != nil {
		return err
	}
	if _, err := artifact.ReconcileFromFilesystem(root, change, policy.EffectivePreset); err != nil {
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
