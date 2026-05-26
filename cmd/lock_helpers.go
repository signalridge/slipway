package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
)

var (
	commandLockWaitUnit    = time.Second
	commandCancelGraceUnit = time.Second
)

func commandLockWaitDuration(seconds int) time.Duration {
	return time.Duration(seconds) * commandLockWaitUnit
}

func commandCancelGraceDuration(seconds int) time.Duration {
	return time.Duration(seconds) * commandCancelGraceUnit
}

func acquireHeldLock(
	lock *fsutil.StateLock,
	timeout time.Duration,
	command string,
	buildTimeoutError func(string) error,
) (*fsutil.HeldLock, error) {
	held, err := lock.Acquire(context.Background(), timeout, command)
	if err == nil {
		return held, nil
	}
	if errors.Is(err, fsutil.ErrLockTimeout) && buildTimeoutError != nil {
		return nil, buildTimeoutError(lock.LockPath())
	}
	return nil, err
}

func releaseHeldLock(held *fsutil.HeldLock) {
	if held == nil {
		return
	}
	_ = held.Release()
}
