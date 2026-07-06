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
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type doneView struct {
	Slug                        string                               `json:"slug"`
	ExecutionMode               string                               `json:"execution_mode"`
	QualityMode                 string                               `json:"quality_mode,omitempty"`
	WorkflowPreset              string                               `json:"workflow_preset,omitempty"`
	EffectiveWorkflowPreset     string                               `json:"effective_workflow_preset,omitempty"`
	PresetUpgradeReasons        []string                             `json:"preset_upgrade_reasons,omitempty"`
	NeedsDiscovery              bool                                 `json:"needs_discovery,omitempty"`
	InvocationRoute             *invocationRouteView                 `json:"invocation_route,omitempty"`
	EvidenceFreshness           string                               `json:"evidence_freshness,omitempty"`
	ExecutionEvidenceFreshness  string                               `json:"execution_evidence_freshness,omitempty"`
	GovernanceEvidenceFreshness string                               `json:"governance_evidence_freshness,omitempty"`
	OverallReadinessFreshness   string                               `json:"overall_readiness_freshness,omitempty"`
	FreshnessDiagnostics        *state.ExecutionFreshnessDiagnostics `json:"freshness_diagnostics,omitempty"`
	Status                      string                               `json:"status"`
	Archived                    bool                                 `json:"archived"`
	ArchivePath                 string                               `json:"archive_path,omitempty"`
	ArchiveKind                 string                               `json:"archive_kind,omitempty"`
	ArchiveCommitRequired       bool                                 `json:"archive_commit_required,omitempty"`
	WorktreeDirtyWarning        string                               `json:"worktree_dirty_warning,omitempty"`
	WorktreeDirtyFiles          []string                             `json:"worktree_dirty_files,omitempty"`
	RemediationSources          []model.ArchiveReference             `json:"remediation_sources,omitempty"`
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

// doneArchiveStep identifies which step of the shared done-archive helper failed,
// so each caller can surface the failure in its own shape: interactive `done`
// propagates the raw error, while `--all-ready` maps the step to a distinct
// reason-coded bulk item.
type doneArchiveStep int

const (
	doneArchiveStepNone doneArchiveStep = iota
	doneArchiveStepLifecycleEvent
	doneArchiveStepArchive
)

// finalizeDoneArchive performs the terminal done transition shared by interactive
// `done` and bulk `--all-ready`: it marks the change done, emits the canonical
// `done.marked` lifecycle event (snapshotting the pre-mark state for BeforeState),
// and archives the change, returning the archived snapshot. Callers own their own
// surrounding concerns — remediation merge, display-path resolution, and
// error-to-surface mapping — before and after this call. On failure it returns the
// raw underlying error together with the step that failed.
func finalizeDoneArchive(root string, change *model.Change) (model.Change, doneArchiveStep, error) {
	beforeState := change.CurrentState
	markChangeDone(change)
	if err := appendCLILifecycleEvent(root, *change, state.LifecycleEvent{
		Command:     "done",
		EventType:   "done.marked",
		Action:      "archived",
		Reason:      "operator_finalized_done_ready",
		Result:      string(model.ChangeStatusDone),
		GateID:      string(gate.GateShip),
		BeforeState: beforeState,
		AfterState:  change.CurrentState,
	}); err != nil {
		return model.Change{}, doneArchiveStepLifecycleEvent, err
	}
	archived, err := state.ArchiveChange(root, *change, model.ChangeStatusDone)
	if err != nil {
		return model.Change{}, doneArchiveStepArchive, err
	}
	return archived, doneArchiveStepNone, nil
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

func detectRemediationSources(root string, change model.Change) ([]model.ArchiveReference, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return nil, err
	}
	refs := []model.ArchiveReference{}
	walkErr := filepath.WalkDir(paths.GovernedBundleDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			return nil
		}
		// Skip symlinks and other non-regular entries: ReadFileNoSymlink
		// intentionally refuses them, and that refusal is a deliberate skip
		// rather than an unreadable-file failure. WalkDir does not follow
		// symlinks, so they arrive here with a nil walk error and a
		// symlink-typed entry.
		if !entry.Type().IsRegular() {
			return nil
		}
		data, readErr := fsutil.ReadFileNoSymlink(path)
		if readErr != nil {
			return readErr
		}
		refs = append(refs, archiveReferencesFromText(string(data))...)
		return nil
	})
	// A missing GovernedBundleDir is tolerated (baseline behavior): there is
	// simply nothing to detect. Genuine read/permission errors on existing
	// files still surface (C3), so only the not-exist case is skipped.
	if walkErr != nil && !errors.Is(walkErr, fs.ErrNotExist) {
		return nil, walkErr
	}
	return mergeArchiveReferences(nil, existingArchiveReferences(root, refs)), nil
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

			readCtx := newStateReadContext(root)
			active, err := resolveActiveChangeRefWithReadContext(readCtx, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, active.Slug, "done", func() error {
				change, err := readCtx.reloadChange(active.Slug)
				if err != nil {
					return err
				}
				if change.Status != model.ChangeStatusActive {
					return newPreconditionError(
						"not_active",
						fmt.Sprintf("cannot finalize non-active change status %q", change.Status),
						"Only active changes can be finalized.",
						change.Slug,
						map[string]any{"status": string(change.Status)},
					)
				}
				route := commandInvocationRoute(cmd, root, change, strings.TrimSpace(changeSlug) != "")

				if !action.CanFinalizeDone(change.CurrentState) {
					return newGovernanceBlockedError(
						"not_done_ready",
						"governed change is not done-ready",
						"Complete all governance stages before finalizing.",
						active.Slug,
						nil,
					)
				}
				prep, _, err := prepareDoneFinalize(root, &change)
				if err != nil {
					return err
				}
				if prep.shipBlocked {
					return shipGateBlockedError(root, change, prep.shipEval)
				}
				worktreeDirtyWarning, worktreeDirtyFiles := prep.worktreeDirtyWarning, prep.worktreeDirtyFiles
				doneFreshnessChange := change
				doneExecCtx, err := loadExecutionContext(root, change)
				if err != nil {
					return err
				}
				doneVerificationRecords, err := readCtx.verificationRecords(change)
				if err != nil {
					return wrapGovernanceReadinessError("evaluate done freshness", change.Slug, err)
				}
				doneReadiness, err := progression.EvaluateGovernanceReadiness(
					root,
					change,
					progression.GovernanceReadinessOptions{
						IncludeGateEvaluations: true,
						VerificationRecords:    doneVerificationRecords,
					},
				)
				if err != nil {
					return wrapGovernanceReadinessError("evaluate done freshness", change.Slug, err)
				}
				detectedRemediation, err := detectRemediationSources(root, change)
				if err != nil {
					return err
				}
				change.RemediationSources = mergeArchiveReferences(
					change.RemediationSources,
					detectedRemediation,
				)
				archivePaths, err := state.ResolveChangePaths(root, change)
				if err != nil {
					return err
				}
				archived, _, err := finalizeDoneArchive(root, &change)
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
					InvocationRoute:         route,
					Status:                  string(model.ChangeStatusDone),
					Archived:                true,
					ArchivePath:             state.DisplayPath(root, archivePaths.GovernedBundleArchive),
					ArchiveKind:             archiveKind,
					ArchiveCommitRequired:   strings.TrimSpace(change.WorktreePath) != "",
					WorktreeDirtyWarning:    worktreeDirtyWarning,
					WorktreeDirtyFiles:      worktreeDirtyFiles,
					RemediationSources:      archived.RemediationSources,
				}
				applyReadinessFreshnessToDone(
					root,
					&view,
					doneFreshnessChange,
					doneExecCtx.Summary,
					doneReadiness,
					doneReadiness.Blockers,
				)
				applyCommandInvocationWorkspacePath(cmd, root, view.FreshnessDiagnostics)

				return encodeJSONResponse(cmd, view)
			})
		},
	}
	cmd.Flags().BoolVar(&allReady, "all-ready", false, "Archive every active change that is done-ready")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
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
	prep, step, err := prepareDoneFinalize(root, &change)
	if err != nil {
		switch step {
		case doneFinalizeStepReconcile:
			return newDoneBulkFailed(slug, "artifact_reconcile_failed", err.Error())
		case doneFinalizeStepValidate:
			return newDoneBulkFailed(slug, "artifact_validation_failed", err.Error())
		default:
			return doneBulkFailedFromError(slug, err)
		}
	}
	if prep.shipBlocked {
		return newDoneBulkSkippedWithReasonCodes(
			slug,
			string(change.CurrentState),
			"ship_gate_blocked",
			append([]model.ReasonCode(nil), prep.shipEval.ReasonCodes...),
		)
	}
	worktreeDirtyWarning, worktreeDirtyFiles := prep.worktreeDirtyWarning, prep.worktreeDirtyFiles
	if _, step, err := finalizeDoneArchive(root, &change); err != nil {
		if step == doneArchiveStepLifecycleEvent {
			return newDoneBulkFailed(slug, "lifecycle_event_write_failed", err.Error())
		}
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

// doneFinalizeStep identifies which step of the shared pre-archive finalize
// sequence failed, so each caller can surface the failure in its own error
// shape (a raw error propagated by interactive `done`, or a reason-coded bulk
// item emitted by `--all-ready`).
type doneFinalizeStep int

const (
	doneFinalizeStepNone doneFinalizeStep = iota
	doneFinalizeStepReconcile
	doneFinalizeStepValidate
	doneFinalizeStepShipGate
)

// doneFinalizePrep carries the results of the shared pre-archive finalize
// sequence back to each call site.
type doneFinalizePrep struct {
	worktreeDirtyWarning string
	worktreeDirtyFiles   []string
	shipEval             gate.GateEvaluation
	shipBlocked          bool
}

// prepareDoneFinalize runs the finalize steps shared by interactive `done` and
// bulk `--all-ready` archive: it snapshots the dirty-worktree advisory,
// reconciles the governed bundle to the filesystem, validates governed
// artifacts, and refreshes the ship gate. change is mutated exactly as the
// inline sequences did (reconcile persists the reconciled bundle; ship-gate
// refresh re-evaluates against it). On failure it returns the raw underlying
// error together with the step that failed, leaving error-to-surface mapping to
// each caller. On success step is doneFinalizeStepNone and prep.shipBlocked
// reports whether the refreshed ship gate blocks finalization (prep.shipEval
// carries the evaluation).
func prepareDoneFinalize(root string, change *model.Change) (doneFinalizePrep, doneFinalizeStep, error) {
	prep := doneFinalizePrep{}
	prep.worktreeDirtyWarning, prep.worktreeDirtyFiles = doneWorktreeDirtyState(root, *change)
	if err := reconcileDoneFilesystemState(root, change); err != nil {
		return prep, doneFinalizeStepReconcile, err
	}
	if err := validateGovernedDoneArtifacts(root, *change); err != nil {
		return prep, doneFinalizeStepValidate, err
	}
	shipEval, shipBlocked, err := refreshDoneShipGate(root, change)
	if err != nil {
		return prep, doneFinalizeStepShipGate, err
	}
	prep.shipEval = shipEval
	prep.shipBlocked = shipBlocked
	return prep, doneFinalizeStepNone, nil
}

func shipGateBlockedError(root string, change model.Change, eval gate.GateEvaluation) error {
	reasonCodes := append([]model.ReasonCode(nil), eval.ReasonCodes...)
	remediation := "Refresh verification evidence, resolve ship gate blockers, and rerun `slipway done`."
	if staleTarget, staleAvailable, staleErr := progression.StaleEvidenceRepairAvailable(root, change, reasonCodes); staleErr == nil && staleAvailable {
		reasonCodes = append(reasonCodes, model.NewReasonCode("review_alignment_required", staleTarget.SkillName))
		remediation = "Realign stale evidence through review alignment for " + staleTarget.Label() + ", refresh affected S2+ evidence, then rerun `slipway done`."
	}
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
		remediation,
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
	assurancePath := artifact.ResolveArtifactPath(paths.GovernedBundleDir, "assurance.md")
	content, err := os.ReadFile(assurancePath) // #nosec G304 -- path is resolved from CLI/project authority before this read.
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
