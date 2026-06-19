package toolgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
)

const (
	ownershipManifestVersion  = 1
	ownershipManifestFileName = "ownership-manifest.json"
)

type ownershipClassification string

const (
	ownershipUnknownUserFile     ownershipClassification = "unknown_user_file"
	ownershipPristineManagedFile ownershipClassification = "pristine_managed_file"
	ownershipManagedModifiedFile ownershipClassification = "managed_modified_file"
	ownershipManagedMissingFile  ownershipClassification = "managed_missing_file"
)

type ownershipManifest struct {
	Version int                     `json:"version"`
	ToolID  string                  `json:"tool_id"`
	Files   []ownershipManifestFile `json:"files"`
}

type ownershipManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type toolRefreshPlan struct {
	root                         string
	cfg                          ToolConfig
	previousIndex                map[string]ownershipManifestFile
	manifestlessBootstrapAllowed bool
	generated                    map[string]ownershipManifestFile
	ops                          []fsutil.FileTransactionOp
	transactionStarted           bool
}

var toolgenApplyFileTransaction = fsutil.ApplyFileTransaction

func newToolRefreshPlan(
	root string,
	cfg ToolConfig,
	refresh bool,
	manifestlessBootstrapAllowed bool,
) (*toolRefreshPlan, error) {
	if !refresh {
		return nil, nil
	}
	manifest, found, err := loadOwnershipManifest(root, cfg)
	if err != nil {
		return nil, err
	}
	previousIndex := map[string]ownershipManifestFile{}
	if found {
		previousIndex = manifest.index()
	}
	return &toolRefreshPlan{
		root:                         filepath.Clean(root),
		cfg:                          cfg,
		previousIndex:                previousIndex,
		manifestlessBootstrapAllowed: manifestlessBootstrapAllowed,
		generated:                    map[string]ownershipManifestFile{},
	}, nil
}

func generatedOwnershipManifestPath(cfg ToolConfig) string {
	return filepath.Join(ToolRootPath(cfg), "slipway", ownershipManifestFileName)
}

