package adapter

import (
	"fmt"
	"path/filepath"

	"github.com/signalridge/slipway/internal/fsutil"
)

var legacyRunstoreResidueCodes = map[string]string{
	"runtime":        "legacy_runtime_residue",
	"cache":          "legacy_cache_residue",
	"scope-root":     "legacy_scope_root_residue",
	"scopes":         "legacy_scopes_residue",
	"locks":          "legacy_locks_residue",
	"processes":      "legacy_processes_residue",
	"repair-backups": "legacy_repair_backups_residue",
}

func legacyRunstoreChecks(root string) []DoctorCheck {
	repository, err := fsutil.DiscoverGit(root)
	if err != nil {
		return []DoctorCheck{}
	}
	entries, err := fsutil.LstatTopLevel(filepath.Join(repository.CommonDir, "slipway"))
	if err != nil {
		return []DoctorCheck{legacyRunstoreInspectionWarning()}
	}
	checks := make([]DoctorCheck, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "runs" {
			continue
		}
		code, known := legacyRunstoreResidueCodes[entry.Name]
		if !known {
			code = "legacy_unknown_residue"
		}
		checks = append(checks, DoctorCheck{
			Code:   code,
			Status: "warning",
			HostID: "-",
			Name:   "legacy_runstore",
			Detail: fmt.Sprintf(
				"legacy runstore residue %q exists; stop the old Slipway binary, back up the Git common directory, and manually clean it if desired; no migration or automatic deletion is performed.",
				entry.Name,
			),
		})
	}
	return checks
}

func legacyRunstoreInspectionWarning() DoctorCheck {
	return DoctorCheck{
		Code:   "legacy_unknown_residue",
		Status: "warning",
		HostID: "-",
		Name:   "legacy_runstore",
		Detail: "the legacy runstore namespace could not be inspected safely; stop the old Slipway binary, back up the Git common directory, and manually inspect it if desired; no migration or automatic deletion is performed.",
	}
}
