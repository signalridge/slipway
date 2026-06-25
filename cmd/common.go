package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"

	"github.com/spf13/cobra"
)

// changeRef is the lightweight active-change selector used across cmd/.
type changeRef struct {
	Slug string
}

const governedExecutionMode = string(ctxpack.ExecutionModeGoverned)

type projectRootContextKey struct{}

func projectRootFromCommand(cmd *cobra.Command) (string, error) {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		if _, err := os.Stat(state.ConfigPath(root)); err == nil {
			return root, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		return "", fmt.Errorf("%w: workspace is not initialized; run `slipway init`", fsutil.ErrProjectRootNotFound)
	}
	return projectRootFromWD()
}

func workspaceRootFromCommandOrWD(cmd *cobra.Command) (string, error) {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		return root, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if normalized, err := state.NormalizePath(wd); err == nil {
		return normalized, nil
	}
	return filepath.Clean(wd), nil
}

func projectRootOverrideFromCommand(cmd *cobra.Command) (string, bool) {
	if cmd != nil {
		if root, ok := cmd.Context().Value(projectRootContextKey{}).(string); ok {
			root = strings.TrimSpace(root)
			if root != "" {
				normalized, err := state.NormalizePath(root)
				if err == nil {
					root = normalized
				} else {
					root = filepath.Clean(root)
				}
				return root, true
			}
		}
	}
	return "", false
}

func projectRootFromWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := fsutil.FindProjectRoot(wd)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(state.ConfigPath(root)); err == nil {
		return root, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return "", fmt.Errorf("%w: workspace is not initialized; run `slipway init`", fsutil.ErrProjectRootNotFound)
}

// invocationWorkspaceRoot resolves the git worktree where the current command
// is running. Adapter prompt/agent paths must follow the active invocation
// workspace, not always the canonical scope root.
func invocationWorkspaceRoot(projectRoot string) string {
	workspaceRoot := projectRoot
	wd, err := os.Getwd()
	if err != nil {
		return workspaceRoot
	}

	resolved, err := state.ResolveGitWorkspaceRoot(wd)
	if err != nil {
		return workspaceRoot
	}
	if resolved == "" {
		return workspaceRoot
	}
	if normalized, err := state.NormalizePath(resolved); err == nil {
		return normalized
	}
	return filepath.Clean(resolved)
}

func invocationWorkspaceRootFromCommand(cmd *cobra.Command, projectRoot string) string {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		return root
	}
	return invocationWorkspaceRoot(projectRoot)
}

func repairRootFromWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := fsutil.FindProjectRoot(wd)
	if err == nil && hasRepairableWorkspaceMarkers(root) {
		return root, nil
	}
	if err == nil {
		err = fmt.Errorf("%w: discovered project root %q has no slipway repair markers", fsutil.ErrProjectRootNotFound, root)
	}

	// Accept the cwd as a repair root when slipway-specific runtime or
	// artifact markers are already present.
	for dir := wd; ; dir = filepath.Dir(dir) {
		if hasRepairableWorkspaceMarkers(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", err
}

func repairRootFromCommand(cmd *cobra.Command) (string, error) {
	if root, ok := projectRootOverrideFromCommand(cmd); ok {
		if hasRepairableWorkspaceMarkers(root) {
			return root, nil
		}
		return "", fmt.Errorf("%w: provided project root %q has no slipway repair markers", fsutil.ErrProjectRootNotFound, root)
	}
	return repairRootFromWD()
}

func hasRepairableWorkspaceMarkers(root string) bool {
	for _, marker := range []string{
		state.GitStateDir(root),
		filepath.Join(root, "artifacts", "changes"),
		filepath.Join(root, "artifacts", "codebase"),
	} {
		info, err := os.Stat(marker)
		if err == nil && info.IsDir() {
			return true
		}
	}
	for _, marker := range []string{
		state.ConfigPath(root),
		state.ScopeMarkerPath(root),
	} {
		info, err := os.Stat(marker)
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func loadConfigAtRoot(root string) (model.Config, error) {
	return loadConfigAtRootWithStderr(root, os.Stderr)
}

func loadConfigAtRootWithStderr(root string, stderr io.Writer) (model.Config, error) {
	cfgPath := state.ConfigPath(root)
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		return model.Config{}, newStateIntegrityError(
			"config_parse_failure",
			fmt.Sprintf("failed to load .slipway.yaml: %v", err),
			"Run `slipway repair` to back up broken config and rewrite deterministic defaults.",
			"",
			map[string]any{"path": cfgPath},
		)
	}
	// Surface (rather than silently swallow) unrecognized top-level config keys so a
	// typo'd or stale key is visible. This must go to STDERR, never stdout:
	// loadConfigAtRoot is on every `--json` code path, and writing to stdout would
	// corrupt machine-readable output. Emitted once per load, only when keys exist.
	warnUnknownTopLevelConfigKeys(stderr, cfgPath, cfg)
	return cfg, nil
}

// warnUnknownTopLevelConfigKeys writes a single concise warning naming every
// unrecognized top-level key captured during config decode. It is a no-op when
// there are no unknown keys. The sink is always a stderr writer so the warning
// can never corrupt `--json` stdout.
func warnUnknownTopLevelConfigKeys(stderr io.Writer, cfgPath string, cfg model.Config) {
	if len(cfg.UnknownTopLevel) == 0 {
		return
	}
	keys := make([]string, 0, len(cfg.UnknownTopLevel))
	for key := range cfg.UnknownTopLevel {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(stderr, "warning: %s has unknown top-level config keys (ignored): %s\n", cfgPath, strings.Join(keys, ", "))
}

func wrapRequiredSkillsEvaluationError(operation, slug string, err error) error {
	if err == nil {
		return nil
	}

	errorCode := "required_skills_evaluation_failed"
	remediation := "Run `slipway repair` to restore generated skills, then fix malformed governance skill or evidence files."
	details := map[string]any{
		"operation": operation,
	}

	var registryErr *skill.GovernanceRegistryError
	if errors.As(err, &registryErr) {
		errorCode = "skill_registry_invalid"
		remediation = "Run `slipway repair` to restore generated skills or fix malformed governance skill frontmatter."
		if registryErr.Path != "" {
			details["path"] = registryErr.Path
		}
	}

	return newStateIntegrityError(
		errorCode,
		fmt.Sprintf("%s: %v", operation, err),
		remediation,
		slug,
		details,
	)
}

func governanceReadinessErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var registryErr *skill.GovernanceRegistryError
	if errors.As(err, &registryErr) {
		return "skill_registry_invalid"
	}
	var verificationErr *state.VerificationLoadError
	if errors.As(err, &verificationErr) {
		return "verification_load_failed"
	}
	var summaryErr *state.ExecutionSummaryLoadError
	if errors.As(err, &summaryErr) {
		return "execution_summary_load_failed"
	}
	return "governance_readiness_failed"
}

func wrapGovernanceReadinessError(operation, slug string, err error) error {
	if err == nil {
		return nil
	}

	if governanceReadinessErrorCode(err) == "skill_registry_invalid" {
		return wrapRequiredSkillsEvaluationError(operation, slug, err)
	}

	errorCode := governanceReadinessErrorCode(err)
	remediation := "Run `slipway repair` to restore canonical governance files, then fix malformed verification, bundle, or worktree authority data."
	if errorCode == "verification_load_failed" {
		remediation = "Run `slipway repair` to inspect authoritative verification files, then fix malformed verification records or restore unreadable verification files."
	}
	if errorCode == "execution_summary_load_failed" {
		remediation = "Run `slipway repair` to restore execution-summary authority, then fix malformed execution summary files."
	}
	details := map[string]any{
		"operation": operation,
	}
	var verificationErr *state.VerificationLoadError
	if errors.As(err, &verificationErr) && strings.TrimSpace(verificationErr.Path) != "" {
		details["path"] = verificationErr.Path
	}
	var summaryErr *state.ExecutionSummaryLoadError
	if errors.As(err, &summaryErr) && strings.TrimSpace(summaryErr.Path) != "" {
		details["path"] = summaryErr.Path
	}

	return newStateIntegrityError(
		errorCode,
		fmt.Sprintf("%s: %v", operation, err),
		remediation,
		slug,
		details,
	)
}

func readOnlyRequiredSkillInputs(change model.Change) []model.PlanSubStep {
	if change.CurrentState != model.StateS1Plan {
		return nil
	}
	if change.PlanSubStep == model.PlanSubStepNone {
		return nil
	}
	return []model.PlanSubStep{change.PlanSubStep}
}

func workflowStateLabel(currentState model.WorkflowState, intakeSubStep model.IntakeSubStep, planSubStep model.PlanSubStep) string {
	if currentState == model.StateS0Intake && intakeSubStep != "" {
		return fmt.Sprintf("%s/%s", currentState, intakeSubStep)
	}
	if currentState != model.StateS1Plan || planSubStep == model.PlanSubStepNone {
		return string(currentState)
	}
	return fmt.Sprintf("%s/%s", currentState, planSubStep)
}

func planningNote(currentState model.WorkflowState, planSubStep model.PlanSubStep) string {
	if currentState == model.StateS1Plan && planSubStep == model.PlanSubStepValidate {
		return "This is a recovery-only planning state entered after post-audit machine validation failed."
	}
	return ""
}

func resolveActiveChangeRef(root string, explicitSlug string) (changeRef, error) {
	// When --change is provided, load that specific change.
	if strings.TrimSpace(explicitSlug) != "" {
		return resolveExplicitChange(root, strings.TrimSpace(explicitSlug))
	}

	// Worktree-based resolution.
	worktreePath, err := currentWorktreeRoot()
	if err != nil {
		return changeRef{}, wrapResolutionError(err)
	}
	if worktreePath != "" {
		change, err := state.FindActiveChangeForWorktree(root, worktreePath)
		if err != nil {
			// Before surfacing "no active change here" or "bound to another
			// worktree", prefer this worktree's own archived change when it hosts
			// one, so an archived-change worktree reports its own terminal state
			// (archived_change_not_validatable) instead of an unrelated active
			// change bound to a different worktree (#283).
			if shouldTryArchivedWorktreeFallback(err) {
				if archived, ok, archErr := state.FindArchivedChangeForWorktree(root, worktreePath); archErr != nil {
					return changeRef{}, wrapArchivedWorktreeResolutionError(archErr)
				} else if ok {
					return resolveExplicitChange(root, archived.Slug)
				}
			}
			if recoveryErr := deleteRecoveryError(root, ""); recoveryErr != nil {
				return changeRef{}, recoveryErr
			}
			return changeRef{}, wrapResolutionError(err)
		}
		if strings.TrimSpace(change.WorktreePath) == "" {
			// A single unbound active change is only a fallback. In an archived
			// review worktree, local archived authority is more specific and must
			// fail closed before commands operate on the unrelated unbound change.
			if archived, ok, archErr := state.FindArchivedChangeForWorktree(root, worktreePath); archErr != nil {
				return changeRef{}, wrapArchivedWorktreeResolutionError(archErr)
			} else if ok {
				return resolveExplicitChange(root, archived.Slug)
			}
		}
		return changeRef{Slug: change.Slug}, nil
	}

	change, err := state.FindActiveChange(root)
	if err != nil {
		if recoveryErr := deleteRecoveryError(root, ""); recoveryErr != nil {
			return changeRef{}, recoveryErr
		}
		return changeRef{}, wrapResolutionError(err)
	}
	return changeRef{Slug: change.Slug}, nil
}

func shouldTryArchivedWorktreeFallback(err error) bool {
	if errors.Is(err, state.ErrNoActiveChange) {
		return true
	}
	var boundElsewhere *state.ChangeBoundElsewhereError
	return errors.As(err, &boundElsewhere)
}

func addChangeSelectorFlags(cmd *cobra.Command, target *string, usage string) {
	cmd.Flags().StringVar(target, "change", "", usage)
}

// resolveExplicitChange loads a change by slug and verifies it is active.
func resolveExplicitChange(root string, slug string) (changeRef, error) {
	slug = strings.TrimSpace(slug)
	if err := state.ValidateChangeSlug(slug); err != nil {
		return changeRef{}, newInvalidUsageError(
			"invalid_change_slug",
			fmt.Sprintf("invalid change slug %q: %v", slug, err),
			"Use a canonical change slug containing lowercase letters, digits, and single hyphen separators.",
			map[string]any{"slug": slug},
		)
	}
	change, err := state.LoadChange(root, slug)
	if err != nil {
		// An archived DONE change can leave its active bundle directory behind
		// after its change.yaml is moved to the archive. LoadChange then reports a
		// missing-authority error rather than os.ErrNotExist, so attempt the
		// archived fallback for both. This mirrors status's
		// shouldFallbackToArchivedStatus predicate (issue #196).
		if errors.Is(err, os.ErrNotExist) || state.IsMissingBundleAuthority(err) {
			if archived, archiveErr := state.LoadArchivedChange(root, slug); archiveErr == nil {
				archivePath := filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug, "change.yaml"))
				if path, pathErr := state.ArchivedChangeFilePathForRead(root, slug); pathErr == nil {
					archivePath = state.DisplayPath(root, path)
				}
				return changeRef{}, newPreconditionError(
					"archived_change_not_validatable",
					fmt.Sprintf("change %q is archived with status=%s; active governance commands only validate active changes", slug, archived.Status),
					fmt.Sprintf("Inspect archived evidence at %s, or choose an active change with `slipway status`.", archivePath),
					slug,
					map[string]any{
						"archived":     true,
						"archive_path": archivePath,
						"status":       string(archived.Status),
					},
				)
			}
			// No archived record was found. Only os.ErrNotExist (no bundle at all)
			// softens to no_active_change. A missing-authority error without an
			// archive is genuine active-bundle corruption and must fall through to
			// fail closed on change_state_load_failed.
			if errors.Is(err, os.ErrNotExist) {
				if recoveryErr := deleteRecoveryError(root, slug); recoveryErr != nil {
					return changeRef{}, recoveryErr
				}
				return changeRef{}, newPreconditionError(
					"no_active_change",
					fmt.Sprintf("no change found for slug %q", slug),
					"Check the slug with `slipway status`.",
					slug,
					nil,
				)
			}
		}
		if recoveryErr := deleteRecoveryError(root, slug); recoveryErr != nil {
			return changeRef{}, recoveryErr
		}
		return changeRef{}, newChangeStateLoadFailedError(slug, err)
	}
	if change.Status != model.ChangeStatusActive {
		return changeRef{}, newPreconditionError(
			"not_active",
			fmt.Sprintf("change %q is not active; current status=%s", slug, change.Status),
			"Use `slipway status` to choose an active change, or inspect `artifacts/changes/<slug>/change.yaml` for state.",
			slug,
			map[string]any{
				"status": string(change.Status),
			},
		)
	}
	if err := state.ValidateChangeSlug(change.Slug); err != nil {
		return changeRef{}, newStateIntegrityError(
			"invalid_change_slug",
			fmt.Sprintf("change %q has invalid embedded slug %q: %v", slug, change.Slug, err),
			"Inspect the change authority file before running active governance commands.",
			slug,
			map[string]any{"embedded_slug": change.Slug},
		)
	}
	return changeRef{Slug: change.Slug}, nil
}

