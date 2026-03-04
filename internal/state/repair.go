package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/signalridge/speclane/internal/engine/artifact"
	"github.com/signalridge/speclane/internal/fsutil"
	"github.com/signalridge/speclane/internal/model"
)

type EvidenceGCResult struct {
	DeletedPaths []string `json:"deleted_paths" yaml:"deleted_paths"`
}

const recoveredSlugMaxAttempts = 10000

func RepairCorruptConfig(root string, now time.Time) (string, error) {
	configPath := filepath.Join(root, ".spln", "config.yaml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	if _, err := model.ParseConfigYAML(raw); err == nil {
		return "", nil
	}

	backupPath := filepath.Join(
		root,
		".spln",
		"archive",
		"config",
		fmt.Sprintf("config.yaml.broken.%s.yaml", now.UTC().Format("20060102T150405Z")),
	)
	if err := fsutil.WriteFileAtomic(backupPath, raw, 0o644); err != nil {
		return "", err
	}
	if err := model.SaveConfig(configPath, model.DefaultConfig()); err != nil {
		return "", err
	}
	return backupPath, nil
}

func RunEvidenceRetentionGC(root string, retentionDays int, now time.Time) (EvidenceGCResult, error) {
	if retentionDays <= 0 {
		return EvidenceGCResult{}, nil
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)

	activeRecords, err := DiscoverActiveRecords(root)
	if err != nil {
		return EvidenceGCResult{}, err
	}
	active := map[string]struct{}{}
	for _, record := range activeRecords {
		active[record.RequestID] = struct{}{}
	}

	result := EvidenceGCResult{DeletedPaths: []string{}}
	for _, baseDir := range []string{
		filepath.Join(root, ".spln", "evidence", "tasks"),
		filepath.Join(root, ".spln", "evidence", "runs"),
		filepath.Join(root, ".spln", "evidence", "skills"),
	} {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			if isNotExist(err) {
				continue
			}
			return EvidenceGCResult{}, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				if baseDir == filepath.Join(root, ".spln", "evidence", "skills") {
					filePath := filepath.Join(baseDir, entry.Name())
					info, statErr := entry.Info()
					if statErr != nil {
						return EvidenceGCResult{}, statErr
					}
					if info.ModTime().After(cutoff) {
						continue
					}
					if err := os.Remove(filePath); err != nil && !isNotExist(err) {
						return EvidenceGCResult{}, err
					}
					result.DeletedPaths = append(result.DeletedPaths, filePath)
				}
				continue
			}
			requestID := entry.Name()
			if _, isActive := active[requestID]; isActive {
				continue
			}
			requestPath := filepath.Join(baseDir, requestID)
			deleted, err := deleteFilesOlderThan(requestPath, cutoff)
			if err != nil {
				return EvidenceGCResult{}, err
			}
			result.DeletedPaths = append(result.DeletedPaths, deleted...)
			_ = removeEmptyDirs(requestPath)
		}
	}

	slices.Sort(result.DeletedPaths)
	return result, nil
}

func RepairOrphanedGovernedAdmissions(root string) ([]string, error) {
	entries, err := os.ReadDir(AdmissionsDir(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	repaired := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		requestID := entry.Name()[:len(entry.Name())-len(".yaml")]
		admission, err := LoadAdmission(root, requestID)
		if err != nil {
			return nil, err
		}
		if admission.AdmissionStatus != model.AdmissionStatusActive {
			continue
		}
		if admission.Level != model.LevelL2 && admission.Level != model.LevelL3 {
			continue
		}
		if _, err := LoadChange(root, requestID); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		slug, err := recoveredSlug(root, requestID)
		if err != nil {
			return nil, err
		}
		if err := artifact.ScaffoldGovernedBundle(root, requestID, slug, admission.Level); err != nil {
			return nil, err
		}
		sealed, change, err := HandoffAdmissionToGoverned(
			admission,
			slug,
			admission.Level,
			maxLevelHistoryEntriesForRoot(root),
		)
		if err != nil {
			return nil, err
		}
		if err := SaveAdmission(root, sealed); err != nil {
			return nil, err
		}
		if err := SaveChange(root, change); err != nil {
			return nil, err
		}
		repaired = append(repaired, requestID)
	}

	slices.Sort(repaired)
	return repaired, nil
}

// RepairInterruptedTerminalArchive repair-forwards partially migrated terminal archives.
func RepairInterruptedTerminalArchive(root, requestID string) (bool, error) {
	repaired := false

	change, changeErr := LoadChange(root, requestID)
	if changeErr == nil {
		if change.ChangeStatus == model.ChangeStatusDone || change.ChangeStatus == model.ChangeStatusCancelled {
			var admission *model.AdmissionState
			if ad, err := LoadAdmission(root, requestID); err == nil {
				admission = &ad
			} else if !isNotExist(err) {
				return false, err
			}

			if _, err := os.Stat(ArchiveChangePath(root, requestID)); isNotExist(err) {
				if _, err := ArchiveGoverned(root, change, admission, change.ChangeStatus); err != nil {
					return false, err
				}
				repaired = true
			}
		}
	} else if !isNotExist(changeErr) {
		return false, changeErr
	}

	admission, admissionErr := LoadAdmission(root, requestID)
	if admissionErr == nil {
		if admission.AdmissionStatus == model.AdmissionStatusDone || admission.AdmissionStatus == model.AdmissionStatusCancelled {
			if _, err := os.Stat(ArchiveAdmissionPath(root, requestID)); isNotExist(err) {
				if err := ArchiveDirectAdmission(root, admission); err != nil {
					return false, err
				}
				repaired = true
			}
		}
	} else if !isNotExist(admissionErr) {
		return false, admissionErr
	}

	return repaired, nil
}

func deleteFilesOlderThan(root string, cutoff time.Time) ([]string, error) {
	deleted := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil && !isNotExist(err) {
			return err
		}
		deleted = append(deleted, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func removeEmptyDirs(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if isNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := removeEmptyDirs(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	entries, err = os.ReadDir(root)
	if err != nil {
		if isNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		if err := os.Remove(root); err != nil && !isNotExist(err) {
			return err
		}
	}
	return nil
}

func recoveredSlug(root, requestID string) (string, error) {
	base := "recovered-" + shortRequestID(requestID)
	if base == "recovered-" {
		base = "recovered-request"
	}
	candidate := base
	for i := 2; i <= recoveredSlugMaxAttempts; i++ {
		path := filepath.Join(root, "aircraft", "changes", candidate)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return candidate, nil
		}
		candidate = base + "-" + strconv.Itoa(i)
	}
	return "", fmt.Errorf("unable to allocate recovered slug after %d attempts", recoveredSlugMaxAttempts)
}

func shortRequestID(requestID string) string {
	if len(requestID) <= 8 {
		return requestID
	}
	return requestID[:8]
}

func isNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
