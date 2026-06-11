package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

type VerificationLoadError struct {
	Path string
	Err  error
}

func (e *VerificationLoadError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *VerificationLoadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapVerificationLoadError(path string, err error) error {
	if err == nil {
		return nil
	}
	var loadErr *VerificationLoadError
	if errors.As(err, &loadErr) {
		return err
	}
	return &VerificationLoadError{
		Path: path,
		Err:  err,
	}
}

// VerificationDir returns the per-change verification directory within the
// governed bundle. root should be the workspace root (project root for
// non-worktree changes, worktree path for worktree-bound changes).
// Path: {root}/artifacts/changes/{slug}/verification/
func VerificationDir(root, slug string) string {
	return filepath.Join(root, "artifacts", "changes", slug, "verification")
}

func archivedVerificationDir(root, slug string) string {
	return filepath.Join(ArchivedBundlesDir(root), slug, "verification")
}

// resolveVerificationDir resolves the best-effort active verification directory
// for display helpers. Read/write entrypoints use the strict authority helpers
// below so hidden sibling bundles fail closed instead of drifting to stale local
// fallbacks.
func resolveVerificationDir(root, slug string) string {
	change, err := LoadChange(root, slug)
	if err != nil {
		return VerificationDir(root, slug)
	}
	dir, err := verificationDirForChange(root, change)
	if err != nil {
		return VerificationDir(root, slug)
	}
	return dir
}

func verificationDirForChange(root string, change model.Change) (string, error) {
	paths, err := ResolveChangePaths(root, change)
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.GovernedBundleDir, "verification"), nil
}

func resolveVerificationDirForRead(root, slug string) (string, error) {
	change, err := LoadChange(root, slug)
	if err == nil {
		return verificationDirForChange(root, change)
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errMissingBundleAuthority) {
		siblingBundleDir, siblingErr := findSiblingBundleDirRegardlessOfVisibility(root, slug)
		if siblingErr != nil {
			return "", siblingErr
		}
		if siblingBundleDir != "" {
			return "", fmt.Errorf("authoritative bundle exists in sibling workspace: %s", siblingBundleDir)
		}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return VerificationDir(root, slug), fs.ErrNotExist
	}
	if errors.Is(err, errMissingBundleAuthority) {
		return "", err
	}
	return "", err
}

func resolveVerificationDirForWrite(root, slug string) (string, error) {
	change, err := LoadChange(root, slug)
	if err == nil {
		return verificationDirForChange(root, change)
	}
	siblingBundleDir, siblingErr := findSiblingBundleDirRegardlessOfVisibility(root, slug)
	if siblingErr != nil {
		return "", siblingErr
	}
	if siblingBundleDir != "" {
		return "", fmt.Errorf("authoritative bundle exists in sibling workspace: %s", siblingBundleDir)
	}
	switch {
	case errors.Is(err, errMissingBundleAuthority):
		return "", err
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("authoritative change bundle not found for %q", slug)
	default:
		return "", err
	}
}

// Ordinary discovery only considers visible workspace roots, but authority
// guards must scan every registered worktree root. That asymmetry is
// intentional: hidden sibling bundles should fail closed instead of allowing
// local reads/writes to drift onto stale fallback files.
func findSiblingBundleDirRegardlessOfVisibility(root, slug string) (string, error) {
	roots, err := allWorkspaceRoots(root)
	if err != nil {
		return "", err
	}
	if len(roots) == 0 {
		return "", nil
	}
	normalizedRoot := roots[0]
	for _, candidateRoot := range roots[1:] {
		normalizedCandidate, err := NormalizePath(candidateRoot)
		if err != nil {
			normalizedCandidate = filepath.Clean(candidateRoot)
		}
		if normalizedCandidate == normalizedRoot {
			continue
		}
		info, err := os.Stat(normalizedCandidate)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", err
		}
		if !info.IsDir() {
			continue
		}
		bundleDir := filepath.Join(ActiveBundlesDir(normalizedCandidate), slug)
		info, err = os.Stat(bundleDir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", err
		}
		if info.IsDir() {
			authorityPath := BundleChangeFilePath(normalizedCandidate, slug)
			authorityInfo, err := os.Stat(authorityPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return "", err
			}
			if authorityInfo.IsDir() {
				continue
			}
			return bundleDir, nil
		}
	}
	return "", nil
}