func loadOwnershipManifest(root string, cfg ToolConfig) (ownershipManifest, bool, error) {
	manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
	raw, err := os.ReadFile(manifestPath) // #nosec G304 -- path is rooted under the adapter-owned project directory.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ownershipManifest{}, false, nil
		}
		return ownershipManifest{}, false, fmt.Errorf("read ownership manifest for %s: %w", cfg.ID, err)
	}
	var manifest ownershipManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return ownershipManifest{}, false, fmt.Errorf("parse ownership manifest for %s: %w", cfg.ID, err)
	}
	if manifest.Version != ownershipManifestVersion {
		return ownershipManifest{}, false, fmt.Errorf("unsupported ownership manifest version %d for %s", manifest.Version, cfg.ID)
	}
	if manifest.ToolID != cfg.ID {
		return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s belongs to %s", cfg.ID, manifest.ToolID)
	}
	for i := range manifest.Files {
		rel, err := normalizeOwnershipPath(manifest.Files[i].Path)
		if err != nil {
			return ownershipManifest{}, false, fmt.Errorf("invalid ownership manifest path %q: %w", manifest.Files[i].Path, err)
		}
		manifest.Files[i].Path = rel
		manifest.Files[i].SHA256 = strings.ToLower(strings.TrimSpace(manifest.Files[i].SHA256))
	}
	slices.SortFunc(manifest.Files, func(a, b ownershipManifestFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	return manifest, true, nil
}

func (manifest ownershipManifest) index() map[string]ownershipManifestFile {
	index := make(map[string]ownershipManifestFile, len(manifest.Files))
	for _, file := range manifest.Files {
		index[file.Path] = file
	}
	return index
}

func encodeOwnershipManifest(manifest ownershipManifest) ([]byte, error) {
	slices.SortFunc(manifest.Files, func(a, b ownershipManifestFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func buildOwnershipManifest(toolID string, generated map[string]ownershipManifestFile) ownershipManifest {
	files := make([]ownershipManifestFile, 0, len(generated))
	for _, file := range generated {
		files = append(files, file)
	}
	slices.SortFunc(files, func(a, b ownershipManifestFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	return ownershipManifest{
		Version: ownershipManifestVersion,
		ToolID:  toolID,
		Files:   files,
	}
}

func classifyOwnership(root string, manifest ownershipManifest, relPath string) (ownershipClassification, error) {
	rel, err := normalizeOwnershipPath(relPath)
	if err != nil {
		return "", err
	}
	record, ok := manifest.index()[rel]
	if !ok {
		return ownershipUnknownUserFile, nil
	}
	sum, err := hashFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ownershipManagedMissingFile, nil
		}
		return "", err
	}
	if sum == record.SHA256 {
		return ownershipPristineManagedFile, nil
	}
	return ownershipManagedModifiedFile, nil
}

func normalizeOwnershipPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("path is required")
	}
	rel := filepath.Clean(filepath.FromSlash(name))
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must be relative and local: %s", name)
	}
	return filepath.ToSlash(rel), nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is rooted under the project or supplied by tests.
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

func (plan *toolRefreshPlan) writeGeneratedFile(path string, data []byte, mode os.FileMode) error {
	rel, insideRoot, err := plan.relativePath(path)
	if err != nil {
		return err
	}
	if !insideRoot {
		return fmt.Errorf("generated file %s is outside %s", path, plan.root)
	}
	if rel == filepath.ToSlash(generatedOwnershipManifestPath(plan.cfg)) {
		return errors.New("ownership manifest must be written by the refresh plan")
	}
	if err := plan.ensureGeneratedWriteAllowed(path, rel, data); err != nil {
		return err
	}
	plan.generated[rel] = ownershipManifestFile{
		Path:   rel,
		SHA256: hashBytes(data),
	}
	plan.ops = append(plan.ops, fsutil.WriteFileTransactionOp(path, data, mode))
	return nil
}

func (plan *toolRefreshPlan) writeUnmanagedFile(path string, data []byte, mode os.FileMode) error {
	plan.ops = append(plan.ops, fsutil.WriteFileTransactionOp(path, data, mode))
	return nil
}

func (plan *toolRefreshPlan) ensureGeneratedWriteAllowed(path, rel string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect generated file %s: %w", rel, err)
	}
	if info.IsDir() {
		return fmt.Errorf("refusing to overwrite generated directory %s", rel)
	}
	currentHash, err := hashFile(path)
	if err != nil {
		return fmt.Errorf("hash generated file %s: %w", rel, err)
	}
	if record, ok := plan.previousIndex[rel]; ok {
		if currentHash != record.SHA256 {
			return fmt.Errorf("refusing to overwrite managed-modified generated file %s", rel)
		}
		return nil
	}
	// Bootstrap: when no previous manifest exists but a pre-existing adapter
	// sentinel proves Slipway generated the tree, allow overwriting so the
	// adapter can be brought into manifest tracking.
	if plan.isManifestlessBootstrap() {
		return nil
	}
	if currentHash == hashBytes(data) {
		return nil
	}
	return fmt.Errorf("refusing to overwrite unknown file %s", rel)
}

func (plan *toolRefreshPlan) invalidateTrustedGeneratedFile(path string) error {
	rel, insideRoot, err := plan.relativePath(path)
	if err != nil {
		return err
	}
	if !insideRoot {
		return nil
	}
	classification, err := plan.classifyExistingRelPath(rel, true)
	if err != nil {
		return err
	}
	switch classification {
	case ownershipPristineManagedFile:
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	case ownershipManagedModifiedFile:
		return fmt.Errorf("refusing to remove managed-modified generated file %s", rel)
	case ownershipUnknownUserFile, ownershipManagedMissingFile:
		return nil
	default:
		return fmt.Errorf("unknown ownership classification %q for %s", classification, rel)
	}
	return nil
}

func (plan *toolRefreshPlan) removeGeneratedPath(path string, allowManifestlessBootstrap bool) error {
	_, insideRoot, err := plan.relativePath(path)
	if err != nil {
		return err
	}
	if !insideRoot {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return plan.removeGeneratedFile(path, allowManifestlessBootstrap)
	}
	return plan.removeGeneratedTree(path, allowManifestlessBootstrap)
}

func (plan *toolRefreshPlan) removeGeneratedFile(path string, allowManifestlessBootstrap bool) error {
	rel, insideRoot, err := plan.relativePath(path)
	if err != nil {
		return err
	}
	if !insideRoot {
		return nil
	}
	classification, err := plan.classifyExistingRelPath(rel, allowManifestlessBootstrap)
	if err != nil {
		return err
	}
	switch classification {
	case ownershipPristineManagedFile:
		plan.ops = append(plan.ops, fsutil.RemoveFileTransactionOp(path))
	case ownershipManagedModifiedFile:
		return fmt.Errorf("refusing to remove managed-modified generated file %s", rel)
	case ownershipUnknownUserFile, ownershipManagedMissingFile:
		return nil
	default:
		return fmt.Errorf("unknown ownership classification %q for %s", classification, rel)
	}
	return nil
}

func (plan *toolRefreshPlan) removeGeneratedTree(path string, allowManifestlessBootstrap bool) error {
	var pristineFiles []string
	var hasUnknown bool
	err := filepath.WalkDir(path, func(entryPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, insideRoot, err := plan.relativePath(entryPath)
		if err != nil {
			return err
		}
		if !insideRoot {
			hasUnknown = true
			return nil
		}
		classification, err := plan.classifyExistingRelPath(rel, allowManifestlessBootstrap)
		if err != nil {
			return err
		}
		switch classification {
		case ownershipPristineManagedFile:
			pristineFiles = append(pristineFiles, entryPath)
		case ownershipManagedModifiedFile:
			return fmt.Errorf("refusing to remove managed-modified generated file %s", rel)
		case ownershipUnknownUserFile, ownershipManagedMissingFile:
			hasUnknown = true
		default:
			return fmt.Errorf("unknown ownership classification %q for %s", classification, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(pristineFiles) == 0 {
		return nil
	}
	if !hasUnknown {
		plan.ops = append(plan.ops, fsutil.RemoveAllTransactionOp(path))
		return nil
	}
	for _, file := range pristineFiles {
		plan.ops = append(plan.ops, fsutil.RemoveFileTransactionOp(file))
	}
	return nil
}

func (plan *toolRefreshPlan) classifyExistingRelPath(rel string, allowManifestlessBootstrap bool) (ownershipClassification, error) {
	record, ok := plan.previousIndex[rel]
	if !ok {
		if allowManifestlessBootstrap && plan.isManifestlessBootstrap() {
			return ownershipPristineManagedFile, nil
		}
		return ownershipUnknownUserFile, nil
	}
	sum, err := hashFile(filepath.Join(plan.root, filepath.FromSlash(rel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ownershipManagedMissingFile, nil
		}
		return "", fmt.Errorf("hash generated file %s: %w", rel, err)
	}
	if sum == record.SHA256 {
		return ownershipPristineManagedFile, nil
	}
	return ownershipManagedModifiedFile, nil
}

func (plan *toolRefreshPlan) isManifestlessBootstrap() bool {
	return plan.manifestlessBootstrapAllowed && len(plan.previousIndex) == 0
}

func (plan *toolRefreshPlan) commit(sentinelPath string, sentinelContent []byte) error {
	sentinelRel, insideRoot, err := plan.relativePath(sentinelPath)
	if err != nil {
		return err
	}
	if !insideRoot {
		return fmt.Errorf("generated sentinel %s is outside %s", sentinelPath, plan.root)
	}
	if err := plan.ensureGeneratedWriteAllowed(sentinelPath, sentinelRel, sentinelContent); err != nil {
		return err
	}
	plan.generated[sentinelRel] = ownershipManifestFile{
		Path:   sentinelRel,
		SHA256: hashBytes(sentinelContent),
	}

	manifest := buildOwnershipManifest(plan.cfg.ID, plan.generated)
	raw, err := encodeOwnershipManifest(manifest)
	if err != nil {
		return fmt.Errorf("encode ownership manifest for %s: %w", plan.cfg.ID, err)
	}
	manifestPath := filepath.Join(plan.root, generatedOwnershipManifestPath(plan.cfg))

	ops := make([]fsutil.FileTransactionOp, 0, len(plan.ops)+3)
	ops = append(ops, fsutil.RemoveFileTransactionOp(sentinelPath))
	ops = append(ops, plan.ops...)
	ops = append(ops,
		fsutil.WriteFileTransactionOp(manifestPath, raw, 0o644),
		fsutil.WriteFileTransactionOp(sentinelPath, sentinelContent, 0o644),
	)
	plan.transactionStarted = true
	if err := toolgenApplyFileTransaction(ops); err != nil {
		return fmt.Errorf("refresh %s adapter transaction: %w", plan.cfg.ID, err)
	}
	return nil
}

func (plan *toolRefreshPlan) relativePath(path string) (string, bool, error) {
	rel, err := filepath.Rel(plan.root, path)
	if err != nil {
		return "", false, err
	}
	rel = filepath.Clean(rel)
	if rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false, nil
	}
	if rel == "." {
		return "", true, nil
	}
	return filepath.ToSlash(rel), true, nil
}
