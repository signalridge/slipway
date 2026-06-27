package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

type stateReadContext struct {
	root          string
	changes       map[string]model.Change
	paths         map[string]state.ResolvedChangePaths
	verifications map[string]map[string]model.VerificationRecord
	execution     map[string]executionContext
}

func newStateReadContext(root string) *stateReadContext {
	return &stateReadContext{
		root:          root,
		changes:       make(map[string]model.Change),
		paths:         make(map[string]state.ResolvedChangePaths),
		verifications: make(map[string]map[string]model.VerificationRecord),
		execution:     make(map[string]executionContext),
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