// resolveExistingVerificationDir resolves the current verification directory
// for reads. Active bundles win; archived bundles fall back to
// artifacts/changes/archived/{slug}/verification when the active bundle no
// longer exists.
func resolveExistingVerificationDir(root, slug string) (string, error) {
	dir, err := resolveVerificationDirForRead(root, slug)
	if err == nil {
		return dir, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	archivedDir := archivedVerificationDir(root, slug)
	if _, err := os.Stat(BundleArchivedChangeFilePath(root, slug)); err == nil {
		return archivedDir, nil
	}
	return dir, fs.ErrNotExist
}

func verificationDirPathForRead(root, slug string) string {
	change, err := LoadChange(root, slug)
	if err == nil {
		if dir, dirErr := verificationDirForChange(root, change); dirErr == nil {
			return dir
		}
	}
	change, err = loadChangeRegardlessOfVisibility(root, slug)
	if err == nil {
		if dir, dirErr := verificationDirForChange(root, change); dirErr == nil {
			return dir
		}
	}
	if _, err := os.Stat(BundleArchivedChangeFilePath(root, slug)); err == nil {
		return archivedVerificationDir(root, slug)
	}
	return VerificationDir(root, slug)
}

// VerificationFilePath returns the best-effort display path to a specific
// skill's verification file. It does not imply that reads or writes at that
// path are authoritative.
func VerificationFilePath(root, slug, skillName string) string {
	return filepath.Join(resolveVerificationDir(root, slug), skillName+".yaml")
}

// SaveVerification writes a validated skill verification record to the
// authoritative governed bundle for the active change.
func SaveVerification(root, slug, skillName string, rec model.VerificationRecord) (string, error) {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return "", fmt.Errorf("skill name is required")
	}
	rec.Normalize()
	if err := rec.Validate(); err != nil {
		return "", err
	}
	dir, err := resolveVerificationDirForWrite(root, slug)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, skillName+".yaml")
	raw, err := yaml.Marshal(rec)
	if err != nil {
		return "", err
	}
	if err := fsutil.WriteFileAtomic(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadVerification reads a skill verification record.
func LoadVerification(root, slug, skillName string) (model.VerificationRecord, error) {
	dir, err := resolveExistingVerificationDir(root, slug)
	if err != nil {
		return model.VerificationRecord{}, err
	}
	path := filepath.Join(dir, skillName+".yaml")
	rec, err := loadVerificationFromPath(path, skillName)
	if err != nil {
		return model.VerificationRecord{}, wrapVerificationLoadError(path, err)
	}
	return rec, nil
}

// ListVerifications reads all verification records for a change.
// Returns a map of skillName → VerificationRecord.
func ListVerifications(root, slug string) (map[string]model.VerificationRecord, error) {
	dir, err := resolveExistingVerificationDir(root, slug)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]model.VerificationRecord{}, nil
		}
		return nil, err
	}
	return listVerificationsInDir(dir)
}

// ListVerificationsForChange reads all verification records for a resolved
// authoritative change without re-discovering bundle ownership by slug.
func ListVerificationsForChange(root string, change model.Change) (map[string]model.VerificationRecord, error) {
	dir, err := verificationDirForChange(root, change)
	if err != nil {
		return nil, err
	}
	return listVerificationsInDir(dir)
}

func listVerificationsInDir(dir string) (map[string]model.VerificationRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]model.VerificationRecord{}, nil
		}
		return nil, wrapVerificationLoadError(dir, err)
	}

	result := make(map[string]model.VerificationRecord, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		if name == ExecutionSummaryFileName || name == WavePlanFileName || name == EvidenceDigestsFileName {
			continue
		}
		skillName := strings.TrimSuffix(name, ".yaml")
		path := filepath.Join(dir, name)
		rec, err := loadVerificationFromPath(path, skillName)
		if err != nil {
			return nil, wrapVerificationLoadError(path, err)
		}
		result[skillName] = rec
	}
	return result, nil
}

func decodeVerificationStrict(raw []byte, rec *model.VerificationRecord) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	return decoder.Decode(rec)
}

// loadVerificationFromPath reads and validates a verification record from a
// specific file path, avoiding redundant directory resolution.
func loadVerificationFromPath(path, skillName string) (model.VerificationRecord, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return model.VerificationRecord{}, err
	}
	var rec model.VerificationRecord
	if err := decodeVerificationStrict(b, &rec); err != nil {
		return model.VerificationRecord{}, fmt.Errorf("parse verification %s: %w", skillName, err)
	}
	rec.Normalize()
	if err := rec.Validate(); err != nil {
		return model.VerificationRecord{}, fmt.Errorf("invalid verification %s: %w", skillName, err)
	}
	return rec, nil
}
