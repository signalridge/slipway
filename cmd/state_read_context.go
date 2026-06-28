package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

type stateReadContext struct {
	root                      string
	invocationWorkspaceRoot   string
	invocationWorkspaceLoaded bool
	currentWorktreeRoot       string
	currentWorktreeRootErr    error
	currentWorktreeRootLoaded bool
	changes                   map[string]model.Change
	changesList               []model.Change
	changesListLoaded         bool
	paths                     map[string]state.ResolvedChangePaths
	verifications             map[string]map[string]model.VerificationRecord
	execution                 map[string]executionContext
	activeRefs                map[string]activeChangeRefCacheEntry
}

type activeChangeRefCacheEntry struct {
	ref changeRef
	err error
}

func newStateReadContext(root string) *stateReadContext {
	return &stateReadContext{
		root:          root,
		changes:       make(map[string]model.Change),
		paths:         make(map[string]state.ResolvedChangePaths),
		verifications: make(map[string]map[string]model.VerificationRecord),
		execution:     make(map[string]executionContext),
		activeRefs:    make(map[string]activeChangeRefCacheEntry),
	}
}

func (ctx *stateReadContext) loadChange(slug string) (model.Change, error) {
	slug = strings.TrimSpace(slug)
	if change, ok := ctx.changes[slug]; ok {
		return change, nil
	}
	change, err := state.LoadChangeFast(ctx.root, slug)
	if err != nil {
		return model.Change{}, err
	}
	ctx.rememberChange(change)
	return change, nil
}

func (ctx *stateReadContext) reloadChange(slug string) (model.Change, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return model.Change{}, fmt.Errorf("slug is required")
	}
	delete(ctx.changes, slug)
	delete(ctx.paths, slug)
	delete(ctx.verifications, slug)
	delete(ctx.execution, slug)
	ctx.invalidateRouteCaches()

	change, err := state.LoadChangeFast(ctx.root, slug)
	if err != nil {
		return model.Change{}, err
	}
	ctx.rememberChange(change)
	return change, nil
}

func (ctx *stateReadContext) rememberChange(change model.Change) {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return
	}
	ctx.changes[slug] = change
}

func (ctx *stateReadContext) invalidateRouteCaches() {
	ctx.changesList = nil
	ctx.changesListLoaded = false
	ctx.activeRefs = make(map[string]activeChangeRefCacheEntry)
}

func (ctx *stateReadContext) invocationWorkspace() string {
	if ctx.invocationWorkspaceLoaded {
		return ctx.invocationWorkspaceRoot
	}
	workspaceRoot := ctx.root
	wd, err := os.Getwd()
	if err == nil {
		if resolved, resolveErr := state.ResolveGitWorkspaceRoot(wd); resolveErr == nil && resolved != "" {
			workspaceRoot = resolved
		}
	}
	if normalized, err := state.NormalizePath(workspaceRoot); err == nil {
		workspaceRoot = normalized
	} else {
		workspaceRoot = filepath.Clean(workspaceRoot)
	}
	ctx.invocationWorkspaceRoot = workspaceRoot
	ctx.invocationWorkspaceLoaded = true
	return ctx.invocationWorkspaceRoot
}

func (ctx *stateReadContext) currentWorktree() (string, error) {
	if ctx.currentWorktreeRootLoaded {
		return ctx.currentWorktreeRoot, ctx.currentWorktreeRootErr
	}
	ctx.currentWorktreeRoot, ctx.currentWorktreeRootErr = currentWorktreeRoot()
	ctx.currentWorktreeRootLoaded = true
	return ctx.currentWorktreeRoot, ctx.currentWorktreeRootErr
}

func (ctx *stateReadContext) activeRef(explicitSlug string) (changeRef, error) {
	key := strings.TrimSpace(explicitSlug)
	if entry, ok := ctx.activeRefs[key]; ok {
		return entry.ref, entry.err
	}
	ref, err := ctx.resolveActiveRefUncached(key)
	ctx.activeRefs[key] = activeChangeRefCacheEntry{ref: ref, err: err}
	return ref, err
}