func loadChangeBySlug(root, slug string) (model.Change, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if recoveryErr := deleteRecoveryError(root, slug); recoveryErr != nil {
				return model.Change{}, recoveryErr
			}
			return model.Change{}, newPreconditionError(
				"change_not_found",
				fmt.Sprintf("no change found for slug %q", slug),
				"Check the slug with `slipway status`.",
				slug,
				nil,
			)
		}
		if recoveryErr := deleteRecoveryError(root, slug); recoveryErr != nil {
			return model.Change{}, recoveryErr
		}
		return model.Change{}, newChangeStateLoadFailedError(slug, err)
	}
	return change, nil
}

func deleteRecoveryError(root, slug string) *CLIError {
	if orphanErr := orphanedChangeBundleError(root, slug); orphanErr != nil {
		return orphanErr
	}
	return staleRuntimeBindingError(root, slug)
}

// unmanagedOrphan pairs an orphan-bundle slug with the live, externally-managed
// worktree match that makes its recovery non-destructive (issue #285).
type unmanagedOrphan struct {
	Slug  string
	Match state.SlugWorktreeMatch
}

// unknownOrphan pairs an orphan-bundle slug with the git cross-check error that
// prevented classifying whether a live worktree holds its work. Recovery must
// fail closed: work whose ownership could not be verified is never routed to the
// destructive discard path (issue #285).
type unknownOrphan struct {
	Slug string
	Err  error
}

// archivedResidueOrphan pairs an orphan active-bundle slug with the archived
// terminal record for the same slug. Recovery deletes only active-state residue;
// the archived record and source commits are retained.
type archivedResidueOrphan struct {
	Slug        string
	ArchivePath string
}

// orphanClassification splits orphan-bundle slugs into the unmanaged-worktree
// case (preserve-first, never discard), the ownership-unknown case (the git
// cross-check failed, so fail closed to preserve-first), and the plain
// discardable residue.
type orphanClassification struct {
	Unmanaged       []unmanagedOrphan
	Unknown         []unknownOrphan
	ArchivedResidue []archivedResidueOrphan
	Plain           []string
}

// classifyOrphanBundles cross-checks each orphan slug against git
// worktrees/branches so the destructive-discard decision is made per orphan, not
// just for the first. A slug naming a live worktree Slipway does not manage maps
// to externally-managed, possibly-unmerged work and is classified Unmanaged; a
// slug whose git cross-check FAILS is classified Unknown and fails closed to
// preserve-first (never the destructive discard path); a slug with no live
// worktree (or one Slipway itself provisioned, which the discard path already
// owns safely) is Plain. It is the single classifier shared by the CLI error and
// status recovery surfaces so they never diverge (#285).
type slugWorktreeMatcher func(root, slug string) (state.SlugWorktreeMatch, bool, error)

func classifyOrphanBundles(root string, orphans []string) orphanClassification {
	return classifyOrphanBundlesWith(root, orphans, state.FindSlugWorktreeMatch)
}

func classifyOrphanBundlesWith(root string, orphans []string, match slugWorktreeMatcher) orphanClassification {
	var class orphanClassification
	for _, slug := range orphans {
		m, ok, err := match(root, slug)
		if err != nil {
			// Fail closed: a git worktree/branch cross-check failure means we cannot
			// prove the slug's work is safe to discard, so it never enters the
			// destructive discard path (#285).
			class.Unknown = append(class.Unknown, unknownOrphan{Slug: slug, Err: err})
			continue
		}
		if ok && !m.SlipwayManaged {
			class.Unmanaged = append(class.Unmanaged, unmanagedOrphan{Slug: slug, Match: m})
			continue
		}
		if archived, ok := archivedResidueOrphanForSlug(root, slug); ok {
			class.ArchivedResidue = append(class.ArchivedResidue, archived)
			continue
		}
		class.Plain = append(class.Plain, slug)
	}
	return class
}

func unmanagedOrphanSlugs(orphans []unmanagedOrphan) []string {
	slugs := make([]string, 0, len(orphans))
	for _, o := range orphans {
		slugs = append(slugs, o.Slug)
	}
	return slugs
}

