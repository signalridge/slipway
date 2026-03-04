package fsutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"
)

var ErrLockTimeout = errors.New("state lock timeout")

type LockMeta struct {
	HolderPID  int       `yaml:"holder_pid"`
	AcquiredAt time.Time `yaml:"acquired_at"`
	Command    string    `yaml:"command"`
}

type StateLock struct {
	lockPath   string
	metaPath   string
	retryDelay time.Duration
}

func NewStateLock(lockPath string) *StateLock {
	return &StateLock{
		lockPath:   lockPath,
		metaPath:   lockPath + ".meta",
		retryDelay: 20 * time.Millisecond,
	}
}

func (s *StateLock) LockPath() string {
	return s.lockPath
}

func (s *StateLock) MetaPath() string {
	return s.metaPath
}

type HeldLock struct {
	stateLock *StateLock
	fl        *flock.Flock
	once      sync.Once
}

func (s *StateLock) Acquire(ctx context.Context, timeout time.Duration, command string) (*HeldLock, error) {
	if err := os.MkdirAll(filepath.Dir(s.lockPath), 0o755); err != nil {
		return nil, err
	}

	f := flock.New(s.lockPath)
	lockCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		lockCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	locked, err := f.TryLockContext(lockCtx, s.retryDelay)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %s", ErrLockTimeout, s.lockPath)
		}
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("%w: %s", ErrLockTimeout, s.lockPath)
	}

	meta := LockMeta{
		HolderPID:  os.Getpid(),
		AcquiredAt: time.Now().UTC(),
		Command:    command,
	}
	if err := s.WriteMeta(meta); err != nil {
		_ = f.Unlock()
		return nil, err
	}

	return &HeldLock{
		stateLock: s,
		fl:        f,
	}, nil
}

func (h *HeldLock) Release() error {
	var releaseErr error
	h.once.Do(func() {
		if err := os.Remove(h.stateLock.metaPath); err != nil && !os.IsNotExist(err) {
			releaseErr = err
		}
		if err := h.fl.Unlock(); err != nil && releaseErr == nil {
			releaseErr = err
		}
	})
	return releaseErr
}

func (s *StateLock) ReadMeta() (LockMeta, error) {
	b, err := os.ReadFile(s.metaPath)
	if err != nil {
		return LockMeta{}, err
	}
	var meta LockMeta
	if err := yaml.Unmarshal(b, &meta); err != nil {
		return LockMeta{}, err
	}
	return meta, nil
}

func (s *StateLock) WriteMeta(meta LockMeta) error {
	b, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.metaPath, b, 0o644)
}

// CleanupStale removes stale lock artifacts when holder is dead and age is beyond threshold.
func (s *StateLock) CleanupStale(staleAfter time.Duration, now time.Time, isPIDAlive func(pid int) bool) (bool, error) {
	meta, err := s.ReadMeta()
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if staleAfter > 0 && now.Sub(meta.AcquiredAt) < staleAfter {
		return false, nil
	}
	if isPIDAlive != nil && isPIDAlive(meta.HolderPID) {
		return false, nil
	}

	// If lock is currently held, avoid destructive cleanup.
	fl := flock.New(s.lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	if err := fl.Unlock(); err != nil {
		return false, err
	}

	cleaned := false
	if err := os.Remove(s.lockPath); err == nil {
		cleaned = true
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.Remove(s.metaPath); err == nil {
		cleaned = true
	} else if !os.IsNotExist(err) {
		return false, err
	}
	return cleaned, nil
}
