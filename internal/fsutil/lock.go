package fsutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

type HeldLock struct {
	stateLock  *StateLock
	fl         *flock.Flock
	once       sync.Once
	unlock     func() error
	removeMeta func(string) error
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
		stateLock:  s,
		fl:         f,
		unlock:     f.Unlock,
		removeMeta: os.Remove,
	}, nil
}

func (h *HeldLock) Release() error {
	unlock := h.unlock
	if unlock == nil {
		unlock = h.fl.Unlock
	}
	removeMeta := h.removeMeta
	if removeMeta == nil {
		removeMeta = os.Remove
	}

	var releaseErr error
	h.once.Do(func() {
		// Unlock before removing meta so that if a crash occurs between the two
		// operations, CleanupStale can still read the (now-stale) meta and clean
		// up properly. The reverse order would leave an orphaned lock file with
		// no meta for CleanupStale to evaluate.
		releaseErr = errors.Join(unlock(), removeMetaIfPresent(removeMeta, h.stateLock.metaPath))
	})
	return releaseErr
}

func removeMetaIfPresent(removeMeta func(string) error, metaPath string) error {
	err := removeMeta(metaPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
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
		if errors.Is(err, fs.ErrNotExist) {
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

	// Acquire the lock to prevent racing with another process that may grab
	// the lock between our check and removal. Remove artifacts while holding
	// the lock so no other process can acquire + write fresh meta in between.
	fl := flock.New(s.lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return false, err
	}
	if !locked {
		// Lock is actively held by another process — not safe to clean up.
		return false, nil
	}
	defer func() {
		_ = fl.Unlock()
	}()

	cleaned := false
	if err := os.Remove(s.metaPath); err == nil {
		cleaned = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.Remove(s.lockPath); err == nil {
		cleaned = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return cleaned, nil
}