func unknownOrphanSlugs(orphans []unknownOrphan) []string {
	slugs := make([]string, 0, len(orphans))
	for _, o := range orphans {
		slugs = append(slugs, o.Slug)
	}
	return slugs
}

func archivedResidueOrphanSlugs(orphans []archivedResidueOrphan) []string {
	slugs := make([]string, 0, len(orphans))
	for _, o := range orphans {
		slugs = append(slugs, o.Slug)
	}
	return slugs
}

func archivedActiveResidueReason(orphan archivedResidueOrphan) model.ReasonCode {
	return model.ReasonCode{
		Code:     "orphaned_change_bundle",
		Severity: model.ReasonSeverityError,
		// Build the message from the shared model sentinel so the prose the model
		// matcher (isArchivedActiveResidueReason) keys on cannot drift from the
		// prose produced here across the package boundary (F2).
		Message: fmt.Sprintf("%s%q remains; archived record and source commits are not deletion targets", model.ArchivedActiveResidueMessagePrefix, orphan.Slug),
		Detail:  orphan.Slug,
	}
}

func archivedResidueOrphanForSlug(root, slug string) (archivedResidueOrphan, bool) {
	if _, err := state.LoadArchivedChange(root, slug); err != nil {
		return archivedResidueOrphan{}, false
	}
	archivePath := filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug, "change.yaml"))
	if path, err := state.ArchivedChangeFilePathForRead(root, slug); err == nil {
		archivePath = state.DisplayPath(root, path)
	}
	return archivedResidueOrphan{Slug: slug, ArchivePath: archivePath}, true
}

func orphanedChangeBundleError(root, slug string) *CLIError {
	orphans, err := orphanedChangeBundleSlugs(root, slug)
	if err != nil || len(orphans) == 0 {
		return nil
	}
	// orphans carries a single slug when called for a specific change, but several
	// when called with an empty slug (the no-target delete-recovery path). Classify
	// EVERY orphan so a mixed no-target recovery preserves a blocker for each one,
	// instead of returning only the first unmanaged match and dropping the rest.
	class := classifyOrphanBundles(root, orphans)

	reasons := make([]model.ReasonCode, 0, len(orphans))
	for _, u := range class.Unmanaged {
		reasons = append(reasons, model.NewReasonCode("orphaned_bundle_unmanaged_worktree", u.Slug))
	}
	for _, u := range class.Unknown {
		reasons = append(reasons, model.NewReasonCode("orphaned_bundle_ownership_unknown", u.Slug))
	}
	for _, archived := range class.ArchivedResidue {
		reasons = append(reasons, archivedActiveResidueReason(archived))
	}
	reasons = append(reasons, orphanedChangeBundleReasons(class.Plain)...)

	// A non-destructive case (a live worktree Slipway does not manage, or one whose
	// ownership could not be verified) is the dangerous one: it leads the error so
	// its preserve-first remediation reaches the operator before any discardable
	// residue is folded in. Unmanaged leads when present (it carries a concrete
	// path/branch); otherwise the ownership-unknown case leads.
	if len(class.Unmanaged) > 0 || len(class.Unknown) > 0 {
		var (
			errorCode   string
			primarySlug string
			message     string
			remediation string
		)
		details := map[string]any{}
		if len(class.Unmanaged) > 0 {
			primary := class.Unmanaged[0]
			errorCode = "orphaned_bundle_unmanaged_worktree"
			primarySlug = primary.Slug
			message = fmt.Sprintf("governed bundle %q lost its change.yaml authority, but a live git worktree Slipway does not manage still holds work for this slug", primary.Slug)
			remediation = unmanagedWorktreeOrphanRemediation(primary.Slug, primary.Match)
			details["unmanaged_worktree_path"] = primary.Match.WorktreePath
			details["unmanaged_worktree_branch"] = primary.Match.Branch
			details["unmanaged_worktree_orphans"] = unmanagedOrphanSlugs(class.Unmanaged)
			if len(class.Unknown) > 0 {
				unknown := unknownOrphanSlugs(class.Unknown)
				details["ownership_unknown_orphans"] = unknown
				remediation += fmt.Sprintf(" Ownership could not be verified for: %s (git worktree cross-check failed); inspect those for live worktrees and preserve any unmerged work before discarding anything.", strings.Join(unknown, ", "))
			}
		} else {
			primary := class.Unknown[0]
			errorCode = "orphaned_bundle_ownership_unknown"
			primarySlug = primary.Slug
			message = fmt.Sprintf("governed bundle %q lost its change.yaml authority and its git worktree ownership could not be verified", primary.Slug)
			remediation = ownershipUnknownOrphanRemediation(primary.Slug, primary.Err)
			details["ownership_unknown_orphans"] = unknownOrphanSlugs(class.Unknown)
		}
		if len(class.Plain) > 0 {
			details["orphaned_change_bundles"] = class.Plain
			remediation += fmt.Sprintf(" Separately, the abandoned residue with no live worktree can be discarded with `slipway delete --change %s` for: %s.", class.Plain[0], strings.Join(class.Plain, ", "))
		}
		if len(class.ArchivedResidue) > 0 {
			archived := class.ArchivedResidue[0]
			details["orphaned_active_residue_archived_changes"] = archivedResidueOrphanSlugs(class.ArchivedResidue)
			details["archive_path"] = archived.ArchivePath
			remediation += " Separately, " + archivedActiveResidueRemediation(archived)
		}
		return newCLIErrorWithReasons(
			categoryPrecondition,
			errorCode,
			message,
			remediation,
			primarySlug,
			reasons,
			details,
		)
	}

	if len(class.ArchivedResidue) > 0 {
		primary := class.ArchivedResidue[0]
		message := fmt.Sprintf("active-state residue for archived change %q is missing its change.yaml authority", primary.Slug)
		remediation := archivedActiveResidueRemediation(primary)
		if len(class.ArchivedResidue) > 1 {
			message = "active-state residue remains for archived changes: " + strings.Join(archivedResidueOrphanSlugs(class.ArchivedResidue), ", ")
			remediation = fmt.Sprintf("Discard each stale active-state residue with `slipway delete --change <slug>`; first suggested command: `slipway delete --change %s`. Archived records and source commits are not deletion targets.", primary.Slug)
		}
		details := map[string]any{
			"orphaned_active_residue_archived_changes": archivedResidueOrphanSlugs(class.ArchivedResidue),
			"archive_path": primary.ArchivePath,
		}
		if len(class.Plain) > 0 {
			details["orphaned_change_bundles"] = class.Plain
			remediation += fmt.Sprintf(" Separately, abandoned residue with no archived record can be discarded with `slipway delete --change %s` for: %s.", class.Plain[0], strings.Join(class.Plain, ", "))
		}
		return newCLIErrorWithReasons(
			categoryPrecondition,
			"orphaned_active_residue_archived_change",
			message,
			remediation,
			primary.Slug,
			reasons,
			details,
		)
	}

	primarySlug := class.Plain[0]
	message := fmt.Sprintf("governed bundle %q is missing its change.yaml authority", primarySlug)
	remediation := fmt.Sprintf("Discard it with `slipway delete --change %s` (add --worktree to also remove its worktree).", primarySlug)
	if len(class.Plain) > 1 {
		message = "governed bundles are missing their change.yaml authority: " + strings.Join(class.Plain, ", ")
		remediation = fmt.Sprintf("Discard each abandoned change with `slipway delete --change <slug>`; first suggested command: `slipway delete --change %s`.", primarySlug)
	}
	return newCLIErrorWithReasons(
		categoryPrecondition,
		"orphaned_change_bundle",
		message,
		remediation,
		primarySlug,
		reasons,
		map[string]any{
			"orphaned_change_bundles": class.Plain,
		},
	)
}

