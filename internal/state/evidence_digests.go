package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

const EvidenceDigestsFileName = "evidence-digests.yaml"

func EvidenceDigestsPathForRead(root, slug string) string {
	return filepath.Join(verificationDirPathForRead(root, slug), EvidenceDigestsFileName)
}

func evidenceDigestsReadPathForChange(root string, change model.Change) (string, error) {
	dir, err := verificationDirForChange(root, change)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, EvidenceDigestsFileName), nil
}

func LoadEvidenceDigestsForChange(root string, change model.Change) (model.EvidenceDigests, error) {
	displayPath := EvidenceDigestsPathForRead(root, change.Slug)
	path, err := evidenceDigestsReadPathForChange(root, change)
	if err != nil {
		return model.EvidenceDigests{}, wrapExecutionSummaryLoadError(displayPath, err)
	}
	digests, err := loadEvidenceDigestsFromPath(path)
	if err != nil {
		return model.EvidenceDigests{}, wrapExecutionSummaryLoadError(path, err)
	}
	return digests, nil
}

func LoadOptionalEvidenceDigestsForChange(root string, change model.Change) (*model.EvidenceDigests, error) {
	digests, err := LoadEvidenceDigestsForChange(root, change)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &digests, nil
}

func SaveEvidenceDigests(root, slug string, digests model.EvidenceDigests) error {
	digests.Normalize()
	if err := digests.Validate(); err != nil {
		return err
	}
	dir, err := resolveVerificationDirForWrite(root, slug)
	if err != nil {
		return fmt.Errorf("resolve evidence digests dir for %q: %w", slug, err)
	}
	path := filepath.Join(dir, EvidenceDigestsFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	raw, err := yaml.Marshal(digests)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}

func loadEvidenceDigestsFromPath(path string) (model.EvidenceDigests, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return model.EvidenceDigests{}, err
	}
	var digests model.EvidenceDigests
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&digests); err != nil {
		return model.EvidenceDigests{}, fmt.Errorf("parse evidence digests: %w", err)
	}
	digests.Normalize()
	if err := digests.Validate(); err != nil {
		return model.EvidenceDigests{}, fmt.Errorf("invalid evidence digests: %w", err)
	}
	return digests, nil
}
