package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// withChangeStateLock acquires a per-change file lock before running fn.
// Design note (TOCTOU): callers resolve the active change (via worktree or --change)
// BEFORE acquiring this lock. The gap is benign because resolution is read-only, and
// mutations inside fn reload state under the lock. Moving resolution inside the lock
// would require a global lock for the resolution step, defeating per-change isolation.
func withChangeStateLock(root string, changeSlug string, commandName string, run func() error) error {
	return withChangeStateLockConfigured(root, changeSlug, commandName, loadConfigAtRoot, run)
}

func withChangeStateLockConfigured(
	root string,
	changeSlug string,
	commandName string,
	loadConfig func(string) (model.Config, error),
	run func() error,
) error {
	cfg, err := loadConfig(root)
	if err != nil {
		return err
	}

	// Ensure the git-internal lock directory exists before acquiring the lock.
	lockPath := state.ChangeStateLockPath(root, changeSlug)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	lock := fsutil.NewStateLock(lockPath)
	timeout := commandLockWaitDuration(cfg.Execution.LockWaitTimeoutSeconds)
	held, err := acquireHeldLock(lock, timeout, "slipway "+commandName, func(lockPath string) error {
		return newPreconditionError(
			"state_lock_timeout",
			fmt.Sprintf("state lock timeout while running `%s` on change %s", commandName, changeSlug),
			"Run `slipway repair` to clear stale lock artifacts or retry after lock holder exits.",
			changeSlug,
			map[string]any{
				"lock_path":   lockPath,
				"command":     commandName,
				"change_slug": changeSlug,
			},
		)
	})
	if err != nil {
		return err
	}
	defer releaseHeldLock(held)

	return run()
}

// withChangeCreateLock acquires the global change-creation lock.
func withChangeCreateLock(root string, run func() error) error {
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return err
	}

	timeout := commandLockWaitDuration(cfg.Execution.LockWaitTimeoutSeconds)
	buildTimeoutError := func(lockPath string) error {
		return newPreconditionError(
			"state_lock_timeout",
			"state lock timeout while creating change",
			"Run `slipway repair` to clear stale lock artifacts or retry after lock holder exits.",
			"",
			map[string]any{
				"lock_path": lockPath,
				"command":   "new",
			},
		)
	}
	lockPaths := changeCreateLockPaths(root)
	heldLocks := make([]*fsutil.HeldLock, 0, len(lockPaths))
	for _, lockPath := range lockPaths {
		lock := fsutil.NewStateLock(lockPath)
		held, err := acquireHeldLock(lock, timeout, "slipway new", buildTimeoutError)
		if err != nil {
			for i := len(heldLocks) - 1; i >= 0; i-- {
				releaseHeldLock(heldLocks[i])
			}
			return err
		}
		heldLocks = append(heldLocks, held)
	}
	defer func() {
		for i := len(heldLocks) - 1; i >= 0; i-- {
			releaseHeldLock(heldLocks[i])
		}
	}()

	return run()
}

func changeCreateLockPaths(root string) []string {
	return []string{
		state.ChangeCreateLockPath(root),
	}
}

func withWorkspaceRepairLock(root string, run func(staleLockCleaned bool) error) error {
	cfg := loadRepairLockConfigAtRoot(root)
	lockPath := state.RepairLockPath(root)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	lock := fsutil.NewStateLock(lockPath)
	timeout := commandLockWaitDuration(cfg.Execution.LockWaitTimeoutSeconds)

	timeoutErr := func(lockPath string) error {
		return newPreconditionError(
			"state_lock_timeout",
			"state lock timeout while running `repair`",
			"Retry after lock holder exits.",
			"",
			map[string]any{
				"lock_path": lockPath,
				"command":   "repair",
			},
		)
	}

	held, err := acquireHeldLock(lock, timeout, "slipway repair", timeoutErr)
	staleLockCleaned := false
	if err != nil {
		var cliErr *CLIError
		if !errors.As(err, &cliErr) || cliErr.ErrorCode != "state_lock_timeout" {
			return err
		}

		staleAfter := time.Duration(cfg.Execution.LockStaleAfterSeconds) * time.Second
		staleLockCleaned, err = lock.CleanupStale(staleAfter, time.Now().UTC(), isPIDAlive)
		if err != nil {
			return err
		}
		if !staleLockCleaned {
			return timeoutErr(lockPath)
		}

		held, err = acquireHeldLock(lock, timeout, "slipway repair", timeoutErr)
		if err != nil {
			return err
		}
	}
	defer releaseHeldLock(held)

	return run(staleLockCleaned)
}

func loadRepairLockConfigAtRoot(root string) model.Config {
	cfgPath := state.ConfigPath(root)
	if cfg, err := model.LoadConfig(cfgPath); err == nil {
		return cfg
	}
	return model.DefaultConfig()
}