// unmanagedWorktreeOrphanRemediation renders non-destructive recovery prose for
// an orphan bundle whose slug names a live worktree Slipway does not manage. It
// leads with inspect/preserve and never recommends removing that worktree.
func unmanagedWorktreeOrphanRemediation(slug string, match state.SlugWorktreeMatch) string {
	location := match.WorktreePath
	if strings.TrimSpace(match.Branch) != "" {
		location = fmt.Sprintf("%s (branch %s)", match.WorktreePath, match.Branch)
	}
	return fmt.Sprintf(
		"A live git worktree Slipway does not manage holds work for %q at %s. Inspect and preserve that worktree and its branch first — Slipway never removes a worktree it did not provision. Once its work is merged or saved, discard only the stale bundle residue with `slipway delete --change %s` (never pass --worktree).",
		slug, location, slug,
	)
}

func archivedActiveResidueRemediation(orphan archivedResidueOrphan) string {
	return fmt.Sprintf(
		"Active-state residue for archived change %q remains under artifacts/changes/%s. Remove only that stale active-state residue with `slipway delete --change %s`. The archived record at %s and source commits are not deletion targets.",
		orphan.Slug, orphan.Slug, orphan.Slug, orphan.ArchivePath,
	)
}

// ownershipUnknownOrphanRemediation renders preserve-first prose for an orphan
// bundle whose git worktree ownership could not be verified (the cross-check
// failed). It must never recommend `--worktree` and must lead with inspect/preserve.
func ownershipUnknownOrphanRemediation(slug string, cause error) string {
	detail := ""
	if cause != nil {
		detail = fmt.Sprintf(" (%v)", cause)
	}
	return fmt.Sprintf(
		"A live git worktree may hold work for %q, but Slipway could not verify its ownership because the git worktree/branch cross-check failed%s. Do not discard this bundle yet: inspect for a live worktree or branch named after %q and preserve any unmerged work first. Once you have confirmed no unmerged work remains, discard only the stale bundle residue with `slipway delete --change %s` (never pass --worktree).",
		slug, detail, slug, slug,
	)
}

func staleRuntimeBindingError(root, slug string) *CLIError {
	stale, err := staleRuntimeBindingSlugs(root, slug)
	if err != nil || len(stale) == 0 {
		return nil
	}
	reasons := staleRuntimeBindingReasons(stale)
	primarySlug := stale[0]
	message := fmt.Sprintf("runtime binding for %q remains after its governed bundle was removed", primarySlug)
	remediation := fmt.Sprintf("Discard it with `slipway delete --change %s` (add --worktree to also remove its worktree).", primarySlug)
	if len(stale) > 1 {
		message = "runtime bindings remain after their governed bundles were removed: " + strings.Join(stale, ", ")
		remediation = fmt.Sprintf("Discard each abandoned change with `slipway delete --change <slug>`; first suggested command: `slipway delete --change %s`.", primarySlug)
	}
	return newCLIErrorWithReasons(
		categoryPrecondition,
		"stale_runtime_binding",
		message,
		remediation,
		primarySlug,
		reasons,
		map[string]any{
			"stale_runtime_bindings": stale,
		},
	)
}

func orphanedChangeBundleSlugs(root, slug string) ([]string, error) {
	orphans, err := state.OrphanBundleSlugs(root)
	if err != nil {
		return nil, err
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return orphans, nil
	}
	for _, orphan := range orphans {
		if orphan == slug {
			return []string{slug}, nil
		}
	}
	return nil, nil
}

func staleRuntimeBindingSlugs(root, slug string) ([]string, error) {
	stale, err := state.StaleRuntimeBindingSlugs(root)
	if err != nil {
		return nil, err
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return stale, nil
	}
	for _, candidate := range stale {
		if candidate == slug {
			return []string{slug}, nil
		}
	}
	return nil, nil
}

func orphanedChangeBundleReasons(slugs []string) []model.ReasonCode {
	reasons := make([]model.ReasonCode, 0, len(slugs))
	for _, slug := range slugs {
		reasons = append(reasons, model.NewReasonCode("orphaned_change_bundle", slug))
	}
	return reasons
}

func staleRuntimeBindingReasons(slugs []string) []model.ReasonCode {
	reasons := make([]model.ReasonCode, 0, len(slugs))
	for _, slug := range slugs {
		reasons = append(reasons, model.NewReasonCode("stale_runtime_binding", slug))
	}
	return reasons
}

// newChangeStateLoadFailedError builds the standard error returned when a
// change's change.yaml cannot be loaded. All change-state loaders share it so
// the error code, remediation, and metadata stay identical.
func newChangeStateLoadFailedError(slug string, err error) *CLIError {
	return newChangeStateLoadFailedErrorForPath(
		slug,
		filepath.Join("artifacts", "changes", slug, "change.yaml"),
		err,
	)
}

func newChangeStateLoadFailedErrorForPath(slug, path string, err error) *CLIError {
	return newStateIntegrityError(
		"change_state_load_failed",
		fmt.Sprintf("failed to load change state for %q: %v", slug, err),
		"Run `slipway repair` to inspect or repair change state files.",
		slug,
		map[string]any{
			"path": path,
		},
	)
}

