package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/signalridge/speclane/internal/fsutil"
	"github.com/signalridge/speclane/internal/model"
	"gopkg.in/yaml.v3"
)

func ArchiveAdmissionsDir(root string) string {
	return filepath.Join(root, ".spln", "archive", "admissions")
}

func ArchiveChangesDir(root string) string {
	return filepath.Join(root, ".spln", "archive", "changes")
}

func ArchiveAdmissionPath(root, requestID string) string {
	return filepath.Join(ArchiveAdmissionsDir(root), requestID+".yaml")
}

func ArchiveChangePath(root, requestID string) string {
	return filepath.Join(ArchiveChangesDir(root), requestID+".yaml")
}

func LoadArchivedChange(root, requestID string) (model.ChangeState, error) {
	b, err := os.ReadFile(ArchiveChangePath(root, requestID))
	if err != nil {
		return model.ChangeState{}, err
	}
	var st model.ChangeState
	if err := yaml.Unmarshal(b, &st); err != nil {
		return model.ChangeState{}, err
	}
	st.Normalize(model.DefaultConfig().Execution.MaxLevelHistoryEntries)
	if err := st.Validate(); err != nil {
		return model.ChangeState{}, err
	}
	return st, nil
}

func governedArtifactsPath(root, slug string) string {
	return filepath.Join(root, "aircraft", "changes", slug)
}

func governedArtifactsArchivePath(root, slug string) string {
	return filepath.Join(root, "aircraft", "changes", "archived", slug)
}

func ValidateGovernedDoneArchivePreconditions(change model.ChangeState) error {
	if change.ChangeStatus != model.ChangeStatusActive && change.ChangeStatus != model.ChangeStatusDone {
		return fmt.Errorf("governed done archive requires change_status in {active,done}, got %q", change.ChangeStatus)
	}
	if change.Level == "" || !change.Level.IsValid() {
		return errors.New("governed done archive requires valid level")
	}
	if change.LevelSource == "" || !change.LevelSource.IsValid() {
		return errors.New("governed done archive requires valid level_source")
	}
	return nil
}

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

func HandoffAdmissionToGoverned(
	admission model.AdmissionState,
	slug string,
	level model.Level,
	maxLevelHistoryEntries int,
) (sealedAdmission model.AdmissionState, change model.ChangeState, err error) {
	if !level.IsValid() || level == model.LevelL1 {
		return model.AdmissionState{}, model.ChangeState{}, fmt.Errorf("handoff requires governed level L2/L3, got %q", level)
	}
	if admission.RequestID == "" {
		return model.AdmissionState{}, model.ChangeState{}, errors.New("admission.request_id is required")
	}
	if slug == "" {
		return model.AdmissionState{}, model.ChangeState{}, errors.New("slug is required")
	}

	sealed := admission
	now := time.Now().UTC()
	sealed.AdmissionStatus = model.AdmissionStatusSealedHandoff
	sealed.CurrentState = model.StateS1Analyze
	sealed.SealedAt = &now

	changeState := model.NewChangeState(admission.RequestID, slug)
	changeState.Level = level
	changeState.LevelSource = admission.LevelSource
	changeState.LevelHistory = append([]model.LevelHistoryEvent(nil), admission.LevelHistory...)
	changeState.LastLevelUpdateAt = admission.LastLevelUpdateAt
	changeState.RouteSnapshot = admission.RouteSnapshot
	changeState.LatestFrozenRunSummaryVersion = admission.LatestFrozenRunSummaryVersion

	if level == model.LevelL3 {
		changeState.CurrentState = model.StateS2Discover
	} else {
		changeState.CurrentState = model.StateS4SpecBundle
	}

	// Handoff trace ownership: governed lane starts fresh.
	changeState.TaskRuns = map[string]model.TaskRun{}
	changeState.ActionHistory = []model.ActionEvent{}
	changeState.EvidenceRefs = map[string]string{}

	if maxLevelHistoryEntries <= 0 {
		maxLevelHistoryEntries = model.DefaultConfig().Execution.MaxLevelHistoryEntries
	}
	sealed.Normalize(maxLevelHistoryEntries)
	changeState.Normalize(maxLevelHistoryEntries)
	return sealed, changeState, nil
}

func ArchiveDirectAdmission(root string, admission model.AdmissionState) error {
	src := AdmissionPath(root, admission.RequestID)
	dst := ArchiveAdmissionPath(root, admission.RequestID)
	return moveFile(src, dst)
}

// ArchiveGoverned handles governed archive migration and returns archived snapshot.
func ArchiveGoverned(
	root string,
	change model.ChangeState,
	admission *model.AdmissionState,
	finalStatus model.ChangeStatus,
) (model.ChangeState, error) {
	switch finalStatus {
	case model.ChangeStatusDone:
		if err := ValidateGovernedDoneArchivePreconditions(change); err != nil {
			return model.ChangeState{}, err
		}
	case model.ChangeStatusCancelled:
		if change.ChangeStatus != model.ChangeStatusCancelled && change.ChangeStatus != model.ChangeStatusActive {
			return model.ChangeState{}, fmt.Errorf(
				"governed cancel archive requires change_status in {active,cancelled}, got %q",
				change.ChangeStatus,
			)
		}
	default:
		return model.ChangeState{}, fmt.Errorf("unsupported finalStatus %q", finalStatus)
	}

	archived := change
	archived.Artifacts = FreezeArtifacts(change.Artifacts)
	archived.ChangeStatus = finalStatus
	archived.Normalize(maxLevelHistoryEntriesForRoot(root))

	// Crash-safe order:
	// 1) persist final archived lifecycle snapshot
	// 2) migrate artifact/admission targets
	// 3) remove runtime governed source
	dstChange := ArchiveChangePath(root, change.RequestID)
	b, err := yaml.Marshal(archived)
	if err != nil {
		return model.ChangeState{}, err
	}
	if err := fsutil.WriteFileAtomic(dstChange, b, 0o644); err != nil {
		return model.ChangeState{}, err
	}

	srcArtifacts := governedArtifactsPath(root, change.Slug)
	dstArtifacts := governedArtifactsArchivePath(root, change.Slug)
	if err := moveDirIfExists(srcArtifacts, dstArtifacts); err != nil {
		return model.ChangeState{}, err
	}

	if admission != nil {
		srcAdmission := AdmissionPath(root, admission.RequestID)
		dstAdmission := ArchiveAdmissionPath(root, admission.RequestID)
		if err := moveFile(srcAdmission, dstAdmission); err != nil {
			return model.ChangeState{}, err
		}
	}

	srcChange := ChangePath(root, change.RequestID)
	if err := os.Remove(srcChange); err != nil && !os.IsNotExist(err) {
		return model.ChangeState{}, err
	}

	return archived, nil
}

func moveFile(src, dst string) error {
	srcParent := filepath.Dir(src)
	dstParent := filepath.Dir(dst)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if srcParent != dstParent {
		if err := syncDir(srcParent); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return syncDir(dstParent)
}

func moveDirIfExists(src, dst string) error {
	srcParent := filepath.Dir(src)
	dstParent := filepath.Dir(dst)
	_, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if srcParent != dstParent {
		if err := syncDir(srcParent); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return syncDir(dstParent)
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
