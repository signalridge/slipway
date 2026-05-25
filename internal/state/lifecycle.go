package state

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

var renameDir = os.Rename
var promoteDir = os.Rename
var removeDirAll = os.RemoveAll

// LoadArchivedChange loads an archived change by slug.
//
// Authority: bundle archive (artifacts/changes/archived/{slug}/change.yaml) is
// the single authority.
func LoadArchivedChange(root, slug string) (model.Change, error) {
	paths, err := candidateArchivedBundlePaths(root, slug)
	if err != nil {
		return model.Change{}, err
	}
	return loadChangeFromCandidates(root, paths)
}

// ValidateDoneArchivePreconditions checks if a change can be archived as done.
func ValidateDoneArchivePreconditions(change model.Change) error {
	if change.Status != model.ChangeStatusActive && change.Status != model.ChangeStatusDone {
		return fmt.Errorf("done archive requires status in {active,done}, got %q", change.Status)
	}
	return nil
}

// FreezeArtifacts sets all artifact states to frozen.
func FreezeArtifacts(artifacts map[string]model.ArtifactState) map[string]model.ArtifactState {
	if artifacts == nil {
		return map[string]model.ArtifactState{}
	}
	out := make(map[string]model.ArtifactState, len(artifacts))
	for key, artifact := range artifacts {
		artifact.State = model.ArtifactLifecycleFrozen
		out[key] = artifact
	}
	return out
}

// ArchiveChange handles change archive migration and returns archived snapshot.
func ArchiveChange(
	root string,
	change model.Change,
	finalStatus model.ChangeStatus,
) (model.Change, error) {
	if change.Slug == "" {
		return model.Change{}, fmt.Errorf("slug is required")
	}

	switch finalStatus {
	case model.ChangeStatusDone:
		if err := ValidateDoneArchivePreconditions(change); err != nil {
			return model.Change{}, err
		}
	case model.ChangeStatusCancelled:
		if change.Status != model.ChangeStatusCancelled && change.Status != model.ChangeStatusActive {
			return model.Change{}, fmt.Errorf(
				"cancel archive requires status in {active,cancelled}, got %q",
				change.Status,
			)
		}
	default:
		return model.Change{}, fmt.Errorf("unsupported finalStatus %q", finalStatus)
	}

	archived := change
	archived.Artifacts = FreezeArtifacts(change.Artifacts)
	archived.Status = finalStatus
	scrubChangeRuntimeEvidenceRefs(&archived)
	archived.Normalize()

	// Crash-recoverable order:
	// 1) move active bundle to archived bundle root when present
	// 2) persist archived change.yaml to archived bundle root
	// 3) remove active runtime sidecars
	//
	// A crash between steps 1 and 3 leaves repair-forwardable residue
	// (archived bundle present, git-local runtime state still present).
	// Unified change layout always archives the governed bundle alongside the
	// runtime state, regardless of final status.
	srcArtifacts, err := GovernedBundleDir(root, change)
	if err != nil {
		return model.Change{}, err
	}
	paths, err := ResolveChangePaths(root, change)
	if err != nil {
		return model.Change{}, err
	}
	rewriteArchivedArtifactPaths(&archived, paths.GovernedBundleArchive)
	b, err := yaml.Marshal(archived)
	if err != nil {
		return model.Change{}, err
	}
	if err := moveDirIfExists(srcArtifacts, paths.GovernedBundleArchive); err != nil {
		return model.Change{}, err
	}
	archivedChangePath := filepath.Join(paths.GovernedBundleArchive, "change.yaml")
	if err := fsutil.WriteFileAtomic(archivedChangePath, b, 0o644); err != nil {
		if rollbackErr := moveDirIfExists(paths.GovernedBundleArchive, srcArtifacts); rollbackErr != nil {
			return model.Change{}, wrapRollbackError(err, rollbackErr)
		}
		if rollbackErr := restoreChangeAuthorityIfNeeded(root, change); rollbackErr != nil {
			return model.Change{}, wrapRollbackError(err, rollbackErr)
		}
		return model.Change{}, err
	}
	if err := scrubArchivedExecutionSummaryRuntimeEvidenceRefsAt(root, change.Slug, paths.GovernedBundleArchive); err != nil {
		if rollbackErr := moveDirIfExists(paths.GovernedBundleArchive, srcArtifacts); rollbackErr != nil {
			return model.Change{}, wrapRollbackError(err, rollbackErr)
		}
		if rollbackErr := restoreChangeAuthorityIfNeeded(root, change); rollbackErr != nil {
			return model.Change{}, wrapRollbackError(err, rollbackErr)
		}
		return model.Change{}, err
	}

	// Archived changes no longer retain git-local runtime sidecars.
	if err := removePerChangeLocalRuntimeState(root, change.Slug); err != nil {
		return model.Change{}, err
	}

	return archived, nil
}