func wrapArchivedWorktreeResolutionError(err error) error {
	var archivedLoad *state.ArchivedChangeLoadError
	if errors.As(err, &archivedLoad) {
		return newChangeStateLoadFailedErrorForPath(
			archivedLoad.Slug,
			filepath.Join("artifacts", "changes", "archived", archivedLoad.Slug, "change.yaml"),
			err,
		)
	}
	return wrapResolutionError(err)
}

func wrapResolutionError(err error) error {
	var boundElsewhere *state.ChangeBoundElsewhereError
	if errors.As(err, &boundElsewhere) {
		boundChanges := make([]map[string]string, 0, len(boundElsewhere.BoundChanges))
		parts := make([]string, 0, len(boundElsewhere.BoundChanges))
		for _, change := range boundElsewhere.BoundChanges {
			boundChanges = append(boundChanges, map[string]string{
				"slug":          change.Slug,
				"worktree_path": change.WorktreePath,
			})
			parts = append(parts, fmt.Sprintf("%s at %s", change.Slug, change.WorktreePath))
		}
		remediation := "Use `slipway next --change <slug>` / `slipway run --change <slug>`, or cd into the bound worktree. To discard an abandoned change instead, run `slipway delete --change <slug>` (add --worktree to also remove its worktree)."
		if len(boundElsewhere.BoundChanges) == 1 {
			change := boundElsewhere.BoundChanges[0]
			remediation = fmt.Sprintf("Use `slipway next --change %s` / `slipway run --change %s`, or cd into %s. To discard it instead, run `slipway delete --change %s` (add --worktree to also remove its worktree).", change.Slug, change.Slug, change.WorktreePath, change.Slug)
		}
		return newPreconditionError(
			"change_bound_to_other_worktree",
			"active change is bound to another worktree: "+strings.Join(parts, ", "),
			remediation,
			"",
			map[string]any{
				"bound_changes": boundChanges,
			},
		)
	}
	if errors.Is(err, state.ErrNoActiveChange) {
		return newPreconditionError(
			"no_active_change",
			"no active change; start one with `slipway new`",
			"Use `slipway new` to create a governed change.",
			"",
			nil,
		)
	}
	if errors.Is(err, state.ErrMultipleActiveChanges) {
		return newPreconditionError(
			"active_context_ambiguous",
			"active change context is ambiguous; use `--change <slug>` or run `slipway status`",
			"Specify change explicitly with `--change <slug>`, or run `slipway status` for diagnostics.",
			"",
			nil,
		)
	}
	return err
}

// loadActiveChange loads a change by slug and verifies it is active.
func loadActiveChange(root, slug, inactiveMessage, remediation string) (model.Change, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return model.Change{}, newChangeStateLoadFailedError(slug, err)
	}
	if change.Status != model.ChangeStatusActive {
		return model.Change{}, newPreconditionError(
			"not_active",
			fmt.Sprintf(inactiveMessage, change.Status),
			remediation,
			slug,
			map[string]any{
				"status": string(change.Status),
			},
		)
	}
	return change, nil
}

// currentWorktreeRoot returns the git worktree root for the current working directory.
func currentWorktreeRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		// Only fall back to CWD when git reports "not a git repository".
		// Other errors (git binary missing, permission denied) should propagate.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 &&
			strings.Contains(string(exitErr.Stderr), "not a git repository") {
			return os.Getwd()
		}
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func projectNextReadyActions(currentState model.WorkflowState) []string {
	return projectNextReadyActionsWithPrimary(currentState, "")
}

func projectNextReadyActionsWithPrimary(currentState model.WorkflowState, primary string) []string {
	actions := []string{}
	if currentState == model.StateDone {
		return actions
	}
	if primary = strings.TrimSpace(primary); primary != "" {
		actions = append(actions, primary)
	} else {
		switch currentState {
		case model.StateS2Implement:
			actions = append(actions, "run")
		default:
			actions = append(actions, "next")
		}
	}
	if currentState == model.StateS2Implement {
		actions = append(actions, "abort")
	}
	actions = append(actions, "cancel")
	return actions
}

func projectFreshnessForExecMode(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	blockers []model.ReasonCode,
) string {
	if !state.ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return string(ctxpack.EvidenceFreshnessUnknown)
	}
	if hasFreshnessBlocker(blockers) {
		return string(ctxpack.EvidenceFreshnessStale)
	}
	diagnostics := state.ExecutionSummaryFreshnessDiagnostics(root, change, summary)
	return string(state.ProjectExecutionFreshnessForState(change.CurrentState, diagnostics))
}

func hasFreshnessBlocker(blockers []model.ReasonCode) bool {
	for _, blocker := range blockers {
		if blocker.Code == state.StaleExecutionEvidenceBlockerToken {
			return true
		}
	}
	return false
}

// executionContext holds the resolved execution summary state used by status,
// validate, review, next, and stats commands.
type executionContext struct {
	Summary          *model.ExecutionSummary
	LatestRunVersion int
	Ready            bool
	// SummaryBlockers are the summary-level OpenBlockers from execution-summary.yaml.
	// These include wave-level blockers, parse issues, and session isolation warnings
	// that are not captured at the task level.
	SummaryBlockers []model.ReasonCode
}

type waveExecutionContext struct {
	Plan model.WavePlan
	Runs []model.WaveRun
}

func loadResumableWaveExecution(
	root string,
	change model.Change,
	execCtx executionContext,
	operation string,
) (int, error) {
	if change.CurrentState != model.StateS2Implement || !execCtx.Ready {
		return 0, nil
	}

	waveCtx, err := loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, operation)
	if err != nil {
		if resumableWavePlanHasStructuralDrift(root, change) {
			return 0, nil
		}
		return 0, err
	}
	if waveCtx == nil {
		return 0, err
	}
	return state.ResumeWaveIndex(waveCtx.Plan, waveCtx.Runs), nil
}

// loadExecutionContext loads the execution summary for a change and extracts
// the unified readiness and blocker surface. This is the single call site for
// execution-summary authority consumption.
func loadExecutionContext(root string, change model.Change) (executionContext, error) {
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return executionContext{}, newStateIntegrityError(
			"execution_summary_load_failed",
			fmt.Sprintf("failed to load execution summary for %q: %v", change.Slug, err),
			"Run `slipway repair` to inspect or repair execution summary files.",
			change.Slug,
			map[string]any{
				"path": state.ExecutionSummaryPathForRead(root, change.Slug),
			},
		)
	}
	ctx := executionContext{Summary: summary}
	if summary != nil {
		ctx.LatestRunVersion = summary.RunSummaryVersion
		ctx.Ready = state.ExecutionSummaryReady(summary)
		if len(summary.OpenBlockers) > 0 {
			ctx.SummaryBlockers = append(ctx.SummaryBlockers, summary.OpenBlockers...)
		}
	}
	return ctx, nil
}

