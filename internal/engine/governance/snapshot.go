package governance

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"gopkg.in/yaml.v3"
)

// SnapshotFileName is the canonical file name for the governance sidecar.
const SnapshotFileName = "governance_snapshot.yaml"

// SnapshotPath returns the canonical path for a change's governance snapshot.
func SnapshotPath(root, slug string) string {
	return state.GovernanceSnapshotCachePath(root, slug)
}

// LoadSnapshot loads the governance snapshot for the given change slug.
// Returns a zero-value snapshot and no error if the file does not exist.
func LoadSnapshot(root, slug string) (model.GovernanceSnapshot, error) {
	path := SnapshotPath(root, slug)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return model.GovernanceSnapshot{}, nil
		}
		return model.GovernanceSnapshot{}, fmt.Errorf("read governance snapshot: %w", err)
	}
	var snap model.GovernanceSnapshot
	if err := decodeGovernanceSnapshotStrict(b, &snap); err != nil {
		return model.GovernanceSnapshot{}, fmt.Errorf("parse governance snapshot: %w", err)
	}
	if snap.Version > 0 {
		if err := snap.Validate(); err != nil {
			return model.GovernanceSnapshot{}, fmt.Errorf("validate governance snapshot: %w", err)
		}
	}
	return snap, nil
}

// BackupUnreadableSnapshot preserves an unreadable governance snapshot alongside
// the canonical path and removes the unreadable file so callers can regenerate
// a clean sidecar without silently destroying the broken payload.
func BackupUnreadableSnapshot(root, slug string, now time.Time) (string, error) {
	path := SnapshotPath(root, slug)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read unreadable governance snapshot: %w", err)
	}

	backupPath := filepath.Join(
		filepath.Dir(path),
		fmt.Sprintf("governance_snapshot.broken.%s.yaml", now.UTC().Format("20060102T150405.000000000Z")),
	)
	if err := fsutil.WriteFileAtomic(backupPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("write unreadable governance snapshot backup: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("remove unreadable governance snapshot: %w", err)
	}
	return backupPath, nil
}

// SaveSnapshot persists the governance snapshot, skipping the write only when
// the material governance payload is unchanged.
// Callers must coordinate through the per-change state lock before calling this
// function; SaveSnapshot does not acquire or validate that lock itself.
func SaveSnapshot(root, slug string, snap model.GovernanceSnapshot) error {
	if err := snap.Validate(); err != nil {
		return fmt.Errorf("validate governance snapshot: %w", err)
	}

	path := SnapshotPath(root, slug)

	// Check whether the existing snapshot is identical.
	existing, err := LoadSnapshot(root, slug)
	if err == nil && existing.Version > 0 && existing.PersistedEqual(snap) {
		return nil // no material change
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}

	b, err := yaml.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal governance snapshot: %w", err)
	}
	return fsutil.WriteFileAtomic(path, b, 0o644)
}

func decodeGovernanceSnapshotStrict(raw []byte, snap *model.GovernanceSnapshot) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	return decoder.Decode(snap)
}