func rewriteArchivedArtifactPaths(change *model.Change, archiveDir string) {
	if change == nil || len(change.Artifacts) == 0 {
		return
	}
	for key, artifact := range change.Artifacts {
		name := strings.TrimSpace(filepath.Base(artifact.Path))
		if name == "." || name == string(filepath.Separator) || name == "" {
			name = artifactFileNameForArchive(key, artifact)
		}
		artifact.Path = filepath.Join(archiveDir, name)
		change.Artifacts[key] = artifact
	}
}

func artifactFileNameForArchive(key string, artifact model.ArtifactState) string {
	if strings.TrimSpace(artifact.ID) != "" {
		return artifact.ID + ".md"
	}
	if strings.TrimSpace(key) != "" {
		return key + ".md"
	}
	return "artifact.md"
}

func moveDirIfExists(src, dst string) error {
	srcParent := filepath.Dir(src)
	dstParent := filepath.Dir(dst)
	_, err := os.Stat(src)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := renameDir(src, dst); err != nil {
		if !errors.Is(err, syscall.EXDEV) {
			return err
		}
		if err := stageAndMoveDirAcrossFilesystems(src, dst); err != nil {
			return err
		}
	}
	if srcParent != dstParent {
		if err := syncDir(srcParent); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return syncDir(dstParent)
}

func stageAndMoveDirAcrossFilesystems(src, dst string) error {
	dstParent := filepath.Dir(dst)
	stagedDir, err := os.MkdirTemp(dstParent, filepath.Base(dst)+".staging-")
	if err != nil {
		return err
	}
	cleanupStagedDir := true
	defer func() {
		if cleanupStagedDir {
			_ = os.RemoveAll(stagedDir)
		}
	}()

	if err := copyDirRecursive(src, stagedDir); err != nil {
		return err
	}
	if err := promoteDir(stagedDir, dst); err != nil {
		return fmt.Errorf("promote staged cross-filesystem move %q to %q: %w", stagedDir, dst, err)
	}
	cleanupStagedDir = false
	if err := removeDirAll(src); err != nil {
		if rollbackErr := removeDirAll(dst); rollbackErr != nil && !errors.Is(rollbackErr, fs.ErrNotExist) {
			return fmt.Errorf("remove source after promoting cross-filesystem move: %w (rollback failed: %v)", err, rollbackErr)
		}
		return fmt.Errorf("remove source after promoting cross-filesystem move: %w", err)
	}
	return nil
}

func copyDirRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := dst
		if rel != "." {
			target = filepath.Join(dst, rel)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case d.Type()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return copyFile(path, target, info.Mode().Perm())
		}
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return err
	}
	return dir.Close()
}

// scrubChangeRuntimeEvidenceRefs removes evidence refs that point to absolute
// local paths. Archived changes must be self-contained; runtime-local paths
// (e.g. cancel preemption evidence) become dangling after archive removes the
// per-change runtime directory. Inline text refs are allowed and preserved.
func scrubChangeRuntimeEvidenceRefs(change *model.Change) {
	for key, ref := range change.EvidenceRefs {
		if filepath.IsAbs(ref) {
			delete(change.EvidenceRefs, key)
		}
	}
}

// Archived execution summaries must not retain machine-local runtime paths, but
// archive-safe relative refs or inline text should survive.
func scrubArchivedExecutionSummaryRuntimeEvidenceRefs(root, slug string) error {
	return scrubArchivedExecutionSummaryRuntimeEvidenceRefsAt(root, slug, filepath.Join(ArchivedBundlesDir(root), slug))
}

func scrubArchivedExecutionSummaryRuntimeEvidenceRefsAt(root, slug, archiveDir string) error {
	path := filepath.Join(archiveDir, "verification", ExecutionSummaryFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	var summary model.ExecutionSummary
	if err := decodeExecutionSummaryStrict(raw, &summary); err != nil {
		// Archive should remain possible even when an optional execution summary is malformed,
		// but the malformed file must not survive as archived authority.
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return removeErr
		}
		return nil
	}
	summary.Normalize()

	changed := false
	for i := range summary.Tasks {
		if !shouldScrubArchivedExecutionSummaryEvidenceRef(root, slug, summary.Tasks[i].EvidenceRef) {
			continue
		}
		summary.Tasks[i].EvidenceRef = ""
		changed = true
	}
	if !changed {
		return nil
	}
	summary.SyncDerivedFields()
	if err := summary.Validate(); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return removeErr
		}
		return nil
	}

	scrubbed, err := yaml.Marshal(summary)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, scrubbed, 0o644)
}

func shouldScrubArchivedExecutionSummaryEvidenceRef(root, slug, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	if filepath.IsAbs(ref) {
		return true
	}

	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}
	resolvedRef := filepath.Clean(filepath.Join(normalizedRoot, ref))
	for _, runtimeRoot := range []string{
		filepath.Clean(GitStateDir(root)),
		filepath.Clean(ChangeDir(root, slug)),
	} {
		if pathWithinRoot(resolvedRef, runtimeRoot) {
			return true
		}
	}
	return false
}

func pathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