// wavePlanCacheUnreadableRemediation guides recovery when the engine-owned
// wave-plan.yaml cache is corrupt or carries unsupported/view-only fields. It
// must point at regeneration via `slipway repair`, never at hand-editing
// tasks.md or the cache itself.
const wavePlanCacheUnreadableRemediation = "wave-plan.yaml is an engine-owned cache and must not be hand-edited. Run `slipway repair` to rebuild it from tasks.md, then `slipway run` to refresh affected execution evidence."

// newWavePlanCacheUnreadableError builds the state-integrity error every command
// surface must emit when loadCurrentWavePlanForCommand reports a corrupt
// engine-owned wave-plan.yaml cache (errors.Is(err, state.ErrWavePlanCacheUnreadable)).
// It fails closed to the canonical wave_plan_unreadable recovery story so a
// copied or hand-edited cache never receives "Fix tasks.md" guidance it cannot
// act on. surface is a short human label for the operation (for example "task
// evidence").
func newWavePlanCacheUnreadableError(root string, change model.Change, surface string, err error) error {
	return newStateIntegrityError(
		"wave_plan_unreadable",
		fmt.Sprintf("%s could not read the engine-owned wave-plan.yaml cache for %q: %v", surface, change.Slug, err),
		wavePlanCacheUnreadableRemediation,
		change.Slug,
		map[string]any{
			"path": state.WavePlanPathForRead(root, change.Slug),
		},
	)
}

func loadAuthoritativeWaveExecution(
	root string,
	change model.Change,
	runVersion int,
	operation string,
) (*waveExecutionContext, error) {
	if runVersion < 1 {
		return nil, nil
	}

	plan, err := loadCurrentWavePlanForCommand(root, change)
	if err != nil {
		// The persisted, engine-owned wave-plan.yaml cache is corrupt or carries
		// unsupported/view-only fields. Point at the cache and the public
		// regenerate path, NOT at editing tasks.md.
		if errors.Is(err, state.ErrWavePlanCacheUnreadable) {
			return nil, newWavePlanCacheUnreadableError(root, change, operation, err)
		}
		errorCode := "wave_plan_load_failed"
		message := fmt.Sprintf("%s failed to derive the current wave plan for %q: %v", operation, change.Slug, err)
		// Default remediation covers the derive-from-tasks.md failure only.
		remediation := "Update tasks.md so it can be converted into the current wave plan before continuing."
		if errors.Is(err, fs.ErrNotExist) {
			errorCode = "wave_plan_missing"
			message = fmt.Sprintf("%s requires tasks.md for %q, but it is missing", operation, change.Slug)
		}
		return nil, newStateIntegrityError(
			errorCode,
			message,
			remediation,
			change.Slug,
			map[string]any{
				"path": state.WavePlanPathForRead(root, change.Slug),
			},
		)
	}

	runs, err := state.LoadOptionalWaveRuns(root, change.Slug, runVersion)
	if err != nil {
		return nil, newStateIntegrityError(
			"wave_runs_load_failed",
			fmt.Sprintf("%s failed to load wave run evidence for %q: %v", operation, change.Slug, err),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug),
			},
		)
	}
	if len(plan.Waves) > 0 && len(runs) == 0 {
		return nil, newStateIntegrityError(
			"wave_runs_missing",
			fmt.Sprintf("%s requires wave run evidence for %q, but none was found for run_summary_version=%d", operation, change.Slug, runVersion),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug),
			},
		)
	}
	if len(runs) > 0 && len(runs) < len(plan.Waves) {
		return nil, newStateIntegrityError(
			"wave_runs_incomplete",
			fmt.Sprintf("%s found incomplete wave run evidence for %q: %d of %d waves are present for run_summary_version=%d", operation, change.Slug, len(runs), len(plan.Waves), runVersion),
			"Run `slipway repair` to reconstruct the missing wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug),
			},
		)
	}
	if len(runs) > len(plan.Waves) {
		return nil, newStateIntegrityError(
			"wave_runs_invalid_count",
			fmt.Sprintf("%s found more wave runs than planned waves for %q (%d > %d)", operation, change.Slug, len(runs), len(plan.Waves)),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug),
			},
		)
	}
	for _, run := range runs {
		if run.RunSummaryVersion == runVersion {
			continue
		}
		return nil, newStateIntegrityError(
			"wave_run_version_mismatch",
			fmt.Sprintf("%s found wave evidence version drift for %q: wave %d points at run_summary_version=%d, expected run_summary_version=%d", operation, change.Slug, run.WaveIndex, run.RunSummaryVersion, runVersion),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path": state.WaveEvidenceDir(root, change.Slug),
			},
		)
	}
	if linkageIssues := state.WaveTaskLinkageIssues(plan, runs); len(linkageIssues) > 0 {
		return nil, newStateIntegrityError(
			"wave_task_linkage_mismatch",
			fmt.Sprintf("%s found wave/task linkage mismatch for %q: %s", operation, change.Slug, strings.Join(linkageIssues, "; ")),
			"Run `slipway repair` to reconstruct wave execution evidence before continuing.",
			change.Slug,
			map[string]any{
				"path":   state.WaveEvidenceDir(root, change.Slug),
				"issues": linkageIssues,
			},
		)
	}

	return &waveExecutionContext{
		Plan: plan,
		Runs: runs,
	}, nil
}

func loadCurrentWavePlanForCommand(root string, change model.Change) (model.WavePlan, error) {
	if change.CurrentState == model.StateS2Implement {
		plan, _, err := state.MaterializeWavePlanTransactionOpAt(root, change, time.Now().UTC())
		return plan, err
	}
	return state.LoadWavePlanForChange(root, change)
}

