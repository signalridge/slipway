package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/signalridge/speclane/internal/fsutil"
	"github.com/signalridge/speclane/internal/model"
)

type EvidenceGCResult struct {
	DeletedPaths []string `json:"deleted_paths" yaml:"deleted_paths"`
}

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

func isNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