func (ctx *stateReadContext) resolveActiveRefUncached(explicitSlug string) (changeRef, error) {
	root := ctx.root
	if strings.TrimSpace(explicitSlug) != "" {
		return resolveExplicitChangeWithReadContext(ctx, strings.TrimSpace(explicitSlug))
	}

	worktreePath, err := ctx.currentWorktree()
	if err != nil {
		return changeRef{}, wrapResolutionErrorForRoot(root, err)
	}
	if worktreePath != "" {
		if change, ok, err := resolveActiveChangeRefFromWorktreeBinding(root, worktreePath); err != nil {
			return changeRef{}, err
		} else if ok {
			ctx.rememberChange(change)
			return changeRef{Slug: change.Slug}, nil
		}

		change, err := ctx.findActiveChangeForWorktree(worktreePath)
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
					return resolveExplicitChangeWithReadContext(ctx, archived.Slug)
				}
			}
			if recoveryErr := deleteRecoveryError(root, ""); recoveryErr != nil {
				return changeRef{}, recoveryErr
			}
			return changeRef{}, wrapResolutionErrorForRoot(root, err)
		}
		if strings.TrimSpace(change.WorktreePath) == "" {
			// A single unbound active change is only a fallback. In an archived
			// review worktree, local archived authority is more specific and must
			// fail closed before commands operate on the unrelated unbound change.
			if archived, ok, archErr := state.FindArchivedChangeForWorktree(root, worktreePath); archErr != nil {
				return changeRef{}, wrapArchivedWorktreeResolutionError(archErr)
			} else if ok {
				return resolveExplicitChangeWithReadContext(ctx, archived.Slug)
			}
		}
		ctx.rememberChange(change)
		return changeRef{Slug: change.Slug}, nil
	}

	change, err := ctx.findActiveChange()
	if err != nil {
		if recoveryErr := deleteRecoveryError(root, ""); recoveryErr != nil {
			return changeRef{}, recoveryErr
		}
		return changeRef{}, wrapResolutionErrorForRoot(root, err)
	}
	ctx.rememberChange(change)
	return changeRef{Slug: change.Slug}, nil
}

func (ctx *stateReadContext) listChanges() ([]model.Change, error) {
	if ctx.changesListLoaded {
		return slices.Clone(ctx.changesList), nil
	}
	changes, err := state.ListChanges(ctx.root)
	if err != nil {
		return nil, err
	}
	ctx.changesList = slices.Clone(changes)
	ctx.changesListLoaded = true
	for _, change := range changes {
		ctx.rememberChange(change)
	}
	return slices.Clone(changes), nil
}

func (ctx *stateReadContext) findActiveChange() (model.Change, error) {
	changes, err := ctx.listChanges()
	if err != nil {
		return model.Change{}, err
	}
	return state.SelectActiveChange(changes)
}

func (ctx *stateReadContext) findActiveChangeForWorktree(currentWorktreePath string) (model.Change, error) {
	changes, err := ctx.listChanges()
	if err != nil {
		return model.Change{}, err
	}
	return state.SelectActiveChangeForWorktree(changes, currentWorktreePath)
}

func (ctx *stateReadContext) resolvedPaths(change model.Change) (state.ResolvedChangePaths, error) {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return state.ResolvedChangePaths{}, fmt.Errorf("slug is required")
	}
	if paths, ok := ctx.paths[slug]; ok {
		return paths, nil
	}
	paths, err := state.ResolveChangePaths(ctx.root, change)
	if err != nil {
		return state.ResolvedChangePaths{}, err
	}
	ctx.paths[slug] = paths
	return paths, nil
}

func (ctx *stateReadContext) verificationRecords(change model.Change) (map[string]model.VerificationRecord, error) {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	if records, ok := ctx.verifications[slug]; ok {
		return records, nil
	}
	records, err := state.ListVerificationsForChange(ctx.root, change)
	if err != nil {
		return nil, err
	}
	ctx.verifications[slug] = records
	return records, nil
}

func (ctx *stateReadContext) loadExecution(change model.Change) (executionContext, error) {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return executionContext{}, fmt.Errorf("slug is required")
	}
	if execCtx, ok := ctx.execution[slug]; ok {
		return execCtx, nil
	}
	execCtx, err := loadExecutionContext(ctx.root, change)
	if err != nil {
		return executionContext{}, err
	}
	ctx.execution[slug] = execCtx
	return execCtx, nil
}

func (ctx *stateReadContext) lifecycleEventTailWithPredecessorTransition(
	change model.Change,
	limit int,
) ([]state.LifecycleEvent, error) {
	paths, err := ctx.resolvedPaths(change)
	if err != nil {
		return nil, err
	}
	activePath := filepath.Join(paths.GovernedBundleDir, "events", state.LifecycleEventLogFileName)
	if _, statErr := os.Stat(activePath); statErr == nil {
		return state.ReadLifecycleEventTailWithPredecessorTransitionFromPath(activePath, limit)
	}
	archivedPath := filepath.Join(paths.GovernedBundleArchive, "events", state.LifecycleEventLogFileName)
	if _, statErr := os.Stat(archivedPath); statErr == nil {
		return state.ReadLifecycleEventTailWithPredecessorTransitionFromPath(archivedPath, limit)
	}
	return state.ReadLifecycleEventTailWithPredecessorTransitionFromPath(activePath, limit)
}