// encodeJSONResponse encodes v as indented JSON to the command's stdout.
func encodeJSONResponse(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// filterAlivePIDs returns only PIDs that are still running.
func filterAlivePIDs(pids []int) []int {
	alive := []int{}
	for _, pid := range pids {
		if isPIDAlive(pid) {
			alive = append(alive, pid)
		}
	}
	return alive
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

// rejectIfConflictingChange enforces one active governed change per workspace
// authority before creating a new change.
func rejectIfConflictingChange(root string, nextChange model.Change) error {
	allChanges, err := state.ListChangesForCreateGuard(root)
	if err != nil {
		return err
	}

	var activeChanges []model.Change
	for _, ch := range allChanges {
		if ch.Status == model.ChangeStatusActive {
			activeChanges = append(activeChanges, ch)
		}
	}
	if len(activeChanges) == 0 {
		return nil
	}

	invocationWorkspace, err := createGuardInvocationWorkspaceRoot(root)
	if err != nil {
		return err
	}
	targetWorkspace, err := newChangeTargetWorkspaceRoot(root, nextChange)
	if err != nil {
		return err
	}

	for _, ch := range activeChanges {
		existingWorkspace, err := existingChangeWorkspaceRoot(root, ch)
		if err != nil {
			return err
		}
		if strings.TrimSpace(ch.WorktreePath) != "" && pathsEqualForCreateGuard(existingWorkspace, invocationWorkspace) {
			return boundWorktreeCreateConflict(root, ch)
		}
		if pathsEqualForCreateGuard(existingWorkspace, targetWorkspace) {
			if strings.TrimSpace(ch.WorktreePath) == "" {
				return unboundWorkspaceCreateConflict(ch)
			}
			return targetWorkspaceCreateConflict(root, ch, targetWorkspace)
		}
	}

	return nil
}

func createGuardInvocationWorkspaceRoot(root string) (string, error) {
	normalizedRoot := normalizePathForCompare(root)
	currentWorktree, err := currentWorktreeRoot()
	if err != nil {
		return "", fmt.Errorf("cannot determine worktree for conflict check: %w", err)
	}
	currentWorktree = normalizePathForCompare(currentWorktree)

	projectWorktree, err := state.ResolveGitWorkspaceRoot(normalizedRoot)
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return normalizedRoot, nil
		}
		return "", fmt.Errorf("resolve project worktree for create guard: %w", err)
	}
	projectWorktree = normalizePathForCompare(projectWorktree)

	// Preserve nested Slipway scopes when the command runs from a sibling git worktree.
	scopeRel, err := filepath.Rel(projectWorktree, normalizedRoot)
	if err != nil {
		return currentWorktree, nil
	}
	scopeRel = filepath.Clean(scopeRel)
	if scopeRel == "." || scopeRel == "" {
		return currentWorktree, nil
	}
	if scopeRel == ".." || strings.HasPrefix(scopeRel, ".."+string(filepath.Separator)) {
		return currentWorktree, nil
	}
	return normalizePathForCompare(filepath.Join(currentWorktree, scopeRel)), nil
}

func newChangeTargetWorkspaceRoot(root string, change model.Change) (string, error) {
	target := root
	// Every governed change is worktree-provisioned by default (not just discovery
	// changes), so predict the dedicated `.worktrees/<slug>` target whenever
	// provisioning is enabled and the repo can support it. This keeps the
	// single-active-change guard correct: worktree-isolated changes do not
	// conflict with one another.
	if autoProvisionWorktreeEnabled(root) {
		repoRoot, err := state.ResolveGitWorkspaceRoot(root)
		if err != nil {
			if !strings.Contains(err.Error(), "not a git repository") {
				return "", fmt.Errorf("resolve git worktree for create guard: %w", err)
			}
		} else if gitWorkspaceHasHead(repoRoot) {
			target = state.DefaultWorktreePath(repoRoot, change.Slug)
		}
	}
	return normalizePathForCompare(target), nil
}

// autoProvisionWorktreeEnabled reports whether `slipway new` will bind a
// dedicated worktree for a governed change. It mirrors the gate used by
// state.EnsureDefaultWorktreeForChange so the create guard predicts the same
// target workspace the change will actually occupy. A missing config defaults to
// enabled; an unreadable/invalid config also falls back to enabled so the guard
// never blocks creation on a config read error (binding surfaces the real error).
func autoProvisionWorktreeEnabled(root string) bool {
	cfg, err := model.LoadConfig(state.ConfigPath(root))
	if err != nil {
		return true
	}
	return cfg.Governance.AutoProvisionWorktreeEnabled()
}

func existingChangeWorkspaceRoot(root string, ch model.Change) (string, error) {
	workspaceRoot, err := state.WorkspaceRootForChange(root, ch)
	if err != nil {
		return "", fmt.Errorf("resolve active change workspace for create guard: %w", err)
	}
	return normalizePathForCompare(workspaceRoot), nil
}

func gitWorkspaceHasHead(repoRoot string) bool {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", "--quiet", "HEAD") // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	return cmd.Run() == nil
}

func normalizePathForCompare(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if normalized, err := state.NormalizePath(path); err == nil {
		return normalized
	}
	return filepath.Clean(path)
}

func pathsEqualForCreateGuard(left, right string) bool {
	return normalizePathForCompare(left) == normalizePathForCompare(right)
}

func unboundWorkspaceCreateConflict(ch model.Change) error {
	return newPreconditionError(
		"active_change_exists",
		fmt.Sprintf("active governed change %s is already active in this workspace; finish or cancel it before creating another change here", ch.Slug),
		"Run `slipway done` or `slipway cancel` for the existing change before creating a new change in this workspace.",
		ch.Slug,
		nil,
	)
}

func boundWorktreeCreateConflict(root string, ch model.Change) error {
	worktreeHint := strings.TrimSpace(state.DisplayPath(root, ch.WorktreePath))
	if worktreeHint == "" {
		worktreeHint = ch.WorktreePath
	}
	remediation := "Run `slipway done` or `slipway cancel` before creating a new change in this worktree."
	if worktreeHint != "" {
		remediation = fmt.Sprintf(
			"Continue, finish, or cancel the active change in %s before creating a new change from that worktree.",
			worktreeHint,
		)
	}
	return newPreconditionError(
		"active_change_exists",
		fmt.Sprintf("active governed change %s is bound to this worktree; finish or cancel it before creating a new one", ch.Slug),
		remediation,
		ch.Slug,
		nil,
	)
}

func targetWorkspaceCreateConflict(root string, ch model.Change, targetWorkspace string) error {
	targetHint := strings.TrimSpace(state.DisplayPath(root, targetWorkspace))
	if targetHint == "" {
		targetHint = targetWorkspace
	}
	return newPreconditionError(
		"active_change_exists",
		fmt.Sprintf("active governed change %s already owns target workspace %s; finish or cancel it before creating a new one there", ch.Slug, targetHint),
		"Use a different workspace, or run `slipway done` / `slipway cancel` for the existing change before creating a new change there.",
		ch.Slug,
		nil,
	)
}
